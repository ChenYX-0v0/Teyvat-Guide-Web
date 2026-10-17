#!/usr/bin/env node
/**
 * 把识图抽到的「配队」**清洗后**并进角色攻略卡的 **`team`** 字段（2026-10-15 用户口径 ✓）
 *
 * 用户决定 ✓：「配队要**先清洗再入库**（拆 A/B 备选 ＋ 用机制卡交叉校验 ＋ 加防回潮测试）」✓；
 *   并对「跨角色回填」要求**先看样例** ✓（钟离 / 班尼特 / 香菱 ✓）⇒ 带 `--dry` 就能只看不写 ✓。
 *
 * 输入：`scripts/cache/baike_teams/<角色>.guide.json` ✓（抽取脚本只落本地 ✗，从不直接写卡 ✓）
 * 输出：`scripts/data/character_guides.json` 的 **`team`** 字段 ✓（**只动这一列** ✗）
 *      ⇒ 之后照旧 `cd server && go run ./cmd/guideimport` 进库 ✓（本脚本不连数据库 ✓）
 *
 * ⚠️⚠️ **为什么是"追加"而不是"覆盖"** ✗（2026-10-15 差点犯的错 ✓）：
 *   攻略卡的 `team` **本来就有内容** ✓ —— 121 张卡是从 biligame wiki 抓的**散文式配队思路** ✓
 *   （`teamFrom=page/reverse` ✓）。我一开始查错了字段名（查不存在的 `teamPick` ✗）才误以为"全空"✗
 *   ⇒ 若照那结论**覆盖**，就是把 wiki 原文删掉 ✗。现在只**追加我们两段** ✓：
 *   `【常见配队（图片攻略口径）】`（本角色自己的页 ✓）＋ `【其他角色页里含本角色的配队】`（回填 ✓）
 *   ⇒ 重跑时按小标题**剥掉旧段** ✓（幂等 ✓），wiki 文字**一字不动** ✓。
 *
 * ⭐ **跨角色回填**（TODO-29 的原设计 ✓「很多新角色的最佳队友仍是老角色 ✓」）：
 *   老角色（钟离/班尼特/香菱…✓）自己的配队帖常常搜不到 ✗ ⇒ 就扫**所有**角色的抽取结果 ✓，
 *   把"**含他**"的队伍补给他 ✓（纯本地计算 ✓，**零 API 成本** ✓）。
 *   排序 = 「队伍里**最新成员**的 id」降序 ✓（`character_ids.json` ✓，id 越大越新 ✓）；
 *   本角色自己的队**永远排前面** ✓（更权威 ✓）；最多回填 `--backfill-cap`（默认 3 ✓）支 ✓。
 *
 * 清洗工序（每条都对得上一次踩坑 ✓）：
 *   ① **拆 A/B 备选** ✓（图上「万叶/菈乌玛」「2号位备选：申鹤」✗ ⇒ 主选 ＋「备选：X」✓）
 *   ② **位置 / 职能词不算人名** ✗（`增益生存副C`、`4号火系`、单字 `班` 一律丢 ✓）
 *   ③ **别名归一** ✓（`万叶→枫原万叶`、`神子→八重神子` ✓；唯一事实源 = character_aliases.json ✓）
 *   ④ **机制卡交叉校验** ✓（主角 conflicts 点到的人出现在队里 ⇒ ⚠️ 标注 ✓；现只有 9 位有机制卡 ✓）
 *   ⑤ **不署名** ✓（用户 2026-10-14 口径 ✓）：团队文本里不许出现平台名 / URL / 作者 / 点赞数 ✓
 *   ⑥ 有名队 ≥2 人 ✓、未标名队 ≥3 人 ✓（后者读得最脏 ✗）；本角色自己最多 `--cap` 支（默认 5 ✓）
 *
 * 用法：
 *   node scripts/build-team-picks.mjs --names 钟离,班尼特,香菱 --dry   # 看回填样例 ✓
 *   node scripts/build-team-picks.mjs                                  # 写盘（追加 ✓ 幂等 ✓）
 *   node scripts/build-team-picks.mjs --no-backfill                    # 关掉回填 ✓
 *   node scripts/build-team-picks.mjs --check                          # 防回潮自检 ✓（有硬伤 exit 1 ✓）
 */

