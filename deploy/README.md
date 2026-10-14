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
#    ⚠️ 地址照抄控制台「ACR → 个人版实例 → 仓库管理 → 访问凭证」给的那行：
#    新版实例是 crpi-xxxx.cn-<地域>.personal.cr.aliyuncs.com，旧版才是 registry.cn-<地域>.aliyuncs.com
sudo docker login --username=<阿里云账号> <ACR 地址>
```

> `bootstrap.sh` 做三件事：写 `/etc/docker/daemon.json`（**镜像加速器** + 日志轮转，原文件会备份）、
> 建 `/opt/teyvat-guide`、检查 Docker 与 compose 插件版本。
> **为什么必须先配加速器**：2026-09-18 实测本机对 `hub.docker.com` / `registry-1.docker.io`
> 均不可达（curl HTTP 000），不配加速器，`docker pull` 会在拉基础镜像这一步失败。

### 1.3.1 镜像加速器怎么选（2026-09-19 本机实测，结论可直接搬到 ECS）

| 源 | `/v2/` 探测 | 实测结果 |
| --- | --- | --- |
| **Docker Hub 直连** | — | ❌ `registry-1.docker.io:443` **actively refused**（被主动拒绝，不是超时） |
| 阿里云**专属**加速器（`<id>.mirror.aliyuncs.com`） | **403** | ❌ 从本机**完全不可用**（连标准 `/v2/` 请求也拒；不是"未登录"的 401） |
| **DaoCloud 公共镜像** `docker.m.daocloud.io` | **401** | ✅ **可用**：`docker pull alpine:3.20` **8.4 秒**完成（401 是 registry 的正常应答，客户端会自动走令牌流程） |
| `docker.1ms.run` | — | 备用（已写入本机 mirrors 列表第二位） |

**所以本机与 ECS 的 mirrors 都按这个顺序配**：`docker.m.daocloud.io` → `docker.1ms.run` → （若你的阿里云专属加速器可用再放第三个）。

> ⚠️ **Docker Desktop（Windows）与 ECS（Linux）的配置文件不是同一个**：
> - Windows Docker Desktop 4.91：Settings → Docker Engine 写的是 **`C:\Users\<你>\.docker\daemon.json`**；
>   手写 **`C:\ProgramData\Docker\config\daemon.json` 不会生效**（2026-09-19 实测：写了它，`docker info` 里 `Mirrors` 仍为空）。
> - ECS（Linux）：`/etc/docker/daemon.json`（`bootstrap.sh` 写的就是它 ✓）。

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
.\deploy\push-images.ps1 -Registry <ACR 地址，如 crpi-xxxx.cn-hangzhou.personal.cr.aliyuncs.com> -Namespace <你的命名空间>
#   ⚠️ 脚本内置 --provenance=false --sbom=false（必需）：不然推到 ACR 最后一步会报
#      unknown manifest class for application/vnd.oci.empty.v1+json（详见 §5 常见坑）
#   推送成功后会打印「要写进 ECS .env 的三行」——那三行带时间戳 tag，回滚就是改回上一个 tag
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

### 3.6 本机容器化联调（2026-09-19 实测通过，上 ECS 前先跑这一遍）

**为什么值得跑**：容器路径与本地 dev 有**本质差异——容器里没有 `config.yaml`，所有配置只能走环境变量**。
本次就靠它抓出两个只在容器里暴露的 bug（见 §5 最后三行），要是直接上 ECS，会在线上才发现。

```powershell
# 0) 先让出 8080/8090（compose 会占用；后台 3000 不受影响）
powershell -ExecutionPolicy Bypass -File scripts\stop-all.ps1

# 1) 起三个容器（用本地已有镜像，不重新 build）
docker compose up -d
docker compose ps                     # 期望：三个都 (healthy)

