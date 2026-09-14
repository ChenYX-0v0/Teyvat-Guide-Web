package service

// AI 提示词只读服务（供 AI 派蒙服务按场景拉取）
//
// 契约：shared-docs/03-主服务对接说明.md §5
// 表与种子：AI 服务仓 `db/migrations/001_ai_prompts.sql`
//
// 口径：同一 scene_key 可有多版本（灰度 / 回滚），对外只暴露**启用且版本最高**的一条；
// 未命中的场景不出现在结果里（AI 服务侧按 not found 处理）。

import (
	"strings"

	"teyvat-guide/server/internal/dto"
	"teyvat-guide/server/internal/repository"
)

// AIPromptService 提示词读取
type AIPromptService struct {
	repo *repository.AIPromptRepository
}

func NewAIPromptService() *AIPromptService {
	return &AIPromptService{repo: repository.NewAIPromptRepository()}
}

// Current 返回各场景当前生效的提示词；scenes 为空则返回全部已启用场景
func (s *AIPromptService) Current(scenes []string) (*dto.AIPromptList, error) {
	rows, err := s.repo.ListEnabled(normalizeScenes(scenes))
	if err != nil {
		return nil, err
	}

	// 结果已按 scene_key asc, version desc 排序：每个场景的第一条就是当前生效版本
	items := make([]dto.AIPromptItem, 0, len(rows))
	seen := make(map[string]bool, len(rows))
	for _, row := range rows {
		if seen[row.SceneKey] {
			continue
		}
		seen[row.SceneKey] = true
		items = append(items, dto.AIPromptItem{
			SceneKey: row.SceneKey,
			Content:  row.Content,
			Version:  row.Version,
		})
	}

	return &dto.AIPromptList{Items: items}, nil
}

// normalizeScenes 解析 scenes 参数：去空白、去重、丢掉空值；空结果表示"不过滤"
func normalizeScenes(scenes []string) []string {
	out := make([]string, 0, len(scenes))
	seen := make(map[string]bool, len(scenes))
	for _, s := range scenes {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}