import fs from 'node:fs'
import path from 'node:path'

const ROOT = path.resolve(import.meta.dirname, '..')
const GUIDE_JSON = path.join(ROOT, 'scripts/data/character_guides.json')
const CACHE = path.join(ROOT, 'scripts/cache/baike_teams')
const MECH_JSON = path.join(ROOT, 'scripts/data/character_mechanics.json')
const ALIAS_JSON = path.join(ROOT, 'scripts/data/character_aliases.json')
const IDS_JSON = path.join(ROOT, 'scripts/data/character_ids.json')

// 我们自己那两段的**固定小标题** ✓（幂等靠它认边界 ✓；文案里**不带平台名** ✗ —— 不署名 ✓）
const HEAD_OWN = '【常见配队（图片攻略口径）】'
const HEAD_BACK = '【其他角色页里含本角色的配队】'
const HEADS = [HEAD_OWN, HEAD_BACK]

const arg = (k, d) => {
  const i = process.argv.indexOf('--' + k)
  if (i < 0) return d
  const v = process.argv[i + 1]
  return v && !v.startsWith('--') ? v : true
}
const DRY = !!arg('dry', false)
const CHECK = !!arg('check', false)
const BACKFILL = !arg('no-backfill', false)
const CAP = Number(arg('cap', 5))
const BCAP = Number(arg('backfill-cap', 3))
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

const ids = {}
try {
  const j = JSON.parse(fs.readFileSync(IDS_JSON, 'utf8'))
  Object.assign(ids, j.ids || {})
} catch { /* 拿不到 id 表就不排序（保持原顺序 ✓） */ }

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
const validTeam = (t) => {
  const n = parseMembers(t).length
  if (n > MAX_MEM) { dropped.tooMany++; return false }     // 矩阵型表格 ✗ ⇒ 丢 ✓
  return n >= (UNNAMED.test(String(t?.name ?? '').trim()) ? 3 : 2)
}
const membersOf = (team) => parseMembers(team).map((m) => m.main)
// ⚠️ **人数上限** ✓ —— 一支队最多 4 人（＋偶尔 1 个替补槽 ✓）⇒ 超过 5 人**说明那不是一支队** ✗
//   实测 2026-10-15：一图流的"矩阵型"表格被读成「烈绽放+卡绽」**15 人** ✗（卡维＋夜兰＋行秋＋托马＋香菱＋…）
//   ⇒ 那种一律**丢掉** ✗（宁可少 ✗，不能把矩阵当配队喂给模型 ✓）
const MAX_MEM = Number(arg('max-members', 5))
const dropped = { tooMany: 0, capped: [] }

// ⚠️ 2026-10-17 **四人口径**（用户报障 ✓）：「玛薇卡 ＋ 希诺宁 ＋ 班尼特 ＋ 茜特菈莉 ＋ 芙宁娜」**五个人** ✗
//   根因 = 识图把"第 4 位"的并列人选读成了第 5 个成员 ✗（`MAX_MEM=5` 又刚好放行 ✓）；
//   全库实测 17 行 / 14 张卡中招 ✓（砂糖超激化、钟离自由位、早柚前置挂草位…✓）
//   ⇒ 渲染层**硬性截到 4 人** ✓：多出来的人只进报告（数据留在 cache 供判定 ✓，不进正文 ✗）；
//   若被截掉的正好是本角色（回填时常见 ✗），用本角色顶掉第 4 位 ✓ ——
//   别搞出「早柚的配队里没有早柚」✗
const MEM_CAP = 4
function capMembers(viewer, mem) {
  if (mem.length <= MEM_CAP) return { mem, dropped: [] }
  const keep = mem.slice(0, MEM_CAP)
  if (!keep.some((m) => m.main === viewer)) {
    const v = mem.find((m) => m.main === viewer)
    if (v) keep[MEM_CAP - 1] = v
  }
  return { mem: keep, dropped: mem.filter((m) => !keep.includes(m)).map((m) => m.main) }
}

