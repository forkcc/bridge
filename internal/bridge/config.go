package bridge

// Config Bridge 服务配置
type Config struct {
	Listen        string `yaml:"listen"`
	EdgeListen    string `yaml:"edge_listen"`
	ApihubURL     string `yaml:"apihub_url"`
	BridgeID      string `yaml:"bridge_id"`
	Token         string `yaml:"token"`
	Ip2RegionXDB  string `yaml:"ip2region_xdb"`  // ip2region v4 xdb 文件路径，用于根据 Edge 连接 IP 解析国家/地区
	Ip2RegionXDB6 string `yaml:"ip2region_xdb6"` // 可选，ip2region v6 xdb 路径
}
