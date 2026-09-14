package repository

// 提示词读取 + 后台维护：读取侧只暴露「启用且版本最高」；写入侧只能"新增版本"与"启停"。

import (
	"errors"

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

// ---------- 后台维护（读写） ----------
//
// 纪律：**版本内容不可变**——改文案一律"新增一版"（服务端分配 version = max+1），
// 老版本只保留 enabled 开关用于回滚（口径见 shared-docs/03-主服务对接说明.md §5.2）。

// ListAll 全部版本（**含未启用**），按 (scene_key asc, version desc) 排序：
// 上层据此判断"每个场景当前生效版本"（该场景第一条 enabled 的行）。
func (r *AIPromptRepository) ListAll() ([]model.AIPrompt, error) {
	if r.db() == nil {
		return nil, nil
	}
	var list []model.AIPrompt
	err := r.db().Order("scene_key asc, version desc").Find(&list).Error
	return list, err
}

// FindByID 按 ID 取（不存在返回 nil, nil）
func (r *AIPromptRepository) FindByID(id uint64) (*model.AIPrompt, error) {
	if r.db() == nil {
		return nil, nil
	}
	var row model.AIPrompt
	err := r.db().First(&row, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

// MaxVersion 某场景当前最大版本号（无记录返回 0），用于服务端分配新版本号。
// 不用 MAX() 聚合而是取 version 最大的那行：两种数据库行为一致，也顺手拿到行。
func (r *AIPromptRepository) MaxVersion(sceneKey string) (int, error) {
	if r.db() == nil {
		return 0, nil
	}
	var row model.AIPrompt
	err := r.db().Where("scene_key = ?", sceneKey).Order("version desc").First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, nil
		}
		return 0, err
	}
	return row.Version, nil
}

// Create 新增一版。
// ⚠️ Enabled 带 `default:true`：GORM 插入时会**跳过 false 零值**（该列取库默认值 true），
// 连 `Select(...)` 也压不住——所以"先入库不发"（enabled=false）要在插入后显式改正一次，
// 否则运营在弹窗里取消"上架"再保存，反而会落成启用（有单测守着这个陷阱）。
func (r *AIPromptRepository) Create(row *model.AIPrompt) error {
	if r.db() == nil {
		return nil
	}
	// 注意顺序：GORM 插入带 default 的字段后会把库里的默认值**回写进结构体**
	// （row.Enabled 会变成 true），因此"是否要未启用"必须在插入前记下来。
	wantDisabled := !row.Enabled
	if err := r.db().Create(row).Error; err != nil {
		return err
	}
	if wantDisabled {
		if err := r.db().Model(&model.AIPrompt{}).
			Where("id = ?", row.ID).
			Update("enabled", false).Error; err != nil {
			return err
		}
		row.Enabled = false
	}
	return nil
}

// UpdateEnabled 只改启用开关（内容不可改）。
// 回滚语义：把当前生效版本置 0，即自动回落到该场景"下一个启用且版本最高"的行。
func (r *AIPromptRepository) UpdateEnabled(id uint64, enabled bool) error {
	if r.db() == nil {
		return nil
	}
	return r.db().Model(&model.AIPrompt{}).Where("id = ?", id).Update("enabled", enabled).Error
}