function teamLine(viewer, team) {
  const raw = String(team.name ?? '').trim()
  const un = UNNAMED.exec(raw)
  const label = un ? '未标名队伍 ' + un[1] : raw
  const star = (team.seen || 0) >= 2 ? '★' : ''      // ⚠️ 实测 seen 几乎全是 1 ✗ ⇒ 实际很少打星 ✓

  const parsed = parseMembers(team)
  const unknown = parsed.filter((m) => !roster.has(m.main)).map((m) => m.main)
  const capped = capMembers(viewer, parsed)
  const mem = capped.mem
  if (capped.dropped.length) dropped.capped.push(viewer + '「' + label + '」丢 ' + capped.dropped.join('、'))
  const warn = []
  // ⚠️ 机制卡冲突判定：**必须跳过角色自己** ✗ —— 踩过的坑 ✓：班尼特的 conflicts 文本里
  //   本来就写着"班尼特"三个字（"而班尼特的治疗条件与频率不匹配"✗）⇒ 不排除自己就**全队误报** ✗
  const ct = conflictText.get(viewer)
  if (ct) for (const m of mem) {
    if (m.main === viewer) continue
    if (ct.includes(m.main) || (m.alts || []).some((a) => a !== viewer && ct.includes(a))) warn.push(m.main)
  }

  const body = mem.map((m) => {
    const alt = m.alts.length ? '；备选：' + m.alts.join('、') : ''
    const role = m.role ? m.role + alt : alt.replace(/^；/, '')
    return m.main + (role ? '（' + role + '）' : '')
  }).join(' ＋ ')

  // ⚠️ 2026-10-17 **口径反转** ✓：机制卡冲突**只进日志与报告** ✓，**不写进正文** ✗ ——
  //   旧版把「⚠️ 与机制卡冲突：XXX」拼进 team 文本 ✗，模型照读 ⇒ 回答里反复强调"这样配不合适"✗
  //   （用户要求：不适合的组合**不提就好** ✓；数据保留用于判定 ✓，只是不在输出里念 ✗）。
  return {
    text: '· ' + label + (star ? ' ' + star : '') + '：' + body,
    star: !!star, warn, unknown,
  }
}

// ---------------------------------------------------------------- 全量索引（供回填 ✓）
const cacheByName = new Map()
for (const f of fs.readdirSync(CACHE).filter((x) => x.endsWith('.guide.json'))) {
  const g = JSON.parse(fs.readFileSync(CACHE + '/' + f, 'utf8'))
  if (g?.name) cacheByName.set(g.name, g)
}
// 每支队伍摊平成一条：{ page 出自哪页, team, members, newest 队里最新成员 id ✓ }
const allTeams = []
for (const [page, g] of cacheByName) {
  for (const t of g.teams || []) {
    if (!validTeam(t)) continue
    const mem = membersOf(t)
    const newest = Math.max(0, ...mem.map((n) => Number(ids[n]) || 0))
    allTeams.push({ page, team: t, members: mem, newest })
  }
}
const sigOf = (team) => membersOf(team).slice().sort().join('|')

// ---------------------------------------------------------------- 主流程
const report = { built: [], empty: [], unknownNames: new Map(), conflicts: 0, own: 0, back: 0, capped: [] }
const applied = new Map()      // name → 我们那两段的文本 ✓

