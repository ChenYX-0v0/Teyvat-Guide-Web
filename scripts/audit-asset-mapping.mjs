// 资产映射体检（只读）—— 14 张表是否"合理"，不只是行数对不对。
//
// 用法：node scripts/audit-asset-mapping.mjs [--guides]
//   --guides 额外核对「角色表 ↔ 攻略页」对齐（阶段② 攻略卡的前置，见 teyvat-docs/ai/07 TODO-18）
//
// 连接：优先读环境变量 TEYVAT_DB_*（HOST/PORT/USER/PASSWORD/NAME），
//       否则从 server/config/config.yaml 的 database 段解析；密码**只读不打印** ✗。
// 参照系：teyvat-docs/shared/02-资产映射管理（AS-IS）.md §3 的表清单与 2026-10-12 实测行数 ✓。
import fs from 'node:fs'
import path from 'node:path'
import mysql from 'file:///C:/Users/HONOR/Desktop/demo_in_windows/demo-app/node_modules/mysql2/promise.js'

// ---------------------------------------------------------------- 连接
function conn() {
  const env = process.env
  if (env.TEYVAT_DB_PASSWORD) {
    return { host: env.TEYVAT_DB_HOST ?? '127.0.0.1', port: Number(env.TEYVAT_DB_PORT ?? 13306), user: env.TEYVAT_DB_USER ?? 'teyvat', password: env.TEYVAT_DB_PASSWORD, database: env.TEYVAT_DB_NAME ?? 'teyvat_guide' }
  }
  const cfg = path.resolve('server/config/config.yaml')
  if (!fs.existsSync(cfg)) { console.error('没有 TEYVAT_DB_PASSWORD，也找不到 ' + cfg); process.exit(1) }
  const block = fs.readFileSync(cfg, 'utf8').match(/database:[\s\S]*?(?=\n[a-z_]+:|$)/i)?.[0] ?? ''
  const get = (k) => block.match(new RegExp('^\\s*' + k + ':\\s*(\\S+)', 'mi'))?.[1]
  return { host: get('host') ?? '127.0.0.1', port: Number(get('port') ?? 13306), user: get('user') ?? 'teyvat', password: get('password') ?? '', database: get('name') ?? 'teyvat_guide' }
}

const db = await mysql.createConnection({ ...conn(), charset: 'utf8mb4' })
const q = async (sql, args) => (await db.query(sql, args))[0]
const line = (t) => console.log('\n== ' + t + ' ==')
const flag = (ok) => (ok ? '✓' : '✗')

// ---------------------------------------------------------------- A. 行数
line('A) 行数与参照（shared/02 §3 的 2026-10-12 实测）')
const EXPECT = {
  characters: 124, weapons: 255, relic_sets: 65, relic_set_effects: 128, relic_slots: 5,
  text_map_entries: 498, prop_maps: 24, fight_props: 15, affixes: 350,
  character_stat_weights: 124, character_relic_weights: 124, character_relic_weights_mp: 152,
  character_relic_weight_rules: 9, missings: 96,
}
for (const [t, want] of Object.entries(EXPECT)) {
  const [r] = await q('SELECT COUNT(*) c FROM `' + t + '`')
  const ok = r.c === want
  console.log(`   ${t.padEnd(30)} ${String(r.c).padStart(4)}  参照 ${String(want).padStart(4)}  ${flag(ok)}${ok ? '' : '  ← 变了（合理也可能是新增，需看下面）'}`)
}

// ---------------------------------------------------------------- B. 角色
line('B) characters 质量')
const [charBad] = await q(`SELECT
   SUM(name='' OR name IS NULL) empty_name,
   SUM(name REGEXP '^[0-9]+$') digit_name,
   SUM(name REGEXP '[A-Za-z]') has_latin,
   SUM(en_name='' OR en_name IS NULL) empty_en,
   SUM(icon='' OR icon IS NULL) empty_icon,
   SUM(rarity NOT IN (4,5)) bad_rarity,
   SUM(element NOT IN ('火','水','雷','冰','风','岩','草')) bad_element,
   COUNT(*) total FROM characters`)
