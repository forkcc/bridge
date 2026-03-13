package edge

import (
	"log"
)

// Server Edge 服务
type Server struct {
	cfg *Config
}

// NewServer 创建 Edge 服务
func NewServer(cfg *Config) *Server {
	return &Server{cfg: cfg}
}

// Run 启动：注册、心跳、连 Bridge 隧道并处理 CONNECT 流
func (s *Server) Run() error {
	if err := s.Register(); err != nil {
		log.Printf("edge: register failed: %v", err)
	}
	go s.startHeartbeat()
	s.runTunnel()
	return nil
}
