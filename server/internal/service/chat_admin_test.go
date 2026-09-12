package service

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"teyvat-guide/server/internal/config"
	"teyvat-guide/server/internal/dto"
	"teyvat-guide/server/internal/model"
	"teyvat-guide/server/internal/pkg/database"
)

// 用 sqlite 文件库跑通后台会话查询：聚合 / 「只搜提问」口径 / 降级过滤 / 游标分页
func setupChatAdminService(t *testing.T) *ChatAdminService {
	t.Helper()

	dsn := filepath.Join(t.TempDir(), "chat_admin_test.db")
	if err := database.Init(config.DatabaseConfig{Driver: "sqlite", DSN: dsn}); err != nil {
		t.Fatalf("初始化 sqlite 失败：%v", err)
	}
	db := database.DB()
	if err := db.AutoMigrate(&model.User{}, &model.ChatSession{}, &model.ChatMessage{}); err != nil {
		t.Fatalf("迁移失败：%v", err)
	}
	// 测试收尾：先关连接，否则 t.TempDir 删不掉 sqlite 文件
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err != nil {
			return
		}
		if err := sqlDB.Close(); err != nil {
			t.Logf("关闭测试数据库失败：%v", err)
		}
	})

	base := time.Date(2026, 9, 12, 10, 0, 0, 0, time.Local)
	if err := db.Create(&model.User{
		ID: 1, Username: "traveler", DisplayName: "旅行者",
		GenshinUID: "197794747", Status: model.UserStatusActive,
	}).Error; err != nil {
		t.Fatalf("建用户失败：%v", err)
	}

	// s1：登录用户，注入过上下文，含一条降级回复
	sessions := []model.ChatSession{
		{
			ID: "s1", Title: "胡桃怎么玩", UserID: 1,
			HasPlayerContext: true, HasAssetReference: true,
			CreatedAt: base, UpdatedAt: base,
		},
		{
			ID: "s2", Title: "今天天气怎么样", UserID: 0, // 匿名
			CreatedAt: base.Add(time.Hour), UpdatedAt: base.Add(time.Hour),
		},
	}
	for _, s := range sessions {
		if err := db.Create(&s).Error; err != nil {
			t.Fatalf("建会话失败：%v", err)
		}
	}

	messages := []model.ChatMessage{
		{SessionID: "s1", Role: "user", Content: "胡桃怎么玩", CreatedAt: base},
		{SessionID: "s1", Role: "assistant", Content: "胡桃是火系主C哦", Tokens: 645, TokensInput: 600, TokensOutput: 45, TokensCached: 384, Model: "deepseek-flash", Profile: "chat", CreatedAt: base.Add(time.Second)},
		{SessionID: "s1", Role: "user", Content: "那圣遗物呢", CreatedAt: base.Add(2 * time.Second)},
		{SessionID: "s1", Role: "assistant", Content: aiDegradeReply, Degraded: true, CreatedAt: base.Add(3 * time.Second)},
		{SessionID: "s2", Role: "user", Content: "今天天气怎么样", CreatedAt: base.Add(time.Hour)},
		{SessionID: "s2", Role: "assistant", Content: "提瓦特今天晴", Tokens: 30, CreatedAt: base.Add(time.Hour + time.Second)},
	}
	for _, m := range messages {
		if err := db.Create(&m).Error; err != nil {
			t.Fatalf("建消息失败：%v", err)
		}
	}

	return NewChatAdminService()
}

func TestListSessionsBasic(t *testing.T) {
	svc := setupChatAdminService(t)

	data, err := svc.ListSessions(dto.AdminChatQuery{})
	if err != nil {
		t.Fatalf("查询失败：%v", err)
	}
	if data.Total != 2 || len(data.Items) != 2 {
		t.Fatalf("期望 2 个会话，实际 total=%d items=%d", data.Total, len(data.Items))
	}
	// 按活跃时间倒序：s2 更新
	if data.Items[0].SessionID != "s2" {
		t.Fatalf("排序不符（应按 updated_at 倒序）：%+v", data.Items[0])
	}

	// 登录会话：聚合与归属
	var s1 dto.AdminChatSessionItem
	for _, it := range data.Items {
		if it.SessionID == "s1" {
			s1 = it
		}
	}
	if s1.MessageCount != 4 {
		t.Fatalf("期望 4 条消息，实际 %d", s1.MessageCount)
	}
	if s1.Tokens != 645 {
		t.Fatalf("期望 tokens=645，实际 %d", s1.Tokens)
	}
	if s1.DegradedCount != 1 {
		t.Fatalf("期望 1 条降级，实际 %d", s1.DegradedCount)
	}
	if !s1.HasPlayerContext || !s1.HasAssetReference {
		t.Fatalf("上下文标记丢失：%+v", s1)
	}
	if s1.Username == nil || *s1.Username != "traveler" || s1.GenshinUID == nil || *s1.GenshinUID != "197794747" {
		t.Fatalf("归属信息不符：%+v", s1)
	}
	// 摘要应是「最近一条提问」，且截断（此处未超长）
	if s1.LastQuestion == nil || *s1.LastQuestion != "那圣遗物呢" {
		t.Fatalf("lastQuestion 不符：%+v", s1.LastQuestion)
	}

	// 匿名会话：userId=0，用户字段为 null
	for _, it := range data.Items {
		if it.SessionID == "s2" && (it.UserID != 0 || it.Username != nil || it.GenshinUID != nil) {
			t.Fatalf("匿名会话归属不符：%+v", it)
		}
	}
}