console.log('  ', charBad)
const [dupChar] = await q("SELECT name, COUNT(*) c FROM characters GROUP BY name HAVING c>1")
console.log('   重名角色：', dupChar.length ? dupChar : '无 ✓')
console.log('   元素分布：', (await q("SELECT element, COUNT(*) c FROM characters GROUP BY element ORDER BY c DESC")).map((r) => `${r.element}${r.c}`).join(' '))
console.log('   稀有度：', (await q('SELECT rarity, COUNT(*) c FROM characters GROUP BY rarity')).map((r) => `${r.rarity}星×${r.c}`).join(' '))

// ---------------------------------------------------------------- C. 武器 / 部位
line('C) weapons / relic_slots 质量')
const [wpnBad] = await q(`SELECT SUM(name='') empty_name, SUM(rarity NOT BETWEEN 1 AND 5) bad_rarity,
   SUM(type='') empty_type, SUM(icon='') empty_icon, COUNT(*) total FROM weapons`)
console.log('  ', wpnBad)
console.log('   类型分布：', (await q('SELECT type, COUNT(*) c FROM weapons GROUP BY type ORDER BY c DESC')).map((r) => `${r.type}${r.c}`).join(' '))
const dupWpn = await q('SELECT name, COUNT(*) c FROM weapons GROUP BY name HAVING c>1')
console.log('   重名武器：', dupWpn.length ? dupWpn.slice(0, 5) : '无 ✓')
console.log('   部位：', (await q('SELECT pos, name FROM relic_slots ORDER BY pos')).map((r) => `${r.name}`).join(' / '))

// ---------------------------------------------------------------- D. 套装效果
line('D) 套装效果覆盖（65 套 × 2/4 件 ⇒ 期望 130 行）')
const noEffect = await q('SELECT s.set_id, s.name FROM relic_sets s LEFT JOIN relic_set_effects e ON e.set_id=s.set_id GROUP BY s.set_id, s.name HAVING COUNT(e.piece)=0')
const missPiece = await q(`SELECT s.set_id, s.name, GROUP_CONCAT(e.piece ORDER BY e.piece) pieces
   FROM relic_sets s LEFT JOIN relic_set_effects e ON e.set_id=s.set_id
   GROUP BY s.set_id, s.name HAVING COUNT(e.piece) < 2`)
console.log('   完全没有效果的套装：', noEffect.length ? noEffect : '无 ✓')
console.log('   缺 2 或 4 件的套装：', missPiece.length ? missPiece : '无 ✓')
console.log('   效果文本可疑（空/过短/占位）：', (await q("SELECT set_id, piece, LEFT(effect,24) t FROM relic_set_effects WHERE effect='' OR CHAR_LENGTH(effect)<8 OR effect REGEXP '待补|TODO|暂无'")).length ? '有 ✗（见下）' : '无 ✓')
for (const r of await q("SELECT set_id, piece, LEFT(effect,24) t FROM relic_set_effects WHERE effect='' OR CHAR_LENGTH(effect)<8 OR effect REGEXP '待补|TODO|暂无'")) console.log('     ', r)
console.log('   Source 分布（人工整理状态）：', (await q('SELECT source, COUNT(*) c FROM relic_set_effects GROUP BY source')).map((r) => `[${r.source || '空'}]×${r.c}`).join(' '))

// ---------------------------------------------------------------- E. 推荐词条
line('E) character_stat_weights（推荐词条）')
const [swOk] = await q(`SELECT COUNT(*) total,
   SUM(payload IS NULL OR payload='') empty_payload,
   SUM(JSON_EXTRACT(payload,'$.score') IS NOT NULL) legacy_score,
   SUM(JSON_EXTRACT(payload,'$.mainStats.sand') IS NULL) no_sand,
   SUM(JSON_EXTRACT(payload,'$.mainStats.goblet') IS NULL) no_goblet,
   SUM(JSON_EXTRACT(payload,'$.mainStats.circlet') IS NULL) no_circlet,
   SUM(JSON_LENGTH(JSON_EXTRACT(payload,'$.subStats')) = 0) no_subs
   FROM character_stat_weights`)
