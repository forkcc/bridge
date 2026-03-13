package client

import (
	"log"
	"net"
	"os"
	"sync"

	"github.com/mattn/go-isatty"
)

// Server Client SOCKS5 服务
type Server struct {
	cfg         *Config
	tunnelState *tunnelState
	tsOnce      sync.Once
	userID      uint // 认证后获取的 user_id，用于流量计费
}

// NewServer 创建 Client 服务
func NewServer(cfg *Config) *Server {
	return &Server{cfg: cfg}
}

func (s *Server) getTunnelState() *tunnelState {
	s.tsOnce.Do(func() {
		s.tunnelState = &tunnelState{}
	})
	return s.tunnelState
}

// Run 启动 SOCKS5 监听与心跳，并运行 TUI（动态刷新 edge 连接状态与余额）；TUI 退出后返回
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
	go s.startHeartbeat()
	// 启动后自动尝试绑定 edge，无需等首次请求或 TUI 刷新
	go func() { _, _, _ = s.ensureTunnel() }()
	if !isatty.IsTerminal(os.Stdout.Fd()) {
		// 非 TTY（如脚本/管道）不启 TUI，仅阻塞 SOCKS5
		log.Printf("client: SOCKS5 on %s (no TUI)", addr)
		for {
			conn, err := ln.Accept()
			if err != nil {
				return err
			}
			go s.serveSOCKS5(conn)
		}
	}
	go func() {
		log.Printf("client: SOCKS5 on %s", addr)
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go s.serveSOCKS5(conn)
		}
	}()
	return s.RunTUI()
}
