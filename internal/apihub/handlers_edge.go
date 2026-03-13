package apihub

import (
	"encoding/json"
	"log"
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
	node, err := auth.ValidateToken(s.db, req.Token, models.NodeTypeEdge)
	if err != nil {
		responseErr(w, http.StatusUnauthorized, "invalid token")
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
		reg.NodeID = node.ID
		reg.Addr = req.Addr
		reg.Country = req.Country
		reg.LastSeen = now
		if err := s.db.Save(&reg).Error; err != nil {
			responseErr(w, http.StatusInternalServerError, "update failed")
			return
		}
	}
	edgeNodeUpd := map[string]interface{}{
		"last_seen": now,
		"node_id":   req.EdgeID,
		"status":    models.NodeStatusIdle,
	}
	var user models.User
	if s.db.Where("token = ?", req.Token).First(&user).Error == nil && user.ID > 0 {
		edgeNodeUpd["user_id"] = user.ID
	}
	_ = s.db.Model(&models.Node{}).Where("id = ?", node.ID).Updates(edgeNodeUpd)
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
		node, err := auth.ValidateToken(s.db, req.Token, models.NodeTypeEdge)
		if err != nil {
			responseErr(w, http.StatusUnauthorized, "invalid token")
			return
		}
		// 仅当当前为 offline 时改为 idle，避免覆盖 busy
		_ = s.db.Exec("UPDATE nodes SET last_seen = ?, status = CASE WHEN status = ? THEN ? ELSE status END WHERE id = ?", now, models.NodeStatusOffline, models.NodeStatusIdle, node.ID)
		res := s.db.Model(&models.EdgeRegistration{}).Where("node_id = ?", node.ID).Update("last_seen", now)
		if res.RowsAffected == 0 {
			responseErr(w, http.StatusNotFound, "edge not registered")
			return
		}
	} else {
		res := s.db.Model(&models.EdgeRegistration{}).Where("edge_id = ?", req.EdgeID).Updates(map[string]interface{}{"last_seen": now})
		if res.RowsAffected > 0 {
			_ = s.db.Exec("UPDATE nodes SET last_seen = ?, status = CASE WHEN status = ? THEN ? ELSE status END WHERE id IN (SELECT node_id FROM edge_registrations WHERE edge_id = ?)", now, models.NodeStatusOffline, models.NodeStatusIdle, req.EdgeID)
		}
		if res.Error != nil || res.RowsAffected == 0 {
			responseErr(w, http.StatusNotFound, "edge not found")
			return
		}
	}
	responseJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// EdgeSetCountryRequest Bridge 上报 edge 国家（连接时按对端 IP 用 ip2region 解析）
type EdgeSetCountryRequest struct {
	EdgeID  string `json:"edge_id"`
	Country string `json:"country"`
}

func (s *Server) handleEdgeSetCountry(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		responseErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req EdgeSetCountryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responseErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.EdgeID == "" {
		responseErr(w, http.StatusBadRequest, "edge_id required")
		return
	}
	if req.Country == "" {
		req.Country = "unknown"
	}
	if res := s.db.Model(&models.EdgeRegistration{}).Where("edge_id = ?", req.EdgeID).Update("country", req.Country); res.Error != nil {
		responseErr(w, http.StatusInternalServerError, "update failed")
		return
	}
	res := s.db.Exec(
		"UPDATE nodes SET country = ? WHERE id IN (SELECT node_id FROM edge_registrations WHERE edge_id = ?)",
		req.Country, req.EdgeID,
	)
	if res.Error != nil {
		responseErr(w, http.StatusInternalServerError, "update nodes failed")
		return
	}
	if res.RowsAffected == 0 {
		log.Printf("apihub: set country edge_id=%s: no node found (edge not registered?)", req.EdgeID)
	}
	responseJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
