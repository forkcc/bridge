package apihub

import (
	"encoding/json"
	"net/http"

	"proxy-bridge/pkg/auth"
	"proxy-bridge/pkg/models"
)

// ClientAuthRequest Client 认证请求
type ClientAuthRequest struct {
	Token string `json:"token"`
}

// ClientAuthResponse Client 认证响应
type ClientAuthResponse struct {
	OK       bool   `json:"ok"`
	ClientID string `json:"client_id,omitempty"`
	NodeID   uint   `json:"node_id,omitempty"`
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
	node, err := auth.ValidateToken(s.db, req.Token)
	if err != nil {
		responseErr(w, http.StatusUnauthorized, "invalid token")
		return
	}
	if node.NodeType != models.NodeTypeClient {
		responseErr(w, http.StatusForbidden, "not a client node")
		return
	}
	responseJSON(w, http.StatusOK, ClientAuthResponse{
		OK:       true,
		NodeID:   node.ID,
		ClientID: node.Token,
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
	_, err := auth.ValidateToken(s.db, token)
	if err != nil {
		responseErr(w, http.StatusUnauthorized, "invalid token")
		return
	}
	country := r.URL.Query().Get("country")
	var list []models.EdgeRegistration
	q := s.db.Model(&models.EdgeRegistration{}).Order("last_seen DESC")
	if country != "" {
		q = q.Where("country = ?", country)
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
