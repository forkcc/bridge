package client

// Config Client 服务配置
type Config struct {
	Socks5Listen string `yaml:"socks5_listen"`
	BridgeURL    string `yaml:"bridge_url"`
	Token        string `yaml:"token"`
	Country      string `yaml:"country"`
}
