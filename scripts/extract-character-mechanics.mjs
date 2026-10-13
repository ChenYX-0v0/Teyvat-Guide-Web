// 从缓存的攻略页**自动抽取**「角色机制卡」初稿（阶段③ 铺全量，见 teyvat-docs/ai/07 TODO-20）
//
// 用法：
//   node scripts/extract-character-mechanics.mjs                       # 全部（写到 data/character_mechanics_auto.json）
//   node scripts/extract-character-mechanics.mjs --names 胡桃,钟离 --out /tmp/x.json
//   node scripts/extract-character-mechanics.mjs --explain 胡桃        # 只打印某个角色抽到哪些句（不写文件 ✓）
//
// 与 `character_mechanics.json`（人工机制卡 ✓）的关系：
//   · 人工卡 = **结构化事实**（provides / cannot / limits / needs…，每条带出处 ✓，逐条人工过 ✓）
//   · 本脚本 = **原句摘录**（按"做不到 / 限制 / 需要 / 要点"分桶 ✓，原句照抄 ✓）⇒ 落库时 review=draft ✓
//   ⚠️ 两条硬规矩：
//     ① **只摘不改写** ✗ —— 每句都是攻略页原句（保留 `evidence` 的追溯能力 ✓）；
//        摘错的风险因此被限制在"分桶放错"✓，而句子本身仍然是真的 ✓。
//     ② **绝不覆盖人工卡** ✗ —— 落库时若该角色已有 review=reviewed 的卡，跳过 ✓（见 cmd/mechanicsimport ✓）。
//
// 为什么不做"结构化 provides" ✗：那一步需要读懂全页再做归纳，规则化必然失真 ✓
//   ⇒ 自动卡只出"做不到 / 限制 / 需要 / 要点"四类**句子**✓；`provides` 结构化留给人工卡 ✓。
import fs from 'node:fs'
import path from 'node:path'

const RAW = path.resolve('scripts/cache/guides_raw')
const args = process.argv.slice(2)
const namesArg = args.includes('--names') ? args[args.indexOf('--names') + 1].split(',').map((s) => s.trim()) : null
const OUT = path.resolve(args.includes('--out') ? args[args.indexOf('--out') + 1] : 'scripts/data/character_mechanics_auto.json')
const explainArg = args.includes('--explain') ? args[args.indexOf('--explain') + 1].trim() : null

// 每类最多留几句（注入侧单卡预算 1100 字 ✓，这里留足余量 ✓）
const MAX_PER_BUCKET = 6
const MAX_SENT_CHARS = 90

// ---------------------------------------------------------------- 页面清洗（与 extract-character-guides.mjs 同源的精简版）
const stripComments = (t) => t.replace(/<!--[\s\S]*?-->/g, '')

