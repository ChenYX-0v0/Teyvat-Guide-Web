package service

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"teyvat-guide/server/internal/config"
	"teyvat-guide/server/internal/dto"
	"teyvat-guide/server/internal/model"
	"teyvat-guide/server/internal/pkg/database"
	"teyvat-guide/server/internal/repository"
)

// 用 sqlite 文件库跑通 C 端会话管理：
// 归属隔离（只返回/只允许本人）、最新一屏 + 向上游标、重命名、写入侧的会话归属校验
func setupChatSessionService(t *testing.T) *ChatSessionService {
	t.Helper()

	dsn := filepath.Join(t.TempDir(), "chat_session_test.db")
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
	sessions := []model.ChatSession{
		{ID: "s1", Title: "胡桃怎么玩", UserID: 1, CreatedAt: base, UpdatedAt: base},
		{ID: "s3", Title: "配队", UserID: 1, CreatedAt: base.Add(time.Hour), UpdatedAt: base.Add(time.Hour)},
		{ID: "s2", Title: "别人的会话", UserID: 2, CreatedAt: base, UpdatedAt: base},
		{ID: "s0", Title: "匿名会话", UserID: 0, CreatedAt: base, UpdatedAt: base},
	}
	for _, s := range sessions {
		if err := db.Create(&s).Error; err != nil {
			t.Fatalf("建会话失败：%v", err)
		}
	}

	// s1 内 5 条消息（第 1~5 条，每秒一条），用于游标分页
	for i := 0; i < 5; i++ {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		m := model.ChatMessage{
			SessionID: "s1",
			Role:      role,
			Content:   fmt.Sprintf("第 %d 条", i+1),
			CreatedAt: base.Add(time.Duration(i) * time.Second),
		}
		if err := db.Create(&m).Error; err != nil {
			t.Fatalf("建消息失败：%v", err)
		}
	}

	return NewChatSessionService()
}

func TestChatSessionListOnlyOwn(t *testing.T) {
	svc := setupChatSessionService(t)

	data, err := svc.List(1, 1, 20)
	if err != nil {
		t.Fatalf("查询失败：%v", err)
	}
	if data.Total != 2 || len(data.Items) != 2 {
		t.Fatalf("期望 2 个会话，实际 total=%d items=%d", data.Total, len(data.Items))
	}
	// 活跃时间倒序：s3（11:00）在 s1（10:00）之前
	if data.Items[0].SessionID != "s3" || data.Items[1].SessionID != "s1" {
		t.Fatalf("排序不符（应按 updated_at 倒序）：%+v", data.Items)
	}
	if data.Items[1].MessageCount != 5 {
		t.Fatalf("期望 s1 有 5 条消息，实际 %d", data.Items[1].MessageCount)
	}
	// 别人的会话与匿名会话都不应出现
	for _, it := range data.Items {
		if it.SessionID == "s2" || it.SessionID == "s0" {
			t.Fatalf("越权返回了会话：%+v", it)
		}
	}
}

// 未登录（userID = 0）不能拿到任何人的会话列表
func TestChatSessionListRequiresLogin(t *testing.T) {
	svc := setupChatSessionService(t)

	if _, err := svc.List(0, 1, 20); !errors.Is(err, ErrChatQueryInvalid) {
		t.Fatalf("期望 ErrChatQueryInvalid，实际 %v", err)
	}
}

// 非本人 / 匿名 / 不存在，一律 404 语义（不区分，避免探测会话是否存在）
func TestChatSessionMessagesOwnership(t *testing.T) {
	svc := setupChatSessionService(t)

	cases := []struct {
		name      string
		userID    uint
		sessionID string
	}{
		{name: "别人的会话", userID: 1, sessionID: "s2"},
		{name: "匿名会话", userID: 1, sessionID: "s0"},
		{name: "不存在的会话", userID: 1, sessionID: "not-exist"},
		{name: "匿名调用者", userID: 0, sessionID: "s0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := svc.Messages(tc.userID, tc.sessionID, 0, 0); !errors.Is(err, ErrSessionNotFound) {
				t.Fatalf("期望 ErrSessionNotFound，实际 %v", err)
			}
		})
	}

	if _, err := svc.Messages(1, "  ", 0, 0); !errors.Is(err, ErrChatQueryInvalid) {
		t.Fatalf("空 sessionId 期望 ErrChatQueryInvalid，实际 %v", err)
	}
}

