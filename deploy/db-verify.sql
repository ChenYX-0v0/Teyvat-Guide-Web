-- 云上导入后的校验（M3）
--
-- 用法：
--   mysql --host=<内网地址> --user=<用户> -p teyvat_guide < deploy/db-verify.sql
--
-- 判读：
--   · **只报不一致的行**（actual <> expected）。没有任何输出 = 全部对得上 ✓
--   · 期望值基线 = 2026-09-19 本地库快照（与 deploy/out/rows-local.txt 同一份）
--   · 业务类表（users / chat_* / player_profile_snapshots）**不放进断言**：
--     本地导出之后还在跑就会更多，用「≥ 基线」人工看即可（见文件末尾）

-- ---------------------------------------------------------------- 数据类：必须完全一致
-- 这些是"内容/映射"数据，导入后行数不等 = 导出或导入不完整
SELECT tbl, expected, actual, (actual - expected) AS diff FROM (
    SELECT 'ai_prompts'              AS tbl,   5 AS expected, COUNT(*) AS actual FROM ai_prompts
    UNION ALL SELECT 'relic_set_effects',   120, COUNT(*) FROM relic_set_effects
    UNION ALL SELECT 'relic_sets',           65, COUNT(*) FROM relic_sets
    UNION ALL SELECT 'characters',          122, COUNT(*) FROM characters
    UNION ALL SELECT 'character_stat_weights', 122, COUNT(*) FROM character_stat_weights
    UNION ALL SELECT 'weapons',             249, COUNT(*) FROM weapons
    UNION ALL SELECT 'prop_maps',            32, COUNT(*) FROM prop_maps
    UNION ALL SELECT 'fight_props',          15, COUNT(*) FROM fight_props
    UNION ALL SELECT 'relic_slots',           5, COUNT(*) FROM relic_slots
    UNION ALL SELECT 'affixes',             350, COUNT(*) FROM affixes
    UNION ALL SELECT 'text_map_entries',    436, COUNT(*) FROM text_map_entries
    UNION ALL SELECT 'preset_qas',            6, COUNT(*) FROM preset_qas
) AS data_tables
WHERE actual <> expected;

-- ---------------------------------------------------------------- 业务类：只看"没少"
-- 导出是 2026-09-19 的本地快照，之后本地还在用，所以云端行数**可能更多**，但**不该更少**。
SELECT tbl, baseline, actual, (actual - baseline) AS diff FROM (
    SELECT 'users'                     AS tbl,   1 AS baseline, COUNT(*) AS actual FROM users
    UNION ALL SELECT 'chat_sessions',          50, COUNT(*) FROM chat_sessions
    UNION ALL SELECT 'chat_messages',         131, COUNT(*) FROM chat_messages
    UNION ALL SELECT 'player_profile_snapshots', 1, COUNT(*) FROM player_profile_snapshots
    UNION ALL SELECT 'missings',               71, COUNT(*) FROM missings
) AS biz_tables
WHERE actual < baseline;

-- ---------------------------------------------------------------- 内容抽查（可选）
-- 行数对不代表内容对：抽查两条"刚核对过"的套装效果，确认文本没被截断
-- SELECT set_id, piece, CHAR_LENGTH(effect) AS chars, source
-- FROM relic_set_effects WHERE set_id IN (15020, 15041) ORDER BY set_id, piece;
-- 期望：15020/4 = 41 字符、15041/4 ≈ 118 字符，source 均以「联网核对」结尾

-- 提示词版本：应为 5 行（base v1 / rules v2,v3 / fewshot v2,v3），且对外生效版本是 v3
-- SELECT scene_key, version, enabled, CHAR_LENGTH(content) AS chars FROM ai_prompts ORDER BY scene_key, version;
