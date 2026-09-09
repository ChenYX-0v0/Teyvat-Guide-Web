# 04 · API 接口文档

基础路径：`http://localhost:8080/api`（前端通过 Vite 代理使用 `/api`）
统一响应：`{ "code": 0, "message": "ok", "data": ... }`，`code !== 0` 表示业务失败。

## 错误码

| code | HTTP | 含义 |
| --- | --- | --- |
| 0 | 200 | 成功 |
| 400 | 400 | 参数错误 |
| 401 | 401 | 未登录 / 凭证无效 |
| 404 | 404 | 玩家不存在或未公开 |
| 409 | 409 | 用户名已存在 |
| 500 | 500 | 服务内部错误 |
| 502 | 502 | 上游错误 |
| 503 | 503 | 上游限流 / 游戏维护 |

---

## 1. 健康检查

`GET /api/health`

```json
{
  "code": 0,
  "message": "ok",
  "data": { "status": "up", "version": "0.1.0", "time": "2026-09-10T10:00:00+08:00" }
}
```

---

## 2. 对话

`POST /api/chat`

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| sessionId | string | 否 | 为空则创建新会话 |
| message | string | 是 | 用户提问，最长 4000 字符 |
| stream | bool | 否 | 流式输出（预留） |

**响应 data**：`{ sessionId, reply, tokens, elapsedMs }`

---

## 3. 鉴权（已实现）

| 方法 | 路径 | 鉴权 | 说明 |
| --- | --- | --- | --- |
| POST | `/api/auth/register` | 无 | `{username, password, displayName?}`；用户名 3~32 字符，密码 ≥6 位 |
| POST | `/api/auth/login` | 无 | 返回 `{token, displayName}`（JWT，有效期 `auth.jwt_ttl_hours`） |
| GET | `/api/auth/me` | Bearer | 当前用户 `{id, username}` |

注册成功后前端自动调用登录完成会话建立。

**curl 示例**

```powershell
$body = '{"username":"admin","password":"admin123"}'
$r = Invoke-RestMethod -Method Post http://localhost:8080/api/auth/login -ContentType "application/json" -Body $body
$H = @{ Authorization = "Bearer " + $r.data.token }
```

---

## 4. 玩家信息查询（Enka.Network 数据源）

