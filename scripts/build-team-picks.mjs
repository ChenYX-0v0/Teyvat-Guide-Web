#!/usr/bin/env node
/**
 * 把识图抽到的「配队」**清洗后**并进角色攻略卡的 **`team`** 字段（2026-10-15 用户口径 ✓）
 *
 * 用户决定 ✓：「配队要**先清洗再入库**（拆 A/B 备选 ＋ 用机制卡交叉校验 ＋ 加防回潮测试）」
 *   ⇒ 本脚本是 **`.guide.json` → 攻略卡** 的唯一入口 ✓（抽取脚本**只落本地** ✗，从不直接写卡 ✓）。
 *
 * 输入：`scripts/cache/baike_teams/<角色>.guide.json` ✓
 * 输出：`scripts/data/character_guides.json` 的 **`team`** 字段 ✓（**只动这一列** ✗）
 *      ⇒ 之后照旧 `cd server && go run ./cmd/guideimport` 进库 ✓（本脚本**不连数据库** ✓）
 *
 * ⚠️⚠️ **为什么是"追加"而不是"覆盖"** ✗（2026-10-15 差点犯的错 ✓）：
 *   攻略卡的 `team` **本来就有内容** ✓ —— 121 张卡是从 biligame wiki 抓的**散文式配队思路** ✓
 *   （`teamFrom=page/reverse` ✓，里面也有具体队友名 ✓）。我一开始用错了字段名（查 `teamPick` ✗）
 *   才误以为"128 位全空"✗ ⇒ 若照那个结论**覆盖**，就是把 wiki 的原文**删掉** ✗（信息只少不多 ✗）。
 *   ⇒ 现在只**追加我们这一段** ✓：开头一个**固定小标题** ✓，重跑时先按它**剥掉上一次的段** ✓
 *   ⇒ 幂等 ✓、原 wiki 文字**一字不动** ✓、`teamFrom` 保持原值 ✓（它描述的是 wiki 那半 ✓）。
 *
 * 清洗工序（每条都对得上一次踩坑 ✓）：
 *   ① **拆 A/B 备选** ✓：图上「万叶/菈乌玛」「2号位备选：申鹤」✗ ⇒ 主选进正文、其余写「备选：X」✓
 *      （提示词"不要让用户继续猜" ✓ ⇒ 备选也得写出来 ✗ 不许丢 ✓）
 *   ② **位置 / 职能词不算人名** ✗：`增益生存副C`、`4号火系`、`班`（单字 ✗）一律丢 ✓
 *   ③ **别名归一** ✓：`万叶→枫原万叶`、`神子→八重神子` ✓（唯一事实源 = character_aliases.json ✓）
 *      ⇒ 注入文本里的名字与库里**完全一致** ✓（主服务的角色命中也能认 ✓）
 *   ④ **机制卡交叉校验** ✓：主角 `conflicts` 点到的人出现在队里 ⇒ 标 ⚠️ ✓（现只有 9 位有机制卡 ✓）
 *   ⑤ **不署名** ✓（用户 2026-10-14 口径 ✓）：来源帖只留在本地 `.guide.json` ✗
 *      ⇒ team 文本里**不许出现**平台名 / URL / 作者 / 点赞数 ✓
 *   ⑥ 每角色最多 `--cap` 支（默认 5 ✓）；有名队 ≥2 人 ✓、**未标名队 ≥3 人** ✓（后者读得最脏 ✗）
 *
 * 用法：
 *   node scripts/build-team-picks.mjs --dry     # 只打印（不写盘 ✓）—— 改规则时先这样看 ✓
 *   node scripts/build-team-picks.mjs          # 写进 character_guides.json ✓（追加 ✓，可重复跑 ✓）
 *   node scripts/build-team-picks.mjs --check  # 防回潮自检 ✓（有硬伤 exit 1 ✓）
 *   node scripts/build-team-picks.mjs --names 芙宁娜,菲林斯 --dry
 */

import fs from 'node:fs'
import path from 'node:path'

const ROOT = path.resolve(import.meta.dirname, '..')
const GUIDE_JSON = path.join(ROOT, 'scripts/data/character_guides.json')
const CACHE = path.join(ROOT, 'scripts/cache/baike_teams')
const MECH_JSON = path.join(ROOT, 'scripts/data/character_mechanics.json')
const ALIAS_JSON = path.join(ROOT, 'scripts/data/character_aliases.json')

