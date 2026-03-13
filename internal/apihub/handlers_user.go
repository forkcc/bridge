package apihub

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"

	"proxy-bridge/pkg/models"
)

// UserLoginRequest 用户登录请求
type UserLoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// UserLoginResponse 用户登录响应
type UserLoginResponse struct {
	OK     bool   `json:"ok"`
	UserID uint   `json:"user_id,omitempty"`
	Token  string `json:"token,omitempty"`
}

func (s *Server) handleUserLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		responseErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req UserLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responseErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.Username == "" || req.Password == "" {
		responseErr(w, http.StatusBadRequest, "username and password required")
		return
	}
	var u models.User
	if err := s.db.Where("username = ?", req.Username).First(&u).Error; err != nil {
		responseErr(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	// 密码为 SHA256 十六进制存储（生产环境建议改用 bcrypt）
	sum := sha256.Sum256([]byte(req.Password))
	if hex.EncodeToString(sum[:]) != u.PasswordHash {
		responseErr(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	responseJSON(w, http.StatusOK, UserLoginResponse{OK: true, UserID: u.ID})
}
