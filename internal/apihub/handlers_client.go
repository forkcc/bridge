package apihub

import (
	"encoding/json"
	"net/http"
	"time"

	"proxy-bridge/pkg/auth"
	"proxy-bridge/pkg/models"
)

const offlineThreshold = 2 * time.Minute // client/edge 超过此时长未心跳视为下线，用于解绑与列表过滤

// ClientAuthRequest Client 认证请求
type ClientAuthRequest struct {
	Token    string `json:"token"`
	ClientID string `json:"client_id"` // 可选，启动时传入的 id，会写入 nodes.node_id
}

// ClientHeartbeatRequest Client 心跳请求（携带当前绑定的 edge_id，用于双向解绑）
type ClientHeartbeatRequest struct {
	Token    string `json:"token"`
	ClientID string `json:"client_id"` // 可选，启动时传入的 id，会写入 nodes.node_id
	EdgeID   string `json:"edge_id"`
}

// ClientAuthResponse Client 认证响应
type ClientAuthResponse struct {
	OK       bool   `json:"ok"`
	ClientID string `json:"client_id,omitempty"`
	NodeID   uint   `json:"node_id,omitempty"`
	UserID   uint   `json:"user_id,omitempty"`
}

// EdgesListResponse Edge 列表项
type EdgesListResponse struct {
	EdgeID  string `json:"edge_id"`
	Addr    string `json:"addr"`
	Country string `json:"country"`
}

func (s *Server) handleClientAuth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		responseErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req ClientAuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responseErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.Token == "" {
		responseErr(w, http.StatusBadRequest, "token required")
		return
	}
	node, err := auth.ValidateToken(s.db, req.Token, models.NodeTypeClient)
	if err != nil {
		responseErr(w, http.StatusUnauthorized, "invalid token")
		return
	}
	now := time.Now()
	var user models.User
	_ = s.db.Where("token = ?", req.Token).First(&user).Error
	upd := map[string]interface{}{"last_seen": now, "status": "online"}
	if req.ClientID != "" {
		upd["node_id"] = req.ClientID
	}
	if user.ID > 0 {
		upd["user_id"] = user.ID
	}
	_ = s.db.Model(&models.Node{}).Where("id = ?", node.ID).Updates(upd)
	responseJSON(w, http.StatusOK, ClientAuthResponse{
		OK:       true,
		NodeID:   node.ID,
		ClientID: node.Token,
		UserID:   user.ID,
	})
}

func (s *Server) handleEdgesList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		responseErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	token := r.URL.Query().Get("token")
	if token == "" {
		if h := r.Header.Get("Authorization"); len(h) > 7 && h[:7] == "Bearer " {
			token = h[7:]
		}
	}
	if token == "" {
		responseErr(w, http.StatusUnauthorized, "token required")
		return
	}
	node, err := auth.ValidateToken(s.db, token, models.NodeTypeClient)
	if err != nil {
		responseErr(w, http.StatusUnauthorized, "invalid token")
		return
	}
	now := time.Now()
	_ = s.db.Model(&models.Node{}).Where("id = ?", node.ID).Updates(map[string]interface{}{"last_seen": now, "status": "online"})
	country := r.URL.Query().Get("country")
	onlineSince := now.Add(-offlineThreshold)
	var list []models.EdgeRegistration
	// 返回空闲(idle)或已绑定到该 client（busy 但属于本 client）的 edge
	q := s.db.Model(&models.EdgeRegistration{}).
		Joins("JOIN nodes ON nodes.id = edge_registrations.node_id AND nodes.node_type = ? AND nodes.token = ?", models.NodeTypeEdge, node.Token).
		Joins("LEFT JOIN client_edge_bindings ON client_edge_bindings.edge_id = edge_registrations.edge_id AND client_edge_bindings.client_id = ?", node.Token).
		Where("edge_registrations.last_seen >= ?", onlineSince).
		Where("nodes.status = ? OR client_edge_bindings.client_id IS NOT NULL", models.NodeStatusIdle).
		Order("edge_registrations.last_seen DESC")
	if country != "" {
		q = q.Where("nodes.country = ?", country)
	}
	if err := q.Find(&list).Error; err != nil {
		responseErr(w, http.StatusInternalServerError, "list failed")
		return
	}
	out := make([]EdgesListResponse, 0, len(list))
	for _, e := range list {
		out = append(out, EdgesListResponse{EdgeID: e.EdgeID, Addr: e.Addr, Country: e.Country})
	}
	if out == nil {
		out = []EdgesListResponse{}
	}
	responseJSON(w, http.StatusOK, out)
}

func (s *Server) handleClientHeartbeat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		responseErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req ClientHeartbeatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responseErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.Token == "" {
		responseErr(w, http.StatusBadRequest, "token required")
		return
	}
	node, err := auth.ValidateToken(s.db, req.Token, models.NodeTypeClient)
	if err != nil {
		responseErr(w, http.StatusUnauthorized, "invalid token")
		return
	}
	now := time.Now()
	upd := map[string]interface{}{"last_seen": now, "status": "online"}
	if req.ClientID != "" {
		upd["node_id"] = req.ClientID
	}
	_ = s.db.Model(&models.Node{}).Where("id = ?", node.ID).Updates(upd)

	var prevBind models.ClientEdgeBinding
	prevErr := s.db.Where("client_id = ?", node.Token).First(&prevBind).Error
	prevEdgeID := ""
	if prevErr == nil {
		prevEdgeID = prevBind.EdgeID
	}

	if req.EdgeID != "" {
		// 绑定：原绑定的 edge 解绑后设回 idle；新 edge 设为 busy
		if prevEdgeID != "" && prevEdgeID != req.EdgeID {
			_ = s.db.Exec("UPDATE nodes SET status = ? WHERE id IN (SELECT node_id FROM edge_registrations WHERE edge_id = ?)", models.NodeStatusIdle, prevEdgeID)
		}
		if prevErr != nil {
			_ = s.db.Create(&models.ClientEdgeBinding{ClientID: node.Token, EdgeID: req.EdgeID}).Error
		} else {
			_ = s.db.Model(&prevBind).Updates(map[string]interface{}{"edge_id": req.EdgeID})
		}
		_ = s.db.Exec("UPDATE nodes SET status = ? WHERE id IN (SELECT node_id FROM edge_registrations WHERE edge_id = ?)", models.NodeStatusBusy, req.EdgeID)
	} else {
		// 解绑：当前绑定的 edge 设回 idle
		if prevEdgeID != "" {
			_ = s.db.Exec("UPDATE nodes SET status = ? WHERE id IN (SELECT node_id FROM edge_registrations WHERE edge_id = ?)", models.NodeStatusIdle, prevEdgeID)
			_ = s.db.Where("client_id = ?", node.Token).Delete(&models.ClientEdgeBinding{}).Error
		}
	}
	responseJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ClientSetCountryRequest Bridge 上报 client 国家
type ClientSetCountryRequest struct {
	Token   string `json:"token"`
	Country string `json:"country"`
}

func (s *Server) handleClientSetCountry(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		responseErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req ClientSetCountryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responseErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.Token == "" {
		responseErr(w, http.StatusBadRequest, "token required")
		return
	}
	if req.Country == "" {
		req.Country = "unknown"
	}
	s.db.Model(&models.Node{}).Where("token = ? AND node_type = ?", req.Token, models.NodeTypeClient).Update("country", req.Country)
	responseJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