# 2) 生产入口路径验收（走 Nginx，等价线上）
#    http://127.0.0.1:8088/            → 前端页面
#    http://127.0.0.1:8088/api/health  → Nginx 反代到主服务
$body = '{"username":"admin","password":"<你的密码>"}'
$tok = (curl.exe -s -X POST -H 'Content-Type: application/json' -d $body http://127.0.0.1:8088/api/auth/login | ConvertFrom-Json).data.token
curl.exe -s -X POST -H "Authorization: Bearer $tok" -H 'Content-Type: application/json' `
  -d '{"message":"绝缘之旗印四件套效果是什么？"}' http://127.0.0.1:8088/api/chat

# 3) 停：docker compose down      回到 dev 栈：scripts\start-all.ps1
```

**实测资源占用（ECS 选规格的依据）**：

| 容器 | 内存 |
| --- | --- |
| `teyvat-guide-server` | 7.6 MiB |
| `teyvat-ai` | 96 MiB |
| `teyvat-guide-web` | 12.9 MiB |
| **合计** | **约 117 MiB**（另加 Docker 自身开销）→ **2C2G 的 ECS 足够** |

**验收要点**：`docker compose logs server` 应出现 `asset maps loaded`（= 读到 RDS 的表）与
`ai asset reference attached`（= 资料注入生效）；`logs ai-service` 应出现 `llm.completed`（含 `tokens_cached` > 0 表示提示词缓存命中）。

> ⚠️ 容器栈与 dev 栈**互斥**（都占 8080/8090）：切换时先停另一套。

---

### 3.7 后台管理接「云上主服务」（SSH 隧道 + 项目注册表）

**关键差异：业务项目的地址不在环境变量里，而在管理库里**
—— `teyvat_admin.managed_projects.base_url`（项目注册表，见 `admin/05-中转网关项目设计.md`）；
只有**鉴权密钥**走环境变量，键名由 code 生成：`teyvat-guide` → `PROJECT_TEYVAT_GUIDE_ADMIN_KEY`（`src/lib/projects.ts:18`）。
所以「切本机 / 切云上」改的是**一行 SQL**，不是配置文件。

```powershell
# ① 建隧道：本机 18080 → 云上主服务 8080；本机 18090 → 云上 AI 服务 8090
#    （刻意用 18080/18090，避免和本机容器栈的 8080/8090 抢端口）
ssh -i <私钥路径> -N -L 18080:127.0.0.1:8080 -L 18090:127.0.0.1:8090 root@<ECS公网IP>
```

```sql
-- ② 指向云上（经隧道）
UPDATE managed_projects SET base_url='http://127.0.0.1:18080' WHERE code='teyvat-guide';
UPDATE managed_projects SET base_url='http://127.0.0.1:18090' WHERE code='teyvat-ai';
-- 回退到本机容器栈（旧值也备份在 deploy/out/admin-base-url-backup.txt）：
-- UPDATE managed_projects SET base_url='http://127.0.0.1:8080' WHERE code='teyvat-guide';
-- UPDATE managed_projects SET base_url='http://127.0.0.1:8090' WHERE code='teyvat-ai';
```

**验收（2026-09-20 实测通过）**：后台「资产映射」保存一次 → 提示「已保存并刷新内存，档案快照已清空」；
云上 `docker compose logs server` 出现 `POST /api/admin/assets/reload status:200`（请求确实落在云上）。

⚠️ 三个注意点：
1. **隧道必须常开**（它只在你本机与云上之间）；不开隧道时后台会提示"内存刷新失败"。
   ✅ 自愈脚本（2026-09-28 加，**计划任务已于 2026-10-11 按用户要求移除** ✗）：`scripts\tunnel-watchdog.ps1`
   判据是「**经隧道打真请求**」（`/api/health` + 18090/9088/13306 三个 TCP ✓），所以连"**进程还在、通道已死**"的
   假死也能救 ✓；重建调 `tunnel-remote.ps1`（转发定义只有一份 ✓）。`-Status` 只看状态不改动 ✓、
   `-InstallTask` / `-UninstallTask` 管任务 ✓（`schtasks` 建的，**不需要管理员** ✓）、日志 `logs\tunnel-watchdog.log` ✓。
   ⚠️ **为什么被移除**：任务动作写成了 **`.cmd`** ✗ —— 计划任务以交互方式运行 `.cmd` 会**真开一个控制台窗口** ✓，
   每 5 分钟闪一次 ✗（2026-10-11 用户反馈）。
   ⇒ 若以后要重新启用，**先换掉启动方式**：用 `wscript.exe` 跑一个 `.vbs`（内部 `WScript.Shell.Run(cmd, 0, false)` ✓）
   或其它不产生控制台窗口的包装 ✓，再 `-InstallTask`（否则弹窗会回来 ✗）。
   另一个已知限制：`schtasks /SC ONLOGON` **需要管理员** ✗，所以当时用的是 `/SC ONCE` + 由看门狗 `schtasks /Run` 拉起 ✓
   ⇒ 重启后最迟两个探测周期（≤10 分钟）才恢复 ✓。
2. **长期方案**：域名 + HTTPS 就绪后把 `base_url` 改成 `https://<域名>`（同样一行 SQL），后台就不再依赖隧道。
3. `managed_projects.status` 必须是 **`enabled`** —— 后台只对启用的项目发起调用（`src/lib/projects.ts:12`）。

> **后续（规划中，本轮不做）**：把**后台管理自身**也做成容器放进同一台 ECS，就不需要隧道了。
> 要点：补 `Dockerfile`（Next.js standalone）→ 只绑回环 → 外层 Nginx 按子域转发 → **管理面配访问控制**
> （安全组白名单 / Basic Auth / VPN 三选一）→ `base_url` 改为**容器间地址**（同一 compose 网络用服务名
> `http://server:8080`，否则 `http://host.docker.internal:8080`；⚠️ **别写 `127.0.0.1:8080`**，容器里的它是容器自己）。
> 完整要点见 `teyvat-docs/ai/06-阿里云部署方案.md §1.2`，排期登记在 `ai/05-开发路线图.md §8 的 O5`。

---

### 3.8 云上自检（一条命令验完整链路）

```bash
# 本机：推脚本 → 远端执行
scp -i <私钥> deploy/smoke-ecs.sh root@<ECS公网IP>:/opt/teyvat-guide/deploy/
ssh -i <私钥> root@<ECS公网IP> 'bash /opt/teyvat-guide/deploy/smoke-ecs.sh'
```

覆盖 6 段：容器状态 → 三端健康检查（**经 Nginx，与线上入口同路径**）→ 登录 + 提问（端到端）
→ 关键日志（`asset maps loaded` / `asset reference attached` / `llm.completed`）→ 资源占用 → 暴露面。
结尾打印「通过 N 项，失败 M 项」，**M 应为 0**。属只读检查（提问会写一条 `chat_messages`，是正常业务写入）。

**2026-09-20 首跑：6/6 通过** —— 三容器 healthy；`/api/health`=200；登录 + 提问正常作答（`tokens_cached=2176`、797ms）；
资源 ≈107MB；公网仅 22 开放（其余全 `127.0.0.1`）。

---

### 3.9 生产当前版本（每次发版后更新本节）

> 换机器 / 交接 / 回滚时，「线上跑的是哪一版」只看这里。
> 云上对应的是 `/opt/teyvat-guide/.env` 里三行 `*_IMAGE` 的 tag（**2026-09-27 复核**：`docker ps` 与 `.env` 一致 ✓）。

| 服务 | 当前生产 tag | 内容要点 |
| --- | --- | --- |
| `teyvat-guide-server` | **`20261014-2345`** | **机制卡边界字段的注入顺序修正**（2026-10-14 深夜 ✓，只发 server ✓）：① 段顺序从 `提供 > 做不到 > 要点 > 需要 > 边界 > 挂元素 > 共鸣 > 抗打断 > 护盾 > 冲突 > 基数` ⇒ **把 `需要` / `冲突` 提进前四位** （用户问得最多的是"这队怎么搭 / 谁能进队"✓，回答它的正是 提供 / 做不到 / 需要 / 冲突 四段 ✓）；② 新增**单段最多 4 条 ＋ 单条最多 80 字（超出截断加「…」）** ✗ —— 起因：边界字段批量补齐后单段可达 10~14 条 ✗，一段就吃光整张卡预算 ⇒ 排在最后的 `冲突` / `基数` **整段消失** ✗（实测：问「芙宁娜怎么配队」时 `冲突` 没进上下文）。配套单测：新增 `TestRenderMechanicItemCapAndTruncate` ✓、改写 `TestRenderMechanicDropsLowPriorityFirst` ✓（旧手法"塞一条超长基数逼出裁剪"已被截断取代 ✗）✓ |
| `teyvat-guide-web` | **`20261012-2348`** | **聊天页：「派蒙看不到你的角色」提示条**（与 server `20261012-2348` **同批** ✓，修用户报障的另一半 ✓）：进聊天页时调一次 `GET /api/genshin/context-status` ✓，`ready=false` 时在标题栏下方显示一条提示 ✓，按 `reason` 分两种文案（`not_bound` → 「还没绑定游戏 UID，派蒙看不到你的角色。」✓；`no_profile` → 「你的展柜是空的或未公开…」✓）+ 「去账号页处理」链接 ✓；i18n zh/en 各 **+3 键** ✓。请求失败**静默**处理（少一行提示，不打扰对话 ✓）。<br>验收：线上包 **`assets/index-2NmBhKHO.js`** ✓、包里可搜到键 `chat.ctxNotBound` ✓ 与文案「还没绑定游戏 UID」✓、`/` 与 `/healthz` 均 200 ✓、`X-Robots-Tag: noindex` ✓、`robots.txt: Disallow: /` ✓。⚠️ 本条**必须与 server `2348` 同批上线** ✗（前端调的是当批新增的接口 ✓）—— 本次顺序正确：**先 server 后 web** ✓（按 §3.9 回滚注里那条教训 ✓）。<br>此前（`20261012-2312`）：**派蒙回答按行渲染：行首分点做悬挂缩进**（与提示词 v11「解释类收口」配套 ✓，见 `teyvat-docs/ai/07` TODO-17 ✓）：新增 `utils/renderAnswer.tsx` —— **纯函数、每次按当前完整文本重算** ⇒ 流式安全 ✓（回答是增量拼接的，某一帧可能正好停在「1.」这种半行状态 ✗，任何"已缩进/已渲染"缓存都会闪烁或漏缩进 ✓）；只认行首 `1. ` / `· ` 这类**纯文本分点**（**不是 Markdown** ✓，不引任何 Markdown 库 ⇒ 无 XSS 面 ✓）；空行给固定行距（否则块级元素会把段落间空行吃掉 ✗）；流式光标改挂在**最后一行内部**（否则会被挤到新的一行 ✗）。`MessageBubble.tsx` 只在**助手**分支用它 ✓，用户消息保持原样插值 ✓；`ChatPage` 与账号页 `AnalysisPanel` 共用同一气泡 ⇒ 两个入口同时生效 ✓。<br>验收：线上镜像 tag **`20261012-2312`** ✓（`docker inspect` 核对 ✓）、容器内产物 **`assets/index-CBu4hrUe.js`** ✓、特征串 **`1.4em` 命中 1 个文件** ✓（悬挂缩进的内联样式 ✓）、`/` 与 `/healthz` 均 **200** ✓、`X-Robots-Tag: noindex, nofollow, noarchive` ✓、`robots.txt: Disallow: /` ✓；**合规复核**：JS 包里 `辽ICP备2026022414号-1` 与 `beian.miit.gov.cn` 均在 ✓。<br>⚠️ **本脚本第 5 步的 `index.html 含 ICP` 检查恒为 0** ✗ —— 备案号由 React 渲染进 **JS 包** ✓，`index.html` 里本来就没有 ✓（本次一度据此误判"合规缺失"✗，查对位置后确认无问题 ✓）。首次判读时还错把检索串写成「备案」✗（页脚原文是 `辽ICP备…` ✓，不含"备案"两字 ✓）。**结论：要在 `assets/*.js` 里搜 `2026022414` 才有意义** ✓。<br>此前（`20261012-2218`）：**反馈弹窗「类型」下拉改自制主题下拉**（用户反馈"下拉是一片白" ✓）：根因是那处用了**原生 `<select>`** ✗ —— 收起态由**系统控件**绘制、我们设的深色底/文字/边框基本被忽略，候选项在 Windows 上是**系统白底** ✓（`:root` 已设 `color-scheme: dark` 也压不住各浏览器差异 ✗）。改为**自制 listbox**：浮层 `teyvat-panel-soft`（L2）/ 悬停 `teyvat-raised`（L3）/ 选中金色 ✓，配色全走主题令牌；列表**走文档流**（Modal 内容区 `overflow-y-auto`，绝对定位会被裁掉 ✗）；Esc **捕获阶段** `stopImmediatePropagation` 先收下拉 ✓（否则 Modal 会连带关掉整个弹窗 ✗）；a11y `role=listbox/option` + `aria-*` ✓。全站已无原生 `<select>` 标签 ✓。<br>验收：线上包 **`assets/index-CpRBRGOh.js`** ✓（与本机构建**同名同字节** 441714 B ✓）；特征串 `listbox` / `teyvat-panel-soft` / `fb-cat-label` / `aria-haspopup` / `aria-expanded` 均在包里 ✓；回归对照 `feedback.title` / `功能反馈` / `feedback.rule` ✓；`/`、`/healthz`、`/api/health`（经 Nginx）均 200 ✓。⚠️ **未做视觉实测** ✗（该弹窗登录后可见，本机连生产库且 admin 密码已改过 ⇒ 无账号可登 ✓）。<br>此前（`20261012-2212`）：**用户卡片新增「功能反馈」**（侧栏卡片菜单 → **个人信息管理 / 修改密码 / 功能反馈 / 退出登录** ✓）：`account/FeedbackModal`（类型下拉 4 项 + 正文 + 选填联系方式 + 字数计数 ✓，成功态换成"再提一条 / 关闭" ✓）、`api/feedback.ts`、i18n zh/en 各 +17 键（含补 `common.close` ✓）。<br>验收：线上包 **`assets/index-CZbV7yfw.js`** ✓（与本机构建**同名同字节** 440285 B ✓）；新键 `user.feedback` / `feedback.title` / `feedback.submit` / `feedback.ok` / `feedback.rule` 均在包里 ✓（出现 2 次/键）、新文案「功能反馈 / 功能建议 / 体验意见」在包里 ✓；已删键 `password.otherDevices` / `profile.goAccount` / `profile.terms` / `profile.nicknameHint` **全为 0** ✓；`/`、`/healthz`、`/api/health`（经 Nginx）均 200 ✓。<br>此前（`20261011-2349`）：**侧栏底部用户卡片（前端部分）** —— 「用户那一栏」从顶部栏**移到菜单列底部**：新增 `layout/UserCard.tsx`（贴底卡片 + **向上弹**的浮层菜单 ✓；桌面侧栏与移动抽屉**共用同一组件** ✓）、`Sidebar` 改列布局（`mt-auto` 压底）、`MobileNav` 底部同挂一份、`Header` 去掉昵称胶囊与退出图标 ✓；新增 `account/ProfileModal`（账号与绑定 UID 只读 / 昵称可改 ✓）与 `account/PasswordModal`（当前 + 新 + 确认 ✓）。同日按用户要求**三次精简**：标签→「账号」✓、去掉昵称与改密码弹窗里的说明小字 ✓、去掉「去账号信息页 / 查看用户须知」两个按钮 ✓。<br>验收：线上包 **`assets/index-D9v6fzx6.js`** ✓（与本机构建**同名同字节** 433937 B ✓）；新增键 `profile.account`/`user.profile`/`user.password`/`password.mismatch`/`common.cancel`/`auth/profile`/`auth/password` 均在包里 ✓；已删键 `password.otherDevices`/`profile.goAccount`/`profile.terms`/`profile.nicknameHint`/`profile.usernameLabel`/`account.forceRefresh` **全为 0** ✓；`/`、`/healthz`、`/api/health` 均 200 ✓。<br>此前（`20261011-2316`）：**注册页提示改为一行普通小字**（同日二次调整）：上一版把它做成了「描边 + 浅底」的小方块，用户反馈套框反而不好看 ⇒ 改回 `text-[11px] leading-relaxed text-teyvat-text-4` 的一行小字（`0cb1539` ✓）。验收：线上包 `index-DoeSTkV8.js`（改前 `index-CYpp6a-l`）✓、提示原文在包里 ✓、普通小字样式类存在 ✓、**旧的那个方块类名组合已消失** ✓、两句被删文案仍为 False ✓、`/`、`/terms`、`/api/health` 均 200 ✓。<br>同时把这次用的 ECS 发版脚本**收进仓库**：`deploy/load-web-on-ecs.sh`（usage：`bash load-web-on-ecs.sh <tag>` ✓，省得每次现写 ✗）。<br>此前（`20261011-2311`）：**注册页文案调整**：去掉注册态副标题「创建你的旅行者账号」与表单小字「用手机号注册，注册后可直接用手机号登录」，改为小字提示「**该网站需使用一次性邀请码进行注册，相关功能请咨询管理员。**」（`f0eb4bc` ✓）。该提示按 `requireInvite` 显示 —— 线上 `/api/auth/config` 实测 `requireInviteCode:true` ✓ ⇒ 会显示 ✓。<br>验收：线上包 `index-CYpp6a-l.js`（改前 `index-Ckb85XWi`）✓、新提示字符串在包里 ✓、两句旧文案与「路线规划 / 查看图鉴」全为 False ✓、`/`、`/terms`、`/api/health`、`/api/auth/config` 均 200 ✓。⚠️ 仍走 **save → scp（20.2 MB）→ load**（ECS 从未 `docker login` ACR ✓）；本次只发 web ✓（server / ai 无改动 ✓）。<br>此前（`20260928-0109`）：**三件一起发（都是"撤掉没用的东西"）**：① 账号页去掉「**强制刷新（绕过本地缓存）**」入口（`2e03b8c` —— 档案一律走服务端快照，用户侧不需要绕过缓存的开关；接口 `refresh=true` **保留**，运维核对资产还要用、旧 bundle 也还在发它 ✓）；② 首页**撤掉三张功能卡**（智能问答 / 档案查询 / 路线规划，`7987052` —— 它们只是重复左侧导航入口、无新信息）；③ **图鉴从菜单撤下**（`e53a81f` —— 仍是占位页；桌面侧栏与移动抽屉共用 `navItems.ts` 一份来源 ⇒ 两处一起消失；首页「查看图鉴」CTA 一并撤，否则首页指着刚藏起来的页面 ✗；**路由保留**，直接输 `/wiki` 仍可访问 ✓）。验收：线上包 `index-Ckb85XWi.js`（改前 `ByKu1-Dx`）✓、`路线规划/强制刷新/查看图鉴` 字符串全为 False ✓、`/wiki` 出现次数 3→1（路由那 1 处）✓、`/`、`/terms`、`/api/health`、`/api/auth/config` 均 200 ✓。⚠️ **发版过程踩坑**：ECS **从未 `docker login` 过 ACR**（`/root/.docker/` 里只有 buildx、无 `config.json`）⇒ `docker compose pull web` 直接 `pull access denied` ✗、`up -d` 又试图 `build` 而 `/opt/teyvat-guide/web` 不存在 ✗（**线上未受影响** ✓ 旧容器照常跑）。改用 `deploy/README` 记的应急路线② **save → scp（20.2 MB）→ load → tag → up -d --no-build** 成功 ✓。想以后能直接 pull：在 ECS 上手动 `docker login` 一次，或把本机凭据同步过去 ✓。<br>此前（`20260928-0031`）： **注册表单加邀请码栏（在手机号上方）** —— `auth.require_invite_code=true` 时才显示 ✓；**邀请码为空 ⇒「获取验证码」按钮禁用** ✓（服务端同样会拦 ✓ 双保险）；前端只判"非空"、格式与有效性交给服务端判定（避免两边规则漂移 ✗）。**邀请码不填 / 填错 ⇒ 服务端一条短信都不发** ✓<br>此前（`20260928-0021`）：**修注册后自动登录崩溃** —— store 里写 `this.login(...)` ✗：组件里是 `const register = useAuthStore(s => s.register)` 后**裸调用** `register({...})`，`this` 会丢（严格模式 undefined）⇒ 首次真机注册崩在 `Cannot read properties of undefined (reading 'login')` ✗（账号已建成 ✓、却没登上 ✓、须知也没同意 ✓）。改为 `get().login(...)` ✓。**真浏览器实测**：注册 → 自动登录 → 跳到首页 → 弹出须知弹窗 ✓（弹窗只在已登录且须知未同意时出现，等于登录成功的铁证 ✓）<br>此前（`20260927-2352`）：**登录页支持手机号注册** —— 登录/注册双模式（手机号 + 验证码 + 密码 + 昵称可选 ✓）、「获取验证码」带 60s 倒计时（429 时用后端下发的 `retryAfterSeconds` 直接接上 ✓）、注册入口**由服务端 `GET /api/auth/config` 决定**（不硬编码 ✗，否则会有"点了才发现 403"的死按钮 ✗）；`api/client` 的 ApiError 现在带 `code`/`data` ✓<br>此前（`20260927-1610`）：**圣遗物评分提到卡片第一行**（用户第一诉求是"比高低" ⇒ 评分从次要位置提到头部首行 ✓）<br>此前（20260927-1606）：**修圣遗物卡片头部溢出**：评分徽标变长后把末尾的等级 `+20` 挤出卡片（用户反馈 ✗）—— 头部行改 `flex-wrap` + 部位/评分/等级三项都 `shrink-0` + `whitespace-nowrap`，手机端徽标收紧内边距：**放不下就换行，只挪位置不丢信息**（`components/account/RelicDetailCard.tsx`）<br>此前：圣遗物评分展示（单件徽标 + 套装总评/词条/双暴）+ 顶栏副标题修正 + 用户须知弹窗 + `/terms` + AI 标识 + 备案号页脚 + 不参与搜索收录 |
| `teyvat-ai` | **`20261013-0007`** | **上下文预算放开（与 server `20261013-0007` 同批，配套 ✓）** —— AI 侧两处：`system_budget_tokens 3000→8000`、`player_context_budget_tokens 1500→3000` ✓（`app/core/settings.py` 默认值 **+ 线上 `/opt/teyvat-guide/ai-service/.env`** 都要改 ✗：该容器用的是 `env_file` ✓，`.env` 里钉着旧值会把镜像默认值**盖掉** ✓）；顺带 `chat` 档 `max_tokens 1024→2048` ✓（`app/llm/profiles.py`）。⚠️ 改完 `.env` 必须 `docker compose up -d ai-service` 才生效（settings 在启动时读取 ✓）。<br>此前（`20260927-1610`）：**防注入（L2）事前短路守卫**：`app/llm/guard.py` + `orchestrator` 的 `complete()`/`stream()` 双分支勾子 —— 越狱套话 / 现实任务（写代码·周报·翻译等）/ 情感类 / 严重违规**命中即不调用大模型**，直接回固定话术（`model=guard`、`tokens=0`、`offTopic=true` + reason）；守卫侧做 NFKC 归一化去零宽/全角绕过。<br>开关：`AI_GUARD_ENABLED`（默认 true）、`AI_GUARD_MODE`（`block` 默认 / `observe` 灰度 / `off`）<br>⚠️ 发布法：先 `docker build -t …/teyvat-ai:<tag>`，再跑 `deploy/out/deploy-ai-full.ps1`（该脚本只 save/scp/load，**不 build**）<br>此前：修 `context_render` 丢字段（终局概要 + relicScore 进上下文，配套提示词 v7） |

> **回滚**：server 上一版 **`20261014-2219`**（开门人名单补全 ＋ 规则额度再抬 ✓）（2026-10-14 更新 ✓ —— 本批换的就是它 ✓）、web 上一版 **`20261012-2348`**（本批**未动 web** ✓）、ai **镜像未动**（仍是 `20261013-0007` ✓，只改了 `ai-service/.env` 的两个预算值 ⇒ 回滚 = 恢复 `ai-service/.env.bak-20261014-0103` ＋ `docker compose up -d ai-service` ✓，**不用 pull** ✓）。
> ✅ 2026-10-14（**同日第二次发版**，只发 server ✓）：错别字容错热修 ✓（用户报障「伊涅夫怎么配队」✗）—— 本机 build/push ✓ 后 ECS `sed` 改 `SERVER_IMAGE` ＋ `pull && up -d --no-build server` ✓，备份 = `/opt/teyvat-guide/.env.bak-20261014-0112` ✓；digest 核对一致 ✓。⚠️ 本批**只动匹配层** ✗：数据（攻略卡 128 / 机制卡 120 / 规则 5 / 提示词 v12）与 AI 侧预算都不受影响 ✓。
> ✅ 2026-10-14 深夜（**机制卡注入顺序修正**，只发 server ✓ `20261014-2345`）：本机 `SERVER_IMAGE=<新 tag> docker compose build --provenance=false --sbom=false server` ＋ `docker push` ✓ 后，ECS `cp -a .env .env.bak-20261014-2345` ＋ `sed` 改 tag ＋ `docker compose pull server && up -d --no-build server` ✓；上线后 `/api/health` ok ✓、日志无异常 ✓。⚠️ 踩坑：先用 `push-images.ps1 -SkipBuild` 推的是**早先残留的 `teyvat-guide-server:0.1.0`** ✗（那次报的还是老毛病 `unknown manifest class …` ✗）—— 本地 `.env` 的 `SERVER_IMAGE` 是旧 tag ⇒ `compose build` 会把新产物打成**旧名字** ✓，**必须显式覆盖 `SERVER_IMAGE`** ✗（2026-10-14 记 ✓）。
> ✅ 2026-10-14 夜（机制卡上线 ＋ 旅行者拆形态，**只发 server** ✓）：按本节「方式①」直连 pull ✓（`docker compose pull server && up -d --no-build server` ✓）—— 本机 `docker compose build --provenance=false --sbom=false server` → `docker push` ✓，ECS 侧 `sed` 改 `SERVER_IMAGE` ✓；备份 = `/opt/teyvat-guide/.env.bak-20261014-0103` ✓。⚠️ 本批**数据已在库里** ✓（攻略卡 128 行 / 机制卡 120 行 / 规则 5 条 / 提示词 v12 ✓）⇒ 回滚镜像只回滚"怎么用数据"，不回滚数据本身 ✗。
> ✅ 2026-10-13 夜（**角色攻略卡 ① 阶段②**，只发 server ✓）：**已按本节「方式①」直连 pull** ✓（`docker compose pull server && up -d server` ✓，ECS 已 `docker login` ACR ✓）—— 首次不用再 `save → scp → load`（约 90MB ✗），**从改完到上线约 3 分钟** ✓。备份 = `/opt/teyvat-guide/.env.bak-20261013-225102` ✓。
> ⚠️ 攻略卡**不走内存缓存**（与 `statWeights` 等资产不同 ✓）⇒ 后台改完**不需要** `reload?purgeSnapshots=true` ✓。
> ⚠️ 2026-10-13（server + ai **同 tag `20261013-0007`**，上下文预算放开）**已同步本行** ✓ —— 本次备份 = `/opt/teyvat-guide/.env.bak-20261013-001036` + `ai-service/.env.bak-20261013-001036` ✓（回滚 = 恢复这两份 + `docker compose up -d ai-service server`，**不用 pull** ✓）。
> ⚠️ 本批依旧只能走 **save → scp → load**（ECS 从未 `docker login` ACR ✓，见 §5）—— 两个镜像压到 **90MB** ✓（server 14MB + ai 76MB ✓）。**新踩一个坑** ✗：压缩时**不要**写 `tar -czf x.tgz x.tar` ✗（那样 tar 只是包里的**一个成员** ⇒ `docker load` 报 `open /var/lib/docker/tmp/docker-import-*/repositories: no such file or directory` ✓），**直接把镜像 tar gzip 成 `x.tar.gz`** 即可 ✓（docker 自动识别 gzip ✓）。
> ✅ **2026-10-13 补做：ECS 已 `docker login` ACR** ✓ —— `/root/.docker/config.json` 权限 **600**（仅 root ✓）、`docker manifest inspect` + `docker pull …:20261013-0007` 实测均通过 ✓ ⇒ **下一批起走本节「方式①」直接 `docker compose pull`** ✓，不必再 `save → scp → load`（那要传 ~90MB ✗）。
> 说明：密码以**明文（base64）**存在该文件里 ✓ —— 机器上没装 credential helper 时属正常现象 ✓（本机 Docker Desktop 走 `credsStore` 所以无此提示 ✓）；**别把这个文件拷出去/入库** ✗，必要时在控制台**重置访问凭证** ✓。
> ⚠️ 2026-10-12 夜（第三批：server + web **同 tag `20261012-2348`**，修"聊天取不到角色信息"）**已同步本行** ✓。
> 本批顺序刻意是 **先 server 后 web** ✓（前端调的是本批新增的 `/api/genshin/context-status`）—— 反过来会出现"前端调 404"的空窗 ✗（虽然前端对失败是静默的 ✓，但仍不该发 ✗）。
> ⚠️ 2026-10-12 夜（本批发了 web `20261012-2312` + server `20261012-2316`）**已同步本行** ✓ —— 按上一条教训，"上一版"必须**随每次发版一起更新** ✗（否则照着它回滚会跳回更老的版本 ✓）。
> ⚠️ **两个服务若是一起改的，必须一起发** ✗：本次「查询 UID 即绑定」前端先上、后端晚 11 分钟 ⇒ 中间那段时间线上是「界面写明会自动绑、实际不绑，且手动绑定按钮已被新版界面删掉」✗（用户当时**没法绑定** ✓）。教训：**改到"界面上承诺的行为"时，先发后端、再发前端** ✓。
> ⚠️ 2026-10-12 更正：本行此前写着"server 上一版 `20260928-0031`、web 上一版 `20261011-2316`" ✗ ——
> **上一次发版（`20261011-2348/2349`）没有更新本行**，照着它回滚会跳回更老的版本 ✓；本次一并对准 ✓。
> ⚠️ 另：**配额是环境变量级** ⇒ 只想把额度改回 10 万**不必回滚镜像**，改 `.env` 的
> `TEYVAT_QUOTA_DAILY_TOKENS_PER_USER=100000` + `docker compose up -d --no-build server` 即可 ✓。
> 另有环境变量级回滚：AI 侧 `AI_GUARD_MODE=observe|off` 可**不发版**关掉事前短路（只记日志 / 完全关闭）。
> 每次发版前脚本会留一份 `/opt/teyvat-guide/.env.bak-<tag>`；回滚 = 把 `SERVER_IMAGE` / `WEB_IMAGE` 改回上一版 tag + `docker compose up -d --no-build <服务>`。
> ⚠️ 评分权重数据独立于镜像：回滚 server 镜像不影响已导入的权重表；但要重算旧快照需再 `reload?purgeSnapshots=true`。
>
> ⚠️⚠️ **compose 用 `environment` 白名单，不是 `env_file`** ✗ —— 新增任何 `TEYVAT_*` 配置项，
> 必须**同时**写进 `docker-compose.yml` 的 `environment:` 段；只写宿主机 `.env` 容器里**看不到** ✗
> （2026-09-27 加短信配置时踩到：`docker exec teyvat-guide-server env | grep TEYVAT_SMS` 为空 ✓）。
> 另外 `TEYVAT_AUTH_ALLOW_REGISTER` 之前是**写死 `"false"`** ✗，已改成 `${TEYVAT_AUTH_ALLOW_REGISTER:-false}` ✓
> （现在改 `.env` 才真的生效 ✓）。
>
> **手机号注册怎么开**（默认**全关** ✓ —— 没配好时绝不误发短信，发一条就是钱 ✗）：
> 1) `.env`：`TEYVAT_SMS_MOCK=false`（联调期可先 `true`：不发短信、验证码只进服务端日志 ✓）
> 2) 配 AK：`TEYVAT_SMS_ACCESS_KEY_ID` / `TEYVAT_SMS_ACCESS_KEY_SECRET`
>    —— 两者都留空 = 走"**无 AK**"凭据链（ECS 绑了 RAM 角色时用这种 ✓）
> 3) `.env`：`TEYVAT_AUTH_ALLOW_REGISTER=true`
> 4) `docker compose up -d --no-build server`；**回滚 = 把第 3 步改回 false** 再执行第 4 步 ✓
>
> 签名/模板默认 `恒创联众` / `100001`（可在 `.env` 覆盖 ✓）；限速默认 同号 60s / 同号日 10 / 同 IP 时 20 ✓。
>
> ✅ **现状（2026-09-28）：已按方案 A 开启真发短信** —— ECS 绑了 RAM 角色 **`EcsDypnsRole`** ✓，
> `.env` 里显式 AK **留空** ⇒ 代码走容器内的**实例元数据凭据链** ✓（实测容器内可直接取到
> STS 临时凭据 ✓）；`TEYVAT_SMS_MOCK=false`、`TEYVAT_AUTH_ALLOW_REGISTER=true` ✓。
> 首条真发冒烟：`198****8158` 发送成功 ✓（日志 `sms sent`，无阿里云错误码 ✓）。
> ⚠️ 至此**公网上任何人都能注册**（防护：同号 60s / 同号日 10 / 同 IP 时 20 ✓，把刷费框住 ✓）——
> 真正的准入门槛是**邀请码** ✓ —— 已于 **2026-09-28 上线**（`auth.require_invite_code=true` ✓，
> 见 `teyvat-docs/ai/07` TODO-14 与 `shared/04` §2.5；怎么发码见本节末尾 ✓）。
> ⚠️ 凭据链的临时凭据有有效期（实测 6 小时 ✓），SDK 会自动刷新 ✓，无需干预 ✓；RAM 角色的策略必须含
> `dypns:SendSmsVerifyCode` + `dypns:CheckSmsVerifyCode`（或 `AliyunDypnsFullAccess` ✓），否则发送报 `isv.*` 权限错误 ✗。
>
> 🎟 **邀请码怎么发**（2026-09-28 起注册需要邀请码 ✓；开关 `TEYVAT_AUTH_REQUIRE_INVITE_CODE=true`）：
> ```bash
> KEY=$(grep '^TEYVAT_AUTH_INTERNAL_KEY=' /opt/teyvat-guide/.env | cut -d= -f2-)
> # ① 生成 5 张（note 写清给谁/什么渠道，便于对账）
> curl -s -X POST -H "X-Internal-Key: $KEY" -H 'Content-Type: application/json' \
>   -d '{"note":"某渠道 2026-09-28","count":5}' \
>   http://127.0.0.1:8080/internal/v1/admin/invite-codes
> # ② 看还剩几张可用（usable = 可用总数）
> curl -s -H "X-Internal-Key: $KEY" \
>   "http://127.0.0.1:8080/internal/v1/admin/invite-codes?unused=true&limit=20"
> # ③ 收回某张（停用；**不删行**，留痕便于对账）
> curl -s -X PUT -H "X-Internal-Key: $KEY" -H 'Content-Type: application/json' \
>   -d '{"enabled":false}' http://127.0.0.1:8080/internal/v1/admin/invite-codes/<CODE>
> ```
> 口径：**一次性** ✓（注册成功即作废 ✓）；**发短信前就校验** ⇒ 错的码连短信都不发、不花钱 ✓；
> **注册失败（短信码填错 / 手机号已存在）不消耗码** ✓；码 8 位、去掉了易混字符（0/O、1/I 之类 ✓），
> 抄错大小写或带连字符都会被服务端归一化后接受 ✓。
> 💡 只改 AI 上下文文案（如 `relicScore.note`）时**不需要**清快照 —— 上下文是每次请求实时拼的，快照里只存档案数据。
> ⚠️ 但改**会进快照的资产数据**（如推荐词条 `card.statWeights`）时**必须** `reload?purgeSnapshots=true`：
> 原因是 `asset.Service` 有**两个实例**（handler 那份被 `import` 刷新、**PlayerService 那份不会**），
> 且推荐词条会写进档案快照 ⇒ 只 import 不 reload+purge，接口读到的还是旧值（2026-09-27 实测 `0/12` ✗）。

