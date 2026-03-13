package models

import "time"

// MessageTracking RabbitMQ 消息跟踪（幂等、重试）
type MessageTracking struct {
	ID        uint      `gorm:"primaryKey"`
	MessageID string    `gorm:"column:message_id;uniqueIndex;size:128;not null"`
	Topic     string    `gorm:"size:64;not null"`
	Payload   string    `gorm:"type:text"`
	Status    string    `gorm:"size:16;default:pending"` // pending | done | failed
	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

// TableName 指定表名
func (MessageTracking) TableName() string {
	return "message_tracking"
}
