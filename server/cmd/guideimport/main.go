// guideimport：把 `scripts/data/character_guides.json`（阶段② 攻略卡，设计见 teyvat-docs/ai/07 TODO-18）
// 导入 `character_guides` 表。
//
// 用法（在 server/ 目录下）：
//
//	go run ./cmd/guideimport                                  # 默认读 ../scripts/data/character_guides.json
//	go run ./cmd/guideimport -file <路径> -only 芙宁娜,班尼特 -dry-run
//
// 口径（与 cmd/assetimport 一致，别改 ✗）：
//   - **只补 / 只改，不删** ✓ —— JSON 里没有的角色，库里既有的行**原样保留**（不做清理 ✓）
//   - **幂等**：`content_hash` 未变则跳过（不打无谓的 UPDATE ✓，也便于日志看出"这次到底改了什么"）
//   - **主 / 副词条不在本表** ✗ —— 以 `asset.CharacterStatWeights` 为准（TODO-18 硬约束一 ✓）
//   - 表由本命令自己 AutoMigrate（CLI 独立运行要能建表 ✓，后端启动时 router 也会建 ✓）
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"teyvat-guide/server/internal/config"
	"teyvat-guide/server/internal/model"
	"teyvat-guide/server/internal/pkg/database"
	"teyvat-guide/server/internal/service"
	"teyvat-guide/server/internal/service/asset"
)

// ---------------------------------------------------------------- JSON 形状（与 scripts/extract-character-guides.mjs 对齐）

type guideFile struct {
	ExtractedAt string      `json:"extractedAt"`
	Source      string      `json:"source"`
	Cards       []guideCard `json:"cards"`
}

type namedWhy struct {
	Name string `json:"name"`
	Why  string `json:"why"`
}

// flexString 兼容「字符串或数字」的字段 —— 攻略卡里 `sourceRev` 来自 manifest 的 revid
// （是**数字** ✓），而 `fetchedAt` 是字符串 ✓ ⇒ 不一律按字符串收 ✗，否则整份 JSON 解析失败 ✓
type flexString string

func (f *flexString) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(b))
	if s == "null" {
		*f = ""
		return nil
	}
	*f = flexString(strings.Trim(s, `"`))
	return nil
}

type relicPick struct {
	Sets    []namedWhy `json:"sets"`
	Reasons string     `json:"reasons"`
}

type weaponPick struct {
	Graduation    string     `json:"graduation"`
	GraduationWhy string     `json:"graduationWhy"`
	Options       []namedWhy `json:"options"`
}

type guideCard struct {
	Name       string     `json:"name"`
	SourceURL  string     `json:"sourceUrl"`
	SourceRev  flexString `json:"sourceRev"`
	FetchedAt  string     `json:"fetchedAt"`
	Role       string     `json:"role"`
	Talents    string     `json:"talents"`
	RelicPick  relicPick  `json:"relicPick"`
	WeaponPick weaponPick `json:"weaponPick"`
	Team       string     `json:"team"`
	TeamFrom   string     `json:"teamFrom"`
	Graduation string     `json:"graduation"`
	Notes      string     `json:"notes"`
}

// ---------------------------------------------------------------- 渲染（结构化槽位 → 注入用的整段文本）

// renderNamedWhy 把 [{name, why}] 拼成「A（理由）、B（理由）」——
// 理由为空时只留名字 ✓（有些条目确实没写 why ✓，别硬凑 ✗）
func renderNamedWhy(items []namedWhy, sep string) string {
	parts := make([]string, 0, len(items))
	for _, it := range items {
		name := strings.TrimSpace(it.Name)
		if name == "" {
			continue
		}
		if why := strings.TrimSpace(it.Why); why != "" {
			parts = append(parts, name+"（"+why+"）")
		} else {
			parts = append(parts, name)
		}
	}
	return strings.Join(parts, sep)
}

