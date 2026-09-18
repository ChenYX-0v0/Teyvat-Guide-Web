# deploy —— 阿里云上线的「照做清单」（M2 / M3）

> 目标：**ECS 到手后按这个文件从头做到尾**，不需要再回头翻文档。
> 配套的设计与选型在 `teyvat-docs/ai/06-阿里云部署方案.md`，本文只做"可执行版"。

---

## 0. 先说清楚边界：备案期间能验什么

| 能验（不依赖域名） | 不能验（等备案通过） |
| --- | --- |
| 三服务容器起来且 healthy | 域名解析、80/443 对外 |
| 回环探活（`/api/health`、`:8090/health`、`/metrics`） | HTTPS 证书与自动续期 |
| **站内完整功能**：登录 → 流式对话 → 资料注入 | 备案号展示 |
| 后台（SSH 隧道 / 白名单） | 流式经外层 Nginx 的表现 |
| 数据库迁移、备份与恢复演练 | —— |

**所以：ECS 到手当天就能把「M1 容器化验证（O1）」和「M5 云端回环联调」一次做完。**

本包共 3 个镜像 + 1 份 compose + 2 个 `.env`：

```
镜像：teyvat-ai / teyvat-guide-server / teyvat-guide-web
编排：<主仓>/docker-compose.yml（三服务，全部只绑 127.0.0.1）
配置：<主仓>/.env（主服务 + compose）  +  <ai-service>/.env（AI 服务密钥）
```

---

## 1. M2：买机器与初始化

### 1.1 ECS 选型

| 项 | 建议 | 说明 |
| --- | --- | --- |
| 地域 | 与**备案主体/目标用户**同地域 | 备案与访问延迟都看它 |
| 规格 | ≥ 2 vCPU / 4 GiB（AI 服务含 langchain，`ai/06` 提醒 ≥1 GiB 可用内存；再留 DB 与构建余量） | 只在 ECS 上做 `docker pull`，不做构建的话 2C2G 也够 |
| 系统 | Alibaba Cloud Linux 3 或 Ubuntu 22.04+ | 两者 Docker 安装命令见 §1.3 |
| 系统盘 | ≥ 40 GiB | 镜像 + MySQL 数据 + 日志 |
| 带宽 | 按量/固定 1~5 Mbps | **只用于出网调用 API**（DeepSeek/百炼），C 端流量走备案后的入口 |
| 公网 IP | 需要（出网 + 你的 SSH） | 但**先不开 80/443** |

### 1.2 安全组（这一步最容易埋雷）

| 方向 | 端口 | 源 | 说明 |
| --- | --- | --- | --- |
| 入 | 22 | **你的固定 IP/32** | 不要 `0.0.0.0/0` |
| 入 | 80/443 | —— | **备案通过前不开** |
| 入 | 8080 / 8090 | —— | **永远不开**（主服务与 AI 服务只走回环） |
| 入 | 3000（后台） | 你的固定 IP | 或者干脆不开，用 SSH 隧道（§4.4） |
| 出 | 443 | 任意 | DeepSeek / 百炼 / ACR |

### 1.3 初始化（在 ECS 上执行）

```bash
# ① 装 Docker（二选一）
#    Alibaba Cloud Linux / CentOS：
sudo dnf install -y dnf-utils && sudo dnf config-manager --add-repo https://mirrors.aliyun.com/docker-ce/linux/centos/docker-ce.repo
sudo dnf install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin
#    Ubuntu：
# sudo apt-get update && sudo apt-get install -y docker.io docker-compose-v2

# ② 一键收尾：镜像加速 + 建目录 + 校验（本包脚本）
sudo bash deploy/bootstrap.sh

# ③ 登录 ACR（拉/推私有镜像用）
sudo docker login --username=<阿里云账号> registry.cn-hangzhou.aliyuncs.com
```

> `bootstrap.sh` 做三件事：写 `/etc/docker/daemon.json`（**镜像加速器** + 日志轮转，原文件会备份）、
> 建 `/opt/teyvat-guide`、检查 Docker 与 compose 插件版本。
> **为什么必须先配加速器**：2026-09-18 实测本机对 `hub.docker.com` / `registry-1.docker.io`
> 均不可达（curl HTTP 000），不配加速器，`docker pull` 会在拉基础镜像这一步失败。

