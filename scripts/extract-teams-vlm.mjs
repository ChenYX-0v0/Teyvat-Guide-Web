#!/usr/bin/env node
/**
 * 配队抽取（视觉模型版 ✓）—— 给角色攻略补「推荐配队」用 ✓
 *
 * 为什么走图 ✗（2026-10-15 实测 ✓）：
 *   米游社攻略帖的**正文抓不到** ✗（`getPostFull` 的 `content=0` ✓ 未登录被剥 ✓）；
 *   而攻略的配队内容基本都在**长图**里 ✓ ⇒ 只能识图 ✓（用户口径 ✓）。
 *
 * 流程（每位角色 ✓）：
 *   ① bbs 搜索「<角色> 配队攻略 / 配队推荐」⇒ 标题含「配队」＋ 排掉提问帖 ＋ 只要攻略帖 ✓
 *   ② 按点赞/浏览排序，取**最热的一篇**（默认 ✓，可 --posts N ✓）
 *   ③ 从这篇帖子里挑**最长的图**（大概率是"一图流"✓，里面就有队伍搭配 ✓）
 *   ④ 交给 qwen3.8-flash 识图 ⇒ 要 **JSON**（队名 + 成员 + 分工 ✓）
 *   ⑤ 落盘 `scripts/cache/baike_teams/<角色>.teams.json` ＋ 记 token ✓
 *
 * 用法：
 *   node scripts/extract-teams-vlm.mjs                        # 默认 10 位（用户指定的试点 ✓）
 *   node scripts/extract-teams-vlm.mjs --names 芙宁娜,丝柯克
 *   node scripts/extract-teams-vlm.mjs --all --tries 2        # 全部角色；每篇最多试 2 张图 ✓
 *   node scripts/extract-teams-vlm.mjs --model qwen3-vl-plus  # 换模型 ✓
 */

import fs from 'node:fs'
import path from 'node:path'

const ROOT = path.resolve(import.meta.dirname, '..')
const OUT = path.join(ROOT, 'scripts/cache/baike_teams')
const ENV_FILE = 'C:/Users/HONOR/Desktop/AI派蒙服务器/ai-service/.env'
const SEARCH = 'https://bbs-api.mihoyo.com/post/wapi/searchPosts'
const UA = 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Safari/537.36'
const H = { 'User-Agent': UA, Referer: 'https://www.miyoushe.com/' }

const arg = (k, d) => {
  const i = process.argv.indexOf('--' + k)
  if (i < 0) return d
  const v = process.argv[i + 1]
  return v && !v.startsWith('--') ? v : true
}
const SMOKE10 = ['芙宁娜', '菲林斯', '丝柯克', '伊涅芙', '桑多涅', '薇斯纳', '奥黛塔', '阿罗夏', '沃雅妮莎', '那维莱特']
const NAMES = (() => {
  const n = arg('names', null)
  if (typeof n === 'string') return n.split(',').map((s) => s.trim()).filter(Boolean)
  if (arg('all', false)) return JSON.parse(fs.readFileSync(path.join(ROOT, 'scripts/data/character_guides.json'), 'utf8')).cards
    .map((c) => c.name).filter((x) => !/^旅行者/.test(x))
  return SMOKE10
})()
const POSTS = Number(arg('posts', 1))
const TRIES = Number(arg('tries', 3))
const MODEL = String(arg('model', 'qwen3.8-flash'))
const QUERIES = ['配队攻略', '配队推荐']
const GUIDE = /攻略|养成|推荐|全解|一图|一次看懂|合集|解析|思路|体系|培养|配装/
const ASK = /求助|怎么配|求问|求带|问一下|吗？|吗\?|？|\?|不懂|急/

const sleep = (ms) => new Promise((r) => setTimeout(r, ms))
const env = {}
for (const l of fs.readFileSync(ENV_FILE, 'utf8').split(/\r?\n/)) {
  const m = l.match(/^\s*([A-Z0-9_]+)\s*=\s*(.*?)\s*$/)
  if (m) env[m[1]] = m[2].replace(/^["']|["']$/g, '')
}
const KEY = env.DASHSCOPE_API_KEY
const BASE = (env.DASHSCOPE_BASE_URL || 'https://dashscope.aliyuncs.com/compatible-mode/v1').replace(/\/+$/, '')
if (!KEY) { console.log('  ✗ ai-service/.env 里没有 DASHSCOPE_API_KEY'); process.exit(1) }

const PROMPT = [
  '这是一张《原神》角色攻略长图（可能只是其中一个片段）。',
  '任务：找出图里「队伍搭配 / 配队推荐 / 阵容推荐」部分，**逐支**列出。',
  '要求：',
  '1. **只抄图上写的东西** ✓，不要补充任何图上没有的成员或作用 ✗；',
  '2. 队名照抄（哪怕是"白万芙特"这种黑话 ✓）；',
  '3. 每支队伍给出 4 名成员的名字，以及图上标注的**作用**（没标就留空字符串）；',
  '4. 如果这张图里**没有**配队内容，只返回 {"hasTeams": false}（不要编 ✗）；',
  '5. **只输出 JSON**，格式：',
  '{"hasTeams": true, "teams": [{"name": "队伍名", "members": [{"name": "角色名", "role": "作用"}]}]}',
].join('\n')

const tally = { calls: 0, imageTokens: 0, textTokens: 0, outputTokens: 0 }

async function search(kw) {
  const r = await fetch(`${SEARCH}?keyword=${encodeURIComponent(kw)}&size=20`, { headers: H })
  return (await r.json()).data?.posts || []
}

async function vlm(imageUrl) {
  const ir = await fetch(imageUrl, { headers: H })
  const buf = Buffer.from(await ir.arrayBuffer())
  const mime = imageUrl.split('.').pop().split('?')[0].toLowerCase() === 'jpg' ? 'image/jpeg' : 'image/png'
  const t0 = Date.now()
  const r = await fetch(BASE + '/chat/completions', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Authorization: 'Bearer ' + KEY },
    body: JSON.stringify({
      model: MODEL,
      temperature: 0.1,
      messages: [{ role: 'user', content: [
        { type: 'image_url', image_url: { url: `data:${mime};base64,` + buf.toString('base64') } },
        { type: 'text', text: PROMPT },
      ] }],
    }),
  })
  const j = await r.json()
  if (j.error) throw new Error(JSON.stringify(j.error).slice(0, 200))
  const u = j.usage || {}
  tally.calls++
  tally.imageTokens += u.prompt_tokens_details?.image_tokens || 0
  tally.textTokens += u.prompt_tokens_details?.text_tokens || 0
  tally.outputTokens += u.completion_tokens || 0
  return { ms: Date.now() - t0, kb: Math.round(buf.length / 1024), text: j.choices?.[0]?.message?.content || '', usage: u }
}

