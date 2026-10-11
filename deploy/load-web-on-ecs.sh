#!/usr/bin/env bash
# ---------------------------------------------------------------------------
# ECS 侧「单服务（web）发版」—— 只传 tag，其余全自动（2026-10-11 起固化在仓库里）
#
#   bash load-web-on-ecs.sh <tag>      例：bash load-web-on-ecs.sh 20261011-2316
#
# 前提：镜像 tar 已传到 ECS 的 /tmp/web-<tag>.tar
#   （本机：docker save <ACR全名>/teyvat/teyvat-guide-web:<tag> -o web-<tag>.tar ✓ 再 scp ✓）
#
# 为什么走 load 而不是 pull ✗：ECS 从未 `docker login` 过 ACR（/root/.docker/ 只有 buildx、无 config.json），
#   `docker compose pull` 会 `pull access denied` ✓ —— 这就是 deploy/README 里记的"应急路线②"（无需登录 ✓）。
#   想以后能直接 pull：在 ECS 上手动 `docker login` 一次，或把本机凭据同步过去 ✓。
#
# 它做四件事：① 备份 /opt/teyvat-guide/.env（回滚点）② 改 WEB_IMAGE ③ load + `up -d --no-build web`
#   ④ 自检（容器 tag / 容器内产物 hash 文件名 / 回环 200）并清理 tar
# ⚠️ 本文件是 .sh ⇒ 必须 **LF** 换行（.gitattributes 已约定 `*.sh text eol=lf` ✓），从 Windows 生成后要归一化 ✗
# ---------------------------------------------------------------------------
set -e
TAG="$1"
if [ -z "$TAG" ]; then echo "用法: $0 <tag>"; exit 1; fi

ACR=crpi-ecqw70nr0f83o93t.cn-shanghai.personal.cr.aliyuncs.com/teyvat/teyvat-guide-web
cd /opt/teyvat-guide

TS=$(date +%Y%m%d-%H%M%S)
cp -a .env ".env.bak-$TS"
echo "==== 1) 回滚点：.env.bak-$TS ===="

echo "==== 2) 改前 WEB_IMAGE ===="
grep -n '^WEB_IMAGE=' .env

sed -i "s|^WEB_IMAGE=.*|WEB_IMAGE=$ACR:$TAG|" .env

echo "==== 3) 改后 WEB_IMAGE ===="
grep -n '^WEB_IMAGE=' .env

echo "==== 4) docker load ===="
docker load -i "/tmp/web-$TAG.tar"

echo "==== 5) 重启 web（--no-build ✓）===="
docker compose up -d --no-build web 2>&1 | tail -5
sleep 5

echo "==== 6) 容器 ===="
docker ps --format '{{.Names}} | {{.Image}} | {{.Status}}'

echo "==== 7) 容器内产物（核对 hash 文件名是否为新包 ✓）===="
docker exec teyvat-guide-web sh -c 'ls /usr/share/nginx/html/assets'

echo "==== 8) 回环自测 ===="
curl -s -o /dev/null -w '  / -> %{http_code}\n' http://127.0.0.1:8088/
curl -s -o /dev/null -w '  /api/health -> %{http_code}\n' http://127.0.0.1:8088/api/health

echo "==== 9) 清理 ===="
rm -f "/tmp/web-$TAG.tar" && echo "  已删 /tmp/web-$TAG.tar ✓"
echo "  回滚：把 .env 的 WEB_IMAGE 改回上一 tag，再 docker compose up -d --no-build web ✓"
