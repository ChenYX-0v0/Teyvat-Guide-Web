# AGENTS.md — AI 协作开发规范

> 本文件面向 AI 编码代理与人类协作者，描述本项目的架构、约定与"必须/禁止"事项。
> 修改任何代码前请先阅读本文；与本文冲突的旧代码不代表规范本身，遇到不一致请遵循本文。

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

统一走 `service/ai.go` 的 `AIClient` 接口（`Complete(ctx, system, history, prompt)`）。现有实现：`mock`（无密钥兜底）、`openai`（直连兼容协议）、`remote`（**默认，对接独立 AI 派蒙服务**，见 `docs/08`）。新增供应商实现该接口并在 `NewAIClient` 注册；禁止在其他地方直接引用模型 SDK。

- remote 模式配置：`ai.base_url` 指向 AI 服务（默认 `http://127.0.0.1:8090`），`ai.api_key` 为 AI 服务密钥（`X-AI-Key`，仅走环境变量 `TEYVAT_AI_API_KEY`），`ai.timeout_seconds` 必须大于 AI 服务自身超时。
- **AI 调用失败必须降级**：`ChatService.Ask` 捕获错误后记 `Errorw` 并返回兜底文案 + 落库，禁止把上游错误透传给前端（见 `chat.go` 的 `aiDegradeReply`）。
- **玩家上下文注入**（`service/ai_context.go`，契约见根目录 `03-主服务对接说明.md` §6）：仅对「已登录 + 已绑定 UID」的用户生效（`/api/chat` 挂 `JWTAuthOptional`），组装逻辑必须遵守：
  - **分级注入**：所有展柜角色给「概览」（name/level/constellation/talents/weapon）；只有**提问点名**或**养成/配队意图**才展开 `artifacts` 明细（含 mainStats）；超出 `ctxTokenBudget`(1500) 时**整条丢弃末尾角色**并在 `summary` 注明省略数量
  - 字段受 AI 服务 schema 强校验，改动前对照上述文档（已踩坑：`weapon` 必须是对象；`refine` 字段名；`artifacts[].mainStats` 的 key 为中文部位名）
  - 上下文构建失败只记 `Warnw` 并照常对话，绝不让对话失败
  - 拉档案必须走缓存：快照 TTL `snapshotTTL`(10min) + Enka 客户端 TTL(60s) + 同 UID 单飞，禁止每次对话直查上游
  - 用户私有数据只在本次请求内存在，AI 服务不落库

## 4.7 玩家查询（Enka 数据源）

数据来自 Enka.Network（`internal/service/enka/`，免登录免风控），映射数据存 MySQL（`internal/service/asset/`）。

- **禁止在代码中硬编码**角色/武器/圣遗物名称等游戏资产映射——一律入 `asset_*` 映射表，通过管理接口导入，未命中走 fallback 并记入 `missings` 表。
- Enka 响应的数值字段大量是**字符串形式的数字**，模型统一用 `json.Number` 接收；新增字段时先 curl 真实返回值确认类型。
- 翻译层 `enka/translate.go` 依赖 `asset.Service`（内存 map），映射数据更新后调用 `Import`（自动 reload）或 `POST /api/admin/assets/reload`。
- 管理接口使用 `X-Admin-Key` 头鉴权（值为 `auth.admin_key`，未配置时回落 `auth.jwt_secret`）；**禁止把 admin_key 与 jwt_secret 混用**——后者泄露即可伪造任意用户 Token。外部系统/后台管理系统只下发 admin_key，且必须走内网或 HTTPS。禁止回传密钥、禁止无鉴权暴露。
- 直连数据库改映射的外部系统，改完必须调 `POST /api/admin/assets/reload?purgeSnapshots=true` 或 `/assets/purge-snapshots`，否则运行中的服务仍用旧内存映射、旧翻译快照仍会命中。
- Enka 无 SLA：查询有 60s 缓存 + 5min 负缓存，新增批量功能必须考虑限速。

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

## 8. 变更后的自检清单

- [ ] `web/`：`npm run lint` + `npm run build` 通过
- [ ] `server/`：`go build ./...` + `go vet ./...` 通过
- [ ] 新接口已在 `docs/04-API接口文档.md` 登记
- [ ] 新配置项已同步 `config.yaml` / `.env.example` / `config.go`
- [ ] 破坏性变更已同步更新 `docs/` 与本文件