func renderWeapon(w weaponPick) string {
	parts := make([]string, 0, 2)
	if g := strings.TrimSpace(w.Graduation); g != "" {
		s := "毕业：" + g
		if why := strings.TrimSpace(w.GraduationWhy); why != "" {
			s += "（" + why + "）"
		}
		parts = append(parts, s)
	}
	if opts := renderNamedWhy(w.Options, "、"); opts != "" {
		parts = append(parts, "可选："+opts)
	}
	return strings.Join(parts, "；")
}

func renderRelic(r relicPick) string {
	parts := make([]string, 0, 2)
	if sets := renderNamedWhy(r.Sets, "、"); sets != "" {
		parts = append(parts, sets)
	}
	// ⚠️ 这里**不写主/副词条** ✗：那部分以结构化词条表为准（TODO-18 硬约束一 ✓）
	//   只留"为什么这么堆"的思路 ✓
	if reasons := strings.TrimSpace(r.Reasons); reasons != "" {
		parts = append(parts, "词条思路："+reasons)
	}
	return strings.Join(parts, "；")
}

func parseTime(s string) time.Time {
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05Z", "2006-01-02"} {
		if t, err := time.Parse(layout, strings.TrimSpace(s)); err == nil {
			return t
		}
	}
	return time.Time{}
}

// contentHash 用"要写进库的内容"算哈希 ⇒ 脚本改了措辞才会重写 ✓，纯重跑不会动库 ✓
func contentHash(row model.CharacterGuide) string {
	payload := strings.Join([]string{
		row.Name, row.Role, row.Talents, row.WeaponPick, row.RelicPick,
		row.TeamPick, row.TeamFrom, row.Graduation, row.Notes,
		row.SourceURL, row.SourceRev, row.FetchedAt.UTC().Format(time.RFC3339),
	}, "\x1f")
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}

