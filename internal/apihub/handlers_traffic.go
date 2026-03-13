package apihub

import (
	"encoding/json"
	"net/http"
	"time"

	"gorm.io/gorm"

	"proxy-bridge/pkg/models"
)

// TrafficReportRequest 流量上报请求
type TrafficReportRequest struct {
	UserID   uint   `json:"user_id"`
	EdgeID   string `json:"edge_id"`
	BytesIn  int64  `json:"bytes_in"`
	BytesOut int64  `json:"bytes_out"`
}

func (s *Server) handleTrafficReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		responseErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req TrafficReportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responseErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.UserID == 0 || req.EdgeID == "" {
		responseErr(w, http.StatusBadRequest, "user_id and edge_id required")
		return
	}
	now := time.Now()
	if err := s.db.Create(&models.TrafficStat{
		UserID:     req.UserID,
		EdgeID:     req.EdgeID,
		BytesIn:    req.BytesIn,
		BytesOut:   req.BytesOut,
		ReportedAt: now,
	}).Error; err != nil {
		responseErr(w, http.StatusInternalServerError, "report failed")
		return
	}
	// 同步计费：按流量冻结余额（简化：每 MB 1 单位，不足 1MB 按 0）
	amount := (req.BytesIn + req.BytesOut) / (1024 * 1024)
	if amount > 0 {
		refID := time.Now().Format("20060102150405")
		if err := s.db.Create(&models.FrozenBalance{
			UserID: req.UserID,
			Amount: amount,
			Reason: "traffic",
			RefID:  refID,
		}).Error; err != nil {
			responseJSON(w, http.StatusOK, map[string]string{"status": "ok", "warning": "freeze failed"})
			return
		}
		_ = s.db.Model(&models.User{}).Where("id = ?", req.UserID).Update("frozen_balance", gorm.Expr("frozen_balance + ?", amount))
	}
	responseJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
