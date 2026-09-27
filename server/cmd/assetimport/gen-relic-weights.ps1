# gen-relic-weights.ps1 —— 生成圣遗物评分权重种子（relic_weights_miao.json / relic_weight_rules_miao.json）
#
# 数据来源：miao-plugin（GitHub: yoimiya-kokomi/miao-plugin）
#   · resources/meta-gs/artifact/artis-mark.js   → 逐角色默认权重（121 条，0~100，十维）
#   · resources/meta-gs/character/<角色>/artis.js → 角色级覆盖规则
#   · 许可：MIT License, Copyright (c) 2023 Yoimiya —— 允许使用/修改/再分发，
#     义务：保留版权与许可声明（本文件与生成的种子 JSON 均保留 ✓）
#   · 快照锚点：artis-mark.js@a6200801（2026-09-23 上游最后改动），抓取日期见 FETCHED_AT
#
# 用法：pwsh -File gen-relic-weights.ps1            （在仓库根目录或任意目录均可）
# 产出：server/cmd/assetimport/relic_weights_miao.json
#       —— 一份即可直接喂给 POST /api/admin/assets/import（键名 = ImportPayload 的 json tag）
#
# 设计见 teyvat-docs/ai/08-圣遗物评分设计与落地.md
# 注意：本脚本只做"翻译"，不做判断；ID 对齐以 character_stat_weights 的 notes（= 库里 characters）为准。

$ErrorActionPreference = 'Stop'
$SOURCE_REF = 'artis-mark.js@a6200801'
$FETCHED_AT = '2026-09-27'
$root = (Resolve-Path (Join-Path $PSScriptRoot '..\..\..')).Path
$seedDir = $PSScriptRoot

