// 用模型从缓存的攻略页**抽取**角色机制卡初稿（阶段③ 铺全量，见 teyvat-docs/ai/07 TODO-20）
//
//   node scripts/extract-character-mechanics-llm.mjs                # 全部（可重复跑：已抽过的跳过 ✓）
//   node scripts/extract-character-mechanics-llm.mjs --limit 5      # 先抽 5 位试水
//   node scripts/extract-character-mechanics-llm.mjs --names 胡桃,钟离 --force
//   node scripts/extract-character-mechanics-llm.mjs --dry 胡桃     # 只打印提示词与返回（不写盘 ✓）
//
// ⚠️ 为什么敢用模型抽 ✗：**每一条都必须带 `quote`（逐字原文）** ✓，脚本会把 quote 拿回页面里
//   做**子串校验** ✓，校验不过的条目**直接丢弃** ✓（并计数 ✓）。⇒ 模型只能"搬运"，编出来的
//   内容进不了库 ✓✓。分桶/归类可能有偏差，但句子是真的 ✓（库里落成 review=draft ✓，不覆盖人工卡 ✓）。
//
// 数据流：scripts/cache/guides_raw/*.wiki → 本脚本 → scripts/data/character_mechanics_auto.json
//         → cd server && go run ./cmd/mechanicsimport（跳过已有人工卡的角色 ✓）
import fs from 'node:fs'
import path from 'node:path'

const RAW = path.resolve('scripts/cache/guides_raw')
const OUT = path.resolve('scripts/data/character_mechanics_auto.json')
// ⚠️ args 必须在最前 ✗（下面 ENV_FILE 就要用；const 有 TDZ，写后面会直接报错 ✓）
const args = process.argv.slice(2)

// 取 --flag value（找不到返回 null ✓）
function args6(flag) {
  const i = args.indexOf(flag)
  return i === -1 ? null : args[i + 1]
}

const ENV_FILE = path.resolve(args6('--env') ?? '../AI派蒙服务器/ai-service/.env')

const namesArg = args6('--names') ? args6('--names').split(',').map((s) => s.trim()) : null
const limit = args6('--limit') ? Number(args6('--limit')) : 0
const force = args.includes('--force')
// --merge：只给「四个边界字段还空着」的角色补抽 ✓，并与已有条目**求并集** ✓（不动 provides/limits/points ✗）
//   2026-10-14 加（TODO-22 的批量修法 ✓）：此前 schema 里没有 conflicts / scaling ⇒ 114 / 116 张全空 ✗
const merge = args.includes('--merge')
// --wide：摘录用**整页**（截断 12000 字 ✓），而不是只取机制/阵容章节 ✓ —— 补早期角色的缺口用 ✓
const wide = args.includes('--wide')
const dry = args6('--dry')
const CONCURRENCY = Number(args6('--concurrency') ?? 3)