// 打开会话先拿"最新一屏"，再用 beforeId 往前翻更早的
func TestChatSessionMessagesLatestAndCursor(t *testing.T) {
	svc := setupChatSessionService(t)

	latest, err := svc.Messages(1, "s1", 0, 3)
	if err != nil {
		t.Fatalf("查询失败：%v", err)
	}
	if len(latest.Messages) != 3 || !latest.HasMore {
		t.Fatalf("期望最近 3 条且还有更早的：len=%d hasMore=%v", len(latest.Messages), latest.HasMore)
	}
	// 按 id 升序 → 第 3、4、5 条
	if latest.Messages[0].Content != "第 3 条" || latest.Messages[2].Content != "第 5 条" {
		t.Fatalf("最新一屏内容不符：%+v", latest.Messages)
	}

	older, err := svc.Messages(1, "s1", latest.Messages[0].ID, 3)
	if err != nil {
		t.Fatalf("查询失败：%v", err)
	}
	if len(older.Messages) != 2 || older.HasMore {
		t.Fatalf("期望只剩 2 条且无更多：len=%d hasMore=%v", len(older.Messages), older.HasMore)
	}
	if older.Messages[0].Content != "第 1 条" || older.Messages[1].Content != "第 2 条" {
		t.Fatalf("更早的一屏内容不符：%+v", older.Messages)
	}

	// limit 缺省（0）→ 用默认上限，一次取全
	all, err := svc.Messages(1, "s1", 0, 0)
	if err != nil {
		t.Fatalf("查询失败：%v", err)
	}
	if len(all.Messages) != 5 || all.HasMore {
		t.Fatalf("期望 5 条且无更多：len=%d hasMore=%v", len(all.Messages), all.HasMore)
	}
}

func TestChatSessionRename(t *testing.T) {
	svc := setupChatSessionService(t)
	repo := repository.NewChatRepository()

	res, err := svc.Rename(1, "s1", "  新手开荒  ")
	if err != nil {
		t.Fatalf("重命名失败：%v", err)
	}
	if res.SessionID != "s1" || res.Title != "新手开荒" {
		t.Fatalf("期望返回去空白后的标题，实际 %+v", res)
	}

	session, err := repo.FindSession("s1")
	if err != nil || session == nil {
		t.Fatalf("读取会话失败：err=%v session=%v", err, session)
	}
	if session.Title != "新手开荒" {
		t.Fatalf("标题未落库：%q", session.Title)
	}
	// 重命名不该刷新活跃时间，否则列表顺序会因改名跳动
	want := time.Date(2026, 9, 12, 10, 0, 0, 0, time.Local)
	if !session.UpdatedAt.Equal(want) {
		t.Fatalf("重命名不应改变 updated_at，期望 %v 实际 %v", want, session.UpdatedAt)
	}

	// 非本人 / 不存在 / 匿名会话：一律 404 语义，且不改动数据
	for _, tc := range []struct {
		name      string
		userID    uint
		sessionID string
	}{
		{name: "别人的会话", userID: 2, sessionID: "s1"},
		{name: "不存在的会话", userID: 1, sessionID: "not-exist"},
		{name: "匿名会话", userID: 1, sessionID: "s0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := svc.Rename(tc.userID, tc.sessionID, "改名"); !errors.Is(err, ErrSessionNotFound) {
				t.Fatalf("期望 ErrSessionNotFound，实际 %v", err)
			}
		})
	}

	// 参数错误：空标题 / 超长
	if _, err := svc.Rename(1, "s1", "   "); !errors.Is(err, ErrChatQueryInvalid) {
		t.Fatalf("空标题期望 ErrChatQueryInvalid，实际 %v", err)
	}
	if _, err := svc.Rename(1, "s1", strings.Repeat("字", sessionTitleMaxRunes+1)); !errors.Is(err, ErrChatQueryInvalid) {
		t.Fatalf("超长标题期望 ErrChatQueryInvalid，实际 %v", err)
	}

	// 上面这些失败都不应改动数据
	session, err = repo.FindSession("s1")
	if err != nil || session == nil {
		t.Fatalf("读取会话失败：err=%v session=%v", err, session)
	}
	if session.Title != "新手开荒" {
		t.Fatalf("失败的请求改动了标题：%q", session.Title)
	}
}

