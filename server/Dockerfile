# syntax=docker/dockerfile:1
#
# 主服务（teyvat-guide-server）：Go + Gin。
#
# 两条刻意设计：
# 1) **镜像里不含任何站点配置与密钥**——`config/config.yaml` 已被 .gitignore 排除（含库密码），
#    这里也不 COPY 它。配置一律走环境变量（viper 前缀 `TEYVAT_`，点号换下划线，
#    如 `TEYVAT_DATABASE_PASSWORD` ← `database.password`），见 04-部署运维接入说明 §8。
#    若确实想用 yaml，把文件挂到 /app/config/config.yaml 即可（viper 会读）。挂载优先于默认值，
#    环境变量优先于 yaml，所以两者可混用。
# 2) 构建与运行分离：builder 里编译静态二进制，runtime 只要 ca-certificates（HTTPS 出网）
#    + tzdata（配额按本地时区自然日重置，别用 UTC 跑偏）。

ARG GO_VERSION=1.24

# ============================================================
# builder —— 只负责编译
# ============================================================
FROM golang:${GO_VERSION}-alpine AS builder

ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOFLAGS=-trimpath

WORKDIR /build

# 先只拷依赖清单：依赖没变时这层命中缓存，改业务代码不会重下依赖
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# -s -w：去掉符号表与调试信息，镜像更小
RUN go build -ldflags "-s -w" -o /out/teyvat-guide ./main.go

# ============================================================
# runtime —— 最小运行时，非 root
# ============================================================
FROM alpine:3.20 AS runtime

# ca-certificates：访问 Enka / DeepSeek 走 HTTPS 需要
# tzdata：日志与配额要用本地时区（Asia/Shanghai）
# wget：给 HEALTHCHECK 用
RUN apk add --no-cache ca-certificates tzdata wget \
    && adduser -D -u 10001 -s /sbin/nologin teyvat

ENV TZ=Asia/Shanghai \
    TEYVAT_SERVER_MODE=release \
    TEYVAT_LOG_MODE=json

WORKDIR /app
COPY --from=builder /out/teyvat-guide /app/teyvat-guide

USER teyvat
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
    CMD wget -q -O /dev/null http://127.0.0.1:8080/api/health || exit 1

ENTRYPOINT ["/app/teyvat-guide"]
