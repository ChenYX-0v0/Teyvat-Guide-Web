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
// 已知未来角色 / 未实装角色按元素套用通用输出或辅助模板，后续可按实测调整
const CHARS = [
  [10000002, 'ATK_C', '冰'],            // 神里绫华
  [10000003, 'HEAL_ATK', '风'],         // 琴
  [10000005, 'SUP_ER', '风'],           // 旅行者(风)
  [10000006, 'ER_C', '雷'],             // 丽莎
  [10000007, 'SUP_ER', '草'],           // 旅行者(草)
  [10000014, 'HEAL_HP', '水'],          // 芭芭拉
  [10000015, 'ATK_C', '冰'],            // 凯亚
  [10000016, 'ATK_C', '火'],            // 迪卢克
  [10000020, 'PHYS_C', '雷'],           // 雷泽
  [10000021, 'ATK_C', '火'],            // 安柏
  [10000022, 'EM_FULL', '风'],          // 温迪
  [10000023, 'ER_C', '火'],             // 香菱
  [10000024, 'ER_C', '雷'],             // 北斗
  [10000025, 'ER_C', '水'],             // 行秋
  [10000026, 'ATK_C', '风'],            // 魈
  [10000027, 'ATK_C', '岩'],            // 凝光
  [10000029, 'ATK_C', '火'],            // 可莉
  [10000030, 'HP_C', '岩'],             // 钟离（盾/副C 生命优先）
  [10000031, 'ATK_C', '雷'],            // 菲谢尔
  [10000032, 'HEAL_ATK', '火'],         // 班尼特
  [10000033, 'ATK_C', '水'],            // 达达利亚
  [10000034, 'DEF_C', '岩'],            // 诺艾尔
  [10000035, 'HEAL_ATK', '冰'],         // 七七
  [10000036, 'ATK_C', '冰'],            // 重云
  [10000037, 'ATK_C', '冰', { circlet: '暴击伤害' }], // 甘雨
  [10000038, 'DEF_C', '岩', { circlet: '暴击伤害' }], // 阿贝多
  [10000039, 'HEAL_HP', '冰'],          // 迪奥娜
  [10000041, 'ER_C', '水'],             // 莫娜
  [10000042, 'ATK_C', '雷'],            // 刻晴
  [10000043, 'EM_FULL', '风'],          // 砂糖
  [10000044, 'PHYS_C', '火'],           // 辛焱
  [10000045, 'ATK_C', '冰'],            // 罗莎莉亚
  [10000046, 'HP_C', '火', { circlet: '暴击伤害' }],  // 胡桃
  [10000047, 'EM_FULL', '风'],          // 枫原万叶
  [10000048, 'ATK_C', '火'],            // 烟绯
  [10000049, 'ATK_C', '火'],            // 宵宫
  [10000050, 'SHIELD_HP', '火'],        // 托马
  [10000051, 'PHYS_C', '冰', { circlet: '暴击伤害' }], // 优菈
  [10000052, 'ER_C', '雷'],             // 雷电将军（充能沙 + 双暴）
  [10000053, 'EM_FULL', '风'],          // 早柚
  [10000054, 'HEAL_HP', '水'],          // 珊瑚宫心海
  [10000055, 'SUP_DEF', '岩'],          // 五郎
  [10000056, 'SUP_ER', '雷'],           // 九条裟罗
  [10000057, 'DEF_C', '岩', { circlet: '暴击伤害' }], // 荒泷一斗
  [10000058, 'ATK_C', '雷'],            // 八重神子（走 MANUAL 定制）
  [10000059, 'ATK_C', '风'],            // 鹿野院平藏
  [10000060, 'HP_C', '水'],             // 夜兰
  [10000061, 'SHIELD_HP', '草'],        // 绮良良
  [10000062, 'ATK_C', '冰'],            // 埃洛伊
  [10000063, 'SUP_ATK', '冰'],          // 申鹤
  [10000064, 'SUP_DEF', '岩'],          // 云堇
  [10000065, 'HEAL_HP', '雷'],          // 久岐忍
  [10000066, 'ATK_C', '水'],            // 神里绫人
  [10000067, 'SUP_ER', '草'],           // 柯莱
  [10000068, 'HEAL_HP', '雷'],          // 多莉
  [10000069, 'EM_C', '草'],             // 提纳里
  [10000070, 'SHIELD_HP', '水'],        // 妮露（纯生命堆核心被动）
  [10000071, 'EM_C', '雷'],             // 赛诺
  [10000072, 'SUP_ER', '水'],           // 坎蒂丝
  [10000073, 'EM_C', '草'],             // 纳西妲
  [10000074, 'SHIELD_HP', '冰'],        // 莱依拉
  [10000075, 'ATK_C', '风'],            // 流浪者
  [10000076, 'SUP_ER', '风'],           // 珐露珊
  [10000077, 'HEAL_ATK', '草'],         // 瑶瑶
  [10000078, 'EM_C', '草'],             // 艾尔海森
  [10000079, 'HP_C', '火'],             // 迪希雅
  [10000080, 'HEAL_HP', '冰'],          // 米卡
  [10000081, 'EM_FULL', '草'],          // 卡维
  [10000082, 'HEAL_HP', '草'],          // 白术
  [10000083, 'ATK_C', '风'],            // 琳妮特
  [10000084, 'ATK_C', '火'],            // 林尼
  [10000085, 'PHYS_C', '冰'],           // 菲米尼
  [10000086, 'ATK_C', '冰'],            // 莱欧斯利
  [10000087, 'HP_C', '水', { circlet: '暴击伤害' }],  // 那维莱特
  [10000088, 'HEAL_ATK', '冰'],         // 夏洛蒂
  [10000090, 'HEAL_HP', '火'],          // 夏沃蕾
  [10000091, 'ATK_C', '岩'],            // 娜维娅
  [10000092, 'ATK_C', '火'],            // 嘉明
  [10000093, 'HEAL_ATK', '风'],         // 闲云
  [10000094, 'DEF_C', '岩', { circlet: '暴击伤害' }], // 千织
  [10000095, 'HEAL_HP', '水'],          // 希格雯
  [10000096, 'ATK_C', '火'],            // 阿蕾奇诺
  [10000097, 'EM_C', '雷'],             // 赛索斯
  [10000098, 'ATK_C', '雷'],            // 克洛琳德
  [10000099, 'ATK_C', '草'],            // 艾梅莉埃
  [10000100, 'ATK_C', '岩'],            // 卡齐娜
  [10000101, 'ATK_C', '草'],            // 基尼奇
  [10000102, 'HP_C', '水'],             // 玛拉妮
  [10000103, 'HEAL_DEF', '岩'],         // 希诺宁
  [10000104, 'ATK_C', '风'],            // 恰斯卡
  [10000105, 'EM_FULL', '雷'],          // 欧洛伦
  [10000106, 'ATK_C', '火'],            // 玛薇卡
  [10000107, 'SUP_ER', '冰'],           // 茜特菈莉
  [10000108, 'EM_FULL', '风'],          // 蓝砚
  [10000109, 'EM_FULL', '风'],          // 梦见月瑞希
  [10000110, 'SUP_ER', '雷'],           // 伊安珊
  [10000111, 'ATK_C', '雷'],            // 瓦雷莎
  [10000112, 'HEAL_ATK', '冰'],         // 爱可菲
  [10000113, 'EM_FULL', '风'],          // 伊法
  [10000114, 'ATK_C', '冰'],            // 丝柯克
  [10000115, 'SUP_ER', '水'],           // 塔利雅
  [10000116, 'ATK_C', '雷'],            // 伊涅芙
  [10000117, 'ATK_C', '草'],            // 奇偶·男性（未实装，通用模板）
  [10000118, 'ATK_C', '水'],            // 奇偶·女性（未实装，通用模板）
  [10000119, 'ATK_C', '草'],            // 菈乌玛
  [10000120, 'ATK_C', '雷'],            // 菲林斯（走 MANUAL 定制）
  [10000121, 'SUP_ER', '水'],           // 爱诺
  [10000122, 'ATK_C', '草'],            // 奈芙尔
  [10000123, 'ATK_C', '火'],            // 杜林
  [10000124, 'EM_FULL', '风'],          // 雅珂达
  [10000125, 'ATK_C', '水'],            // 哥伦比娅
  [10000126, 'ATK_C', '岩'],            // 兹白
  [10000127, 'SUP_ER', '岩'],           // 叶洛亚
  [10000128, 'ATK_C', '风'],            // 法尔伽
  [10000129, 'ATK_C', '冰'],            // 洛恩
  [10000130, 'ATK_C', '岩'],            // 莉奈娅
  [10000131, 'ATK_C', '火'],            // 尼可
  [10000132, 'SUP_ER', '风'],           // 布伦妮
  [10000133, 'ATK_C', '冰'],            // 桑多涅
  [10000148, 'ATK_C', '雷'],            // 阿罗夏
  [10000150, 'ATK_C', '冰'],            // 奥黛塔
]

