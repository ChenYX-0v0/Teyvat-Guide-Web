// 抓取全站「角色攻略」页（阶段② 攻略卡语料，见 teyvat-docs/ai/07 TODO-18）
//
// 用法：node scripts/fetch-character-guides.mjs [--limit N] [--force]
//
// 设计要点（都是踩过的坑）：
//  - **枚举靠模板反查**：list=embeddedin&eititle=Template:角色攻略头 ⇒ 拿到"哪些页是角色攻略"，
//    而不是靠分类猜测（分类会混进活动/机制页 ✗）。
//  - **批量取正文**：prop=revisions + titles=A|B|C（一次最多 50 页）⇒ 3 次请求拿完 128 页 ✓，
//    比逐页 action=raw 快近 40 倍，也更礼貌 ✓。
//  - **必须带 UA**：biligame 对空 UA/脚本 UA 会拒 ✅；连不上时退避重试 ✓。
//  - **本地缓存 + manifest**：落 scripts/cache/guides_raw/<名>.wiki，并记 revid/timestamp/sha256；
//    重跑时 hash 未变就跳过 ⇒ 不重复打上游 ✓（也顺带当了后面"增量更新"的基础 ✓）。
import fs from 'node:fs'
import path from 'node:path'
import crypto from 'node:crypto'

const API = 'https://wiki.biligame.com/ys/api.php'
const UA = 'teyvat-guide/1.0 (personal fan project; contact: local dev)'
const OUT = path.resolve('scripts/cache/guides_raw')
const MANIFEST = path.resolve('scripts/cache/guides.manifest.json')
const THROTTLE_MS = 400

const args = process.argv.slice(2)
const limitArg = args.includes('--limit') ? Number(args[args.indexOf('--limit') + 1]) : 0
const force = args.includes('--force')
const sleep = (ms) => new Promise((r) => setTimeout(r, ms))

async function api(params, tries = 3) {
  const url = API + '?' + new URLSearchParams({ format: 'json', formatversion: '2', ...params })
  for (let i = 1; i <= tries; i++) {
    try {
      const resp = await fetch(url, { headers: { 'User-Agent': UA, 'Accept': 'application/json' } })
      if (!resp.ok) throw new Error('HTTP ' + resp.status)
      return await resp.json()
    } catch (err) {
      if (i === tries) throw err
      console.log(`    [重试 ${i}/${tries - 1}] ${err.message}`)
      await sleep(1200 * i)
    }
  }
}

const safeName = (t) => t.replace(/[\\/:*?"<>|]/g, '_').trim()

// ---------------- 1) 枚举 ----------------
console.log('== 1) 枚举「角色攻略」页（Template:角色攻略头 反查）==')
const emb = await api({
  action: 'query', list: 'embeddedin', eititle: 'Template:角色攻略头', einamespace: '0', eilimit: '500',
})
let titles = (emb.query?.embeddedin ?? []).map((p) => p.title)
console.log(`   命中 ${titles.length} 页`)
if (limitArg > 0) { titles = titles.slice(0, limitArg); console.log(`   （--limit ${limitArg} ⇒ 只抓前 ${titles.length} 页）`) }
if (!titles.length) { console.error('   没枚举到页面 ✗'); process.exit(1) }

// ---------------- 2) 批量取正文 ----------------
fs.mkdirSync(OUT, { recursive: true })
const manifest = fs.existsSync(MANIFEST) ? JSON.parse(fs.readFileSync(MANIFEST, 'utf8')) : { fetchedAt: null, pages: {} }
let fetched = 0, skipped = 0

console.log('\n== 2) 批量取正文（50 页/次）==')
for (let i = 0; i < titles.length; i += 50) {
  const batch = titles.slice(i, i + 50)
  const data = await api({
    action: 'query', prop: 'revisions', rvprop: 'content|ids|timestamp', rvslots: 'main',
    titles: batch.join('|'), redirects: '1',
  })
  for (const page of data.query?.pages ?? []) {
    if (page.missing) { console.log(`   缺页 ✗ ${page.title}`); continue }
    const rev = page.revisions?.[0]
    const text = rev?.slots?.main?.content ?? ''
    const sha = crypto.createHash('sha256').update(text).digest('hex').slice(0, 16)
    const prev = manifest.pages[page.title]
    const file = path.join(OUT, safeName(page.title) + '.wiki')

    if (!force && prev && prev.sha === sha && fs.existsSync(file)) { skipped++; continue }
    fs.writeFileSync(file, text, 'utf8')
    manifest.pages[page.title] = { pageid: page.pageid, revid: rev?.revid, timestamp: rev?.timestamp, sha, bytes: Buffer.byteLength(text) }
    fetched++
    console.log(`   ${String(fetched + skipped).padStart(3)}/${titles.length}  ${page.title}  ${(Buffer.byteLength(text) / 1024).toFixed(1)}KB  rev=${rev?.revid}`)
  }
  if (i + 50 < titles.length) await sleep(THROTTLE_MS)
}

manifest.fetchedAt = new Date().toISOString()
manifest.count = Object.keys(manifest.pages).length
fs.writeFileSync(MANIFEST, JSON.stringify(manifest, null, 2), 'utf8')

const sizes = Object.values(manifest.pages).map((p) => p.bytes)
const avg = sizes.length ? sizes.reduce((a, b) => a + b, 0) / sizes.length : 0
console.log(`\n== 完成 ==`)
console.log(`   新抓 ${fetched} 页、跳过（未变）${skipped} 页、manifest 共 ${manifest.count} 页`)
console.log(`   正文平均 ${(avg / 1024).toFixed(1)}KB，最小 ${(Math.min(...sizes) / 1024).toFixed(1)}KB，最大 ${(Math.max(...sizes) / 1024).toFixed(1)}KB`)
console.log(`   落盘：${OUT}`)
console.log(`   清单：${MANIFEST}`)
