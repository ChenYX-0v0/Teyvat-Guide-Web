package model

import "time"

// User 用户（账号密码登录）
type User struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Username     string    `gorm:"size:32;uniqueIndex;not null" json:"username"`
	PasswordHash string    `gorm:"size:128;not null" json:"-"`
	DisplayName  string    `gorm:"size:64" json:"displayName"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}
