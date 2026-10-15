#!/usr/bin/env node
/**
 * 角色攻略识图抽取（视觉模型版 ✓）—— **配队 ＋ 推荐武器 ＋ 圣遗物推荐**三块一起抽 ✓
 *
 * 为什么走图 ✗（2026-10-15 实测 ✓）：
 *   米游社攻略帖的**正文抓不到** ✗（`getPostFull` 的 `content=0` ✓ 未登录被剥 ✓）；
 *   而攻略的配队 / 武器 / 圣遗物内容基本都在**长图**里 ✓ ⇒ 只能识图 ✓（用户口径 ✓）。
 *
 * ⚠️ 2026-10-15 用户口径补充 ✓：「推荐武器和圣遗物推荐也拉一下，先保存到本地，
 *   然后看看要不要更新到数据库」✓ ⇒ 本脚本**一次调用同时抽三块** ✓（同一张图 ✓，
 *   多个板块一起读不额外花图钱 ✓），输出 `scripts/cache/baike_teams/<角色>.guide.json` ✓。
 *
 *   为什么要额外拉 ✗：我们的攻略卡里 weaponPick / relicPick **一位都不缺** ✓，
 *   但那来自 **biligame wiki 的模板参数**（机械解析、无幻觉 ✓）⇒ 口径是"攻略站"✓；
 *   米游社一图流代表**大众口径** ✓ ⇒ 这份数据用于**交叉核对**（差异清单见下方 compare ✓），
 *   **落后台**：差异先给人眼过 ✓，确认后才谈入库 ✓（不许自动覆盖 wiki 数据 ✗）。
 *
 * 流程（每位角色 ✓）：
 *   ① bbs 搜索「<角色> 配队攻略 / 配队推荐」⇒ 标题含「配队」＋ 排掉提问帖 ＋ 只要攻略帖 ✓
 *   ② 按点赞/浏览排序，取最热的 N 篇（`--posts` ✓）
 *   ③ 每篇里挑图（跳过头像/封面/GIF ✗，按顺序试 `--tries` 张 ✓）
 *   ④ 交给视觉模型 ⇒ 要 JSON（队伍 ＋ 武器 ＋ 圣遗物 ✓；没有的板块必须报 false ✗ 不许编 ✓）
 *   ⑤ 多图 / 多篇**合并**（按名字去重、只补空 ✗ 不覆盖已有 ✓）
 *   ⑥ 三块都拿到就收工 ✓（拿不全就把所有图试完 ✓）；落盘 + 控制台小结 ✓
 *
 * 用法：
 *   node scripts/extract-teams-vlm.mjs                        # 默认 10 位（用户指定的试点 ✓）
 *   node scripts/extract-teams-vlm.mjs --names 芙宁娜,菲林斯
 *   node scripts/extract-teams-vlm.mjs --all --skip-done       # 全部角色，跳过已抽过的 ✓
 *   node scripts/extract-teams-vlm.mjs --all --posts 2 --tries 4  # 多篇多图（更全，更费 token ✓）
 *   node scripts/extract-teams-vlm.mjs --model qwen3-vl-plus  # 换模型 ✓
 *
 * ⚠️ 端点/密钥取自 `AI派蒙服务器/ai-service/.env` 的 `DASHSCOPE_API_KEY` / `DASHSCOPE_BASE_URL` ✓
 *   （2026-10-15 起指向 MaaS token-plan：`https://token-plan.maas.qianwenaiapi.com/compatible-mode/v1` ✓）
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
const SKIP_DONE = !!arg('skip-done', false)     // 已有 <角色>.guide.json 就跳过 ✓（全量重跑时省 token ✓）
// ⚠️ 思考链：qwen3.8-flash 默认**开思考** ✗ —— 一句"收到"就烧 21 思考 token ✓（实测 2026-10-15），
//   而"照抄图上的内容"这种活儿**不需要**思考 ✓ ⇒ 默认发 `enable_thinking:false` ✓（实测 out 24→1 ✓）。
//   若发现抽取质量变差 ✗，加 `--think` 开回来 ✓（代价是输出 token 涨几倍 ✗）。
const THINK = !!arg('think', false)
const QUERIES = ['配队攻略', '配队推荐']
// 缺项回补用的检索词 ✓（配队帖里常常真没有武器/圣遗物 ✗ ⇒ 换这批词再搜一轮 ✓）
const GEAR_QUERIES = ['武器推荐', '圣遗物推荐', '出装配装']
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
  '这是一张《原神》角色攻略长图（可能只是整图里的一个片段）。',
  '图里可能有**三块**内容：① 队伍搭配 / 配队推荐 / 阵容推荐；② 武器推荐；③ 圣遗物推荐。',
  '任务：**逐块**照抄出来（哪块在图里没有，就把那块报 false ✗，绝对不要编 ✓）。',
  '要求：',
  '1. **只抄图上写的东西** ✓，不要补充图上没有的名字、理由、词条 ✗；',
  '2. 名字照抄（黑话/简称也照抄 ✓，如"白万芙特"）；',
  '3. 队伍：每支给 4 名成员的名字，以及图上标注的**作用**（没标就留空字符串 ✓）；',
  '4. 武器：按图上的**档位标签**分组照抄（如"毕业 / 平替 / 四星毕业 / 过渡"✓，图上没标就留空 ✓）；',
  '5. 圣遗物：套装名照抄（含"4件套""2+2"这种写法 ✓）；图上若写了**主词条**（时之沙/空之杯/理之冠 ⇒ 沙/杯/头 ✓）**照抄**，副词条写了也照抄 ✓，没写就留空字符串 ✗；',
  '6. 三块都没有 ⇒ 只返回 {"hasTeams": false, "hasWeapons": false, "hasArtifacts": false}；',
  '7. **只输出 JSON**（不要解释文字 ✗），格式：',
  '{"hasTeams": true, "teams": [{"name": "队伍名", "members": [{"name": "角色名", "role": "作用"}]}],',
  ' "hasWeapons": true, "weapons": [{"name": "武器名", "rank": "档位/标签（图上写的）", "why": "图上写的理由"}],',
  ' "hasArtifacts": true, "artifacts": [{"name": "套装名·4件套", "rank": "档位/标签", "why": "图上写的理由",',
  '   "mainStats": {"沙": "", "杯": "", "头": ""}, "subStats": ""}]}',
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
      ...(THINK ? {} : { enable_thinking: false }),   // 关思考链 ✓（照抄任务不需要 ✗；省输出 token ✓）
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

// ---------------------------------------------------------------- 合并（多图 / 多篇 ✓）
const norm = (s) => String(s ?? '').replace(/[\s\u3000]/g, '').replace(/[「」《》【】“”"]/g, '')
const blank = (v) => v == null || (typeof v === 'string' && !v.trim()) || (Array.isArray(v) && !v.length)

// ⚠️ OCR 尾巴 ✗（实测 2026-10-15：菲林斯图上"护摩之杖"被读成 **护摩之杖1** ✗）。
//   修法**只做减法** ✓：拿"已知名字表"（= 我们攻略卡里的角色 / 武器 / 圣遗物名 ✓）比对，
//   命中才修尾巴 ✓；**不猜、不纠错、不补全** ✗（猜错就是把名字搞脏 ✗ —— 上次"奈芙尔打月结晶"的教训 ✓）。
const KNOWN = (() => {
  const s = new Set()
  try {
    const j = JSON.parse(fs.readFileSync(path.join(ROOT, 'scripts/data/character_guides.json'), 'utf8'))
    for (const c of j.cards || []) {
      s.add(c.name)
      const w = c.weaponPick || {}
      if (w.graduation) s.add(w.graduation)
      for (const o of [...(w.options || []), ...(w.detail || [])]) if (o?.name) s.add(o.name)
      for (const st of c.relicPick?.sets || []) s.add(String(st.name).split('·')[0])
    }
  } catch { /* 名字表拿不到就不归一化 ✓（宁可原样 ✓） */ }
  try {
    const m = JSON.parse(fs.readFileSync(path.join(ROOT, 'scripts/data/character_mechanics.json'), 'utf8'))
    for (const c of m.characters || []) if (c.name) s.add(c.name)
  } catch { /* 同上 ✓ */ }
  return s
})()
const TAIL = /[\s.·・\-—:：0-9]+$/
function cleanName(n) {
  const t = String(n ?? '').trim()
  if (!t || KNOWN.has(t)) return t
  // ① 尾部数字/标点（"护摩之杖1" ✗）
  let b = t.replace(TAIL, '')
  if (b && KNOWN.has(b)) return b
  // ② 尾巴粘了档位字（"静水流涌之辉 毕业" ✗ / "血染荒城-首选" ✗）
  b = t.replace(/[\s.·・\-—:：]*(毕业|首选|次选|平替|过渡|推荐|可选|下位|上位|前期|后期|纪行|锻造|四星|五星|三星|活动|限定)$/, '')
  if (b && KNOWN.has(b)) return b
  return t
}

