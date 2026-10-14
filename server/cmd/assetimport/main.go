// assetimport：一次性导入工具
// 把 EnkaGameAssets 的 store/gi 数据（avatars/weapons/relics/locs）转换为项目映射表并入库
//
// 用法：
//   go run ./cmd/assetimport -dir "…/store/gi"
//
// 数据源结构（v1）：
//   avatars.json { "<avatarId>": { Element, NameTextMapHash, SideIconName, QualityType, ... } }
//   weapons.json { "<weaponId>": { Rarity, WeaponType, Icon, NameTextMapHash, ... } }
//   relics.json  { "Items": {...}, "Sets": { "<setId>": { Name: <hash> } } }
//   locs.json    { "zh-cn": { "<hash>": "中文文本", ... } }

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"teyvat-guide/server/internal/config"
	"teyvat-guide/server/internal/pkg/database"
	"teyvat-guide/server/internal/service/asset"

	"gorm.io/gorm/clause"
)

type avatarEntry struct {
	Element         string `json:"Element"`
	NameTextMapHash int64  `json:"NameTextMapHash"`
	SideIconName    string `json:"SideIconName"`
	QualityType     string `json:"QualityType"`
}

type weaponEntry struct {
	Rarity          int    `json:"Rarity"`
	WeaponType      int    `json:"WeaponType"`
	Icon            string `json:"Icon"`
	NameTextMapHash int64  `json:"NameTextMapHash"`
}

type relicsFile struct {
	Sets map[string]struct {
		Name string `json:"Name"`
	} `json:"Sets"`
}

type locsFile map[string]map[string]string

// Enka Element 枚举 → 中文
var elementMap = map[string]string{
	"Fire":     "火",
	"Water":    "水",
	"Wind":     "风",
	"Ice":      "冰",
	"Electric": "雷",
	"Rock":     "岩",
	"Grass":    "草",
}

// QualityType → 星级
// 未命中会落到默认值 0（= 无星级），所以新出现的 QualityType 必须及时补：
// QUALITY_ORANGE_SP 是联动/特殊形态用的 5★（埃洛伊、奇偶形态等），2026-09-26 补 ——
// 此前它落到 0，导出了 3 行 rarity=0 的脏数据（见 ai/07 TODO-05 §3）。
var qualityMap = map[string]int{
	"QUALITY_ORANGE":    5,
	"QUALITY_ORANGE_MC": 5,
	"QUALITY_ORANGE_SP": 5,
	"QUALITY_PURPLE":    4,
}

// 武器类型数字 → 中文（原神 WeaponType 约定）
var weaponTypeMap = map[int]string{
	1: "单手剑",
	2: "法器",
	3: "双手剑",
	4: "弓",
	5: "长柄武器",
}

