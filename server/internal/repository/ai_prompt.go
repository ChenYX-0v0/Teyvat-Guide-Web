package repository

// 提示词读取（只读：写入由 SQL / 后续管理接口负责，AI 服务与本服务都不在运行时改提示词）

import (
	"teyvat-guide/server/internal/model"
	"teyvat-guide/server/internal/pkg/database"

	"gorm.io/gorm"
)

// AIPromptRepository 提示词读取
type AIPromptRepository struct{}

func NewAIPromptRepository() *AIPromptRepository { return &AIPromptRepository{} }

func (r *AIPromptRepository) db() *gorm.DB { return database.DB() }

// ListEnabled 取启用中的提示词行，按 (scene_key asc, version desc) 排序。
// scenes 为空表示不过滤场景；由上层取每个场景的第一条作为「当前生效版本」——
// 不用窗口函数，保证 MySQL 与测试用的 sqlite 行为一致。
func (r *AIPromptRepository) ListEnabled(scenes []string) ([]model.AIPrompt, error) {
	if r.db() == nil {
		return nil, nil
	}
	q := r.db().Where("enabled = ?", true)
	if len(scenes) > 0 {
		q = q.Where("scene_key IN ?", scenes)
	}
	var list []model.AIPrompt
	err := q.Order("scene_key asc, version desc").Find(&list).Error
	return list, err
}
