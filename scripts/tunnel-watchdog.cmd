@echo off
rem Entry point for the Windows scheduled task "TeyvatTunnelWatchdog".
rem Kept as a tiny .cmd wrapper (pure ASCII) because schtasks /TR only takes
rem one string and this repo path contains spaces.
rem The real logic lives in tunnel-watchdog.ps1 next to this file.
powershell.exe -NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -File "%~dp0tunnel-watchdog.ps1" -Quiet
