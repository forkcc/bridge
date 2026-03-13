package client

// Config Client 服务配置
type Config struct {
	Socks5Listen  string `yaml:"socks5_listen"`
	BridgeURL     string `yaml:"bridge_url"`
	BridgeTunnel  string `yaml:"bridge_tunnel"` // Bridge 隧道地址，如 localhost:8081
	Token         string `yaml:"token"`
	ID            string `yaml:"id"` // 启动时传入的 id，如 client-1，会写入 nodes.node_id
	Country       string `yaml:"country"`
}