function cleanWiki(text) {
  let t = text ?? ''
  t = t.replace(/\{\{颜色\|[^|]*\|([^{}|]*)\}\}/g, '$1')
  t = t.replace(/\{\{图标(?:\/小)?\|([^{}]*)\}\}/g, (_, seg) => {
    const parts = seg.split('|').map((s) => s.trim()).filter(Boolean)
    return (parts[1] ?? parts[0] ?? '').replace(/^\d件套$/, '')
  })
  // 表格语法（`! colspan=4 | xxx` / `|+ 表头` / `|-`）✗ —— 实测会漏进正文 ✓
  t = t.replace(/^\s*!.*$/gm, '').replace(/^\s*\|[+\-].*$/gm, '')
  t = t.replace(/\[\[[^\]|]*\|([^\]]*)\]\]/g, '$1').replace(/\[\[([^\]]*)\]\]/g, '$1')
  t = t.replace(/\{\{[^{}]*\}\}/g, '') // 剩下的模板一律去掉（不做参数展开，避免把参数行当正文 ✗）
  t = t.replace(/'''?/g, '').replace(/<[^>]*>/g, '')
  t = t.replace(/[ \t]+/g, ' ')
  return t
}

// 取某个二级标题下的正文（到下一个同级/更高级标题为止）
function section(text, title, level = 2) {
  const re = new RegExp('^={' + level + '}\\s*' + title + '\\s*={' + level + '}\\s*$', 'm')
  const m = re.exec(text)
  if (!m) return ''
  const rest = text.slice(m.index + m[0].length)
  const end = rest.search(new RegExp('^={2,' + level + '}[^=]', 'm'))
  return end === -1 ? rest : rest.slice(0, end)
}

// 章节标题清单（用于 evidenceIndex 与"只看这些章节"✓）
const SECTIONS = ['角色定位', '技能解析', '阵容搭配', '配装推荐', '命之座', '天赋', '玩法分析', '角色特点']

function sentencesOf(text) {
  return cleanWiki(stripComments(text))
    .split(/[。；\n]/)
    .map((s) => s.trim().replace(/^[|*#:!]+\s*/, '').replace(/[：:]$/, ''))
    .filter((s) => s && s.length >= 8 && s.length <= MAX_SENT_CHARS)
    .filter((s) => !/[{}[\]<>]/.test(s)) // 还有模板/链接残渣的不要 ✗
}

// ---------------------------------------------------------------- 分桶规则（顺序 = 优先级，命中即归类 ✓）
// 这些规则只看**句面词**（不读语义 ✓）⇒ 分桶可能放错 ✓，但句子本身是真的 ✓（见文件头 ①）
const BUCKETS = [
  {
    key: 'cannot',
    label: '做不到',
    // 明确"不能/无法/不适用" ✓ —— 这是用户最在意的一类（"希诺宁不能减风抗"✓）
    test: /(不能|无法|不可|不适用|不生效|不会触发|无法触发|不叠加|取最高|免疫)/,
  },
  {
    key: 'limits',
    label: '限制/前提',
    // "仅在/只能/必须/前提"这类边界 ✓
    test: /(仅(限|在|能|适用)|只能|只有在|必须|前提是|不足以|无法同时|单一|单体|对单|需要.{0,10}才能)/,
  },
  {
    key: 'needs',
    label: '需要',
    // 自身需求（充能 / 队友元素数 / 站场时间 ✓）
    test: /(需要|依赖|依靠|要求|以.{1,10}为主|绑定|配合.{1,8}角色)/,
  },
  {
    key: 'points',
    label: '要点',
    // 给队伍带来什么 / 与该角色的联动（散文式 ✓，不硬做结构化 provides ✓）
    test: /(为(队伍|全队|场上|领域内|附近).{0,10}提供|提[升高]|增[加伤]|降[低减]|回复|恢复|附魔|聚怪|保护|充能效率|元素充能)/,
  },
]

function bucketize(sentences) {
  const out = {}
  for (const b of BUCKETS) out[b.key] = []
  for (const s of sentences) {
    for (const b of BUCKETS) {
      if (b.test.test(s)) {
        if (out[b.key].length < MAX_PER_BUCKET) out[b.key].push(s)
        break
      }
    }
  }
  return out
}

// ---------------------------------------------------------------- 逐页抽取
const manifest = JSON.parse(fs.readFileSync(path.resolve('scripts/cache/guides.manifest.json'), 'utf8'))
let titles = Object.keys(manifest.pages)
if (namesArg) titles = titles.filter((t) => namesArg.some((n) => t === n + '/攻略' || t.startsWith(n + '/')))

const characters = []
const stats = { total: 0, empty: 0, buckets: { cannot: 0, limits: 0, needs: 0, points: 0 } }

for (const title of titles) {
  const file = path.join(RAW, title.replace(/[\\/:*?"<>|]/g, '_') + '.wiki')
  if (!fs.existsSync(file)) continue
  const raw = stripComments(fs.readFileSync(file, 'utf8'))
  const name = title.replace(/\/攻略$/, '').replace(/\/.*$/, '') // 旅行者/风 → 旅行者
  if (name === '旅行者') continue // 旅行者的多形态页跳过（机制差异大，留人工 ✓）

  const usedSections = []
  const sentences = []
  for (const sec of SECTIONS) {
    const body = section(raw, sec, 2)
    if (!body) continue
    const sents = sentencesOf(body)
    if (sents.length) usedSections.push(sec)
    sentences.push(...sents)
  }

  if (explainArg) {
    if (name !== explainArg) continue
    const bk = bucketize([...new Set(sentences)])
    console.log(`\n  === ${title}（章节：${usedSections.join('、') || '无'}）===`)
    for (const b of BUCKETS) {
      console.log(`  [${b.label}] ${bk[b.key].length} 句`)
      for (const s of bk[b.key]) console.log('     · ' + s)
    }
    continue
  }

  const bk = bucketize([...new Set(sentences)])
  const filled = BUCKETS.filter((b) => bk[b.key].length)
  if (!filled.length) {
    stats.empty++
    continue
  }
  for (const b of filled) stats.buckets[b.key]++

  characters.push({
    name,
    formElement: title.includes('/') && !title.endsWith('/攻略') ? title.split('/')[1] : null,
    cannot: bk.cannot,
    limits: bk.limits,
    needs: bk.needs,
    points: bk.points,
    _review: '自动抽取（初稿，未人工复核）',
    _draft: true,
    _auto: true,
    evidenceIndex: {
      page: title,
      sections: usedSections,
      note: '每句均为该页原句；本卡由 scripts/extract-character-mechanics.mjs 自动分桶，未逐条复核',
    },
  })
  stats.total++
  console.log(
    `   ${name.padEnd(10)} 做不到${bk.cannot.length} 限制${bk.limits.length} 需要${bk.needs.length} 要点${bk.points.length}（章节 ${usedSections.length} 个）`,
  )
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
      extractedAt: new Date().toISOString(),
      count: characters.length,
      source: 'biligame wiki 攻略页（自动分桶的**原句摘录**，review=draft ✓）',
      _howto: 'node scripts/extract-character-mechanics.mjs → cd server && go run ./cmd/mechanicsimport（会跳过已有人工卡的角色 ✓）',
      characters,
    },
    null,
    2,
  ),
  'utf8',
)
console.log(`\n  共 ${characters.length} 位（${stats.empty} 位没抽到机制句 ⇒ 不生成 ✓）`)
console.log(`  覆盖：做不到 ${stats.buckets.cannot} 位 / 限制 ${stats.buckets.limits} / 需要 ${stats.buckets.needs} / 要点 ${stats.buckets.points}`)
console.log(`  ⇒ ${OUT}`)
