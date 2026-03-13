package bridge

import (
	"log"
	"net/http"

	"github.com/lionsoul2014/ip2region/binding/golang/service"
)

// Server Bridge 服务
type Server struct {
	cfg *Config
}

// NewServer 创建 Bridge 服务
func NewServer(cfg *Config) *Server {
	return &Server{cfg: cfg}
}

// Run 启动：注册、心跳、隧道转发、HTTP 服务
func (s *Server) Run() error {
	if err := s.Register(); err != nil {
		log.Printf("bridge: register failed: %v", err)
	}
	go s.startHeartbeat()

	var ip2r *service.Ip2Region
	if s.cfg.Ip2RegionXDB != "" {
		v4Config, err := service.NewV4Config(service.VIndexCache, s.cfg.Ip2RegionXDB, 4)
		if err != nil {
			log.Printf("bridge: ip2region v4 init: %v", err)
		} else {
			var v6Config *service.Config
			if s.cfg.Ip2RegionXDB6 != "" {
				v6Config, _ = service.NewV6Config(service.VIndexCache, s.cfg.Ip2RegionXDB6, 2)
			}
			ip2r, err = service.NewIp2Region(v4Config, v6Config)
			if err != nil {
				log.Printf("bridge: ip2region: %v", err)
			} else {
				log.Printf("bridge: ip2region enabled (v4=%s)", s.cfg.Ip2RegionXDB)
			}
		}
	}

	go func() {
		if err := newRelay(s.cfg, ip2r).run(); err != nil {
			log.Printf("bridge: relay: %v", err)
		}
	}()

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
	mux.HandleFunc("POST /api/client/heartbeat", func(w http.ResponseWriter, r *http.Request) {
		s.proxyToApihub(w, r, "/api/client/heartbeat")
	})
	mux.HandleFunc("GET /api/user/balance", func(w http.ResponseWriter, r *http.Request) {
		s.proxyToApihub(w, r, "/api/user/balance")
	})

	addr := s.cfg.Listen
	if addr == "" {
		addr = ":8080"
	}
	log.Printf("bridge: listening on %s", addr)
	return http.ListenAndServe(addr, mux)
}