fs.mkdirSync(OUT, { recursive: true })
console.log('  模型=' + MODEL + '　角色=' + NAMES.length + ' 位：' + NAMES.join('、') + '\n')

const report = []
for (const name of NAMES) {
  const pool = new Map()
  for (const q of QUERIES) {
    try {
      for (const item of await search(`${name} ${q}`)) {
        const p = item?.post
        if (!p?.post_id || !p.subject) continue
        if (!p.subject.includes('配队')) continue
        if (ASK.test(p.subject)) continue
        if (!GUIDE.test(p.subject)) continue
        if (!pool.has(p.post_id)) pool.set(p.post_id, {
          post_id: p.post_id, title: p.subject, likes: item.stat?.like_num || 0, views: item.stat?.view_num || 0,
          images: (item.post.images || []).slice(), url: 'https://www.miyoushe.com/ys/article/' + p.post_id,
        })
      }
    } catch (e) { console.log('  ! ' + name + ' 检索「' + q + '」失败：' + e.message) }
    await sleep(800)
  }
  const posts = [...pool.values()].sort((a, b) => b.likes - a.likes || b.views - a.views).slice(0, POSTS)
  if (!posts.length) { console.log('  ✗ ' + name.padEnd(8) + ' 没搜到合适的配队攻略'); report.push({ name, teams: 0 }); continue }

  let got = null
  for (const post of posts) {
    // 挑"最长的那张图"（一图流 ✓）—— 用文件名里的 `_H_W.` 拿不到尺寸 ✗ ⇒ 先按顺序取前几张试 ✓
    const cands = post.images.filter((u) => !/_small|_cover|\.gif/.test(u)).slice(0, Math.max(TRIES, 1))
    console.log('  · ' + name.padEnd(8) + ' 用帖「' + post.title.slice(0, 34) + '」（赞' + post.likes + '，图 ' + post.images.length + ' 张，试前 ' + cands.length + ' 张）')
    for (const u of cands) {
      try {
        const res = await vlm(u)
        let parsed = null
        const m = res.text.match(/\{[\s\S]*\}/)
        if (m) { try { parsed = JSON.parse(m[0]) } catch {} }
        const n = parsed?.teams?.length || 0
        console.log('        ' + (parsed?.hasTeams ? '★ 抽出 ' + n + ' 支' : '· 无配队') + '　[' + res.kb + ' KB，' + (res.ms / 1000).toFixed(1) + 's，img ' + (res.usage.prompt_tokens_details?.image_tokens || 0) + ' tok]')
        if (parsed?.hasTeams && n) {
          got = { name, post, teams: parsed.teams, tried: cands.indexOf(u) + 1 }
          break
        }
      } catch (e) { console.log('        ✗ ' + e.message) }
      await sleep(400)
    }
    if (got) break
  }

  if (got) {
    const file = path.join(OUT, `${name}.teams.json`)
    fs.writeFileSync(file, JSON.stringify({
      name, model: MODEL, source: { post_id: got.post.post_id, title: got.post.title, url: got.post.url, likes: got.post.likes, views: got.post.views },
      teams: got.teams,
    }, null, 2) + '\n', 'utf8')
    console.log('        ⇒ 已存 ' + path.basename(file))
    report.push({ name, teams: got.teams.length, first: got.teams[0]?.name })
  } else report.push({ name, teams: 0 })
  await sleep(600)
}

console.log('\n  ==== 小结 ====')
for (const r of report) console.log('   ' + r.name.padEnd(8) + (r.teams ? ' 抽到 ' + r.teams + ' 支（首支：' + r.first + '）' : ' 0 支 ✗'))
console.log('\n  ==== token 账单 ====')
console.log('   调用 ' + tally.calls + ' 次；图片 ' + tally.imageTokens + ' ＋ 文本 ' + tally.textTokens + ' = 输入 ' + (tally.imageTokens + tally.textTokens) + '；输出 ' + tally.outputTokens)
console.log('   合计 ' + (tally.imageTokens + tally.textTokens + tally.outputTokens) + ' token；平均每次 ' +
  (tally.calls ? Math.round((tally.imageTokens + tally.textTokens + tally.outputTokens) / tally.calls) : 0) + ' token')
