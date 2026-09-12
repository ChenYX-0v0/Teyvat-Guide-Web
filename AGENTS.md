# AGENTS.md — AI 协作开发规范

> 本文件面向 AI 编码代理与人类协作者，描述本项目的架构、约定与"必须/禁止"事项。
> 修改任何代码前请先阅读本文；与本文冲突的旧代码不代表规范本身，遇到不一致请遵循本文。

> **本地开发调试例外条款（2026-09-12 约定）**
> 在**本机开发调试期间**，AI 代理/协作者**可以直接读取、使用并写入本地密钥**——包括 `.env`（本项目为 `server/.env`、`web/.env`）与 `server/config/config.yaml` 里的 `auth.jwt_secret` / `auth.admin_key` / `auth.internal_key` / `ai.api_key`（`X-AI-Key`）等——用于：启动服务、跑测试、带密钥调用本机接口做联调（如带 `X-Internal-Key` 调 `/internal/v1/admin/*`、带 `X-Admin-Key` 调 `/api/admin/*`）、把两端同名密钥对齐。
> **以下红线不变，任何情况下不得违反**：
> 1. **不入仓**——密钥不得出现在代码、测试、文档、提交信息里；`.env.example` 只留空值；提交前确认没有真实密钥（`config.yaml` 已 gitignore，不要取消）。
> 2. **不回显**——不在聊天回复、日志、注释里输出密钥原文；可以说明「已读取 `X-Internal-Key` 用于本地调用」，但不要打印值。
> 3. **只对本机**——只用于 `localhost` / `127.0.0.1` 或本机调试环境；禁止用于公网/生产地址，禁止对生产数据做写操作。
> 4. **不擅自换钥**——除本地两端对齐外，修改密钥值、重新生成、覆盖线上配置**仍需明确确认**（会影响正在运行的服务与其他项目）。
> 5. **付费 Key 节制**——`ai.api_key` 背后是 DeepSeek 按量计费，可用于联调，但不得循环调用、压测或批量刷量。
>
> 生产环境（非本机）维持原规则：密钥只从环境变量注入，不落盘、不入仓、不打日志。

## 0. 文档入口（共享文档仓）

本文档与 API 规范等**不在本仓**，而在三项目共享文档仓 `teyvat-docs`（远程 `git@github.com:ChenYX-0v0/Teyvat-Guide-Docs.git`），通过 **Junction 目录链接**挂载。三个项目访问到的 `docs/` 是**同一份文件**（哈希一致），改一处三处同步。

### 0.1 布局与挂载关系

| 项目 | 项目内路径 | 实际位置 | 内容 |
| --- | --- | --- | --- |
| 主服务（本仓） | `docs/` | `teyvat-docs\web` | 本项目文档（架构 / 前后端规范 / API 文档 / Enka 方案 / AI 规格 / 待办） |
| 主服务（本仓） | `shared-docs/` | `teyvat-docs\shared` | **跨项目契约**（接口契约 / 主服务对接说明 / 用户体系与鉴权边界设计） |
| 后台 demo-app | `docs/` | `teyvat-docs\admin` | 后台运维系统文档 |
| 后台 demo-app | `aidoc/` | `teyvat-docs\ai` | AI 服务文档 |
| AI 派蒙服务 | `doc/` | `teyvat-docs\ai` | AI 服务文档 |
| 三个项目 | `shared-docs/` | `teyvat-docs\shared` | 共用契约（同源） |

编号只在各自目录内唯一（`web/01` 与 `admin/01` 互不冲突），**交叉引用必须写明路径**（如 `shared-docs/03-主服务对接说明.md`）。

### 0.2 读 / 改 / 提交（标准操作）

```powershell
# 读：任一项目的链接路径都能打开，内容同源，无需特殊操作
# 改：建议在共享仓内编辑（从项目 docs/ 路径编辑也完全等效——同一份文件）

cd C:\Users\HONOR\Desktop\teyvat-docs
git add -A
git commit -m "docs: 说明本次改动"
git push
```

