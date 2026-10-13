// 从外部缓存页里抽出「毕业面板」的**指标：数值**行（阶段③ 补 empty 毕业线，见 ai/07 TODO-18 ✓）
//
//   node scripts/extract-graduation-external.mjs            # 读 scripts/cache/external/*.html
//   node scripts/extract-graduation-external.mjs --explain 行秋   # 只打印某位的解析过程（不写盘 ✓）
//
// 口径：
//   · **只取数字行，不搬文章** ✓：输出形如「攻击力：1700；元素充能效率：180%；暴击率：60%；暴击伤害：160%」✓
//   · **不署名** ✓：来源 URL 只写进 JSON/DB 备查 ✓，绝不进注入文本 ✗（主服务渲染时也不带 ✓）
//   · 解析只在"毕业面板"小节内进行 ✓（避免把圣遗物/武器小节的数字混进来 ✗）
import fs from 'node:fs'
import path from 'node:path'

const CACHE = path.resolve('scripts/cache/external')
const MANIFEST = path.join(CACHE, 'external.manifest.json')
const OUT = path.resolve('scripts/data/character_graduation_external.json')

const args = process.argv.slice(2)
const explainArg = args.includes('--explain') ? args[args.indexOf('--explain') + 1].trim() : null

// ---------------------------------------------------------------- 清洗
function cleanBody(html) {
  return html
    .replace(/<script[\s\S]*?<\/script>/gi, ' ')
    .replace(/<style[\s\S]*?<\/style>/gi, ' ')
    .replace(/<!--[\s\S]*?-->/g, ' ')
    .replace(/<br\s*\/?>/gi, '\n')
    .replace(/<\/(p|div|li|h\d|tr|td|th)>/gi, '\n')
    .replace(/<[^>]+>/g, ' ')
    .replace(/&nbsp;/g, ' ')
    .replace(/&amp;/g, '&')
    // HTML 实体要解码 ✓：实测班尼特页写成「元素充能效率：&ge;220%；生命值：&ge;30000」✗
    .replace(/&ge;/gi, '≥')
    .replace(/&le;/gi, '≤')
    .replace(/&gt;/gi, '>')
    .replace(/&lt;/gi, '<')
    .replace(/&#(\d+);/g, (_, d) => String.fromCodePoint(Number(d)))
    .replace(/[ \t\u3000]+/g, ' ')
    .split('\n')
    .map((l) => l.trim())
    .filter(Boolean)
    .join('\n')
}

// 指标名归一化（各站写法不一 ✓）：先认长名，再认简写 ✓
const INDICATORS = [
  ['暴击伤害', /^(暴击伤害|爆伤|暴伤)$/],
  ['暴击率', /^(暴击率|暴击|暴率)$/],
  ['元素充能效率', /^(元素充能效率|元素充能|充能效率|充能)$/],
  ['元素精通', /^(元素精通|精通)$/],
  ['生命值', /^(生命值上限|生命值|生命)$/],
  ['防御力', /^(防御力|防御)$/],
  ['攻击力', /^(攻击力|攻击)$/],
  ['治疗加成', /^(治疗加成)$/],
  ['元素伤害加成', /^(元素伤害加成|元素伤|伤害加成)$/],
]
const normalize = (label) => INDICATORS.find(([, re]) => re.test(label))?.[0] ?? ''

// 值：允许 1700 / 180% / 2000-2500+ / 60/130 / 3.5万 / 5.5W 之类 ✓（只留原样文本 ✓，不换算 ✗）
const VALUE_RE = /^[（(]?[0-9][0-9.,%\-~/～+万Ww\s]*[+）)]?$/

// 行内拆："攻击： 1700" / "攻击力 | 1100" / "暴击60%（西风剑）" ✓
function parseLine(line) {
  const m = /^([\u4e00-\u9fa5]{2,6})\s*[:：|｜]\s*(.+)$/.exec(line)
  if (!m) return null
  const key = normalize(m[1])
  if (!key) return null
  let value = m[2].replace(/\s+/g, ' ').trim()
  if (!/\d/.test(value)) return null
  if (value.length > 40) value = value.slice(0, 40)
  return { key, value }
}

// 散文式面板（实测久岐忍/部分四星页这么写 ✗）：
//   「毕业面板为：攻击力1100+、生命值2.7W+、元素精通780+，E技能半血治疗效果在4000以上。」
//   ⇒ 指标名后面**紧跟数值**也要能认出来（严格解析拿不到时才用这条 ✓）
const IND_ALT =
  '暴击伤害|爆伤|暴伤|暴击率|暴击|暴率|元素充能效率|元素充能|充能效率|充能|元素精通|精通|生命值上限|生命值|生命|防御力|防御|攻击力|攻击|治疗加成|元素伤害加成'
const PROSE_RE = new RegExp(`(${IND_ALT})\\s*([0-9][0-9.,%\\-~/～+万Ww]*)`, 'g')

function parseProse(text, limit = 8) {
  const out = []
  const seen = new Set()
  let m
  PROSE_RE.lastIndex = 0
  while ((m = PROSE_RE.exec(text)) !== null) {
    const key = normalize(m[1])
    if (!key) continue
    const value = m[2].replace(/\s+/g, '').slice(0, 24)
    const sig = key + value
    if (seen.has(sig)) continue
    seen.add(sig)
    out.push({ key, value })
    if (out.length >= limit) break
  }
  return out
}

// ---------------------------------------------------------------- 逐页解析
if (!fs.existsSync(MANIFEST)) {
  console.error('  没有 manifest：先跑 node scripts/fetch-graduation-external.mjs')
  process.exit(1)
}
const manifest = JSON.parse(fs.readFileSync(MANIFEST, 'utf8'))

const characters = []
const skipped = []
const stats = { pages: 0, withPanels: 0, lines: 0 }

for (const [id, meta] of Object.entries(manifest.pages ?? {})) {
  const file = path.join(CACHE, id + '.html')
  if (!fs.existsSync(file)) continue
  stats.pages++
  const body = cleanBody(fs.readFileSync(file, 'utf8'))
  const lines = body.split('\n').map((l) => l.trim()).filter(Boolean)

  if (explainArg && meta.name !== explainArg) continue

  // 找面板小节：标题行可能是「毕业面板」也可能是「面板数据」✓（实测香菱那页写的是"面板数据" ✗）
  //   标题串「原神行秋毕业面板-…」不会命中 ✓（它不以这些词开头 ✓）
  const start = lines.findIndex((l) => /^(一、)?\s*(毕业面板|面板数据|毕业属性|面板参考|参考面板)/.test(l))
  const stopRe = /^(二、|三、|四、|五、|圣遗物词条|词条推荐|武器推荐|天赋|阵容|配队|以上就是|总结|小编)/
  const block = []
  if (start >= 0) {
    for (let i = start + 1; i < lines.length && block.length < 30; i++) {
      if (stopRe.test(lines[i])) break
      block.push(lines[i])
    }
  }

  // 角色名优先从页面标题取 ✓（`原神行秋毕业面板-…` ⇒ 行秋）
  //   ⚠️ 早期的 `--extra` 抓取在 manifest 里名字是"(额外)" ✗，标题才是最稳的 ✓
  //
  // ⚠️ 2026-10-14：**必须由标题确认这是"某角色的毕业面板"页** ✗ ——
  //   一跳扩展会抓来一堆非角色页（标题是"圣遗物 / 武器 / 突破材料 / 配队 / 立绘"✓），
  //   它们的 manifest.name 只是一篇普通文章名 ✗ ⇒ 放进来会产出张冠李戴的数值 ✗✗
  const titleName = /^原神([\u4e00-\u9fa5]{2,4})毕业面板/.exec(meta.title ?? '')?.[1]
  if (!titleName) {
    skipped.push(meta.name + '(非角色页)')
    continue
  }
  const charName = titleName

  let panels = []
  for (const line of block) {
    const hit = parseLine(line)
    if (hit) panels.push(hit)
  }
  // 严格解析拿不到 ⇒ 退到**散文式**（"攻击力1100+、生命值2.7W+" ✓）
  //   ⚠️ 逐行试、**取第一行有结果的那行** ✓ —— 有的页按"多套配装"分行（久岐忍 4 套 ✗），
  //   全合并会把 4 套数值混成一张卡 ✓（8 条里 3 条是别的配装 ✗）⇒ 只留第一套 ✓
  if (!panels.length) {
    for (const line of block) {
      const hit = parseProse(line)
      if (hit.length) {
        panels = hit
        break
      }
    }
  }
  stats.lines += panels.length

  if (explainArg) {
    if (charName !== explainArg) continue
    console.log(`\n  === ${charName}（${id}）===`)
    console.log(`  标题行命中：${start >= 0 ? '是' : '否 ✗'}`)
    console.log('  小节前 12 行：')
    for (const l of block.slice(0, 12)) console.log('    · ' + l.slice(0, 90))
    console.log(`  解析出 ${panels.length} 条：` + panels.map((p) => `${p.key}：${p.value}`).join('；'))
    continue
  }

  if (!panels.length) {
    skipped.push(charName)
    continue
  }
  stats.withPanels++
  characters.push({
    name: charName,
    text: panels.map((p) => `${p.key}：${p.value}`).join('；'),
    sourceTitle: meta.title ?? '',
    sourceUrl: meta.url ?? '',
    fetchedAt: meta.fetchedAt ?? '',
  })
  console.log(`   ${String(charName).padEnd(6)} ${panels.length} 条：${panels.map((p) => `${p.key}${p.value.replace(/\s/g, '')}`).join(' ').slice(0, 96)}`)
}

if (explainArg) {
  console.log('\n  （--explain 只读 ✓：不写文件 ✓）')
  process.exit(0)
}

fs.mkdirSync(path.dirname(OUT), { recursive: true })
fs.writeFileSync(
  OUT,
  JSON.stringify(
    {
      _meta: {
        what: '外部「毕业面板」数值参考（社区转载汇总，**不署名** ✓）',
        role: '只用于**填补** wiki 攻略页里没有达标线的角色 ✓（wiki 有值时优先用 wiki ✓）',
        source: '游侠手游《原神 XX 毕业面板数据一览》系列（转载汇总 ✓；数值为社区参考值，非官方 ✓）',
        extractedAt: new Date().toISOString(),
        pages: stats.pages,
        covered: characters.length,
        parsedLines: stats.lines,
      },
      characters,
    },
    null,
    2,
  ),
  'utf8',
)
console.log(`\n  解析 ${stats.pages} 页 ⇒ 有面板的 ${stats.withPanels} 位（共 ${stats.lines} 行数值）`)
if (skipped.length) console.log(`  没有面板小节的 ${skipped.length} 位：${skipped.join('、')}`)
console.log(`  ⇒ ${OUT}`)
