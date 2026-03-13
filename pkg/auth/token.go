package auth

import (
	"errors"

	"gorm.io/gorm"

	"proxy-bridge/pkg/models"
)

var ErrInvalidToken = errors.New("invalid token")

// ValidateToken 校验 token：须在 users 表存在（users.token），再按 nodeType 从 nodes 表取节点；nodes.token 指向 users.token
func ValidateToken(db *gorm.DB, token string, nodeType string) (*models.Node, error) {
	if token == "" {
		return nil, ErrInvalidToken
	}
	var user models.User
	if err := db.Where("token = ?", token).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvalidToken
		}
		return nil, err
	}
	var node models.Node
	// 不按 status 过滤，以便离线节点重连时能通过校验并被重新标为 online
	err := db.Where("token = ? AND node_type = ?", token, nodeType).First(&node).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvalidToken
		}
		return nil, err
	}
	return &node, nil
}
