package models

import "time"

// ClientSession Client 会话
type ClientSession struct {
	ID        uint      `gorm:"primaryKey"`
	NodeID    uint      `gorm:"column:node_id;index;not null"`
	BridgeID  string    `gorm:"column:bridge_id;size:64;not null"`
	StartedAt time.Time `gorm:"column:started_at;not null"`
	EndedAt   *time.Time `gorm:"column:ended_at"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

// TableName 指定表名
func (ClientSession) TableName() string {
	return "client_sessions"
}