- 提交**只在 `teyvat-docs` 发生**，三个项目仓不产生文档改动。

### 0.3 规矩（必须遵守）

1. 文档**只在共享仓 `C:\Users\HONOR\Desktop\teyvat-docs` 修改并提交**；项目内的 `docs/`、`shared-docs/` 是链接，禁止在本仓提交文档内容（已在 `.gitignore` 忽略）。
2. 禁止在项目内另建文档副本 / 快照（历史上 `admin/docs/teyvatGuide/` 就是副本，已清理）。
3. 涉及跨项目契约（`shared-docs/`）的改动，必须在提交信息中说明影响面，并同步检查另外两个项目（admin / ai-service）。
4. Junction 不进 git：新机器 / 新克隆后运行 `teyvat-docs\setup-docs.ps1` 重建链接（7 个链接）。
5. **项目仓 `git status` 必须看不到任何 `docs/` 项**——出现即为 `.gitignore` 忽略规则被破坏（见第 7 条），这是最快的一致性自检信号。
6. 禁止在项目仓执行 `git clean -fd`，禁止直接删除项目内的 `docs/`、`shared-docs/`、`doc/`、`aidoc/` 目录——Junction 删除会**连带清空真源内容**，等于删掉共享文档。
7. 禁止删除三个项目 `.gitignore` 中的 `/docs/` `/shared-docs/` `/doc/` `/aidoc/` 忽略规则，否则 git 会顺着链接把共享文档当项目文件收录。
8. `setup-docs.ps1` 必须存为 **UTF-8 with BOM**（PowerShell 5.1 否则按 GBK 读，中文路径报 `Unexpected token`）；**`.yaml` 反之绝对不能带 BOM**（见 §7.14）。

## 1. 项目概览

「AI 提瓦特向导」— 《原神》主题智能助手，前后端分离：

| 模块 | 路径 | 技术栈 | 端口 |
| --- | --- | --- | --- |
| 前端 | `web/` | React 19 + TypeScript + Vite 7 + Tailwind CSS 4 + Zustand + React Router 7 | 5173 |
| 后端 | `server/` | Go + Gin + GORM + SQLite(pure-go) + Viper + Zap + JWT + go-openai | 8080 |

- Go module 名：`teyvat-guide/server`
- 前端通过 Vite 代理将 `/api` 转发到 `http://127.0.0.1:8080`
- 文档在 `docs/`，根 `README.md` 为入口

## 2. 常用命令

```powershell
# 前端（在 web/ 下）
npm run dev        # 开发服务器 :5173
npm run build      # tsc -b && vite build，提交前必须通过
npm run lint       # eslint，提交前必须通过
npm run preview    # 预览生产构建

# 后端（在 server/ 下）
go run ./main.go        # 启动 :8080
go build ./...          # 编译检查，提交前必须通过
go vet ./...            # 静态检查，提交前必须通过
go test ./...           # 测试
go mod tidy             # 依赖变更后执行
```

测试基线：`/api/health` 返回 `{"code":0,...}` 即后端可用。

## 3. 目录结构与分层

```
web/src/
├─ api/            # 请求层：client.ts(axios实例+拦截器) + 各领域文件
├─ components/     # common(通用) / layout(布局) / <feature>(业务)
├─ pages/          # 路由页面，一页一文件，命名 XxxPage.tsx
├─ router/         # 路由表 index.tsx
├─ store/          # Zustand，useXxxStore.ts
├─ types/          # 全局类型
└─ index.css       # Tailwind 4 主题（@theme 定义颜色变量）

server/
├─ main.go                 # 入口：config → logger → database → router → 优雅退出
├─ config/config.yaml      # 配置文件
└─ internal/
   ├─ config/    ├─ router/     ├─ handler/   ├─ service/
   ├─ repository/├─ model/      ├─ dto/       ├─ middleware/
   └─ pkg/       # logger / response / database 基础设施
```

**Go 依赖方向严格单向**：`router → handler → service → repository → model`。

- Handler 禁止直接调用 repository；Service 禁止感知 HTTP（只收 `context.Context` 与领域参数）；`pkg/*` 禁止依赖业务包。
- React 组件禁止直接 `axios`，一律走 `api/` 层。

