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
  SUP_ER: {
    subs: [[KEY.ER, 10], [KEY.ATK_P, 8], [KEY.CRIT, 6], [KEY.HP_P, 4], [KEY.EM, 2]],
    main: { sand: '元素充能效率', goblet: '{E}元素伤害加成', circlet: '暴击率' },
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

function build([id, arch, element, override]) {
  if (MANUAL[id]) {
    return {
      characterId: id,
      recommendation: {
        subStats: MANUAL[id].subStats.map(([statKey, weight, tier]) => ({ statKey, weight, tier })),
        mainStats: MANUAL[id].mainStats,
      },
    }
  }
  const tpl = ARCHETYPES[arch]
  if (!tpl) throw new Error(`未知原型: ${arch} (characterId=${id})`)
  const main = Object.fromEntries(
    Object.entries(tpl.main).map(([slot, v]) => [slot, v.replaceAll('{E}', element)]),
  )
  return {
    characterId: id,
    recommendation: {
      subStats: tpl.subs.map(([statKey, weight], i) => ({
        statKey,
        weight,
        tier: i < 3 ? 'primary' : 'secondary', // 权重前 3 = 主要属性
      })),
      mainStats: { ...main, ...(override ?? {}) },
    },
  }
}

const payload = { statWeights: CHARS.map(build) }
const out = new URL('../server/cmd/assetimport/stat_weights_full.json', import.meta.url)
const { writeFileSync } = await import('node:fs')
writeFileSync(out, JSON.stringify(payload, null, 1), 'utf8')
console.log(`OK: ${payload.statWeights.length} 个角色 → ${out.pathname.slice(1)}`)
