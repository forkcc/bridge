package models

import "time"

// FundFlow 资金流水
type FundFlow struct {
	ID        uint      `gorm:"primaryKey"`
	UserID    uint      `gorm:"column:user_id;index;not null"`
	Amount    int64     `gorm:"not null"` // 单位 KB：正为赚取，负为花费
	Balance   int64     `gorm:"not null"`  // 变动后余额快照（单位 KB）
	Type      string    `gorm:"size:32;not null"` // 如 recharge、traffic、refund
	CreatedAt time.Time `gorm:"autoCreateTime"`
}

// TableName 指定表名
func (FundFlow) TableName() string {
	return "fund_flows"
}