**发一版（三种方式，任选）**：

```bash
# ① 常规：ECS 已 docker login 过 ACR 时最省事
cd /opt/teyvat-guide
sed -i 's|\(SERVER_IMAGE=.*:\).*|\1<新 tag>|' .env
docker compose pull && docker compose up -d

# ② ECS 未登录 ACR：本地 save → scp → load（2026-09-21 两次发版用的都是这条）
docker save -o svr.tar <镜像>:<tag>
scp svr.tar root@<ECS>:/tmp/
ssh root@<ECS> 'docker load -i /tmp/svr.tar && cd /opt/teyvat-guide && docker compose up -d --no-build server'

# ③ 回滚：把 .env 的 tag 改回上一个版本再 up -d（旧镜像仍留在 ECS 上，可直接切回）
```

⚠️ 两个前提：**改完 tag 必须 `docker compose up -d`**（只 `pull` 不会换容器）；**必须带 `--no-build`** —— 否则 compose 找不到构建上下文会回退去 build，报 `unable to prepare context: path "/opt/teyvat-guide/server" not found`。

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
| **容器里**调 AI 服务报 401「缺少 X-AI-Key」/ 管理接口 401 | viper 的 `Unmarshal` 只回填**已知键**（config 文件里出现过的，或 `SetDefault` 注册过的）；`AutomaticEnv` 单独存在的键**它看不见**。容器里没有 `config.yaml`，于是没注册默认值的项**静默失效** | `server/internal/config/config.go` 的 `setDefaults()` 必须为**每个**配置项注册默认值（2026-09-19 已补 `ai.api_key`、`auth.admin_key`、`quota.whitelist_user_ids`）。**新增配置项时务必同步补一行** |
| **容器里**对话报 502「上游模型拒绝了请求」 | `ai.model` 的默认值曾是模板遗留的 `gpt-4o-mini`（本地有 config.yaml 时无感，容器里没有配置 → 直接把它传给 DeepSeek → 400） | 默认已改为**空**（交给 AI 服务决定模型）；要指定就设 `TEYVAT_AI_MODEL` |
| 容器起得来、但每次提问都秒回兜底话术（"派蒙现在有点晕乎乎的…"） | 就是上两条的**共同症状**：环境变量没进到配置里 → 主服务调 AI 服务失败 | 先验变量真进了进程：`docker compose exec server sh -c "printenv TEYVAT_AI_API_KEY | wc -c"`；再看 `docker compose logs server` 的 `ai complete failed` 具体 err |
| `docker push` 到 ACR **最后一步**失败：`error from registry: unknown manifest class for application/vnd.oci.empty.v1+json` | Docker Desktop 的 BuildKit **默认给镜像附加 provenance / SBOM 证明清单**，而 ACR 个人版不认这种 OCI 空清单（现象很迷惑：**所有层都 Pushed**，只有 manifest 报错） | 构建时关掉：`docker compose build --provenance=false --sbom=false`（`deploy/push-images.ps1` 已内置该 flag）。⚠️ **已构建的镜像必须重建后再推**，只重推没用 |
| **首次 `up -d` 时 `web` 反复重启**，日志 `nginx: [emerg] host not found in upstream "server"` | nginx 在**启动瞬间**解析 upstream 主机名，而 `server` 容器此刻还没起来 → 解析失败 nginx 直接退出（**不是 web 镜像坏了**） | **会自愈**：`restart: unless-stopped` 会一直重试，等 `server` 就绪后 web 自己起来（2026-09-20 ECS 实测）。排查时先看 `docker compose ps` 里 **server 的状态**，别被 web 的报错带偏 |
| ECS 上 `docker compose pull` 报 `pull access denied` | ECS 没登录过 ACR（`docker login` 需要交互输密码） | 两条路：① 在 ECS 上跑一次 `docker login --username=<登录名> <ACR地址>`（之后可长期用 `pull`）；② 应急用 `docker save` → `scp` → `docker load`（**无需登录**；本次首部署就是这么做的：三个镜像 tar 仅 109MB）<br>✅ **2026-10-13 已按①解决** ✓：`docker login --username=<阿里云账号> crpi-ecqw70nr0f83o93t.cn-shanghai.personal.cr.aliyuncs.com` **Login Succeeded** ✓（`/root/.docker/config.json` 权限 `600` ✓）；随后 `docker manifest inspect` 与 `docker pull …:20261013-0007` 均通过 ✓ ⇒ **此后优先走①** ✓，②只作应急 ✓。⚠️ 登录时会提示「password will be stored unencrypted」✗ —— 不是错误 ✓，无 credential helper 的机器都这样 ✓；代价是 root 可读明文密码（能接受 ✓，别拷出去 ✗） |
| `docker load -i x.tgz` 报 `open /var/lib/docker/tmp/docker-import-*/repositories: no such file or directory` | **压缩姿势错了** ✗：`tar -czf x.tgz x.tar` 只是把 **tar 当成包里的一个成员**打进去（`tar -tzf x.tgz` 只列出一项 `x.tar` ✓）⇒ docker 拿到看不懂 ✓。2026-10-13 实测踩到 | 直接把镜像 tar **gzip**：`docker save -o x.tar <镜像>` → 压成 **`x.tar.gz`**（Windows 可用 .NET `GZipStream` ✓）→ `docker load -i x.tar.gz` ✓（docker 会自动识别 gzip ✓） |

