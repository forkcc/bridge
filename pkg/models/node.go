package models

import "time"

// NodeType 节点类型
const (
	NodeTypeClient = "client"
	NodeTypeEdge   = "edge"
)

// Edge 节点状态（仅 nodes.node_type=edge 时使用）
const (
	NodeStatusOffline = "offline" // 下线
	NodeStatusIdle    = "idle"    // 空闲，可被 client 绑定
	NodeStatusBusy    = "busy"    // 工作，已被 client 绑定
)

// Node 统一节点表（Client / Edge），用 node_type 区分；nodes.token 指向 users.token，同 token 即同用户
// Edge 的 status：offline=下线，idle=空闲，busy=工作；Client 仅绑定 idle，绑定后变 busy，解绑后变 idle
type Node struct {
	ID        uint      `gorm:"primaryKey"`
	NodeType  string    `gorm:"column:node_type;size:16;not null;index"` // client | edge
	NodeID    string    `gorm:"column:node_id;size:64;default:''"`        // 启动时传入的 id，如 edge-1、client-1
	Token     string    `gorm:"index;size:128;not null"`                  // 与 users.token 一致，用于与 user 挂钩
	UserID    uint      `gorm:"column:user_id;index;not null;default:0"`  // 关联 users.id
	Country   string    `gorm:"size:8;default:''"`   // 国家代码，如 US、CN
	Status    string    `gorm:"size:16;default:online"` // client: online|offline；edge: offline|idle|busy
	LastSeen  *time.Time `gorm:"column:last_seen"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

// TableName 指定表名
func (Node) TableName() string {
	return "nodes"
}