// 写入侧：拿别人的 sessionId 提问不能续聊那条会话，而是新开一个属于调用者的会话
func TestChatAskRejectsForeignSession(t *testing.T) {
	setupChatSessionService(t)
	svc := NewChatService(config.AIConfig{Provider: "mock"}, nil)
	ctx := context.Background()

	sessionID, reply, _, _, err := svc.Ask(ctx, 2, "s1", "你好")
	if err != nil {
		t.Fatalf("Ask 失败：%v", err)
	}
	if sessionID == "s1" {
		t.Fatalf("不应复用别人的会话：%s", sessionID)
	}
	if reply == "" {
		t.Fatal("期望有回复内容")
	}

	repo := repository.NewChatRepository()
	created, err := repo.FindSession(sessionID)
	if err != nil || created == nil {
		t.Fatalf("新会话未落库：err=%v session=%v", err, created)
	}
	if created.UserID != 2 {
		t.Fatalf("新会话应归属调用者，实际 user_id=%d", created.UserID)
	}

	// 本人的会话照常续聊
	again, _, _, _, err := svc.Ask(ctx, 1, "s1", "继续")
	if err != nil {
		t.Fatalf("Ask 失败：%v", err)
	}
	if again != "s1" {
		t.Fatalf("本人的会话应复用，实际 %s", again)
	}
}

// 软删除：C 端不可见（列表 / 明细 / 改名 / 续聊都当"不存在"），但数据仍在、后台可查
func TestChatSessionSoftDelete(t *testing.T) {
	svc := setupChatSessionService(t)
	admin := NewChatAdminService()
	repo := repository.NewChatRepository()

	if _, err := svc.Delete(1, "s1"); err != nil {
		t.Fatalf("删除失败：%v", err)
	}

	// C 端
	list, err := svc.List(1, 1, 20)
	if err != nil {
		t.Fatalf("查询失败：%v", err)
	}
	if list.Total != 1 || len(list.Items) != 1 || list.Items[0].SessionID != "s3" {
		t.Fatalf("已删除的会话仍出现在列表：%+v", list.Items)
	}
	if _, err := svc.Messages(1, "s1", 0, 0); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("已删除会话的明细应 404，实际 %v", err)
	}
	if _, err := svc.Rename(1, "s1", "改名"); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("已删除会话不应可改名，实际 %v", err)
	}
	if _, err := svc.Delete(1, "s1"); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("重复删除应 404，实际 %v", err)
	}
	if _, err := svc.Delete(2, "s3"); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("不应能删别人的会话，实际 %v", err)
	}

	// 软删而非物理删：数据在、删除时间已落库
	session, err := repo.FindSession("s1")
	if err != nil || session == nil {
		t.Fatalf("会话被物理删除了：err=%v session=%v", err, session)
	}
	if session.DeletedAt == nil {
		t.Fatal("deleted_at 未落库")
	}
	deletedAt := *session.DeletedAt

	// B 端：默认仍可见并带删除时间；只看已删除时只剩它；明细照常可读
	adminList, err := admin.ListSessions(dto.AdminChatQuery{})
	if err != nil {
		t.Fatalf("后台查询失败：%v", err)
	}
	var found *dto.AdminChatSessionItem
	for i := range adminList.Items {
		if adminList.Items[i].SessionID == "s1" {
			found = &adminList.Items[i]
		}
	}
	if found == nil || found.DeletedAt == nil {
		t.Fatalf("后台应看到已删除会话并带 deletedAt：%+v", adminList.Items)
	}
	onlyDeleted, err := admin.ListSessions(dto.AdminChatQuery{OnlyDeleted: true})
	if err != nil {
		t.Fatalf("后台查询失败：%v", err)
	}
	if onlyDeleted.Total != 1 || len(onlyDeleted.Items) != 1 || onlyDeleted.Items[0].SessionID != "s1" {
		t.Fatalf("onlyDeleted 过滤不符：%+v", onlyDeleted.Items)
	}
	adminMsgs, err := admin.Messages("s1", 0, 0)
	if err != nil {
		t.Fatalf("后台读明细失败：%v", err)
	}
	if len(adminMsgs.Messages) != 5 {
		t.Fatalf("后台仍应读到 5 条消息，实际 %d", len(adminMsgs.Messages))
	}

	// 写入侧：已删除的会话不能续聊（新开会话），且删除标记不被改写
	chat := NewChatService(config.AIConfig{Provider: "mock"}, nil)
	again, _, _, _, err := chat.Ask(context.Background(), 1, "s1", "继续")
	if err != nil {
		t.Fatalf("Ask 失败：%v", err)
	}
	if again == "s1" {
		t.Fatal("已删除的会话不应可续聊")
	}
	session, err = repo.FindSession("s1")
	if err != nil || session == nil || session.DeletedAt == nil || !session.DeletedAt.Equal(deletedAt) {
		t.Fatalf("删除标记被后续动作改写：%+v", session)
	}
}
