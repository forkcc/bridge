package models

import "time"

// NodeType 节点类型
const (
	NodeTypeClient = "client"
	NodeTypeEdge   = "edge"
)

// Node 统一节点表（Client / Edge），用 node_type 区分
type Node struct {
	ID        uint      `gorm:"primaryKey"`
	NodeType  string    `gorm:"column:node_type;size:16;not null;index"` // client | edge
	Token     string    `gorm:"uniqueIndex;size:128;not null"`
	Country   string    `gorm:"size:8;default:''"`   // 国家代码，如 US、CN
	Region    string    `gorm:"size:64;default:''"`
	Balance   int64     `gorm:"default:0"`
	Status    string    `gorm:"size:16;default:online"` // online | offline
	LastSeen  *time.Time `gorm:"column:last_seen"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

// TableName 指定表名
func (Node) TableName() string {
	return "nodes"
}
