#!/usr/bin/env node
/**
 * 派蒙剧情台词抽取（2026-10-16）
 *
 * 目的：为"派蒙人格/活人感"提示词工程提供**精准**语料 —— 只要派蒙本人的台词，
 * 且只要**新剧情**（2025-2026，默认 `所属版本 ≥ 5.3`，可调）。
 *
 * 数据源：BWiki（wiki.biligame.com/ys）任务页。
 *   · 已知：**必须带 UA + Referer**，否则拿不到；`action=raw` 给干净 wikitext ✓
 *   · 枚举：`insource:"任务类型=魔神任务"` 搜索（≈290 页），比 embeddedin 分页更精准 ✓
 *   · 版本：任务页 `{{任务|所属版本=7.0|...}}` 元数据直接可读 ✓
 *   · 台词：`==任务剧情==` 段内 `*说话人：台词` 行；派蒙行形如 `*派蒙：……` ✓
 *
 * 产物（默认写 scripts/data/）：
 *   · paimon_lines.json  结构化（含版本/任务/地区/上一句上下文）
 *   · paimon_lines.txt    纯台词（每行一条，便于人读/粘贴）
 *
 * 用法：
 *   node scripts\extract-paimon-lines.mjs                 # 魔神任务，版本 >=5.3
 *   node scripts\extract-paimon-lines.mjs --limit 20      # 冒烟测试
 *   node scripts\extract-paimon-lines.mjs --min-version 6.0 --types 魔神任务,传说任务
 */