---

## 6. 备案通过后（还没做，先记着）

1. 域名解析到 ECS + 外层 Nginx（80→443，`server_name` 用你的域名）；
2. 证书（阿里云免费证书或 Let's Encrypt）+ **自动续期**；
3. 安全组开 80/443；把 `web` 容器的端口从 `127.0.0.1:8088` 改成对外映射（或让外层 Nginx 反代 `web:80`）；
4. 页脚挂备案号（链接工信部）；
5. 重跑 `web/11 §6` 的上线前检查清单（HTTPS/证书/到期提醒那几条）。

### 6.1 刻意不参与搜索收录（2026-09-27 决策）

**目标**：邀请制站点，只给受邀用户用 —— **不主动获取自然搜索流量**（不是"隐藏存在"，见下面的边界）。

三处开关（**默认关闭收录**，都已在仓库里）：

| 位置 | 内容 |
| --- | --- |
| `web/public/robots.txt` | `User-agent: *` / `Disallow: /`（Vite 原样拷进 `dist/`，站点根的 `/robots.txt`） |
| `web/index.html` | `<meta name="robots" content="noindex, nofollow, noarchive" />` |
| `web/nginx.conf` | `add_header X-Robots-Tag "noindex, nofollow, noarchive" always;`（server 级 + `/assets/`、`= /index.html` 两处各补一行 —— **nginx 的 `add_header` 不继承进自带 `add_header` 的 location**，改时别漏） |

