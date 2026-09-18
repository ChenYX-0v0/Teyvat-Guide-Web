<#
.SYNOPSIS
    提交前 / CI 扫描：防止把密钥、凭据文件、数据库 dump 误提交进仓库。

.DESCRIPTION
    三类检查：
      ① 敏感文件名——.env / .env.* / server/config/config.yaml / deploy/out/ / *.pem / *.key / *.sql dump
      ② 内容里"像真密钥"的赋值——password|secret|api_key|admin_key|internal_key|token = <16 位以上长串>
         并排除占位值（change-me、your-、<...>、example、placeholder、fake、xxxx、TODO…）
      ③ 与【已知真值】精确比对——若存在 deploy/out/known-secrets.txt（本机生成、已 gitignore、
         每行一个真值）则逐条精确匹配。正则只能猜形态，这一条能确定命中，是主要防线。

    扫描范围：
      · 默认只扫【暂存区】（取出的是 git show :path 的暂存内容，即"将要提交的东西"）
      · -All 扫全部被跟踪文件的工作区内容

    命中时**只打印遮蔽后的片段**（前 4 位 + 长度），不完整回显密钥；
    有任何命中就以退出码 1 结束，可直接用于阻断提交 / CI 失败。

.EXAMPLE
    powershell -ExecutionPolicy Bypass -File .\deploy\check-secrets.ps1
    powershell -ExecutionPolicy Bypass -File .\deploy\check-secrets.ps1 -All

.NOTES
    · 与其它部署脚本一致：$ErrorActionPreference 保持 'Continue'，外部命令后显式检查退出码
    · 本文件必须存成 **UTF-8 带 BOM**（PowerShell 5.1 会按 ANSI 读无 BOM 文件，中文注释会乱码/报错）
    · 两个踩过的坑（别改回去）：
        1. 不要用 `git grep -E <正则>` 传复杂正则——PowerShell 传参时会吃掉引号，正则变非法；
           统一在 PowerShell 内部用 .NET 正则匹配。
        2. git 列文件必须带 `-c core.quotepath=false`，否则中文/特殊路径会被引号+八进制转义，
           `Test-Path` 直接报"路径含非法字符"。
#>
param(
    [string]$RepoRoot = (Split-Path -Parent (Split-Path -Parent $PSCommandPath)),
    [switch]$All
)

$ErrorActionPreference = 'Continue'

function Ok($m)   { Write-Host "  [OK]   $m" -ForegroundColor Green }
function Bad($m)  { Write-Host "  [命中] $m" -ForegroundColor Red }
function Info($m) { Write-Host "  [i]    $m" -ForegroundColor Cyan }

$gitCommon = @('-c', 'core.quotepath=false')
function GitRaw([string[]]$args2) { return (& git -C $RepoRoot @($gitCommon + $args2) 2>$null) }

# ---------------------------------------------------------------- 遮蔽：只露前 4 位与长度
function Mask-Secret([string]$s) {
    $t = $s.Trim().Trim('"').Trim("'").TrimEnd(';').TrimEnd(',').Trim()
    if ($t.Length -le 4) { return ('*' * $t.Length) }
    return ($t.Substring(0, 4) + '…（共 ' + $t.Length + ' 位）')
}

# ---------------------------------------------------------------- 规则
$sensitiveFiles = @(
    '(^|/)\.env$',
    '(^|/)\.env\.[^/]+$',                 # .env.local / .env.production …
    '(^|/)server/config/config\.yaml$',
    '(^|/)deploy/out/',
    '\.(pem|key|pfx|p12|jks)$',
    '(?i)(dump|backup).*\.sql$',
    '\.sql\.gz$'
)
$allowFiles = @('\.env\.example$', '\.example$', 'known-secrets\.txt$')

# "像真密钥"的赋值：关键字 + 16 位以上值（引号可有可无，值里允许常见符号）
# ⚠️ 结尾必须加负向先行 `(?![\w\.\(\)\[\]])`：否则 `const token = localStorage.getItem('token')`
#    会把 `localStorage.getItem` 当成"20 位密钥"误报（2026-09-19 实测踩过）。
$contentPattern = [regex]'(?i)\b(password|passwd|pwd|secret|api[_-]?key|admin[_-]?key|internal[_-]?key|access[_-]?key|client[_-]?secret|token|credential)\b\s*[:=]\s*["'']?([A-Za-z0-9_+/=!@#\.\-]{16,})(?![\w\.\(\)\[\]])'
# 明显是代码而非密钥的取值（再兜一层，防同类误报）
$codeValuePattern = [regex]'(?i)^(localStorage|sessionStorage|process\.env|os\.environ|import\.meta|window\.|document\.|require\(|import\()'
# 已知形态的第三方密钥前缀
$vendorPattern = [regex]'(sk-[A-Za-z0-9]{20,}|LTAI[A-Za-z0-9]{8,}|AKIA[A-Z0-9]{12,})'
# 占位/假值标记：命中这些就不算问题
$placeholderPattern = [regex]'(?i)(change[-_ ]?me|your[-_]|_here|<[^>]{2,}>|placeholder|example|sample|fake|dummy|mock|xxxx|todo|redacted|not[-_]set)'
# 二进制/大文件跳过
$binaryExt = @('\.(png|jpe?g|gif|webp|ico|woff2?|ttf|eot|pdf|zip|gz|exe|dll|so|dylib|class|jar|wasm|sqlite|db)$')
$maxBytes = 5MB

