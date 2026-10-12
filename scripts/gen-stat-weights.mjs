// 生成全量角色推荐词条种子文件（ImportPayload 格式）
// 用法: node scripts/gen-stat-weights.mjs
// 输出: server/cmd/assetimport/stat_weights_full.json
// 之后导入: go run ./cmd/assetimport -stat-weights-file cmd/assetimport/stat_weights_full.json
//
// 规则（与芙宁娜手工配置一致）：
//   - 每个角色 4~5 条副词条，权重越大越优先
//   - 权重前 3 = primary（主要属性），其余 = secondary（次要属性），未列出的属性前端显示灰色
//   - mainStats: sand(时之沙) / goblet(空之杯) / circlet(理之冠)
//   - {E} 会被替换为角色元素名，如 "冰元素伤害加成"

const KEY = {
  ATK_P: 'FIGHT_PROP_ATTACK_PERCENT',
  ATK: 'FIGHT_PROP_ATTACK',
  HP_P: 'FIGHT_PROP_HP_PERCENT',
  HP: 'FIGHT_PROP_HP',
  DEF_P: 'FIGHT_PROP_DEFENSE_PERCENT',
  DEF: 'FIGHT_PROP_DEFENSE',
  EM: 'FIGHT_PROP_ELEMENT_MASTERY',
  CRIT: 'FIGHT_PROP_CRITICAL',
  CD: 'FIGHT_PROP_CRITICAL_HURT',
  ER: 'FIGHT_PROP_CHARGE_EFFICIENCY',
  HEAL: 'FIGHT_PROP_HEAL_ADD',
  PHYS: 'FIGHT_PROP_PHYSICAL_ADD',
}

// 副词条模板：数值 = 权重；mainStats 支持 "{E}" 占位
// 定义: [主属性三元组, 副词条数组, mainStats]
const ARCHETYPES = {
  // 输出：攻击力型
  ATK_C: {
    subs: [[KEY.ATK_P, 10], [KEY.CRIT, 8], [KEY.CD, 8], [KEY.ER, 4], [KEY.ATK, 2]],
    main: { sand: '攻击力', goblet: '{E}元素伤害加成', circlet: '暴击率' },
  },
  // 输出：生命值型
  HP_C: {
    subs: [[KEY.HP_P, 10], [KEY.CRIT, 8], [KEY.CD, 8], [KEY.ER, 4], [KEY.HP, 2]],
    main: { sand: '生命值', goblet: '{E}元素伤害加成', circlet: '暴击率' },
  },
  // 输出：防御力型
  DEF_C: {
    subs: [[KEY.DEF_P, 10], [KEY.CRIT, 8], [KEY.CD, 8], [KEY.ER, 4], [KEY.DEF, 2]],
    main: { sand: '防御力', goblet: '{E}元素伤害加成', circlet: '暴击率' },
  },
  // 输出：物理伤害型
  PHYS_C: {
    subs: [[KEY.ATK_P, 10], [KEY.CRIT, 8], [KEY.CD, 8], [KEY.ER, 4], [KEY.ATK, 2]],
    main: { sand: '攻击力', goblet: '物理伤害加成', circlet: '暴击率' },
  },
  // 输出：激化 / 元素精通型
  EM_C: {
    subs: [[KEY.EM, 10], [KEY.CRIT, 8], [KEY.CD, 8], [KEY.ER, 4], [KEY.ATK_P, 2]],
    main: { sand: '元素精通', goblet: '{E}元素伤害加成', circlet: '暴击率' },
  },
  // 副C：充能需求高的后台输出
  ER_C: {
    subs: [[KEY.ER, 10], [KEY.CRIT, 8], [KEY.CD, 8], [KEY.ATK_P, 6], [KEY.EM, 2]],
    main: { sand: '元素充能效率', goblet: '{E}元素伤害加成', circlet: '暴击率' },
  },
  // 扩散辅助：精通三件套
  EM_FULL: {
    subs: [[KEY.EM, 10], [KEY.ER, 8], [KEY.CRIT, 4], [KEY.ATK_P, 2]],
    main: { sand: '元素精通', goblet: '元素精通', circlet: '元素精通' },
  },
  // 治疗：攻击力倍率
  HEAL_ATK: {
    subs: [[KEY.ATK_P, 10], [KEY.ER, 8], [KEY.ATK, 6], [KEY.CRIT, 2]],
    main: { sand: '攻击力', goblet: '攻击力', circlet: '治疗加成' },
  },
  // 治疗 / 护盾：生命值倍率
  HEAL_HP: {
    subs: [[KEY.HP_P, 10], [KEY.ER, 8], [KEY.HP, 6], [KEY.EM, 2]],
    main: { sand: '生命值', goblet: '生命值', circlet: '治疗加成' },
  },
  // 治疗：防御力倍率
  HEAL_DEF: {
    subs: [[KEY.DEF_P, 10], [KEY.ER, 8], [KEY.DEF, 6], [KEY.EM, 2]],
    main: { sand: '防御力', goblet: '防御力', circlet: '治疗加成' },
  },
  // 攻击力增益辅助（buff 基于自身攻击）
  SUP_ATK: {
    subs: [[KEY.ATK_P, 10], [KEY.ER, 8], [KEY.ATK, 6], [KEY.CRIT, 2]],
    main: { sand: '攻击力', goblet: '攻击力', circlet: '攻击力' },
  },
  // 防御力增益辅助
  SUP_DEF: {
    subs: [[KEY.DEF_P, 10], [KEY.ER, 8], [KEY.DEF, 6], [KEY.EM, 2]],
    main: { sand: '元素充能效率', goblet: '防御力', circlet: '防御力' },
  },
  // 护盾辅助（生命值堆盾量）
  SHIELD_HP: {
    subs: [[KEY.HP_P, 10], [KEY.ER, 8], [KEY.HP, 6], [KEY.EM, 2]],
    main: { sand: '生命值', goblet: '生命值', circlet: '生命值' },
  },
  // 通用充能辅助
  // ⚠️ 2026-10-12 修正：这一档原先杯/冠是照**输出**写的（元素增伤 + 暴击率）✗ ⇒
  // 只要角色被定成这个原型，哪怕他是纯拐 / 盾 / 奶，杯上也自动挂了元素增伤（用户实测反馈）✗。
  // 现在杯默认【攻击力】（拐大多按攻击力给增益 ✓），冠留【暴击率】（少数确实吃暴击的：
  // 九条裟罗 / 珐露珊 / 柯莱 ✓）。逐角色定稿一律走 stat_weights_source.json 的 overrides ✓。
  SUP_ER: {
    subs: [[KEY.ER, 10], [KEY.ATK_P, 8], [KEY.CRIT, 6], [KEY.HP_P, 4], [KEY.EM, 2]],
    main: { sand: '元素充能效率', goblet: '攻击力', circlet: '暴击率' },
  },
}

