// mechanicsimport：把 `scripts/data/character_mechanics.json`（阶段② 角色机制卡 + 全局机制规则，
// 设计见 teyvat-docs/ai/07 TODO-20）导入 `character_mechanics` / `mechanic_rules` 两张表。
//
// 用法（在 server/ 目录下）：
//
//	go run ./cmd/mechanicsimport                              # 默认读 ../scripts/data/character_mechanics.json
//	go run ./cmd/mechanicsimport -dry-run -only 班尼特,希诺宁
//	go run ./cmd/mechanicsimport -probe "班尼特能进魈队吗"      # 打印**真链路**会注入给模型的全部文本
//	go run ./cmd/mechanicsimport -questions <评测题.json> -out <上下文.json>
//	                                                          # 批量导出注入文本（给评测器复刻线上口径 ✓）
//
// 口径（与 cmd/guideimport 完全一致，别改 ✗）：
//   - **只补 / 只改，不删** ✓ —— JSON 里没有的角色/规则，库里既有的行**原样保留** ✓
//   - **幂等**：`content_hash` 未变则跳过 ✓
//   - **不存结论** ✗ —— 「班尼特加攻击力」存 ✓；「班尼特适合魈队」不存 ✗（留给模型推 ✓）
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
)

// ---------------------------------------------------------------- JSON 形状（与 scripts/data/character_mechanics.json 对齐）

type mechanicsFile struct {
	Meta       map[string]any   `json:"_meta"`
	Characters []map[string]any `json:"characters"`
	Rules      []ruleEntry      `json:"rules"`
}

type ruleEntry struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Text      string   `json:"text"`
	Triggers  []string `json:"triggers"`
	Confirmed bool     `json:"confirmed"`
}

// ---------------------------------------------------------------- 内容哈希（幂等导入用）

// hashParts 用"要写进库的内容"算哈希 ⇒ 脚本改了措辞才会重写 ✓，纯重跑不会动库 ✓
func hashParts(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x1f")))
	return hex.EncodeToString(sum[:])
}

// questionFile 评测题文件形状（AI 侧 tests/eval/*.json：{"questions":[{id, q, ...}]} ✓）
type questionFile struct {
	Questions []struct {
		ID int    `json:"id"`
		Q  string `json:"q"`
	} `json:"questions"`
}

// contextOut 批量导出的注入文本（评测器读它 ✓ ⇒ 与线上**逐字相同** ✓）
type contextOut struct {
	GeneratedAt string       `json:"generatedAt"`
	Source      string       `json:"source"`
	Items       []contextRow `json:"items"`
}

type contextRow struct {
	ID      int    `json:"id"`
	Q       string `json:"q"`
	Summary string `json:"summary"`
}

