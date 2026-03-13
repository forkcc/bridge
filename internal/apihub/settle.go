package apihub

import (
	"log"
	"time"

	"gorm.io/gorm"

	"proxy-bridge/pkg/models"
)

const settleInterval = 10 * time.Minute

// startSettleLoop 每 10 分钟收集未结算的冻结余额并扣款、写流水
func (s *Server) startSettleLoop() {
	ticker := time.NewTicker(settleInterval)
	defer ticker.Stop()
	for range ticker.C {
		if err := s.settleFrozenBalances(); err != nil {
			log.Printf("apihub: settle frozen balances: %v", err)
		}
	}
}

// settleFrozenBalances 将未结算的 FrozenBalance 汇总结算
func (s *Server) settleFrozenBalances() error {
	var list []models.FrozenBalance
	if err := s.db.Where("settled_at IS NULL").Find(&list).Error; err != nil {
		return err
	}
	if len(list) == 0 {
		return nil
	}
	// 按用户汇总
	userTotal := make(map[uint]int64)
	for _, f := range list {
		userTotal[f.UserID] += f.Amount
	}
	now := time.Now()
	return s.db.Transaction(func(tx *gorm.DB) error {
		for userID, total := range userTotal {
			var u models.User
			if err := tx.Where("id = ?", userID).First(&u).Error; err != nil {
				return err
			}
			newBalance := u.Balance - total
			newFrozen := u.FrozenBalance - total
			if newFrozen < 0 {
				newFrozen = 0
			}
			if newBalance < 0 {
				newBalance = 0
			}
			if err := tx.Model(&models.User{}).Where("id = ?", userID).Updates(map[string]interface{}{
				"balance":        newBalance,
				"frozen_balance": newFrozen,
			}).Error; err != nil {
				return err
			}
			if err := tx.Create(&models.FundFlow{
				UserID: userID,
				Amount: -total,
				Balance: newBalance,
				Type:   "traffic",
				RefID:  "settle",
			}).Error; err != nil {
				return err
			}
		}
		ids := make([]uint, 0, len(list))
		for _, f := range list {
			ids = append(ids, f.ID)
		}
		return tx.Model(&models.FrozenBalance{}).Where("id IN ?", ids).Update("settled_at", now).Error
	})
}