// 逐角色配置: [characterId, 原型, 元素, mainStats覆盖(可选)]
// 逐角色定档与人工覆盖 —— 【数据不入代码】
// 数据在种子文件 cmd/assetimport/stat_weights_source.json（档位 / 人工覆盖 / 中文注记）。
// 运行时真源是数据库 character_stat_weights：本脚本只用于批量重生成与重建；
// 日常改数据走后台「资产映射管理 → 角色 → 推荐词条」或 POST /api/admin/assets/import（见 web/07 §4.2）。
import { readFileSync } from 'node:fs'
const SOURCE = JSON.parse(readFileSync(
  new URL('../server/cmd/assetimport/stat_weights_source.json', import.meta.url), 'utf8'))
const CHARS = SOURCE.chars
const MANUAL = SOURCE.manual

// 月曜（月感电/月绽放/月结晶）与星烁（星超导/星扩散）反应**不吃元素伤害加成**（2026-10-12 联网核实，
// 见 docs）⇒ 名单里的角色不许挂元素增伤杯。名单是**数据**（放 stat_weights_source.json ✓，不入代码 ✓）。
const NO_ELEM_GOBLET = new Set(SOURCE.noElementGoblet ?? [])
// 名单内角色该拿什么杯：按原型赖以吃饭的属性给（输出原型 → 攻击/生命/防御/精通）
const NO_ELEM_FALLBACK_BY_ARCH = {
  ATK_C: '攻击力', HP_C: '生命值', DEF_C: '防御力', EM_C: '元素精通', ER_C: '攻击力', SUP_ER: '攻击力',
}
// 逐角色主词条定稿：**最后**套用，优先于原型与 withAlternatives 的二选一展开 ✓
const OVERRIDES = SOURCE.overrides ?? {}