func main() {
	file := flag.String("file", filepath.Join("..", "scripts", "data", "character_mechanics.json"), "机制卡 JSON 路径")
	// 自动抽取的初稿（阶段③ 铺全量 ✓，由 scripts/extract-character-mechanics-llm.mjs 生成 ✓）：
	// review=draft ✓；**绝不覆盖人工卡** ✗（见 importCards 的守卫 ✓）；文件不存在就跳过 ✓
	autoFile := flag.String("auto-file", filepath.Join("..", "scripts", "data", "character_mechanics_auto.json"),
		"自动抽取的机制卡初稿 JSON（可选 ✓）")
	noAuto := flag.Bool("no-auto", false, "不导入自动初稿（只导人工卡 ✓）")
	only := flag.String("only", "", "只导入这些角色（逗号分隔；默认全部）")
	dryRun := flag.Bool("dry-run", false, "只打印将写什么，不写库")
	probe := flag.String("probe", "", "排障：拿一个问题跑一次完整 summary 组装，打印真正会注入给模型的文本")
	questions := flag.String("questions", "", "批量模式：读这个评测题 JSON，导出每题会注入的 summary")
	out := flag.String("out", "", "批量模式的输出路径（默认 questions 同目录下的 mechanics_contexts.json ✓）")
	flag.Parse()

	raw, err := os.ReadFile(*file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "读不到机制卡 JSON：%s\n", *file)
		os.Exit(1)
	}
	var mf mechanicsFile
	if err := json.Unmarshal(raw, &mf); err != nil {
		fmt.Fprintf(os.Stderr, "解析失败：%v\n", err)
		os.Exit(1)
	}
	fmt.Printf("  机制卡：%d 位角色、%d 条全局规则（%s）\n",
		len(mf.Characters), len(mf.Rules), filepath.Base(*file))

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
	if err := db.AutoMigrate(&model.CharacterMechanic{}, &model.MechanicRule{}); err != nil {
		fmt.Fprintf(os.Stderr, "建表失败：%v\n", err)
		os.Exit(1)
	}

	// -probe / -questions 都是**只读**模式：走真链路组装 summary ✓
	if strings.TrimSpace(*probe) != "" {
		runProbe(db, *probe)
		return
	}
	if strings.TrimSpace(*questions) != "" {
		runBatch(db, *questions, *out)
		return
	}

	// 先读自动初稿（可选 ✓）：名字要**一起**送进 characters 查询 ✓，
	// 否则只在初稿里出现的角色会被误判成"不在 characters 表里"✗
	var autoCards []map[string]any
	if !*noAuto {
		if rawAuto, err := os.ReadFile(*autoFile); err == nil {
			var af mechanicsFile
			if err := json.Unmarshal(rawAuto, &af); err != nil {
				fmt.Fprintf(os.Stderr, "解析自动卡失败：%v\n", err)
				os.Exit(1)
			}
			autoCards = af.Characters
			fmt.Printf("  自动初稿：%d 位（%s）\n", len(autoCards), filepath.Base(*autoFile))
		} else {
			fmt.Printf("  （没有自动初稿文件 %s ⇒ 只导人工卡 ✓）\n", *autoFile)
		}
	}

	// 名字 → characters.id（角色 id 是 Enka avatarId ✓，JSON 里只存名字 ✓）
	names := make([]string, 0, len(mf.Characters)+len(autoCards))
	for _, c := range append(append([]map[string]any{}, mf.Characters...), autoCards...) {
		if n := cardString(c["name"]); n != "" {
			names = append(names, n)
		}
	}
	var chars []struct {
		ID   int64
		Name string
	}
	if err := db.Table("characters").Select("id, name").Where("name IN ?", names).Scan(&chars).Error; err != nil {
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

	// ① 人工卡（reviewed / draft ✓）
	writtenCh, unchangedCh, skippedCh := importCards(db, mf.Characters, idByName, onlySet, *dryRun, "curated")
	// ② 自动初稿（review=auto ✓）—— 已有人工卡的角色会被跳过 ✓（绝不覆盖 ✗）
	autoWritten, autoUnchanged, autoSkipped := 0, 0, 0
	if len(autoCards) > 0 {
		fmt.Printf("  --- 自动初稿（review=auto ✓，不覆盖人工卡 ✗）---\n")
		autoWritten, autoUnchanged, autoSkipped = importCards(db, autoCards, idByName, onlySet, *dryRun, "auto")
	}

	var writtenRu, unchangedRu int

	for _, r := range mf.Rules {
		key := strings.TrimSpace(r.ID)
		if key == "" {
			continue
		}
		content := strings.TrimSpace(r.Text)
		if content == "" {
			fmt.Printf("  [跳过] 规则 %-28s 正文为空 ✗\n", key)
			continue
		}
		triggers := strings.Join(cleanList(r.Triggers), ",")
		row := model.MechanicRule{
			RuleKey:   key,
			Title:     strings.TrimSpace(r.Title),
			Content:   content,
			Triggers:  triggers,
			Confirmed: r.Confirmed,
		}
		row.ContentHash = hashParts(row.RuleKey, row.Title, row.Content, row.Triggers, fmt.Sprint(row.Confirmed))

		var existing model.MechanicRule
		found := db.Where("rule_key = ?", key).Limit(1).Find(&existing).RowsAffected > 0
		if found && existing.ContentHash == row.ContentHash {
			fmt.Printf("  [未变] 规则 %-28s %4d 字（确认=%v，触发词 %d 个）\n",
				key, len([]rune(content)), row.Confirmed, len(cleanList(r.Triggers)))
			unchangedRu++
			continue
		}
		action := "新增"
		if found {
			action = "更新"
		}
		// ⚠️ 未确认的规则**只入库、不注入**（注入侧按 confirmed 过滤 ✗）—— 打印出来提醒 ✓
		fmt.Printf("  [%s] 规则 %-28s %4d 字（确认=%v，触发词 %d 个）%s\n",
			action, key, len([]rune(content)), row.Confirmed, len(cleanList(r.Triggers)),
			map[bool]string{true: "", false: " ⚠️ 未确认 ⇒ 不会注入"}[row.Confirmed])
		if *dryRun {
			continue
		}
		if err := db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "rule_key"}},
			DoUpdates: clause.AssignmentColumns([]string{"title", "content", "triggers", "confirmed", "content_hash", "updated_at"}),
		}).Create(&row).Error; err != nil {
			fmt.Fprintf(os.Stderr, "写入规则 %s 失败：%v\n", key, err)
			os.Exit(1)
		}
		writtenRu++
	}

	fmt.Printf("  完成：人工卡 写入 %d / 未变 %d / 跳过 %d；自动初稿 写入 %d / 未变 %d / 跳过(含保护) %d；规则 写入 %d / 未变 %d（dry-run=%v）\n",
		writtenCh, unchangedCh, skippedCh, autoWritten, autoUnchanged, autoSkipped, writtenRu, unchangedRu, *dryRun)
}

