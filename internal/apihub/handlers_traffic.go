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
	// 流量统计始终记录（审计用途），计费扣款在后续判断阈值
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
	// 计费：按 KB 计算费用，直接从余额扣除
	totalKB := (req.BytesIn + req.BytesOut) / 1024
	if totalKB > 0 {
		s.db.Transaction(func(tx *gorm.DB) error {
			var u models.User
			if err := tx.Where("id = ?", req.UserID).First(&u).Error; err != nil {
				return err
			}
			newBalance := u.Balance - totalKB
			if newBalance < 0 {
				newBalance = 0
			}
			if err := tx.Model(&models.User{}).Where("id = ?", req.UserID).Update("balance", newBalance).Error; err != nil {
				return err
			}
			return tx.Create(&models.FundFlow{
				UserID:  req.UserID,
				Amount:  -totalKB,
				Balance: newBalance,
				Type:    "traffic",
			}).Error
		})
	}
	responseJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
