#!/usr/bin/env node
/**
 * 米游社「官方公告」读取 —— 可行性探测 / 爬虫学习样板（2026-09-28）
 *
 * ⚠️ 定位：**还不是产品代码** ✓ —— 目前只用来回答一个问题：
 *    "首页『当前祈愿 + 抽取建议』要的 UP 角色与起止时间，能不能从官方公告里**稳定结构化**拿到？"
 * 结论若成立，再照 `server/cmd/assetimport` 的路子提升为正式导入器（落库 + 后台维护兜底）。
 *
 * 实测结论（2026-09-28，本机直连）：
 *   - `hk4e-api.mihoyo.com` 与 `hk4e-api-static.mihoyo.com` 都 **200 / retcode=0** ✓
 *     （`api-takumi.mihoyo.com` 不通 ✗）
 *   - **免登录、免签名、免 Cookie** ✓ —— 只要那几个查询参数（game / game_biz / region / uid / level ...）
 *   - ⚠️ 这和 `docs/07` 里"米游社数据源被放弃"**不是一回事** ✗：
 *     那条说的是**玩家档案**接口（要 authkey、有端点级风控 1034 ✗）；
 *     公告是**公开只读**接口 ✓ —— 但**礼貌抓取**仍要守（见文件末尾"爬虫自律"）。
 *
 * 用法：
 *   node scripts/mihoyo-announce.mjs              # 列公告分组 + 找祈愿相关条目
 *   node scripts/mihoyo-announce.mjs --content    # 再抓命中的公告正文（HTML → 纯文本，看能不能取到 UP 与时间）
 *   node scripts/mihoyo-announce.mjs --dump       # 把原始 JSON 落到 %TEMP% 便于对照
 */

