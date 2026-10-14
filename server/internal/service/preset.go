package service

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"teyvat-guide/server/internal/dto"
	"teyvat-guide/server/internal/model"
	"teyvat-guide/server/internal/pkg/logger"
	"teyvat-guide/server/internal/repository"
)

// 预设问答的字段上限（按**字符**计，不是字节）
const (
	presetSceneMaxRunes    = 32
	presetQuestionMaxRunes = 200  // 表列 size:255
	presetAnswerMaxRunes   = 4000 // 太长没人看，也避免前端撑爆
	presetSortMin          = -9999
	presetSortMax          = 9999
)

// ErrPresetNotFound 预设问答不存在（Handler 映射 404）
var ErrPresetNotFound = errors.New("预设问答不存在")

// ErrPresetInvalid 参数不合法（Handler 映射 400）
var ErrPresetInvalid = errors.New("预设问答参数有误")

// PresetQAService 预设问答：匿名用户在对话页看到的内容。
// 匿名已被禁用 AI 对话（口径见 shared-docs/05 §1），因此这条链路
// **不调用模型、不消耗 token、不落库会话**，只是把运营准备好的问答发出去。
type PresetQAService struct {
	repo *repository.PresetQARepository
}

// NewPresetQAService 创建服务；表为空时灌入种子数据（改文案直接改库，下架用 enabled=0）
func NewPresetQAService() *PresetQAService {
	s := &PresetQAService{repo: repository.NewPresetQARepository()}
	s.ensureSeed()
	return s
}

// List 全部启用的预设问答
func (s *PresetQAService) List() (*dto.PresetQAList, error) {
	rows, err := s.repo.ListEnabled()
	if err != nil {
		return nil, err
	}
	items := make([]dto.PresetQAItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, dto.PresetQAItem{
			ID:       row.ID,
			SceneKey: row.SceneKey,
			Question: row.Question,
			Answer:   row.Answer,
			Sort:     row.Sort,
		})
	}
	return &dto.PresetQAList{Items: items}, nil
}

// ensureSeed 表为空时灌默认问答。
// 内容刻意保持"通用 + 引导登录"：具体到某个角色/账号的建议需要玩家档案，那是登录后的事。
func (s *PresetQAService) ensureSeed() {
	count, err := s.repo.Count()
	if err != nil {
		logger.L().Warnw("preset qa: count failed", "err", err)
		return
	}
	if count > 0 {
		return
	}

	items := []model.PresetQA{
		{
			SceneKey: "onboarding", Sort: 1,
			Question: "新手开局应该优先做什么？",
			Answer: "先把主线推到能开「每日委托」和「周本」，然后固定做三件事：\n" +
				"1. 每天把树脂用掉（优先合成浓缩树脂）；\n" +
				"2. 做完每日委托领原石；\n" +
				"3. 每周打周本拿突破材料。\n" +
				"开局不用急着抽角色，先把一支队伍练到能稳定过秘境，再决定抽卡方向。",
		},
		{
			SceneKey: "onboarding", Sort: 2,
			Question: "树脂怎么用最不浪费？",
			Answer: "按这个优先级用：\n" +
				"1. 周本（每周有次数上限，优先打满）；\n" +
				"2. 当前主力角色的突破材料与天赋书；\n" +
				"3. 武器突破材料；\n" +
				"4. 最后才是刷圣遗物。\n" +
				"树脂上限是 160，白天没时间玩就合成浓缩树脂存着，别让它溢出。",
		},
		{
			SceneKey: "onboarding", Sort: 3,
			Question: "想抽角色，原石该怎么规划？",
			Answer: "先定「一支主力队伍」，再围绕它抽人：\n" +
				"1. 明确一个核心输出（主C），资源优先给他；\n" +
				"2. 抽卡前先看保底还差多少抽，别在多个卡池之间分散；\n" +
				"3. 版本活动、每日委托、深渊是稳定的原石来源，能拿的都拿。\n" +
				"登录后派蒙可以结合你的展柜角色，说说先补哪个位置更划算。",
		},
		{
			SceneKey: "build", Sort: 1,
			Question: "圣遗物先看什么？",
			Answer: "顺序是：套装效果 → 主词条 → 副词条。\n" +
				"1. 先凑齐需要的 4 件套（多数情况下套装效果比单件词条更重要）；\n" +
				"2. 再确认主词条方向对不对（输出角色通常是攻击/元素伤害/暴击类）；\n" +
				"3. 最后才是堆副词条（暴击率、暴击伤害、充能效率等，不同角色优先级不同）。\n" +
				"想知道你练的角色具体该堆哪些词条，登录后派蒙可以按你的展柜逐件对照。",
		},
		{
			SceneKey: "build", Sort: 2,
			Question: "怎么打元素反应伤害更高？",
			Answer: "关键在「谁先挂元素、谁触发反应」：\n" +
				"1. 先让辅助挂上要反应的元素（比如火、水）；\n" +
				"2. 再让主C按顺序触发（蒸发/融化这类倍率反应收益最高）；\n" +
				"3. 注意元素附着有内置冷却，别指望一次附着触发多次反应；\n" +
				"4. 队伍里留一个能提供治疗或护盾的角色，站得住才打得出伤害。",
		},
		{
			SceneKey: "explore", Sort: 1,
			Question: "深境螺旋怎么配队比较好？",
			Answer: "思路是「两支完整队伍」而不是一支队伍打到底：\n" +
				"1. 一队偏对单（打单体 BOSS 类），一队偏对群（清小怪）；\n" +
				"2. 每队都要有明确的输出核心 + 挂元素/聚怪 + 生存位；\n" +
				"3. 注意上下半场的元素需求（比如需要破除特定护盾时提前留角色）；\n" +
				"4. 层数卡住时优先补练度（等级/天赋/武器），比反复换阵容更有效。",
		},
	}

	if err := s.repo.Seed(items); err != nil {
		logger.L().Errorw("preset qa: seed failed", "err", err)
		return
	}
	logger.L().Infow("preset qa seeded", "count", len(items))
}

