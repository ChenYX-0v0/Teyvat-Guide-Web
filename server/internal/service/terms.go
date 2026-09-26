package service

// 用户须知（免责声明）的版本管理。
//
// 设计取舍（2026-09-27 定，配合 AI 项目合规口径）：
//   · **版本号唯一真源在主服务**：前端不写死版本，由 `GET /api/auth/me` 下发
//     `termsCurrentVersion`；用户已同意的版本存列 `users.terms_version`。
//     两者不一致 → 前端**强制弹窗**（不可关闭），重新"阅读 5 秒 → 同意"。
//   · **存后端列而不是 localStorage**：换设备 / 清缓存不会重复弹，
//     且"已告知 + 用户已确认"留有服务端时间戳，可举证。
//   · 文案本身在前端（`web/src/components/legal/TermsContent.tsx`）：
//     **改文案 = 改这里的版本号**，所有用户会重新确认一次（这是刻意的，
//     否则用户"同意"的是旧条款）。
//
// 合规背景：`teyvat-docs/web/11-上线准备（域名·备案·HTTPS）.md` §7.4/§8
// （生成式 AI 应用侧合规：生成内容标识、投诉渠道、用户告知等）。

import (
	"errors"
	"strings"
	"time"
)

// CurrentTermsVersion 当前生效的用户须知版本。**改条款文案时同步 +1**
// （格式：v<主>.<次>-<日期>；日期用于人工核对发布批次）。
const CurrentTermsVersion = "v1.0-20260927"

var (
	// ErrTermsVersionInvalid 提交的版本号与当前生效版本不一致
	// （前端页面过期、或伪造请求）—— Handler 映射 400
	ErrTermsVersionInvalid = errors.New("用户须知已更新，请刷新页面后重新阅读")
)

// TermsState 用户须知状态（`/api/auth/me` 一并下发，供前端判断是否弹窗）
type TermsState struct {
	// AcceptedVersion 用户已同意的版本（空 = 从未同意）
	AcceptedVersion string `json:"acceptedVersion"`
	// AcceptedAt 同意时间（可空）
	AcceptedAt *time.Time `json:"acceptedAt"`
	// CurrentVersion 服务端当前生效版本
	CurrentVersion string `json:"currentVersion"`
}

// TermsState 查询某用户的须知状态
func (s *AuthService) TermsState(userID uint) (*TermsState, error) {
	u, err := s.Profile(userID)
	if err != nil {
		return nil, err
	}
	return &TermsState{
		AcceptedVersion: u.TermsVersion,
		AcceptedAt:      u.TermsAcceptedAt,
		CurrentVersion:  CurrentTermsVersion,
	}, nil
}

// AcceptTerms 记录"我已阅读并同意"。
//
// 只接受**当前生效版本**：旧页面/伪造请求提交的版本一律拒绝（否则会写进一个
// 无意义的版本号，让前端误以为"已同意最新条款"）。
// 校验先于任何数据库访问，因此这一层可以脱离 DB 单独测试。
func (s *AuthService) AcceptTerms(userID uint, version string) error {
	if strings.TrimSpace(version) != CurrentTermsVersion {
		return ErrTermsVersionInvalid
	}
	if userID == 0 {
		return ErrUserNotFound
	}
	return s.repo.UpdateTerms(userID, CurrentTermsVersion, time.Now())
}