## 4. 后端规范（Go）

### 4.1 统一响应

所有接口返回信封，`code === 0` 为成功：

```go
response.OK(c, data)                      // {code:0, message:"ok", data}
response.BadRequest(c, "参数错误")          // 400
response.Internal(c, "服务开小差了")        // 500
response.Fail(c, status, code, msg)       // 自定义
```

DTO 定义在 `internal/dto/`，禁止把 GORM model 直接作为出参返回。

### 4.2 错误与日志

- Service 返回 `error`，由 Handler 决定 HTTP 状态码；不向客户端暴露内部细节。
- 日志统一用 `logger.L()`（`*zap.SugaredLogger`）：

```go
logger.L().Infow("chat finished", "session", id, "cost", d)
logger.L().Errorw("chat failed", "err", err)
```

- 禁止在日志打印密钥或完整用户内容；禁止在 handler 中 `panic`。

### 4.3 配置

优先级：环境变量（前缀 `TEYVAT_`，`.` → `_`，如 `TEYVAT_AI_API_KEY`）> `config/config.yaml` > 代码默认值。
新增配置项必须：`config.go` 加结构体字段 + `setDefaults()` 给默认值 + `config.yaml` 同步 + 更新 `.env.example`。

### 4.4 数据库

- **所有业务数据一律存数据库，禁止在代码中硬编码记录数据**（如映射表、枚举文案、词条分级、配置常量等）；代码只定义结构与逻辑，数据通过表或导入接口维护。
- 默认 **MySQL**：`127.0.0.1:3306`，`root/root`，库名 `teyvat_guide`；启动时自动 `CREATE DATABASE IF NOT EXISTS`（utf8mb4）并执行 `AutoMigrate`。
- 切库只改 `config.yaml` 的 `database.*`（`driver` 支持 `mysql` / `sqlite`），业务代码不得感知具体数据库。
- 表结构变更：在 `router.go` 的 `autoMigrate()` 注册新模型。
- Repository 方法保持 `r.db() == nil` 时返回 nil 的降级约定（未启用数据库仍可运行）。

### 4.5 新增接口五步法

1. `dto/xxx.go` 出入参 → 2. `model/xxx.go` 表结构（如需）→ 3. `repository/xxx.go` 数据访问 → 4. `service/xxx.go` 业务 → 5. `handler/xxx.go` + `router.go` 注册。

### 4.6 AI 能力

统一走 `service/ai.go` 的 `AIClient` 接口（`Complete` 整段补全 / `CompleteStream` 流式补全）。现有实现：`mock`（无密钥兜底）、`openai`（直连兼容协议）、`remote`（**默认，对接独立 AI 派蒙服务**，见 `docs/08`）。新增供应商实现该接口并在 `NewAIClient` 注册；禁止在其他地方直接引用模型 SDK。

- remote 模式配置：`ai.base_url` 指向 AI 服务（默认 `http://127.0.0.1:8090`），`ai.api_key` 为 AI 服务密钥（`X-AI-Key`，仅走环境变量 `TEYVAT_AI_API_KEY`），`ai.timeout_seconds` 必须大于 AI 服务自身超时。
- **AI 调用失败必须降级**：`ChatService.Ask` / `AskStream` 捕获错误后记 `Errorw` 并返回兜底文案 + 落库，禁止把上游错误透传给前端（见 `chat.go` 的 `aiDegradeReply`）。
- **流式（SSE）**：`POST /api/chat` 的 `stream=true` 由 `handler/chat.go` 写成 SSE（`session` → `delta*` → `done|error`，契约见 `docs/04-API接口文档.md` §2.2）；`remote` 实现逐帧解析 AI 服务的 `/v1/complete/stream`（`parseSSE`）。两条硬约束：
  - **流式下 `ai.timeout_seconds` 是"空闲超时"**（`withIdleTimeout`）——整段回答不设整体超时，否则长回答会被 `http.Client.Timeout` 掐断；
  - 前端必须用 `fetch` + `ReadableStream`（axios 读不了浏览器流），走 `api/chat.ts` 的 `stream()`，禁止在组件里直接 `fetch`。
