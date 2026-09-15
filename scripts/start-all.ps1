<#
.SYNOPSIS
    一键启动本地开发环境（AI 派蒙服务 + 主服务 + C 端前端）。

.DESCRIPTION
    依次启动三件套，并等到各自健康就绪：
      1) AI 派蒙服务      http://127.0.0.1:8090   （只监听回环，由主服务调用）
      2) 主服务（Go）     http://127.0.0.1:8080   （C 端的 /api 由 Vite 代理过来）
      3) C 端前端（Vite） http://localhost:5173   （--host 0.0.0.0，手机同 WiFi 可访问）

    行为：
      - 端口被占用时先停掉旧进程（本地开发常有残留进程）
      - 默认重新构建主服务（产物落在 %TEMP%\teyvat-run\，不进仓库）；-SkipBuild 可跳过
      - 每个服务都带就绪等待与超时；起不来时自动打印它的日志尾部，便于定位
      - 日志统一写进 %TEMP%\teyvat-run\（main/ai/web 的 .out/.err）

.PARAMETER AiServicePath
    AI 派蒙服务（Python）目录。默认指向桌面上的独立仓库；换机器请改这里。
.PARAMETER AdminPath
    后台管理系统（demo-app，Next.js）目录，仅在 -WithAdmin 时用到。
.PARAMETER SkipBuild
    跳过 go build，直接用上次产物启动。
.PARAMETER WithAdmin
    额外启动后台管理系统（默认 3000）。

.EXAMPLE
    powershell -ExecutionPolicy Bypass -File scripts\start-all.ps1

.EXAMPLE
    powershell -ExecutionPolicy Bypass -File scripts\start-all.ps1 -SkipBuild

.EXAMPLE
    powershell -ExecutionPolicy Bypass -File scripts\start-all.ps1 -WithAdmin
#>
[CmdletBinding()]
param(
    [string]$AiServicePath = 'C:\Users\HONOR\Desktop\AI派蒙服务器\ai-service',
    [string]$AdminPath = 'C:\Users\HONOR\Desktop\demo_in_windows\demo-app',
    [int]$AiPort = 8090,
    [int]$ServerPort = 8080,
    [int]$WebPort = 5173,
    [int]$AdminPort = 3000,
    [switch]$SkipBuild,
    [switch]$WithAdmin
)

$ErrorActionPreference = 'Stop'

$repoRoot = Split-Path -Parent $PSScriptRoot
$serverDir = Join-Path $repoRoot 'server'
$webDir = Join-Path $repoRoot 'web'
$runDir = Join-Path $env:TEMP 'teyvat-run'
$serverExe = Join-Path $runDir 'teyvat-guide-dev.exe'

function Write-Step($text) { Write-Host "==> $text" -ForegroundColor Cyan }
function Write-Ok($text) { Write-Host "    [OK] $text" -ForegroundColor Green }
function Write-Note($text) { Write-Host "    [!!] $text" -ForegroundColor Yellow }
function Fail($text) { Write-Host "    [错误] $text" -ForegroundColor Red; exit 1 }

function Stop-Port($port) {
    $owners = Get-NetTCPConnection -State Listen -LocalPort $port -ErrorAction SilentlyContinue |
        Select-Object -ExpandProperty OwningProcess -Unique
    foreach ($procId in $owners) {
        Stop-Process -Id $procId -Force -ErrorAction SilentlyContinue
        Write-Note "端口 $port 已被占用，已停掉旧进程 $procId"
    }
    if ($owners) { Start-Sleep -Seconds 2 }
}

# 日志被运行中的进程独占，必须用 FileShare.ReadWrite 读（否则报"正由另一进程使用"）
function Get-LogTail($path, $lines = 12) {
    if (-not (Test-Path $path)) { return '(无日志)' }
    try {
        $fs = [System.IO.File]::Open($path, 'Open', 'Read', 'ReadWrite')
        $sr = New-Object System.IO.StreamReader($fs)
        $text = $sr.ReadToEnd(); $sr.Close(); $fs.Close()
        $tail = $text -split "`n" | Where-Object { $_.Trim() -ne '' } | Select-Object -Last $lines
        return ($tail -join "`n")
    }
    catch { return '(日志读取失败：' + $_.Exception.Message + ')' }
}

