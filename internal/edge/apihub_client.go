package edge

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

const heartbeatInterval = 30 * time.Second

// Register 向 apiHub 注册
func (s *Server) Register() error {
	addr := s.cfg.BridgeTunnel
	if addr == "" {
		addr = "localhost:8081"
	}
	body, _ := json.Marshal(map[string]string{
		"token":   s.cfg.Token,
		"edge_id": s.cfg.ID,
		"addr":    addr,
		"country": "",
	})
	resp, err := http.Post(s.cfg.ApihubURL+"/api/edge/register", "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("register: %d", resp.StatusCode)
	}
	return nil
}

// startHeartbeat 定时向 apiHub 上报心跳
func (s *Server) startHeartbeat() {
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()
	for range ticker.C {
		body, _ := json.Marshal(map[string]string{
			"token":   s.cfg.Token,
			"edge_id": s.cfg.ID,
		})
		resp, err := http.Post(s.cfg.ApihubURL+"/api/edge/heartbeat", "application/json", bytes.NewReader(body))
		if err != nil {
			log.Printf("edge: heartbeat error: %v", err)
			continue
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			log.Printf("edge: heartbeat %d", resp.StatusCode)
		}
	}
}