// 按 name 合并：**已有值不动** ✗，只补空 ✓（先命中的更权威：图更全 / 帖更热 ✓）
function mergeBy(list, add) {
  for (const it of add || []) {
    // ⚠️ 图上**常常不写队名** ✗（实测 2026-10-15：伊涅芙长图读到"队3"却被当空项丢掉 ✗）。
    //   有成员的队伍就算有效 ✓，给个占位名 ✓（合并时按成员去重会更稳，见下 ✓）；武器/圣遗物没名字才是真无效 ✗。
    if (blank(it?.name) && Array.isArray(it.members) && it.members.length) {
      it.name = '（图上未标队名·' + (list.filter((x) => /^（图上未标队名/.test(String(x.name))).length + 1) + '）'
    }
    if (blank(it?.name)) continue
    it.name = cleanName(it.name)
    // 队伍成员同理 ✓（成员的"名字"也是角色名 ✓，最容易被读脏 ✗）
    if (Array.isArray(it.members)) for (const m of it.members) if (m?.name) m.name = cleanName(m.name)
    // 未标名的队**按成员组合去重** ✓（同一支队伍在图里出现两次很常见 ✗，占位名不同会重复入库 ✗）
    const sig = (t) => (t.members || []).map((m) => norm(m?.name)).filter(Boolean).sort().join('|')
    const hit = list.find((x) => norm(x.name) === norm(it.name)) ||
      (/^（图上未标队名/.test(it.name) && sig(it) ? list.find((x) => sig(x) === sig(it)) : null)
    // `seen` = 这条在多张图/多篇里**出现了几次** ✓ ⇒ 后面清洗时当**置信度**用 ✓
    //   （同一支队伍在多份攻略里都出现 = 大众共识 ✓；只出现一次的 = 单篇口径 ✗，标注出来 ✓）
    if (!hit) { if (it.seen == null) it.seen = 1; list.push(it); continue }
    hit.seen = (hit.seen || 1) + 1
    for (const [k, v] of Object.entries(it)) {
      if (blank(v)) continue
      if (k === 'members') continue
      if (k === 'mainStats') {
        hit.mainStats = hit.mainStats || {}
        for (const [s, sv] of Object.entries(v)) if (blank(hit.mainStats[s]) && !blank(sv)) hit.mainStats[s] = sv
        continue
      }
      if (blank(hit[k])) hit[k] = v
    }
    if (Array.isArray(it.members)) {
      hit.members = hit.members || []
      mergeBy(hit.members, it.members)
    }
  }
  return list
}

