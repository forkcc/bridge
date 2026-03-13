package models

import "time"

// ClientEdgeBinding Client 与 Edge 的绑定关系
type ClientEdgeBinding struct {
	ID        uint      `gorm:"primaryKey"`
	ClientID  string    `gorm:"column:client_id;index;size:64;not null"`
	EdgeID    string    `gorm:"column:edge_id;index;size:64;not null"`
	Country   string    `gorm:"size:8;default:''"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

// TableName 指定表名
func (ClientEdgeBinding) TableName() string {
	return "client_edge_bindings"
}