### 1.4 上传代码

```bash
sudo mkdir -p /opt/teyvat-guide && sudo chown $USER /opt/teyvat-guide
# 方式一：git（推荐）
git clone <主仓地址> /opt/teyvat-guide
git clone <AI 服务仓地址> /opt/AI派蒙服务器/ai-service      # 目录名要与 compose 的 AI_SERVICE_CONTEXT 对上
# 方式二：scp 打包上传（排除 .env、node_modules、dist）
```

> ⚠️ **不要上传任何 `.env`**：本地 `.env` 里有本地库密码与开发密钥，云上要重新生成（§2.3）。

---

## 2. 构建与发布：两条路，选 B

### 2.1 路 A：ECS 上直接构建（简单但慢、吃资源）

```bash
cd /opt/teyvat-guide
cp .env.example .env && vi .env       # 见 §2.3
docker compose up -d --build
```

### 2.2 路 B：本地构建 → 推 ACR → ECS 只拉（**推荐**）

```powershell
# 本地（Windows）：
#   先在 Docker Desktop 里配好同一个镜像加速器
.\deploy\push-images.ps1 -Registry registry.cn-hangzhou.aliyuncs.com -Namespace <你的命名空间>
```

```bash
# ECS 上：把 .env 里的镜像地址指到 ACR，然后只拉不起
docker compose pull && docker compose up -d
```

好处：ECS 不编译（省 CPU/内存/时间）、镜像带 tag 可回滚、构建失败不影响线上。

### 2.3 云上的 `.env`（三把密钥必须重新生成）

```bash
python3 -c "import secrets;print(secrets.token_urlsafe(48))"   # 生成一把
```

| 键 | 值来源 |
| --- | --- |
| `TEYVAT_DATABASE_*` | RDS 或容器内 MySQL 的连接信息（§3.1） |
| `AI_SERVICE_KEY` | 自己生成，**必须与 `ai-service/.env` 的 `AI_SERVICE_KEY` 完全一致** |
| `TEYVAT_AUTH_JWT_SECRET` / `ADMIN_KEY` / `INTERNAL_KEY` | **各自独立**、强随机（`web/11 §6` 的必须项） |
| `TEYVAT_AUTH_SEED_PASS` | 首次启动种子账号密码，建完建议改密 |
| `TEYVAT_AUTH_ALLOW_REGISTER=false` | 邀请制（已在 compose 里写死） |

---

## 3. M3：数据库

### 3.1 选型：**RDS（2026-09-19 已定，两个库都上云）**

| 方案 | 结论 | 备注 |
| --- | --- | --- |
| **RDS MySQL（已购，包年）** | **采用** | 自动备份 + 可恢复演练；**业务库与后台库放同一实例**（两个 schema，直连成本低） |
| 容器内 MySQL | 不用 | compose 末尾那段注释保留着，仅当本地单机自足时才考虑 |

**两个库、两个账号（别混）**：

| 库 | 归谁用 | 账号 | 权限范围 |
| --- | --- | --- | --- |
| `teyvat_guide` | 主服务 + 后台的业务数据源（资产表、用户、会话） | `teyvat` | 仅 `teyvat_guide.*`（含 DDL，GORM AutoMigrate 需要） |
| `teyvat_admin` | **后台自己的库**（管理员账号、会话、项目注册表） | `teyvat_ops` | 仅 `teyvat_admin.*` |

> **为什么要两个账号**：`teyvat_admin` 里有 `users`（管理员）表，业务服务**不该**有写它的权限——
> 主服务万一被攻破，攻击者也造不出一个管理员账号。这是刻意划的权限边界，别图省事合成一个。
> 库名约定：业务库 `teyvat_guide`；管理库 `teyvat_admin`（旧名 `demo_project` 已废弃，本地保留作回退）。

