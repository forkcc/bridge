package apihub

import (
	"encoding/json"
	"fmt"
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
	if s.mq != nil {
		messageID := fmt.Sprintf("traffic-%d-%s", now.UnixNano(), req.EdgeID)
		payload, _ := json.Marshal(map[string]interface{}{
			"message_id": messageID,
			"user_id":    req.UserID,
			"edge_id":    req.EdgeID,
			"bytes_in":   req.BytesIn,
			"bytes_out":  req.BytesOut,
		})
		_ = s.db.Create(&models.MessageTracking{
			MessageID: messageID,
			Topic:     s.cfg.RabbitMQ.QueueTraffic,
			Payload:   string(payload),
			Status:    "pending",
		})
		_ = s.mq.Publish(s.cfg.RabbitMQ.QueueTraffic, map[string]interface{}{
			"message_id": messageID,
			"user_id":    req.UserID,
			"edge_id":    req.EdgeID,
			"bytes_in":   req.BytesIn,
			"bytes_out":  req.BytesOut,
		})
	} else {
		// 无 MQ 时同步计费
		amount := (req.BytesIn + req.BytesOut) / (1024 * 1024)
		if amount > 0 {
			refID := time.Now().Format("20060102150405")
			_ = s.db.Create(&models.FrozenBalance{UserID: req.UserID, Amount: amount, Reason: "traffic", RefID: refID})
			_ = s.db.Model(&models.User{}).Where("id = ?", req.UserID).Update("frozen_balance", gorm.Expr("frozen_balance + ?", amount))
		}
	}
	responseJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
