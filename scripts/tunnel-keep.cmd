@echo off
rem ---------------------------------------------------------------------------
rem Scheduled-task entry point: keep the SSH tunnel alive as a TOP-LEVEL process.
rem
rem Task name: TeyvatTunnelKeep   (registered by: scripts\tunnel-keep-setup.ps1)
rem
rem Why a wrapper instead of running ssh directly: the forwards must stay defined
rem in ONE place (scripts\tunnel-remote.ps1) - this file only starts it in
rem foreground mode, where ssh runs inside that PowerShell process, so the tunnel
rem is owned by the scheduled task itself and is not reaped when a run ends.
rem
rem The 5-minute watchdog (scripts\tunnel-watchdog.ps1) probes real requests
rem through the tunnel and restarts THIS task when the tunnel is unhealthy.
rem ---------------------------------------------------------------------------
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0tunnel-remote.ps1" -Foreground
