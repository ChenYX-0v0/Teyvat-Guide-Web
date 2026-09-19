#!/usr/bin/env bash
# ECS 初始化（M2）：镜像加速 + 目录 + 环境自检
#
# 用法：sudo bash deploy/bootstrap.sh [加速器地址]
#   - 不传加速器地址时，用 daemon.json 模板（里面是占位符，会提示你去 ACR 控制台取）
#
# 为什么必须有这一步：2026-09-18 实测 hub.docker.com / registry-1.docker.io 均不可达
# （curl HTTP 000），不配镜像加速器，docker pull 拉基础镜像就会失败。
#
# 本脚本**不装 Docker**（发行版差异大，见 deploy/README.md §1.3），只做收尾与自检。
# 幂等：可重复执行；改 daemon.json 前会备份原文件。

set -euo pipefail

MIRROR="${1:-}"
DAEMON_JSON=/etc/docker/daemon.json
BACKUP_SUFFIX="$(date +%F-%H%M%S)"
INSTALL_DIR=/opt/teyvat-guide

log()  { printf '\033[36m==> %s\033[0m\n' "$*"; }
ok()   { printf '\033[32m    [OK] %s\033[0m\n' "$*"; }
warn() { printf '\033[33m    [!!] %s\033[0m\n' "$*"; }
die()  { printf '\033[31m    [错误] %s\033[0m\n' "$*" >&2; exit 1; }

[[ "$(id -u)" -eq 0 ]] || die "请用 sudo 执行：sudo bash deploy/bootstrap.sh"

# ---------------------------------------------------------------- 1) Docker 是否就绪
log "检查 Docker"
if ! command -v docker >/dev/null 2>&1; then
  die "未安装 Docker。先按 deploy/README.md §1.3 安装，再重跑本脚本。"
fi
ok "docker $(docker --version | sed 's/^Docker version //;s/,.*//')"
if docker compose version >/dev/null 2>&1; then
  ok "compose 插件 $(docker compose version --short 2>/dev/null || echo '可用')"
else
  die "缺 docker compose 插件（安装 docker-compose-plugin），compose 起不来。"
fi
if systemctl is-active --quiet docker; then ok "docker 服务运行中"; else warn "docker 服务未运行，尝试启动"; systemctl enable --now docker; fi

# ---------------------------------------------------------------- 2) 镜像加速器
log "配置镜像加速器（$DAEMON_JSON）"
if [[ -z "$MIRROR" ]]; then
  warn "未传加速器地址。请到「容器镜像服务 ACR → 镜像工具 → 镜像加速器」复制你的专属地址，"
  warn "然后重跑：sudo bash deploy/bootstrap.sh https://<你的ID>.mirror.aliyuncs.com"
  warn "（本次跳过 daemon.json 写入，其余步骤继续）"
else
  # 形状校验：避免把控制台里的整段说明文字粘进来
  if [[ ! "$MIRROR" =~ ^https://[^/[:space:]]+$ ]]; then
    die "加速器地址形状不对：$MIRROR（应形如 https://xxxx.mirror.aliyuncs.com，不要带路径或空格）"
  fi
  mkdir -p /etc/docker
  if [[ -f "$DAEMON_JSON" ]]; then
    cp "$DAEMON_JSON" "${DAEMON_JSON}.bak.${BACKUP_SUFFIX}"
    ok "已备份原文件 → ${DAEMON_JSON}.bak.${BACKUP_SUFFIX}"
  fi
  cat > "$DAEMON_JSON" <<JSON
{
  "registry-mirrors": ["${MIRROR}"],
  "log-driver": "json-file",
  "log-opts": { "max-size": "10m", "max-file": "3" },
  "max-concurrent-downloads": 6
}
JSON
  ok "已写入 $DAEMON_JSON"
  systemctl restart docker
  ok "docker 已重启"
  # 验证：能拉到基础镜像才算数（用最小的 alpine 试）
  if docker pull alpine:3.20 >/dev/null 2>&1; then
    ok "拉取测试通过（alpine:3.20）"
  else
    warn "拉取测试失败：加速器可能还没生效，或 ACR 侧未开通。可先走 README §2.2 的 ACR 镜像路线。"
  fi
fi

# ---------------------------------------------------------------- 3) 部署目录
log "准备部署目录 $INSTALL_DIR"
mkdir -p "$INSTALL_DIR"
ok "已创建（用你的普通用户 clone 代码进去：chown \$USER $INSTALL_DIR）"

# ---------------------------------------------------------------- 4) 防火墙提示
log "本机防火墙检查"
if command -v ufw >/dev/null 2>&1 && ufw status 2>/dev/null | grep -q active; then
  warn "ufw 处于启用状态，注意别把 8080/8090 放行；只需放行 22（且限来源 IP）"
elif command -v firewall-cmd >/dev/null 2>&1 && systemctl is-active --quiet firewalld; then
  warn "firewalld 运行中，同样只放行 22；8080/8090 必须保持关闭"
else
  ok "未检测到启用中的本机防火墙（真正生效的是**安全组**，别忘配：只开 22，且限你的固定 IP）"
fi

# ---------------------------------------------------------------- 5) 收尾提示
log "下一步"
cat <<'TIP'
    1) 安全组：只开 22（限你的固定 IP）；80/443 备案前不开；8080/8090 永不开
    2) clone 代码到 /opt/teyvat-guide（AI 服务仓放到 compose 里 AI_SERVICE_CONTEXT 指的位置）
    3) 填 .env（三把密钥各自强随机；AI_SERVICE_KEY 与 AI 服务一致）：
         python3 -c "import secrets;print(secrets.token_urlsafe(48))"
    4) docker login --username=<阿里云账号> <ACR 地址，见控制台「仓库管理 → 访问凭证」>
    5) 按 deploy/README.md §2 起服务（推荐：本地构建 → 推 ACR → ECS 只 pull）
TIP
ok "初始化收尾完成"
