package bridge

import (
	"log"
	"net/http"
)

// Server Bridge 服务
type Server struct {
	cfg *Config
}

// NewServer 创建 Bridge 服务
func NewServer(cfg *Config) *Server {
	return &Server{cfg: cfg}
}

// Run 启动：注册、心跳、HTTP 服务
func (s *Server) Run() error {
	if err := s.Register(); err != nil {
		log.Printf("bridge: register failed: %v", err)
	}
	go s.startHeartbeat()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("GET /api/edges", func(w http.ResponseWriter, r *http.Request) {
		s.proxyToApihub(w, r, "/api/edges")
	})
	mux.HandleFunc("POST /api/client/auth", func(w http.ResponseWriter, r *http.Request) {
		s.proxyToApihub(w, r, "/api/client/auth")
	})

	addr := s.cfg.Listen
	if addr == "" {
		addr = ":8080"
	}
	log.Printf("bridge: listening on %s", addr)
	return http.ListenAndServe(addr, mux)
}
