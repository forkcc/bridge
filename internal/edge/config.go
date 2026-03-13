package edge

// Config Edge 服务配置
type Config struct {
	Listen         string `yaml:"listen"`
	BridgeTunnel   string `yaml:"bridge_tunnel"`   // Bridge 隧道地址，如 localhost:8081
	ApihubURL      string `yaml:"apihub_url"`
	Token          string `yaml:"token"`
	ID             string `yaml:"id"`
	MaxConnections int    `yaml:"max_connections"`
	MaxMemoryMB    int    `yaml:"max_memory_mb"`
	LowPowerMode   bool   `yaml:"low_power_mode"`
}
