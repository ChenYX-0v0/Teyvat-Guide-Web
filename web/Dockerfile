# syntax=docker/dockerfile:1
#
# C 端前端（Vite + React）：构建成静态产物，交给 Nginx 托管。
#
# 为什么同域：前端请求走相对路径 `/api`（vite.config.ts 里的 dev proxy 只用于本地开发），
# 生产由 Nginx 把 `/api/` 反代到主服务容器 —— 这样**没有跨域**，也就完全不需要配 CORS
# （主服务 CORS 白名单只放行回环/内网，见 router.go）。
#
# 另注：`npm run build` 是 `tsc -b && vite build`，类型检查不过就不会出产物 ✓

ARG NODE_VERSION=22
ARG NGINX_VERSION=1.27

# ============================================================
# builder —— 装依赖 + 打包静态产物
# ============================================================
FROM node:${NODE_VERSION}-alpine AS builder

WORKDIR /build

# 先只拷依赖清单：lock 没变时这层命中缓存
COPY package.json package-lock.json ./
RUN npm ci

COPY . .
RUN npm run build

# ============================================================
# runtime —— Nginx，只放静态产物 + 一份反代配置
# ============================================================
FROM nginx:${NGINX_VERSION}-alpine AS runtime

# 换掉默认站点配置（默认那份只 serve 静态、不反代 /api）
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=builder /build/dist /usr/share/nginx/html

EXPOSE 80

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -q -O /dev/null http://127.0.0.1/ || exit 1