// 我们自己那一段的**固定小标题** ✓（幂等靠它认边界 ✓；文案里**不带平台名** ✗ —— 不署名 ✓）
const HEAD = '【常见配队（图片攻略口径）】'

const arg = (k, d) => {
  const i = process.argv.indexOf('--' + k)
  if (i < 0) return d
  const v = process.argv[i + 1]
  return v && !v.startsWith('--') ? v : true
}
const DRY = !!arg('dry', false)
const CHECK = !!arg('check', false)
const CAP = Number(arg('cap', 5))
const NAMES = typeof arg('names', null) === 'string' ? String(arg('names')).split(',').map((s) => s.trim()).filter(Boolean) : null

// ---------------------------------------------------------------- 载入
const guidesDoc = JSON.parse(fs.readFileSync(GUIDE_JSON, 'utf8'))
const cards = guidesDoc.cards || []
const cardByName = new Map(cards.map((c) => [c.name, c]))

const roster = new Set(cards.map((c) => c.name))
try {
  const m = JSON.parse(fs.readFileSync(MECH_JSON, 'utf8'))
  for (const c of m.characters || []) if (c.name) roster.add(c.name)
} catch { /* 拿不到机制卡名单就不做交叉校验 ✓ */ }

const aliasMap = new Map()
try {
  const a = JSON.parse(fs.readFileSync(ALIAS_JSON, 'utf8'))
  for (const x of a.aliases || []) if (x.alias && x.name) aliasMap.set(x.alias, x.name)
} catch { /* 拿不到别名表就原样 ✓ */ }
const withAlias = (n) => aliasMap.get(n) || n

const conflictText = new Map()
try {
  const m = JSON.parse(fs.readFileSync(MECH_JSON, 'utf8'))
  for (const c of m.characters || []) {
    if (!c.name) continue
    const t = [...(c.conflicts || []), ...(c.cannot || [])].join(' ')
    if (t) conflictText.set(c.name, t)
  }
} catch { /* 同上 ✓ */ }

// ---------------------------------------------------------------- 成员名判定
const ALT_SPLIT = /[\/／、，,]|\s+或\s*|\s*\|\s*/      // "A/B"、"A、B"、"A 或 B" 都是"主选/备选" ✓
const UNNAMED = /^（图上未标队名·(\d+)）$/
const JUNK = /副C|主C|增益|生存|续航|增伤|减伤|治疗|护盾|辅助|站场|后台|挂水|挂草|挂雷|挂火|挂冰|元素|水系|火系|雷系|冰系|风系|岩系|草系|物理|反应|输出|水位|草位|冰位|火位|雷位|风位|岩位|C位|号位|号$|位$|换为|搭配|任意|其他|可选|优先|功能/
const ALT_HINT = /备选|替补|可换|下位|平替|替代/

function resolveToken(tok) {
  const t0 = String(tok ?? '').trim().replace(/[（(]\d+[)）]$/, '').replace(/^[-·•\s]+/, '').trim()   // "宵宫(1)" ⇒ 宵宫 ✓
  if (!t0) return null
  if (roster.has(t0)) return { name: t0, alt: false }
  if (t0.length < 2) return null            // 单字缩写（"班"/"芙"/"行"✗）⇒ 丢掉 ✓（不猜是谁 ✗）
  const a = aliasMap.get(t0)
  if (a) return { name: a, alt: false }
  if (JUNK.test(t0) || ALT_HINT.test(t0)) {
    const m = /[：:]\s*([^：:]+)$/.exec(t0)   // "2号位备选：申鹤" ⇒ 申鹤 ✓
    if (m) {
      const inner = m[1].trim()
      if (roster.has(inner)) return { name: inner, alt: true }
      const ai = aliasMap.get(inner)
      if (ai) return { name: ai, alt: true }
    }
    return null                              // 纯职能 / 位置词 ⇒ 不是成员 ✗
  }
  return { name: withAlias(t0), alt: false } // 认不出但不像职能词（"冰主"这类旅行者形态 ✓）⇒ 留着并报告 ✓
}

function parseMembers(team) {
  const out = []
  for (const m of team.members || []) {
    const raw = String(m?.name ?? '').trim()
    if (!raw) continue
    const parts = raw.split(ALT_SPLIT).map((s) => s.trim()).filter(Boolean)
    const role = String(m?.role ?? '').trim()
    let first = true
    for (const p of parts) {
      const r = resolveToken(p)
      if (!r) continue
      if (first) {
        first = false
        if (r.alt && out.length) { out[out.length - 1].alts.push(r.name); continue }
        out.push({ main: r.name, alts: [], role: r.alt ? '' : role })
        continue
      }
      // 同一格并列的名字 = **备选** ✓（踩过的坑 ✗：一开始当独立成员 ⇒ 丝柯克那队被拉成"6 人"✗）
      if (out.length) out[out.length - 1].alts.push(r.name)
      else out.push({ main: r.name, alts: [], role: '' })
    }
  }
  const seen = new Set()
  return out.filter((x) => (seen.has(x.main) ? false : (seen.add(x.main), true)))
}