function Wait-Http($url, $seconds) {
    $deadline = (Get-Date).AddSeconds($seconds)
    while ((Get-Date) -lt $deadline) {
        try {
            $r = Invoke-WebRequest -Uri $url -TimeoutSec 3 -UseBasicParsing -ErrorAction Stop
            if ($r.StatusCode -ge 200 -and $r.StatusCode -lt 500) { return $true }
        }
        catch { }
        Start-Sleep -Milliseconds 700
    }
    return $false
}

# ---------------------------------------------------------------- 0) 前置检查
Write-Step '检查环境'
if (-not (Test-Path $serverDir)) { Fail "主服务目录不存在：$serverDir" }
if (-not (Test-Path $webDir)) { Fail "C 端前端目录不存在：$webDir" }
if (-not (Test-Path $AiServicePath)) { Fail "AI 派蒙服务目录不存在：$AiServicePath（用 -AiServicePath 指定）" }
$python = Join-Path $AiServicePath '.venv\Scripts\python.exe'
if (-not (Test-Path $python)) { Fail "找不到虚拟环境：$python（先在 AI 服务目录建好 .venv）" }
if ($WithAdmin -and -not (Test-Path $AdminPath)) { Fail "后台目录不存在：$AdminPath（用 -AdminPath 指定）" }
New-Item -ItemType Directory -Force -Path $runDir | Out-Null

$mysql = Get-Service -Name 'MySQL84' -ErrorAction SilentlyContinue
if ($mysql -and $mysql.Status -ne 'Running') {
    Write-Note 'MySQL84 未运行——主服务连不上数据库（可执行 Start-Service MySQL84）'
}
elseif ($mysql) { Write-Ok 'MySQL84 正在运行' }
else { Write-Note '未找到名为 MySQL84 的服务（用别的实例名/远程库时可忽略）' }

# ---------------------------------------------------------------- 1) 腾端口
Write-Step "腾出端口（$ServerPort / $AiPort / $WebPort）"
Stop-Port $ServerPort
Stop-Port $AiPort
Stop-Port $WebPort
if ($WithAdmin) { Stop-Port $AdminPort }

# ---------------------------------------------------------------- 2) 构建主服务
if (-not $SkipBuild) {
    Write-Step '构建主服务（go build）'
    Push-Location $serverDir
    try { & go build -o $serverExe ./main.go } finally { Pop-Location }
    if ($LASTEXITCODE -ne 0 -or -not (Test-Path $serverExe)) { Fail 'go build 失败，见上面的编译错误' }
    Write-Ok "产物：$serverExe"
}
elseif (-not (Test-Path $serverExe)) {
    Fail "没有现成产物（$serverExe）：去掉 -SkipBuild 先构建一次"
}

# ---------------------------------------------------------------- 3) AI 派蒙服务
Write-Step "启动 AI 派蒙服务（:$AiPort）"
$aiOut = Join-Path $runDir 'ai.out'
$aiErr = Join-Path $runDir 'ai.err'
$aiProc = Start-Process -FilePath $python -ArgumentList '-m', 'app.main' -WorkingDirectory $AiServicePath `
    -PassThru -RedirectStandardOutput $aiOut -RedirectStandardError $aiErr -WindowStyle Hidden
if (Wait-Http "http://127.0.0.1:$AiPort/health" 45) {
    Write-Ok "就绪（PID $($aiProc.Id)）"
}
else {
    Write-Note "未就绪，日志尾部：`n$(Get-LogTail $aiErr)"
}

