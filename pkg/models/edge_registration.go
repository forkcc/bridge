package models

import "time"

// EdgeRegistration Edge 与 apiHub 的注册关系
type EdgeRegistration struct {
	ID        uint      `gorm:"primaryKey"`
	EdgeID    string    `gorm:"column:edge_id;index;size:64;not null"`
	NodeID    uint      `gorm:"column:node_id;not null"` // 关联 nodes.id
	Addr      string    `gorm:"size:256;not null"`
	Country   string    `gorm:"size:8;default:''"`
	LastSeen  time.Time `gorm:"column:last_seen;not null"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

// TableName 指定表名
func (EdgeRegistration) TableName() string {
	return "edge_registrations"
}
