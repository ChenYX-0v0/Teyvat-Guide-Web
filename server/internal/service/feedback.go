package service

// 功能反馈（2026-10-12）：用户从侧栏用户卡片的「功能反馈」提交的意见 ⇒ 落库一张专表 ✓
// 契约：`POST /api/feedback`（需登录），见 handler/feedback.go 与 docs/web/04 §3 ✓

import (
	"errors"
	"fmt"
	"strings"

	"teyvat-guide/server/internal/model"
	"teyvat-guide/server/internal/repository"
)

// 反馈内容的长度口径（**按字符计** ✓ —— 与昵称 displayNameMaxRunes 同一口径）：
// 前端提示里的数字必须与这里一致（同一组真源 ✓，见 web/src/i18n 的 feedback.rule ✓）。
const (
	feedbackMinRunes        = 5
	feedbackMaxRunes        = 500
	feedbackContactMaxRunes = 64
)

// ErrFeedbackInvalid 反馈内容不合规（Handler 映射 **400** ✓，不是 401 ✗）
var ErrFeedbackInvalid = errors.New("反馈内容不合规")

// feedbackCategories 允许的类别 —— **白名单** ✓：
// 前端传上来的字符串不校验就直接落库，等于把库交给调用方随意填 ✗（将来做统计也没法聚合 ✓）。
var feedbackCategories = map[string]bool{
	"bug": true, "suggest": true, "experience": true, "other": true,
}

type FeedbackService struct {
	repo *repository.FeedbackRepository
}

func NewFeedbackService() *FeedbackService {
	return &FeedbackService{repo: repository.NewFeedbackRepository()}
}

// Submit 提交一条功能反馈。
//
// 口径：
//   - `userID` **只由中间件注入** ✓ —— 绝不接受前端传的 userId ✗（否则能伪造他人反馈 ✓）；
//   - 内容 `TrimSpace` ✓：纯空白不算内容 ✓（否则"提交成功"却什么都没写 ✗）；
//   - 长度**按字符**（rune）✓ 5~500 ✓ —— 按字节算的话中文会被凭空中砍 3 倍 ✗；
//   - 类别走白名单 ✓：未知 / 空 / 乱填 ⇒ 归一化为 `other` ✓（这里**不报错** ✗ —— 类别是"我们自己给的选项"，
//     用户不可能填错；报错只会把一次正常提交挡掉 ✓）；
//   - 联系方式选填 ✓，限长 ✓。
func (s *FeedbackService) Submit(userID uint, category, content, contact string) (*model.Feedback, error) {
	if userID == 0 {
		return nil, ErrUserNotFound
	}

	body := strings.TrimSpace(content)
	n := len([]rune(body))
	if n < feedbackMinRunes {
		return nil, fmt.Errorf("%w：内容至少 %d 个字", ErrFeedbackInvalid, feedbackMinRunes)
	}
	if n > feedbackMaxRunes {
		return nil, fmt.Errorf("%w：内容最多 %d 个字", ErrFeedbackInvalid, feedbackMaxRunes)
	}

	cat := strings.TrimSpace(category)
	if !feedbackCategories[cat] {
		cat = "other"
	}

	ct := strings.TrimSpace(contact)
	if len([]rune(ct)) > feedbackContactMaxRunes {
		return nil, fmt.Errorf("%w：联系方式最多 %d 个字", ErrFeedbackInvalid, feedbackContactMaxRunes)
	}

	f := &model.Feedback{
		UserID:   userID,
		Category: cat,
		Content:  body,
		Contact:  ct,
	}
	if err := s.repo.Create(f); err != nil {
		return nil, err
	}
	return f, nil
}
