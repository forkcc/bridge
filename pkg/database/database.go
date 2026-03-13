package database

import (
	"fmt"
	"log"
	"sync"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"proxy-bridge/pkg/models"
)

var (
	db   *gorm.DB
	once sync.Once
)

// Config 数据库连接配置
type Config struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
}

// DSN 生成 PostgreSQL DSN
func (c *Config) DSN() string {
	if c.SSLMode == "" {
		c.SSLMode = "disable"
	}
	if c.Port <= 0 {
		c.Port = 5432
	}
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.DBName, c.SSLMode)
}

// Open 打开数据库连接并执行 AutoMigrate（单例）
func Open(cfg Config) (*gorm.DB, error) {
	var err error
	once.Do(func() {
		db, err = gorm.Open(postgres.Open(cfg.DSN()), &gorm.Config{})
		if err != nil {
			return
		}
		if err = autoMigrate(db); err != nil {
			log.Printf("database: AutoMigrate error: %v", err)
			return
		}
	})
	return db, err
}

// Get 返回已初始化的 DB（需先调用 Open）
func Get() *gorm.DB {
	return db
}

// autoMigrate 自动迁移所有模型
func autoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.User{},
		&models.Node{},
		&models.BridgeRegistration{},
		&models.EdgeRegistration{},
		&models.ClientSession{},
		&models.ClientEdgeBinding{},
		&models.FrozenBalance{},
		&models.FundFlow{},
		&models.TrafficStat{},
		&models.MessageTracking{},
		&models.CountryPreference{},
	)
}
