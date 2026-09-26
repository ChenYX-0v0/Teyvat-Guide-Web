<#
.SYNOPSIS
    本机 → ECS 的 SSH 隧道（访问云端资源：主服务 / AI 服务 / C 端前端 / 阿里云 RDS）。

.DESCRIPTION
    云上的服务都只监听回环（docker -p 127.0.0.1:xxx），RDS 也只有内网地址——
    本机要直连就必须经 ECS 转发。本脚本把四样东西映射到本机：

      本机 18080  → ECS 127.0.0.1:8080   主服务 /api（含 X-Admin-Key 管理接口）
      本机 18090  → ECS 127.0.0.1:8090   AI 派蒙服务
      本机 9088   → ECS 127.0.0.1:8088   C 端前端（Nginx）
      本机 13306  → RDS 内网地址:3306    MySQL（本机 3306 被本地 MySQL 占着，故用 13306）

    RDS 走 -L 转发时，**目标由 ECS 侧解析**，所以这里写的是 RDS 的**内网地址**
    （rm-*.mysql.rds.aliyuncs.com，ECS 解析为 172.x 私网 IP）。

    背景：RDS 外网地址已按计划释放（见 .env 顶部注释），本机直连会 10060 超时。

    ⚠️ 这条隧道只是**通路**：本机应用要不要连生产库，取决于 config.yaml / .env 指向哪 ——
    只有把它们写成 host=127.0.0.1 / port=13306 时，本机的读写才会落到生产 RDS。
    日常开发请让它们指向本机 MySQL，需要查/补云端数据时再切过来（本机配置不入仓）。

.PARAMETER Stop
    只停止隧道，不新建。

.PARAMETER DbHost
    RDS 内网地址。默认脚本内的当前值；换库/换实例时改这里或传参。

.PARAMETER SshHost / KeyPath
    跳板机与私钥路径（换机器请改）。

.EXAMPLE
    powershell -ExecutionPolicy Bypass -File scripts\tunnel-remote.ps1

.EXAMPLE
    powershell -ExecutionPolicy Bypass -File scripts\tunnel-remote.ps1 -Stop
#>
[CmdletBinding()]
param(
    [string]$SshHost = 'root@8.159.154.246',
    [string]$KeyPath = 'C:\Users\HONOR\Desktop\teyvat-docs\_DEV.pem',
    [string]$DbHost = 'rm-uf6o31bzhf9zy5139.mysql.rds.aliyuncs.com',
    [int]$DbPort = 3306,
    [switch]$Stop
)

$ErrorActionPreference = 'Continue'

# 本机端口 → 目标（目标由 ECS 侧解析）
$forwards = @(
    [pscustomobject]@{ Local = 18080; Target = '127.0.0.1:8080'; What = 'main service /api' },
    [pscustomobject]@{ Local = 18090; Target = '127.0.0.1:8090'; What = 'ai-service' },
    [pscustomobject]@{ Local = 9088;  Target = '127.0.0.1:8088'; What = 'web (nginx)' },
    [pscustomobject]@{ Local = 13306; Target = "$DbHost`:$DbPort"; What = 'MySQL (RDS private endpoint)' }
)

function Write-Step($t) { Write-Host "==> $t" -ForegroundColor Cyan }
function Write-Ok($t) { Write-Host "    [OK] $t" -ForegroundColor Green }
function Write-Note($t) { Write-Host "    [!!] $t" -ForegroundColor Yellow }

# 只认"用本脚本这把钥匙起的" ssh，避免误伤别人的隧道
function Get-TunnelProcess {
    Get-CimInstance Win32_Process -Filter "Name='ssh.exe'" -ErrorAction SilentlyContinue |
        Where-Object { $_.CommandLine -and $_.CommandLine -match [regex]::Escape($KeyPath) }
}

function Stop-Tunnel {
    $procs = Get-TunnelProcess
    if (-not $procs) { Write-Host '    没有正在运行的隧道。' -ForegroundColor DarkGray; return 0 }
    $n = 0
    foreach ($p in $procs) {
        Stop-Process -Id $p.ProcessId -Force -ErrorAction SilentlyContinue
        $n++
    }
    Start-Sleep -Seconds 1
    Write-Host ("    已停止 {0} 个隧道进程。" -f $n) -ForegroundColor Yellow
    return $n
}

