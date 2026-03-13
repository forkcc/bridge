package bridge

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
	body, _ := json.Marshal(map[string]string{
		"bridge_id": s.cfg.BridgeID,
		"addr":      s.cfg.Listen,
	})
	resp, err := http.Post(s.cfg.ApihubURL+"/api/bridge/register", "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("register: %d", resp.StatusCode)
	}
	log.Printf("bridge: registered to apihub as %s", s.cfg.BridgeID)
	return nil
}

// startHeartbeat 定时向 apiHub 上报心跳
func (s *Server) startHeartbeat() {
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()
	for range ticker.C {
		body, _ := json.Marshal(map[string]string{
			"bridge_id": s.cfg.BridgeID,
			"addr":      s.cfg.Listen,
		})
		resp, err := http.Post(s.cfg.ApihubURL+"/api/bridge/heartbeat", "application/json", bytes.NewReader(body))
		if err != nil {
			log.Printf("bridge: heartbeat error: %v", err)
			continue
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			log.Printf("bridge: heartbeat %d", resp.StatusCode)
		}
	}
}
