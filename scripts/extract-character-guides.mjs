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
// --why-empty：列出**毕业线为空**的角色，并说明"缺的是什么"（逐来源统计拒因 ✓，不写文件 ✓）
const whyEmpty = args.includes('--why-empty')

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
//   ⚠️ 2026-10-14 由 80 放宽到 140（用户问"空的 53 位缺什么"时实测 ✗）：
//     80 会把**真门槛句**误杀 ✓ —— 雅珂达「…依靠能量需求达70点的元素爆发治疗，需要
//     220%以上的元素充能效率，解锁第四命座后…」（= 充能 220% ✓）整句被判"过长"✗✗。
//     放宽后仍由"面板指标 + 数字 + 门槛词"三条把散文挡在外面 ✓（不是靠长度挡 ✓）。
const THRESHOLD_MAX_CHARS = 140

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

// 门槛词：只认**目标/上限**口径 ✓ —— 实测（2026-10-13）放宽到"需要/建议/推荐"会把
// 「夜兰…应优先保证充能循环的前提下提升伤害」✗「阿蕾奇诺…攻击收益只有双暴的40%~60%」✗
// 这类**机制/稀释解释**捞进来 ⇒ 宁可为空也不放噪音 ✓
const hasGate = (s) => /(以上|即可|至少|不低于|起步|上限|不超过|低于|大约|达到)/.test(s)
// **面板表行**（首选 ✓）：攻略页「阵容搭配」里的达标线本来就写成
//   「暴击率/暴击伤害：70%/180%以上；元素精通：200以上」这种 指标：数值 的行 ✓
const STAT_LINE_RE =
  /^(（[^）]{1,24}）|【[^】]{1,24}】)?\s*(双暴|暴击率|暴击伤害|暴伤|元素精通|精通|元素充能效率|充能效率|充能|生命值|攻击力|防御力|元素伤害加成|治疗加成)\s*[:：]/

// thresholdExplain 逐句给出「留 / 拒（附原因）」—— 供 --why-empty 诊断用 ✓
//   判定顺序与正式抽取**完全一致** ✓（改这里必须同步改 thresholdSentences 的语义 ✗）
function thresholdExplain(text) {
  const sents = clean(text ?? '')
    .split(/[。；\n]/)
    .map(cleanSentence)
    .filter(Boolean)

  const statLines = [] // ① 面板表行（最高优先 ✓）
  const prose = [] // ② 散文门槛句（只有该来源**没有**面板表行时才用 ✓）
  const rejected = []
  for (const s of sents) {
    if (/^[」』）)】]/.test(s)) {
      rejected.push({ s, why: '残句（引号/模板被截断）' })
      continue
    }
    // ⚠️ 顺序很关键 ✗：**先认"真门槛"，再判排除项** ——
    //   2026-10-14 实测：先判排除词会把真门槛误杀 ✓
    //   （沃雅妮莎「天赋「十二弦的泪歌」…在生命值达到 65000 点时达到上限」✓含"天赋"⇒被判机制句✗）
    //   ⇒ 只要同时满足 面板指标 + 数字 + 门槛/上限词，就认它是毕业线 ✓；
    //     装备介绍（GEAR_WORDS）仍然**一律排除** ✗（那是用户投诉的那类噪音 ✓）。
    if (STAT_LINE_RE.test(s) && /\d/.test(s)) {
      statLines.push(s)
      continue
    }
    if (/\d/.test(s) && PANEL_WORDS.test(s) && hasGate(s) && !GEAR_WORDS.test(s)) {
      prose.push(s)
      continue
    }
    if (s.length > THRESHOLD_MAX_CHARS) {
      rejected.push({ s, why: `过长（>${THRESHOLD_MAX_CHARS} 字）` })
      continue
    }
    if (GEAR_WORDS.test(s)) {
      rejected.push({ s, why: '装备介绍（武器/沙杯头/套装）' })
      continue
    }
    if (NOTE_WORDS.test(s)) {
      rejected.push({ s, why: '命座/手法/注意类' })
      continue
    }
    if (MECHANIC_WORDS.test(s)) {
      rejected.push({ s, why: '机制/动作描述' })
      continue
    }
    if (!/\d/.test(s)) {
      rejected.push({ s, why: '没有数字' })
      continue
    }
    if (!PANEL_WORDS.test(s)) {
      rejected.push({ s, why: '不是面板指标句' })
      continue
    }
    if (!hasGate(s)) {
      rejected.push({ s, why: '没有门槛/上限词（以上/即可/至少…）' })
      continue
    }
    // 到这里说明：有数字、是面板指标、也有门槛词，但被上面的排除拦过 ⇒ 理论上不可达 ✓（保险起见收入 ✓）
    prose.push(s)
  }
  // ⚠️ **面板表行优先**：同一来源里两者都有时只取面板表行 ✓ ——
  //   2026-10-14 踩过 ✗：混排后散文排在前面，把 240 字预算吃光 ✓（胡桃/宵宫一度变成"茜特菈莉…"✗✗）
  return { kept: [...new Set(statLines.length ? statLines : prose)], rejected, hasStatLines: statLines.length > 0 }
}