Write-Step '停止旧隧道（避免端口占用）'
Stop-Tunnel | Out-Null

if ($Stop) { Write-Ok '已按要求只停不起'; exit 0 }

if (-not (Test-Path $KeyPath)) {
    Write-Note "找不到私钥：$KeyPath"
    exit 1
}

# 这些服务云上只监听回环，转发目标必须是 ECS 自己的回环
Write-Step '建立隧道'
$sshArgs = @(
    '-i', $KeyPath,
    '-o', 'StrictHostKeyChecking=no',
    '-o', 'ExitOnForwardFailure=yes',
    '-o', 'ServerAliveInterval=30',
    '-o', 'ServerAliveCountMax=3',
    '-N'
)
foreach ($f in $forwards) { $sshArgs += @('-L', ('{0}:{1}' -f $f.Local, $f.Target)) }
$sshArgs += $SshHost

$proc = Start-Process -FilePath 'ssh' -ArgumentList $sshArgs -PassThru -WindowStyle Hidden
foreach ($f in $forwards) {
    Write-Host ("    本机 {0,-6} -> {1}   ({2})" -f $f.Local, $f.Target, $f.What)
}

Write-Step '等待端口就绪'
foreach ($f in $forwards) {
    $ok = $false
    foreach ($i in 1..30) {
        Start-Sleep -Milliseconds 400
        if (Get-NetTCPConnection -State Listen -LocalPort $f.Local -ErrorAction SilentlyContinue) { $ok = $true; break }
    }
    if ($ok) { Write-Ok ("{0} 监听中" -f $f.Local) } else { Write-Note ("{0} 未监听（隧道进程可能已退出）" -f $f.Local) }
}

if (-not (Get-Process -Id $proc.Id -ErrorAction SilentlyContinue)) {
    Write-Note 'ssh 进程已退出：多半是端口被占或云端拒绝转发。用 ssh -v 手工排查。'
    exit 1
}

Write-Step '校验数据库链路'
$mysql = Get-Command mysql.exe -ErrorAction SilentlyContinue
if (-not $mysql) { $mysql = Get-Command mysql -ErrorAction SilentlyContinue }
if ($mysql) {
    $envFile = Join-Path (Split-Path -Parent $PSScriptRoot) '.env'
    if (Test-Path $envFile) {
        $raw = [System.IO.File]::ReadAllText($envFile)
        $u = [regex]::Match($raw, '(?m)^TEYVAT_DATABASE_USER=(\S+)').Groups[1].Value
        $d = [regex]::Match($raw, '(?m)^TEYVAT_DATABASE_NAME=(\S+)').Groups[1].Value
        $env:MYSQL_PWD = [regex]::Match($raw, '(?m)^TEYVAT_DATABASE_PASSWORD=(\S+)').Groups[1].Value
        # 用自省查询校验（不写死表名）：能拿出版本与表数量即证明链路与权限正常
        $sql = 'select version(), (select count(*) from information_schema.tables where table_schema=database()) as table_count;'
        $out = (& $mysql.Source -h 127.0.0.1 -P 13306 -u $u $d -N -B -e $sql 2>&1 | Out-String).Trim()
        if ($LASTEXITCODE -eq 0) { Write-Ok ('select ok -> ' + $out) } else { Write-Note ('select 失败：' + ($out -split "`n")[0]) }
        Remove-Item Env:\MYSQL_PWD -ErrorAction SilentlyContinue
    }
}
else { Write-Note '本机没有 mysql 客户端，跳过校验（可用 go run 侧验证）' }

Write-Host ''
Write-Host '———————— 隧道已就绪 ————————' -ForegroundColor Cyan
Write-Host '  主服务   http://127.0.0.1:18080'
Write-Host '  AI 服务  http://127.0.0.1:18090'
Write-Host '  C 端     http://127.0.0.1:9088'
Write-Host '  RDS      127.0.0.1:13306   （.env 里 HOST=127.0.0.1 / PORT=13306）'
Write-Host ''
Write-Host '  停止：powershell -ExecutionPolicy Bypass -File scripts\tunnel-remote.ps1 -Stop'
