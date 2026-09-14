package service

// 覆盖 2026-09-14 落地的三件事（口径真源：shared-docs/05-AI使用边界设计（匿名·配额·内容治理）.md）：
//   §2 每用户每日 token 配额 + 并发闸门
//   §1 预设问答（匿名禁用 AI 后的替代内容）
//   §3 无关问题标记与后台筛选

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"teyvat-guide/server/internal/config"
	"teyvat-guide/server/internal/dto"
	"teyvat-guide/server/internal/model"
	"teyvat-guide/server/internal/pkg/database"
	"teyvat-guide/server/internal/repository"
)

// setupUsageTestDB 用 sqlite 临时库（与既有单测同套路：每个测试独立文件库，收尾先关连接）
func setupUsageTestDB(t *testing.T) {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "usage_quota_test.db")
	if err := database.Init(config.DatabaseConfig{Driver: "sqlite", DSN: dsn}); err != nil {
		t.Fatalf("初始化 sqlite 失败：%v", err)
	}
	db := database.DB()
	if err := db.AutoMigrate(
		&model.User{}, &model.ChatSession{}, &model.ChatMessage{}, &model.PresetQA{},
	); err != nil {
		t.Fatalf("迁移失败：%v", err)
	}
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
}

// stubAIClient 固定返回一段回复与用量（可带无关问题判定），避免依赖真实模型
type stubAIClient struct {
	reply string
	usage AIUsage
}

func (s *stubAIClient) Complete(_ context.Context, _ string, _ []Message, _ string, _ *RequestContext) (string, AIUsage, error) {
	return s.reply, s.usage, nil
}

func (s *stubAIClient) CompleteStream(
	ctx context.Context, system string, history []Message, prompt string, rc *RequestContext,
	onDelta func(string) error,
) (AIUsage, error) {
	reply, usage, err := s.Complete(ctx, system, history, prompt, rc)
	if err != nil {
		return usage, err
	}
	return usage, onDelta(reply)
}

func newStubChatService(quota *QuotaService, stub *stubAIClient) *ChatService {
	svc := NewChatService(config.AIConfig{Provider: "mock"}, nil, quota)
	svc.ai = stub
	return svc
}

// ---------- §2 配额 ----------

