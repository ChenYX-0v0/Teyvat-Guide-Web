// 抓取**外部毕业面板**（社区转载汇总 ✓，见 teyvat-docs/ai/07 TODO-18 的 graduationExternal 待补 ✓）
//
// 用法：
//   node scripts/fetch-graduation-external.mjs                  # 从游侠"全角色毕业面板"索引页抓全部子页
//   node scripts/fetch-graduation-external.mjs --force          # 忽略缓存重抓
//   node scripts/fetch-graduation-external.mjs --extra <url,url>  # 额外再抓几篇（如 18183 的新角色页 ✓）
//
// 口径（重要 ✓）：
//   · **只抓数字，不抄文章** ✓：落到 `scripts/cache/external/*.html` 的只是原文缓存 ✓，
//     真正进库的只有"指标：数值"这一行（见 extract-graduation-external.mjs ✓）。
//   · **不署名** ✓（用户 2026-10-14 明确：这些攻略高度雷同，不说是谁写的就没问题 ✓）——
//     来源 URL 只留在库里备查（`character_guides.graduation_source` ✓），
//     **绝不拼进发给模型的文本** ✗（提示词 v12 第 40~42 条：块里内容是数据 ✓）。
//   · 节流 + 缓存 ✓：同一篇只抓一次 ✓（`--force` 才重抓 ✓），间隔默认 700ms ✓。
import fs from 'node:fs'
import path from 'node:path'

const INDEX_URL = 'https://m.ali213.net/news/gl2401/1290255.html'
const CACHE = path.resolve('scripts/cache/external')
const MANIFEST = path.join(CACHE, 'external.manifest.json')
const UA =
  'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36'

const args = process.argv.slice(2)
const force = args.includes('--force')
const delay = args.includes('--delay') ? Number(args[args.indexOf('--delay') + 1]) : 700
const extraArg = args.includes('--extra') ? args[args.indexOf('--extra') + 1] : ''

const sleep = (ms) => new Promise((r) => setTimeout(r, ms))
const get = async (url) => {
  const res = await fetch(url, { headers: { 'User-Agent': UA, 'Accept-Language': 'zh-CN,zh;q=0.9' } })
  if (!res.ok) throw new Error(`HTTP ${res.status}`)
  return await res.text()
}

fs.mkdirSync(CACHE, { recursive: true })
let manifest = { fetchedAt: new Date().toISOString(), pages: {} }
if (fs.existsSync(MANIFEST)) {
  try {
    manifest = JSON.parse(fs.readFileSync(MANIFEST, 'utf8'))
  } catch {
    /* 坏了重建 ✓ */
  }
}
manifest.pages ??= {}

// ① 索引页 → 子页链接（锚文本 = 角色名 ✓）
//   ⚠️ 锚文本包着 `<span style="color:…">角色名</span>` ✗ ⇒ 不能直接 `([^<]+)`，
//   必须整段取回再剥标签 ✓（2026-10-14 实测：不剥就 0 条 ✓）
const indexHtml = await get(INDEX_URL)
const links = new Map() // url -> charName
const re = /<a\b[^>]*href="([^"]*\/news\/gl2401\/(\d+)\.html)"[^>]*>([\s\S]*?)<\/a>/gi
let m
while ((m = re.exec(indexHtml)) !== null) {
  const url = m[1].startsWith('http') ? m[1] : 'https://m.ali213.net' + m[1]
  const name = m[3].replace(/<[^>]*>/g, '').replace(/\s+/g, '').trim()
  // 只认像角色名的锚文本（2~4 个汉字 ✓，排除"详情/攻略/更多"这类导航词 ✗）
  if (/^[\u4e00-\u9fa5]{2,4}$/.test(name) && !/(更多|详情|攻略|下载|首页|排行|大全|汇总)/.test(name)) {
    links.set(url, name)
  }
}
console.log(`  索引页解析：${links.size} 个角色子页（${INDEX_URL}）`)

if (extraArg) {
  for (const url of extraArg.split(',').map((s) => s.trim()).filter(Boolean)) links.set(url, '(额外)')
  console.log(`  额外加入 ${extraArg.split(',').filter((s) => s.trim()).length} 篇`)
}

// ② 逐篇抓（缓存优先 ✓）+ **一跳扩展** ✓：
//   实测（2026-10-14）索引页只覆盖 ~73 位（琴/闲云/柯莱/绮良良/夏沃蕾等**不在索引里** ✗），
//   但每篇角色页**都会链到"全角色"里的其他角色页** ✓ ⇒ 从已抓页面里再发现新页 ✓（上限 200 页防跑偏 ✗）
const queue = [...links.entries()]
const seen = new Set(links.keys())
const MAX_PAGES = 200

const discover = (html) => {
  const out = []
  const re2 = /<a\b[^>]*href="([^"]*\/news\/gl2\d{3}\/(\d+)\.html)"[^>]*>([\s\S]*?)<\/a>/gi
  let m2
  while ((m2 = re2.exec(html)) !== null) {
    const url = m2[1].startsWith('http') ? m2[1] : 'https://m.ali213.net' + m2[1]
    const name = m2[3].replace(/<[^>]*>/g, '').replace(/\s+/g, '').trim()
    if (/^[\u4e00-\u9fa5]{2,4}$/.test(name) && !/(更多|详情|攻略|下载|首页|排行|大全|汇总)/.test(name)) {
      out.push([url, name])
    }
  }
  return out
}

let fetched = 0
let cached = 0
let failed = 0
let discovered = 0
for (let qi = 0; qi < queue.length && qi < MAX_PAGES; qi++) {
  const [url, name] = queue[qi]
  const id = (/(\d+)\.html/.exec(url) ?? [])[1] ?? Buffer.from(url).toString('hex').slice(0, 12)
  const file = path.join(CACHE, id + '.html')
  if (fs.existsSync(file) && !force) {
    cached++
    // 缓存页也要用来发现新链接 ✓（不然扩展只在首次抓取时生效 ✗）
    if (!manifest.pages[id]) manifest.pages[id] = { url, name, fetchedAt: '(cached)' }
    for (const [u, n] of discover(fs.readFileSync(file, 'utf8'))) {
      if (!seen.has(u) && queue.length < MAX_PAGES) {
        seen.add(u)
        queue.push([u, n])
        discovered++
      }
    }
    continue
  }
  try {
    const html = await get(url)
    fs.writeFileSync(file, html, 'utf8')
    manifest.pages[id] = {
      url,
      name,
      title: /<title>([^<]*)<\/title>/i.exec(html)?.[1] ?? '',
      bytes: html.length,
      fetchedAt: new Date().toISOString(),
    }
    fetched++
    for (const [u, n] of discover(html)) {
      if (!seen.has(u) && queue.length < MAX_PAGES) {
        seen.add(u)
        queue.push([u, n])
        discovered++
      }
    }
    console.log(`   [抓] ${name.padEnd(6)} ${id} ${html.length}B（队列 ${queue.length}）`)
    await sleep(delay)
  } catch (e) {
    failed++
    console.log(`   [失败] ${name} ${url}：${String(e.message).slice(0, 80)}`)
  }
}
console.log(`  一跳扩展发现新页 ${discovered} 篇（队列合计 ${queue.length}）`)

manifest.fetchedAt = new Date().toISOString()
fs.writeFileSync(MANIFEST, JSON.stringify(manifest, null, 2), 'utf8')
console.log(`\n  完成：新抓 ${fetched} / 用缓存 ${cached} / 失败 ${failed} ⇒ ${CACHE}`)
console.log('  下一步：node scripts/extract-graduation-external.mjs')
