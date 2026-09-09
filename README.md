# AI 提瓦特向导 · Teyvat Guide

React 19 + Go(Gin) 的前后端分离骨架，用于搭建《原神》主题的智能助手 / 图鉴 / 工具站。

- 前端：`web/` — React 19 + TypeScript + Vite 7 + Tailwind CSS 4 + Zustand + React Router 7
- 后端：`server/` — Go + Gin + GORM(MySQL) + Viper + Zap + JWT + OpenAI SDK
- 玩家查询：Enka.Network 数据源（免登录免风控），映射数据存 MySQL 管理接口维护
- 文档：`docs/` — 架构、前后端规范、接口文档、环境搭建、路线图

## 快速开始

```powershell
# 1. 前端（已完成依赖安装）
cd web
npm run dev            # http://localhost:5173

# 2. 后端（需先安装 Go 1.22+）
cd server
go mod tidy            # 首次执行，拉取依赖
go run ./main.go       # http://localhost:8080
```

前端通过 Vite 代理把 `/api` 转发到 `http://127.0.0.1:8080`，无需额外配置 CORS。

## 目录结构

```
AI提瓦特向导/
├─ docs/                     # 项目文档
├─ web/                      # React 前端
│  └─ src/
│     ├─ api/                # 请求层（axios + 拆箱）
│     ├─ components/         # 通用组件 / 布局 / 聊天组件
│     ├─ pages/              # 路由页面
│     ├─ router/             # 路由表
│     ├─ store/              # Zustand 状态
│     ├─ types/              # 全局类型
│     └─ index.css           # Tailwind 主题
└─ server/                   # Go 后端
   ├─ config/config.yaml
   └─ internal/
      ├─ config/             # 配置加载
      ├─ handler/            # HTTP 入口
      ├─ service/            # 业务逻辑
      ├─ repository/         # 数据访问
      ├─ model/              # 数据模型
      ├─ middleware/         # 中间件
      ├─ dto/                # 请求/响应结构
      ├─ pkg/                # 基础设施（日志/响应/数据库）
      └─ router/             # 路由注册
```

## 更多

- [项目总览与架构](docs/01-项目总览与架构.md)
- [前端开发规范](docs/02-前端开发规范.md)
- [后端开发规范](docs/03-后端开发规范.md)
- [API 接口文档](docs/04-API接口文档.md)
- [环境搭建与启动](docs/05-环境搭建与启动.md)
- [开发路线图](docs/06-开发路线图.md)