- **历史组装只允许走 `buildHistory`**（`service/chat.go`）：20 条上限 + 1200 tokens 预算，**丢弃尾部与本次提问相同的 user 消息**（避免同一句话既进 history 又进 prompt 的重复计费）、过滤降级文案、始终保留最新一条；流式与非流式共用同一套逻辑。取历史必须用 `repository.ListRecentMessages`——`ORDER BY created_at ASC LIMIT n` 取到的是**最旧**的 n 条（已踩坑）。
- **玩家上下文注入**（`service/ai_context.go`，契约见 `shared-docs/03-主服务对接说明.md` §6）：仅对「已登录 + 已绑定 UID」的用户生效（`/api/chat` 挂 `JWTAuthOptional`），组装逻辑必须遵守：
  - **分级注入**：所有展柜角色给「概览」（name/level/constellation/talents/weapon）；只有**提问点名**或**养成/配队意图**才展开 `artifacts` 明细（含 mainStats）；超出 `ctxTokenBudget`(1500) 时**整条丢弃末尾角色**并在 `summary` 注明省略数量
  - 字段受 AI 服务 schema 强校验，改动前对照上述文档（已踩坑：`weapon` 必须是对象；`refine` 字段名；`artifacts[].mainStats` 的 key 为中文部位名）
  - 上下文构建失败只记 `Warnw` 并照常对话，绝不让对话失败
  - 拉档案必须走缓存：快照 TTL `snapshotTTL`(10min) + Enka 客户端 TTL(60s) + 同 UID 单飞，禁止每次对话直查上游
  - 用户私有数据只在本次请求内存在，AI 服务不落库
- **资产资料注入（V2 结构化查表，公开数据）**：提问命中角色名时，把库里的权威推荐词条渲染进 `context.summary`，让数值类问题不再靠模型记忆。**与登录态无关**（匿名请求同样注入，这是与 V1 的关键区别）。约束：
  - 只允许走 `PlayerService.StatWeightsForQuestion`（名字命中用 `asset.CharactersMentioned`，**长名优先**，否则「雷电将军」会被更短的名字抢先）+ `ai_context.go` 的 `BuildAssetSummary` 渲染，不要另写一套查表/渲染；
  - 资料文本独立预算 `referenceTokenBudget=400`、最多 `maxReferenceCharacters=2` 个角色——与玩家档案的 `ctxTokenBudget=1500` 是**两笔账**，二者共用 AI 服务的上下文预算（AI 服务超预算时会整条丢弃末尾角色）；
  - 库里没有数据的角色必须显式渲染成「本服暂无该角色的资料（不得凭记忆给出数值）」——这是 AI 服务侧「资料外不编造」规则生效的前提；
  - 词条名翻译复用 `enka.TranslateStatRecommendation`（与玩家档案同一实现）；部位中文名从 `asset_relic_slots` 表取，**禁止硬编码中文部位名**；
  - 渲染文本必须顺序稳定（不要直接遍历 map），否则同一问题每次文本不同，会破坏上游前缀缓存的可复现性。

## 4.7 玩家查询（Enka 数据源）

数据来自 Enka.Network（`internal/service/enka/`，免登录免风控），映射数据存 MySQL（`internal/service/asset/`）。

