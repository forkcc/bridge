package apihub

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"gorm.io/gorm"

	"proxy-bridge/pkg/models"
)

// TrafficReportRequest 流量上报 + 立即结算
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

	totalKB := (req.BytesIn + req.BytesOut) / 1024

	err := s.db.Transaction(func(tx *gorm.DB) error {
		// 审计记录
		if err := tx.Create(&models.TrafficStat{
			UserID:     req.UserID,
			EdgeID:     req.EdgeID,
			BytesIn:    req.BytesIn,
			BytesOut:   req.BytesOut,
			ReportedAt: time.Now(),
		}).Error; err != nil {
			return err
		}

		if totalKB <= 0 {
			return nil
		}

		// Client 扣费
		var clientBalance int64
		if err := tx.Raw(
			"UPDATE users SET balance = GREATEST(balance - ?, 0), updated_at = NOW() WHERE id = ? RETURNING balance",
			totalKB, req.UserID,
		).Scan(&clientBalance).Error; err != nil {
			return err
		}
		if err := tx.Create(&models.FundFlow{
			UserID:  req.UserID,
			Amount:  -totalKB,
			Balance: clientBalance,
			Type:    "traffic_client",
		}).Error; err != nil {
			return err
		}

		// Edge 赚取：通过 edge_registrations → nodes 查找 edge 用户
		var node models.Node
		if err := tx.Joins("JOIN edge_registrations ON edge_registrations.node_id = nodes.id").
			Where("edge_registrations.edge_id = ?", req.EdgeID).
			First(&node).Error; err != nil {
			log.Printf("apihub: edge %s user lookup failed: %v", req.EdgeID, err)
			return nil
		}
		if node.UserID == 0 {
			return nil
		}
		var edgeBalance int64
		if err := tx.Raw(
			"UPDATE users SET balance = balance + ?, updated_at = NOW() WHERE id = ? RETURNING balance",
			totalKB, node.UserID,
		).Scan(&edgeBalance).Error; err != nil {
			return err
		}
		return tx.Create(&models.FundFlow{
			UserID:  node.UserID,
			Amount:  totalKB,
			Balance: edgeBalance,
			Type:    "traffic_edge",
		}).Error
	})

	if err != nil {
		log.Printf("apihub: traffic report+settle user=%d edge=%s: %v", req.UserID, req.EdgeID, err)
		responseErr(w, http.StatusInternalServerError, "report failed")
		return
	}
	responseJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
