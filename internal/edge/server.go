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

// Run 启动：注册、心跳；（隧道监听在 c24 实现）
func (s *Server) Run() error {
	if err := s.Register(); err != nil {
		log.Printf("edge: register failed: %v", err)
	}
	go s.startHeartbeat()
	// 隧道监听与 CONNECT 处理在 c24
	select {}
}
