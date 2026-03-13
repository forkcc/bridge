package apihub

import (
	"encoding/json"
	"net/http"
	"time"

	"proxy-bridge/pkg/models"
)

// BridgeRegisterRequest 注册请求
type BridgeRegisterRequest struct {
	BridgeID string `json:"bridge_id"`
	Addr     string `json:"addr"`
}

func (s *Server) handleBridgeRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		responseErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req BridgeRegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responseErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.BridgeID == "" || req.Addr == "" {
		responseErr(w, http.StatusBadRequest, "bridge_id and addr required")
		return
	}
	now := time.Now()
	var reg models.BridgeRegistration
	err := s.db.Where("bridge_id = ?", req.BridgeID).First(&reg).Error
	if err != nil {
		reg = models.BridgeRegistration{BridgeID: req.BridgeID, Addr: req.Addr, LastSeen: now}
		if err := s.db.Create(&reg).Error; err != nil {
			responseErr(w, http.StatusInternalServerError, "create failed")
			return
		}
	} else {
		reg.Addr = req.Addr
		reg.LastSeen = now
		if err := s.db.Save(&reg).Error; err != nil {
			responseErr(w, http.StatusInternalServerError, "update failed")
			return
		}
	}
	responseJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleBridgeHeartbeat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		responseErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req BridgeRegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responseErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.BridgeID == "" {
		responseErr(w, http.StatusBadRequest, "bridge_id required")
		return
	}
	now := time.Now()
	res := s.db.Model(&models.BridgeRegistration{}).Where("bridge_id = ?", req.BridgeID).Updates(map[string]interface{}{
		"last_seen": now,
		"addr":      req.Addr,
	})
	if res.Error != nil {
		responseErr(w, http.StatusInternalServerError, "update failed")
		return
	}
	if res.RowsAffected == 0 {
		responseErr(w, http.StatusNotFound, "bridge not registered")
		return
	}
	responseJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