// 当日额度已满 → 拒绝，且**不建会话、不落消息**（检查发生在建会话之前）
func TestQuotaDailyLimitBlocksChat(t *testing.T) {
	setupUsageTestDB(t)
	db := database.DB()

	now := time.Now()
	if err := db.Create(&model.ChatSession{
		ID: "s1", Title: "已有会话", UserID: 1, CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("建会话失败：%v", err)
	}
	if err := db.Create(&model.ChatMessage{
		SessionID: "s1", Role: "assistant", Content: "已用满", Tokens: 100, CreatedAt: now,
	}).Error; err != nil {
		t.Fatalf("建消息失败：%v", err)
	}

	quota := NewQuotaService(config.QuotaConfig{DailyTokensPerUser: 100, MaxConcurrentPerUser: 2})
	svc := newStubChatService(quota, &stubAIClient{reply: "不该被调用"})

	_, _, _, _, err := svc.Ask(context.Background(), 1, "s1", "你好")
	var quotaErr *ErrQuotaExceeded
	if !errors.As(err, &quotaErr) {
		t.Fatalf("期望配额超限错误，实际 %v", err)
	}
	if quotaErr.Reason != QuotaReasonDaily {
		t.Fatalf("期望 daily，实际 %s", quotaErr.Reason)
	}
	if quotaErr.Used != 100 || quotaErr.Limit != 100 {
		t.Fatalf("用量字段不符：used=%d limit=%d", quotaErr.Used, quotaErr.Limit)
	}
	if quotaErr.ResetAt.IsZero() {
		t.Fatal("日配额超限应带 resetAt")
	}
	if quotaErr.Message() == "" {
		t.Fatal("应给出面向用户的文案")
	}

	// 被拒的请求不能产生任何新数据
	var sessions, messages int64
	if err := db.Model(&model.ChatSession{}).Where("user_id = ?", 1).Count(&sessions).Error; err != nil {
		t.Fatalf("统计会话失败：%v", err)
	}
	if err := db.Model(&model.ChatMessage{}).Count(&messages).Error; err != nil {
		t.Fatalf("统计消息失败：%v", err)
	}
	if sessions != 1 || messages != 1 {
		t.Fatalf("被拒请求落库了：sessions=%d messages=%d", sessions, messages)
	}
}

// 额度未满放行；白名单用户不受额度限制
func TestQuotaAllowsUnderLimitAndWhitelist(t *testing.T) {
	setupUsageTestDB(t)

	quota := NewQuotaService(config.QuotaConfig{DailyTokensPerUser: 100, MaxConcurrentPerUser: 2})
	svc := newStubChatService(quota, &stubAIClient{reply: "派蒙在", usage: AIUsage{Tokens: 10}})
	if _, _, _, _, err := svc.Ask(context.Background(), 7, "", "你好"); err != nil {
		t.Fatalf("额度未满应放行：%v", err)
	}

	// 当日已用 10 token，上限 100 → 仍放行；上限 5 → 拒绝
	if _, _, _, _, err := svc.Ask(context.Background(), 7, "", "再问一次"); err != nil {
		t.Fatalf("第二次仍应在额度内：%v", err)
	}
	tight := NewQuotaService(config.QuotaConfig{DailyTokensPerUser: 5})
	tightSvc := newStubChatService(tight, &stubAIClient{reply: "ok", usage: AIUsage{Tokens: 1}})
	if _, _, _, _, err := tightSvc.Ask(context.Background(), 7, "", "第三次"); err == nil {
		t.Fatal("已超过上限应被拒绝")
	}

	// 白名单：额度检查跳过
	whitelist := NewQuotaService(config.QuotaConfig{DailyTokensPerUser: 1, WhitelistUserIDs: []uint{8}})
	whitelistSvc := newStubChatService(whitelist, &stubAIClient{reply: "ok", usage: AIUsage{Tokens: 9999}})
	for i := 0; i < 2; i++ {
		if _, _, _, _, err := whitelistSvc.Ask(context.Background(), 8, "", "你好"); err != nil {
			t.Fatalf("白名单用户应放行（第 %d 次）：%v", i+1, err)
		}
	}
}

// 并发闸门：同一用户第 3 个在途请求被拒，释放后恢复；其他用户不受影响
func TestQuotaConcurrentGate(t *testing.T) {
	setupUsageTestDB(t)

	quota := NewQuotaService(config.QuotaConfig{MaxConcurrentPerUser: 2})
	release1, err := quota.Acquire(1)
	if err != nil {
		t.Fatalf("第 1 个请求应放行：%v", err)
	}
	release2, err := quota.Acquire(1)
	if err != nil {
		t.Fatalf("第 2 个请求应放行：%v", err)
	}

	_, err = quota.Acquire(1)
	var quotaErr *ErrQuotaExceeded
	if !errors.As(err, &quotaErr) || quotaErr.Reason != QuotaReasonConcurrent {
		t.Fatalf("期望并发超限，实际 %v", err)
	}
	if quotaErr.Limit != 2 || quotaErr.Used != 2 {
		t.Fatalf("并发字段不符：used=%d limit=%d", quotaErr.Used, quotaErr.Limit)
	}
	if !quotaErr.ResetAt.IsZero() {
		t.Fatal("并发超限不应带 resetAt（它不是按天重置的）")
	}

	// 其他用户互不影响
	releaseOther, err := quota.Acquire(2)
	if err != nil {
		t.Fatalf("其他用户不应被影响：%v", err)
	}
	releaseOther()

	// 释放后可再次占用
	release1()
	release2()
	release3, err := quota.Acquire(1)
	if err != nil {
		t.Fatalf("释放后应放行：%v", err)
	}
	release3()
}

// 配额全关（0 = 不限）时不做任何限制
func TestQuotaDisabledByDefaultConfig(t *testing.T) {
	setupUsageTestDB(t)

	quota := NewQuotaService(config.QuotaConfig{})
	if quota.Enabled() {
		t.Fatal("全 0 配置应视为未启用")
	}
	for i := 0; i < 5; i++ {
		release, err := quota.Acquire(1)
		if err != nil {
			t.Fatalf("未启用配额不应限制：%v", err)
		}
		release()
	}
}

// ---------- §1 预设问答 ----------

func TestPresetQASeedAndList(t *testing.T) {
	setupUsageTestDB(t)

	svc := NewPresetQAService()
	first, err := svc.List()
	if err != nil {
		t.Fatalf("查询失败：%v", err)
	}
	if len(first.Items) == 0 {
		t.Fatal("首次启动应灌入种子数据")
	}
	for _, it := range first.Items {
		if it.Question == "" || it.Answer == "" {
			t.Fatalf("预设问答内容不能为空：%+v", it)
		}
	}

	// 重复构造不会重复灌种子
	NewPresetQAService()
	count, err := repository.NewPresetQARepository().Count()
	if err != nil {
		t.Fatalf("统计失败：%v", err)
	}
	if count != int64(len(first.Items)) {
		t.Fatalf("重复灌了种子：期望 %d，实际 %d", len(first.Items), count)
	}

	// 下架（enabled=0）的条目不再下发，但数据仍保留
	disabled := first.Items[0].ID
	if err := database.DB().Model(&model.PresetQA{}).Where("id = ?", disabled).
		Update("enabled", false).Error; err != nil {
		t.Fatalf("下架失败：%v", err)
	}
	second, err := svc.List()
	if err != nil {
		t.Fatalf("查询失败：%v", err)
	}
	if len(second.Items) != len(first.Items)-1 {
		t.Fatalf("下架的条目仍下发：%d → %d", len(first.Items), len(second.Items))
	}
	for _, it := range second.Items {
		if it.ID == disabled {
			t.Fatal("下架的条目仍下发")
		}
	}
	if count, _ := repository.NewPresetQARepository().Count(); count != int64(len(first.Items)) {
		t.Fatal("下架不应删除数据")
	}
}

// ---------- §3 无关问题 ----------

// 判定结果写在**提问**那条消息上；nil 判定保持 NULL；后台可按会话 / 用户筛出来
func TestOffTopicPersistAndAdminFilters(t *testing.T) {
	setupUsageTestDB(t)
	db := database.DB()
	repo := repository.NewChatRepository()

	if err := db.Create(&model.User{
		ID: 1, Username: "u1", DisplayName: "U1", Status: model.UserStatusActive,
	}).Error; err != nil {
		t.Fatalf("建用户失败：%v", err)
	}

	offTopic := true
	quota := NewQuotaService(config.QuotaConfig{})
	svc := newStubChatService(quota, &stubAIClient{
		reply: "这个派蒙不太懂呢",
		usage: AIUsage{Tokens: 20, OffTopic: &offTopic, OffTopicReason: "other_domain"},
	})

	sessionID, _, _, _, err := svc.Ask(context.Background(), 1, "", "帮我写一段排序算法")
	if err != nil {
		t.Fatalf("Ask 失败：%v", err)
	}

	var userMsg model.ChatMessage
	if err := db.Where("session_id = ? AND role = 'user'", sessionID).First(&userMsg).Error; err != nil {
		t.Fatalf("读取提问失败：%v", err)
	}
	if userMsg.OffTopic == nil || !*userMsg.OffTopic || userMsg.OffTopicReason != "other_domain" {
		t.Fatalf("提问未被标记为无关：%+v", userMsg)
	}

	var assistantMsg model.ChatMessage
	if err := db.Where("session_id = ? AND role = 'assistant'", sessionID).First(&assistantMsg).Error; err != nil {
		t.Fatalf("读取回答失败：%v", err)
	}
	if assistantMsg.OffTopic != nil {
		t.Fatal("回答消息不应带无关判定")
	}

	// 未判定（nil）必须保持 NULL，不能被写成 false
	if err := repo.MarkMessageOffTopic(assistantMsg.ID, nil, ""); err != nil {
		t.Fatalf("nil 判定不应报错：%v", err)
	}
	var reloaded model.ChatMessage
	if err := db.First(&reloaded, assistantMsg.ID).Error; err != nil {
		t.Fatalf("重新读取失败：%v", err)
	}
	if reloaded.OffTopic != nil {
		t.Fatal("未判定被写成了非 NULL")
	}

	// 后台会话列表：onlyOffTopic 命中，且带 offTopicCount
	admin := NewChatAdminService()
	hit, err := admin.ListSessions(dto.AdminChatQuery{OnlyOffTopic: true})
	if err != nil {
		t.Fatalf("后台查询失败：%v", err)
	}
	if hit.Total != 1 || len(hit.Items) != 1 || hit.Items[0].OffTopicCount != 1 {
		t.Fatalf("onlyOffTopic 未命中：total=%d items=%+v", hit.Total, hit.Items)
	}
	// 明细里能看到判定
	detail, err := admin.Messages(sessionID, 0, 0)
	if err != nil {
		t.Fatalf("明细查询失败：%v", err)
	}
	var marked int
	for _, m := range detail.Messages {
		if m.OffTopic != nil && *m.OffTopic && m.OffTopicReason == "other_domain" {
			marked++
		}
	}
	if marked != 1 {
		t.Fatalf("明细里应有 1 条无关标记，实际 %d", marked)
	}

	// 用户列表：无关提问筛选 + 当日用量筛选（5 万观察线的同一条链路）
	users := NewUserAdminService()
	byOffTopic, err := users.List(dto.AdminUserQuery{OnlyOffTopic: true})
	if err != nil {
		t.Fatalf("用户列表失败：%v", err)
	}
	if byOffTopic.Total != 1 || byOffTopic.Items[0].OffTopicCount != 1 {
		t.Fatalf("onlyOffTopic 未命中用户：%+v", byOffTopic.Items)
	}
	if byOffTopic.Items[0].TodayTokens != 20 || byOffTopic.Items[0].TodayRequests != 1 {
		t.Fatalf("当日用量字段不符：%+v", byOffTopic.Items[0])
	}

	byUsage, err := users.List(dto.AdminUserQuery{MinDailyTokens: 10})
	if err != nil {
		t.Fatalf("按用量筛选失败：%v", err)
	}
	if byUsage.Total != 1 {
		t.Fatalf("今日用量 ≥ 10 应命中 1 人，实际 %d", byUsage.Total)
	}
	noHit, err := users.List(dto.AdminUserQuery{MinDailyTokens: 100000})
	if err != nil {
		t.Fatalf("按用量筛选失败：%v", err)
	}
	if noHit.Total != 0 || len(noHit.Items) != 0 {
		t.Fatalf("今日用量 ≥ 10 万应无人命中，实际 %d", noHit.Total)
	}
	// 两个条件叠加（今日用量高 **且** 有无关提问）
	both, err := users.List(dto.AdminUserQuery{MinDailyTokens: 10, OnlyOffTopic: true})
	if err != nil {
		t.Fatalf("组合筛选失败：%v", err)
	}
	if both.Total != 1 {
		t.Fatalf("组合筛选应命中 1 人，实际 %d", both.Total)
	}
}