// thresholdSentences 正式口径：只保留 kept ✓（与 thresholdExplain 同一套判定 ✓）
function thresholdSentences(text) {
  return thresholdExplain(text).kept
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

// ---------------------------------------------------------------- 外部面板兜底（TODO-18 的 graduationExternal ✓）
// 背景：128 篇 wiki 攻略页里只有约 89 篇写了达标线 ✗（其余的「阵容搭配」小节是空的 ✓，见 --why-empty ✓）。
// 外部补法（2026-10-14，用户点头："从别的地方抓一下，抓转载的也可以，不署名就没事" ✓）：
//   scripts/fetch-graduation-external.mjs  抓页面（缓存 ✓）
//   scripts/extract-graduation-external.mjs 抽「指标：数值」⇒ character_graduation_external.json ✓
//   **只在 wiki 摘不到时才用外部** ✓（wiki 是本项目一贯的可溯源来源 ✓，优先它 ✓）
//   ⇒ 注入文本里**不出现任何来源/署名** ✗；URL 只落进 `graduation_source` 列备查 ✓（后台可见 ✓）
const EXT_FILE = path.resolve('scripts/data/character_graduation_external.json')
let EXT_MAP = new Map()
try {
  const ext = JSON.parse(fs.readFileSync(EXT_FILE, 'utf8'))
  EXT_MAP = new Map(ext.characters.map((c) => [c.name, c]))
  console.log(`  外部面板兜底：${EXT_MAP.size} 位（${path.basename(EXT_FILE)}）`)
} catch {
  console.log('  外部面板兜底：没有文件 ⇒ 只用 wiki（跑 fetch/extract-graduation-external.mjs 可补 ✓）')
}

// ---------------------------------------------------------------- 人工毕业线（**优先级最高** ✓，用户 2026-10-14 定稿）
// 背景：128 篇里 wiki 写了的 87 位 + 外源补齐 18 位 = 105 位，剩 23 位两处都缺 ✗。
//   2026-10-14 用户逐位定稿（见文件 `_meta.decided_by_user` ✓）：
//     · `characters` = 人工给值 ⇒ **覆盖 wiki 面板行与外源** ✓（优先级：人工 > wiki 面板行 > 外源 > wiki 散文 ✓）
//     · `_skip`      = 明确**不指定达标标准** ⇒ 该页**对任何源都不取值** ✓（哪怕是真面板行也清空 ✗）
//       ⚠️ 这条是防"以后抓取又把它当成缺数据补回来"✓（同一份文件里带上理由 ✓，不会再回来问一遍 ✓）
const MANUAL_FILE = path.resolve('scripts/data/character_graduation_manual.json')
let MANUAL_BY_PAGE = new Map()
let MANUAL_SKIP = new Set()
try {
  const m = JSON.parse(fs.readFileSync(MANUAL_FILE, 'utf8'))
  MANUAL_BY_PAGE = new Map((m.characters ?? []).map((c) => [c.page, c]))
  MANUAL_SKIP = new Set((m._skip?.characters ?? []).map((c) => c.page))
  console.log(`  人工毕业线：有值 ${MANUAL_BY_PAGE.size} 位、明确不填 ${MANUAL_SKIP.size} 位（${path.basename(MANUAL_FILE)}）`)
} catch {
  console.log('  人工毕业线：没有文件 ⇒ 只用 wiki + 外部面板 ✓')
}

// stripWikiTables 剔掉 wiki 表格与它的残留标签 ✓（2026-10-14 加 ✓）
//
// 为什么两道都要 ✗：表格可能**嵌套在模板里**（如 `{{切换板}}`）⇒ 只按 `{|`…`|}` 配对会剩下
//   `输出位=` / `|-|增伤位=` 这类行 ✓（实测：林尼的 team 里就漏出过 ✓）。所以再用**行级规则**兜底 ✓。
function stripWikiTables(text) {
  return text
    .replace(/\{\|[\s\S]*?\n\s*\|\}/g, '') // 整张表（到行首的 |} ✓）
    .replace(/^\s*\|[-+}].*$/gm, '') // 表格行标记：|- 、|+ 、|} ✓
    .replace(/^\s*[!|].*$/gm, '') // 单元格行：! / | 开头 ✓
    .replace(/^\s*[\u4e00-\u9fff]{1,6}位=\s*$/gm, '') // "输出位=" 这类残留标签 ✓
    .replace(/^\s*参考属性[^\n]*如下[：:]\s*$/gm, '')
    .replace(/\n{3,}/g, '\n\n')
    .trim()
}