- **禁止在代码中硬编码**角色/武器/圣遗物名称等游戏资产映射——一律入 `asset_*` 映射表，通过管理接口导入，未命中走 fallback 并记入 `missings` 表。
- Enka 响应的数值字段大量是**字符串形式的数字**，模型统一用 `json.Number` 接收；新增字段时先 curl 真实返回值确认类型。
- 翻译层 `enka/translate.go` 依赖 `asset.Service`（内存 map），映射数据更新后调用 `Import`（自动 reload）或 `POST /api/admin/assets/reload`。
- 管理接口使用 `X-Admin-Key` 头鉴权（值为 `auth.admin_key`，未配置时回落 `auth.jwt_secret`）；**禁止把 admin_key 与 jwt_secret 混用**——后者泄露即可伪造任意用户 Token。外部系统/后台管理系统只下发 admin_key，且必须走内网或 HTTPS。禁止回传密钥、禁止无鉴权暴露。
- 直连数据库改映射的外部系统，改完必须调 `POST /api/admin/assets/reload?purgeSnapshots=true` 或 `/assets/purge-snapshots`，否则运行中的服务仍用旧内存映射、旧翻译快照仍会命中。
- Enka 无 SLA：查询有 60s 缓存 + 5min 负缓存，新增批量功能必须考虑限速。

### 4.8 会话管理（C 端）

- 接口：`GET /api/sessions`、`GET /api/sessions/:sessionId/messages`、`PATCH /api/sessions/:sessionId`（`handler/session.go` → `service/chat_session.go`），契约见共享仓 `web/04-API接口文档.md` §2.4。
- **一律挂 `middleware.JWTAuth`**（不是 `JWTAuthOptional`）：匿名没有「我的会话」，未登录 401。
- **归属校验两条线都要**：读/改名用 `id + user_id` 过滤，非本人**返回 404**（不返回 403，避免探测会话是否存在）；`POST /api/chat` 收到非本人的 `sessionId` 时**当作新会话**（`ChatService.ownsSession`），禁止续聊别人的会话。匿名（uid = 0）与匿名会话（user_id = 0）归属一致，同一页面内仍可连续对话。
- **消息只从服务端拉**（前端不缓存正文）；分页用 `beforeId` 向上翻（`repository.ListMessagesBefore`，`id desc` 取 N 条后反转），不要用 `afterId`。
- **删除是软删除**（`repository.SoftDeleteSession` 置 `chat_sessions.deleted_at`，不物理删除）：C 端四条路径（列表 / 明细 / 改名 / 续聊）都必须过滤 `deleted_at IS NULL`；**后台不过滤**，照常列出并用 `deletedAt` 标注、`onlyDeleted` 筛选，后台不提供删除入口。字段用 `*time.Time`，**不要用 `gorm.DeletedAt`**——后者会把后台查询也一起隐式过滤掉。
- 重命名用 `UpdateColumn`、删除用 `UpdateColumn`：**都不要刷新 `updated_at`**，否则列表顺序会因改名 / 删除而跳动。
- 前端 `store/useChatStore.ts` 负责列表 / 切换 / 重命名 / 删除 / 恢复：**只对登录用户持久化 `sessionId`**（localStorage），匿名不持久化；`token` 变化（退出登录 / 换账号）时必须清空内存视图，否则上一个账号的对话会留在页面上。

## 5. 前端规范（React + TS）

### 5.1 命名

| 对象 | 规则 | 示例 |
| --- | --- | --- |
| 组件文件 | PascalCase | `MessageBubble.tsx` |
| 页面 | XxxPage.tsx | `ChatPage.tsx` |
| hooks/store | use 前缀 | `useChatStore.ts` |
| 类型 | PascalCase | `ChatMessage` |

### 5.2 状态与数据流

数据流固定为：`页面 → store(async action) → api/ → 后端`。

- 全局共享状态才进 Zustand；局部 UI 状态用 `useState`。
- 新领域新增 `api/xxx.ts`，复用 `http` 实例并用 `unwrap()` 拆箱，不在页面散落 URL。
- 类型定义在 `src/types/index.ts`；禁止 `any`，需要时用 `unknown` + 收窄。

### 5.3 样式

- Tailwind 4 原子类；主题色定义在 `index.css` 的 `@theme`（如 `--color-teyvat-gold` → `text-teyvat-gold`），禁止硬编码颜色值。
- 条件类名用 `clsx`。
- 路径别名：`@/*` → `src/*`，import 一律用 `@/`。

### 5.4 新增页面三步

1. `pages/XxxPage.tsx` → 2. `router/index.tsx` 注册 → 3. 需要入口则在 `Sidebar.tsx` 加导航项。

