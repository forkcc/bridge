package models

import "time"

// CountryPreference 用户国家选择偏好（Client 选 Edge 时用）
type CountryPreference struct {
	ID        uint      `gorm:"primaryKey"`
	UserID    uint      `gorm:"column:user_id;index;not null"`
	Country   string    `gorm:"size:8;not null"`
	Priority  int       `gorm:"default:0"` // 优先级，越大越优先
	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

// TableName 指定表名
func (CountryPreference) TableName() string {
	return "country_preferences"
}
