package bridge

// Config Bridge 服务配置
type Config struct {
	Listen     string `yaml:"listen"`
	EdgeListen string `yaml:"edge_listen"`
	ApihubURL  string `yaml:"apihub_url"`
	BridgeID   string `yaml:"bridge_id"`
	Token      string `yaml:"token"`
}