**要开放收录**：删掉上面三处 → 在搜索引擎站长平台（百度：`ziyuan.baidu.com`）验证站点 → 提交 sitemap。

**已收录怎么撤**：先用站长平台的"移除 URL"工具，或让 noindex 生效后等它自动掉（百度通常 1~4 周）。
⚠️ 注意一个经典悖论：**`robots.txt` 的 `Disallow` 只阻止抓取，不保证 URL 不出现在结果里**（有外链时可能以"无摘要"形式出现）；
而一旦 `Disallow`，爬虫就**看不到** `noindex` 了 ✗。所以要"彻底不被收录"时，正确顺序是：**先允许抓取（临时放开 Disallow）→ 让 noindex 生效 → 再恢复 Disallow**。

**两条边界（别误解这套措施）**：

1. **备案会留下公开记录**：备案通过后域名与主体在**工信部备案系统公开可查**，阿里云也要求页脚挂备案号（法规要求，不能省）。
   所以"没人知道这个域名"做不到 ✗；能做到的是"**搜索引擎搜不到**" ✓。若要连备案公示都没有，唯一办法是**不用境内服务器**（香港/海外 + 域名不备案），代价是国内访问慢、合规风险。
2. **noindex ≠ 访问控制**：它只约束守规矩的搜索引擎。要真正限制访问，靠的是现有的**邀请制**（`TEYVAT_AUTH_ALLOW_REGISTER=false` + 邀请码 + 配额白名单），必要时再加外层 Basic Auth / 安全组白名单。

**配套（不用改代码）**：不要把链接贴到公开渠道（贴吧/B站简介/公开仓库 README）—— **外链是搜索引擎发现新站的主要途径**；也不要在站长平台提交 sitemap。

> 📌 **状态：已改完、尚未发版**（2026-09-27）。登记在 `teyvat-docs/web/06-开发路线图.md` 的「待发版」清单里，
> 与"首页文案删除"一起发同一个 `teyvat-guide-web` 镜像即可。

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