# ── 1. miao 默认权重：名字,hp,atk,def,cpct,cdmg,mastery,dmg,phy,recharge,heal ──
$miaoCsv = @'
芭芭拉,100,0,0,0,0,0,0,0,55,100
甘雨,0,75,0,100,100,75,100,0,0,0
雷电将军,0,75,0,100,100,0,75,0,90,0
神里绫人,50,75,0,100,100,0,100,0,30,0
八重神子,0,75,0,100,100,75,75,0,55,0
申鹤,0,100,0,0,0,0,0,0,100,0
云堇,0,0,100,0,0,0,0,0,80,0
荒泷一斗,0,50,100,100,100,0,100,0,30,0
五郎,0,0,75,0,0,0,0,0,100,0
班尼特,100,0,0,0,0,0,0,0,100,100
枫原万叶,0,0,0,0,0,100,0,0,75,0
行秋,0,75,0,100,100,0,100,0,100,0
钟离,100,0,0,0,0,0,0,0,55,0
神里绫华,0,85,0,100,100,0,100,0,55,0
香菱,0,75,0,100,100,75,100,0,100,0
胡桃,80,50,0,100,100,75,100,0,0,0
温迪,0,100,0,100,100,0,100,0,45,0
珊瑚宫心海,100,0,0,0,0,75,100,0,55,100
莫娜,0,75,0,100,100,75,100,0,75,0
阿贝多,0,0,75,100,100,0,100,0,0,0
迪奥娜,100,0,0,0,0,0,0,0,100,100
优菈,0,75,0,100,100,0,40,100,55,0
达达利亚,0,75,0,100,100,75,100,0,30,0
魈,0,75,0,100,100,0,100,0,55,0
宵宫,0,75,0,100,100,75,100,0,0,0
九条裟罗,0,75,0,100,100,0,100,0,100,0
琴,0,100,0,0,0,0,0,0,100,100
菲谢尔,0,75,0,100,100,75,100,0,50,0
罗莎莉亚,0,75,0,100,0,0,0,0,100,0
可莉,0,75,0,100,100,75,100,0,30,0
凝光,0,75,0,100,100,0,100,0,30,0
北斗,75,0,0,0,0,0,0,0,100,0
刻晴,0,75,0,100,100,0,100,0,0,0
托马,100,0,0,0,0,0,0,0,100,0
迪卢克,0,75,0,100,100,75,100,0,0,0
诺艾尔,0,0,100,0,0,0,0,0,0,99
旅行者,0,75,0,100,100,0,100,0,55,0
重云,0,75,0,100,100,75,100,0,55,0
七七,0,100,0,0,0,0,0,0,75,100
凯亚,0,75,0,0,0,0,0,0,100,0
烟绯,0,75,0,100,100,75,100,0,30,0
早柚,0,100,0,0,0,0,0,0,100,100
安柏,0,75,0,100,100,75,100,0,0,0
丽莎,0,75,0,0,0,0,0,0,100,0
埃洛伊,0,75,0,100,100,0,100,0,0,0
辛焱,0,75,75,100,100,0,100,100,0,0
砂糖,0,0,0,0,0,100,0,0,100,0
雷泽,0,75,0,100,100,0,100,100,0,0
夜兰,80,0,0,100,100,0,100,0,100,0
久岐忍,100,0,0,0,0,100,0,0,55,100
鹿野院平藏,0,75,0,100,100,30,100,0,30,0
提纳里,0,75,0,100,100,90,100,0,30,0
柯莱,0,0,0,0,0,0,0,0,100,0
赛诺,0,75,0,100,100,75,100,0,55,0
坎蒂丝,100,0,0,0,0,0,0,0,100,0
妮露,100,0,0,0,0,80,0,0,30,0
纳西妲,0,55,0,100,100,100,100,0,55,0
多莉,100,0,0,0,0,0,0,0,100,100
莱依拉,100,0,0,0,0,0,0,0,75,0
流浪者,0,80,0,100,100,0,100,0,35,0
珐露珊,0,75,0,100,100,0,100,0,100,0
瑶瑶,100,0,0,0,0,75,0,0,100,100
艾尔海森,0,55,0,100,100,100,100,0,35,0
迪希雅,75,75,0,100,100,100,100,0,55,0
米卡,100,0,0,0,0,0,0,0,100,100
白术,100,0,0,0,0,50,0,0,100,100
卡维,0,75,0,100,100,75,100,0,75,0
绮良良,100,0,0,0,0,0,0,0,75,0
林尼,0,75,0,100,100,0,100,0,30,0
琳妮特,0,75,0,0,0,0,0,0,100,0
菲米尼,0,75,0,100,100,0,100,100,55,0
那维莱特,100,0,0,100,100,0,100,0,55,0
莱欧斯利,0,75,0,100,100,75,100,0,45,0
芙宁娜,100,0,0,100,100,0,95,0,100,95
夏洛蒂,0,100,0,0,0,0,0,0,100,100
娜维娅,0,75,0,100,100,0,100,0,55,0
夏沃蕾,100,0,0,0,0,0,0,0,55,55
闲云,0,100,0,0,0,0,0,0,100,75
嘉明,0,75,0,100,100,75,100,0,55,0
千织,0,50,75,100,100,0,100,0,55,0
阿蕾奇诺,0,75,0,100,100,75,100,0,30,0
赛索斯,0,30,0,100,100,100,100,0,55,0
克洛琳德,0,100,0,100,100,30,100,0,35,0
希格雯,100,0,0,100,100,0,95,0,30,100
艾梅莉埃,0,100,0,100,100,30,100,0,55,0
卡齐娜,0,0,75,100,100,0,100,0,75,0
玛拉妮,100,0,0,100,100,100,100,0,45,0
基尼奇,0,85,0,100,100,0,100,0,50,0
希诺宁,0,0,100,0,0,0,0,0,100,100
恰斯卡,0,85,0,100,100,30,85,0,40,0
欧洛伦,0,75,0,100,100,50,100,0,75,0
玛薇卡,0,75,0,100,100,85,100,0,0,0
茜特菈莉,0,50,0,50,50,100,80,0,100,0
蓝砚,0,100,0,0,0,30,0,0,75,0
梦见月瑞希,0,0,0,0,0,100,0,0,75,95
伊安珊,0,100,0,0,0,0,0,0,100,0
瓦雷莎,0,90,0,100,100,0,100,0,40,0
爱可菲,0,100,0,100,100,0,99,0,75,95
伊法,0,75,0,0,0,100,0,0,35,100
丝柯克,0,100,0,100,100,0,100,0,0,0
塔利雅,100,0,0,0,0,0,0,0,100,0
伊涅芙,0,75,0,100,100,75,0,0,40,0
菈乌玛,0,0,0,0,0,100,0,0,100,0
爱诺,0,0,0,0,0,100,0,0,100,0
菲林斯,0,100,0,100,100,50,0,0,50,0
奈芙尔,0,0,0,100,100,100,0,0,20,0
杜林,0,75,0,100,100,75,100,0,20,0
雅珂达,0,100,0,0,0,0,0,0,100,100
哥伦比娅,100,0,0,100,100,75,0,0,100,0
兹白,0,0,100,100,100,60,0,0,40,0
叶洛亚,0,0,0,0,0,100,0,0,100,0
法尔伽,0,100,0,100,100,0,99,0,20,0
莉奈娅,0,0,100,100,100,50,0,0,50,99
尼可,0,100,0,0,0,0,0,0,50,0
布伦妮,0,100,0,0,0,0,0,0,100,0
洛恩,0,100,0,100,100,0,100,0,30,0
桑多涅,0,85,0,100,100,60,0,0,50,0
奥黛塔,0,100,0,100,100,75,0,0,50,0
阿罗夏,0,100,0,0,0,0,0,0,100,99
沃雅妮莎,100,0,0,0,0,0,0,0,0,75
薇斯纳,0,100,0,100,100,100,0,0,50,0
'@