func main() {
	file := flag.String("file", filepath.Join("..", "scripts", "data", "character_guides.json"), "攻略卡 JSON 路径")
	only := flag.String("only", "", "只导入这些角色（逗号分隔；默认全部）")
	dryRun := flag.Bool("dry-run", false, "只打印将写什么，不写库")
	probe := flag.String("probe", "", "排障：拿一个问题跑一次「命中 → 查卡 → 渲染」，打印真正会注入给模型的文本")
	flag.Parse()

	raw, err := os.ReadFile(*file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "读不到攻略卡 JSON：%s（先用 node scripts/extract-character-guides.mjs 生成）\n", *file)
		os.Exit(1)
	}
	var gf guideFile
	if err := json.Unmarshal(raw, &gf); err != nil {
		fmt.Fprintf(os.Stderr, "解析失败：%v\n", err)
		os.Exit(1)
	}
	fmt.Printf("  攻略卡：%d 张（源：%s，生成于 %s）\n", len(gf.Cards), gf.Source, gf.ExtractedAt)

	cfg := config.Load()
	if err := database.Init(cfg.Database); err != nil {
		fmt.Fprintf(os.Stderr, "数据库初始化失败：%v\n", err)
		os.Exit(1)
	}
	db := database.DB()
	if db == nil {
		fmt.Fprintln(os.Stderr, "数据库未启用")
		os.Exit(1)
	}
	if err := db.AutoMigrate(&model.CharacterGuide{}); err != nil {
		fmt.Fprintf(os.Stderr, "建表失败：%v\n", err)
		os.Exit(1)
	}

	if strings.TrimSpace(*probe) != "" {
		runProbe(db, *probe)
		return
	}

	// 名字 → characters.id（角色 id 是 Enka avatarId ✓，攻略 JSON 里只存名字 ✓）
	names := make([]string, 0, len(gf.Cards))
	for _, c := range gf.Cards {
		names = append(names, c.Name)
	}
	var chars []asset.Character
	if err := db.Where("name IN ?", names).Find(&chars).Error; err != nil {
		fmt.Fprintf(os.Stderr, "查 characters 失败：%v\n", err)
		os.Exit(1)
	}
	idByName := make(map[string]int64, len(chars))
	for _, c := range chars {
		idByName[c.Name] = c.ID
	}

	onlySet := map[string]bool{}
	if strings.TrimSpace(*only) != "" {
		for _, n := range strings.Split(*only, ",") {
			onlySet[strings.TrimSpace(n)] = true
		}
	}

	updateCols := []string{
		"name", "role", "talents", "weapon_pick", "relic_pick",
		"team_pick", "team_from", "graduation", "notes",
		"source_url", "source_rev", "fetched_at", "content_hash", "updated_at",
	}

	var written, unchanged, skipped int
	for _, c := range gf.Cards {
		if len(onlySet) > 0 && !onlySet[c.Name] {
			continue
		}
		id, ok := idByName[c.Name]
		if !ok {
			fmt.Printf("  [跳过] %-8s 不在 characters 表里（先补资产映射 ✓）\n", c.Name)
			skipped++
			continue
		}

		row := model.CharacterGuide{
			CharacterID: id,
			Name:        c.Name,
			Role:        c.Role,
			Talents:     c.Talents,
			WeaponPick:  renderWeapon(c.WeaponPick),
			RelicPick:   renderRelic(c.RelicPick),
			TeamPick:    c.Team,
			TeamFrom:    c.TeamFrom,
			Graduation:  c.Graduation,
			Notes:       c.Notes,
			SourceURL:   c.SourceURL,
			SourceRev:   string(c.SourceRev),
			FetchedAt:   parseTime(c.FetchedAt),
		}
		row.ContentHash = contentHash(row)

		// ⚠️ 用 Find（不是 First）：First 查不到会打一条 `record not found` 的红色错误日志 ✗，
		//   而"这位还没卡"是完全正常的情况 ✓ ⇒ Find + RowsAffected 判断，输出干净 ✓
		var existing model.CharacterGuide
		found := db.Where("character_id = ?", id).Limit(1).Find(&existing).RowsAffected > 0
		if found && existing.ContentHash == row.ContentHash {
			fmt.Printf("  [未变] %-8s id=%-9d\n", c.Name, id)
			unchanged++
			continue
		}

		action := "新增"
		if found {
			action = "更新"
		}
		fmt.Printf("  [%s] %-8s id=%-9d 定位%4d 天赋%4d 武器%4d 圣遗物%4d 配队%4d 毕业%4d 注意%4d（配队来源=%s）\n",
			action, c.Name, id,
			len([]rune(row.Role)), len([]rune(row.Talents)), len([]rune(row.WeaponPick)),
			len([]rune(row.RelicPick)), len([]rune(row.TeamPick)), len([]rune(row.Graduation)),
			len([]rune(row.Notes)), orDash(row.TeamFrom),
		)

		if *dryRun {
			continue
		}
		// 冲突只认 character_id（唯一索引 ✓）⇒ 既有行进 UPDATE，PK 不动 ✓
		if err := db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "character_id"}},
			DoUpdates: clause.AssignmentColumns(updateCols),
		}).Create(&row).Error; err != nil {
			fmt.Fprintf(os.Stderr, "写入 %s 失败：%v\n", c.Name, err)
			os.Exit(1)
		}
		written++
	}

	fmt.Printf("  完成：写入 %d 行、未变 %d 行、跳过 %d 行（dry-run=%v）\n", written, unchanged, skipped, *dryRun)
}

// runProbe 打印「这个问题真正会注入给模型的攻略卡文本」—— 排障用 ✓
// 走的是与线上**完全相同**的代码路径：asset 名字命中（长名优先）→ 直查库 → renderGuide ✓
// ⇒ 不用登录、不用起服务，就能看到上下文长什么样 ✓（也顺带验证了注入体积 ✓）
func runProbe(db *gorm.DB, question string) {
	svc := asset.New(db)
	if err := svc.Load(); err != nil {
		fmt.Fprintf(os.Stderr, "加载资产映射失败：%v\n", err)
		os.Exit(1)
	}
	text := service.NewGuideStore(db).ForQuestion(svc, question)
	if text == "" {
		fmt.Printf("  问题「%s」⇒ 没命中任何攻略卡（本次不注入 ✓ 走原逻辑）\n", question)
		return
	}
	fmt.Printf("  问题「%s」⇒ 注入 %d 字（≈ %d token 粗估 ✓）：\n\n%s\n", question, len([]rune(text)), len([]rune(text)), text)
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}
