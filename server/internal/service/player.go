package service

// PlayerService 玩家查询编排：Enka 拉取 → 映射翻译 → 结构化输出
// 查询成功后异步将档案快照（JSON）落库至 player_profiles 表

import (
	"context"
	"encoding/json"

	"teyvat-guide/server/internal/dto"
	"teyvat-guide/server/internal/model"
	"teyvat-guide/server/internal/pkg/logger"
	"teyvat-guide/server/internal/repository"
	"teyvat-guide/server/internal/service/asset"
	"teyvat-guide/server/internal/service/enka"

	"gorm.io/gorm"
)

type PlayerService struct {
	assets     *asset.Service
	enka       *enka.Client
	translator *enka.Translator
	repo       *repository.PlayerRepository
	userRepo   *repository.UserRepository
}

// NewPlayerService db 用于映射数据持久层与快照存储（nil 时翻译走 fallback、快照不落库）
func NewPlayerService(db *gorm.DB) *PlayerService {
	assets := asset.New(db)
	if err := assets.Load(); err != nil {
		logger.L().Warnw("asset maps load failed, fallback names will be used", "err", err)
	}
	return &PlayerService{
		assets:     assets,
		enka:       enka.NewClient(),
		translator: enka.NewTranslator(assets),
		repo:       repository.NewPlayerRepository(),
		userRepo:   repository.NewUserRepository(),
	}
}

// Query 查询玩家档案
// 默认先查本地快照（player_profiles 表），命中直接返回；未命中或 forceRefresh 时才调 Enka API
// 查询成功后异步保存快照
func (s *PlayerService) Query(ctx context.Context, uid string, forceRefresh bool) (*dto.PlayerProfile, error) {
	if !forceRefresh {
		if profile, err := s.cachedProfile(uid); err == nil && profile != nil {
			logger.L().Infow("player profile served from cache", "uid", uid)
			return profile, nil
		}
	}

	profile, err := s.enka.FetchProfile(ctx, uid)
	if err != nil {
		return nil, err
	}
	result := s.translator.TranslateProfile(profile)
	s.saveSnapshot(result)
	return result, nil
}

// cachedProfile 从快照表恢复档案（表未启用 / 无记录 / 解析失败均返回 nil）
func (s *PlayerService) cachedProfile(uid string) (*dto.PlayerProfile, error) {
	snapshot, err := s.repo.GetSnapshot(uid)
	if err != nil || snapshot == nil {
		return nil, err
	}
	var profile dto.PlayerProfile
	if err := json.Unmarshal([]byte(snapshot.Profile), &profile); err != nil {
		logger.L().Warnw("unmarshal cached profile failed", "uid", uid, "err", err)
		return nil, err
	}
	return &profile, nil
}

// saveSnapshot 异步将档案 JSON 落库（uid 主键 upsert，失败仅记日志不阻塞查询）
func (s *PlayerService) saveSnapshot(p *dto.PlayerProfile) {
	raw, err := json.Marshal(p)
	if err != nil {
		logger.L().Warnw("marshal player profile failed", "uid", p.UID, "err", err)
		return
	}
	if err := s.repo.SaveSnapshot(p.UID, string(raw)); err != nil {
		logger.L().Warnw("save player snapshot failed", "uid", p.UID, "err", err)
		return
	}
	logger.L().Infow("player snapshot saved", "uid", p.UID, "bytes", len(raw))
}

// Snapshot 读取已落库的档案快照（不发起上游查询）
func (s *PlayerService) Snapshot(uid string) (*model.PlayerProfileSnapshot, error) {
	return s.repo.GetSnapshot(uid)
}

// ImportAssets 批量导入映射数据
func (s *PlayerService) ImportAssets(p asset.ImportPayload) error {
	return s.assets.Import(p)
}

// ---------- 账号 ↔ 游戏 UID 绑定（一对一） ----------

// BindUID 将 UID 绑定到用户：绑定前先通过 Enka 实查验证 UID 存在且档案公开
func (s *PlayerService) BindUID(ctx context.Context, userID uint, uid, server string) error {
	if _, err := s.enka.FetchProfile(ctx, uid); err != nil {
		return err // UID 不存在 / 未公开 / 上游异常，均拒绝绑定
	}
	return s.userRepo.UpdateBinding(userID, uid, server)
}

// Binding 查询用户绑定的 UID（未绑定时返回空串）
func (s *PlayerService) Binding(userID uint) (uid, server string, err error) {
	u, err := s.userRepo.FindByID(userID)
	if err != nil || u == nil {
		return "", "", err
	}
	return u.GenshinUID, u.GenshinServer, nil
}

// Unbind 解除绑定
func (s *PlayerService) Unbind(userID uint) error {
	return s.userRepo.UpdateBinding(userID, "", "")
}

// MissingAssets 缺漏清单
func (s *PlayerService) MissingAssets(limit int) ([]asset.Missing, error) {
	return s.assets.MissingList(limit)
}

// ReloadAssets 手动触发映射重载；purgeSnapshots 为真时同时清空档案快照
// （快照里存的是旧翻译结果，映射变更后不清会成为脏数据）
func (s *PlayerService) ReloadAssets(purgeSnapshots bool) error {
	if err := s.assets.Reload(); err != nil {
		return err
	}
	if purgeSnapshots {
		return s.PurgeSnapshots()
	}
	return nil
}

// PurgeSnapshots 清空全部档案快照
func (s *PlayerService) PurgeSnapshots() error {
	if err := s.repo.ClearSnapshots(); err != nil {
		logger.L().Warnw("clear player snapshots failed", "err", err)
		return err
	}
	logger.L().Infow("player snapshots cleared")
	return nil
}
