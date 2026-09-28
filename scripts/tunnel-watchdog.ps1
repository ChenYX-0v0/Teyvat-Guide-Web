<#
.SYNOPSIS
    隧道自愈看门狗（2026-09-28）—— 让「本机 → ECS」的 SSH 隧道在断开/假死后自己爬起来。

.DESCRIPTION
    为什么需要：`scripts\tunnel-remote.ps1` 只负责**起**隧道 ✓；但隧道会以两种方式失效 ✗：

      ① 进程没了（ECS 重启 / 网络长断 / ssh 自己退出）；
      ② **进程还在、通道已经死了**（假死）—— 最常见，而且**"看进程在不在"这种检查抓不到** ✗✗。

    ⚠️ 所以本脚本的判据是「**经隧道打真请求**」✓，不是「端口在监听」✗：
    本机 18080/18090/9088/13306 是 **ssh 自己**监听的，通道死了它照样 `Listening` ✗（假象 ✓）。
    真请求必须穿过 本机 ssh → ECS sshd → 目标 才算数 ✓。

      主服务  GET http://127.0.0.1:18080/api/health  → 要 200 且 body 是 {code:0,...} 【强判据】
      AI 服务 TCP 127.0.0.1:18090                   → 只要能连上 【弱判据：AI 服务可能没起】
      C 端    TCP 127.0.0.1:9088                    → 只要能连上
      RDS     TCP 127.0.0.1:13306                   → 只要能连上（ECS 侧再转发到 RDS 内网）

    判定不健康 ⇒ 调 `tunnel-remote.ps1` 重建（**转发定义只有一份** ✓，不在本脚本里复制 ✗）
    ⇒ 复检 ⇒ 记日志。重建修不好也**不会**对着空气狂启 ✓（见 -FailThreshold 与退避说明）。

.PARAMETER Status
    只看状态、不改动（排障时用 ✓）。

.PARAMETER FailThreshold
    连续失败几次才重建（默认 2 ✓）—— 防"抖一下就重启" ✗。

.PARAMETER Quiet
    健康时完全静默（给计划任务用 ✓；发生动作仍然写日志 ✓）。

.PARAMETER InstallTask
    注册计划任务：**登录时启动 + 每 5 分钟检查一次** ✓（不需要管理员权限 ✓）。

.PARAMETER UninstallTask
    卸载该计划任务。

.EXAMPLE
    powershell -ExecutionPolicy Bypass -File scripts\tunnel-watchdog.ps1 -Status
    powershell -ExecutionPolicy Bypass -File scripts\tunnel-watchdog.ps1
    powershell -ExecutionPolicy Bypass -File scripts\tunnel-watchdog.ps1 -InstallTask
#>
[CmdletBinding()]
param(
    [string]$SshHost = 'root@8.159.154.246',
    [string]$KeyPath = 'C:\Users\HONOR\Desktop\teyvat-docs\_DEV.pem',
    [int]$FailThreshold = 2,
    [int]$ProbeTimeoutMs = 6000,
    [switch]$Status,
    [switch]$Quiet,
    [switch]$InstallTask,
    [switch]$UninstallTask
)

$ErrorActionPreference = 'Continue'

# 本脚本所在的 scripts 目录（同目录下就是 tunnel-remote.ps1 ✓）
# ⚠️ 用 $PSCommandPath 而不是 $MyInvocation.MyCommand.Path ✗ —— 后者在**函数内部**会变成
# 那个函数的调用信息（.Path 为空 ★），脚本级变量只在顶层可靠 ✓。
$scriptDir = Split-Path -Parent $PSCommandPath
$tunnelScript = Join-Path $scriptDir 'tunnel-remote.ps1'

# 日志：<仓库根>\logs\tunnel-watchdog.log（.gitignore 有 *.log ✓ 不入仓）
$repoRoot = Split-Path -Parent $scriptDir
$logDir = Join-Path $repoRoot 'logs'
$logFile = Join-Path $logDir 'tunnel-watchdog.log'

# 连续失败计数（跨计划任务调用要持久 ⇒ 落到 logs 下的小文件 ✓）
$stateFile = Join-Path $logDir 'tunnel-watchdog.state'
$taskName = 'TeyvatTunnelWatchdog'

function Write-Log([string]$msg, [string]$level = 'INFO') {
    $line = '{0} [{1}] {2}' -f (Get-Date -Format 'yyyy-MM-dd HH:mm:ss'), $level, $msg
    if (-not $Quiet -or $level -ne 'INFO') { Write-Host $line }
    try {
        if (-not (Test-Path $logDir)) { New-Item -ItemType Directory -Path $logDir -Force | Out-Null }
        Add-Content -Path $logFile -Value $line -Encoding UTF8
        # 简单轮转：超过 ~512KB 就只留最后 2000 行 ✓（日志别无限长 ✗）
        if ((Get-Item $logFile).Length -gt 512KB) {
            $tail = Get-Content $logFile -Tail 2000 -Encoding UTF8
            Set-Content -Path $logFile -Value $tail -Encoding UTF8
        }
    } catch { }
}

