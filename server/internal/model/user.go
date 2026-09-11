package model

import "time"

// User 用户（账号密码登录）
type User struct {
	ID            uint       `gorm:"primaryKey" json:"id"`
	Username      string     `gorm:"size:32;uniqueIndex;not null" json:"username"`
	PasswordHash  string     `gorm:"size:128;not null" json:"-"`
	DisplayName   string     `gorm:"size:64" json:"displayName"`
	GenshinUID    string     `gorm:"size:16" json:"genshinUid"`                     // 绑定的游戏 UID（一对一）
	GenshinServer string     `gorm:"size:16" json:"genshinServer"`                  // 绑定区服 cn_gf01 / cn_qd01
	Status        string     `gorm:"size:16;not null;default:active" json:"status"` // active | banned
	LastLoginAt   *time.Time `json:"lastLoginAt"`                                   // 最近登录时间（可空）
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}

// 用户状态枚举
const (
	UserStatusActive = "active"
	UserStatusBanned = "banned"
)

// IsBanned 是否已封禁
func (u *User) IsBanned() bool { return u.Status == UserStatusBanned }
