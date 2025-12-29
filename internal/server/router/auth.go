package router

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"

	"github.com/gotunnel/pkg/auth"
	"github.com/gotunnel/pkg/security"
)

// AuthHandler 认证处理器
type AuthHandler struct {
	username string
	password string
	jwtAuth  *auth.JWTAuth
}

// NewAuthHandler 创建认证处理器
func NewAuthHandler(username, password string, jwtAuth *auth.JWTAuth) *AuthHandler {
	return &AuthHandler{
		username: username,
		password: password,
		jwtAuth:  jwtAuth,
	}
}

// RegisterAuthRoutes 注册认证路由
func RegisterAuthRoutes(r *Router, h *AuthHandler) {
	r.HandleFunc("/api/auth/login", h.handleLogin)
	r.HandleFunc("/api/auth/check", h.handleCheck)
}

// handleLogin 处理登录请求
func (h *AuthHandler) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	// 验证用户名密码
	userMatch := subtle.ConstantTimeCompare([]byte(req.Username), []byte(h.username)) == 1
	passMatch := subtle.ConstantTimeCompare([]byte(req.Password), []byte(h.password)) == 1

	if !userMatch || !passMatch {
		security.LogWebLogin(r.RemoteAddr, req.Username, false)
		http.Error(w, `{"error":"invalid credentials"}`, http.StatusUnauthorized)
		return
	}

	// 生成 token
	token, err := h.jwtAuth.GenerateToken(req.Username)
	if err != nil {
		http.Error(w, `{"error":"failed to generate token"}`, http.StatusInternalServerError)
		return
	}

	security.LogWebLogin(r.RemoteAddr, req.Username, true)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"token": token,
	})
}

// handleCheck 检查 token 是否有效
func (h *AuthHandler) handleCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	// 从 Authorization header 获取 token
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		http.Error(w, `{"error":"missing authorization header"}`, http.StatusUnauthorized)
		return
	}

	// 解析 Bearer token
	const prefix = "Bearer "
	if len(authHeader) < len(prefix) || authHeader[:len(prefix)] != prefix {
		http.Error(w, `{"error":"invalid authorization format"}`, http.StatusUnauthorized)
		return
	}
	tokenStr := authHeader[len(prefix):]

	// 验证 token
	claims, err := h.jwtAuth.ValidateToken(tokenStr)
	if err != nil {
		http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"valid":    true,
		"username": claims.Username,
	})
}
