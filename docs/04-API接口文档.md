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
| 502 | 502 | 上游错误 |
| 503 | 503 | 上游限流 / 游戏维护 |
| 500 | 500 | 服务内部错误 |

---

## 1. 健康检查

`GET /api/health`

**响应示例**

```json
{
  "code": 0,
  "message": "ok",
  "data": { "status": "up", "version": "0.1.0", "time": "2026-09-07T10:00:00+08:00" }
}
```

---

## 2. 发起对话

`POST /api/chat`

**请求体**

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| sessionId | string | 否 | 为空则创建新会话 |
| message | string | 是 | 用户提问，最长 4000 字符 |
| stream | bool | 否 | 流式输出（暂未实现，预留） |

```json
{ "sessionId": "", "message": "万叶值得抽吗？" }
```

**响应体**

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| sessionId | string | 会话 ID，前端需保存用于多轮对话 |
| reply | string | 助手回复 |
| tokens | int | 消耗 token 数（mock 模式为字符数） |
| elapsedMs | int64 | 处理耗时（毫秒） |

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "sessionId": "0f1c...",
    "reply": "万叶是强力的增伤辅助……",
    "tokens": 128,
    "elapsedMs": 742
  }
}
```

**curl**

```powershell
curl -X POST http://localhost:8080/api/chat -H "Content-Type: application/json" -d '{\"message\":\"新手开局怎么玩？\"}'
```

---

## 3. 玩家信息查询（Enka.Network 数据源）

通过 UID 查询玩家档案。数据来自 [Enka.Network](https://github.com/EnkaNetwork/API-docs)：**免登录、免签名、免风控**。
名称等映射数据存于 MySQL（`characters` / `weapons` / `relic_sets` / `text_map_entries` / `prop_maps` 表），由管理接口维护，代码零硬编码。设计详见 `docs/07-Enka方案设计.md`。

### 3.1 查询玩家档案

`GET /api/genshin/player?uid=197794747`

| 参数 | 必填 | 说明 |
| --- | --- | --- |
| uid | 是 | 9 位数字 |

**能力边界**：昵称 / 等级 / 签名 / 世界等级 / 成就数 / **展柜角色**（≤8，含武器、圣遗物、命座、好感）。
不含深渊战绩、探索度、全角色列表（米游社独有数据，已随米游社方案放弃）。

**错误映射**：404 玩家不存在或未公开；503 游戏维护 / 上游限流；502 其他上游错误。

**缓存**：正常 60s；"不存在"负缓存 5min（防扫号穿透）。

### 3.2 映射数据管理（X-Admin-Key 鉴权）

请求头：`X-Admin-Key: <auth.jwt_secret 配置值>`

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| POST | `/api/admin/assets/import` | 批量 upsert 映射（characters / weapons / relicSets / textMap / propMap），成功后自动刷新内存 |
| GET | `/api/admin/assets/missing?limit=100` | 翻译缺漏清单（按命中次数倒序），补录依据 |
| POST | `/api/admin/assets/reload` | 手动刷新内存映射 |

初始导入：`go run ./cmd/assetimport -dir <EnkaGameAssets store/gi 目录>`
（已导入：152 角色 / 249 武器 / 65 套装 / 436 文本；词条表见 `cmd/assetimport/props_zh.json`。）

> 圣遗物**件名** hash 不在当前 locs 数据中，会显示"未映射名称 #xxx"；缺失项已自动收集到 `missings` 表，补充文本映射后即恢复。

## 4. 图鉴检索（规划中）

`GET /api/wiki?q=钟离`

```json
{ "code": 0, "message": "ok", "data": { "items": [], "total": 0 } }
```

## 5. 鉴权接口（规划中）

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| POST | `/api/auth/login` | 登录，返回 JWT |
| GET | `/api/auth/me` | 当前用户信息，需 `Authorization: Bearer <token>` |

需要保护的路由在 `router.go` 中使用 `middleware.JWTAuth(cfg.Auth)` 包裹即可。
