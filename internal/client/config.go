package client

// Config Client 服务配置
type Config struct {
	Socks5Listen  string `yaml:"socks5_listen"`
	BridgeURL     string `yaml:"bridge_url"`
	BridgeTunnel  string `yaml:"bridge_tunnel"` // Bridge 隧道地址，如 localhost:8081
	Token         string `yaml:"token"`
	Country       string `yaml:"country"`
}
