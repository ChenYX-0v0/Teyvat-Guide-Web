#!/usr/bin/env node
/**
 * 抓「配队」攻略（米游社 ✓）—— 给角色攻略卡补「推荐配队」用 ✓
 *
 * 为什么换源 ✗（2026-10-15 用户口径 ✓）：
 *   原来源只有 biligame 的「阵容搭配」段 ✗，而 **126 / 128 张页**是模板渲染
 *   （`{{角色/阵容搭配|展示}}` ✗）⇒ raw wikitext 抓不到 ✗ ⇒ 用户问配队时"一个示例都给不出来"✗。
 *   用户给的替代来源：米游社百科「角色攻略」合集 ⇒ 挑**标题带「配队」**的攻略 ✓。
 *
 * 实测接口 ✓（2026-10-15）：
 *   搜索：GET https://bbs-api.mihoyo.com/post/wapi/searchPosts?keyword=<角色 配队攻略>&size=20
 *         ⇒ data.posts[] = { post{post_id,subject,content(⚠️只给 100 字预览 ✗)}, stat{view_num,like_num,…} }
 *   全文：GET https://bbs-api.mihoyo.com/post/wapi/getPostFull?post_id=<id>&read=1
 *         ⇒ data.post.content（纯文本 ✓ 图片位置是 `[图片]` 占位 ✗）
 *
 * 选文口径（对齐"大众理解" ✓）：
 *   ① 标题**必须**含「配队」✓（用户口径 ✓）；② 排掉提问帖 ✗（求助/怎么配/求问/吗？/？）；
 *   ③ 优先攻略帖 ✓（攻略/养成/推荐/全解/一图/合集/体系…）；④ 再按**点赞/浏览**排序 ✓（热 = 大众认 ✓）。
 *
 * 用法：
 *   node scripts/crawl-baike-teams.mjs                     # 默认 5 位冒烟（丝柯克/芙宁娜/菲林斯/伊涅芙/桑多涅）
 *   node scripts/crawl-baike-teams.mjs --all               # 全部有攻略卡的角色（不含旅行者形态 ✓）
 *   node scripts/crawl-baike-teams.mjs --names 丝柯克,芙宁娜 --per 6
 *   node scripts/crawl-baike-teams.mjs --no-full           # 只存搜索结果（不取全文，快 ✓）
 */

import fs from 'node:fs'
import path from 'node:path'

const ROOT = path.resolve(import.meta.dirname, '..')
const OUT_DIR = path.join(ROOT, 'scripts/cache/baike_teams')
const GUIDES = path.join(ROOT, 'scripts/data/character_guides.json')

const SEARCH = 'https://bbs-api.mihoyo.com/post/wapi/searchPosts'
const FULL = 'https://bbs-api.mihoyo.com/post/wapi/getPostFull'
const UA = 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Safari/537.36'
const H = { 'User-Agent': UA, Referer: 'https://www.miyoushe.com/' }

const arg = (k, d) => {
  const i = process.argv.indexOf('--' + k)
  if (i < 0) return d
  const v = process.argv[i + 1]
  return v && !v.startsWith('--') ? v : true
}

const PER_CHAR = Number(arg('per', 5))              // 每位留几篇 ✓
const WANT = String(arg('match', '配队')).split(',').map((s) => s.trim()).filter(Boolean)  // 标题必含词 ✓
const FULLTEXT = !arg('no-full', false)             // 是否取全文 ✓
const THROTTLE = Number(arg('throttle', 900))       // 对别人的站点客气点 ✓
const QUERIES = ['配队攻略', '配队推荐']             // 两轮检索，面更宽 ✓
const SMOKE = ['丝柯克', '芙宁娜', '菲林斯', '伊涅芙', '桑多涅']

const GUIDE = /攻略|养成|推荐|全解|一图|一次看懂|合集|解析|思路|体系|培养|配装/
const ASK = /求助|怎么配|求问|求带|问一下|吗？|吗\?|？|\?|不懂|急/

const sleep = (ms) => new Promise((r) => setTimeout(r, ms))
const stripHtml = (s) => String(s || '')
  .replace(/<br\s*\/?>/gi, '\n').replace(/<[^>]+>/g, '')
  .replace(/&nbsp;/g, ' ').replace(/&amp;/g, '&').replace(/\n{3,}/g, '\n\n').trim()