### 3.2 本地导出（Windows，会用仓库里的 MySQL 客户端）

```powershell
.\deploy\db-export.ps1            # 产出 deploy\out\teyvat_guide-YYYYMMDD-HHmm.sql + 行数快照
```

> ⚠️ **dump 里含用户数据与密码哈希**（`users.password_hash` 是 bcrypt，实测确认非明文）。
> `deploy/out/` 已在 `.gitignore` 里，**不要入库、不要随手发群**；传 ECS 用 `scp`（别经第三方网盘）。

### 3.3 云上导入 + 校验

```bash
# 导入（RDS 或容器 MySQL 都适用；容器内：docker exec -i teyvat-guide-mysql mysql -uroot -p ...）
mysql --host=<内网地址> --user=<用户> -p < deploy/out/teyvat_guide-YYYYMMDD-HHmm.sql
# 校验关键表行数（期望值写在 SQL 注释里）
mysql --host=<内网地址> --user=<用户> -p teyvat_guide < deploy/db-verify.sql
```

> ⚠️ **`db-export.ps1` 已带 `--skip-add-locks`，别去掉**：`mysqldump` 的 `--add-locks` 默认开启，
> 会给每张表的 INSERT 包一层 `LOCK TABLES ... WRITE` / `UNLOCK TABLES`，而应用账号没授 `LOCK TABLES` 权限，
> 导入会在第一条 `LOCK TABLES` 处整批中断并报 `ERROR 1044 Access denied ... to database`（**看着像"连库权限都没有"，
> 极易误判成 GRANT 没生效**——2026-09-19 实测踩过）。老 dump 若已带这句，用 `Select-String -NotMatch '^(LOCK|UNLOCK) TABLES'`
> 剥掉再导，或临时给账号补 `LOCK TABLES` 权限。

> 2026-09-19 实测记录（两个库都按此走通）：RDS MySQL **8.0.36**，本地导出用 **8.4 客户端**，导入无版本问题；
> `SHOW GRANTS` 已显示库级权限时，**新建账号的授权可能要过一会儿才在实际连接上生效**，导入报 1044 先确认不是 `LOCK TABLES` 那句，再重试一次。

### 3.4 备份与恢复演练（`web/11 §6` 明确要求）

```bash
# 备份
mysqldump --single-transaction --routines --triggers --set-gtid-purged=OFF \
  --databases teyvat_guide > /opt/backup/teyvat_guide-$(date +%F).sql
# 恢复演练：建一个临时库导入一遍，确认行数与 §3.3 一致（别直接在生产库上试）
```

### 3.5 后台管理库（`teyvat_admin`）—— 与业务库同一实例

后台（`demo-app`）走的是 `DB_*`（管理库）与 `TEYVAT_DB_*`（业务库）**两套连接**，迁移时两边都要改。
脚本与产物在 `server/cmd/assetimport` 之外的 `deploy/out/`（已 gitignore，含密码，别提交）：

```bash
# ① 建库 + 建后台账号（用 RDS 高权限账号）
mysql --host=<RDS地址> --user=<高权限账号> -p < deploy/out/rds-init-admin.sql

# ② 建表（结构以仓库里的 database.sql 为唯一真相；它已去掉建库块，6 张表）
mysql --host=<RDS地址> --user=teyvat_ops -p --database=teyvat_admin < deploy/out/rds-schema-admin.sql

# ③ 导数据（纯 INSERT，文件内已 SET FOREIGN_KEY_CHECKS=0 —— dump 按表名排序，
#    user_sessions 会排在 users 前面，不关外键就会失败）
mysql --host=<RDS地址> --user=teyvat_ops -p --database=teyvat_admin < deploy/out/rds-import-admin-data.sql

# ④ 校验（无输出 = 对得上；再打印管理员与两个已注册项目）
mysql --host=<RDS地址> --user=teyvat_ops -p --database=teyvat_admin < deploy/db-verify-admin.sql
```

预期：`users`=1（`super_admin`，`permission_level=0`）、`managed_projects`=2（`teyvat-guide` / `teyvat-ai`）、
`user_sessions`≥5，6 张表 collation 全是 `utf8mb4_unicode_ci`。

