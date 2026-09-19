<#
.SYNOPSIS
    本地构建三个镜像并推送到 ACR（M2「路 B」：ECS 只拉不编译）。

.DESCRIPTION
    为什么推荐这条路：
      · ECS 不必装 Go / Node，不占 CPU 与内存，部署只需 docker pull
      · 镜像带 tag，回滚就是改一个 tag
      · 构建失败不影响线上
    前置：本机 Docker Desktop 可用，且**已配好镜像加速器**（否则拉基础镜像会失败，
          实测 hub.docker.com 在本机不可达，见 deploy/README.md §1.3）

.EXAMPLE
    .\deploy\push-images.ps1 -Registry crpi-xxxx.cn-hangzhou.personal.cr.aliyuncs.com -Namespace my-ns
    # ACR 地址照抄控制台「个人版实例 → 仓库管理 → 访问凭证」：新版是 crpi-xxxx.cn-<地域>.personal.cr.aliyuncs.com
#>
param(
    [Parameter(Mandatory = $true)][string]$Registry,          # 如 crpi-xxxx.cn-hangzhou.personal.cr.aliyuncs.com（控制台「访问凭证」里照抄）
    [Parameter(Mandatory = $true)][string]$Namespace,         # ACR 命名空间
    [string]$Tag = (Get-Date -Format 'yyyyMMdd-HHmm'),
    [string[]]$Images = @('teyvat-ai:0.1.0', 'teyvat-guide-server:0.1.0', 'teyvat-guide-web:0.1.0'),
    [switch]$SkipBuild
)

# ⚠️ 刻意不用 'Stop'：docker 的构建/推送进度全部走 stderr，EAP=Stop 下会被当成终止性错误
# 而中断（与 db-export.ps1 同一坑）。用 'Continue' + 显式检查 $LASTEXITCODE。
$ErrorActionPreference = 'Continue'
function Fail($msg) { Write-Host "    [错误] $msg" -ForegroundColor Red; exit 1 }
function Ok($msg)   { Write-Host "    [OK] $msg" -ForegroundColor Green }
function Note($msg) { Write-Host "    [!!] $msg" -ForegroundColor Yellow }

$repoRoot = Split-Path -Parent $PSScriptRoot
$composeFile = Join-Path $repoRoot 'docker-compose.yml'

Write-Host '==> 前置检查' -ForegroundColor Cyan
if (-not (Get-Command docker -ErrorAction SilentlyContinue)) { Fail '本机没有 docker（需装 Docker Desktop 并配镜像加速）' }
if (-not (Test-Path $composeFile)) { Fail "找不到 $composeFile" }
if ($Namespace -cmatch '[A-Z]') { Note "命名空间 $Namespace 含大写字母：ACR 仓库名必须全小写，推送可能失败" }
Ok "docker 可用；compose：$composeFile"
Ok "目标：$Registry/$Namespace（tag=$Tag）"

# compose 里用了 ${VAR:?} 强校验，构建时也要能解析出值。
# 这里只给"构建期占位"，不会进入任何容器（build 不创建容器）；已有真值就不覆盖。
$placeholders = @{
    'TEYVAT_DATABASE_PASSWORD' = 'build-only-placeholder'
    'AI_SERVICE_KEY'           = 'build-only-placeholder'
    'TEYVAT_AUTH_JWT_SECRET'   = 'build-only-placeholder'
    'TEYVAT_AUTH_ADMIN_KEY'    = 'build-only-placeholder'
    'TEYVAT_AUTH_INTERNAL_KEY' = 'build-only-placeholder'
    'TEYVAT_AUTH_SEED_PASS'    = 'build-only-placeholder'
}
foreach ($k in $placeholders.Keys) {
    if (-not (Get-Item "Env:$k" -ErrorAction SilentlyContinue)) {
        Set-Item "Env:$k" $placeholders[$k]
    }
}
$aiContext = Join-Path (Split-Path -Parent $repoRoot) 'AI派蒙服务器\ai-service'
if (-not (Test-Path (Join-Path $aiContext '.env'))) {
    Note "AI 服务目录缺少 .env（$aiContext\.env）：compose 的 env_file 指向它，pull/up 时会报错；先按 ai-service/.env.example 建好"
}

# ---------------------------------------------------------------- 构建
Push-Location $repoRoot
try {
    if (-not $SkipBuild) {
        Write-Host '==> 构建三个镜像（docker compose build）' -ForegroundColor Cyan
        # ⚠️ --provenance=false --sbom=false 是必需的，不是可选项：
        #   Docker Desktop 的 BuildKit 默认给镜像附加「来源证明（provenance）/ SBOM」清单，
        #   而 ACR 个人版不认这种 OCI 空清单，推送最后一步会失败：
        #     error from registry: unknown manifest class for application/vnd.oci.empty.v1+json
        #   （2026-09-20 实测：所有层都 Pushed，唯独 manifest 报这个错）
        docker compose build --provenance=false --sbom=false
        if ($LASTEXITCODE -ne 0) { Fail 'docker compose build 失败（看上面的输出）' }
        Ok '构建完成'
    } else {
        Note '按 -SkipBuild 跳过构建，直接推本地已有镜像'
    }

    # ---------------------------------------------------------------- 打标 + 推送
    Write-Host '==> 打标并推送' -ForegroundColor Cyan
    foreach ($img in $Images) {
        $name = ($img -split ':')[0]
        $remote = "$Registry/$Namespace/${name}:$Tag"
        docker tag $img $remote
        if ($LASTEXITCODE -ne 0) { Fail "打标失败：$img → $remote" }
        docker push $remote
        if ($LASTEXITCODE -ne 0) { Fail "推送失败：$remote（先 docker login $Registry）" }
        Ok "已推送 $remote"
    }
} finally {
    Pop-Location
}

Write-Host ''
Write-Host 'ECS 侧（deploy/README.md §2.2）：把下面三行加进 /opt/teyvat-guide/.env' -ForegroundColor Cyan
Write-Host "    AI_IMAGE=$Registry/$Namespace/teyvat-ai:$Tag"
Write-Host "    SERVER_IMAGE=$Registry/$Namespace/teyvat-guide-server:$Tag"
Write-Host "    WEB_IMAGE=$Registry/$Namespace/teyvat-guide-web:$Tag"
Write-Host ''
Write-Host '然后：docker compose pull && docker compose up -d' -ForegroundColor Cyan