import { get } from 'node:https'
import { writeFileSync, readFileSync, mkdirSync, existsSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const HOST = 'wiki.biligame.com'
const UA =
  'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36'
const REFERER = 'https://wiki.biligame.com/ys/'

const __dirname = dirname(fileURLToPath(import.meta.url))

// ---------- 参数 ----------
const argv = process.argv.slice(2)
const argOf = (name, def) => {
  const i = argv.indexOf(`--${name}`)
  return i >= 0 && argv[i + 1] ? argv[i + 1] : def
}
const TYPES = argOf('types', '魔神任务').split(',').map((s) => s.trim()).filter(Boolean)
const MIN_VERSION = argOf('min-version', '5.3')
const LIMIT = Number(argOf('limit', '0')) // 0 = 不限
const OFFSET = Number(argOf('offset', '0')) // 分块抓取用
const APPEND = argv.includes('--append') // 分块时累加写盘
const OUT_DIR = resolve(__dirname, argOf('out-dir', 'data'))
const CACHE_DIR = resolve(__dirname, argOf('cache-dir', 'cache/quests_raw'))
const THROTTLE_MS = Number(argOf('throttle', '350'))
const NO_CACHE = argv.includes('--no-cache')

// ---------- HTTP ----------
function fetchText(pathAndQuery, { timeout = 25000 } = {}) {
  return new Promise((resolve, reject) => {
    const req = get(
      `https://${HOST}${pathAndQuery}`,
      { headers: { 'User-Agent': UA, Referer: REFERER, Accept: '*/*' }, timeout },
      (res) => {
        let buf = ''
        res.setEncoding('utf8')
        res.on('data', (c) => (buf += c))
        res.on('end', () => resolve({ status: res.statusCode, body: buf }))
      }
    )
    req.on('timeout', () => req.destroy(new Error('timeout')))
    req.on('error', reject)
  })
}
const apiUrl = (q) => `/ys/api.php?format=json&formatversion=2&${q}`
const rawUrl = (title) => `/ys/index.php?title=${encodeURIComponent(title)}&action=raw`
const sleep = (ms) => new Promise((r) => setTimeout(r, ms))

/** 抓原始 wikitext，带本地缓存（cache/quests_raw/<安全文件名>.txt） */
async function getRawCached(title) {
  const safe = title.replace(/[\\/:*?"<>|]/g, '_')
  const file = resolve(CACHE_DIR, `${safe}.txt`)
  if (!NO_CACHE && existsSync(file)) {
    return { status: 200, body: readFileSync(file, 'utf8'), cached: true }
  }
  const r = await fetchText(rawUrl(title))
  if (r.status === 200 && r.body) {
    mkdirSync(CACHE_DIR, { recursive: true })
    writeFileSync(file, r.body, 'utf8')
  }
  return { ...r, cached: false }
}

// ---------- 版本比较 ----------
const verNum = (v) => {
  const m = String(v).match(/(\d+)\.(\d+)/)
  return m ? Number(m[1]) * 100 + Number(m[2]) : -1
}
const MIN_NUM = verNum(MIN_VERSION)
const allVersions = (s) => (String(s).match(/\d+\.\d+/g) ?? [])

// ---------- 枚举任务页 ----------
async function enumerateQuestTitles() {
  const titles = new Set()
  for (const type of TYPES) {
    let offset = 0
    while (true) {
      const q = `action=query&list=search&srnamespace=0&srlimit=50&sroffset=${offset}&srsearch=${encodeURIComponent(
        `insource:"任务类型=${type}"`
      )}`
      const r = await fetchText(apiUrl(q))
      let j
      try {
        j = JSON.parse(r.body)
      } catch {
        console.log(`  ⚠️ 枚举解析失败 http=${r.status}`)
        break
      }
      const hits = j?.query?.search ?? []
      for (const h of hits) titles.add(h.title)
      const total = j?.query?.searchinfo?.totalhits ?? hits.length
      console.log(`  [${type}] 已枚举 ${titles.size} 个标题（本类 totalhits=${total}）`)
      const cont = j?.continue
      if (!cont || hits.length === 0) break
      offset = cont.sroffset
      await sleep(THROTTLE_MS)
    }
  }
  return [...titles]
}

// ---------- 解析 ----------
function field(templateText, name) {
  const re = new RegExp(`\\|\\s*${name}\\s*=\\s*([^|\\n}]*)`)
  const m = templateText.match(re)
  if (!m) return ''
  // 去掉字段值里的 HTML 注释（如 |任务类型=魔神任务<!--魔神任务/传说任务/...-->）
  return m[1].replace(/<!--[\s\S]*?-->/g, '').trim()
}

function cleanLine(s) {
  return s
    .replace(/<!--[\s\S]*?-->/g, '')
    .replace(/<br\s*\/?>/gi, ' ')
    .replace(/\{\{[^{}]*\}\}/g, '') // 去掉 {{...}} 模板
    .replace(/\[\[([^\]|]+)\|([^\]]+)\]\]/g, '$2') // [[a|b]] -> b
    .replace(/\[\[([^\]]+)\]\]/g, '$1') // [[a]] -> a
    .replace(/'''?/g, '')
    .replace(/\s+/g, ' ')
    .trim()
}

/** 抽取页面里的派蒙台词（含上一句上下文） */
function extractPaimonLines(wikitext) {
  const lines = wikitext.split('\n')
  const out = []
  let prevSpeaker = ''
  let prevLine = ''
  for (const raw of lines) {
    const m = raw.match(/^\*\s*([^：:]{1,24})[：:](.*)$/)
    if (!m) {
      // 空行或非对话行：保留 prev 的连续性，但不重置（同段对话）
      continue
    }
    const speaker = cleanLine(m[1])
    const text = cleanLine(m[2])
    if (/^派蒙/.test(speaker) && text) {
      out.push({ speaker, text, prevSpeaker, prevLine })
    }
    if (text) {
      prevSpeaker = speaker
      prevLine = text
    }
  }
  return out
}

// ---------- 主流程 ----------
async function main() {
  console.log('==== 枚举任务页 ====')
  const titles = await enumerateQuestTitles()
  const start = OFFSET
  const end = LIMIT > 0 ? start + LIMIT : titles.length
  const limited = titles.slice(start, end)
  console.log(`\n待抓取 ${limited.length} 个页面（offset=${start}，类型=${TYPES.join('/')}，版本阈值 >=${MIN_VERSION}）\n`)

  const records = []
  const versionCount = {}
  let keptPages = 0
  let scanned = 0
  let cachedPages = 0

  for (const title of limited) {
    scanned += 1
    let text = ''
    try {
      const r = await getRawCached(title)
      if (r.status !== 200 || !r.body) {
        console.log(`  ✗ [${scanned}/${limited.length}] ${title} http=${r.status}`)
        await sleep(THROTTLE_MS)
        continue
      }
      text = r.body
      if (r.cached) cachedPages += 1
    } catch (e) {
      console.log(`  ✗ [${scanned}/${limited.length}] ${title} 失败：${e.message}`)
      await sleep(THROTTLE_MS)
      continue
    }

    const type = field(text, '任务类型')
    const versionField = field(text, '所属版本')
    const versions = allVersions(versionField)
    const maxV = versions.length ? Math.max(...versions.map(verNum)) : -1
    // 新的 "空月之歌" 线用「月之一~月之四」标注版本（无数字），一并纳入
    const isMonthLabel = /月之/.test(versionField)
    const inType = TYPES.includes(type)
    const inVersion = maxV >= MIN_NUM || isMonthLabel

    if (inType && inVersion) {
      const quest = field(text, '任务名称') || title
      const region = field(text, '任务地区')
      const series = field(text, '系列任务')
      const lines = extractPaimonLines(text)
      if (lines.length > 0) {
        keptPages += 1
        for (const l of lines) {
          records.push({
            quest: title,
            name: quest,
            version: versionField,
            maxVersion: maxV >= 0 ? `${Math.floor(maxV / 100)}.${maxV % 100}` : versionField,
            region,
            series,
            prevSpeaker: l.prevSpeaker,
            prevLine: l.prevLine,
            line: l.text,
          })
        }
      }
      console.log(`  ✓ [${scanned}/${limited.length}] ${title} v${versionField} 派蒙×${lines.length}`)
    } else {
      console.log(`  · [${scanned}/${limited.length}] ${title} 跳过（type=${type} v=${versionField}）`)
    }
    await sleep(THROTTLE_MS)
  }

  for (const rec of records) {
    const key = rec.maxVersion || '未知'
    versionCount[key] = (versionCount[key] ?? 0) + 1
  }

  mkdirSync(OUT_DIR, { recursive: true })
  const jsonPath = resolve(OUT_DIR, 'paimon_lines.json')
  const txtPath = resolve(OUT_DIR, 'paimon_lines.txt')

  let finalRecords = records
  if (APPEND) {
    // 分块模式：读取已有结果并合并去重（按 quest+line）
    try {
      const prev = JSON.parse(readFileSync(jsonPath, 'utf8'))
      const seen = new Set(prev.map((r) => `${r.quest}||${r.line}`))
      finalRecords = prev.concat(records.filter((r) => !seen.has(`${r.quest}||${r.line}`)))
    } catch {
      /* 首次抓取，无历史文件 */
    }
  }
  writeFileSync(jsonPath, JSON.stringify(finalRecords, null, 2), 'utf8')
  writeFileSync(txtPath, finalRecords.map((r) => r.line).join('\n'), 'utf8')

  console.log('\n==== 完成 ====')
  console.log(`扫描页面：${scanned}（其中缓存命中 ${cachedPages}）`)
  console.log(`命中页面（含派蒙台词）：${keptPages}`)
  console.log(`本次台词条数：${records.length}`)
  console.log(`落盘总条数：${finalRecords.length}`)
  console.log('按最高版本分布：', JSON.stringify(versionCount))
  console.log(`输出：\n  ${jsonPath}\n  ${txtPath}`)
}

main().catch((e) => {
  console.error('未捕获异常：', e)
  process.exit(1)
})
