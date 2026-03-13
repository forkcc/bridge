package models

import "time"

// FundFlow 资金流水
type FundFlow struct {
	ID        uint      `gorm:"primaryKey"`
	UserID    uint      `gorm:"column:user_id;index;not null"`
	Amount    int64     `gorm:"not null"` // 正为收入/充值，负为扣费
	Balance   int64     `gorm:"not null"`  // 变动后余额快照
	Type      string    `gorm:"size:32;not null"` // 如 recharge、traffic、refund
	RefID     string    `gorm:"column:ref_id;size:64;default:''"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
}

// TableName 指定表名
func (FundFlow) TableName() string {
	return "fund_flows"
}
