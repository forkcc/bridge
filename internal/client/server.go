package client

import (
	"log"
	"net"
)

// Server Client SOCKS5 服务
type Server struct {
	cfg         *Config
	tunnelState *tunnelState
}

// NewServer 创建 Client 服务
func NewServer(cfg *Config) *Server {
	return &Server{cfg: cfg}
}

// Run 启动 SOCKS5 监听（c26 直连；c27/c28 改为经 Bridge 隧道）
func (s *Server) Run() error {
	addr := s.cfg.Socks5Listen
	if addr == "" {
		addr = ":1080"
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	defer ln.Close()
	log.Printf("client: SOCKS5 on %s", addr)
	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		go s.serveSOCKS5(conn)
	}
}