数据来自 [Enka.Network](https://github.com/EnkaNetwork/API-docs)：免登录、免签名、免风控。
名称等映射数据存 MySQL 映射表（见 §6.2），由管理接口维护，代码零硬编码。设计详见 `docs/07-Enka方案设计.md`。

### 4.1 查询玩家档案

`GET /api/genshin/player?uid=197794747[&refresh=true]`

| 参数 | 必填 | 说明 |
| --- | --- | --- |
| uid | 条件 | 9 位数字；**携带 Bearer Token 时可省略**，自动使用当前用户绑定的 UID（见 4.3） |
| refresh | 否 | `true` 强制绕过本地快照直接查上游并更新快照 |

**响应 data（PlayerProfile）**

| 字段 | 说明 |
| --- | --- |
| uid / nickname / level / signature / worldLevel / achievements / avatarUrl | 玩家基础信息 |
| characters[] | 展柜角色（≤8）：name / element / rarity / level / constellation / fetter / iconUrl |
| characters[].stats[] | 面板属性 `{name, value, tier?}`；tier=`primary`/`secondary`（推荐属性，前端金色），空为未分级（灰色） |
| characters[].statWeights | 推荐强化词条（来源 asset_character_stat_weights，见 6.2） |
| characters[].weapon | `{name, rarity, level, affix}` |
| characters[].relics[] | 圣遗物：name / set / rarity / level / pos / mainStat / mainStatTier / subStats[]{name,value,count,tier} |
| warnings | 翻译缺漏提示（前端当前不展示，明细仍落 asset_missings 表） |

**错误映射**：404 玩家不存在或未公开；503 游戏维护 / 上游限流；502 其他上游错误。
**缓存**：本地快照优先（`asset_player_profile_snapshots` 表，uid 主键）；负缓存 5min 防扫号穿透。

### 4.2 档案快照

`GET /api/genshin/player-snapshot?uid=xxx` —— 读取最近一次查询落库的快照，不发起上游查询。

### 4.3 账号 ↔ UID 绑定（需 Bearer Token，一对一）

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| POST | `/api/genshin/binding` | 绑定 `{uid, server?}`（server 默认 `cn_gf01`）；**绑定前实查 Enka 验证 UID 存在且档案公开** |
| GET | `/api/genshin/binding` | 查询 `{bound, uid?, server?}` |
| DELETE | `/api/genshin/binding` | 解绑 |

---

## 5. 图鉴检索（规划中）

`GET /api/wiki?q=钟离`

---

## 6. 资产映射管理（后台管理系统核心）

### 6.1 接口（X-Admin-Key 鉴权）

请求头：`X-Admin-Key: <config.yaml 的 auth.jwt_secret 值>`

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| POST | `/api/admin/assets/import` | 批量 upsert 映射，成功后自动刷新内存 |
| GET | `/api/admin/assets/missing?limit=100` | 翻译缺漏清单 `{items, total}`（按 hit_count 倒序），补录依据 |
| POST | `/api/admin/assets/reload` | 手动刷新内存映射 |

> CLI 批量导入：`go run ./cmd/assetimport -dir <EnkaGameAssets store/gi 目录>`，或分表导入
> `-textmap-file` / `-affixes-file` / `-fight-props-file` / `-stat-weights-file`（种子文件见 `cmd/assetimport/*.json`）。

### 6.2 ImportPayload 与数据表（字段全部可选，按表分组 upsert）

| 载荷字段 | 数据表 | 结构 |
| --- | --- | --- |
| characters[] | asset_characters | `{id(avatarId), name, enName, element, rarity, icon}` |
| weapons[] | asset_weapons | `{id, name, enName, rarity, type, icon}` |
| relicSets[] | asset_relic_sets | `{setId, name, icon}` |
| textMap[] | asset_text_map_entries | `{hash, text}`（TextMapCHS：hash → 中文） |
| propMap[] | asset_prop_maps | `{propId, name, percent}` 词条 ID → 中文名；percent 决定数值格式 |
| fightProps[] | asset_fight_props | `{numId, propId}` Enka fightPropMap 数字枚举 → FIGHT_PROP_XXX |
| affixes[] | asset_affixes | `{id, propType, propValue}` 圣遗物词缀表（解码强化次数） |
| statWeights[] | asset_character_stat_weights | `{characterId, recommendation}`，JSON payload：`{subStats:[{statKey,weight,tier?}], mainStats:{sand,goblet,circlet}}`；tier=`primary`/`secondary` |

**tier（属性分级）说明**：`primary`/`secondary` 会透传到面板属性（stats[].tier）与圣遗物词条（relics[].mainStatTier / subStats[].tier），前端据此着色；未分级为空。修改 payload 后需 reload 或重启生效，并清空 `asset_player_profile_snapshots`（旧快照不含新翻译）。

**请求示例**

```json
{
  "propMap": [{ "propId": "FIGHT_PROP_HP_PERCENT", "name": "百分比生命值", "percent": true }],
  "fightProps": [{ "numId": "45", "propId": "FIGHT_PROP_WATER_ADD" }],
  "statWeights": [{
    "characterId": 10000089,
    "recommendation": {
      "subStats": [
        { "statKey": "FIGHT_PROP_HP_PERCENT", "weight": 10, "tier": "primary" },
        { "statKey": "FIGHT_PROP_CRITICAL", "weight": 8, "tier": "primary" }
      ],
      "mainStats": { "sand": "元素充能效率", "goblet": "水元素伤害加成", "circlet": "暴击率" }
    }
  }]
}
```

**响应**：`{ "characters": n, "weapons": n, "relicSets": n, "textMap": n, "propMap": n }`

---

## 7. 业务数据表总览（teyvat_guide 库）

| 表 | 用途 |
| --- | --- |
| users | 用户（username / password_hash / display_name / genshin_uid / genshin_server） |
| chat_sessions / chat_messages | 对话会话与消息 |
| asset_player_profile_snapshots | 玩家档案快照缓存（uid 主键，JSON） |
| asset_characters / asset_weapons / asset_relic_sets | 角色 / 武器 / 套装映射 |
| asset_text_map_entries | 文本 hash → 中文 |
| asset_prop_maps / asset_fight_props / asset_affixes | 词条名 / 数字枚举 / 词缀映射 |
| asset_character_stat_weights | 角色推荐词条（JSON payload，含 tier 分级） |
| asset_missings | 翻译缺漏收集（补录依据） |