# ---------------------------------------------------------------- 目标文件列表
$scopeLabel = if ($All) { '全部被跟踪文件（工作区）' } else { '暂存区（即将提交的内容）' }
Write-Host "==> 扫描密钥：$scopeLabel" -ForegroundColor Cyan

if ($All) { $fileList = @(GitRaw @('ls-files')) }
else      { $fileList = @(GitRaw @('diff','--cached','--name-only','--diff-filter=ACMR')) }
$fileList = @($fileList | Where-Object { $_.Trim() })

if ($fileList.Count -eq 0) {
    Info '没有待检查的文件（暂存区为空？想全量检查请加 -All）'
    exit 0
}

# 取文件内容：暂存区取索引内容（git show :path），全量取磁盘内容；失败返回 $null
function Get-FileText([string]$rel) {
    $low = $rel.ToLower()
    foreach ($e in $binaryExt) { if ($low -match $e) { return $null } }
    if (-not $All) {
        $out = (& git -C $RepoRoot @($gitCommon + @('show', (':' + $rel))) 2>$null)
        if ($LASTEXITCODE -eq 0 -and $out) { return ($out -join "`n") }
    }
    $full = Join-Path $RepoRoot ($rel -replace '/', '\')
    if (-not (Test-Path -LiteralPath $full)) { return $null }
    $fi = Get-Item -LiteralPath $full -ErrorAction SilentlyContinue
    if ($fi -and $fi.Length -gt $maxBytes) { return $null }
    return [System.IO.File]::ReadAllText($full)
}

$problems = 0

# ---------------------------------------------------------------- ① 敏感文件名
Write-Host '  -- ① 敏感文件是否被纳入' -ForegroundColor DarkGray
foreach ($f in $fileList) {
    $skip = $false
    foreach ($a in $allowFiles) { if ($f -match $a) { $skip = $true } }
    if ($skip) { continue }
    foreach ($p in $sensitiveFiles) {
        if ($f -match $p) {
            Bad "$f  ← 敏感文件（规则 $p）；确需入库请 `git add -f` 并在评审里说明"
            $problems++
        }
    }
}

# ---------------------------------------------------------------- ② 内容里的"像真密钥"
Write-Host '  -- ② 内容里是否有"像真密钥"的赋值' -ForegroundColor DarkGray
foreach ($f in $fileList) {
    $skip = $false
    foreach ($a in $allowFiles) { if ($f -match $a) { $skip = $true } }
    if ($skip) { continue }
    $text = Get-FileText $f
    if (-not $text) { continue }
    $lines = $text -split "`n"
    for ($i = 0; $i -lt $lines.Count; $i++) {
        $line = $lines[$i]
        if ($line.Length -gt 4000) { continue }
        if ($placeholderPattern.IsMatch($line)) { continue }
        foreach ($pat in @($contentPattern, $vendorPattern)) {
            if ($pat.IsMatch($line)) {
                $val = $pat.Match($line).Groups[2].Value
                if (-not $val) { $val = $pat.Match($line).Groups[1].Value }
                if (-not $val) { $val = $pat.Match($line).Value }
                if ($codeValuePattern.IsMatch($val.Trim())) { continue }   # 代码取值，不是密钥
                Bad ("{0}:{1}  →  疑似值 {2}" -f $f, ($i + 1), (Mask-Secret $val))
                $problems++
                break
            }
        }
    }
}

# ---------------------------------------------------------------- ③ 已知真值精确比对
$known = Join-Path $RepoRoot 'deploy\out\known-secrets.txt'
if (Test-Path -LiteralPath $known) {
    Write-Host '  -- ③ 与已知真值精确比对（deploy/out/known-secrets.txt）' -ForegroundColor DarkGray
    $secrets = @([System.IO.File]::ReadAllLines($known) |
        ForEach-Object { $_.Trim() } |
        Where-Object { $_ -and -not $_.StartsWith('#') -and $_.Length -ge 8 })
    Info ("读入 {0} 条已知真值（只计数，不回显）" -f $secrets.Count)
    foreach ($f in $fileList) {
        $text = Get-FileText $f
        if (-not $text) { continue }
        for ($i = 0; $i -lt $secrets.Count; $i++) {
            if ($text.Contains($secrets[$i])) {
                Bad ("{0}  ← 命中已知真值第 {1} 条（{2}）" -f $f, ($i + 1), (Mask-Secret $secrets[$i]))
                $problems++
            }
        }
    }
} else {
    Info '未发现 deploy/out/known-secrets.txt（可选：把本机在用的真值逐行写进去，检查会更准）'
}

# ---------------------------------------------------------------- 结论
Write-Host ''
if ($problems -gt 0) {
    Write-Host ("==> 发现 {0} 处问题：把值删干净再提交；若已经提交过，按 shared/05 §5 轮换" -f $problems) -ForegroundColor Red
    exit 1
}
Write-Host '==> 未发现问题 ✅' -ForegroundColor Green
exit 0
