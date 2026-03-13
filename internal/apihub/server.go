package apihub

import (
	"encoding/json"
	"log"
	"net/http"

	"gorm.io/gorm"

	"proxy-bridge/pkg/database"
	"proxy-bridge/pkg/models"
	"proxy-bridge/pkg/rabbitmq"
)

// Server apiHub HTTP 服务
type Server struct {
	cfg *Config
	db  *gorm.DB
	mq  *rabbitmq.Client
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
	srv := &Server{cfg: cfg, db: db}
	if cfg.RabbitMQ.URL != "" {
		mq, err := rabbitmq.New(cfg.RabbitMQ.URL)
		if err != nil {
			log.Printf("apihub: rabbitmq init skip: %v", err)
		} else {
			srv.mq = mq
			_ = mq.EnsureQueue(cfg.RabbitMQ.QueueTraffic)
		}
	}
	return srv, nil
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

	// Client
	mux.HandleFunc("POST /api/client/auth", s.handleClientAuth)
	mux.HandleFunc("GET /api/edges", s.handleEdgesList)

	// 流量与计费
	mux.HandleFunc("POST /api/traffic/report", s.handleTrafficReport)

	// 用户
	mux.HandleFunc("POST /api/user/login", s.handleUserLogin)

	return mux
}

// handleHealth 健康检查
func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	responseJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}


// Run 启动 HTTP 服务与可选的 MQ 消费者、定时结算
func (s *Server) Run() error {
	if s.mq != nil {
		go s.consumeTraffic()
	}
	go s.startSettleLoop()
	addr := s.cfg.Listen
	if addr == "" {
		addr = ":8082"
	}
	log.Printf("apihub: listening on %s", addr)
	return http.ListenAndServe(addr, s.Router())
}

// consumeTraffic 消费流量计费队列：冻结余额并更新 message_tracking
func (s *Server) consumeTraffic() {
	q := s.cfg.RabbitMQ.QueueTraffic
	if q == "" {
		q = "traffic_billing"
	}
	_ = s.mq.Consume(q, func(body []byte) error {
		var msg struct {
			MessageID string `json:"message_id"`
			UserID    uint   `json:"user_id"`
			EdgeID    string `json:"edge_id"`
			BytesIn   int64  `json:"bytes_in"`
			BytesOut  int64  `json:"bytes_out"`
		}
		if err := json.Unmarshal(body, &msg); err != nil {
			return err
		}
		amount := (msg.BytesIn + msg.BytesOut) / (1024 * 1024)
		if amount > 0 {
			if err := s.db.Create(&models.FrozenBalance{
				UserID: msg.UserID,
				Amount: amount,
				Reason: "traffic",
				RefID:  msg.MessageID,
			}).Error; err != nil {
				return err
			}
			_ = s.db.Model(&models.User{}).Where("id = ?", msg.UserID).Update("frozen_balance", gorm.Expr("frozen_balance + ?", amount))
		}
		_ = s.db.Model(&models.MessageTracking{}).Where("message_id = ?", msg.MessageID).Update("status", "done")
		return nil
	})
}