// ---------------------------------------------------------------- 模型配置（从 AI 服务的 .env 读，不落任何密钥到日志 ✓）
function loadLLMConfig() {
  const txt = fs.readFileSync(ENV_FILE, 'utf8')
  const get = (k) => {
    const m = new RegExp('^' + k + '=(.*)$', 'm').exec(txt)
    return m ? m[1].trim().replace(/^["']|["']$/g, '') : ''
  }
  const key = get('DEEPSEEK_API_KEY')
  if (!key) throw new Error('DEEPSEEK_API_KEY 为空：' + ENV_FILE)
  return {
    key,
    base: (get('DEEPSEEK_BASE_URL') || 'https://api.deepseek.com').replace(/\/$/, ''),
    model: get('LLM_MODEL') || 'deepseek-flash',
  }
}

// ---------------------------------------------------------------- 页面清洗（与 extract-character-guides.mjs 同源的精简版）
const stripComments = (t) => t.replace(/<!--[\s\S]*?-->/g, '')

function cleanWiki(text) {
  let t = text ?? ''
  t = t.replace(/\{\{颜色\|[^|]*\|([^{}|]*)\}\}/g, '$1')
  t = t.replace(/\{\{图标(?:\/小)?\|([^{}]*)\}\}/g, (_, seg) => {
    const parts = seg.split('|').map((s) => s.trim()).filter(Boolean)
    return (parts[1] ?? parts[0] ?? '').replace(/^\d件套$/, '')
  })
  t = t.replace(/^\s*!.*$/gm, '').replace(/^\s*\|[+\-].*$/gm, '')
  t = t.replace(/\[\[[^\]|]*\|([^\]]*)\]\]/g, '$1').replace(/\[\[([^\]]*)\]\]/g, '$1')
  t = t.replace(/\{\{[^{}]*\}\}/g, '')
  t = t.replace(/'''?/g, '').replace(/<[^>]*>/g, '')
  return t.replace(/[ \t]+/g, ' ').replace(/\n{3,}/g, '\n\n')
}

function section(text, title, level = 2) {
  const re = new RegExp('^={' + level + '}\\s*' + title + '\\s*={' + level + '}\\s*$', 'm')
  const m = re.exec(text)
  if (!m) return ''
  const rest = text.slice(m.index + m[0].length)
  const end = rest.search(new RegExp('^={2,' + level + '}[^=]', 'm'))
  return end === -1 ? rest : rest.slice(0, end)
}

// 参与抽取的章节（机制事实基本都在这里 ✓）；每节截断，控制单次请求体积 ✓
//
// ⚠️ 章节**分工**很关键 ✗（2026-10-13 实测踩到）：把「阵容搭配」与本人机制混在一起抽，
//   模型会把**队友**的能力写成本角色的 provides ✗（胡桃卡里出现"玉璋护盾…全队抗性降低 20%"= 钟离的 ✓）
//   ⇒ 摘录里显式分区：本人机制章节 / 阵容章节，并告诉模型后者的内容只归 points ✓
const SELF_SECTIONS = ['角色定位', '技能解析', '命之座', '玩法分析', '角色特点']
const TEAM_SECTIONS = ['阵容搭配']
const SECTION_MAX = 4000

function buildExcerpt(raw) {
  const collect = (list) =>
    list
      .map((sec) => {
        const body = cleanWiki(section(raw, sec, 2)).trim()
        return body ? `### ${sec}\n${body.slice(0, SECTION_MAX)}` : ''
      })
      .filter(Boolean)
      .join('\n\n')
  const self = collect(SELF_SECTIONS)
  const team = collect(TEAM_SECTIONS)
  const parts = []
  if (self) parts.push(`【本角色机制章节】（provides / cannot / limits / needs / scaling 只能来自这里）\n${self}`)
  if (team) parts.push(`【阵容搭配章节】（points 与 **conflicts** 可以来自这里 ✓；其中**队友**的能力不算本角色的 provides）\n${team}`)
  return parts.join('\n\n')
}

// ---------------------------------------------------------------- 放宽摘录（--wide ✓，2026-10-14 加 ✓）
//
// 为什么要它 ✗：第一轮只取「角色定位 / 技能解析 / 命之座 / 玩法分析 / 角色特点 ＋ 阵容搭配」✓，
//   结果 62 位角色仍有缺口 ✗（`conflicts` 缺 50、`cannot` 缺 37…✓），尤其早期待辅角色的"机制章节"本身就薄 ✗。
//   而**禁忌 / 数值**常常写在别处（配装思路、注意事项、命座分析…✓）⇒ 直接把**整页**给模型 ✓（截断到 12000 字 ✓）。
const WIDE_TOTAL_MAX = 12000

function buildWideExcerpt(raw) {
  const body = cleanWiki(stripComments(raw)).replace(/\n{3,}/g, '\n\n').trim()
  return `【本角色全页摘录】（provides / cannot / limits / needs / conflicts / scaling / points 都可以来自这里 ✓）\n${body.slice(0, WIDE_TOTAL_MAX)}`
}

const SYSTEM = `你是《原神》的资料整理员。任务：从给定的攻略页摘录中，提取该角色的**机制事实**，输出严格 JSON。

铁律（违反即作废）：
1. 只允许使用给定摘录里的内容；**禁止**补充任何摘录之外的常识、推测或你自己的记忆。
2. 每一条都**必须**带 quote 字段：从给定摘录里**逐字连续复制**的原文片段（12~60 字，不要改写、不要省略号、不要拼接不相邻的句子）。做不到逐字引用，就不要输出这一条。
3. 只写机制，不写结论。禁止出现"值得培养""建议抽""适合当主C的队伍""强度如何"这类评价。
4. **只写本角色自己的机制**：摘录里提到的队友（钟离的护盾、芙宁娜的增伤、班尼特的加攻…）不是本角色的能力，禁止写进 provides / limits ✗。队友与阵容相关的内容写进 points（联动要点）或 conflicts（禁忌）✓。
5. **六个类别一个都不能漏** ✗（2026-10-14 加：此前只抽 provides/limits/points ⇒ cannot/needs/conflicts/scaling 大面积空 ✗）。**多列不算错** ✓ —— 每条都要能逐字引用 ✓，所以多列不会编 ✓；**漏列才是错** ✗。确实没有对应句子时才输出空数组。
6. 不用 Markdown，纯 JSON。

**六个类别怎么认**（照下面的字面线索去摘录里找 ✓）：
· provides 给什么加成（自身或队伍 ✓，带条件/按什么算 ✓）
· limits   成立前提或边界（"前提是…""仅在…时""长按才…"✓）
· points   与队友 / 队伍的联动要点（联动、互充、共鸣、谁给谁打下手 ✓）
· **cannot**  明确**做不到 / 不适用 / 不享受** ✗ —— 字面线索：「不能」「无法」「不可」「不吃」「无效」「不生效」「不触发」「不叠加」「不适用于」「只对单人生效」「会退出」「为 0」「不产」「切人」「会浪费」✓
· **needs**   自己**或队伍必须提供什么** ✓ —— 字面线索：「需要」「必须」「依赖」「否则」「前提是」「绑定」「搭配…才能」「有一位」「要有」✓
· **conflicts** **禁忌 / 冲突 / 别这么配** ✗ —— 字面线索：「不建议」「不要」「别」「会损失」「会浪费」「仅在高抗性环境…时才算可选」「同名…不叠加」「两套体系」「硬塞」「分开用」✓（**可以来自阵容搭配章节** ✓）
  ⚠️ **别把"收益低 / 不适配 / 覆盖率低"塞进 conflicts** ✗（2026-10-14 细筛过一轮 ✓）—— 那是**收益评价** ✓，归 limits ✓；conflicts 只收"**别这么配 / 与谁冲突 / 会浪费 / 不叠加**"这类硬禁忌 ✓
· **scaling** **数值怎么算 / 阈值** ✓ —— 字面线索：「每…点」「每…%」「至多 +…%」「基于…上限」「按…的 6%」「达到上限」「吃满」「超过则不浪费/溢出」✓

输出格式（严格 JSON，不要任何多余文字）：
{
  "provides":  [{"kind": "给什么加成（短语）", "to": "给谁", "condition": "条件（可省略）", "scale": "按什么算（可省略）", "quote": "原文片段"}],
  "cannot":    [{"text": "明确做不到/不适用什么", "quote": "原文片段"}],
  "limits":    [{"text": "成立前提或边界", "quote": "原文片段"}],
  "needs":     [{"text": "自己需要什么（充能/队友/站场时间等）", "quote": "原文片段"}],
  "conflicts": [{"text": "禁忌 / 与谁冲突 / 别怎么配", "quote": "原文片段"}],
  "scaling":   [{"text": "数值算法或阈值", "quote": "原文片段"}],
  "points":    [{"text": "与队友/队伍的联动要点", "quote": "原文片段"}]
}`

// ---------------------------------------------------------------- quote 校验（防幻觉的关键 ✓）
const norm = (s) => (s ?? '').replace(/\s+/g, '').replace(/[，。；、：:（）()「」『』“”"']/g, '')
function verified(items, pageNorm, key) {
  const ok = []
  let dropped = 0
  for (const it of items ?? []) {
    const q = typeof it === 'string' ? it : it?.quote
    if (!q || norm(q).length < 8 || !pageNorm.includes(norm(q))) {
      dropped++
      continue
    }
    if (key === 'provides') {
      const kind = (it.kind ?? '').trim()
      if (!kind) {
        dropped++
        continue
      }
      ok.push({ kind, to: it.to?.trim() || undefined, condition: it.condition?.trim() || undefined, scale: it.scale?.trim() || undefined, evidence: q.trim() })
    } else {
      const text = (it.text ?? it).toString().trim()
      if (!text) {
        dropped++
        continue
      }
      ok.push({ text, evidence: q.trim() })
    }
  }
  return { ok, dropped }
}

async function callModel(cfg, excerpt, name) {
  const body = {
    model: cfg.model,
    messages: [
      { role: 'system', content: SYSTEM },
      { role: 'user', content: `角色：${name}\n\n【攻略页摘录】\n${excerpt}` },
    ],
    temperature: 0.2,
    stream: false,
    response_format: { type: 'json_object' },
    // ⚠️ 必须显式关掉思考 ✗：默认开着 ⇒ 每次 20~40 秒（实测 120 位要 40 分钟+ ✓）；
    //   与 AI 服务 json/classify 档同一口径（profiles.py 的 thinking_param ✓）
    thinking: { type: 'disabled' },
    // 3000 会把长页面的 JSON 截断 ✗（实测赛诺/优菈/桑多涅/薇斯纳 4 位报 Unterminated string ✓）
    max_tokens: 8000,
  }
  const res = await fetch(`${cfg.base}/chat/completions`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${cfg.key}` },
    body: JSON.stringify(body),
  })
  if (!res.ok) throw new Error(`HTTP ${res.status} ${(await res.text()).slice(0, 200)}`)
  const data = await res.json()
  const content = data?.choices?.[0]?.message?.content ?? ''
  const usage = data?.usage ?? {}
  return { json: JSON.parse(content), usage }
}

// ---------------------------------------------------------------- 主流程
const cfg = loadLLMConfig()
const manifest = JSON.parse(fs.readFileSync(path.resolve('scripts/cache/guides.manifest.json'), 'utf8'))
let titles = Object.keys(manifest.pages)
if (namesArg) titles = titles.filter((t) => namesArg.some((n) => t === n + '/攻略' || t.startsWith(n + '/')))

// 已有结果（支持重复跑：抽过的跳过 ✓）
let out = { _meta: {}, characters: [] }
if (fs.existsSync(OUT)) {
  try {
    const prev = JSON.parse(fs.readFileSync(OUT, 'utf8'))
    out.characters = prev.characters ?? []
  } catch {
    /* 坏了就从头来 ✓ */
  }
}
const done = new Set(out.characters.map((c) => c.name))
const save = () => {
  out._meta = {
    extractedAt: new Date().toISOString(),
    count: out.characters.length,
    source: 'biligame wiki 攻略页 → 模型抽取 + **逐字 quote 校验** ✓（每条都能在页面里原样找到 ✓）',
    review: 'review=draft：模型抽取的初稿，未经人工复核 ✓（落库时不覆盖人工卡 ✓）',
    stats: out._meta.stats ?? {},
  }
  fs.writeFileSync(OUT, JSON.stringify(out, null, 2), 'utf8')
}

const NEED_FIELDS = ['cannot', 'needs', 'conflicts', 'scaling']
const needFill = (c) => NEED_FIELDS.some((k) => !(c?.[k] ?? []).length)
const jobs = titles
  .map((title) => ({ title, name: title.replace(/\/攻略$/, '').replace(/\/.*$/, '') }))
  .filter((j) => j.name !== '旅行者')
  .filter((j) => {
    if (force) return true
    const prev = out.characters.find((c) => c.name === j.name)
    if (!prev) return true // 还没有卡 ⇒ 抽 ✓
    if (merge) return needFill(prev) // --merge：只补四个边界字段还空着的 ✓
    return false // 原口径：抽过就跳过 ✓
  })
if (limit) jobs.splice(limit)

console.log(`  模型 ${cfg.model}｜待抽 ${jobs.length} 位｜已有 ${done.size} 位｜并发 ${CONCURRENCY}`)

const stats = { dropped: 0, verified: 0, failed: 0 }
let idx = 0
async function worker() {
  while (idx < jobs.length) {
    const job = jobs[idx++]
    const file = path.join(RAW, job.title.replace(/[\\/:*?"<>|]/g, '_') + '.wiki')
    if (!fs.existsSync(file)) continue
    const pageText = cleanWiki(stripComments(fs.readFileSync(file, 'utf8')))
    const excerpt = wide
      ? buildWideExcerpt(fs.readFileSync(file, 'utf8'))
      : buildExcerpt(stripComments(fs.readFileSync(file, 'utf8')))
    if (!excerpt.trim()) {
      console.log(`   [跳过] ${job.name}（没有可用的机制章节 ✓）`)
      continue
    }
    try {
      const { json } = await callModel(cfg, excerpt, job.name)
      const pageNorm = norm(pageText)
      const buckets = {}
      let drop = 0
      for (const [key, field] of [
        ['provides', 'provides'],
        ['cannot', 'cannot'],
        ['limits', 'limits'],
        ['needs', 'needs'],
        ['conflicts', 'conflicts'],
        ['scaling', 'scaling'],
        ['points', 'points'],
      ]) {
        const { ok, dropped } = verified(json[field], pageNorm, key)
        drop += dropped
        buckets[key] = ok
      }
      const card = {
        name: job.name,
        provides: buckets.provides.map((x) => ({ kind: x.kind, ...(x.to ? { to: x.to } : {}), ...(x.condition ? { condition: x.condition } : {}), ...(x.scale ? { scale: x.scale } : {}), evidence: x.evidence })),
        cannot: buckets.cannot.map((x) => x.text),
        limits: buckets.limits.map((x) => x.text),
        needs: buckets.needs.map((x) => x.text),
        conflicts: buckets.conflicts.map((x) => x.text),
        scaling: buckets.scaling.map((x) => x.text),
        points: buckets.points.map((x) => x.text),
        _review: '模型抽取 + 逐字校验（初稿，未人工复核）',
        _draft: true,
        _auto: true,
        evidenceIndex: { page: job.title, note: '每条均带 evidence = 页面原句片段（已逐字校验 ✓）' },
      }
      const total = (b) => Object.values(b).reduce((n, v) => n + (Array.isArray(v) ? v.length : 0), 0)
      if (!total(buckets)) {
        console.log(`   [空] ${job.name}（校验后 0 条 ✓ 不落库）`)
        continue
      }
      // 同一角色重复抽时覆盖（--force ✓）；--merge 时**只并上四个边界字段** ✓
      const at = out.characters.findIndex((c) => c.name === job.name)
      if (at === -1) {
        out.characters.push(card)
      } else if (merge) {
        const prev = out.characters[at]
        let added = 0
        for (const k of NEED_FIELDS) {
          const have = new Set((prev[k] ?? []).map((x) => norm(String(x))))
          const add = (card[k] ?? []).filter((x) => !have.has(norm(String(x))))
          added += add.length
          card[k] = [...(prev[k] ?? []), ...add]
        }
        card.provides = prev.provides ?? card.provides // 保持原样 ✗（本轮只补边界 ✓）
        card.limits = prev.limits ?? card.limits
        card.points = prev.points ?? card.points
        if (added === 0) {
          console.log(`   [无补] ${job.name}（四个边界字段都没能补上 ✓）`)
          continue
        }
        card._review = '模型抽取 + 逐字校验（初稿，未人工复核）；2026-10-14 按 TODO-22 补边界字段 cannot/needs/conflicts/scaling ✓'
        out.characters[at] = card
      } else {
        out.characters[at] = card
      }
      stats.dropped += drop
      const kept = total(buckets)
      stats.verified += kept
      console.log(
        `   ${job.name.padEnd(10)} 提供${buckets.provides.length} 做不到${buckets.cannot.length} 限制${buckets.limits.length} 需要${buckets.needs.length} 禁忌${buckets.conflicts.length} 基数${buckets.scaling.length} 要点${buckets.points.length}` +
          (drop ? `（丢弃 ${drop} 条校验不过 ✗）` : ''),
      )
      out._meta.stats = stats
      save()
    } catch (e) {
      stats.failed++
      console.log(`   [失败] ${job.name}：${String(e.message).slice(0, 120)}`)
    }
  }
}

if (dry) {
  const job = { title: dry + '/攻略', name: dry }
  const file = path.join(RAW, dry + '_攻略.wiki')
  if (!fs.existsSync(file)) throw new Error('没有缓存页：' + file)
  const raw = stripComments(fs.readFileSync(file, 'utf8'))
  const excerpt = buildExcerpt(raw)
  console.log(`  --- 摘录（${excerpt.length} 字）---\n` + excerpt.slice(0, 1200) + '\n  …')
  const { json } = await callModel(cfg, excerpt, dry)
  console.log('  --- 模型返回（原始）---\n' + JSON.stringify(json, null, 1).slice(0, 2000))
  const pageNorm = norm(cleanWiki(raw))
  for (const field of ['provides', 'cannot', 'limits', 'needs', 'points']) {
    const { ok, dropped } = verified(json[field], pageNorm, field)
    console.log(`  [${field}] 通过 ${ok.length} / 丢弃 ${dropped}`)
    for (const x of ok.slice(0, 3)) console.log('     ✓ ' + JSON.stringify(x).slice(0, 140))
  }
  process.exit(0)
}

await Promise.all(Array.from({ length: CONCURRENCY }, worker))
save()
console.log(`\n  完成：${out.characters.length} 位 → ${OUT}`)
console.log(`  校验通过条目 ${stats.verified}；**被丢弃（引用对不上原文）${stats.dropped}**；失败 ${stats.failed}`)
