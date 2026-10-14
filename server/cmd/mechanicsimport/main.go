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
	"sort"
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

// ---------------------------------------------------------------- 月兆角色（{{月兆开门人}} 占位符）

// moonEnablersFile 月兆角色的**唯一事实源**（`scripts/data/moon_enablers.json` ✓）。
//
// ⚠️ 为什么要有这层 ✗（2026-10-14 用户连续 4 次点出名单漏人：奈芙尔 / 雅珂达 / 叶洛亚 / 菲林斯 ✓）：
//
//	根因不是数据缺 ✗ —— 菲林斯的天赋原文**就写在我们自己的攻略卡里** ✓ ——
//	而是**名单靠人手抄进规则正文** ✗：谁被想起来谁就在名单里 ✓，漏掉就永远漏 ✓。
//	改成：事实源一份 JSON ✓，规则正文写占位符 `{{月兆开门人}}` ✓，导入时**自动渲染** ✓
//	⇒ 加人只动 JSON ✓，正文再也不会漏 ✗。
type moonEnablersFile struct {
	Meta     map[string]any `json:"_meta"`
	Enablers []moonEnabler  `json:"enablers"`
}

type moonEnabler struct {
	Name     string   `json:"name"`
	Talent   string   `json:"talent"`
	Kind     string   `json:"kind"`
	Branches []string `json:"branches"`
	Level    bool     `json:"level"`
	Evidence string   `json:"evidence"`
	Verified bool     `json:"verified"`
}

// moonEnablers 进程内唯一一份（main 里加载 ✓，规则导入时用 ✓）
var moonEnablers moonEnablersFile

// moonEnablerPlaceholder 规则正文里的占位符 ✓（`character_mechanics.json` 用 ✓）
const moonEnablerPlaceholder = "{{月兆开门人}}"

// loadMoonEnablers 读**与机制卡 JSON 同目录**的 moon_enablers.json ✓
//
//	缺文件 ⇒ 返回空 ✓（占位符会**原样留在正文里** ⇒ 一眼看得出没渲染 ✗，比悄悄少一段好 ✓）
func loadMoonEnablers(dir string) moonEnablersFile {
	var f moonEnablersFile
	raw, err := os.ReadFile(filepath.Join(dir, "moon_enablers.json"))
	if err != nil {
		return f
	}
	_ = json.Unmarshal(raw, &f)
	return f
}

