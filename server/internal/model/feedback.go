package model

import "time"

// Feedback 功能反馈（2026-10-12 新增）—— 侧栏用户卡片「功能反馈」提交的用户意见，落一张**专表**。
//
// 为什么单开一张表（而不是塞进既有的表）✗：
//   - `usage_events` 是**埋点**（事件名 + 结构化属性 ✓）⇒ 装自由文本会把这个口径搞糊 ✗；
//   - `chat_messages` 是**对话**（有会话归属、会被软删 ✗）⇒ 反馈不能跟着对话一起消失 ✓；
//   - 反馈是"用户主动提的意见" ✓：要能长期留存、按用户回溯、将来还能标记处理状态 ✓。
//
// 口径（刻意只存这些字段）：
//   - `user_id`：**必须有** ✓ —— 否则一堆匿名文本无法回溯、也无法回复 ✓（由中间件注入，不收前端传值 ✗）；
//   - `content`：反馈本体（自由文本 ✓，服务端按**字符**限长，见 service/feedback.go ✓）；
//   - `category`：少打字用（固定白名单 ✓，未知一律落 `other` ✓）；
//   - `contact`：选填 ✓ —— 手机号是**登录凭据**，不该逼用户再填一遍 ✗，但留个口子方便回复 ✓；
//   - ⚠️ **不存 IP、不存 UA** ✗：有 user_id 就能定位到人，再存这些属过度收集 ✓。
type Feedback struct {
	ID uint `gorm:"primaryKey" json:"id"`
	// UserID 提交者（中间件注入 ✓；建索引：后台要按用户查 ✓）
	UserID uint `gorm:"index;not null" json:"userId"`
	// Category 类别（白名单：bug / suggest / experience / other ✓）
	Category string `gorm:"size:32" json:"category,omitempty"`
	// Content 反馈内容（自由文本；限长在服务层，见 feedbackMaxRunes ✓）
	Content string `gorm:"size:2000;not null" json:"content"`
	// Contact 联系方式（选填，方便回复 ✓）
	Contact   string    `gorm:"size:128" json:"contact,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