console.log('  ', swOk, '  （legacy_score 应=0 ✓ 旧的独立评分块已迁走）')
console.log('   孤儿行（character_id 不在 characters）：', (await q('SELECT w.character_id FROM character_stat_weights w LEFT JOIN characters c ON c.id=w.character_id WHERE c.id IS NULL')).length ? '有 ✗' : '无 ✓')
const noSw = await q('SELECT c.id, c.name FROM characters c LEFT JOIN character_stat_weights w ON w.character_id=c.id WHERE w.character_id IS NULL')
console.log('   **有角色但无推荐词条**：', noSw.length ? noSw : '无 ✓')
console.log('   主词条取值域：')
for (const slot of ['sand', 'goblet', 'circlet']) {
  const rows = await q(`SELECT JSON_UNQUOTE(JSON_EXTRACT(payload,'$.mainStats.${slot}')) v, COUNT(*) c
     FROM character_stat_weights GROUP BY v ORDER BY c DESC`)
  console.log(`     ${slot.padEnd(8)} ` + rows.map((r) => `${r.v}(${r.c})`).join(' '))
}
const badGoblet = await q(`SELECT c.name, JSON_UNQUOTE(JSON_EXTRACT(w.payload,'$.mainStats.goblet')) g
   FROM character_stat_weights w JOIN characters c ON c.id=w.character_id
   WHERE JSON_UNQUOTE(JSON_EXTRACT(w.payload,'$.mainStats.goblet')) REGEXP '治疗|充能'`)
console.log('   ⚠️ 杯 = 治疗/充能（不合理）：', badGoblet.length ? badGoblet : '无 ✓')
const badCirclet = await q(`SELECT c.name, JSON_UNQUOTE(JSON_EXTRACT(w.payload,'$.mainStats.circlet')) g
   FROM character_stat_weights w JOIN characters c ON c.id=w.character_id
   WHERE JSON_UNQUOTE(JSON_EXTRACT(w.payload,'$.mainStats.circlet')) REGEXP '元素伤害|物理伤害|充能'`)
console.log('   ⚠️ 冠 = 元素伤/物伤/充能（不合理）：', badCirclet.length ? badCirclet : '无 ✓')

// ---------------------------------------------------------------- F. 评分权重
line('F) 评分权重（V3 character_relic_weights / mp 兜底 / 规则）')
const [w3] = await q(`SELECT COUNT(*) total,
   SUM(hp+atk+def+cpct+cdmg+mastery+dmg+phy+recharge+heal = 0) all_zero,
   SUM(GREATEST(hp,atk,def,cpct,cdmg,mastery,dmg,phy,recharge,heal) > 100) over100,
   SUM(name = '' OR name IS NULL) empty_name,
   SUM(source IS NULL OR source = '') no_source, SUM(fetched_at IS NULL OR fetched_at='') no_fetched
   FROM character_relic_weights`)
console.log('  ', w3, '  （all_zero 应为 0 ✗；over100 应 0 ✗）')
const nameMis = await q(`SELECT COUNT(*) c FROM character_relic_weights w JOIN characters c2 ON c2.id=w.character_id
   WHERE w.name <> '' AND w.name <> c2.name`)
console.log('   权重表角色名与 characters 不一致：', nameMis[0].c, flag(nameMis[0].c === 0))
console.log('   V3 缺哪些角色：', (await q('SELECT c.name FROM characters c LEFT JOIN character_relic_weights w ON w.character_id=c.id WHERE w.character_id IS NULL')).map((r) => r.name).join('、') || '无 ✓')
console.log('   mp 兜底表覆盖：', (await q('SELECT COUNT(DISTINCT character_id) c FROM character_relic_weights_mp'))[0].c, '个角色')
console.log('   规则表（已停用，仅留回滚）：', (await q('SELECT COUNT(*) c FROM character_relic_weight_rules'))[0].c, '条；按 2026-09-27 口径**不参与算分** ✓')

