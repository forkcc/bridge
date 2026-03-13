package apihub

// Config apiHub 服务配置
type Config struct {
	Listen   string         `yaml:"listen"`
	Database DatabaseConfig `yaml:"database"`
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	DBName   string `yaml:"dbname"`
	SSLMode  string `yaml:"sslmode"`
}
