package apihub

import (
	"log"
	"time"

	"proxy-bridge/pkg/models"
)

// startBindingCleanupLoop 定时清理离线 client/edge 的绑定，并将超时未心跳的 nodes 标为 offline
func (s *Server) startBindingCleanupLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		threshold := time.Now().Add(-offlineThreshold)
		// 将超时未心跳的节点标为 offline
		res := s.db.Model(&models.Node{}).Where("last_seen IS NULL OR last_seen < ?", threshold).Update("status", models.NodeStatusOffline)
		if res.Error != nil {
			log.Printf("apihub: set nodes offline: %v", res.Error)
		} else if res.RowsAffected > 0 {
			log.Printf("apihub: %d node(s) set offline", res.RowsAffected)
		}
		// client 下线：被绑的 edge 设回 idle，再删除绑定
		_ = s.db.Exec(
			"UPDATE nodes SET status = ? WHERE id IN (SELECT node_id FROM edge_registrations WHERE edge_id IN (SELECT edge_id FROM client_edge_bindings WHERE client_id IN (SELECT token FROM nodes WHERE node_type = ? AND (last_seen IS NULL OR last_seen < ?))))",
			models.NodeStatusIdle, models.NodeTypeClient, threshold,
		)
		res = s.db.Exec(
			"DELETE FROM client_edge_bindings WHERE client_id IN (SELECT token FROM nodes WHERE node_type = ? AND (last_seen IS NULL OR last_seen < ?))",
			models.NodeTypeClient, threshold,
		)
		if res.Error != nil {
			log.Printf("apihub: binding cleanup client: %v", res.Error)
		} else if res.RowsAffected > 0 {
			log.Printf("apihub: unbind %d client(s) offline", res.RowsAffected)
		}
		// edge 下线：删除绑定到该 edge 的绑定（该 edge 已在上文标为 offline）
		res = s.db.Exec(
			"DELETE FROM client_edge_bindings WHERE edge_id IN (SELECT edge_id FROM edge_registrations WHERE last_seen < ?)",
			threshold,
		)
		if res.Error != nil {
			log.Printf("apihub: binding cleanup edge: %v", res.Error)
		} else if res.RowsAffected > 0 {
			log.Printf("apihub: unbind %d edge(s) offline", res.RowsAffected)
		}
	}
}