// 既定口径：关键词只搜提问，搜回答内容不返回结果（不是 bug）
func TestListSessionsKeywordOnlyMatchesQuestions(t *testing.T) {
	svc := setupChatAdminService(t)

	hit, err := svc.ListSessions(dto.AdminChatQuery{Keyword: "胡桃"})
	if err != nil {
		t.Fatalf("查询失败：%v", err)
	}
	if hit.Total != 1 || hit.Items[0].SessionID != "s1" {
		t.Fatalf("期望命中 s1：%+v", hit.Items)
	}

	// 「火系主C」只出现在 assistant 消息里 → 不应命中
	miss, err := svc.ListSessions(dto.AdminChatQuery{Keyword: "火系主C"})
	if err != nil {
		t.Fatalf("查询失败：%v", err)
	}
	if miss.Total != 0 {
		t.Fatalf("关键词不应匹配回答内容，实际命中 %d 个会话", miss.Total)
	}
}

func TestListSessionsFilters(t *testing.T) {
	svc := setupChatAdminService(t)

	degraded, err := svc.ListSessions(dto.AdminChatQuery{OnlyDegraded: true})
	if err != nil {
		t.Fatalf("查询失败：%v", err)
	}
	if degraded.Total != 1 || degraded.Items[0].SessionID != "s1" {
		t.Fatalf("降级过滤不符：%+v", degraded.Items)
	}

	byUser, err := svc.ListSessions(dto.AdminChatQuery{UserID: 1})
	if err != nil {
		t.Fatalf("查询失败：%v", err)
	}
	if byUser.Total != 1 || byUser.Items[0].SessionID != "s1" {
		t.Fatalf("按用户过滤不符：%+v", byUser.Items)
	}

	byUID, err := svc.ListSessions(dto.AdminChatQuery{UID: "197794747"})
	if err != nil {
		t.Fatalf("查询失败：%v", err)
	}
	if byUID.Total != 1 || byUID.Items[0].SessionID != "s1" {
		t.Fatalf("按游戏 UID 过滤不符：%+v", byUID.Items)
	}

	// 时间范围：只覆盖 s1（s1=10:00，s2=11:00）
	from := time.Date(2026, 9, 12, 9, 0, 0, 0, time.Local).Format(time.RFC3339)
	to := time.Date(2026, 9, 12, 10, 30, 0, 0, time.Local).Format(time.RFC3339)
	byTime, err := svc.ListSessions(dto.AdminChatQuery{From: from, To: to})
	if err != nil {
		t.Fatalf("查询失败：%v", err)
	}
	if byTime.Total != 1 || byTime.Items[0].SessionID != "s1" {
		t.Fatalf("按时间过滤不符：%+v", byTime.Items)
	}

	// 参数错误（非 RFC3339）应返回可识别的 400 错误
	if _, err := svc.ListSessions(dto.AdminChatQuery{From: "2026-09-12"}); !errors.Is(err, ErrChatQueryInvalid) {
		t.Fatalf("期望 ErrChatQueryInvalid，实际 %v", err)
	}
}

func TestMessagesCursor(t *testing.T) {
	svc := setupChatAdminService(t)

	all, err := svc.Messages("s1", 0, 0)
	if err != nil {
		t.Fatalf("查询失败：%v", err)
	}
	if len(all.Messages) != 4 || all.HasMore {
		t.Fatalf("期望 4 条且无更多：len=%d hasMore=%v", len(all.Messages), all.HasMore)
	}
	if all.Messages[0].Role != "user" || all.Messages[1].Role != "assistant" {
		t.Fatalf("消息顺序不符：%+v", all.Messages)
	}
	// 观测字段要能读出来
	if all.Messages[1].Model != "deepseek-flash" || all.Messages[1].Profile != "chat" || all.Messages[1].TokensCached != 384 {
		t.Fatalf("用量字段不符：%+v", all.Messages[1])
	}
	if !all.Messages[3].Degraded {
		t.Fatalf("降级标记丢失：%+v", all.Messages[3])
	}

	// 游标：取第二条之后的消息
	rest, err := svc.Messages("s1", all.Messages[1].ID, 0)
	if err != nil {
		t.Fatalf("查询失败：%v", err)
	}
	if len(rest.Messages) != 2 || rest.Messages[0].ID != all.Messages[2].ID {
		t.Fatalf("游标分页不符：%+v", rest.Messages)
	}

	// 会话不存在 → 404 语义
	if _, err := svc.Messages("not-exist", 0, 0); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("期望 ErrSessionNotFound，实际 %v", err)
	}
	// 空 sessionId → 参数错误
	if _, err := svc.Messages("  ", 0, 0); !errors.Is(err, ErrChatQueryInvalid) {
		t.Fatalf("期望 ErrChatQueryInvalid，实际 %v", err)
	}
}
