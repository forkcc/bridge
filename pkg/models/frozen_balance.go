package models

import "time"

// FrozenBalance 冻结余额明细（流量计费待结算）
type FrozenBalance struct {
	ID          uint      `gorm:"primaryKey"`
	UserID      uint      `gorm:"column:user_id;index;not null"`
	Amount      int64     `gorm:"not null"`
	Reason      string    `gorm:"size:64;default:''"` // 如 traffic、subscription
	RefID       string    `gorm:"column:ref_id;size:64;default:''"`
	SettledAt   *time.Time `gorm:"column:settled_at"`
	CreatedAt   time.Time `gorm:"autoCreateTime"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime"`
}

// TableName 指定表名
func (FrozenBalance) TableName() string {
	return "frozen_balances"
}
