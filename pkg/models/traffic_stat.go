package models

import "time"

// TrafficStat 流量统计（用于计费与报表）
type TrafficStat struct {
	ID         uint      `gorm:"primaryKey"`
	UserID     uint      `gorm:"column:user_id;index;not null"`
	EdgeID     string    `gorm:"column:edge_id;index;size:64;not null"`
	BytesIn    int64     `gorm:"column:bytes_in;not null"`
	BytesOut   int64     `gorm:"column:bytes_out;not null"`
	ReportedAt time.Time `gorm:"column:reported_at;not null"`
	CreatedAt  time.Time `gorm:"autoCreateTime"`
}

// TableName 指定表名
func (TrafficStat) TableName() string {
	return "traffic_stats"
}