const targets = cards.filter((c) => !NAMES || NAMES.includes(c.name))
for (const c of targets) {
  const name = c.name
  const g = cacheByName.get(name)
  const own = (g?.teams || []).filter(validTeam)
  const ownRows = own.map((t) => teamLine(name, t))

  let backRows = []
  if (BACKFILL) {
    const ownSigs = new Set(own.map(sigOf))
    backRows = allTeams
      .filter((x) => x.page !== name && x.members.includes(name) && !ownSigs.has(sigOf(x.team)))
      .sort((a, b) => b.newest - a.newest)      // 队里最新成员 id 降序 ✓（TODO-29 的设计 ✓）
      .map((x) => teamLine(name, x.team))
  }

  if (!ownRows.length && !backRows.length) { report.empty.push(name); continue }

  for (const r of [...ownRows, ...backRows]) {
    for (const u of r.unknown) report.unknownNames.set(u, (report.unknownNames.get(u) || 0) + 1)
    if (r.warn.length) report.conflicts++
  }
  const ownSorted = ownRows.slice().sort((a, b) => (b.star ? 1 : 0) - (a.star ? 1 : 0)).slice(0, CAP)
  const backSorted = backRows.slice(0, BCAP)
  if (ownRows.length > CAP) report.capped.push(name + '（自有 ' + ownRows.length + ' → ' + CAP + '）')

  // ⚠️ 只在**真有内容**时才写标题 ✗（本角色自己 0 支时留个空标题既难看又费 token ✓）
  const parts = []
  if (ownSorted.length) parts.push(HEAD_OWN + '\n' + ownSorted.map((r) => r.text).join('\n'))
  if (backSorted.length) parts.push(HEAD_BACK + '\n' + backSorted.map((r) => r.text).join('\n'))
  const seg = parts.join('\n')
  applied.set(name, seg)
  report.own += ownSorted.length
  report.back += backSorted.length
  report.built.push({ name, own: ownSorted.length, back: backSorted.length, star: ownRows.filter((r) => r.star).length })
}

// 剥掉上一次我们加的段 ✓（幂等 ✓；wiki 原文**一字不动** ✓）
function stripOurs(t) {
  const s = String(t ?? '')
  const idx = HEADS.map((h) => s.indexOf(h)).filter((i) => i >= 0)
  return idx.length ? s.slice(0, Math.min(...idx)).replace(/\s+$/, '') : s.replace(/\s+$/, '')
}
const compose = (base, seg) => (base ? base + '\n\n' + seg : seg)

// ---------------------------------------------------------------- 自检
if (CHECK) {
  console.log('  ==== 防回潮自检（只看我们那两段 ✓，wiki 原文不归本脚本管 ✗）====')
  let bad = 0, checked = 0
  for (const c of cards) {
    const s = String(c.team ?? '')
    const idx = HEADS.map((h) => s.indexOf(h)).filter((i) => i >= 0)
    if (!idx.length) continue
    checked++
    const seg = s.slice(Math.min(...idx))
    const errs = []
    if (/https?:|miyoushe|米游社|bbs|赞\d|作者|原帖|攻略帖/.test(seg)) errs.push('带来源/署名 ✗（口径：转载可用但不署名 ✓）')
    if (/图上未标队名/.test(seg)) errs.push('占位符没清理 ✗')
    if (/^\s*·.*[\/／]/.test(seg.replace(/（[^）]*）/g, ''))) errs.push('成员名里还有没拆开的 A/B ✗')
    for (const line of seg.split('\n')) {
      if (!line.startsWith('·')) continue
      if (line.split('＋').length < 2) errs.push('成员数 < 2 的行 ✗：' + line.slice(0, 30))
      // ⚠️ 2026-10-17 加 ✓：一支队只有 4 个位置 ✓ ⇒ 第 5 个人一律算硬伤 ✗（实测曾出现 17 行 ✓）
      if (line.split('＋').length > 4) errs.push('超过 4 人 ✗：' + line.slice(0, 40))
    }
    if (errs.length) { bad++; console.log('   ✗ ' + c.name + '：' + errs.join('；')) }
  }
  console.log('   带我们段落（含回填 ✓）的卡 = ' + checked + ' / ' + cards.length)
  console.log(bad ? '   ✗ ' + bad + ' 张卡有硬伤' : '   ✓ 全部通过（无署名 / 无占位符 / 无未拆备选 / 每行 2~4 人）')
  process.exit(bad ? 1 : 0)
}