// ---------------------------------------------------------------- 收集帖子 / 扫图（主流程用 ✓）
// 收集候选帖 ✓：标题必须像这一类（titleRe ✓）＋ 排掉提问帖 ✗ ＋ 只要攻略帖 ✓，按热度取前 POSTS 篇 ✓
async function collect(name, queries, titleRe) {
  const pool = new Map()
  const rejected = []
  for (const q of queries) {
    try {
      for (const item of await search(`${name} ${q}`)) {
        const p = item?.post
        if (!p?.post_id || !p.subject) continue
        if (!titleRe.test(p.subject)) continue
        // ⚠️ **标题必须含角色名** ✓ —— 实测 2026-10-15 教训 ✗：bbs 搜索会返回**别人的**攻略帖 ✓
        //   （搜「那维莱特 圣遗物推荐」竟返回『【V7.0】「奥黛塔」全攻略 | 出装配队』✗，
        //    差点把奥黛塔的武器圣遗物写进那维莱特 ✓）⇒ 宁可少要几篇 ✗，绝不张冠李戴 ✓
        //   （同"奈芙尔被拿去打月结晶"的教训 ✓）。
        if (!p.subject.includes(name)) { rejected.push(p.subject); continue }
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
  if (rejected.length) console.log('        （排掉 ' + rejected.length + ' 篇标题不含「' + name + '」的帖 ✗，例：' + rejected[0].slice(0, 30) + '）')
  return [...pool.values()].sort((a, b) => b.likes - a.likes || b.views - a.views).slice(0, POSTS)
}

// 扫这批帖的图 ⇒ 合并进 acc ✓（`need()` 报还缺哪几块 ⇒ 缺啥找啥 ✓；全齐立刻收工 ✓ 省 token ✓）
async function scanPosts(name, posts, acc, sources, need, usedPosts) {
  let calls = 0, sawImage = false
  for (const post of posts) {
    if (usedPosts) usedPosts.add(post.post_id)   // 记下扫过的帖 ✓（回补轮不再重复扫 ✗，否则 `seen` 会被自己刷高 ✓）
    const cands = post.images.filter((u) => !/_small|_cover|\.gif/.test(u)).slice(0, Math.max(TRIES, 1))
    console.log('  · ' + name.padEnd(8) + ' 用帖「' + post.title.slice(0, 34) + '」（赞' + post.likes + '，图 ' + post.images.length + ' 张，试 ' + cands.length + ' 张）')
    if (!cands.length) { console.log('        · 该帖无可用图 ✗（跳过 ✓）'); continue }

    for (const u of cands) {
      sawImage = true
      try {
        const res = await vlm(u); calls++
        let parsed = null
        const m = res.text.match(/\{[\s\S]*\}/)
        if (m) { try { parsed = JSON.parse(m[0]) } catch {} }
        if (!parsed) { console.log('        ✗ 返回不是 JSON：' + res.text.replace(/\s+/g, ' ').slice(0, 90)); await sleep(400); continue }

        const got = { t: parsed.hasTeams ? (parsed.teams?.length || 0) : 0, w: parsed.hasWeapons ? (parsed.weapons?.length || 0) : 0, a: parsed.hasArtifacts ? (parsed.artifacts?.length || 0) : 0 }
        mergeBy(acc.teams, parsed.hasTeams ? parsed.teams : [])
        mergeBy(acc.weapons, parsed.hasWeapons ? parsed.weapons : [])
        mergeBy(acc.artifacts, parsed.hasArtifacts ? parsed.artifacts : [])
        if (got.t || got.w || got.a) sources.push({ post_id: post.post_id, title: post.title, url: post.url, likes: post.likes, image: u })
        console.log('        ' + (got.t || got.w || got.a ? '★ 本图 队' + got.t + ' 武' + got.w + ' 圣' + got.a : '· 三块都无') +
          '　[合计 队' + acc.teams.length + ' 武' + acc.weapons.length + ' 圣' + acc.artifacts.length + ']' +
          '　[' + res.kb + ' KB，' + (res.ms / 1000).toFixed(1) + 's，img ' + (res.usage.prompt_tokens_details?.image_tokens || 0) + ' tok]')
        if (!need().length) break   // 三块都齐 ⇒ 收工 ✓
      } catch (e) { console.log('        ✗ ' + e.message) }
      await sleep(400)
    }
    if (!need().length) break
  }
  return { calls, sawImage }
}

// ---------------------------------------------------------------- 主流程
fs.mkdirSync(OUT, { recursive: true })
console.log('  端点=' + BASE)
console.log('  模型=' + MODEL + '　思考链=' + (THINK ? '开 ✗（费 token ✓）' : '关 ✓') + '　角色=' + NAMES.length + ' 位　每角色最多 ' + POSTS + ' 篇 × ' + TRIES + ' 图' + (SKIP_DONE ? '　（跳过已有 ✓）' : ''))
console.log('  目标：' + NAMES.join('、') + '\n')

const report = []
for (const name of NAMES) {
  const outFile = path.join(OUT, `${name}.guide.json`)
  if (SKIP_DONE && fs.existsSync(outFile)) {
    const old = JSON.parse(fs.readFileSync(outFile, 'utf8'))
    console.log('  · ' + name.padEnd(8) + ' 已有 ⇒ 跳过（配队 ' + (old.teams?.length || 0) + ' 支，武器 ' + (old.weapons?.length || 0) + ' 件，圣遗物 ' + (old.artifacts?.length || 0) + ' 套 ✓）')
    report.push({ name, teams: old.teams?.length || 0, weapons: old.weapons?.length || 0, artifacts: old.artifacts?.length || 0, skipped: true })
    continue
  }

  const acc = { teams: [], weapons: [], artifacts: [] }
  const sources = []
  const usedPosts = new Set()
  const missLabel = () => [!acc.teams.length && '配队', !acc.weapons.length && '武器', !acc.artifacts.length && '圣遗物'].filter(Boolean)
  let calls = 0, sawImage = false

  // ① 先找「配队」帖 ✓（用户口径 ✓）
  const posts = await collect(name, QUERIES, /配队/)
  if (!posts.length) console.log('  ✗ ' + name.padEnd(8) + ' 没搜到合适的配队攻略')
  const r1 = await scanPosts(name, posts, acc, sources, missLabel, usedPosts)
  calls += r1.calls; sawImage = sawImage || r1.sawImage

  // ② **缺项回补** ✓（实测 2026-10-15：那维莱特 / 丝柯克的"配队帖"里**没有**武器与圣遗物 ✗
  //    ⇒ 换「武器推荐 / 圣遗物推荐 / 出装」再搜一轮 ✓，**只补缺的那几块** ✓，全齐就不再烧 token ✓）
  //    ⚠️ 已经扫过的帖**不再扫第二遍** ✗（否则同一篇既算配队帖又算装备帖 ⇒ `seen` 虚高 ✓）
  if (missLabel().length) {
    const all2 = await collect(name, GEAR_QUERIES, /武器|圣遗物|圣遗|配装|出装|装备|毕业/)
    const posts2 = all2.filter((p) => !usedPosts.has(p.post_id))
    console.log('  · ' + name.padEnd(8) + ' 缺「' + missLabel().join('/') + '」✗ ⇒ 换装备帖再搜（' + posts2.length + ' 篇' +
      (all2.length > posts2.length ? '，去掉已扫过的 ' + (all2.length - posts2.length) + ' 篇 ✓' : '') + '）')
    const r2 = await scanPosts(name, posts2, acc, sources, missLabel, usedPosts)
    calls += r2.calls; sawImage = sawImage || r2.sawImage
  }

  // ⚠️ 丢掉"**整句当名字**"的噪声 ✗（实测 2026-10-15：伊涅芙图上那句
  //   「风信之锋等攻击力武器均可使用」被整句抄成一件"武器"✗）⇒ 只按**句式**丢 ✓，不猜不补 ✓。
  const SENT = /均可|等其他|等武器|等攻击力|等可用|皆可|同上|等等|可选武器|推荐武器$/
  const dropNoise = (arr, label) => {
    const dropped = []
    const keep = (arr || []).filter((x) => { const bad = SENT.test(String(x?.name)); if (bad) dropped.push(String(x.name)); return !bad })
    if (dropped.length) console.log('        （丢掉 ' + dropped.length + ' 条「整句当名字」的' + label + ' ✗：' + dropped[0].slice(0, 24) + '…）')
    return keep
  }

  const payload = {
    name, model: MODEL, extractedAt: new Date().toISOString().slice(0, 19),
    endpoint: BASE,
    sources,
    stats: { calls, teams: acc.teams.length, weapons: acc.weapons.length, artifacts: acc.artifacts.length },
    teams: acc.teams, weapons: dropNoise(acc.weapons, '武器'), artifacts: dropNoise(acc.artifacts, '圣遗物'),
  }
  fs.writeFileSync(outFile, JSON.stringify(payload, null, 2) + '\n', 'utf8')
  console.log('        ⇒ 已存 ' + path.basename(outFile) + (sawImage ? '' : '（⚠️ 全程无图 ✗）'))
  report.push({ name, teams: acc.teams.length, weapons: acc.weapons.length, artifacts: acc.artifacts.length, calls })
  await sleep(600)
}

console.log('\n  ==== 小结（' + report.length + ' 位 ✓）====')
for (const r of report) {
  const miss = [!r.teams ? '配队' : '', !r.weapons ? '武器' : '', !r.artifacts ? '圣遗物' : ''].filter(Boolean)
  console.log('   ' + r.name.padEnd(8) + ' 队' + String(r.teams).padStart(2) + ' 武' + String(r.weapons).padStart(2) + ' 圣' + String(r.artifacts).padStart(2) +
    (r.skipped ? '　（跳过 ✓）' : '') + (miss.length ? '　⚠️ 缺：' + miss.join('/') + ' ✗' : ' ✓'))
}
const sum = (k) => report.reduce((a, r) => a + (r[k] || 0), 0)
console.log('   合计：配队 ' + sum('teams') + ' 支，武器 ' + sum('weapons') + ' 件，圣遗物 ' + sum('artifacts') + ' 套')

const reportFile = path.join(OUT, '_extract_report.json')
fs.writeFileSync(reportFile, JSON.stringify({ at: new Date().toISOString().slice(0, 19), endpoint: BASE, model: MODEL, report }, null, 2) + '\n', 'utf8')
console.log('   明细报告：' + path.basename(reportFile))

console.log('\n  ==== token 账单 ====')
console.log('   调用 ' + tally.calls + ' 次；图片 ' + tally.imageTokens + ' ＋ 文本 ' + tally.textTokens + ' = 输入 ' + (tally.imageTokens + tally.textTokens) + '；输出 ' + tally.outputTokens)
console.log('   合计 ' + (tally.imageTokens + tally.textTokens + tally.outputTokens) + ' token；平均每次 ' +
  (tally.calls ? Math.round((tally.imageTokens + tally.textTokens + tally.outputTokens) / tally.calls) : 0) + ' token')
console.log('\n  ⚠️ 下一步（**要人过一眼** ✗）：本次只落本地 ✓ —— 与现有攻略卡（wiki 口径 ✓）逐项对差异，')
console.log('     确认后再决定要不要入库 ✓；**不许自动覆盖** ✓（本脚本不写 character_guides.json ✓）。')
