<#
.SYNOPSIS
    停止本地开发环境（AI 派蒙服务 / 主服务 / C 端前端）。

.DESCRIPTION
    按端口找监听进程并强制停掉，默认停 8080（主服务）、8090（AI 派蒙服务）、5173（C 端前端）；
    加 -WithAdmin 连后台管理系统（3000）一起停。

    ⚠️ 只杀"占端口的那个进程"是不够的：这几条链路都是**父子结构**——
      AI 派蒙服务 = python(监督) → python(工作，占 8090)
      后台        = npm run dev → next dev → start-server(占 3000)
    父进程留着会在下次启动时抢占/干扰，因此这里会**顺着父子链一起停**：
    端口持有者无条件停；其祖先进程**只有命令行匹配本项目的服务特征**时才停
    （`app.main` / `vite.js` / `next dev` / `npm run dev` / `teyvat-guide`），避免误伤别人的进程。

    只动这些进程，不影响 MySQL 服务与其它程序。

.EXAMPLE
    powershell -ExecutionPolicy Bypass -File scripts\stop-all.ps1

.EXAMPLE
    powershell -ExecutionPolicy Bypass -File scripts\stop-all.ps1 -WithAdmin
#>
[CmdletBinding()]
param(
    [int[]]$Ports = @(8080, 8090, 5173),
    [switch]$WithAdmin
)

if ($WithAdmin) { $Ports += 3000 }

# 本项目服务进程的命令行特征（用于判断"父进程要不要一起停"）
$servicePattern = 'app\.main|vite\.js|next"?\s+dev|npm-cli\.js"?\s+run\s+dev|teyvat-guide'

function Stop-ProcessChain([int]$port) {
    $owners = Get-NetTCPConnection -State Listen -LocalPort $port -ErrorAction SilentlyContinue |
        Select-Object -ExpandProperty OwningProcess -Unique
    if (-not $owners) {
        Write-Host ("  端口 {0,-5} 无监听" -f $port) -ForegroundColor DarkGray
        return 0
    }

    $count = 0
    foreach ($ownerId in $owners) {
        $current = $ownerId
        $depth = 0
        while ($current -and $depth -lt 6) {
            $depth++
            $proc = Get-CimInstance Win32_Process -Filter "ProcessId=$current" -ErrorAction SilentlyContinue
            if (-not $proc) { break }

            $isOwner = ($depth -eq 1)
            $isOurs = ($proc.CommandLine -and $proc.CommandLine -match $servicePattern)
            if ($isOwner -or $isOurs) {
                Stop-Process -Id $proc.ProcessId -Force -ErrorAction SilentlyContinue
                if ($isOwner) {
                    Write-Host ("  端口 {0,-5} 已停止进程 {1}（{2}）" -f $port, $proc.ProcessId, $proc.Name) -ForegroundColor Yellow
                }
                else {
                    Write-Host ("        连带停止父进程 {0}（{1}）" -f $proc.ProcessId, $proc.Name) -ForegroundColor DarkYellow
                }
                $count++
            }
            $current = $proc.ParentProcessId
        }
    }
    return $count
}

$stopped = 0
foreach ($port in $Ports) { $stopped += (Stop-ProcessChain $port) }

if ($stopped -gt 0) {
    Start-Sleep -Seconds 1
    Write-Host ("已停止 {0} 个进程。" -f $stopped) -ForegroundColor Green
}
else {
    Write-Host '没有需要停止的进程。' -ForegroundColor Green
}