> 结构为什么用 `database.sql` 重建、而不是直接导本地表结构：本地 `users`/`user_sessions` 是**走非脚本路径**
> 建出来的（`id` 是有符号 `bigint`、collation 是 `0900`），与 `database.sql` 的 `BIGINT UNSIGNED` / `unicode_ci` 不一致。
> 迁移正好把这个历史包袱丢掉，云端结构 = 脚本结构，以后新环境照脚本建即可复现。

---

## 4. 验收（备案期间就能做的部分）

### 4.1 三服务状态

```bash
docker compose ps          # 三个都应是 healthy / running
docker compose logs -f server
```

### 4.2 回环探活（不需要域名）

```bash
curl -s http://127.0.0.1:8080/api/health              # 主服务
curl -s http://127.0.0.1:8090/health                  # AI 服务
curl -s -H "X-AI-Key: $AI_KEY" http://127.0.0.1:8090/metrics   # 指标 + 告警规则状态
curl -s http://127.0.0.1:8088/                        # C 端静态页（容器内 Nginx）
# 安全自检：下面这条**必须超时/拒绝**
curl -m 5 http://<ECS公网IP>:8090/health
```

### 4.3 站内功能（最有说服力的一条）

```bash
# 登录换 token，然后走一次流式对话，确认 token 用量与资料注入日志
curl -s -X POST http://127.0.0.1:8080/api/auth/login \
  -H 'Content-Type: application/json' -d '{"username":"admin","password":"<种子密码>"}'
# 拿到 token 后：
curl -N -X POST http://127.0.0.1:8080/api/chat \
  -H "Authorization: Bearer <token>" -H 'Content-Type: application/json' \
  -d '{"message":"绝缘之旗印四件套效果是什么？","stream":true}'
# 期望：SSE 逐段输出；日志出现 ai asset reference attached {"sets":1}
```

### 4.4 后台（先不开公网）

```bash
# 本地执行：把云上的 3000 端口隧道到本机
ssh -L 3000:127.0.0.1:3000 <user>@<ECS公网IP>
# 然后浏览器开 http://localhost:3000
```

### 4.5 数据库

见 §3.3 的行数校验。

---

## 5. 常见坑（都踩过或有明确证据）

| 现象 | 原因 | 处理 |
| --- | --- | --- |
| `docker pull` 拉基础镜像失败/超时 | **国内拉不到 Docker Hub**（实测 HTTP 000） | 先跑 `bootstrap.sh` 配镜像加速器；或走 §2.2 的 ACR 路线 |
| 主服务起来但报数据库连接失败 | 宿主 MySQL 只允许 `localhost`（容器走的是 Docker 网段） | 建一个 `'teyvat'@'%'` 用户授权；或改用 RDS / 容器内 MySQL |
| 配额重置时间不对 | 容器时区是 UTC | 镜像已设 `TZ=Asia/Shanghai` + 装了 tzdata，别覆盖 |
| 回答"憋"到最后一次性出现 | 外层 Nginx 缓冲了 SSE | `web/nginx.conf` 已 `proxy_buffering off`；若前面再接一层，也要关 |
| 公网能访问到 8090 | 安全组放错了 | 8090 **永远不开**；`curl -m 5 http://<公网IP>:8090/health` 必须失败 |
| 改提示词没生效 | AI 服务 60s TTL 缓存 | 等 ≤60s 或重启；改法见后台「提示词管理」 |
| 不小心把 `.env` 传上去了 | 打包时没排除 | 立刻轮换所有密钥（三把 + AI + DB） |
| `mysql -e "source X.sql"` 报 `Failed to open file` | **路径含中文**（Windows 常见，实测过） | 用 `<` 重定向（`mysql ... < X.sql`），或把文件复制到纯英文路径再 source |
| PowerShell 脚本跑一半就中断 | `mysqldump`/`docker` 把进度与提示写到 **stderr**，`$ErrorActionPreference='Stop'` 会当成终止性错误 | 本包的 ps1 已统一用 `'Continue'` + 显式检查 `$LASTEXITCODE`（踩过，别再改回去） |
| ps1 里的中文变乱码 / 报"字符串缺少终止符" | 脚本存成 **UTF-8 无 BOM**，PowerShell 5.1 按 ANSI 读 | 本包两个 ps1 已带 BOM（`EF BB BF`）；新增脚本请保持带 BOM |
| 后台登录报数据库错误 / 连上了却没数据 | 后台有**两套**连接：`DB_*`=管理库 `teyvat_admin`、`TEYVAT_DB_*`=业务库 `teyvat_guide`（且**不会**回落到 `DB_*`） | 迁移时两套都要改；`.env.local` 里 `TEYVAT_DB_NAME` 必须显式写 |
| 导入后台库数据报外键错 | dump 按表名排序，`user_sessions` 先于 `users` | 用 `deploy/out/rds-import-admin-data.sql`（文件内已包 `SET FOREIGN_KEY_CHECKS=0`） |
| 导入报 `ERROR 1044 Access denied ... to database`，但 `SHOW GRANTS` 明明有权限 | dump 里有 `LOCK TABLES`（`--add-locks` 默认开），而应用账号没授 `LOCK TABLES` | 导出加 `--skip-add-locks`（`db-export.ps1` 已加）；老 dump 剥掉 `LOCK TABLES`/`UNLOCK TABLES` 两行再导 |
| 用 `rm-xxx.mysql.rds.aliyuncs.com:3306` 连不上、报错像密码错 | ① 账号名打错一个字母；② 把 `@@port`（服务器内部端口，如 3009）当成外网端口 | 外网入口用 **3306**；账号名从控制台「账号管理」原样复制；`SELECT USER(), CURRENT_USER(), @@version, @@port` 可自查 |