// renderMoonEnablers 渲染成规则正文 —— **按「管哪条反应」分组** ✓
//
// ⚠️ 分组不是装饰 ✗（2026-10-14 用户点名：「为什么月结晶和月感电会出现奈芙尔，这怎么想的」✓）：
//
//	早先把 9 位压成一行 ⇒ 读起来像"谁都能开三条" ✗ ⇒ 会给出**错误搭配** ✓。
//	另外「只拉月兆等级」的三位（爱诺 / 叶洛亚 / 雅珂达 ✓）天赋原文只写"月兆上升一级"✓，
//	**不会把反应变成月曜** ✗ ⇒ 必须单列并写明"别当开门人用" ✓。
func renderMoonEnablers(f moonEnablersFile) string {
	order := []string{"月感电", "月绽放", "月结晶"}
	byBranch := map[string][]string{}
	var allRound, levelOnly []string
	for _, e := range f.Enablers {
		if !e.Verified || strings.TrimSpace(e.Name) == "" {
			continue
		}
		label := e.Name + "「" + strings.TrimPrefix(strings.TrimSpace(e.Talent), "月兆祝赐·") + "」"
		switch {
		case len(e.Branches) >= len(order):
			allRound = append(allRound, label)
		case len(e.Branches) == 0:
			if e.Level {
				levelOnly = append(levelOnly, label)
			}
		default:
			for _, b := range e.Branches {
				byBranch[b] = append(byBranch[b], label)
			}
		}
	}
	var b strings.Builder
	b.WriteString("**开门人（能把普通反应换成月曜 ✓；**各管各的** ✗ —— 别混用）**：\n")
	for _, br := range order {
		if len(byBranch[br]) == 0 {
			continue
		}
		b.WriteString("· **" + br + "** = " + strings.Join(byBranch[br], "、") + " ✓\n")
	}
	if len(allRound) > 0 {
		b.WriteString("· **三条全通** = " + strings.Join(allRound, "、") + " ✓\n")
	}
	if len(levelOnly) > 0 {
		b.WriteString("⚠️ **只拉「月兆等级 +1」（不换反应 ✗，只为凑「满辉」✓）** = " +
			strings.Join(levelOnly, "、") + " —— **别当开门人用** ✗\n")
	}
	b.WriteString("⚠️ 名单**不封口** ✗（没列到 ≠ 不能开门 ✓，以角色卡为准 ✓）；也**不许张冠李戴** ✗" +
		"（例：奈芙尔只能开月绽放 ✗，拿去打月结晶 / 月感电是错的 ✓）。")
	return strings.TrimRight(b.String(), "\n")
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
	// review=auto ✓；**与人工卡求并集** ✓（2026-10-14 起 ✓ —— 原先"绝不覆盖人工卡"✗ 的守卫已撤掉 ✓，
	// 见 importCards 里的 unionFacts 调用与注释 ✓）；文件不存在就跳过 ✓
	autoFile := flag.String("auto-file", filepath.Join("..", "scripts", "data", "character_mechanics_auto.json"),
		"自动抽取的机制卡初稿 JSON（可选 ✓）")
	noAuto := flag.Bool("no-auto", false, "不导入自动初稿（只导人工卡 ✓）")
	// -rebuild：从两份 JSON **重建**机制卡（文件 = 事实源 ✓、库 = 物化视图 ✓）
	//
	// ⚠️ 为什么需要它 ✗（2026-10-14 ✓）：默认导入是**并集** ✓（只加不减 ✗）——
	//   补内容时正好 ✓，但**挪内容**（例如把误放进 `conflicts` 的"收益低"挪去 `limits` ✓）
	//   就做不到 ✗：并集会把旧位置那份留着 ✓。重建 = 按文件重算一遍 ✓，
	//   库里有、文件里没有的条目会被**替换掉** ✓（库里存在、两份文件都没有的卡片**不动** ✗）。
	rebuild := flag.Bool("rebuild", false, "从两份 JSON 重建机制卡（union(人工, 自动) ＋ 替换语义 ✓，用于挪字段/清理 ✗）")
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
	// 月兆角色事实源（规则正文里的 {{月兆开门人}} 由它渲染 ✓；缺文件 ⇒ 占位符原样留着 ✗ 可见 ✓）
	moonEnablers = loadMoonEnablers(filepath.Dir(*file))
	{
		n := 0
		for _, e := range moonEnablers.Enablers {
			if e.Verified {
				n++
			}
		}
		if n == 0 {
			fmt.Println("  ⚠️ 月兆角色：没读到 moon_enablers.json（或没有已核实条目）⇒ 规则里的 {{月兆开门人}} 不会被替换 ✗")
		} else {
			fmt.Printf("  月兆角色：%d 位已核实（moon_enablers.json ✓；规则正文自动渲染 ✓）\n", n)
		}
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
	writtenCh, unchangedCh, skippedCh := 0, 0, 0
	// ② 自动初稿（review=auto ✓）—— 与人工卡**求并集** ✓（2026-10-14 起 ✓，见 importCards ✓）
	autoWritten, autoUnchanged, autoSkipped := 0, 0, 0
	if *rebuild {
		fmt.Printf("  --- 重建模式：以文件为准（人工 ∪ 自动 ✓，替换语义 ✓）---\n")
		writtenCh, unchangedCh, skippedCh = rebuildMechanics(db, mf.Characters, autoCards, idByName, onlySet, *dryRun)
	} else if !*noAuto && len(autoCards) > 0 {
		writtenCh, unchangedCh, skippedCh = importCards(db, mf.Characters, idByName, onlySet, *dryRun, "curated")
		fmt.Printf("  --- 自动初稿（review=auto ✓，与人工卡求并集 ✓）---\n")
		autoWritten, autoUnchanged, autoSkipped = importCards(db, autoCards, idByName, onlySet, *dryRun, "auto")
	} else {
		writtenCh, unchangedCh, skippedCh = importCards(db, mf.Characters, idByName, onlySet, *dryRun, "curated")
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
		// {{月兆开门人}} ⇒ 用 moon_enablers.json 渲染 ✓
		//   ⚠️ 名单**必须**从这里来 ✗（2026-10-14：手抄名单连续漏人 4 次 ✓ ⇒ 见 moonEnablersFile 的注释 ✓）
		if strings.Contains(content, moonEnablerPlaceholder) {
			content = strings.ReplaceAll(content, moonEnablerPlaceholder, renderMoonEnablers(moonEnablers))
		}
		// ⚠️ 自检：单条规则按**注入格式**渲染后若超过额度 ⇒ 注入时会被**整条跳过** ✗（静默丢失 ✗）
		//   2026-10-14 实测踩到：855 字的反应体系规则一直进不去、且**没有任何报错** ✓ ⇒ 导入时就把话说出来 ✓
		if n := service.RuleInjectionLen(r.Title, content, key); n > service.MechanicRuleCapChars {
			fmt.Printf("  ⚠️ 规则 %-26s 渲染后 %d 字 > 注入额度 %d ⇒ **注入时会被跳过** ✗（拆短或抬额度 ✓）\n",
				key, n, service.MechanicRuleCapChars)
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

// importCards 把一份卡片数组写进 character_mechanics：幂等 ✓ + **人工卡与自动卡求并集** ✓（2026-10-14 起 ✓）。
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
		// ⚠️ 2026-10-14：从「人工卡赢 ✗」改成 **求并集** ✓
		//
		// 用户口径 ✓：「这两个为什么要设置优先级，都把他们设置为优先级一致的资料就可以了」✓ ——
		//   人工卡与自动卡**都是资料** ✓（一个来自人、一个来自模型抽取，每条都带页面出处 ✓），
		//   各自缺的部分正好互补 ✓ ⇒ 应当**并集**，而不是"谁盖谁" ✗。
		//   旧守卫的本意只是防「整张卡被替换掉」✗（早期自动抽取手法保守、会丢内容 ✓）——
		//   并集完全满足这个需求 ✓（人工条目在前 ✓、按内容去重 ✓，两边都不丢 ✓），
		//   所以优先级/保护这套可以撤掉了 ✓。
		if found && reviewKind == "auto" && existing.Review != "auto" {
			merged, added, err := unionFacts(existing.Facts, row.Facts)
			if err != nil {
				fmt.Fprintf(os.Stderr, "合并 %s 的机制卡失败：%v\n", name, err)
				os.Exit(1)
			}
			if added == 0 {
				fmt.Printf("  [未变] %-8s id=%-9d 人工卡已含自动抽到的全部条目 ✓\n", name, id)
				unchanged++
				continue
			}
			// 只并上"库里没有的"条目 ✓（人工内容一字不动 ✗）；review 保留既有标记 ✓（draft / reviewed ✓）
			row.Facts = merged
			row.Review = existing.Review
			row.ContentHash = hashParts(row.Name, row.Facts, row.Review)
			fmt.Printf("  [并入] %-8s id=%-9d 人工卡 %4d 字 ＋ 自动新条目 %d 条 ⇒ %4d 字（%s）✓\n",
				name, id, len([]rune(existing.Facts)), added, len([]rune(row.Facts)), row.Review)
		} else if found && reviewKind != "auto" {
			// ⚠️ 人工卡这一遍也要求并集 ✗（否则会非幂等 ✓，2026-10-14 踩到 ✓）：
			//   库里那行可能已经含"上一轮并进去的自动条目" ✓，而文件里没有 ✗ ⇒
			//   若直接覆盖，就会把它们**删掉** ✗（下一次跑又要重并 ⇒ 每次导入都在抖 ✓）。
			//   口径：**以文件为准的排序在前** ✓（人工新写的条目领读 ✓），库里多出来的跟在后面 ✓。
			merged, added, err := unionFacts(row.Facts, existing.Facts)
			if err != nil {
				fmt.Fprintf(os.Stderr, "合并 %s 的机制卡失败：%v\n", name, err)
				os.Exit(1)
			}
			if added > 0 {
				fmt.Printf("  [保留] %-8s id=%-9d 库里另有 %d 条（上一轮并入的 ✓）⇒ 留在并集里 ✓\n", name, id, added)
			}
			row.Facts = merged
			row.ContentHash = hashParts(row.Name, row.Facts, row.Review)
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

// rebuildMechanics 从两份 JSON **重建** character_mechanics（最终事实 = union(人工, 自动) ✓）。
//
// 与 importCards（并集 ✗只加不减 ✓）的分工：
//
//	· 补内容 ⇒ 默认导入（并集 ✓）
//	· 挪字段 / 清理 ⇒ `-rebuild`（替换语义 ✓，把文件里已经挪走的条目从库里去掉 ✓）
//
// 口径（2026-10-14 ✓）：
//
//	· 有人工卡 ⇒ 事实 = 人工 ∪ 自动（**人工条目在前** ✓）；review 按人工卡定 ✓（draft / reviewed ✓）
//	· 只有自动卡 ⇒ 事实 = 自动 ✓；review = auto ✓
//	· 两份文件都没有的角色 ⇒ **不动** ✗（不删库里的行 ✓）
func rebuildMechanics(db *gorm.DB, curated, auto []map[string]any, idByName map[string]int64, onlySet map[string]bool, dryRun bool) (written, unchanged, skipped int) {
	hand := map[string]map[string]any{}
	for _, c := range curated {
		if n := cardString(c["name"]); n != "" {
			hand[n] = c
		}
	}
	autoByName := map[string]map[string]any{}
	for _, c := range auto {
		if n := cardString(c["name"]); n != "" {
			autoByName[n] = c
		}
	}
	names := make([]string, 0, len(hand)+len(autoByName))
	for n := range hand {
		names = append(names, n)
	}
	for n := range autoByName {
		if _, ok := hand[n]; !ok {
			names = append(names, n)
		}
	}
	sort.Strings(names)

	for _, name := range names {
		if len(onlySet) > 0 && !onlySet[name] {
			continue
		}
		id, ok := idByName[name]
		if !ok {
			fmt.Printf("  [跳过] %-8s 不在 characters 表里\n", name)
			skipped++
			continue
		}
		var facts string
		review := "auto"
		if hCard, hasHand := hand[name]; hasHand {
			hf, err := factsJSON(hCard)
			if err != nil {
				fmt.Fprintf(os.Stderr, "序列化 %s 失败：%v\n", name, err)
				os.Exit(1)
			}
			facts, review = hf, reviewOf(hCard)
			if aCard := autoByName[name]; aCard != nil {
				af, err := factsJSON(aCard)
				if err != nil {
					fmt.Fprintf(os.Stderr, "序列化 %s（自动）失败：%v\n", name, err)
					os.Exit(1)
				}
				merged, _, err := unionFacts(hf, af) // 人工在前 ✓
				if err != nil {
					fmt.Fprintf(os.Stderr, "合并 %s 失败：%v\n", name, err)
					os.Exit(1)
				}
				facts = merged
			}
		} else if aCard := autoByName[name]; aCard != nil {
			af, err := factsJSON(aCard)
			if err != nil {
				fmt.Fprintf(os.Stderr, "序列化 %s 失败：%v\n", name, err)
				os.Exit(1)
			}
			facts = af
		} else {
			continue
		}

		row := model.CharacterMechanic{CharacterID: id, Name: name, Facts: facts, Review: review}
		row.ContentHash = hashParts(row.Name, row.Facts, row.Review)
		var existing model.CharacterMechanic
		found := db.Where("character_id = ?", id).Limit(1).Find(&existing).RowsAffected > 0
		if found && existing.ContentHash == row.ContentHash {
			unchanged++
			continue
		}
		action := "新增"
		if found {
			action = "重建"
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

// unionFacts 把两份事实取**并集**（base 在前 ✓、按内容去重 ✓）—— 2026-10-14 加 ✓
//
// 口径（用户 2026-10-14 ✓）：人工卡与自动卡**优先级一致** ✓，都是资料 ⇒ 缺哪边补哪边 ✓。
//
//	· 字符串数组（cannot / needs / limits / points / conflicts / scaling ✓）：按"去掉空白与标点"后的文本去重 ✓
//	  （"不能降低风抗" 与 "不能降低风抗。" 算同一条 ✓）
//	· 对象数组（provides / shield / interruptResist / application ✓）：按整条 JSON 去重 ✓（人工条目在前 ✓）
//	· 对象（resonance ✓）：逐个子数组合并 ✓
//	· 标量：base 已有就保留 ✗（不覆盖 ✓）
//
// 返回：并集 JSON ＋ **新加了几条**（0 ＝ 没有新东西 ⇒ 调用方省一次写库 ✓）
func unionFacts(baseJSON, addJSON string) (string, int, error) {
	var base, add map[string]any
	if err := json.Unmarshal([]byte(baseJSON), &base); err != nil {
		return "", 0, err
	}
	if err := json.Unmarshal([]byte(addJSON), &add); err != nil {
		return "", 0, err
	}
	if base == nil {
		base = map[string]any{}
	}
	// base 侧的元字段也顺手清掉 ✓（库里存的本就没有 ✓，但离线调用时可能带上 ✓）
	for k := range base {
		if strings.HasPrefix(k, "_") || k == "evidenceIndex" {
			delete(base, k)
		}
	}
	added := 0
	for k, av := range add {
		// 元字段（_review / _draft / evidenceIndex…）只在文件里 ✓，不进库也并入 ✗
		if strings.HasPrefix(k, "_") || k == "evidenceIndex" {
			continue
		}
		bv, ok := base[k]
		if !ok || bv == nil {
			base[k] = av
			added += countItems(av)
			continue
		}
		switch at := av.(type) {
		case []any:
			bt, _ := bv.([]any)
			seen := map[string]bool{}
			for _, it := range bt {
				seen[unionKey(it)] = true
			}
			for _, it := range at {
				kk := unionKey(it)
				if seen[kk] {
					continue
				}
				// ⚠️ **包含关系也去重** ✗→✓（2026-10-14 加 ✓）：
				//   人工卡常写短句 ✓、自动卡写页面长句 ✓ ⇒ 同一条事实会以两种长度并存 ✓，
				//   渲染时单段只取前 4 条 ✗ ⇒ 重复会白占位置 ✓（实测：芙宁娜「增伤对剧变反应不生效」
				//   与它的长句版本同时进过上下文 ✓）。口径：**只留信息更全的那条** ✓。
				if _, isStr := it.(string); isStr {
					drop := false
					for i := 0; i < len(bt); i++ {
						bs, isStr2 := bt[i].(string)
						if !isStr2 {
							continue
						}
						bn := normalizeFactText(bs)
						if bn == kk || strings.Contains(bn, kk) {
							drop = true // 库里已有更全的 ✓
							break
						}
						if strings.Contains(kk, bn) { // 新这条更全 ⇒ 换掉旧的 ✓
							delete(seen, bn)
							bt = append(bt[:i], bt[i+1:]...)
							i--
						}
					}
					if drop {
						continue
					}
				}
				seen[kk] = true
				bt = append(bt, it)
				added++
			}
			base[k] = bt
		case map[string]any:
			bm, _ := bv.(map[string]any)
			if bm == nil {
				bm = map[string]any{}
			}
			for sk, sv := range at {
				sa, isArr := sv.([]any)
				if !isArr {
					if _, exist := bm[sk]; !exist {
						bm[sk] = sv
					}
					continue
				}
				ba, _ := bm[sk].([]any)
				seen := map[string]bool{}
				for _, it := range ba {
					seen[unionKey(it)] = true
				}
				for _, it := range sa {
					kk := unionKey(it)
					if seen[kk] {
						continue
					}
					seen[kk] = true
					ba = append(ba, it)
					added++
				}
				bm[sk] = ba
			}
			base[k] = bm
		default:
			// 标量（如 "to": "自身"）：base 已有一律保留 ✓
		}
	}
	out, err := json.Marshal(base)
	if err != nil {
		return "", 0, err
	}
	return string(out), added, nil
}

// unionKey 去重键：字符串去掉空白与常见标点后比较 ✓；对象取整条 JSON ✓
func unionKey(v any) string {
	if s, ok := v.(string); ok {
		return normalizeFactText(s)
	}
	b, _ := json.Marshal(v)
	return string(b)
}

func normalizeFactText(s string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case ' ', '\t', '\n', '\r', '　', '，', '。', '；', '、', '：', ':', ',', ';', '（', '）', '(', ')', '「', '」', '“', '”', '"', '\'', '·', '—', '-':
			return -1
		}
		return r
	}, s)
}

// countItems 数"加进来几条"（数组按元素数 ✓；对象算 1 ✓）
func countItems(v any) int {
	if a, ok := v.([]any); ok {
		return len(a)
	}
	return 1
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