## 6. 通用代码风格

- 注释与 UI 文案使用简体中文；标识符、错误信息、日志 key 使用英文。
- Go 遵循 Effective Go / gofmt 默认风格；错误一律显式处理，不用 `_` 吞错误（日志降级等明确意图除外）。
- TS 遵循 `tsconfig` 的 strict 全开配置；组件用函数式 + hooks，不写 class。
- Git：不提交 `.env`、密钥、`server/data/`、`server/bin/`、`web/dist/`、`node_modules/`（见 `.gitignore`）。

## 7. 禁止事项（AI 高频踩坑清单）

1. ❌ 在 Go 里使用 cgo 依赖（如 mattn/go-sqlite3）——本机无 C 工具链；SQLite 只能用 pure-go 的 `glebarez/sqlite`。
2. ❌ 把数据库错误用 `_ =` 吞掉——必须 `logger.L().Errorw(...)` 记录，否则故障无法定位（已有过踩坑）。
3. ❌ 把 `*zap.Logger` 的 `Infow/Errorw` 当存在——本项目 `logger.L()` 返回 `*zap.SugaredLogger`。
4. ❌ Handler 直接写业务/查库；Service 引用 `gin.Context`。
5. ❌ 直接返回 GORM model 给前端（应经过 dto）。
6. ❌ 前端绕过 `api/` 层直接 fetch/axios；绕过 store 直接在组件里发请求。
7. ❌ 加了侧边栏入口却没在 `router/index.tsx` 注册路由——点击即落 404 兜底页；新增页面必须执行「三步法」全部三步。
8. ❌ 硬编码后端地址——前端走 `/api` 代理，后端地址只在 `vite.config.ts` 配。
9. ❌ 修改 `tsconfig` 的 `strict`、关闭 eslint 规则来"修复"报错；应修代码本身。
10. ❌ 触碰 `.codebuddy/` 目录；它不是临时缓存。
11. ❌ 提交含真实 API Key 的文件；密钥只通过环境变量注入。
12. ❌ 未经确认执行 `git commit` / `git push` / 删除文件等破坏性操作。
13. ❌ 把业务数据写死在代码里（map/常量/switch 枚举）——数据一律入数据库，代码只留结构与逻辑。
14. ❌ **用 PowerShell 的 `Set-Content` / `Out-File` 改 `.yaml` 配置文件**——会写入 UTF-8 BOM，Viper 解析失败后静默回落默认值（表现为"配置改了但完全没生效"，已踩坑）。改配置请用编辑器工具直接写，或写完用 `.NET ReadAllBytes` 检查首字节不是 `EF BB BF`。
15. ❌ 在鉴权中间件里直接依赖 `repository` 查库——封禁检查等业务判断通过注入的函数（如 `middleware.UserStatusChecker`）实现，保持中间件只依赖 `config` / `pkg`。
16. ❌ 复用密钥：用户 JWT 签名（`jwt_secret`）／资产管理（`admin_key`）／内网管理（`internal_key`）／AI 服务（`X-AI-Key`）必须各自独立生成，泄露任何一个不得影响其他信任域。
17. ❌ 把本地调试读到的密钥写进代码 / 测试 / 文档 / `.env.example`，或提交任何含真实密钥的文件——本地调试可读，**入仓即事故**（见文首「本地开发调试例外条款」）。
18. ❌ 把本地密钥用于非本机目标（公网 / 生产地址）或对生产数据做写操作；除本地两端对齐外，修改 / 重新生成密钥仍需明确确认。

## 8. 变更后的自检清单

- [ ] `web/`：`npm run lint` + `npm run build` 通过
- [ ] `server/`：`go build ./...` + `go vet ./...` 通过
- [ ] 新接口已在 `docs/04-API接口文档.md` 登记
- [ ] 新配置项已同步 `config.yaml` / `.env.example` / `config.go`
- [ ] 破坏性变更已同步更新 `docs/` 与本文件
- [ ] 文档改动只在 `teyvat-docs` 提交，本仓 `git status` 无 `docs/` / `shared-docs/` 项
