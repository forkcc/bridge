package main

import (
	"log"
	"os"

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
	if err := db.AutoMigrate(&models.Node{}); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	for _, n := range []models.Node{
		{NodeType: models.NodeTypeClient, Token: "client-token-123", Status: "online"},
		{NodeType: models.NodeTypeEdge, Token: "edge-token-456", Status: "online"},
	} {
		var exist models.Node
		if err := db.Where("token = ?", n.Token).First(&exist).Error; err != nil {
			if err := db.Create(&n).Error; err != nil {
				log.Fatalf("create node: %v", err)
			}
			log.Printf("created node %s %s", n.NodeType, n.Token)
		}
	}
}
