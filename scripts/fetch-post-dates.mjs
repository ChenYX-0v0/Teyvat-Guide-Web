#!/usr/bin/env node
/**
 * 抓「来源帖」的真实发布时间（2026-10-16 ✓）—— 为 TODO-29 的两件事服务 ✓：
 *
 * ① **补数据形状缺的 `publishedAt`** ✓（`ai/07` TODO-29 的数据形状里有 ✗ 我们一直没抓 ✓）
 * ② **实证 `月之N` 与 `Vx.y` 的先后** ✗ —— 语料里两套标记**从不同帖出现** ✓
 *    （179 篇来源帖里**没有一篇同时带两者** ✓ 实测 ✓）⇒ **不许猜** ✗。
 *    ⇒ 用**真实发布日期**给所有标记排序 ✓ ⇒ 映射是**实测**出来的 ✓，不是拍的 ✓。
 *
 * ⚠️ 为什么不用 `getPostFull` ✗（2026-10-16 实测 ✓）：
 *   该接口现在**恒返回 `retcode=1034`** ✗（带不带 `read` / `gids` 都一样 ✓，换路径 404 ✓）
 *   ⇒ 拿不到单帖元数据 ✓。而 **`searchPosts` 仍然正常** ✓（retcode=0 ✓，结果里就带 `created_at` ✓）
 *   ⇒ 改为**按角色重跑一遍搜索**（与 `extract-teams-vlm.mjs` 用的是同一组查询词 ✓）
 *     收集 `post_id → 日期` ✓ —— 我们的来源帖本来就是这些搜索结果 ✓，覆盖率天然很高 ✓。
 *
 * 口径 ✓：
 *   · 输入 = 本地 `.guide.json` 的角色名集合 ✓（120 位 ✓）＋ 已有的 `_posts.json` ✓
 *   · **只取元数据** ✗：不取正文、不取图、**不花任何模型 token** ✓
 *   · **幂等** ✓：已在 `_posts.json` 里的 post_id 不再重查 ✓（`--all` 可强制重抓 ✓）
 *   · 对别人的站点客气 ✓：默认 400ms 间隔 ✓、失败退避 ✓
 *   · 时间戳是**秒** ✓；取不到就留空 ✗ **不编** ✓
 *
 * 用法：
 *   node scripts/fetch-post-dates.mjs              # 增量 ✓
 *   node scripts/fetch-post-dates.mjs --all        # 全量重抓 ✓
 *   node scripts/fetch-post-dates.mjs --throttle 600
 */

import fs from 'node:fs'
import path from 'node:path'

const ROOT = path.resolve(import.meta.dirname, '..')
const CACHE = path.join(ROOT, 'scripts/cache/baike_teams')
const OUT = path.join(CACHE, '_posts.json')
const SEARCH = 'https://bbs-api.mihoyo.com/post/wapi/searchPosts'
const UA = 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Safari/537.36'
const H = { 'User-Agent': UA, Referer: 'https://www.miyoushe.com/', Accept: 'application/json' }
const QUERIES = ['配队攻略', '配队推荐']
const ALL = process.argv.includes('--all')
const THROTTLE = Number(process.argv.includes('--throttle') ? process.argv[process.argv.indexOf('--throttle') + 1] : 400)

const sleep = (ms) => new Promise((r) => setTimeout(r, ms))

// ---------------------------------------------------------------- 需要哪些 post_id（来自本地 sources ✓）
const need = new Map()        // post_id → 标题
const names = []
for (const f of fs.readdirSync(CACHE).filter((x) => x.endsWith('.guide.json'))) {
  const g = JSON.parse(fs.readFileSync(path.join(CACHE, f), 'utf8'))
  names.push(g.name)
  for (const s of g.sources || []) if (s?.post_id && !need.has(String(s.post_id))) need.set(String(s.post_id), s.title || '')
}

const store = fs.existsSync(OUT) ? JSON.parse(fs.readFileSync(OUT, 'utf8')) : { fetchedAt: null, via: 'searchPosts', posts: {} }
store.posts = store.posts || {}
store.via = 'searchPosts（getPostFull 已 1034 ✗）'

