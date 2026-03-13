package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Load 从 YAML 文件加载配置到 dest，dest 应为结构体指针
func Load(path string, dest interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, dest)
}
