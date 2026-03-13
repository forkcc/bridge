package auth

import (
	"errors"

	"gorm.io/gorm"

	"proxy-bridge/pkg/models"
)

var ErrInvalidToken = errors.New("invalid token")

// ValidateToken 根据 token 从 nodes 表校验并返回节点信息；无效则返回 ErrInvalidToken
func ValidateToken(db *gorm.DB, token string) (*models.Node, error) {
	if token == "" {
		return nil, ErrInvalidToken
	}
	var node models.Node
	err := db.Where("token = ? AND status = ?", token, "online").First(&node).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvalidToken
		}
		return nil, err
	}
	return &node, nil
}