// ---------------------------------------------------------------- G. 映射链路完整性
line('G) 映射链路（affixes / fight_props → prop_maps）')
console.log('   affixes.prop_type 不在 prop_maps 里：', (await q('SELECT DISTINCT a.prop_type FROM affixes a LEFT JOIN prop_maps p ON p.prop_id=a.prop_type WHERE p.prop_id IS NULL')).map((r) => r.prop_type).join('、') || '无 ✓')
console.log('   fight_props.prop_id 不在 prop_maps 里：', (await q('SELECT DISTINCT f.prop_id FROM fight_props f LEFT JOIN prop_maps p ON p.prop_id=f.prop_id WHERE p.prop_id IS NULL')).map((r) => r.prop_id).join('、') || '无 ✓')
console.log('   prop_maps 空名/空 id：', (await q("SELECT prop_id, name FROM prop_maps WHERE name='' OR prop_id=''")).length ? '有 ✗' : '无 ✓')
console.log('   Enka 常用枚举抽查（数字 id → PropID）：')
const KEY = { 2000: 'HP', 2001: '攻击', 2002: '防御', 20: '暴击率', 22: '暴击伤害', 23: '充能', 28: '精通', 30: '物理伤害', 40: '火伤', 41: '雷伤', 42: '水伤', 43: '草伤', 44: '风伤', 45: '岩伤', 46: '冰伤', 26: '治疗' }
for (const [num, want] of Object.entries(KEY)) {
  const r = (await q('SELECT f.prop_id, p.name FROM fight_props f LEFT JOIN prop_maps p ON p.prop_id=f.prop_id WHERE f.num_id=?', [num]))[0]
  console.log(`     ${String(num).padStart(4)} → ${(r?.prop_id ?? '缺失 ✗').padEnd(32)} ${r?.name ?? ''}   （期望含「${want}」）`)
}

// ---------------------------------------------------------------- H. 缺漏
line('H) missings（翻译层未命中）')
console.log('   按 kind：', (await q('SELECT kind, COUNT(*) c, SUM(hit_count) hits FROM missings GROUP BY kind')).map((r) => `${r.kind}×${r.c}(命中${r.hits})`).join(' '))
for (const r of await q('SELECT kind, asset_id, hit_count FROM missings ORDER BY hit_count DESC LIMIT 5')) console.log('     Top：', r)

// ---------------------------------------------------------------- I. 攻略页对齐（阶段②）
if (process.argv.includes('--guides')) {
  line('I) 角色表 ↔ 攻略页对齐（TODO-18 前置）')
  const MAN = path.resolve('scripts/cache/guides.manifest.json')
  if (!fs.existsSync(MAN)) { console.log('   没有 manifest ✗ 先跑 scripts/fetch-character-guides.mjs') }
  else {
    const titles = Object.keys(JSON.parse(fs.readFileSync(MAN, 'utf8')).pages).map((t) => t.replace(/\/攻略$/, ''))
    const chars = (await q('SELECT id, name FROM characters ORDER BY id')).map((r) => r.name)
    const tset = new Set(titles), cset = new Set(chars)
    const noGuide = chars.filter((n) => !tset.has(n))
    const noChar = titles.filter((n) => !cset.has(n))
    console.log(`   攻略页 ${titles.length} 页 ／ 角色表 ${chars.length} 位`)
    console.log('   有角色但**没有攻略页**：', noGuide.length ? noGuide.join('、') : '无 ✓')
    console.log('   有攻略页但**角色表里没有**：', noChar.length ? noChar.join('、') : '无 ✓')
  }
}

// ---------------------------------------------------------------- J. 明细
line('J) 明细（上面各节发现的具体条目）')
console.log('   评分权重表里名字与 characters 不一致的：')
for (const r of await q(`SELECT w.character_id, w.name AS wn, c.name AS cn FROM character_relic_weights w
   JOIN characters c ON c.id = w.character_id WHERE w.name <> '' AND w.name <> c.name`)) {
  console.log(`     id=${r.character_id}  权重表="${r.wn}"  characters="${r.cn}"`)
}
console.log('   missings 里 kind=prop 的具体条目：')
for (const r of await q("SELECT asset_id, hit_count, first_seen FROM missings WHERE kind='prop' ORDER BY hit_count DESC")) {
  console.log(`     prop_id=${r.asset_id}  命中 ${r.hit_count} 次  首次 ${new Date(r.first_seen).toISOString().slice(0, 10)}`)
}
console.log('   affixes 里那些"没有中文名"的属性各有多少条：')
for (const r of await q(`SELECT a.prop_type, COUNT(*) c FROM affixes a LEFT JOIN prop_maps p ON p.prop_id = a.prop_type
   WHERE p.prop_id IS NULL GROUP BY a.prop_type ORDER BY c DESC`)) {
  console.log(`     ${r.prop_type.padEnd(40)} ${r.c} 条`)
}

await db.end()
