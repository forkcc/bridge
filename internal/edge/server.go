package edge

import (
	"log"
	"time"
)

// Server Edge 服务
type Server struct {
	cfg *Config
}

// NewServer 创建 Edge 服务
func NewServer(cfg *Config) *Server {
	return &Server{cfg: cfg}
}

// Run 启动：注册、心跳、连 Bridge 隧道并处理 CONNECT 流；隧道断开时自动重连，不退出进程
func (s *Server) Run() error {
	if err := s.Register(); err != nil {
		log.Printf("edge: register failed: %v", err)
	}
	go s.startHeartbeat()
	for {
		s.runTunnel()
		time.Sleep(5 * time.Second)
	}
}
