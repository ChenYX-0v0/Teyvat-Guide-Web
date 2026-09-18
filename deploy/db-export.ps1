<#
.SYNOPSIS
    导出本地 teyvat_guide 库，供迁移到阿里云（M3）。

.DESCRIPTION
    · 连接信息**从 server/config/config.yaml 读**（该文件含密码且未入库，这里只读不打印密码）
    · 用 mysqldump 的 --result-file 落盘：**不要用 PowerShell 的 `>`**，那会把文件写成 UTF-16
    · 同时输出关键表行数基线（rows-local.txt），云上导入后与 db-verify.sql 对照
    · 导出内容含 CREATE DATABASE + USE（--databases），云上可直接整体导入

.EXAMPLE
    .\deploy\db-export.ps1
    .\deploy\db-export.ps1 -MysqlBin 'C:\Program Files\MySQL\MySQL Server 8.4\bin'
#>
param(
    [string]$ConfigPath = (Join-Path (Split-Path -Parent $PSScriptRoot) 'server\config\config.yaml'),
    [string]$MysqlBin = '',
    [string]$OutDir = (Join-Path $PSScriptRoot 'out')
)

# ⚠️ 这里刻意**不用** 'Stop'：mysqldump / mysql 会把「Using a password on the command line
# can be insecure」这类提示写到 stderr，在 EAP=Stop 下 PowerShell 5.1 会把它当终止性错误、
# 在第一条命令就中断（实测踩过）。所以统一用 'Continue'，并**每个外部命令后显式检查
# $LASTEXITCODE** —— 少一次检查就是少一次"静默失败"。
$ErrorActionPreference = 'Continue'

function Fail($msg) { Write-Host "    [错误] $msg" -ForegroundColor Red; exit 1 }
function Ok($msg)   { Write-Host "    [OK] $msg" -ForegroundColor Green }
function Note($msg) { Write-Host "    [!!] $msg" -ForegroundColor Yellow }

Write-Host '==> 准备导出环境' -ForegroundColor Cyan

if (-not (Test-Path $ConfigPath)) { Fail "找不到配置文件：$ConfigPath" }
if (-not $MysqlBin) {
    foreach ($c in @('C:\Program Files\MySQL\MySQL Server 8.4\bin', 'C:\Program Files\MySQL\MySQL Server 8.0\bin')) {
        if (Test-Path (Join-Path $c 'mysqldump.exe')) { $MysqlBin = $c; break }
    }
}
if (-not $MysqlBin -or -not (Test-Path (Join-Path $MysqlBin 'mysqldump.exe'))) {
    Fail "找不到 mysqldump.exe。用 -MysqlBin '<MySQL 的 bin 目录>' 指定"
}
$dump = Join-Path $MysqlBin 'mysqldump.exe'
$mysql = Join-Path $MysqlBin 'mysql.exe'
Ok "客户端目录：$MysqlBin"

# ---------------------------------------------------------------- 读连接信息（不打印密码）
$cfg = [System.IO.File]::ReadAllText($ConfigPath)
$block = [regex]::Match($cfg, '(?ms)^database:\s*\n(?<b>(?:[ \t]+[^\n]*\n?)+)').Groups['b'].Value
function Get-Key($k) {
    $v = [regex]::Match($block, "(?m)^\s*$k\s*:\s*(.+)$").Groups[1].Value.Trim().Trim('"').Trim("'")
    return ($v -replace '\s+#.*$', '').Trim()
}
$dbHost = Get-Key 'host'; if (-not $dbHost) { $dbHost = '127.0.0.1' }
$dbPort = Get-Key 'port'; if (-not $dbPort) { $dbPort = '3306' }
$dbUser = Get-Key 'user'
$dbPass = Get-Key 'password'
$dbName = Get-Key 'name'
if (-not $dbName) { Fail 'config.yaml 里没有 database.name' }
Ok "目标：$dbUser@$dbHost`:$dbPort/$dbName"

New-Item -ItemType Directory -Force -Path $OutDir | Out-Null
$stamp = Get-Date -Format 'yyyyMMdd-HHmm'
$sqlFile = Join-Path $OutDir "$dbName-$stamp.sql"
$rowsFile = Join-Path $OutDir 'rows-local.txt'

$conn = @("--host=$dbHost", "--port=$dbPort", "--user=$dbUser", "--password=$dbPass", '--default-character-set=utf8mb4')

# ---------------------------------------------------------------- 导出
Write-Host '==> 导出（结构 + 数据 + 存储过程/触发器）' -ForegroundColor Cyan
& $dump @conn '--single-transaction' '--routines' '--triggers' '--set-gtid-purged=OFF' '--databases' $dbName "--result-file=$sqlFile" 2>&1 |
    Where-Object { $_ -notmatch 'password on the command line' }
if ($LASTEXITCODE -ne 0 -or -not (Test-Path $sqlFile)) { Fail 'mysqldump 失败（看上面的输出）' }
$sizeMb = [math]::Round((Get-Item $sqlFile).Length / 1MB, 2)
Ok "导出完成：$sqlFile（$sizeMb MB）"

# ---------------------------------------------------------------- 行数基线
Write-Host '==> 采集行数基线（云上导入后用 db-verify.sql 对照）' -ForegroundColor Cyan
$tables = & $mysql @conn "--database=$dbName" '--batch' '--skip-column-names' '-e' 'SHOW TABLES;' 2>&1 |
    Where-Object { $_ -notmatch 'password on the command line' -and $_.Trim() }
$lines = @("# teyvat_guide 行数基线（$stamp）—— 与 deploy/db-verify.sql 的期望值对齐")
foreach ($t in $tables) {
    $n = & $mysql @conn "--database=$dbName" '--batch' '--skip-column-names' '-e' "SELECT COUNT(*) FROM ``$t``;" 2>&1 |
        Where-Object { $_ -match '^\d+$' } | Select-Object -First 1
    $lines += ('{0,-30} {1}' -f $t, $n)
    Write-Host ('    {0,-30} {1}' -f $t, $n)
}
[System.IO.File]::WriteAllLines($rowsFile, $lines, (New-Object System.Text.UTF8Encoding($false)))
Ok "基线已写入：$rowsFile"

Write-Host ''
Write-Host '下一步（deploy/README.md §3.3）：' -ForegroundColor Cyan
Write-Host "  1) 上传 $sqlFile 到 ECS"
Write-Host "  2) mysql --host=<内网地址> --user=<用户> -p < $sqlFile"
Write-Host '  3) mysql ... teyvat_guide < deploy/db-verify.sql   # 无输出 = 全部对得上'
