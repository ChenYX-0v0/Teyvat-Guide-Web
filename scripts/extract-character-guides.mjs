// 从缓存的攻略页提炼「角色攻略卡」（阶段②，见 teyvat-docs/ai/07 TODO-18）
//
// 用法：node scripts/extract-character-guides.mjs [--names 胡桃,芙宁娜] [--all] [--cap 400]
//
// 为什么**优先解析模板参数**、而不是让模型读散文再总结：
//   攻略页的关键信息本来就写在 `{{简易配装}}` / `{{角色/配装推荐/词条}}` / `{{角色/配装推荐/详表}}`
//   的参数里（沙/杯/头/副词条、技能升级推荐 **= 天赋顺序**、毕业武器/可选武器、逐条推荐理由 ✓）
//   ⇒ 直接解析 = **不引入幻觉** ✓、**不花 API 钱** ✓、**可复现** ✓（模型总结只适合"定位/配队"这类散文）
//
// 槽位（与 TODO-18 的规划一致）：
//   role 定位 ／ talents 天赋顺序 ／ relic_pick 圣遗物+主词条+副词条 ／ weapon_pick 武器
//   team_pick 配队思路 ／ graduation 毕业线 ／ notes 注意事项
// ⚠️ graduation（毕业线）：**biligame 攻略页里没有"毕业面板"小节** ✗ ⇒ 本脚本**不编**，
//   只把带有阈值性质的句子（如"实战精通 250-350 即可"）挪到 notes 里带上出处 ✓
import fs from 'node:fs'
import path from 'node:path'

const RAW = path.resolve('scripts/cache/guides_raw')
const args = process.argv.slice(2)
const cap = args.includes('--cap') ? Number(args[args.indexOf('--cap') + 1]) : 420
const namesArg = args.includes('--names') ? args[args.indexOf('--names') + 1].split(',').map((s) => s.trim()) : null
// --out：写到别的路径（改抽取规则时先写临时文件、和现网 JSON 对比 ✓，别直接覆盖 ✗）
const OUT = path.resolve(args.includes('--out') ? args[args.indexOf('--out') + 1] : 'scripts/data/character_guides.json')
// --explain 角色名：只打印这个角色**各来源**摘出的阈值句（排障用 ✓，不写文件 ✓）
const explainArg = args.includes('--explain') ? args[args.indexOf('--explain') + 1].trim() : null

