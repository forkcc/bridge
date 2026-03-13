package models

import "time"

// User 用户账户，用于登录与计费
type User struct {
	ID             uint      `gorm:"primaryKey"`
	Username       string    `gorm:"uniqueIndex;size:64;not null"`
	PasswordHash   string    `gorm:"column:password_hash;size:255;not null"`
	Balance        int64     `gorm:"default:0"`        // 单位：分或最小货币单位
	FrozenBalance  int64     `gorm:"default:0"`        // 冻结余额
	Role           string    `gorm:"size:32;default:user"`
	CreatedAt      time.Time `gorm:"autoCreateTime"`
	UpdatedAt      time.Time `gorm:"autoUpdateTime"`
}

// TableName 指定表名
func (User) TableName() string {
	return "users"
}
