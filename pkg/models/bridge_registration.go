package models

import "time"

// BridgeRegistration Bridge 注册记录
type BridgeRegistration struct {
	ID        uint      `gorm:"primaryKey"`
	BridgeID  string    `gorm:"column:bridge_id;uniqueIndex;size:64;not null"`
	Addr      string    `gorm:"size:256;not null"` // 对外地址，如 host:port
	LastSeen  time.Time `gorm:"column:last_seen;not null"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

// TableName 指定表名
func (BridgeRegistration) TableName() string {
	return "bridge_registrations"
}