# ── 2. 角色级覆盖规则（源自 character/<角色>/artis.js）──
#    丝柯克：无 artis.js（404）⇒ 直接用默认表，不生成规则
$rulesCsv = @'
菲林斯,weapon,=,血染荒城,recharge,-5,专武
哥伦比娅,weapon,=,帷间夜曲,recharge,-15,专武
哥伦比娅,cons,>=,4,recharge,-15,高命
'@

# ── 3. ID→名字（真源：stat_weights_source.json 的 notes，与库里 characters 一致，共 124 条）──
$srcPath = Join-Path $seedDir 'stat_weights_source.json'
$src = [System.IO.File]::ReadAllText($srcPath, [System.Text.Encoding]::UTF8) | ConvertFrom-Json
$pairs = @()
foreach ($p in $src.notes.PSObject.Properties) { $pairs += [pscustomobject]@{ id = [int64]$p.Name; name = [string]$p.Value } }
Write-Host ("  ID→名字 共 {0} 条（源: stat_weights_source.json notes）" -f $pairs.Count)

$byName = @{}
foreach ($p in $pairs) { if (-not $byName.ContainsKey($p.name)) { $byName[$p.name] = @() }; $byName[$p.name] += $p.id }

# 旅行者：miao 的权重表只有 1 条 `旅行者`，但我们库里是 4 个 ID（含 奇偶·男性 / 奇偶·女性）
# ⇒ 同一个角色，统一套用同一份权重（2026-09-27 决策）
if ($byName.ContainsKey('旅行者')) {
  foreach ($tid in @(10000005, 10000007, 10000117, 10000118)) {
    if ($byName['旅行者'] -notcontains $tid) { $byName['旅行者'] += $tid }
  }
}