---

## 6. 备案通过后（还没做，先记着）

1. 域名解析到 ECS + 外层 Nginx（80→443，`server_name` 用你的域名）；
2. 证书（阿里云免费证书或 Let's Encrypt）+ **自动续期**；
3. 安全组开 80/443；把 `web` 容器的端口从 `127.0.0.1:8088` 改成对外映射（或让外层 Nginx 反代 `web:80`）；
4. 页脚挂备案号（链接工信部）；
5. 重跑 `web/11 §6` 的上线前检查清单（HTTPS/证书/到期提醒那几条）。

---

## 7. 密钥扫描（提交前自检）

```powershell
powershell -ExecutionPolicy Bypass -File .\deploy\check-secrets.ps1        # 扫暂存区（即将提交的内容）
powershell -ExecutionPolicy Bypass -File .\deploy\check-secrets.ps1 -All   # 扫全部被跟踪文件
```

三类检查：

1. **敏感文件名被纳入**：`.env*`、`server/config/config.yaml`、`deploy/out/`、`*.pem|key|pfx`、dump（`*.sql`）；
2. **内容里"像真密钥"的赋值**：`password|secret|api_key|admin_key|internal_key|token = <16 位以上>`，
   自动排除占位值（`change-me`、`your-`、`<...>`、`example`、`fake`…）与代码取值（`localStorage…`）；
3. **与已知真值精确比对**：读 `deploy/out/known-secrets.txt`（本机真值清单，**已 gitignore**；
   由本机在用配置提取，轮换后追加新值）。正则只能猜形态，这一条能确定命中，是主要防线。

命中时**只打印遮蔽片段**（前 4 位 + 长度），**退出码 1**，可直接接进 pre-commit / CI。
想让它每次提交自动跑：自己加 `.githooks/pre-commit` 并 `git config core.hooksPath .githooks`
（改 git 配置由你决定，部署包不代劳）。

> 密钥清单、轮换手册、泄露应急流程见 `shared-docs/05-密钥与凭据管理.md`。
> ⚠️ 脚本必须保持 **UTF-8 带 BOM**；另外两处踩过的坑写在脚本头注释里（别用 `git grep -E` 传复杂正则；
> git 列文件必须带 `-c core.quotepath=false`，否则中文路径会被转义成非法路径）。
