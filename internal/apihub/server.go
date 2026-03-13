package apihub

import (
	"encoding/json"
	"log"
	"net/http"

	"gorm.io/gorm"

	"proxy-bridge/pkg/database"
)

// Server apiHub HTTP 服务
type Server struct {
	cfg *Config
	db  *gorm.DB
}

// NewServer 创建 apiHub 服务
func NewServer(cfg *Config) (*Server, error) {
	dbCfg := database.Config{
		Host:     cfg.Database.Host,
		Port:     cfg.Database.Port,
		User:     cfg.Database.User,
		Password: cfg.Database.Password,
		DBName:   cfg.Database.DBName,
		SSLMode:  cfg.Database.SSLMode,
	}
	db, err := database.Open(dbCfg)
	if err != nil {
		return nil, err
	}
	return &Server{cfg: cfg, db: db}, nil
}

// responseJSON 写 JSON 响应
func responseJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

// responseErr 写错误响应
func responseErr(w http.ResponseWriter, status int, msg string) {
	responseJSON(w, status, map[string]string{"error": msg})
}

// Router 注册所有路由
func (s *Server) Router() *http.ServeMux {
	mux := http.NewServeMux()

	// 健康检查
	mux.HandleFunc("GET /health", s.handleHealth)

	// Bridge
	mux.HandleFunc("POST /api/bridge/register", s.handleBridgeRegister)
	mux.HandleFunc("POST /api/bridge/heartbeat", s.handleBridgeHeartbeat)

	// Edge
	mux.HandleFunc("POST /api/edge/register", s.handleEdgeRegister)
	mux.HandleFunc("POST /api/edge/heartbeat", s.handleEdgeHeartbeat)
	mux.HandleFunc("POST /api/edge/country", s.handleEdgeSetCountry)

	// Client
	mux.HandleFunc("POST /api/client/auth", s.handleClientAuth)
	mux.HandleFunc("POST /api/client/heartbeat", s.handleClientHeartbeat)
	mux.HandleFunc("POST /api/client/country", s.handleClientSetCountry)
	mux.HandleFunc("GET /api/edges", s.handleEdgesList)

	// 流量与计费
	mux.HandleFunc("POST /api/traffic/report", s.handleTrafficReport)

	// 用户
	mux.HandleFunc("POST /api/user/login", s.handleUserLogin)
	mux.HandleFunc("GET /api/user/balance", s.handleUserBalance)

	return mux
}

// handleHealth 健康检查
func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	responseJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}


// Run 启动 HTTP 服务
func (s *Server) Run() error {
	go s.startBindingCleanupLoop()
	addr := s.cfg.Listen
	if addr == "" {
		addr = ":8082"
	}
	log.Printf("apihub: listening on %s", addr)
	return http.ListenAndServe(addr, s.Router())
}