import { get } from 'node:https'
import { writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'

// 公共查询参数：官方客户端就是这么带的（uid/level 只用于选服与"适配内容"，随便给合法的即可 ✓）
const BASE_QUERY = {
  game: 'hk4e',
  game_biz: 'hk4e_cn',
  lang: 'zh-cn',
  bundle_id: 'hk4e_cn',
  platform: 'pc',
  region: 'cn_gf01',
  level: '60',
  uid: '100000000',
}

const HOSTS = [
  'hk4e-api.mihoyo.com',
  'hk4e-api-static.mihoyo.com', // 静态镜像：公告内容常挂这（对爬虫更友好）
]

const qs = (obj) => new URLSearchParams(obj).toString()

/** 极简 GET（用 node:https 而非 fetch：不挑 Node 版本，也顺便看清请求细节 ✓） */
function httpGetJson(host, path, params, { timeout = 15000 } = {}) {
  const url = `https://${host}${path}?${qs({ ...BASE_QUERY, ...params })}`
  return new Promise((resolve, reject) => {
    const req = get(
      url,
      {
        headers: {
          // 带个正常的 UA：官方接口对空 UA/异常 UA 的容忍度不明，礼貌且真实一点 ✓
          'User-Agent':
            'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36',
          Accept: 'application/json,text/plain,*/*',
          Referer: 'https://webstatic.mihoyo.com/',
        },
        timeout,
      },
      (res) => {
        let buf = ''
        res.setEncoding('utf8')
        res.on('data', (c) => (buf += c))
        res.on('end', () => {
          try {
            resolve({ url, status: res.statusCode, json: JSON.parse(buf) })
          } catch (e) {
            resolve({ url, status: res.statusCode, json: null, raw: buf.slice(0, 400) })
          }
        })
      }
    )
    req.on('timeout', () => req.destroy(new Error(`timeout after ${timeout}ms`)))
    req.on('error', reject)
  })
}

/** 公告分组 → 扁平条目（每条的 list 里还嵌着实际公告 ✓） */
function flattenGroups(data) {
  const groups = data?.list ?? []
  const out = []
  for (const g of groups) {
    for (const a of g.list ?? []) {
      out.push({
        groupTypeId: g.type_id,
        groupLabel: g.type_label,
        annId: a.ann_id,
        title: a.title,
        subtitle: a.subtitle,
        tag: a.tag_label,
        // 公告自带起止时间（毫秒时间戳字符串）；祈愿公告用它就能算"当前/下期"✓
        start: a.start_time,
        end: a.end_time,
      })
    }
  }
  return { groups, flat: out }
}

const stripHtml = (html) =>
  String(html ?? '')
    .replace(/<br\s*\/?>/gi, '\n')
    .replace(/<[^>]+>/g, '')
    .replace(/&nbsp;/g, ' ')
    .replace(/&amp;/g, '&')
    .replace(/\n{3,}/g, '\n\n')
    .trim()

/**
 * 时间字段是**绝对时间字符串**（如 `2026-09-21 12:00:00`，UTC+8）—— **不是毫秒时间戳** ✗
 * （2026-09-28 踩到：按时间戳解析 ⇒ 列表里的起止时间全打印成 "-" ✓）
 * 兼容两种：字符串直接原样显示；纯数字才按毫秒时间戳转。
 */
const fmt = (v) => {
  if (typeof v === 'string' && v.trim()) {
    if (/^\d{4}-\d{2}-\d{2}/.test(v.trim())) return v.trim() + ' (UTC+8)'
    const n = Number(v)
    if (Number.isFinite(n) && n > 0) return new Date(n).toISOString().replace('T', ' ').slice(0, 16) + 'Z'
  }
  return '-'
}

// 祈愿相关：标题/标签命中即可（官方用词：角色活动祈愿 / 神铸赋形 / 集录祈愿 …）
const GACHA_RE = /祈愿|神铸赋形|集录/

async function main() {
  const argv = new Set(process.argv.slice(2))
  const withContent = argv.has('--content')

  console.log('==== 1) 尝试各 host ====')
  let ok = null
  for (const host of HOSTS) {
    try {
      const r = await httpGetJson(host, '/common/hk4e_cn/announcement/api/getAnnList', {})
      const rc = r.json?.retcode
      const n = r.json?.data?.list?.length ?? 0
      console.log(`  ${host.padEnd(34)} http=${r.status} retcode=${rc} 分组=${n}`)
      if (r.status === 200 && rc === 0 && !ok) ok = { host, json: r.json }
    } catch (e) {
      console.log(`  ${host.padEnd(34)} 失败：${e.message}`)
    }
  }
  if (!ok) {
    console.error('  ✗ 两个 host 都没拿到数据')
    process.exit(1)
  }

  console.log(`\n==== 2) 公告分组（用 ${ok.host}）====`)
  const { groups, flat } = flattenGroups(ok.json.data)
  for (const g of groups) {
    console.log(`  type_id=${String(g.type_id).padEnd(3)} ${String(g.type_label).padEnd(8)} ${(g.list ?? []).length} 条`)
  }

  console.log(`\n==== 3) 命中"祈愿"的公告（共 ${flat.filter((a) => GACHA_RE.test(a.title ?? '')).length} 条）====`)
  const gacha = flat.filter((a) => GACHA_RE.test(a.title ?? '') || GACHA_RE.test(a.tag ?? ''))
  for (const a of gacha.slice(0, 12)) {
    console.log(`  ann_id=${String(a.annId).padEnd(7)} [${a.groupLabel}/${a.tag ?? '-'}] ${String(a.title).slice(0, 46)}`)
    console.log(`           ${fmt(a.start)} ~ ${fmt(a.end)}`)
  }
  if (!gacha.length) console.log('  （没有标题含"祈愿"的条目 —— 需要改判据，见下一步）')

  // 时间字段到底填没填？"当前/下期"要靠它机器判定 ⇒ 必须看原始值 ✓
  if (gacha.length) {
    const rawItem = (id) => groups.flatMap((g) => g.list ?? []).find((x) => x.ann_id === id) ?? {}
    console.log('\n  （原始时间字段核对）')
    for (const a of gacha.slice(0, 5)) {
      const r = rawItem(a.annId)
      console.log(
        `    ann_id=${String(a.annId).padEnd(7)} start_time=${JSON.stringify(r.start_time)} end_time=${JSON.stringify(r.end_time)}` +
          ` tag_start_time=${JSON.stringify(r.tag_start_time)} tag_end_time=${JSON.stringify(r.tag_end_time)}`
      )
    }
  }

  if (argv.has('--fields')) {
    // 这一步是为了**看字段名**（"当前/下期"要靠起止时间 ⇒ 先确认它叫什么 ✓）
    console.log('\n==== 3.5) 字段名探查 ====')
    const g0 = groups[0]
    console.log(`  分组项字段：${Object.keys(g0).join(', ')}`)
    const a0 = (g0.list ?? [])[0]
    if (a0) console.log(`  公告项字段：${Object.keys(a0).join(', ')}`)
    const sample = gacha[0]
    if (sample) {
      const raw = flat.find((x) => x.annId === sample.annId)
      void raw
      const r = await httpGetJson(ok.host, '/common/hk4e_cn/announcement/api/getAnnContent', { ann_id: sample.annId })
      const item = r.json?.data?.list?.find((x) => x.ann_id === sample.annId) ?? r.json?.data?.list?.[0]
      if (item) {
        console.log(`  正文项字段：${Object.keys(item).join(', ')}`)
        console.log(`  正文项时间字段：start=${item.start_time ?? '-'} end=${item.end_time ?? '-'}`)
        console.log('  原始 HTML（前 900 字，用于判断"祈愿介绍"表格好不好解）：')
        console.log(String(item.content ?? '').slice(0, 900).split('\n').map((l) => '    ' + l).join('\n'))
      }
    }
  }

  if (argv.has('--dump')) {
    const p = join(tmpdir(), 'mihoyo-annlist.json')
    writeFileSync(p, JSON.stringify(ok.json, null, 2), 'utf8')
    console.log(`\n  原始 JSON 已落盘：${p}`)
  }

  if (!withContent || !gacha.length) {
    console.log('\n提示：加 --content 抓命中条目的正文（看能否取出 UP 角色与起止时间）；加 --dump 落盘原始 JSON')
    return
  }

  console.log('\n==== 4) 抓正文（看能否结构化出 UP 角色）====')
  for (const a of gacha.slice(0, 2)) {
    try {
      const r = await httpGetJson(ok.host, '/common/hk4e_cn/announcement/api/getAnnContent', { ann_id: a.annId })
      const item = r.json?.data?.list?.find((x) => x.ann_id === a.annId) ?? r.json?.data?.list?.[0]
      const text = stripHtml(item?.content)
      console.log(`\n  --- ann_id=${a.annId} 《${a.title}》 ---`)
      console.log(`  正文长度 ${text.length} 字；前 700 字：`)
      console.log(
        text
          .slice(0, 700)
          .split('\n')
          .map((l) => '    ' + l)
          .join('\n')
      )
    } catch (e) {
      console.log(`  ann_id=${a.annId} 抓取失败：${e.message}`)
    }
  }
}

main().catch((e) => {
  console.error('未捕获异常：', e)
  process.exit(1)
})

/**
 * 爬虫自律（写给自己看的规矩 ✓）
 *   1) **只读公开公告**，不带任何用户凭据；不碰需要 authkey 的祈愿历史接口 ✗（那是用户私密数据）
 *   2) **节流**：公告一天更新几次而已 ⇒ 产品化时用「15~30 分钟一次」足够；不要秒级轮询 ✗
 *   3) **缓存 + 条件请求**：正文抓过一次就落库/落盘，别重复打（若接口给 ETag/Last-Modified 就用上）
 *   4) **失败要能降级**：抓不到就显示"本期数据待更新"，绝不展示上一期卡池（会说错话 ✗）
 *   5) **只取结构化字段**（UP 角色 / 武器 / 起止时间），**不整段搬运公告原文**（版权与"内容一致性"都要避 ✗）
 *   6) 把 host 写进配置：官方换域名/加签名是常态 ⇒ 一层可切换的适配（本次已实测两个 host 可互为备份 ✓）
 */