// ---------- 后台维护（内网读写；下架用 enabled=0，不做物理删除） ----------

// ListAll 全部条目（含已下架）
func (s *PresetQAService) ListAll() (*dto.PresetQAAdminList, error) {
	rows, err := s.repo.ListAll()
	if err != nil {
		return nil, err
	}
	items := make([]dto.PresetQAAdminItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, toAdminItem(row))
	}
	return &dto.PresetQAAdminList{Items: items}, nil
}

// Create 新建（Enabled 缺省为 true）
func (s *PresetQAService) Create(req dto.PresetQASaveRequest) (*dto.PresetQAAdminItem, error) {
	req, err := normalizeSave(req)
	if err != nil {
		return nil, err
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	item := &model.PresetQA{
		SceneKey: req.SceneKey,
		Question: req.Question,
		Answer:   req.Answer,
		Sort:     req.Sort,
		Enabled:  enabled,
	}
	if err := s.repo.Create(item); err != nil {
		return nil, err
	}
	logger.L().Infow("preset qa created", "id", item.ID, "enabled", item.Enabled)
	out := toAdminItem(*item)
	return &out, nil
}

// Update 更新（Enabled 为 nil 时保持原值）。
// 先查存在性再更新：MySQL 在"值没变化"时 RowsAffected 为 0，不能据此判 404。
func (s *PresetQAService) Update(id uint, req dto.PresetQASaveRequest) (*dto.PresetQAAdminItem, error) {
	current, err := s.repo.FindByID(id)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, ErrPresetNotFound
	}
	req, err = normalizeSave(req)
	if err != nil {
		return nil, err
	}

	fields := map[string]any{
		"scene_key": req.SceneKey,
		"question":  req.Question,
		"answer":    req.Answer,
		"sort":      req.Sort,
	}
	if req.Enabled != nil {
		fields["enabled"] = *req.Enabled
	}
	if err := s.repo.Update(id, fields); err != nil {
		return nil, err
	}

	updated, err := s.repo.FindByID(id)
	if err != nil {
		return nil, err
	}
	if updated == nil {
		return nil, ErrPresetNotFound
	}
	logger.L().Infow("preset qa updated", "id", id, "enabled", updated.Enabled)
	out := toAdminItem(*updated)
	return &out, nil
}

// normalizeSave 去首尾空白 + 校验长度（按字符数）
func normalizeSave(req dto.PresetQASaveRequest) (dto.PresetQASaveRequest, error) {
	req.SceneKey = strings.TrimSpace(req.SceneKey)
	req.Question = strings.TrimSpace(req.Question)
	req.Answer = strings.TrimSpace(req.Answer)

	if req.Question == "" {
		return req, fmt.Errorf("%w: 提问不能为空", ErrPresetInvalid)
	}
	if req.Answer == "" {
		return req, fmt.Errorf("%w: 回答不能为空", ErrPresetInvalid)
	}
	if utf8.RuneCountInString(req.SceneKey) > presetSceneMaxRunes {
		return req, fmt.Errorf("%w: 分组最多 %d 个字符", ErrPresetInvalid, presetSceneMaxRunes)
	}
	if utf8.RuneCountInString(req.Question) > presetQuestionMaxRunes {
		return req, fmt.Errorf("%w: 提问最多 %d 个字符", ErrPresetInvalid, presetQuestionMaxRunes)
	}
	if utf8.RuneCountInString(req.Answer) > presetAnswerMaxRunes {
		return req, fmt.Errorf("%w: 回答最多 %d 个字符", ErrPresetInvalid, presetAnswerMaxRunes)
	}
	if req.Sort < presetSortMin || req.Sort > presetSortMax {
		return req, fmt.Errorf("%w: 排序需在 %d~%d 之间", ErrPresetInvalid, presetSortMin, presetSortMax)
	}
	return req, nil
}

func toAdminItem(row model.PresetQA) dto.PresetQAAdminItem {
	return dto.PresetQAAdminItem{
		ID:        row.ID,
		SceneKey:  row.SceneKey,
		Question:  row.Question,
		Answer:    row.Answer,
		Sort:      row.Sort,
		Enabled:   row.Enabled,
		CreatedAt: row.CreatedAt.Format(time.RFC3339),
		UpdatedAt: row.UpdatedAt.Format(time.RFC3339),
	}
}