// 手工维护的角色（完全按指定内容输出，不走模板）
// 芙宁娜：与 DB 现状一致（含"水元素伤害加成"主要属性）
const MANUAL = {
  10000089: {
    subStats: [
      ['FIGHT_PROP_HP_PERCENT', 10, 'primary'],
      ['FIGHT_PROP_CRITICAL', 8, 'primary'],
      ['FIGHT_PROP_CRITICAL_HURT', 8, 'primary'],
      ['FIGHT_PROP_CHARGE_EFFICIENCY', 6, 'secondary'],
      ['FIGHT_PROP_HP', 2, 'secondary'],
      ['FIGHT_PROP_WATER_ADD', 5, 'primary'],
    ],
    mainStats: { sand: '元素充能效率', goblet: '水元素伤害加成', circlet: '暴击率' },
  },
  // 八重神子（原 demo 数据补分级 + 补齐副词条）
  10000058: {
    subStats: [
      ['FIGHT_PROP_ATTACK_PERCENT', 10, 'primary'],
      ['FIGHT_PROP_CRITICAL', 9, 'primary'],
      ['FIGHT_PROP_CRITICAL_HURT', 9, 'primary'],
      ['FIGHT_PROP_CHARGE_EFFICIENCY', 6, 'secondary'],
      ['FIGHT_PROP_ELEMENT_MASTERY', 4, 'secondary'],
      ['FIGHT_PROP_ATTACK', 2, 'secondary'],
    ],
    mainStats: { sand: '攻击力', goblet: '雷元素伤害加成', circlet: '暴击伤害' },
  },
  // 菲林斯（原 demo 数据补分级 + 补齐副词条）
  10000120: {
    subStats: [
      ['FIGHT_PROP_CRITICAL', 10, 'primary'],
      ['FIGHT_PROP_CRITICAL_HURT', 10, 'primary'],
      ['FIGHT_PROP_ATTACK_PERCENT', 8, 'primary'],
      ['FIGHT_PROP_CHARGE_EFFICIENCY', 5, 'secondary'],
      ['FIGHT_PROP_ELEMENT_MASTERY', 4, 'secondary'],
      ['FIGHT_PROP_ATTACK', 2, 'secondary'],
    ],
    mainStats: { sand: '攻击力', goblet: '雷元素伤害加成', circlet: '暴击伤害' },
  },
}

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
