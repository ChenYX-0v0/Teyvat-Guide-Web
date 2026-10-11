package repository

// 玩家档案快照持久化

import (
	"teyvat-guide/server/internal/model"
	"teyvat-guide/server/internal/pkg/database"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// PlayerRepository 玩家快照读写
type PlayerRepository struct{}

func NewPlayerRepository() *PlayerRepository { return &PlayerRepository{} }

func (r *PlayerRepository) db() *gorm.DB {
	return database.DB()
}

// SaveSnapshot upsert 玩家档案快照（按 uid 覆盖）
func (r *PlayerRepository) SaveSnapshot(uid, profileJSON string) error {
	if r.db() == nil {
		return nil
	}
	return r.db().Clauses(clause.OnConflict{UpdateAll: true}).Create(&model.PlayerProfileSnapshot{
		UID:     uid,
		Profile: profileJSON,
	}).Error
}

// ClearSnapshots 清空全部档案快照（映射数据变更后旧翻译失效，需重新生成）
func (r *PlayerRepository) ClearSnapshots() error {
	if r.db() == nil {
		return nil
	}
	return r.db().Where("1 = 1").Delete(&model.PlayerProfileSnapshot{}).Error
}

// UpsertCharacters 批量 upsert「已收录角色」（2026-10-11）—— **只增/只改，永不删** ✓。
//
// ⚠️ 冲突时**只更新这四个字段**：card / in_showcase / last_seen_at / updated_at。
// 特意**不更新 first_seen_at** ✗ —— 它要保留"这个角色首次被收录"的时间 ✓
//（若用 UpdateAll 会把首次时间一路刷成当前时间，历史就没了 ✓）。
func (r *PlayerRepository) UpsertCharacters(rows []model.PlayerCharacter) error {
	if r.db() == nil || len(rows) == 0 {
		return nil
	}
	return r.db().Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "uid"}, {Name: "character_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"card", "in_showcase", "last_seen_at", "updated_at"}),
	}).Create(&rows).Error
}

// MarkCharactersOutOfShowcase 把该 UID 下**不在本轮展柜**里的角色 `in_showcase` 置 false ✓
// （不是删除 ✗：历史角色继续留着，用户换回展柜时还能看到 ✓）。
// keepIDs 为空表示本轮一个都没见到 —— 此时整账号置 false ✓（不会误删任何行 ✓）。
func (r *PlayerRepository) MarkCharactersOutOfShowcase(uid string, keepIDs []int64) error {
	if r.db() == nil {
		return nil
	}
	q := r.db().Model(&model.PlayerCharacter{}).Where("uid = ?", uid)
	if len(keepIDs) > 0 {
		q = q.Where("character_id NOT IN ?", keepIDs)
	}
	return q.Update("in_showcase", false).Error
}

// ListCharacters 读取该 UID 已收录的全部角色：**展柜中的在前** ✓，
// 同一组内按"最近见到"倒序 ✓（界面上越靠前越新鲜 ✓）。
func (r *PlayerRepository) ListCharacters(uid string) ([]model.PlayerCharacter, error) {
	if r.db() == nil {
		return nil, nil
	}
	var list []model.PlayerCharacter
	err := r.db().Where("uid = ?", uid).
		Order("in_showcase desc, last_seen_at desc").
		Find(&list).Error
	return list, err
}

// GetSnapshot 读取快照（不存在返回 nil, nil）
func (r *PlayerRepository) GetSnapshot(uid string) (*model.PlayerProfileSnapshot, error) {
	if r.db() == nil {
		return nil, nil
	}
	var snapshot model.PlayerProfileSnapshot
	err := r.db().First(&snapshot, "uid = ?", uid).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &snapshot, nil
}