function teamLine(name, team) {
  const raw = String(team.name ?? '').trim()
  const un = UNNAMED.exec(raw)
  const label = un ? '未标名队伍 ' + un[1] : raw
  const star = (team.seen || 0) >= 2 ? '★' : ''      // ⚠️ 实测 seen 几乎全是 1 ✗ ⇒ 实际很少打星 ✓

  const mem = parseMembers(team)
  const unknown = mem.filter((m) => !roster.has(m.main)).map((m) => m.main)
  const warn = []
  const ct = conflictText.get(name)
  if (ct) for (const m of mem) if (ct.includes(m.main) || (m.alts || []).some((a) => ct.includes(a))) warn.push(m.main)

  const body = mem.map((m) => {
    const alt = m.alts.length ? '；备选：' + m.alts.join('、') : ''
    const role = m.role ? m.role + alt : alt.replace(/^；/, '')
    return m.main + (role ? '（' + role + '）' : '')
  }).join(' ＋ ')

  return {
    text: '· ' + label + (star ? ' ' + star : '') + '：' + body + (warn.length ? '　⚠️ 与机制卡冲突：' + warn.join('、') : ''),
    star: !!star, warn, unknown,
  }
}

// ---------------------------------------------------------------- 主流程
const files = fs.readdirSync(CACHE).filter((f) => f.endsWith('.guide.json')).sort()
const report = { built: [], empty: [], unknownNames: new Map(), conflicts: 0, teams: 0, capped: [] }
const applied = new Map()      // name → 我们那一段的文本 ✓

for (const f of files) {
  const g = JSON.parse(fs.readFileSync(CACHE + '/' + f, 'utf8'))
  const name = g.name
  if (NAMES && !NAMES.includes(name)) continue

  const teams = (g.teams || []).filter((t) => parseMembers(t).length >= (UNNAMED.test(String(t.name ?? '').trim()) ? 3 : 2))
  if (!teams.length) { report.empty.push(name); continue }

  const rows = teams.map((t) => teamLine(name, t))
  for (const r of rows) {
    for (const u of r.unknown) report.unknownNames.set(u, (report.unknownNames.get(u) || 0) + 1)
    if (r.warn.length) report.conflicts++
  }
  const sorted = rows.slice().sort((a, b) => (b.star ? 1 : 0) - (a.star ? 1 : 0))
  const kept = sorted.slice(0, CAP)
  if (sorted.length > CAP) report.capped.push(name + '（' + sorted.length + ' → ' + CAP + '）')

  applied.set(name, HEAD + '\n' + kept.map((r) => r.text).join('\n'))
  report.built.push({ name, teams: teams.length, kept: kept.length, star: rows.filter((r) => r.star).length })
  report.teams += teams.length
}

// 剥掉上一次我们加的段 ✓（幂等 ✓；原 wiki 文字**一字不动** ✓）
function stripOurs(t) {
  const s = String(t ?? '')
  const i = s.indexOf(HEAD)
  return i < 0 ? s.replace(/\s+$/, '') : s.slice(0, i).replace(/\s+$/, '')
}
const compose = (base, seg) => (base ? base + '\n\n' + seg : seg)

// ---------------------------------------------------------------- 自检
if (CHECK) {
  console.log('  ==== 防回潮自检（只看我们那一段 ✓，wiki 原文不归本脚本管 ✗）====')
  let bad = 0, checked = 0
  for (const c of cards) {
    const i = String(c.team ?? '').indexOf(HEAD)
    if (i < 0) continue
    checked++
    const seg = String(c.team).slice(i)
    const errs = []
    if (/https?:|miyoushe|米游社|bbs|赞\d|作者|原帖|攻略帖/.test(seg)) errs.push('带来源/署名 ✗（口径：转载可用但不署名 ✓）')
    if (/图上未标队名/.test(seg)) errs.push('占位符没清理 ✗')
    if (/^\s*·.*[\/／]/.test(seg.replace(/（[^）]*）/g, ''))) errs.push('成员名里还有没拆开的 A/B ✗')
    for (const line of seg.split('\n')) {
      if (!line.startsWith('·')) continue
      if (line.split('＋').length < 2) errs.push('成员数 < 2 的行 ✗：' + line.slice(0, 30))
    }
    if (errs.length) { bad++; console.log('   ✗ ' + c.name + '：' + errs.join('；')) }
  }
  console.log('   带我们那段（' + HEAD + '）的卡 = ' + checked + ' / ' + cards.length)
  console.log(bad ? '   ✗ ' + bad + ' 张卡有硬伤' : '   ✓ 全部通过（无署名 / 无占位符 / 无未拆备选 / 每行 ≥2 人）')
  process.exit(bad ? 1 : 0)
}

