package models

import "time"

// User 用户账户，用于登录与计费
type User struct {
	ID             uint      `gorm:"primaryKey"`
	Username       string    `gorm:"uniqueIndex;size:64;not null"`
	PasswordHash   string    `gorm:"column:password_hash;size:255;not null"`
	Token          string    `gorm:"uniqueIndex;size:128;default:''"` // 节点 client/edge 用此 token 认证，nodes.token 指向此字段
	Balance        int64     `gorm:"default:0"`        // 余额，单位：KB（每 KB 扣 1）
	CreatedAt      time.Time `gorm:"autoCreateTime"`
	UpdatedAt      time.Time `gorm:"autoUpdateTime"`
}

// TableName 指定表名
func (User) TableName() string {
	return "users"
}
