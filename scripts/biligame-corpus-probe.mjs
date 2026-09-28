#!/usr/bin/env node
/**
 * biligame wiki 语料可得性探测（2026-09-28）
 *
 * 为什么先探：用户要先做"向量搜索库（RAG）"，而 RAG 的成败**首先取决于语料** ✗ ——
 * 没有足够、干净、可持续更新的攻略文本，向量库再漂亮也是空转 ✓。
 * 本脚本不改任何东西，只回答三个问题：
 *   ① 这个 wiki 的**体量**有多大（总条目数 / 页面数）？
 *   ② 单页 `action=raw` 拿到的**是不是干净 wikitext**、有没有攻略性内容（配队/圣遗物/武器推荐…）？
 *   ③ 抓取是否有门槛（UA/Referer、频率、是否要登录）？
 *
 * 已知（`teyvat-docs/web/14` §4）：**必须带 UA + Referer**，否则拿不到；
 * `action=raw` 给的是干净 wikitext（字段形如 `|名称=` / `|两件套效果=`），
 * 而 SMW 查询 / 分类 API / HTML 抓取都不行 ✗（同一页别用错姿势 ✓）。
 *
 * 用法：node scripts\biligame-corpus-probe.mjs
 */

import { get } from 'node:https'

const HOST = 'wiki.biligame.com'
const UA =
  'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36'
const REFERER = 'https://wiki.biligame.com/ys/'

function fetchText(pathAndQuery, { timeout = 20000 } = {}) {
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

const rawUrl = (title) => `/ys/index.php?title=${encodeURIComponent(title)}&action=raw`
const apiUrl = (q) => `/ys/api.php?format=json&formatversion=2&${q}`

const sleep = (ms) => new Promise((r) => setTimeout(r, ms))

async function main() {
  console.log('==== 1) wiki 体量（siteinfo statistics）====')
  try {
    const r = await fetchText(apiUrl('action=query&meta=siteinfo&siprop=statistics'))
    const s = JSON.parse(r.body)?.query?.statistics
    if (s) {
      console.log(`  http=${r.status}  全部页面=${s.pages}  内容条目=${s.articles}  文件=${s.images}  编辑=${s.edits}`)
    } else {
      console.log(`  http=${r.status}  解析失败：${r.body.slice(0, 200)}`)
    }
  } catch (e) {
    console.log('  失败：' + e.message)
  }
  await sleep(300)

  console.log('\n==== 2) 抽样页面：拿得到什么、有多长、像不像攻略 ====')
  // 角色页 / 套装页 / 机制页 / 攻略页 各一，看覆盖面
  const samples = ['胡桃', '炽烈的炎之魔女', '元素反应', '深境螺旋']
  for (const title of samples) {
    try {
      const r = await fetchText(rawUrl(title))
      const text = r.body
      const chars = text.length
      const looksHtml = /<div|<span|<!DOCTYPE/i.test(text.slice(0, 400))
      const head = text.replace(/\s+/g, ' ').slice(0, 150)
      console.log(`\n  【${title}】http=${r.status}  ${chars} 字符  ${looksHtml ? '⚠️ 像 HTML（不该 ✗）' : '✓ 像 wikitext'}`)
      console.log(`    开头：${head}`)
      const marks = ['圣遗物', '配队', '武器', '天赋', '命之座', '突破', '攻略', '阵容', '队伍']
      const hits = marks
        .map((m) => ({ m, n: (text.split(m).length - 1) }))
        .filter((x) => x.n > 0)
        .map((x) => `${x.m}×${x.n}`)
      console.log(`    攻略性关键词：${hits.length ? hits.join('  ') : '（一个都没命中 ✗）'}`)
    } catch (e) {
      console.log(`\n  【${title}】失败：${e.message}`)
    }
    await sleep(400) // 礼貌节流：产品化时用 15~30 分钟一轮，别秒级轮询 ✗
  }

  console.log('\n==== 3) 枚举方式（怎么把"全部角色页"列出来）====')
  console.log('  思路：用模板反查被嵌入的页面（web/14 §4 C1 对套装页就是这么做的 ✓）')
  console.log('    例：list=embeddedin&eititle=Template:角色 …')
  console.log('  ⇒ 也可用 categorymembers（若该 wiki 的角色页有分类 ✓），两者都试比只试一个稳 ✓')
}

main().catch((e) => {
  console.error('未捕获异常：', e)
  process.exit(1)
})