function Get-FailCount {
    try { if (Test-Path $stateFile) { return [int](Get-Content $stateFile -Raw -Encoding UTF8).Trim() } } catch { }
    return 0
}
function Set-FailCount([int]$n) {
    try {
        if (-not (Test-Path $logDir)) { New-Item -ItemType Directory -Path $logDir -Force | Out-Null }
        Set-Content -Path $stateFile -Value $n -Encoding UTF8
    } catch { }
}

# 只认"用这把钥匙起的" ssh ⇒ 不误伤别人的隧道 ✓
function Get-TunnelProcess {
    Get-CimInstance Win32_Process -Filter "Name='ssh.exe'" -ErrorAction SilentlyContinue |
        Where-Object { $_.CommandLine -and $_.CommandLine -match [regex]::Escape($KeyPath) }
}

function Test-Tcp([string]$host_, [int]$port) {
    $client = New-Object System.Net.Sockets.TcpClient
    try {
        $task = $client.ConnectAsync($host_, $port)
        if ($task.Wait($ProbeTimeoutMs) -and $client.Connected) { return $true }
        return $false
    } catch { return $false } finally { $client.Close() }
}

function Test-MainService {
    # ⚠️ 用 **curl.exe** 而不是 Invoke-WebRequest ✗（2026-09-28 踩到）：
    # 后者在**计划任务那种新会话**里会做系统代理 / WPAD 自动探测 ⇒ 连 127.0.0.1 都可能失败 ✗
    # （同一份代码我交互式跑通、计划任务里却 exit=1，就是这个原因 ✓）。
    # 本仓库的排障命令本来就一律用 curl.exe ✓ —— 行为确定、无代理魔法 ✓。
    $tmp = Join-Path $env:TEMP ('tw-health-{0}.json' -f $PID)
    try {
        $code = & curl.exe -s -o $tmp -w '%{http_code}' -m ([int][Math]::Ceiling($ProbeTimeoutMs / 1000)) `
            'http://127.0.0.1:18080/api/health' 2>$null
        if ("$code".Trim() -ne '200') { return @{ ok = $false; why = "curl http=$code" } }
        $body = Get-Content $tmp -Raw -Encoding UTF8 | ConvertFrom-Json
        if ($body.code -ne 0) { return @{ ok = $false; why = "code=$($body.code)" } }
        return @{ ok = $true; why = "version=$($body.data.version)" }
    } catch {
        return @{ ok = $false; why = $_.Exception.Message }
    } finally {
        Remove-Item $tmp -ErrorAction SilentlyContinue
    }
}

function Invoke-Probe {
    $results = @()
    $main = Test-MainService
    $results += [pscustomobject]@{ Name = '18080 主服务 /api/health'; Ok = $main.ok; Why = $main.why; Strong = $true }
    $results += [pscustomobject]@{ Name = '18090 AI 服务 (TCP)'; Ok = (Test-Tcp '127.0.0.1' 18090); Why = ''; Strong = $false }
    $results += [pscustomobject]@{ Name = '9088 C 端 (TCP)'; Ok = (Test-Tcp '127.0.0.1' 9088); Why = ''; Strong = $false }
    $results += [pscustomobject]@{ Name = '13306 RDS (TCP)'; Ok = (Test-Tcp '127.0.0.1' 13306); Why = ''; Strong = $false }
    return $results
}

function Show-Probe($results) {
    foreach ($r in $results) {
        $mark = if ($r.Ok) { 'OK  ' } else { if ($r.Strong) { 'FAIL' } else { 'WARN' } }
        $extra = if ($r.Why) { "  ($($r.Why))" } else { '' }
        Write-Host ("    [{0}] {1}{2}" -f $mark, $r.Name, $extra)
    }
}

# ---------------------------------------------------------------- 计划任务

function Install-WatchdogTask {
    # 为什么用 schtasks.exe 而不是 Register-ScheduledTask ✗：
    # 后者即使只注册"当前用户 + Limited"的任务也要求**管理员**（实测 Access is denied ✗），
    # 而 `schtasks /Create` 在普通权限下就能建 ✓（2026-09-28 实测 create/delete exit=0 ✓）。
    $cmdPath = Join-Path $scriptDir 'tunnel-watchdog.cmd'
    if (-not (Test-Path $cmdPath)) {
        Write-Log ("找不到入口 {0} ✗（应与本脚本同目录）" -f $cmdPath) 'ERROR'
        exit 1
    }
    # /TR 需要一个"带引号的完整命令"字符串；路径含空格 ⇒ 必须自己带上引号 ✓
    $tr = '"' + $cmdPath + '"'
    $out = & schtasks.exe /Create /TN $taskName /TR $tr /SC MINUTE /MO 5 /RL LIMITED /F 2>&1
    $code = $LASTEXITCODE
    $out | ForEach-Object { Write-Host ('    ' + $_) }
    if ($code -ne 0) {
        Write-Log ("注册计划任务失败 ✗（exit={0}）：{1}" -f $code, ($out -join ' ')) 'ERROR'
        exit 5
    }
    Write-Log ("已注册计划任务 {0}（每 5 分钟 ✓；schtasks 建的，无需管理员 ✓）" -f $taskName) 'INFO'
    & schtasks.exe /Query /TN $taskName /FO LIST 2>&1 |
        Where-Object { $_ -match '任务名|状态|下次运行|要运行的任务|TaskName|Status|Next Run|Task To Run' } |
        ForEach-Object { Write-Host ('    ' + $_.Trim()) }
}

function Uninstall-WatchdogTask {
    $out = & schtasks.exe /Delete /TN $taskName /F 2>&1
    if ($LASTEXITCODE -eq 0) {
        Write-Log ("已卸载计划任务 {0}" -f $taskName) 'INFO'
    } else {
        Write-Host ('    ' + ($out -join ' '))
    }
}

# ---------------------------------------------------------------- 主流程

if ($InstallTask) { Install-WatchdogTask; exit 0 }
if ($UninstallTask) { Uninstall-WatchdogTask; exit 0 }

# 互斥：计划任务可能与人手同时跑 ⇒ 抢不到就安静退出 ✓（避免两个进程互相踩）
$mutex = New-Object System.Threading.Mutex($false, 'Global\TeyvatTunnelWatchdog')
$hasLock = $false
try { $hasLock = $mutex.WaitOne(0) } catch { $hasLock = $true }
if (-not $hasLock) { Write-Log '另一个看门狗正在跑，本次跳过 ✓' 'INFO'; exit 0 }

try {
    Write-Log "==== 检查开始（failThreshold=$FailThreshold）====" 'INFO'
    $results = Invoke-Probe
    $mainOk = ($results | Where-Object { $_.Strong }).Ok
    $weakOk = (@($results | Where-Object { -not $_.Strong -and $_.Ok }).Count)
    Show-Probe $results

    if ($mainOk) {
        Set-FailCount 0
        Write-Log ("健康 ✓（主服务通；弱判据 {0}/3 通）" -f $weakOk) 'INFO'
        if ($Status) { Write-Host '    ⇒ 结论：隧道健康 ✓' }
        exit 0
    }

    $fails = (Get-FailCount) + 1
    Set-FailCount $fails
    Write-Log ("主服务探测失败（第 {0}/{1} 次）" -f $fails, $FailThreshold) 'WARN'

    if ($Status) { Write-Host '    ⇒ 结论：隧道不健康 ✗（-Status 不改动）'; exit 2 }

    if ($fails -lt $FailThreshold) {
        Write-Log '未达重建阈值，先观察一次（防抖 ✓）' 'WARN'
        exit 1
    }

    $procs = Get-TunnelProcess
    Write-Log ("达到阈值 ⇒ 重建隧道（当前 {0} 个隧道进程）" -f @($procs).Count) 'WARN'
    if (-not (Test-Path $tunnelScript)) {
        Write-Log ("找不到 {0}，无法重建 ✗" -f $tunnelScript) 'ERROR'
        exit 3
    }

    # 重建：优先"重启那个常驻计划任务" ✓ —— 隧道必须是**计划任务的顶层进程**，
    # 否则计划任务结束时会把整棵树回收 ✗（2026-09-28 实测：由本看门狗直接 Start-Process 起的 ssh
    # 只能活到本次任务结束 ⇒ 隧道反复消失 ✗；详见 tunnel-remote.ps1 里 -Foreground 的注释 ✓）。
    $keepTask = 'TeyvatTunnelKeep'
    $hasTask = $false
    try {
        & schtasks.exe /Query /TN $keepTask *> $null
        $hasTask = ($LASTEXITCODE -eq 0)
    } catch { $hasTask = $false }

    if ($hasTask) {
        & schtasks.exe /End /TN $keepTask *> $null
        Start-Sleep -Seconds 1
        & schtasks.exe /Run /TN $keepTask *> $null
        Write-Log ("已重启常驻计划任务 {0} ✓" -f $keepTask) 'INFO'
    } else {
        Write-Log ('没有常驻任务 {0} ⇒ 退回旧方式直接起 ssh（⚠️ 隧道可能活不过本次任务 ✗）' -f $keepTask) 'WARN'
        & powershell -NoProfile -ExecutionPolicy Bypass -File $tunnelScript *>&1 | ForEach-Object { Write-Log ("tunnel-remote: {0}" -f $_) 'INFO' }
    }
    Start-Sleep -Seconds 4

    $after = Invoke-Probe
    $afterMain = ($after | Where-Object { $_.Strong }).Ok
    Show-Probe $after
    if ($afterMain) {
        Set-FailCount 0
        Write-Log '重建成功 ✓ 隧道已恢复' 'INFO'
        exit 0
    }

    Write-Log '重建后仍不通 ✗ —— 见日志上方 tunnel-remote 的输出（常见：私钥不对、ECS 不可达、云端服务没起）' 'ERROR'
    # 失败退避：把计数压在阈值上（不无限增长），下一次计划任务再试一轮 ✓
    Set-FailCount $FailThreshold
    exit 4
} finally {
    if ($hasLock) { try { $mutex.ReleaseMutex() } catch { } }
    $mutex.Dispose()
}
