package apihub

import (
	"encoding/json"
	"net/http"
	"time"

	"proxy-bridge/pkg/auth"
	"proxy-bridge/pkg/models"
)

// EdgeRegisterRequest Edge 注册请求
type EdgeRegisterRequest struct {
	Token   string `json:"token"`
	EdgeID  string `json:"edge_id"`
	Addr    string `json:"addr"`
	Country string `json:"country"`
}

// EdgeHeartbeatRequest Edge 心跳请求
type EdgeHeartbeatRequest struct {
	Token  string `json:"token"`
	EdgeID string `json:"edge_id"`
}

func (s *Server) handleEdgeRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		responseErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req EdgeRegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responseErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.Token == "" || req.EdgeID == "" || req.Addr == "" {
		responseErr(w, http.StatusBadRequest, "token, edge_id and addr required")
		return
	}
	node, err := auth.ValidateToken(s.db, req.Token)
	if err != nil {
		responseErr(w, http.StatusUnauthorized, "invalid token")
		return
	}
	if node.NodeType != models.NodeTypeEdge {
		responseErr(w, http.StatusForbidden, "not an edge node")
		return
	}
	now := time.Now()
	var reg models.EdgeRegistration
	err = s.db.Where("edge_id = ?", req.EdgeID).First(&reg).Error
	if err != nil {
		reg = models.EdgeRegistration{
			EdgeID:   req.EdgeID,
			NodeID:   node.ID,
			Addr:     req.Addr,
			Country:  req.Country,
			LastSeen: now,
		}
		if err := s.db.Create(&reg).Error; err != nil {
			responseErr(w, http.StatusInternalServerError, "create failed")
			return
		}
	} else {
		reg.Addr = req.Addr
		reg.Country = req.Country
		reg.LastSeen = now
		if err := s.db.Save(&reg).Error; err != nil {
			responseErr(w, http.StatusInternalServerError, "update failed")
			return
		}
	}
	_ = s.db.Model(&models.Node{}).Where("id = ?", node.ID).Update("last_seen", now)
	responseJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleEdgeHeartbeat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		responseErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req EdgeHeartbeatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responseErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.Token == "" && req.EdgeID == "" {
		responseErr(w, http.StatusBadRequest, "token or edge_id required")
		return
	}
	now := time.Now()
	if req.Token != "" {
		node, err := auth.ValidateToken(s.db, req.Token)
		if err != nil {
			responseErr(w, http.StatusUnauthorized, "invalid token")
			return
		}
		_ = s.db.Model(&models.Node{}).Where("id = ?", node.ID).Update("last_seen", now)
		res := s.db.Model(&models.EdgeRegistration{}).Where("node_id = ?", node.ID).Update("last_seen", now)
		if res.RowsAffected == 0 {
			responseErr(w, http.StatusNotFound, "edge not registered")
			return
		}
	} else {
		res := s.db.Model(&models.EdgeRegistration{}).Where("edge_id = ?", req.EdgeID).Updates(map[string]interface{}{
			"last_seen": now,
		})
		if res.Error != nil || res.RowsAffected == 0 {
			responseErr(w, http.StatusNotFound, "edge not found")
			return
		}
	}
	responseJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
