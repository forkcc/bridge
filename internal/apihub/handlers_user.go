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
	responseJSON(w, http.StatusOK, UserLoginResponse{OK: true, UserID: u.ID, Token: u.Token})
}

// handleUserBalance 查询用户余额（支持 token 或 user_id 参数）
func (s *Server) handleUserBalance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		responseErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var u models.User
	if uid := r.URL.Query().Get("user_id"); uid != "" {
		if err := s.db.Where("id = ?", uid).First(&u).Error; err != nil {
			responseErr(w, http.StatusNotFound, "user not found")
			return
		}
	} else {
		token := r.URL.Query().Get("token")
		if token == "" {
			if h := r.Header.Get("Authorization"); len(h) > 7 && h[:7] == "Bearer " {
				token = h[7:]
			}
		}
		if token == "" {
			responseErr(w, http.StatusBadRequest, "token or user_id required")
			return
		}
		if err := s.db.Where("token = ?", token).First(&u).Error; err != nil {
			responseErr(w, http.StatusUnauthorized, "invalid token")
			return
		}
	}
	responseJSON(w, http.StatusOK, map[string]int64{"balance": u.Balance})
}
