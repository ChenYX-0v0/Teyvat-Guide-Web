package service

// 功能反馈（2026-10-12）单测：归一化 / 长度边界 / 类别白名单 / 提交者不可伪造 / 多条可存
// 用例沿用既有 sqlite 套路（setupUsageTestDB ✓），反馈表在本文件里补迁 ✓。

import (
	"errors"
	"strings"
	"testing"

	"teyvat-guide/server/internal/model"
	"teyvat-guide/server/internal/pkg/database"
)

func setupFeedbackTestDB(t *testing.T) {
	t.Helper()
	setupUsageTestDB(t)
	// setupUsageTestDB 只迁了 User / 会话 / 预设 / 邀请码 ⇒ 本表要自己迁 ✓
	if err := database.DB().AutoMigrate(&model.Feedback{}); err != nil {
		t.Fatalf("迁移 feedbacks 失败：%v", err)
	}
}

// 口径 1：正常提交 ⇒ 落库，且内容去空白、类别走白名单
func TestSubmitFeedbackNormalizes(t *testing.T) {
	setupFeedbackTestDB(t)
	svc := NewFeedbackService()

	f, err := svc.Submit(10000001, "bug", "  这里有个小问题：侧栏卡片在手机上点不开  ", "")
	if err != nil {
		t.Fatalf("提交失败：%v", err)
	}
	if f.ID == 0 || f.CreatedAt.IsZero() {
		t.Fatalf("应回填 id / createdAt：%+v", f)
	}
	if strings.HasPrefix(f.Content, " ") || strings.HasSuffix(f.Content, " ") {
		t.Fatalf("内容应去前后空白，实际 %q", f.Content)
	}
	if f.UserID != 10000001 {
		t.Fatalf("user_id 不对：%d", f.UserID)
	}

	// 真的落库了（不是只返回了对象 ✓）
	var got model.Feedback
	if err := database.DB().First(&got, f.ID).Error; err != nil {
		t.Fatalf("查库失败：%v", err)
	}
	if got.Content != f.Content || got.Category != "bug" {
		t.Fatalf("落库内容不一致：%+v", got)
	}
}

// 口径 2：类别走白名单 —— 未知 / 空 / 乱填一律归到 other（**不报错** ✓）
func TestSubmitFeedbackCategoryWhitelist(t *testing.T) {
	setupFeedbackTestDB(t)
	svc := NewFeedbackService()

	for _, in := range []string{"", "  ", "unknown", "suggest' OR 1=1"} {
		f, err := svc.Submit(10000002, in, "类别不在白名单里时应当归到 other", "")
		if err != nil {
			t.Fatalf("类别 %q 不该报错：%v", in, err)
		}
		if f.Category != "other" {
			t.Fatalf("类别 %q 应归一化为 other，实际 %q", in, f.Category)
		}
	}

	f, err := svc.Submit(10000002, "suggest", "这类应该原样保留下来", "")
	if err != nil || f.Category != "suggest" {
		t.Fatalf("白名单内的类别应原样保留：%v %q", err, f.Category)
	}
}

// 口径 3：长度按**字符**算（中文不被按字节砍 ✓），边界值放行 / 越界报 400 语义
func TestSubmitFeedbackLengthByRunes(t *testing.T) {
	setupFeedbackTestDB(t)
	svc := NewFeedbackService()

	// 恰好 5 个字符 ✓ 放行
	if _, err := svc.Submit(10000003, "other", "五个字啊喂", ""); err != nil {
		t.Fatalf("5 个字应放行：%v", err)
	}
	// 4 个字 ✗
	if _, err := svc.Submit(10000003, "other", "四个字喂", ""); !errors.Is(err, ErrFeedbackInvalid) {
		t.Fatalf("4 个字应报 ErrFeedbackInvalid，实际 %v", err)
	}
	// 500 个字（中文）✓ 放行 —— 若按字节算，这里早就被砍掉了 ✗
	if _, err := svc.Submit(10000003, "other", strings.Repeat("好", 500), ""); err != nil {
		t.Fatalf("500 个中文字应放行：%v", err)
	}
	// 501 个字 ✗
	if _, err := svc.Submit(10000003, "other", strings.Repeat("好", 501), ""); !errors.Is(err, ErrFeedbackInvalid) {
		t.Fatalf("501 个字应报 ErrFeedbackInvalid，实际 %v", err)
	}
	// 纯空白 ✗（否则"提交成功"却什么都没写）
	if _, err := svc.Submit(10000003, "other", "      ", ""); !errors.Is(err, ErrFeedbackInvalid) {
		t.Fatalf("纯空白应报 ErrFeedbackInvalid，实际 %v", err)
	}
	// 联系方式超长 ✗
	if _, err := svc.Submit(10000003, "other", "内容本身没问题", strings.Repeat("a", 65)); !errors.Is(err, ErrFeedbackInvalid) {
		t.Fatalf("联系方式超长应报 ErrFeedbackInvalid，实际 %v", err)
	}
}

// 口径 4：提交者不可伪造 —— userID=0（中间件没注入）必须拒，且**不落库** ✓
func TestSubmitFeedbackRequiresUser(t *testing.T) {
	setupFeedbackTestDB(t)
	svc := NewFeedbackService()

	if _, err := svc.Submit(0, "other", "没有登录就不该能提反馈", ""); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("userID=0 应报 ErrUserNotFound，实际 %v", err)
	}
	var n int64
	if err := database.DB().Model(&model.Feedback{}).Count(&n).Error; err != nil {
		t.Fatalf("统计失败：%v", err)
	}
	if n != 0 {
		t.Fatalf("被拒的提交不该落库，实际 %d 条", n)
	}
}

// 口径 5：同一用户可以提多条（**没有唯一约束** ✓ —— 反馈不是"一人一条"✓）
func TestSubmitFeedbackAllowsMultiple(t *testing.T) {
	setupFeedbackTestDB(t)
	svc := NewFeedbackService()

	for i, body := range []string{"第一条：建议加个夜间模式", "第二条：图鉴页面在手机上有点卡"} {
		if _, err := svc.Submit(10000004, "suggest", body, ""); err != nil {
			t.Fatalf("第 %d 条提交失败：%v", i+1, err)
		}
	}
	var n int64
	if err := database.DB().Model(&model.Feedback{}).Where("user_id = ?", 10000004).Count(&n).Error; err != nil {
		t.Fatalf("统计失败：%v", err)
	}
	if n != 2 {
		t.Fatalf("同一用户应能存多条，实际 %d 条", n)
	}
}
