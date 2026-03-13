package main

import (
	"log"
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"proxy-bridge/pkg/models"
)

func main() {
	dsn := os.Getenv("DSN")
	if dsn == "" {
		dsn = "host=localhost user=postgres password=postgres dbname=proxy_bridge port=5432 sslmode=disable"
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.Node{}); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	// 允许 nodes.token 重复（同用户多节点共用一个 token）：去掉旧唯一约束
	_ = db.Exec("DROP INDEX IF EXISTS idx_nodes_token").Error
	// 用户 token 与节点挂钩：users.token 存在，nodes.token 指向同一 token
	const e2eToken = "e2e-token"
	var user models.User
	if err := db.Where("token = ?", e2eToken).First(&user).Error; err != nil {
		if err := db.Where("username = ?", "e2e").First(&user).Error; err != nil {
			user = models.User{Username: "e2e", PasswordHash: "e2e", Token: e2eToken, Balance: 0}
			if err := db.Create(&user).Error; err != nil {
				log.Fatalf("create user: %v", err)
			}
			log.Printf("created user token=%s", e2eToken)
		} else {
			_ = db.Model(&user).Update("token", e2eToken)
			log.Printf("updated user token=%s", e2eToken)
		}
	} else if user.Token == "" {
		_ = db.Model(&user).Update("token", e2eToken)
	}
	now := time.Now()
	for _, n := range []struct {
		NodeType string
		NodeID   string
		Status   string
	}{
		{models.NodeTypeClient, "client-e2e", "online"},
		{models.NodeTypeEdge, "edge-1", models.NodeStatusIdle},
	} {
		var exist models.Node
		if err := db.Where("token = ? AND node_type = ?", e2eToken, n.NodeType).First(&exist).Error; err != nil {
			create := models.Node{NodeType: n.NodeType, Token: e2eToken, Status: n.Status, NodeID: n.NodeID, UserID: user.ID, LastSeen: &now}
			if err := db.Create(&create).Error; err != nil {
				log.Fatalf("create node: %v", err)
			}
			log.Printf("created node %s node_id=%s status=%s user_id=%d", n.NodeType, n.NodeID, n.Status, user.ID)
		} else {
			_ = db.Model(&exist).Updates(map[string]interface{}{"token": e2eToken, "status": n.Status, "node_id": n.NodeID, "user_id": user.ID, "last_seen": now})
		}
	}
}