// 主词条"可选项"派生规则（2026-09-27 新增）
//
// 动机（线上实测）：mainStats 每个部位只能写一个值时，模型会把"推荐"读成"唯一正确"，
// 于是把旅行者身上**完全合法**的另一件主词条判成"错的"——用户反馈原话：
// 「先把理之冠换成暴击率主词条，这是唯一不对的一条」✗（角色同时吃双暴时，两条冠都成立）。
// 做法：写成 "A/B"（渲染层展开为「A 或 B」，见 server/internal/service/ai_context.go），
//       让模型看得出这是二选一、都成立。
// 判据只用我们自己的副词条权重（数据驱动，不引入外部依赖）：
//   · 冠：暴击率与暴击伤害都达到 primary 量级 ⇒ 两条都可选
//   · 沙：现为攻击力且充能需求同样高 ⇒ 充能沙同样可选
// ⚠️ 本生成器产出的 `weight` 是**老的 10 分制"推荐优先级"** ✗。
// 2026-09-27 起，"推荐优先级"与"圣遗物评分权重"合并成**同一个数字**（0~100，见 ai/08 第 11 条）✓，
// 且那个数字**归后台所有** ⇒ 导入侧对它是"**只补缺、不覆盖**"（server/internal/service/asset/import.go）：
//   · 老角色：同 statKey 的权重一律保留库里的 ✓（本脚本的 10 分制不会生效 ✓）
//   · 新角色：首次导入会拿本脚本的值当**初值** ✗ —— 若要精确评分，请用 mp 种子/后台修正 ✓
// ⇒ 重跑本脚本 + 重导入**不会**冲掉（也不会改坏）已有角色的评分权重 ✓。
const CRIT_BOTH_MIN = 6 // 双暴都到这个量级才算"都吃"
const ER_SAND_MIN = 8 // 充能权重大到这个量级，充能沙才成立

function withAlternatives(mainStats, subs) {
  const w = Object.fromEntries(subs.map(([k, v]) => [k, v]))
  const out = { ...mainStats }
  if ((w[KEY.CRIT] ?? 0) >= CRIT_BOTH_MIN && (w[KEY.CD] ?? 0) >= CRIT_BOTH_MIN) {
    if (out.circlet === '暴击率') out.circlet = '暴击率/暴击伤害'
    else if (out.circlet === '暴击伤害') out.circlet = '暴击伤害/暴击率'
  }
  if (out.sand === '攻击力' && (w[KEY.ER] ?? 0) >= ER_SAND_MIN) {
    out.sand = '攻击力/元素充能效率'
  }
  return out
}

function build([id, arch, element, override]) {
  if (MANUAL[id]) {
    // 人工定稿的条目保持原样：改它要动数据文件 stat_weights_source.json（数据不入代码）
    return {
      characterId: id,
      recommendation: {
        subStats: MANUAL[id].subStats.map(([statKey, weight, tier]) => ({ statKey, weight, tier })),
        // ⚠️ 2026-10-12：overrides 仍然**最后套用** ✓（主词条的定稿处就在那里）——
        // 否则 manual 块与 overrides 会各说一套、互相矛盾（菲林斯就踩到过）✗。
        mainStats: { ...MANUAL[id].mainStats, ...(OVERRIDES[id] ?? {}) },
      },
    }
  }
  const tpl = ARCHETYPES[arch]
  if (!tpl) throw new Error(`未知原型: ${arch} (characterId=${id})`)
  const main = Object.fromEntries(
    Object.entries(tpl.main).map(([slot, v]) => [slot, v.replaceAll('{E}', element)]),
  )
  // 月曜 / 星烁体系：不吃元素增伤 ⇒ 元素增伤杯换成该原型的非元素主词条（2026-10-12）
  if (NO_ELEM_GOBLET.has(id) && main.goblet.includes('元素伤害加成')) {
    main.goblet = NO_ELEM_FALLBACK_BY_ARCH[arch] ?? '攻击力'
  }
  return {
    characterId: id,
    recommendation: {
      subStats: tpl.subs.map(([statKey, weight], i) => ({
        statKey,
        weight,
        tier: i < 3 ? 'primary' : 'secondary', // 权重前 3 = 主要属性
      })),
      // 顺序：原型 → 二选一展开 → 逐角色 overrides 定稿（定稿优先级最高 ✓，保证与库一致 ✓）
      mainStats: { ...withAlternatives({ ...main, ...(override ?? {}) }, tpl.subs), ...(OVERRIDES[id] ?? {}) },
    },
  }
}

const payload = { statWeights: CHARS.map(build) }
const out = new URL('../server/cmd/assetimport/stat_weights_full.json', import.meta.url)
const { writeFileSync } = await import('node:fs')
writeFileSync(out, JSON.stringify(payload, null, 1), 'utf8')
console.log(`OK: ${payload.statWeights.length} 个角色 → ${out.pathname.slice(1)}`)