function allCharacters() {
  const g = JSON.parse(fs.readFileSync(GUIDES, 'utf8'))
  const names = new Set()
  for (const c of g.cards || []) if (!/^旅行者/.test(c.name)) names.add(c.name)
  return [...names]
}

const wanted = (() => {
  if (arg('all', false)) return allCharacters()
  const n = arg('names', null)
  return typeof n === 'string' ? n.split(',').map((s) => s.trim()).filter(Boolean) : SMOKE
})()

async function search(kw) {
  const r = await fetch(`${SEARCH}?keyword=${encodeURIComponent(kw)}&size=20`, { headers: H })
  const j = await r.json()
  if (j.retcode !== 0) throw new Error('retcode=' + j.retcode + ' ' + j.message)
  return j.data?.posts || []
}

async function fullText(postId) {
  const r = await fetch(`${FULL}?post_id=${postId}&read=1`, { headers: H })
  const j = await r.json()
  if (j.retcode !== 0) return ''
  return stripHtml(j.data?.post?.content || '')
}

fs.mkdirSync(OUT_DIR, { recursive: true })
console.log('  目标（' + wanted.length + '）：' + wanted.join('、') + '　每位留 ' + PER_CHAR + ' 篇　取全文=' + FULLTEXT + '\n')

const summary = []
for (const name of wanted) {
  const pool = new Map()
  for (const q of QUERIES) {
    try {
      for (const item of await search(`${name} ${q}`)) {
        const p = item?.post
        if (!p?.post_id || !p.subject) continue
        // ① 标题必含「配队」✓  ② 排掉提问帖 ✗  ③ 只要像攻略的 ✓
        if (!WANT.some((w) => p.subject.includes(w))) continue
        if (ASK.test(p.subject)) continue
        if (!GUIDE.test(p.subject)) continue
        if (!pool.has(p.post_id)) {
          pool.set(p.post_id, {
            post_id: p.post_id,
            title: stripHtml(p.subject),
            createdAt: new Date((p.created_at || 0) * 1000).toISOString().slice(0, 10),
            url: 'https://www.miyoushe.com/ys/article/' + p.post_id,
            views: item.stat?.view_num || 0,
            likes: item.stat?.like_num || 0,
            text: '',
          })
        }
      }
    } catch (e) { console.log('  ! ' + name + ' 检索「' + q + '」失败：' + e.message) }
    await sleep(THROTTLE)
  }
  const kept = [...pool.values()].sort((a, b) => b.likes - a.likes || b.views - a.views).slice(0, PER_CHAR)
  if (FULLTEXT) {
    for (const p of kept) {
      p.text = await fullText(p.post_id)
      await sleep(THROTTLE)
    }
  }
  fs.writeFileSync(path.join(OUT_DIR, `${name}.json`),
    JSON.stringify({ name, queries: QUERIES, kept: kept.length, posts: kept }, null, 2) + '\n', 'utf8')
  console.log('  ✓ ' + name.padEnd(10) + ' 候选 ' + pool.size + ' 篇 ⇒ 留 ' + kept.length + ' 篇')
  for (const p of kept) console.log('        · ' + p.createdAt + ' 赞' + String(p.likes).padStart(4) + ' 看' + String(p.views).padStart(6) + '  ' + String(p.text.length).padStart(5) + ' 字  ' + p.title.slice(0, 52))
  summary.push({ name, kept: kept.length, texts: kept.filter((p) => p.text.length > 200).length })
}

const tot = summary.reduce((a, s) => a + s.kept, 0)
const rich = summary.reduce((a, s) => a + s.texts, 0)
console.log('\n  小结：' + summary.length + ' 位 ⇒ 落盘 ' + tot + ' 篇，其中正文 >200 字的 ' + rich + ' 篇')
console.log('  ⚠️ 正文里图片只有 `[图片]` 占位 ✗ ⇒ 下一步靠**多篇取共识**+LLM 抽取结论 ✓')
console.log('  输出目录：' + OUT_DIR)