// factsJSON 把一张卡序列化成**只含事实**的 JSON：剥掉下划线开头的元字段（_review / _draft ✓）、
// 以及 `evidenceIndex`（那是给人看的页码索引 ✗，进注入文本只会浪费预算 ✓）。
// ⚠️ 每段事实里的 `evidence` **保留在库里** ✓（人工复核与后台要看 ✓），
// 但渲染器（service.renderMechanic）不会取它 ✗ ⇒ 不会进上下文 ✓。
func factsJSON(card map[string]any) (string, error) {
	clean := make(map[string]any, len(card))
	for k, v := range card {
		if strings.HasPrefix(k, "_") || k == "evidenceIndex" {
			continue
		}
		clean[k] = v
	}
	b, err := json.Marshal(clean)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// importCards 把一份卡片数组写进 character_mechanics：幂等 ✓ + **绝不覆盖人工卡** ✗。
//
// reviewKind：`curated`（人工卡 ✓，按 `_review`/`_draft` 定 reviewed|draft）/ `auto`（自动抽取 ✓，写 auto）。
// 返回 写入 / 未变 / 跳过 三个计数。
//
// ⚠️ 三态的意义（2026-10-13 踩到后补的 ✗）：人工卡里也有 **draft**（初稿待复核 ✓）——
// 它同样**不能被自动卡盖掉** ✗（人工写的 6 张初稿比自动抽取更准 ✓）。
// ⇒ 守卫是"库里已存在、且不是 auto ⇒ 跳过" ✓，而不是只看 reviewed ✗。
func importCards(db *gorm.DB, cards []map[string]any, idByName map[string]int64, onlySet map[string]bool, dryRun bool, reviewKind string) (written, unchanged, skipped int) {
	for _, card := range cards {
		name := cardString(card["name"])
		if name == "" || (len(onlySet) > 0 && !onlySet[name]) {
			continue
		}
		id, ok := idByName[name]
		if !ok {
			fmt.Printf("  [跳过] %-8s 不在 characters 表里（先补资产映射 ✓）\n", name)
			skipped++
			continue
		}

		facts, err := factsJSON(card)
		if err != nil {
			fmt.Fprintf(os.Stderr, "序列化 %s 的机制卡失败：%v\n", name, err)
			os.Exit(1)
		}
		row := model.CharacterMechanic{
			CharacterID: id,
			Name:        name,
			Facts:       facts,
			Review:      reviewOf(card),
		}
		if reviewKind == "auto" {
			row.Review = "auto" // 自动抽取的标记 ✓（与人工的 reviewed / draft 区分开 ✓）
		}
		row.ContentHash = hashParts(row.Name, row.Facts, row.Review)

		var existing model.CharacterMechanic
		found := db.Where("character_id = ?", id).Limit(1).Find(&existing).RowsAffected > 0
		if found && reviewKind == "auto" && existing.Review != "auto" {
			fmt.Printf("  [保护] %-8s id=%-9d 库里是人工卡（%s）⇒ 不覆盖 ✓\n", name, id, existing.Review)
			skipped++
			continue
		}
		if found && existing.ContentHash == row.ContentHash {
			fmt.Printf("  [未变] %-8s id=%-9d 事实 %4d 字（%s）\n", name, id, len([]rune(row.Facts)), row.Review)
			unchanged++
			continue
		}
		action := "新增"
		if found {
			action = "更新"
		}
		fmt.Printf("  [%s] %-8s id=%-9d 事实 %4d 字（%s）\n", action, name, id, len([]rune(row.Facts)), row.Review)
		if dryRun {
			continue
		}
		if err := db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "character_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"name", "facts", "review", "content_hash", "updated_at"}),
		}).Create(&row).Error; err != nil {
			fmt.Fprintf(os.Stderr, "写入 %s 失败：%v\n", name, err)
			os.Exit(1)
		}
		written++
	}
	return written, unchanged, skipped
}