func main() {
	dir := flag.String("dir", "", "EnkaGameAssets store/gi 目录路径")
	textmapFile := flag.String("textmap-file", "", "仅导入全量文本映射 JSON（{\"<hash>\": \"文本\"}）")
	affixesFile := flag.String("affixes-file", "", "仅导入词缀表（ReliquaryAffixExcelConfigData.json）")
	statWeightsFile := flag.String("stat-weights-file", "", "仅导入角色推荐词条（{\"statWeights\":[...]}）")
	fightPropsFile := flag.String("fight-props-file", "", "仅导入数字枚举映射（{\"fightProps\":[{\"numId\",\"propId\"}]}）")
	// ⚠️ 角色别名（2026-10-14 加 ✓）：玩家只说「雷神」「帝君」「万叶」✗ ⇒ 精确匹配命不中 ✓
	//   真源 scripts/data/character_aliases.json ✓；导入后 `svc.Reload()` ⇒ 立即生效 ✓（无需重启 ✓）
	aliasesFile := flag.String("aliases-file", "", "仅导入角色别名（{\"aliases\":[{\"alias\",\"name\",\"source\"}]}）")
	importFile := flag.String("import-file", "", "通用导入：任意 ImportPayload 子集（如 relicSlots / propMap / fightProps）")
	dumpJSON := flag.String("dump-json", "", "只生成导入 payload 到指定文件、**不连数据库**（供 POST /api/admin/assets/import 远程导入；目前需与 -dir 同用）")
	flag.Parse()

	// 只生成 payload：本地连不上云上 RDS 时，用它产出 JSON，再走内网管理接口导入
	if *dumpJSON != "" {
		if *dir == "" {
			fmt.Fprintln(os.Stderr, "-dump-json 目前只支持与 -dir 一起使用")
			os.Exit(1)
		}
		writeJSON(*dumpJSON, buildDirPayload(*dir))
		fmt.Println("已生成 payload：" + *dumpJSON)
		return
	}

	cfg := config.Load()
	if err := database.Init(cfg.Database); err != nil {
		fmt.Fprintf(os.Stderr, "数据库初始化失败: %v\n", err)
		os.Exit(1)
	}
	db := database.DB()
	if db == nil {
		fmt.Fprintln(os.Stderr, "数据库未启用")
		os.Exit(1)
	}
	svc := asset.New(db)

	// CLI 独立运行，先确保映射表存在（后端启动时也会 autoMigrate）
	if err := db.AutoMigrate(
		&asset.Character{}, &asset.Weapon{}, &asset.RelicSet{}, &asset.RelicSetEffect{},
		&asset.TextMapEntry{}, &asset.PropMap{}, &asset.FightProp{}, &asset.RelicSlot{}, &asset.Affix{}, &asset.Missing{},
		&asset.CharacterStatWeights{}, &asset.CharacterRelicWeight{}, &asset.CharacterRelicWeightRule{},
		&asset.CharacterRelicWeightMP{},
		&asset.CharacterAlias{}, // 角色别名（「雷神」→「雷电将军」✓，2026-10-14 ✓）
	); err != nil {
		fmt.Fprintf(os.Stderr, "建表失败: %v\n", err)
		os.Exit(1)
	}

	// 模式七：仅导入角色别名（「雷神」→「雷电将军」✓，2026-10-14 加 ✓）
	//
	// 口径 ✓：别名**全表唯一** ✓（一个叫法只指一个角色 ✓）；`name` 必须与 `characters` 表里的名字
	//   **完全一致** ✓ —— 对不上就跳过并打印 ✗（宁可少一条，也不要指错角色 ✓）。
	if *aliasesFile != "" {
		var payload struct {
			Aliases []struct {
				Alias  string `json:"alias"`
				Name   string `json:"name"`
				Source string `json:"source"`
			} `json:"aliases"`
		}
		readJSON(*aliasesFile, &payload)
		var chars []asset.Character
		if err := db.Find(&chars).Error; err != nil {
			fmt.Fprintf(os.Stderr, "读 characters 失败: %v\n", err)
			os.Exit(1)
		}
		idByName := make(map[string]int64, len(chars))
		for _, c := range chars {
			idByName[c.Name] = c.ID
		}
		written, skipped := 0, 0
		for _, a := range payload.Aliases {
			alias := strings.TrimSpace(a.Alias)
			name := strings.TrimSpace(a.Name)
			id, ok := idByName[name]
			if alias == "" || !ok {
				fmt.Printf("  [跳过] %s → %s（characters 表里没有这个名字 ✗）\n", alias, name)
				skipped++
				continue
			}
			row := asset.CharacterAlias{Alias: alias, CharacterID: id, Name: name, Source: strings.TrimSpace(a.Source)}
			if err := db.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "alias"}},
				DoUpdates: clause.AssignmentColumns([]string{"character_id", "name", "source", "updated_at"}),
			}).Create(&row).Error; err != nil {
				fmt.Fprintf(os.Stderr, "写入别名 %s 失败: %v\n", alias, err)
				os.Exit(1)
			}
			written++
		}
		fmt.Printf("aliases: %d 条 → 写入 %d、跳过 %d ✓\n", len(payload.Aliases), written, skipped)
		if err := svc.Reload(); err != nil {
			fmt.Fprintf(os.Stderr, "重载失败: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("导入完成，内存映射已刷新")
		return
	}

	// 模式一：仅导入缺失清单中登记的文本 hash（不全量入库）
	if *textmapFile != "" {
		var raw map[string]string
		readJSON(*textmapFile, &raw)
		fmt.Printf("TextMapCHS 总条目: %d\n", len(raw))

		// 读取 missings 表中 kind=text 的缺失 hash
		wanted := map[string]bool{}
		if list, err := svc.MissingList(1000); err == nil {
			for _, m := range list {
				if m.Kind == asset.KindText {
					wanted[m.AssetID] = true
				}
			}
		}
		fmt.Printf("缺失清单中待补的 text hash: %d\n", len(wanted))

		texts := make([]asset.TextMapEntry, 0, len(wanted))
		for hashStr, text := range raw {
			if !wanted[hashStr] {
				continue
			}
			var hash int64
			if _, err := fmt.Sscanf(hashStr, "%d", &hash); err != nil {
				continue
			}
			texts = append(texts, asset.TextMapEntry{Hash: hash, Text: text})
		}
		fmt.Printf("命中可导入: %d 条\n", len(texts))
		if len(texts) == 0 {
			fmt.Println("没有可导入的条目")
			return
		}
		if err := svc.Import(asset.ImportPayload{TextMap: texts}); err != nil {
			fmt.Fprintf(os.Stderr, "导入失败: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("导入完成，内存映射已刷新")
		return
	}

	// 模式四：仅导入角色推荐词条
	if *statWeightsFile != "" {
		var payload asset.ImportPayload
		readJSON(*statWeightsFile, &payload)
		fmt.Printf("statWeights: %d 个角色\n", len(payload.StatWeights))
		if err := svc.Import(payload); err != nil {
			fmt.Fprintf(os.Stderr, "导入失败: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("导入完成，内存映射已刷新")
		return
	}

	// 模式五：仅导入数字枚举映射（fightPropMap 数字键 → FIGHT_PROP_XXX）
	if *fightPropsFile != "" {
		var payload asset.ImportPayload
		readJSON(*fightPropsFile, &payload)
		fmt.Printf("fightProps: %d 条\n", len(payload.FightProps))
		if err := svc.Import(payload); err != nil {
			fmt.Fprintf(os.Stderr, "导入失败: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("导入完成，内存映射已刷新")
		return
	}

	// 模式六：通用导入（任意 ImportPayload 子集，如 relicSlots）
	if *importFile != "" {
		var payload asset.ImportPayload
		readJSON(*importFile, &payload)
		fmt.Printf("relicSlots: %d 条, propMap: %d 条, fightProps: %d 条, relicSetEffects: %d 条\n",
			len(payload.RelicSlots), len(payload.PropMap), len(payload.FightProps), len(payload.RelicSetEffects))
		if err := svc.Import(payload); err != nil {
			fmt.Fprintf(os.Stderr, "导入失败: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("导入完成，内存映射已刷新")
		return
	}

	// 模式三：仅导入词缀表
	if *affixesFile != "" {
		var arr []struct {
			ID        int64   `json:"id"`
			PropType  string  `json:"propType"`
			PropValue float64 `json:"propValue"`
		}
		readJSON(*affixesFile, &arr)
		affixes := make([]asset.Affix, 0, len(arr))
		for _, a := range arr {
			affixes = append(affixes, asset.Affix{ID: a.ID, PropType: a.PropType, PropValue: a.PropValue})
		}
		fmt.Printf("affixes: %d 条\n", len(affixes))
		if err := svc.Import(asset.ImportPayload{Affixes: affixes}); err != nil {
			fmt.Fprintf(os.Stderr, "导入失败: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("导入完成，内存映射已刷新")
		return
	}

	// 模式二：EnkaGameAssets store/gi 全量导入
	if *dir == "" {
		fmt.Fprintln(os.Stderr, "用法: assetimport -dir <store/gi 目录> / -textmap-file <TextMapCHS.json> / -affixes-file <ReliquaryAffixExcelConfigData.json>")
		os.Exit(1)
	}

	// 1. 多语言文本
	payload := buildDirPayload(*dir)

	// 6. 入库
	if err := svc.Import(payload); err != nil {
		fmt.Fprintf(os.Stderr, "导入失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("导入完成，内存映射已刷新")
}

// buildDirPayload 解析 EnkaGameAssets 的 store/gi 目录，组装导入载荷（**不碰数据库**）：
// 拆出来是为了支持 -dump-json —— 本地连不上云上 RDS 时，先产出 JSON 再走内网管理接口导入。
func buildDirPayload(dir string) asset.ImportPayload {
	var locs locsFile
	readJSON(filepath.Join(dir, "locs.json"), &locs)
	locale, ok := locs["zh-cn"]
	if !ok {
		fmt.Fprintln(os.Stderr, "locs.json 中没有 zh-cn 语言包")
		os.Exit(1)
	}
	fmt.Printf("locs zh-cn: %d 条\n", len(locale))

	var avatars map[string]avatarEntry
	readJSON(filepath.Join(dir, "avatars.json"), &avatars)
	characters := make([]asset.Character, 0, len(avatars))
	for idStr, a := range avatars {
		var id int64
		if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil {
			continue
		}
		name, _ := locale[fmt.Sprint(a.NameTextMapHash)]
		characters = append(characters, asset.Character{
			ID:      id,
			Name:    name,
			EnName:  enNameFromSideIcon(a.SideIconName),
			Element: elementMap[a.Element],
			Rarity:  qualityMap[a.QualityType],
			Icon:    trimUIPath(a.SideIconName),
		})
	}
	fmt.Printf("characters: %d 个\n", len(characters))

	var weapons map[string]weaponEntry
	readJSON(filepath.Join(dir, "weapons.json"), &weapons)
	weaponList := make([]asset.Weapon, 0, len(weapons))
	for idStr, w := range weapons {
		var id int64
		if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil {
			continue
		}
		name, _ := locale[fmt.Sprint(w.NameTextMapHash)]
		weaponList = append(weaponList, asset.Weapon{
			ID:     id,
			Name:   name,
			EnName: enNameFromIcon(w.Icon),
			Rarity: w.Rarity,
			Type:   weaponTypeMap[w.WeaponType],
			Icon:   trimUIPath(w.Icon),
		})
	}
	fmt.Printf("weapons: %d 个\n", len(weaponList))

	var relics relicsFile
	readJSON(filepath.Join(dir, "relics.json"), &relics)
	sets := make([]asset.RelicSet, 0, len(relics.Sets))
	for idStr, s := range relics.Sets {
		var setID int
		if _, err := fmt.Sscanf(idStr, "%d", &setID); err != nil {
			continue
		}
		nameHash := strings.Trim(s.Name, "\"")
		name := locale[nameHash]
		if name == "" {
			continue // 套装名不在 locs 中的跳过
		}
		sets = append(sets, asset.RelicSet{SetID: setID, Name: name})
	}
	fmt.Printf("relicSets: %d 个\n", len(sets))

	texts := make([]asset.TextMapEntry, 0, len(locale))
	for hashStr, text := range locale {
		var hash int64
		if _, err := fmt.Sscanf(hashStr, "%d", &hash); err != nil {
			continue
		}
		texts = append(texts, asset.TextMapEntry{Hash: hash, Text: text})
	}
	fmt.Printf("textMap: %d 条\n", len(texts))

	return asset.ImportPayload{
		Characters: characters,
		Weapons:    weaponList,
		RelicSets:  sets,
		TextMap:    texts,
	}
}

// writeJSON 把任意载荷写成 UTF-8 JSON（不转义 HTML，便于直接 POST 给内网接口）
func writeJSON(path string, v any) {
	f, err := os.Create(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "创建 %s 失败: %v\n", path, err)
		os.Exit(1)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		fmt.Fprintf(os.Stderr, "写出 %s 失败: %v\n", path, err)
		os.Exit(1)
	}
}

// readJSON 读取并解析 JSON 文件（UTF-8）
func readJSON(path string, out any) {
	raw, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "读取 %s 失败: %v\n", path, err)
		os.Exit(1)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		fmt.Fprintf(os.Stderr, "解析 %s 失败: %v\n", path, err)
		os.Exit(1)
	}
}

// enNameFromSideIcon 从 "/ui/UI_AvatarIcon_Side_Ayaka.png" 提取 "Ayaka"
func enNameFromSideIcon(sideIcon string) string {
	base := strings.TrimPrefix(sideIcon, "/ui/UI_AvatarIcon_Side_")
	return strings.TrimSuffix(base, ".png")
}

// enNameFromIcon 从 "/ui/UI_EquipIcon_Catalyst_Narukami.png" 提取末段
func enNameFromIcon(icon string) string {
	base := strings.TrimPrefix(icon, "/ui/UI_EquipIcon_")
	base = strings.TrimSuffix(base, ".png")
	if idx := strings.LastIndex(base, "_"); idx >= 0 {
		base = base[idx+1:]
	}
	return base
}

// trimUIPath "/ui/xxx.png" → "xxx"
func trimUIPath(p string) string {
	base := strings.TrimPrefix(p, "/ui/")
	return strings.TrimSuffix(base, ".png")
}