// ---------------------------------------------------------------- 写盘 / 打印
console.log('  本地已抽 ' + files.length + ' 份，本次处理 ' + report.built.length + ' 位（cap=' + CAP + ' ✓）')
if (report.empty.length) {
  console.log('  ⚠️ 图上没抽到有效队伍（' + report.empty.length + ' 位 ⇒ **不写** ✓，原来是 wiki 原文的**保持原样** ✓）：')
  console.log('     ' + report.empty.join('、'))
}
if (report.capped.length) console.log('  截断（cap 取 ★ 优先 ＋ 前几支 ✓）：' + report.capped.join('、'))
if (report.conflicts) console.log('  ⚠️ 有 ' + report.conflicts + ' 支队伍被机制卡标了冲突 ✓（照旧入库 ✓，文本带 ⚠️ 标注 ✓）')
if (report.unknownNames.size) {
  console.log('  ❓ 不在我们名单里的成员名（**请人眼过一眼** ✗）：')
  console.log('     ' + [...report.unknownNames.entries()].sort((a, b) => b[1] - a[1]).map(([n, c]) => n + '×' + c).join('、'))
}

console.log('\n  ==== 预览（前 4 位 ✓；wiki 原文省略 ✓）====')
for (const b of report.built.slice(0, 4)) {
  console.log('\n  【' + b.name + '】（图上 ' + b.teams + ' 支 ⇒ 保留 ' + b.kept + '，★ ' + b.star + '）')
  for (const line of applied.get(b.name).split('\n')) console.log('    ' + line)
}

if (DRY) {
  console.log('\n  （--dry：**没有写盘** ✓）')
} else {
  let changed = 0, added = 0
  for (const [name, seg] of applied) {
    const c = cardByName.get(name)
    if (!c) continue
    const next = compose(stripOurs(c.team), seg)
    if (c.team !== next) { c.team = next; changed++ }
    if (c.teamPick !== undefined) delete c.teamPick       // 清掉我 2026-10-15 写错字段名的残留 ✗
    if (c.teamFrom) { /* 原样保留 ✓（它描述 wiki 那半 ✓） */ } else c.teamFrom = 'baike'
    added++
  }
  // 没抽到队伍的角色：只**剥掉旧段** ✓，wiki 原文保留 ✓（2026-10-15：写错字段时误"全清"过一次 ✗）
  let stripped = 0
  for (const c of cards) {
    if (c.teamPick !== undefined) delete c.teamPick    // 残留键一律清掉 ✓（所有卡 ✓）
    if (applied.has(c.name) || !String(c.team ?? '').includes(HEAD)) continue
    if (DRY) continue
    c.team = stripOurs(c.team); stripped++
  }
  guidesDoc.teamPicksBuiltAt = new Date().toISOString().slice(0, 19)
  guidesDoc.teamPicksNote = '配队段落来自角色攻略长图识图 ⇒ 本脚本清洗后**追加**到 team ✓（wiki 原文保留 ✓）；本地 .guide.json 为来源备查 ✓；**不署名** ✗'
  // ⚠️ 原子写 ✓（2026-10-15 踩过：管道提前收工把进程掐断 ⇒ 落成 0 字节 ✗）
  const tmp = GUIDE_JSON + '.tmp'
  fs.writeFileSync(tmp, JSON.stringify(guidesDoc, null, 2) + '\n', 'utf8')
  fs.renameSync(tmp, GUIDE_JSON)
  console.log('\n  ✓ 写入 ' + changed + ' 张卡（追加段 ✓，wiki 原文一字未动 ✓）' + (stripped ? '；剥掉 ' + stripped + ' 张的旧段 ✓' : ''))
  console.log('  下一步：cd server && go run ./cmd/guideimport   ⇒ 进库 ✓')
}