// ---------------------------------------------------------------- 打印
console.log('  本地已抽 ' + cacheByName.size + ' 份；本次看 ' + targets.length + ' 位（cap=' + CAP + '，回填 ' + (BACKFILL ? '开 ≤' + BCAP + ' ✓' : '关 ✗') + '）')
console.log('  全量队伍池 = ' + allTeams.length + ' 支 ✓')
if (report.empty.length) console.log('  ⚠️ 自有与回填都没分到队（' + report.empty.length + ' 位 ⇒ 不写 ✓，wiki 原文保持原样 ✓）：' + report.empty.join('、'))
if (report.capped.length) console.log('  截断：' + report.capped.join('、'))
if (report.conflicts) console.log('  ⚠️ ' + report.conflicts + ' 支被机制卡标冲突 ✓（只进本报告 ✓，不进正文 ✗）')
if (dropped.capped.length) console.log('  ⚠️ ' + dropped.capped.length + ' 支超过 4 人 ⇒ 已截到 4 人 ✓（多出来的人只进本报告 ✓）：' + dropped.capped.join('；'))
if (report.unknownNames.size) console.log('  ❓ 不在名单里的成员名（请人眼过 ✗）：' + [...report.unknownNames.entries()].sort((a, b) => b[1] - a[1]).map(([n, c2]) => n + '×' + c2).join('、'))

console.log('\n  ==== 预览（' + Math.min(6, report.built.length) + ' 位 ✓；wiki 原文省略 ✓）====')
for (const b of report.built.slice(0, 6)) {
  console.log('\n  【' + b.name + '】（自有 ' + b.own + ' 支 ＋ 回填 ' + b.back + ' 支 ✓）')
  for (const line of applied.get(b.name).split('\n')) console.log('    ' + line)
}

if (DRY) {
  console.log('\n  （--dry：**没有写盘** ✓）')
} else {
  let changed = 0
  for (const c of cards) {
    if (c.teamPick !== undefined) delete c.teamPick        // 清掉 2026-10-15 写错字段名的残留 ✗
    const seg = applied.get(c.name)
    const base = stripOurs(c.team)
    if (!seg) continue                                     // 没分到队 ⇒ 只保留剥掉旧段后的 wiki 原文 ✓
    const next = compose(base, seg)
    if (c.team !== next) { c.team = next; changed++ }
    if (!c.teamFrom) c.teamFrom = 'baike'
  }
  guidesDoc.teamPicksBuiltAt = new Date().toISOString().slice(0, 19)
  guidesDoc.teamPicksNote = '配队段落来自角色攻略长图识图 ⇒ 本脚本清洗后**追加**到 team ✓（wiki 原文保留 ✓）；含跨角色回填段 ✓；本地 .guide.json 为来源备查 ✓；**不署名** ✗'
  // ⚠️ 原子写 ✓（2026-10-15 踩过：管道提前收工把进程掐断 ⇒ 落成 0 字节 ✗）
  const tmp = GUIDE_JSON + '.tmp'
  fs.writeFileSync(tmp, JSON.stringify(guidesDoc, null, 2) + '\n', 'utf8')
  fs.renameSync(tmp, GUIDE_JSON)
  console.log('\n  ✓ 写入 ' + changed + ' 张卡（自有 ' + report.own + ' 支 ＋ 回填 ' + report.back + ' 支 ✓；wiki 原文一字未动 ✓）')
  console.log('  下一步：cd server && go run ./cmd/guideimport   ⇒ 进库 ✓')
}
