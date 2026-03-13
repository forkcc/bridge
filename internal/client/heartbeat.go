package client

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

const heartbeatInterval = 30 * time.Second
const balanceTUIInterval = 30 * time.Second

// startHeartbeat 定时向 Bridge/apiHub 上报心跳并刷新当前绑定的 edge_id，便于双向解绑
func (s *Server) startHeartbeat() {
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()
	for range ticker.C {
		edgeID := ""
		if s.tunnelState != nil {
			s.tunnelState.mu.Lock()
			edgeID = s.tunnelState.edgeID
			s.tunnelState.mu.Unlock()
		}
		payload := map[string]string{"token": s.cfg.Token, "edge_id": edgeID}
		if s.cfg.ID != "" {
			payload["client_id"] = s.cfg.ID
		}
		body, _ := json.Marshal(payload)
		resp, err := http.Post(s.cfg.BridgeURL+"/api/client/heartbeat", "application/json", bytes.NewReader(body))
		if err != nil {
			log.Printf("client: heartbeat: %v", err)
			continue
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			log.Printf("client: heartbeat %d", resp.StatusCode)
		}
	}
}

// startBalanceTUI 定期拉取并打印余额（TUI 只显示余额）
func (s *Server) startBalanceTUI() {
	ticker := time.NewTicker(balanceTUIInterval)
	defer ticker.Stop()
	for range ticker.C {
		url := s.cfg.BridgeURL + "/api/user/balance?token=" + s.cfg.Token
		resp, err := http.Get(url)
		if err != nil {
			log.Printf("client: balance: %v", err)
			continue
		}
		var out struct {
			Balance int64 `json:"balance"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			resp.Body.Close()
			continue
		}
		resp.Body.Close()
		log.Printf("余额: %d", out.Balance)
	}
}