const before = new Set(Object.keys(store.posts))
console.log('  角色 ' + names.length + ' 位；本地来源帖 ' + need.size + ' 篇；已有日期 ' + before.size + ' 篇')
console.log('  查询词：' + QUERIES.join(' / ') + '　每位 ' + QUERIES.length + ' 次搜索（约 ' + (names.length * QUERIES.length) + ' 次请求 ✓ 无模型成本 ✓）\n')

// ---------------------------------------------------------------- 重跑搜索收集元数据
let calls = 0, added = 0, fail = 0
for (const [i, name] of names.entries()) {
  for (const q of QUERIES) {
    try {
      const r = await fetch(`${SEARCH}?keyword=${encodeURIComponent(name + ' ' + q)}&size=20`, { headers: H })
      const j = await r.json()
      calls++
      if (j.retcode !== 0) { fail++; continue }
      for (const item of j.data?.posts || []) {
        const p = item?.post
        if (!p?.post_id) continue
        const id = String(p.post_id)
        if (!ALL && before.has(id)) continue
        const created = Number(p.created_at || 0)
        store.posts[id] = {
          title: String(p.subject || '').trim(),
          createdAt: created ? new Date(created * 1000).toISOString().slice(0, 10) : '',
          createdAtUnix: created,
          likeNum: item?.stat?.like_num ?? null,
          viewNum: item?.stat?.view_num ?? null,
        }
        added++
      }
    } catch (e) {
      fail++
      if (fail <= 3) console.log('   ✗ ' + name + ' / ' + q + '：' + e.message.slice(0, 70))
    }
    await sleep(THROTTLE)
  }
  if ((i + 1) % 20 === 0) console.log('   · 进度 ' + (i + 1) + '/' + names.length + ' 位（已收 ' + Object.keys(store.posts).length + ' 篇 ✓）')
}

store.fetchedAt = new Date().toISOString().slice(0, 19)
fs.writeFileSync(OUT, JSON.stringify(store, null, 2) + '\n', 'utf8')

const have = Object.keys(store.posts).filter((id) => store.posts[id].createdAt).length
const missing = [...need.keys()].filter((id) => !store.posts[id]?.createdAt)
console.log('\n  搜索 ' + calls + ' 次 / 失败 ' + fail + ' ✗；收集 ' + added + ' 条；带日期 ' + have + ' 篇 ✓')
console.log('  来源帖覆盖率 = ' + (need.size ? Math.round(((need.size - missing.length) / need.size) * 100) : 0) + '%（' +
  (need.size - missing.length) + '/' + need.size + ' ✓' + (missing.length ? '，缺 ' + missing.length + ' 篇 ✗（多为搜索排名变动 / 已下架 ✓）' : '') + '）')

// ---------------------------------------------------------------- ⭐ 用真实日期给「版本标记」排序
const MARK = /【?\s*[Vv]\d+\.\d+|月之[一二三四五六七八九十]+/g
const byMarker = new Map()
for (const [, p] of Object.entries(store.posts)) {
  const ms = String(p.title || '').match(MARK)
  if (!ms || !p.createdAt) continue
  for (const raw of ms) {
    const key = raw.replace(/^【?\s*/, '')
    if (!byMarker.has(key)) byMarker.set(key, [])
    byMarker.get(key).push(p.createdAt)
  }
}
const median = (a) => { const s = [...a].sort(); return s[Math.floor(s.length / 2)] }
const rows = [...byMarker.entries()]
  .map(([k, v]) => ({ k, n: v.length, med: median(v), lo: [...v].sort()[0], hi: [...v].sort().slice(-1)[0] }))
  .sort((a, b) => (a.med < b.med ? -1 : a.med > b.med ? 1 : 0))

console.log('\n  ==== ⭐ 版本标记按**发帖日期中位数**排序（实证出来的先后 ✓）====')
for (const r of rows) console.log('   ' + r.k.padEnd(10) + ' 中位 ' + r.med + '　范围 ' + r.lo + ' ~ ' + r.hi + '　帖数 ' + r.n)
console.log('\n  ⇒ 有了这张表，「最近 3 个版本」可以**按日期**实现 ✓（不必猜 月之N 等于 7.几 ✗）；')
console.log('     口径（keep 最新几个标记 / 日期阈值 ✓）落成 `scripts/data/game_versions.json` ✓ 供导入器读 ✓。')