// 取某个二级/三级标题下的正文（到下一个同级或更高级标题为止）
function section(text, title, level = 2) {
  // ⚠️ 2026-10-14：标题级别在**少数页面**上不是二级 ✗ —— 逐级往下找（2 → 3 → 4）✓
  //
  // 实测踩到 ✓：林尼页的「阵容搭配」写成 `=== 阵容搭配 ===`（三级 ✓），里面有 **10595 字**配队内容 ✓，
  //   旧实现只按 `== 阵容搭配 ==` 找 ⇒ 整节取不到 ✗ ⇒ 卡片的 `team` 一直是空的 ✗
  //   （用户看到的就是"林尼缺配队"✓）。安柏 / 埃洛伊 / 旅行者那几位是**页里确实没写** ✓，与级别无关 ✓。
  for (let lv = level; lv <= level + 2; lv++) {
    const re = new RegExp('^={' + lv + '}\\s*' + title + '\\s*={' + lv + '}\\s*$', 'm')
    const m = re.exec(text)
    if (!m) continue
    const rest = text.slice(m.index + m[0].length)
    const end = rest.search(new RegExp('^={2,' + lv + '}[^=]', 'm'))
    return end === -1 ? rest : rest.slice(0, end)
  }
  return ''
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
// --why-empty 的收集器（诊断用 ✓）
const emptyDiag = []
// 外部兜底命中数（诊断用 ✓：为 0 就说明一条都没用上 ✗）
let extFilled = 0
// 人工层计数（诊断用 ✓）
let manualFilled = 0 // 人工给值 ✓
let skipForced = 0 // 人工决定"不填"⇒ 强制留空 ✓
let skipDropped = 0 // 其中原本被源填过 ⇒ 已清空 ✗
let skipEmpty = 0 // --why-empty 里被跳过的"已定不填"个数 ✓
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
  // wiki 摘不到 ⇒ 用外部面板兜底 ✓（并如实记来源；**这条来源不会进注入文本** ✗）
  //
  // ⚠️ 2026-10-14 定：**wiki 只摘到"散文句"时，外部面板优先** ✓ ——
  //   理由：wiki 从「阵容搭配」摘到的是**面板表行**（`暴击率/暴击伤害：70%/180%以上` ✓ 最优 ✓）；
  //   而从「词条推荐理由 / 角色定位」摘到的是**散文**（"治疗…低于 70% 时"✗ 参考价值低 ✓）。
  //   外部（转载汇总 ✓）给的就是指标:数值 ✓ ⇒ 这种情况换成外部更可用 ✓。
  const wikiIsStatLines = grad.from === '阵容搭配'
  const ext = EXT_MAP.get(charName)
  if ((!grad.text || !wikiIsStatLines) && ext) {
    grad.text = ext.text
    grad.from = '外部面板参考'
    grad.source = `${ext.sourceUrl}（采集于 ${String(ext.fetchedAt).slice(0, 10)}；社区参考值，非官方 ✓）`
    extFilled++
    console.log(`   [外部兜底] ${charName.padEnd(6)} ${ext.text}`)
  }

  // 人工层**最后压上** ✓（顺序很重要：wiki → 外源 → 人工 ✓，人工的值不会再被任何源顶掉 ✓）
  const manual = MANUAL_BY_PAGE.get(title)
  const skipped = MANUAL_SKIP.has(title)
  if (manual) {
    grad.text = manual.text
    grad.from = '人工填写'
    grad.source = manual.note ? `人工填写（${manual.note}）` : '人工填写'
    manualFilled++
  } else if (skipped) {
    // 明确不填 ⇒ **连源填的值也清掉** ✗（保"这 18 位保持为空"这条决定 ✓）
    if (grad.text) {
      skipDropped++
      console.log(`   [人工不填] ${charName.padEnd(6)} 清空源值：${grad.text.slice(0, 28)}…`)
    }
    grad.text = ''
    grad.from = ''
    grad.source = '人工确认不指定达标标准（用户 2026-10-14；理由见 character_graduation_manual.json 的 _skip ✓）'
    skipForced++
  }

  if (whyEmpty) {
    // 用户已定"不填"的位置不再报空 ✓（否则每次都当成待办 ✗）—— 只统计个数，末尾统一说明 ✓
    if (skipped) {
      skipEmpty++
      continue
    }
    if (grad.text) continue // 只报"空"的 ✓
    const pageLabel = title.replace(/\/攻略$/, '') // 旅行者/风 这种带形态的要能区分 ✓
    const diag = gradSources.map(([label, text]) => {
      const ex = thresholdExplain(text)
      const why = {}
      for (const r of ex.rejected) why[r.why] = (why[r.why] || 0) + 1
      const top = Object.entries(why).sort((a, b) => b[1] - a[1])[0]
      // 代表句要取**与主因同类**的那一句 ✓（否则会打印"主因=没有数字"却附一句有数字的例句 ✗）
      const sameKind = ex.rejected.find((r) => r.why === top?.[0]) ?? ex.rejected[0]
      return {
        label,
        chars: (text ?? '').replace(/\s+/g, '').length,
        rejected: ex.rejected.length,
        topWhy: top ? top[0] : '',
        sample: sameKind ? `${sameKind.s.slice(0, 44)} ← ${sameKind.why}` : '',
      }
    })
    const hasText = diag.filter((d) => d.chars > 0)

    // **关键判据**：那 75 张有值的卡，值几乎全部来自「阵容搭配」里的**面板数值表** ✓
    //   ⇒ 空的这批到底缺什么，就看这一节：是没内容 ✓、还是有散文但没数值 ✓、还是有数值但被判成机制/装备 ✓
    const teamText = gradSources.find((s) => s[0] === '阵容搭配')?.[1] ?? ''
    const teamChars = teamText.replace(/\s+/g, '').length
    const teamSents = clean(teamText).split(/[。；\n]/).map(cleanSentence).filter(Boolean)
    const teamNumeric = teamSents.filter((s) => /(\d+(\.\d+)?%|\d+\s*以上|\d+\s*点|\d+\s*个词条)/.test(s) && PANEL_WORDS.test(s)).length
    let bucket
    if (teamChars <= 60) bucket = 'A. 「阵容搭配」小节是空的（只有模板占位 ⇒ 页里就没写配队面板）'
    else if (teamNumeric === 0) bucket = 'B. 「阵容搭配」有文字，但没有"指标：数值"的面板行'
    else bucket = 'C. 「阵容搭配」有数值句，但被判为机制/装备/过长等（口径问题，可复核）'

    // 主缺因：把所有来源的拒因合起来取最多的那个 ✓（空来源不计 ✓）
    const whyAll = {}
    for (const d of diag) {
      const ex = thresholdExplain(gradSources.find((s) => s[0] === d.label)?.[1] ?? '')
      for (const r of ex.rejected) whyAll[r.why] = (whyAll[r.why] || 0) + 1
    }
    const mainWhy = Object.entries(whyAll).sort((a, b) => b[1] - a[1])[0]?.[0] ?? '页面里没有可摘的机制/数值句'
    emptyDiag.push({ name: pageLabel, bucket, hasText: hasText.length, diag, teamChars, teamNumeric, mainWhy })
    console.log(`\n  ${pageLabel} ⇒ ${mainWhy}`)
    console.log(
      `  【${pageLabel}】${bucket}\n     可用来源 ${hasText.length}/4（共 ${diag.reduce((n, d) => n + d.chars, 0)} 字）；阵容搭配 ${teamChars} 字/${teamNumeric} 条数值句`,
    )
    for (const d of hasText) {
      console.log(`     · ${d.label}：${d.chars} 字 / 拒 ${d.rejected} 句（${d.topWhy || '—'}）例：${d.sample.slice(0, 44)}`)
    }
    continue
  }

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

  // ⚠️ 2026-10-14：「阵容搭配」小节里常混着 **wiki 表格**（`{| … |}`，写各队友的参考面板/套装 ✓）
  //   外加一句"参考属性、圣遗物、武器如下："的引子 ⇒ 直接塞进 `team` 会污染注入文本 ✗
  //   （表格另有用途：**毕业线**就是从它的"指标：数值"行里抽的 ✓ ⇒ 这里只剔表、不动别处 ✓）
  const ownTeam = clipSent(stripWikiTables(clean(section(raw, '阵容搭配', 2))), 500)
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
    // ⚠️ 形态页的写法是「旅行者/风/攻略」（**三段** ✓）—— 原判断用 `!title.endsWith('/攻略')` 想排除
    //   普通页，却把 8 个形态页一起排除了 ✗ ⇒ 全部 formElement=null ⇒ 入库时按 character_id 互相覆盖 ✓
    //   （2026-10-14 用户点名修：旅行者各元素要**分开**处理 ✓，火主 / 冰主优先 ✓）。
    //   两段 = 普通角色页（`XX/攻略`）⇒ null ✓；三段 = 形态页 ⇒ 取中间那段（= 元素 ✓）
    formElement: title.split('/').length >= 3 ? title.split('/')[1] : null,
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
    graduationSource:
      grad.source ??
      'biligame wiki 攻略页内文摘录（该页没有"毕业面板"小节 ✓；数值=社区口径，非官方 ✓）',
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
// ⚠️ 只读模式必须在**写盘之前**退出 ✗✗（2026-10-13 的教训：--explain 曾把 128 张卡写成 0 张 ✓）
if (whyEmpty) {
  const groups = {}
  for (const e of emptyDiag) (groups[e.bucket] ||= []).push(e.name)
  console.log(`\n  ==== 毕业线为空的角色：${emptyDiag.length} 位，按"缺的是什么"分组 ====`)
  for (const [bucket, names] of Object.entries(groups).sort((a, b) => b[1].length - a[1].length)) {
    console.log(`\n  ▸ ${bucket} —— ${names.length} 位`)
    console.log('    ' + names.join('、'))
  }
  const teamEmpty = emptyDiag.filter((e) => e.teamChars <= 60).length
  const teamProse = emptyDiag.filter((e) => e.teamChars > 60 && e.teamNumeric === 0).length
  const byRule = emptyDiag.length - teamEmpty - teamProse
  console.log(`\n  小结：阵容搭配为空 ${teamEmpty} 位；有散文无面板行 ${teamProse} 位；有数值但被口径挡下 ${byRule} 位`)
  if (skipEmpty) {
    console.log(`  （另有 ${skipEmpty} 位已被用户明确「不指定达标标准」⇒ **不算待办** ✓，见 character_graduation_manual.json 的 _skip ✓）`)
  }
  console.log('  （--why-empty 只读 ✓：不写任何文件 ✓）')
  process.exit(0)
}

if (explainArg) {
  console.log('\n  （--explain 只读 ✓：只打印候选句，不写任何文件 ✓）')
  process.exit(0)
}

// 毕业线来源小结（一眼看出"人工 / 外源 / wiki"各占多少 ✓）
{
  const filled = cards.filter((c) => (c.graduation ?? '').trim()).length
  const byManual = cards.filter((c) => c.graduationFrom === '人工填写').length
  const byExt = cards.filter((c) => c.graduationFrom === '外部面板参考').length
  console.log(
    `\n  毕业线：有值 ${filled}/${cards.length} 位（人工 ${byManual} ＋ 外源 ${byExt} ＋ wiki ${filled - byManual - byExt}），` +
      `人工定"不填" ${skipForced} 位${skipDropped ? `（其中清空源值 ${skipDropped} 位 ✗）` : ''} ⇒ 空 ${cards.length - filled} 位`,
  )
}

fs.mkdirSync(path.dirname(OUT), { recursive: true })
fs.writeFileSync(OUT, JSON.stringify({ extractedAt: new Date().toISOString(), count: cards.length, source: 'biligame wiki（Template:角色攻略头 反查页）', cards }, null, 2), 'utf8')
console.log(`\n  共 ${cards.length} 张卡 → ${OUT}`)
