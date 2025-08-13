package auth

import (
	"encoding/json"
	"net/http"
)

// LoginRequest 登录请求
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginResponse 登录响应
type LoginResponse struct {
	Token     string `json:"token"`
	UserID    string `json:"user_id"`
	Role      string `json:"role"`
	ExpiresIn int64  `json:"expires_in"`
}

// RefreshTokenRequest 刷新令牌请求
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// RefreshTokenResponse 刷新令牌响应
type RefreshTokenResponse struct {
	Token     string `json:"token"`
	ExpiresIn int64  `json:"expires_in"`
}

// AuthHandler 认证处理器
type AuthHandler struct {
	jwtAuth *JWTAuth
}

// NewAuthHandler 创建认证处理器
func NewAuthHandler(jwtAuth *JWTAuth) *AuthHandler {
	return &AuthHandler{
		jwtAuth: jwtAuth,
	}
}

// Login 登录处理
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	// 只允许POST请求
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 解析请求体
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// 简单的用户验证（实际应该从数据库或其他存储中验证）
	userID, role, ok := h.validateUser(req.Username, req.Password)
	if !ok {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	// 生成JWT令牌
	token, err := h.jwtAuth.GenerateToken(userID, role)
	if err != nil {
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	// 设置令牌Cookie
	h.jwtAuth.SetTokenCookie(w, token)

	// 返回响应
	response := LoginResponse{
		Token:     token,
		UserID:    userID,
		Role:      role,
		ExpiresIn: int64(h.jwtAuth.config.TokenExpiry.Seconds()),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Logout 登出处理
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	// 清除令牌Cookie
	h.jwtAuth.ClearTokenCookies(w)

	// 返回成功响应
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Logged out successfully"})
}

// RefreshToken 刷新令牌处理
func (h *AuthHandler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	// 只允许POST请求
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 从Cookie中获取刷新令牌
	refreshToken, err := r.Cookie(h.jwtAuth.config.RefreshCookieName)
	if err != nil {
		http.Error(w, "Refresh token not found", http.StatusUnauthorized)
		return
	}

	// 验证刷新令牌
	claims, err := h.jwtAuth.ValidateRefreshToken(refreshToken.Value)
	if err != nil {
		http.Error(w, "Invalid refresh token", http.StatusUnauthorized)
		return
	}

	// 生成新的访问令牌（增加刷新计数）
	newToken, err := h.jwtAuth.GenerateTokenWithRefreshCount(claims.UserID, claims.Role, claims.RefreshCount+1)
	if err != nil {
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	// 设置新的令牌Cookie
	h.jwtAuth.SetTokenCookie(w, newToken)

	// 返回响应
	response := RefreshTokenResponse{
		Token:     newToken,
		ExpiresIn: int64(h.jwtAuth.config.TokenExpiry.Seconds()),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetProfile 获取用户信息
func (h *AuthHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	// 从上下文获取用户信息
	userID, ok := GetUserID(r.Context())
	if !ok {
		http.Error(w, "User not found", http.StatusUnauthorized)
		return
	}

	role, ok := GetRole(r.Context())
	if !ok {
		http.Error(w, "Role not found", http.StatusUnauthorized)
		return
	}

	// 返回用户信息
	response := map[string]string{
		"user_id": userID,
		"role":    role,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// validateUser 验证用户（增强版本，实际应该从数据库验证）
func (h *AuthHandler) validateUser(username, password string) (string, string, bool) {
	// 增强的用户验证逻辑
	// 实际项目中应该从数据库或其他安全存储中验证
	users := map[string]map[string]string{
		"admin": {
			"password": "$2a$10$92IXUNpkjO0rOQ5byMi.Ye4oKoEa3Ro9llC/.og/at2.uheWG/igi", // "secret"
			"role":     "admin",
		},
		"user": {
			"password": "$2a$10$92IXUNpkjO0rOQ5byMi.Ye4oKoEa3Ro9llC/.og/at2.uheWG/igi", // "secret"
			"role":     "user",
		},
	}

	user, exists := users[username]
	if !exists {
		return "", "", false
	}

	// 使用bcrypt验证密码
	if !CheckPassword(password, user["password"]) {
		return "", "", false
	}

	return username, user["role"], true
}

// RegisterRoutes 注册认证路由
func (h *AuthHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/auth/login", h.Login)
	mux.HandleFunc("/api/auth/logout", h.Logout)
	mux.HandleFunc("/api/auth/refresh", h.RefreshToken)
	mux.Handle("/api/auth/profile", h.jwtAuth.RequireAuth(http.HandlerFunc(h.GetProfile)))
}