# ---------------------------------------------------------------- 4) 主服务
Write-Step "启动主服务（:$ServerPort）"
$mainOut = Join-Path $runDir 'main.out'
$mainErr = Join-Path $runDir 'main.err'
$mainProc = Start-Process -FilePath $serverExe -WorkingDirectory $serverDir `
    -PassThru -RedirectStandardOutput $mainOut -RedirectStandardError $mainErr -WindowStyle Hidden
if (Wait-Http "http://127.0.0.1:$ServerPort/api/health" 30) {
    Write-Ok "就绪（PID $($mainProc.Id)）"
}
else {
    Fail "主服务未就绪，日志尾部：`n$(Get-LogTail $mainErr)"
}

# ---------------------------------------------------------------- 5) C 端前端
Write-Step "启动 C 端前端（:$WebPort）"
$webOut = Join-Path $runDir 'web.out'
$webErr = Join-Path $runDir 'web.err'
$webProc = Start-Process -FilePath 'node' `
    -ArgumentList 'node_modules/vite/bin/vite.js', '--host', '0.0.0.0', '--port', "$WebPort" `
    -WorkingDirectory $webDir -PassThru -RedirectStandardOutput $webOut -RedirectStandardError $webErr -WindowStyle Hidden
if (Wait-Http "http://127.0.0.1:$WebPort" 45) {
    Write-Ok "就绪（PID $($webProc.Id)）"
}
else {
    Write-Note "未就绪，日志尾部：`n$(Get-LogTail $webErr)"
}

# ---------------------------------------------------------------- 6) 可选：后台
$adminProc = $null
if ($WithAdmin) {
    Write-Step "启动后台管理系统（:$AdminPort）"
    $adminOut = Join-Path $runDir 'admin.out'
    $adminErr = Join-Path $runDir 'admin.err'
    # npm 是 .cmd 批处理，Start-Process 不能直接起（会报 "%1 is not a valid Win32 application"），
    # 必须经 cmd.exe /c 包一层
    $adminProc = Start-Process -FilePath 'cmd.exe' -ArgumentList '/c', 'npm', 'run', 'dev' -WorkingDirectory $AdminPath `
        -PassThru -RedirectStandardOutput $adminOut -RedirectStandardError $adminErr -WindowStyle Hidden
    if (Wait-Http "http://127.0.0.1:$AdminPort" 90) {
        Write-Ok "就绪（PID $($adminProc.Id)）"
    }
    else {
        Write-Note "未就绪（dev server 首次编译较慢，可稍后手动打开），日志尾部：`n$(Get-LogTail $adminErr)"
    }
}

# ---------------------------------------------------------------- 7) 汇总
$lan = Get-NetIPAddress -AddressFamily IPv4 -ErrorAction SilentlyContinue |
    Where-Object { $_.IPAddress -notlike '127.*' -and $_.IPAddress -notlike '169.254.*' } |
    Select-Object -First 1 -ExpandProperty IPAddress

Write-Host ''
Write-Host '———————— 服务已启动 ————————' -ForegroundColor Cyan
Write-Host ("  C 端前端     http://localhost:{0}" -f $WebPort)
if ($lan) { Write-Host ("               （手机同 WiFi：http://{0}:{1}）" -f $lan, $WebPort) }
Write-Host ("  主服务       http://127.0.0.1:{0}" -f $ServerPort)
Write-Host ("  AI 派蒙服务  http://127.0.0.1:{0}   （只监听回环，不对外）" -f $AiPort)
if ($WithAdmin) { Write-Host ("  后台管理     http://localhost:{0}" -f $AdminPort) }
Write-Host ''
Write-Host ("  日志目录：{0}" -f $runDir)
Write-Host '  停止服务：powershell -ExecutionPolicy Bypass -File scripts\stop-all.ps1'
Write-Host ''
Write-Note '匿名问 AI 会 401（设计如此：匿名只能看预设问答）；公开注册已关闭，新账号请在后台「用户管理 → 新增用户」创建。'