func reviewOf(card map[string]any) string {
	s := cardString(card["_review"])
	// 自动初稿的 `_review` 写的是「自动抽取（初稿，未人工复核）」✓ ⇒ 一并认成 draft ✓
	if strings.Contains(s, "待复核") || strings.Contains(s, "未人工复核") || strings.Contains(s, "初稿") || card["_draft"] == true {
		return "draft"
	}
	return "reviewed"
}

func cardString(v any) string {
	if s, ok := v.(string); ok {
		return strings.TrimSpace(s)
	}
	return ""
}

func cleanList(items []string) []string {
	out := make([]string, 0, len(items))
	for _, s := range items {
		if v := strings.TrimSpace(s); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// ---------------------------------------------------------------- 只读模式

// runProbe 打印「这个问题真正会注入给模型的全部资料文本」——
// 走的是与线上**完全相同**的代码路径：asset 名字命中 → 直查库 → 机制卡 + 攻略卡 + 权威数值 ✓
// ⇒ 不用登录、不用起服务，就能看到上下文长什么样、体积多大 ✓
func runProbe(db *gorm.DB, question string) {
	players := service.NewPlayerService(db)
	text, stats := players.SummaryForQuestion(question)
	fmt.Printf("  问题「%s」\n", question)
	fmt.Printf("  组装：机制卡 %d 张、全局规则 %d 条（%s）、攻略卡 %d 张、词条 %d 位、套装 %d 个\n",
		stats.MechanicCards, len(stats.Rules), strings.Join(stats.Rules, ","),
		stats.GuideCards, stats.Refs, stats.Sets)
	if text == "" {
		fmt.Printf("  ⇒ 什么都没命中（本次不注入 ✓ 走原逻辑）\n")
		return
	}
	fmt.Printf("  ⇒ 注入 %d 字（≈ %d token 粗估 ✓）：\n\n%s\n", len([]rune(text)), len([]rune(text)), text)
}

// runBatch 批量导出每题会注入的 summary（给 AI 侧评测器复刻线上口径 ✓）。
// 只读 ✓，不写库 ✓。
func runBatch(db *gorm.DB, questionsPath, outPath string) {
	raw, err := os.ReadFile(questionsPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "读不到评测题：%s\n", questionsPath)
		os.Exit(1)
	}
	var qf questionFile
	if err := json.Unmarshal(raw, &qf); err != nil {
		fmt.Fprintf(os.Stderr, "解析评测题失败：%v\n", err)
		os.Exit(1)
	}
	if outPath == "" {
		outPath = filepath.Join(filepath.Dir(questionsPath), "mechanics_contexts.json")
	}

	players := service.NewPlayerService(db)
	res := contextOut{
		GeneratedAt: time.Now().Format("2006-01-02 15:04:05"),
		Source:      filepath.Base(questionsPath),
	}
	empty := 0
	for _, q := range qf.Questions {
		summary, stats := players.SummaryForQuestion(q.Q)
		if summary == "" {
			empty++
		}
		res.Items = append(res.Items, contextRow{ID: q.ID, Q: q.Q, Summary: summary})
		fmt.Printf("  [%2d] 机制 %d 卡 / 规则 %d / 攻略 %d / 词条 %d  ⇒ %d 字  %s\n",
			q.ID, stats.MechanicCards, len(stats.Rules), stats.GuideCards, stats.Refs,
			len([]rune(summary)), truncate(q.Q, 30))
	}
	b, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "序列化失败：%v\n", err)
		os.Exit(1)
	}
	// ⚠️ newline="\n"：别让 Windows 把 \n 翻成 \r\n ✗（评测时与线上文本就不同了 ✓）
	if err := os.WriteFile(outPath, append(b, '\n'), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "写不了 %s：%v\n", outPath, err)
		os.Exit(1)
	}
	fmt.Printf("  完成：%d 题（其中 %d 题没有任何资料命中）⇒ %s\n", len(res.Items), empty, outPath)
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
