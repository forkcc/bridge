package apihub

// Config apiHub 服务配置
type Config struct {
	Listen   string       `yaml:"listen"`
	Database DatabaseConfig `yaml:"database"`
	RabbitMQ RabbitMQConfig `yaml:"rabbitmq"`
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

// RabbitMQConfig RabbitMQ 配置
type RabbitMQConfig struct {
	URL         string `yaml:"url"`
	QueueTraffic string `yaml:"queue_traffic"`
	QueueSettle  string `yaml:"queue_settle"`
}