# ── 4. 组装权重记录 ──
$records = @()
$unmatched = @()
foreach ($line in ($miaoCsv -split "`r?`n")) {
  $t = $line.Trim()
  if (-not $t) { continue }
  $f = $t -split ','
  $name = $f[0]
  if (-not $byName.ContainsKey($name)) { $unmatched += $name; continue }
  foreach ($id in $byName[$name]) {
    $records += [ordered]@{
      characterId = $id
      name        = $name
      hp = [int]$f[1]; atk = [int]$f[2]; def = [int]$f[3]
      cpct = [int]$f[4]; cdmg = [int]$f[5]; mastery = [int]$f[6]
      dmg = [int]$f[7]; phy = [int]$f[8]; recharge = [int]$f[9]; heal = [int]$f[10]
      source = 'miao-plugin'; sourceRef = $SOURCE_REF; fetchedAt = $FETCHED_AT
    }
  }
}

$rules = @()
foreach ($line in ($rulesCsv -split "`r?`n")) {
  $t = $line.Trim()
  if (-not $t) { continue }
  $f = $t -split ','
  if (-not $byName.ContainsKey($f[0])) { $unmatched += $f[0]; continue }
  foreach ($id in $byName[$f[0]]) {
    $rules += [ordered]@{
      characterId = $id; characterName = $f[0]
      condType = $f[1]; condOp = $f[2]; condValue = $f[3]
      statKey = $f[4]; delta = [int]$f[5]; note = $f[6]
      source = 'miao-plugin'; sourceRef = $SOURCE_REF; fetchedAt = $FETCHED_AT
    }
  }
}

$payload = [ordered]@{
  _readme   = '圣遗物评分权重种子（生成器输入，非运行时真源）。运行时真源是数据库 character_relic_weights / character_relic_weight_rules；改数据请走后台或 POST /api/admin/assets/import，不要把数据写回本文件。'
  _license  = 'MIT License, Copyright (c) 2023 Yoimiya — 数据来源 yoimiya-kokomi/miao-plugin（resources/meta-gs/artifact/artis-mark.js 与 resources/meta-gs/character/<角色>/artis.js）'
  _source   = $SOURCE_REF
  _fetchedAt = $FETCHED_AT
  # 键名必须与 asset.ImportPayload 的 json tag 一致，否则管理接口不认（2026-09-27 修）
  relicWeights     = $records
  relicWeightRules = $rules
}

$utf8 = New-Object System.Text.UTF8Encoding($false)
[System.IO.File]::WriteAllText((Join-Path $seedDir 'relic_weights_miao.json'), ($payload | ConvertTo-Json -Depth 5), $utf8)

# ── 5. 自检 ──
Write-Host ("  权重记录 {0} 条（miao 名单 121 条 × 命中 ID）" -f $records.Count)
Write-Host ("  覆盖规则 {0} 条" -f $rules.Count)
if ($unmatched.Count -gt 0) { Write-Host ("  ❌ 未匹配到的名字: " + ($unmatched -join '、')) } else { Write-Host '  ✅ 全部名字都匹配到 ID' }
$nullW = @($records | Where-Object { $_.hp -eq 0 -and $_.atk -eq 0 -and $_.def -eq 0 -and $_.cpct -eq 0 -and $_.cdmg -eq 0 -and $_.mastery -eq 0 -and $_.dmg -eq 0 -and $_.phy -eq 0 -and $_.recharge -eq 0 -and $_.heal -eq 0 })
if ($nullW.Count -gt 0) { Write-Host ("  ⚠️ 全 0 权重记录: " + (($nullW | ForEach-Object { $_.name }) -join '、')) } else { Write-Host '  ✅ 无全 0 记录' }
$covered = @($records | ForEach-Object { $_.characterId } | Sort-Object -Unique)
Write-Host ("  覆盖角色 ID {0} 个 / 库中 {1} 个" -f $covered.Count, $pairs.Count)
$miss = @($pairs | Where-Object { $covered -notcontains $_.id } | ForEach-Object { $_.name + '(' + $_.id + ')' })
Write-Host ("  未覆盖: " + $(if ($miss.Count -gt 0) { $miss -join '、' } else { '无 ✅' }))
