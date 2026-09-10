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