// ---------------------------------------------------------------- wikitext 清洗
const stripComments = (t) => t.replace(/<!--[\s\S]*?-->/g, '')
function clean(text) {
  let t = text ?? ''
  // ① 保留语义的模板：先按"最内层"逐个换成它的文本
  //    {{颜色|火|火元素}} → 火元素 ／ {{效果|蒸发}} → 蒸发
  //    {{图标|武器|护摩之杖}} → 护摩之杖 ／ {{图标/小|圣遗物|追忆之注连|||4件套}} → 追忆之注连4件套
  //    {{图标|攻略|芙宁娜}} → 芙宁娜（⚠️ 这一类此前会被漏掉 ✗，导致"搭配、等治疗辅助"这种掉名字）
  const keepTemplates = [
    [/\{\{颜色\|[^|{}]*\|([^{}]*)\}\}/g, '$1'],
    [/\{\{效果\|([^{}]*)\}\}/g, '$1'],
    // ⚠️ 模板名不止「图标」和「图标/小」：**还有「图标/攻略」** ✗
    //   例：{{图标/攻略|芙宁娜}} —— 此前正则只允许 /小 ⇒ 它落到"其余模板"里被整块删掉
    //   ⇒ 描述里出现"在绑定**时**上限最高""搭配**、**等治疗辅助角色"这种掉名字 ✗
    //   （2026-10-12 用最小复现定位 ✓）⇒ 改成"斜杠后任意后缀"都认 ✓
    [/\{\{图标(?:\/[^|{}]*)?\|([^{}]*)\}\}/g, (_m, inner) => {
      const seg = inner.split('|').map((s) => s.trim()).filter(Boolean)
      const name = seg[1] ?? seg[0] ?? ''      // 第 2 段是名字（第 1 段是类型：武器/圣遗物/攻略…）
      const piece = seg.find((s) => /^\d件套$/.test(s))
      return name + (piece ? '·' + piece.replace('件套', '件套') : '')
    }],
  ]
  // 由内向外迭代：先反复应用"保留语义"的规则，直到不再变化
  for (let i = 0; i < 6; i++) {
    const before = t
    for (const [re, rep] of keepTemplates) t = t.replace(re, rep)
    if (t === before) break
  }
  // ② 剩下的模板整块丢掉（含 {{#info:…|note}} 这类注释模板）
  for (let i = 0; i < 6; i++) {
    const before = t
    t = t.replace(/\{\{[^{}]*\}\}/g, '')
    if (t === before) break
  }
  return t
    // ⚠️ 小节标题要**保留语义再清**（2026-10-13 加）：玛薇卡的「阵容搭配」里有
    //   `===作为辅C角色===` / `===作为站场角色===` 两个子节 ⇒ 直接删会把"这是哪种玩法"的
    //   上下文丢掉 ✗，留着又会把 `===` 原样带进散文 ✓（实测就是这么漏进去的 ✗）
    //   ⇒ 转成【作为辅C角色】这种标签 ✓
    .replace(/^={2,4}\s*(.+?)\s*={2,4}\s*$/gm, '【$1】')
    .replace(/\[\[(?:file|文件):[^\]]*\]\]/gi, '')          // [[file:火.png|24px]] → 去掉
    .replace(/\[\[[^|\]]*\|([^\]]*)\]\]/g, '$1')            // [[A|B]] → B
    .replace(/\[\[([^\]]*)\]\]/g, '$1')                     // [[A]] → A
    .replace(/'''?/g, '')
    .replace(/<br\s*\/?>/gi, '\n')
    .replace(/<[^>]+>/g, '')
    .replace(/[ \t]+/g, ' ')
    .replace(/\n{3,}/g, '\n\n')
    .trim()
}
const clip = (s, n = cap) => (s && s.length > n ? s.slice(0, n) + '…' : s)
// **按句收口**的截断（2026-10-13 加）：不要切在句子中间 ✗
//   线上实测：team 421 字正好等于旧 cap ⇒ 胡桃那张卡断在"…施放元素战技后抗打断系数"✗
//   规则：在 ≤n 的范围内找**最后一个句末标点**；若它太靠前（<一半）说明是一整句超长，才退回硬截断
const clipSent = (s, n = cap) => {
  const t = (s ?? '').trim()
  if (!t || t.length <= n) return t
  const head = t.slice(0, n)
  const cut = Math.max(
    head.lastIndexOf('。'), head.lastIndexOf('！'), head.lastIndexOf('？'), head.lastIndexOf('；'),
  )
  return cut >= Math.floor(n * 0.5) ? head.slice(0, cut + 1) : head + '…'
}
// 取模板参数值：**必须把续行也收进来** ✗
//   ⚠️ 实测（2026-10-12）：`词条推荐理由` / `技能升级推荐理由` 这类是**多行**的 ——
//   只读 `|key=` 那一行会把后半段整段丢掉（胡桃那句"实战精通能达到250-350即可"就在第 3 行 ✗），
//   连带 reasons 被截断、毕业线抽不出来 ✓
//   续行规则：往下吃到「下一个参数行 |x=」「模板结束 }}」「新模板 {{」为止
const kv = (lines, key) => {
  const i = lines.findIndex((l) => l.startsWith('|' + key + '='))
  if (i === -1) return ''
  const parts = [lines[i].slice(key.length + 2).trim()]
  for (let j = i + 1; j < lines.length; j++) {
    const l = lines[j]
    if (/^\|/.test(l) || l.startsWith('}}') || l.startsWith('{{')) break
    if (l) parts.push(l.trim())
  }
  return parts.filter(Boolean).join('\n')
}

// 从正文里摘"带阈值性质"的原句（毕业线/达标线的原句；**只摘不改写** ✓ 便于追溯）
//   先 clean ✓ —— 否则摘出来的句子里会夹着 {{颜色|火|火元素}} 这类模板 ✗
//
// ⚠️ 2026-10-13 **收紧**（用户反馈"毕业线里全是武器介绍"✗）：
//   旧规则 = 「出现 双暴/暴击率/充能/生命值… 且 出现 即可/需…」⇒ 于是**武器与圣遗物的推荐理由**
//   整段被判成阈值句 ✗（实测班尼特：「天空之刃有着较高基础攻击力的同时，副属性还提供了班尼特
//   所需要的元素充能效率」✗、玛薇卡：「岩峰巡歌…需堆高防御力…高精炼时收益明显」✗）
//   ⇒ 现在三条硬条件 + 一条排除：
//     ① 必须有**数字** ✓（毕业线本质是数值门槛；纯文字句不是 ✗）
//     ② 必须有阈值语境词 ✓
//     ③ 必须确实像数值描述 ✓（%，或 点/级/词条/倍率/双暴… ✓）
//     ④ **不是装备介绍句** ✗（提到武器/沙杯头/套装的一律不要 —— 那些归「圣遗物 / 武器」两栏 ✓）
const GEAR_WORDS = /(武器|单手剑|双手剑|长柄武器|弓|法器|锻造|精炼|纪行|限定|时之沙|空之杯|理之冠|四件套|套装效果)/
// 不是毕业线的句子（归「注意事项」✓）：命座/手法/缺点/误区这类行动或限制描述 ✗
const NOTE_WORDS = /(命座|第[一二三四五六]命|缺点|不足|误区|不建议|不推荐|注意|手法|技巧)/
// **面板指标词**：毕业线一定是在说这些指标的门槛或上限 ✓
//   （没有它就说明这句在讲机制/流程 ✗ —— 实测踩过的假阳性：希诺宁「元素爆发治疗场上角色12秒」✗、
//     玛薇卡「至少50%战意（即100点战意）时可施放元素爆发」✗、丝柯克「2~3 名火水雷冰队友」✗）
const PANEL_WORDS = /(双暴|暴击率|暴击伤害|暴伤|元素精通|精通|元素充能|充能效率|充能|生命值|防御力|攻击力|元素伤害加成|治疗加成|元素爆发伤害)/
// **机制/动作描述词**：出现这些基本可以断定不是"面板门槛" ✗（避免把技能说明当毕业线 ✓）
const MECHANIC_WORDS = /(施放|召唤|进入|退出|附着|元素微粒|概率|命座|天赋|元素战技|持续\d|秒)/
// 单句长度上限：毕业线要的是"双暴 60/120 起步"这种短句，长句多半是散文或装备介绍 ✗
const THRESHOLD_MAX_CHARS = 80

// 句子清洗：模板残渣（前导 `|` / 星号）+ **表格语法** ✗
//   实测（2026-10-13）：丝柯克的「阵容搭配」里漏出 `! colspan=4 | 元素充能效率需求` ✗
//   ⇒ 含 `colspan` / 行首 `!` / `{{` 的一律丢掉 ✓（这些不是人话 ✓）
function cleanSentence(s) {
  let t = s.trim().replace(/^\|\s*/, '').replace(/^\*+/, '').replace(/^!\s*/, '')
  if (/colspan|rowspan|^\{\{|^\}\}/.test(t)) return ''
  // 归一化源里的笔误：实测流浪者页写成「暴击率：:60%以上」✗（多一个冒号 ✓）
  t = t.replace(/[:：]\s*[:：]/g, '：')
  return t
}

function thresholdSentences(text) {
  const sents = clean(text ?? '')
    .split(/[。；\n]/)
    .map(cleanSentence)
    .filter(Boolean)
    .filter((s) => !/^[」』）)】]/.test(s)) // 残句（模板/引号被截断）✗：如芙宁娜的「」提高获取「气氛值」效率…」

  // ⚠️ 名字不能叫 clean ✗：外层就是 `clean()`（清模板），同名会把它遮住 ⇒ TDZ 报错 ✓
  const acceptable = (s) =>
    s.length <= THRESHOLD_MAX_CHARS && !GEAR_WORDS.test(s) && !NOTE_WORDS.test(s) && !MECHANIC_WORDS.test(s)
  const hasNumber = (s) => /\d/.test(s)
  // 门槛词：只认**目标/上限**口径 ✓ —— 实测（2026-10-13）放宽到"需要/建议/推荐"会把
  // 「夜兰…应优先保证充能循环的前提下提升伤害」✗「阿蕾奇诺…攻击收益只有双暴的40%~60%」✗
  // 这类**机制/稀释解释**捞进来 ⇒ 宁可为空也不放噪音 ✓
  const hasGate = (s) => /(以上|即可|至少|不低于|起步|上限|不超过|低于|大约|达到)/.test(s)

  // ① **面板表行**（首选 ✓）：攻略页「阵容搭配」里的达标线本来就写成
  //    「暴击率/暴击伤害：70%/180%以上；元素精通：200以上」这种 指标：数值 的行 ✓
  //    这才是真正的毕业线 ✓ —— 有了它就不必再去散文里捞 ✗（那正是噪音来源 ✓）
  const statLines = sents.filter(
    (s) =>
      acceptable(s) &&
      /^(（[^）]{1,24}）|【[^】]{1,24}】)?\s*(双暴|暴击率|暴击伤害|暴伤|元素精通|精通|元素充能效率|充能效率|充能|生命值|攻击力|防御力|元素伤害加成|治疗加成)\s*[:：]/.test(s) &&
      hasNumber(s),
  )
  if (statLines.length) return [...new Set(statLines)]

  // ② 没有面板表行时才退到散文门槛句 ✓（必须同时有 **面板指标** + 数值 + 门槛词 ✓）
  const hit = sents.filter((s) => acceptable(s) && hasNumber(s) && PANEL_WORDS.test(s) && hasGate(s))
  return [...new Set(hit)]
}

// 毕业线来源（**按可信度排序** ✓）：越靠前越像"数值门槛"语境；
//   圣遗物 / 武器的详表理由是**装备介绍**，只有在前面全都摘不出东西时才回退到它们 ✓
//   （这样既保住"丝柯克 / 菲林斯不能空"✓，又不再让装备介绍混进毕业线 ✗）
function graduationSources(ctx) {
  // ⚠️ 2026-10-13 删掉「圣遗物详表理由 / 武器详表理由」两个来源 ✗：
  //   它们天然是**装备介绍**（"绝缘之旗印4件套适合偏后台输出的丽莎…"✗、"对于二命及以上闲云，
  //   昔时之歌的提升等效闲云的300攻击力"✗）—— 正是用户投诉的那类噪音 ✓
  //   ⇒ 宁可这几张卡毕业线为空 ✓，也不放装备介绍进去 ✓
  return [
    ['阵容搭配', section(ctx.raw, '阵容搭配', 2)],
    ['词条推荐理由', kv(ctx.ci, '词条推荐理由')],
    ['技能升级推荐理由', kv(ctx.b0, '技能升级推荐理由')],
    ['角色定位', section(ctx.raw, '角色定位', 2)],
  ]
}

// 逐级回退取毕业线：返回第一个能摘出阈值句的来源 ✓（并报出它来自哪一节 ✓）
function pickGraduation(sources) {
  for (const [label, text] of sources) {
    const s = thresholdSentences(text)
    if (s.length) return { text: s.join('；'), from: label }
  }
  return { text: '', from: '' }
}

// 取某个二级/三级标题下的正文（到下一个同级或更高级标题为止）
function section(text, title, level = 2) {
  const re = new RegExp('^={' + level + '}\\s*' + title + '\\s*={' + level + '}\\s*$', 'm')
  const m = re.exec(text)
  if (!m) return ''
  const rest = text.slice(m.index + m[0].length)
  const end = rest.search(new RegExp('^={2,' + level + '}[^=]', 'm'))
  return end === -1 ? rest : rest.slice(0, end)
}

// 解析第一个/所有 {{简易配装 ... }} 的 key=value（逐行，天然容忍值与嵌套模板）
function parseSimpleBuild(text) {
  const blocks = []
  const re = /\{\{简易配装[\s\S]*?\n\}\}/g
  let m
  while ((m = re.exec(text)) !== null) blocks.push(m[0].split('\n').map((l) => l.trim()))
  return blocks
}

// 一行里可能有**多个图标**（如"和璞鸢/决斗之枪"并列，共用同一段说明 ✓）⇒ 逐个取名字
function iconNames(line) {
  const out = []
  const re = /\{\{图标(?:\/小)?\|([^{}]*)\}\}/g
  let m
  while ((m = re.exec(line)) !== null) {
    const seg = m[1].split('|').map((s) => s.trim()).filter(Boolean)
    const name = seg[1] ?? seg[0] ?? ''
    const piece = seg.find((s) => /^\d件套$/.test(s))
    if (name) out.push(name + (piece ? '·' + piece : ''))
  }
  return out
}

function parseDetailTable(text, iconType) {
  // ⚠️ 别用 `类型=X` 过滤：圣遗物那张表有 `|类型=圣遗物`，**武器那张表没有这个参数** ✗
  //   （2026-10-12 实测：按 类型= 过滤会把武器详表整块漏掉 ⇒ 每张卡"武器 0 条"✗）
  //   ⇒ 改为按图标类型认：`|圣遗物|` / `|武器|`
  const blocks = text.match(/\{\{角色\/配装推荐\/详表[\s\S]*?\n\}\}/g) ?? []
  const out = []
  for (const b of blocks) {
    if (iconType && !b.includes('|' + iconType + '|')) continue
    const lines = b.split('\n').map((l) => l.trim()).filter((l) => l.startsWith('|'))
    let pending = []
    for (const l of lines) {
      const v = l.slice(1).trim()
      const names = iconNames(v)
      if (names.length) { pending = names }
      else if (pending.length) {
        const why = clean(v)
        for (const n of pending) out.push({ name: n, why })
        pending = []
      }
    }
  }
  return out
}

// ---------------------------------------------------------------- 逐页提炼
const manifest = JSON.parse(fs.readFileSync(path.resolve('scripts/cache/guides.manifest.json'), 'utf8'))
let titles = Object.keys(manifest.pages)
if (namesArg) titles = titles.filter((t) => namesArg.some((n) => t === n + '/攻略' || t.startsWith(n + '/')))

// ---------------------------------------------------------------- 配队反向索引（2026-10-13 加）
// 为什么需要它：**有的攻略页自己根本没写配队** ✗ —— 班尼特 / 希诺宁 的「阵容搭配」小节里
//   只有 `{{角色/阵容搭配|展示}}`（不含配队参数 ✓）⇒ 清洗后 0 字 ✓。那是 wiki 的真实状态，
//   **不该编** ✗，但如果留空，用户问"班尼特怎么配队"就又回到"没有查到"✗。
// 换个方向就有数据 ✓：这两个角色**出现在别的角色攻略页的 `|辅助角色=` 里**
//   （实测：希诺宁 55 处、班尼特 43 处、芙宁娜 58 处、那维莱特仅 1 处 ✓）
//   ⇒ 从"谁把 TA 当队友 + 在什么队伍类型里"反推出搭配 ✓，且每一条都能追回原页 ✓。
const teamIndex = []
for (const t of Object.keys(manifest.pages)) {
  const f = path.join(RAW, t.replace(/[\\/:*?"<>|]/g, '_') + '.wiki')
  if (!fs.existsSync(f)) continue
  const owner = t.replace(/\/攻略$/, '').replace(/\/.*$/, '')
  const blocks = stripComments(fs.readFileSync(f, 'utf8')).match(/\{\{角色\/阵容搭配\|配队[\s\S]*?\n\}\}/g) ?? []
  for (const b of blocks) {
    const type = /^\|队伍类型=([^\n]*)/m.exec(b)?.[1]?.trim() ?? ''
    const mates = (/^\|辅助角色=([^\n]*)/m.exec(b)?.[1] ?? '')
      .split(/[、，,]/).map((s) => s.trim()).filter(Boolean)
    if (mates.length) teamIndex.push({ owner, type, mates })
  }
}
// 返回"TA 常被谁编进什么队"的摘要（去重到 6 个队，附总数便于判断可信度 ✓）
const seenIn = (name) => {
  const hits = teamIndex.filter((e) => e.mates.includes(name))
  const byOwner = new Map()
  for (const h of hits) if (!byOwner.has(h.owner)) byOwner.set(h.owner, h.type)
  return {
    count: hits.length,
    owners: [...byOwner.entries()].slice(0, 6).map(([o, ty]) => (ty ? `${o}（${ty}）` : o)),
  }
}

const cards = []
for (const title of titles) {
  const file = path.join(RAW, title.replace(/[\\/:*?"<>|]/g, '_') + '.wiki')
  if (!fs.existsSync(file)) { console.log(`   [缺缓存] ${title}`); continue }
  const raw = stripComments(fs.readFileSync(file, 'utf8'))

  const builds = parseSimpleBuild(raw)
  const b0 = builds[0] ?? []
  const buildNames = (raw.match(/\{\{角色\/配装推荐\|默认显示\|([^}]*)\}\}/g) ?? []).map((s) => s.split('|').pop().replace('}}', '').trim())

  const relics = parseDetailTable(raw, '圣遗物')
  const weapons = parseDetailTable(raw, '武器')
  const ci = raw.match(/\{\{角色\/配装推荐\/词条[\s\S]*?\n\}\}/)?.[0]?.split('\n').map((l) => l.trim()) ?? []

  const charName = title.replace(/\/攻略$/, '').replace(/\/.*$/, '') // 旅行者/风 → 旅行者

  // 毕业线：先算（explain 模式要用 ✓）
  const gradSources = graduationSources({ raw, ci, b0, relics, weapons })
  const grad = pickGraduation(gradSources)
  if (explainArg) {
    if (charName !== explainArg) continue
    console.log(`\n  === ${title} 的毕业线候选（按可信度排序 ✓）===`)
    for (const [label, text] of gradSources) {
      const s = thresholdSentences(text)
      console.log(`  [${label}] 摘出 ${s.length} 句`)
      for (const x of s.slice(0, 6)) console.log('     · ' + x)
    }
    console.log(`  ⇒ 采用：${grad.from || '(空)'}　最终 ${grad.text.length} 字`)
    console.log('     ' + (grad.text || '(无阈值句 ✓ 不硬凑 ✗)'))
    continue
  }

  const ownTeam = clipSent(clean(section(raw, '阵容搭配', 2)), 500)
  const rev = seenIn(charName)
  // ⚠️ **不加"据其他角色攻略页"前缀** ✗：主服务渲染时会按 `teamFrom` 自己加标签 ✓
  //   两处都加 ⇒ 注入文本会变成「配队思路（据其他角色攻略页）：据其他角色攻略页：共 70 处…」✗
  //   （2026-10-13 用 `guideimport -probe` 实测看到 ✓）
  const teamsSeen = rev.count
    ? `共 ${rev.count} 处把 TA 列为队友，例如 ${rev.owners.join('、')}`
    : ''

  const card = {
    page: title,
    // ⚠️ 字段名就叫 name（2026-10-13 由 characterGuess 改名 ✓）：入库时要按它锚 characters.id ✓
    name: charName,
    formElement: title.includes('/') && !title.endsWith('/攻略') ? title.split('/')[1] : null,
    sourceUrl: 'https://wiki.biligame.com/ys/' + encodeURIComponent(title),
    sourceRev: manifest.pages[title]?.revid,
    fetchedAt: manifest.pages[title]?.timestamp,
    // ① 定位（散文；2026-10-13 放宽到 500 字 + 按句收口 ✓）
    role: clipSent(clean(section(raw, '角色定位', 2)), 500),
    // ② 天赋顺序：来自「技能升级推荐」+ 理由（结构化 ✓）
    talents: clipSent([clean(kv(b0, '技能升级推荐')), clean(kv(b0, '技能升级推荐理由'))].filter(Boolean).join('｜'), 300),
    // ③ 圣遗物：套装 + 沙/杯/头 + 副词条（结构表与攻略重叠的那部分，只留"理由"给攻略卡 ✓）
    //   ⚠️ 刻意**不写 mainStats / subStats** ✗（2026-10-13 按 TODO-18 硬约束「攻略卡不重写词条」）：
    //     主/副词条一律以结构化表 character_stat_weights 为准（能分级、能参与评分 ✓），
    //     攻略卡只保留"为什么这么堆"的理由 ⇒ 两处都写必然打架（用户会看到 A/B 两种说法 ✗）
    relicPick: {
      sets: [1, 2, 3].map((i) => ({ name: clean(kv(b0, `圣遗物${i}`)), why: clip(clean(kv(b0, `圣遗物描述${i}`)), 200) })).filter((x) => x.name),
      reasons: clipSent(clean(kv(ci, '词条推荐理由')), 400),
      detail: relics.slice(0, 6).map((r) => ({ ...r, why: clip(r.why, 200) })),
    },
    // ④ 武器
    weaponPick: {
      graduation: clean(kv(b0, '毕业武器')),
      graduationWhy: clean(kv(b0, '毕业武器描述')),
      options: [1, 2, 3].map((i) => ({ name: clean(kv(b0, `可选武器${i}`)), why: clean(kv(b0, `可选武器描述${i}`)) })).filter((x) => x.name),
      detail: weapons.slice(0, 6).map((r) => ({ ...r, why: clip(r.why, 200) })),
    },
    // ⑤ 配队思路（2026-10-13 放宽到 500 字 + 按句收口 ✓）
    //   ⚠️ 本页没写配队时（班尼特 / 希诺宁 ✓）**不硬凑** ✗：退用反向索引的"常被编入"摘要 ✓
    //   `teamFrom` 标明这条配队是**本页原文**还是**反向索引** ✓（注入时要能说清依据 ✓）
    team: ownTeam || teamsSeen,
    teamFrom: ownTeam ? 'page' : (teamsSeen ? 'reverse' : ''),
    teamsSeen,
    // ⑥ 毕业线
    //   ⚠️ 事实：128 篇攻略页的**全部 41 种小节标题**里没有"毕业/面板"小节 ✗
    //     （2026-10-12 实测；提到"面板"的 67 篇里绝大多数是"面板武器"这种说法 ≠ 毕业阈值 ✓）
    //   ⇒ 本脚本**不编数值** ✗，只把页面内**带阈值性质的原句**摘出来（可追溯到本页 ✓）
    //     外部毕业面板数据（九游/游侠等，多源数值基本一致 ✓）留作 graduationExternal ✓ 待补
    // ⑥ 毕业线（口径按用户 2026-10-13 定：**先按社区能抓到的来**，不合理再调 ✓）
    //   来源由 3 处扩到 6 处（原来只查前 3 处 ⇒ 丝柯克/菲林斯两张卡是空的 ✗）
    // 上限 240 字（2026-10-13 由 400 收紧 ✗）：攻略页的达标线是**按队伍逐行**列的，
    //   实测妮露有 20+ 行（每套配队一行 ✓）⇒ 全塞进去会把这一栏变成噪音 ✓
    graduation: clipSent(grad.text, 240),
    // graduationFrom 是哪一节摘出来的（只进表与后台 ✓，不进注入文本 ✗）
    graduationFrom: grad.from,
    graduationSource: 'biligame wiki 攻略页内文摘录（该页没有"毕业面板"小节 ✓；数值=社区口径，非官方 ✓）',
    // ⑦ 注意事项：从定位与词条理由里挑带"数值阈值"的句子
    // ⑦ 注意事项：**换口径**（2026-10-13）—— 原来与 graduation 抢同一段文本 ⇒ 两张卡内容重叠 ✗
    //   现在只挑"命座 / 手法 / 误区"这类**行动建议**句（数值阈值句归 graduation ✓），并剔掉带百分号的，
    //   避免又变成同一批句子 ✓
    notes: clipSent(
      clean([section(raw, '角色定位', 2), section(raw, '阵容搭配', 2)].join('\n'))
        .split(/[。\n]/)
        .map((s) => s.trim().replace(/^\|\s*/, '').replace(/^\*+/, ''))
        .filter((s) => s && /(命座|手法|不建议|注意|建议|优先|误区|不推荐|要小心)/.test(s) && !/\d+(\.\d+)?%/.test(s))
        .slice(0, 8)
        .join('；'),
      300,
    ),
    buildsSeen: buildNames,
    rawBytes: Buffer.byteLength(raw),
  }
  cards.push(card)
  console.log(`   ${title.padEnd(18)} 定位${card.role.length}字 天赋${card.talents.length}字 圣遗物${card.relicPick.detail.length}条 武器${card.weaponPick.detail.length}条 配队${card.team.length}字${!ownTeam && teamsSeen ? '（反向索引 ✓）' : ''}`)
}

// ⚠️ --explain 必须**在这里退出** ✗✗：循环里的 `continue` 只跳过了建卡，
//   脚本末尾照样会把空 cards 落盘 ⇒ 2026-10-13 实测把 128 张卡的
//   `scripts/data/character_guides.json` 覆盖成了「0 张」✗（教训：调试开关绝不能
//   穿过写盘语句 ✓；且这份 JSON 必须入库版本管理，才能在被误写时恢复 ✓）。
if (explainArg) {
  console.log('\n  （--explain 只读 ✓：只打印候选句，不写任何文件 ✓）')
  process.exit(0)
}

fs.mkdirSync(path.dirname(OUT), { recursive: true })
fs.writeFileSync(OUT, JSON.stringify({ extractedAt: new Date().toISOString(), count: cards.length, source: 'biligame wiki（Template:角色攻略头 反查页）', cards }, null, 2), 'utf8')
console.log(`\n  共 ${cards.length} 张卡 → ${OUT}`)
