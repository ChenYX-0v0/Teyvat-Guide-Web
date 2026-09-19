#!/usr/bin/env bash
# ────────────────────────────────────────────────────────────────
# 云上服务自检（在 ECS 上执行，一条命令验完整链路）
#
# 用法：
#   scp -i <私钥> deploy/smoke-ecs.sh root@<ECS公网IP>:/opt/teyvat-guide/deploy/
#   ssh -i <私钥> root@<ECS公网IP> 'bash /opt/teyvat-guide/deploy/smoke-ecs.sh'
#
# 覆盖：容器状态 → 三端健康检查（走 Nginx，与线上入口同路径）→ 登录 + 提问（端到端）
#       → 关键日志（资产加载 / 资料注入 / 模型调用）→ 资源占用 → 暴露面
# 判读：最后打印「通过 N 项，失败 M 项」，M 应为 0。
# 注：只读检查，不改动任何数据（提问会写一条 chat_messages，属正常业务写入）。
# ────────────────────────────────────────────────────────────────
set -u
cd /opt/teyvat-guide || exit 1
pass=0; fail=0
ok()  { echo "  [OK]   $1"; pass=$((pass+1)); }
bad() { echo "  [FAIL] $1"; fail=$((fail+1)); }

echo "=== 1) 容器状态 ==="
docker compose ps --format "table {{.Service}}\t{{.Status}}\t{{.Ports}}"
n=$(docker compose ps --format '{{.Status}}' | grep -c healthy)
if [ "$n" -eq 3 ]; then ok "三容器均 healthy"; else bad "healthy 数量 = $n（期望 3）"; fi

echo "=== 2) 健康检查（经 Nginx，与线上入口同路径）==="
c=$(curl -s -o /dev/null -w '%{http_code}' --max-time 10 http://127.0.0.1:8088/api/health)
if [ "$c" = "200" ]; then ok "GET /api/health = 200（Nginx→主服务）"; else bad "GET /api/health = $c"; fi
c2=$(curl -s -o /dev/null -w '%{http_code}' --max-time 10 http://127.0.0.1:8090/health)
if [ "$c2" = "200" ]; then ok "AI 服务 /health = 200"; else bad "AI 服务 /health = $c2"; fi
c3=$(curl -s -o /dev/null -w '%{http_code}' --max-time 10 http://127.0.0.1:8088/)
if [ "$c3" = "200" ]; then ok "C 端前端首页 = 200"; else bad "C 端前端首页 = $c3"; fi

echo "=== 3) 端到端：登录 + 提问 ==="
tok=$(curl -s --max-time 20 -X POST -H 'Content-Type: application/json' \
      -d '{"username":"admin","password":"admin123"}' \
      http://127.0.0.1:8088/api/auth/login | grep -o '"token":"[^"]*"' | cut -d'"' -f4)
if [ -n "${tok:-}" ]; then ok "登录成功（token 长度 ${#tok}）"; else bad "登录失败（看 server 日志）"; fi
if [ -n "${tok:-}" ]; then
  rep=$(curl -s --max-time 90 -X POST -H "Authorization: Bearer $tok" -H 'Content-Type: application/json' \
        -d '{"message":"绝缘之旗印四件套效果是什么？"}' http://127.0.0.1:8088/api/chat)
  if echo "$rep" | grep -q '晕乎乎'; then
    bad "走到了兜底话术（AI 链路有问题：查 ai-service 日志）"
  else
    ok "提问得到正常作答"
    echo "         回复片段：$(echo "$rep" | head -c 150)"
  fi
fi

echo "=== 4) 关键日志 ==="
docker compose logs server --tail 150 2>&1 | grep -E 'asset maps loaded|asset reference attached' | tail -2 | sed 's/^/  /'
docker compose logs ai-service --tail 80 2>&1 | grep -E 'llm.completed' | tail -1 | sed 's/^/  /'

echo "=== 5) 资源占用 ==="
docker stats --no-stream --format '  {{.Name}}  CPU={{.CPUPerc}}  MEM={{.MemUsage}}'

echo "=== 6) 暴露面（公网只应有 22；其余应为 127.0.0.1）==="
ss -tlnp 2>/dev/null | grep -vE '127\.0\.0\.1|\[::1\]' | sed 's/^/  /'

echo
echo "=== 结果：通过 $pass 项，失败 $fail 项 ==="
