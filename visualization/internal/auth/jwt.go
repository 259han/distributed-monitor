package auth

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWTConfig JWT配置
type JWTConfig struct {
	Secret            string        // JWT密钥
	TokenExpiry       time.Duration // 令牌过期时间
	RefreshExpiry     time.Duration // 刷新令牌过期时间
	CookieSecure      bool          // Cookie是否安全
	CookieHTTPOnly    bool          // Cookie是否仅HTTP
	CookieName        string        // Cookie名称
	RefreshCookieName string        // 刷新令牌Cookie名称
}

// Claims JWT声明
type Claims struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// JWTAuth JWT认证
type JWTAuth struct {
	config JWTConfig
}

// NewJWTAuth 创建JWT认证
func NewJWTAuth(config JWTConfig) *JWTAuth {
	// 设置默认值
	if config.CookieName == "" {
		config.CookieName = "token"
	}
	if config.RefreshCookieName == "" {
		config.RefreshCookieName = "refresh_token"
	}
	if config.TokenExpiry == 0 {
		config.TokenExpiry = 1 * time.Hour
	}
	if config.RefreshExpiry == 0 {
		config.RefreshExpiry = 24 * time.Hour
	}

	return &JWTAuth{
		config: config,
	}
}

// GenerateToken 生成JWT令牌
func (j *JWTAuth) GenerateToken(userID, role string) (string, error) {
	// 创建声明
	claims := &Claims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(j.config.TokenExpiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	// 创建令牌
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// 签名令牌
	return token.SignedString([]byte(j.config.Secret))
}

// GenerateRefreshToken 生成刷新令牌
func (j *JWTAuth) GenerateRefreshToken(userID string) (string, error) {
	// 创建声明
	claims := &Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(j.config.RefreshExpiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	// 创建令牌
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// 签名令牌
	return token.SignedString([]byte(j.config.Secret))
}

// ValidateToken 验证JWT令牌
func (j *JWTAuth) ValidateToken(tokenString string) (*Claims, error) {
	// 解析令牌
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// 验证签名方法
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(j.config.Secret), nil
	})

	if err != nil {
		return nil, err
	}

	// 验证令牌有效性
	if !token.Valid {
		return nil, errors.New("invalid token")
	}

	// 获取声明
	claims, ok := token.Claims.(*Claims)
	if !ok {
		return nil, errors.New("invalid claims")
	}

	return claims, nil
}

// SetTokenCookie 设置令牌Cookie
func (j *JWTAuth) SetTokenCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     j.config.CookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: j.config.CookieHTTPOnly,
		Secure:   j.config.CookieSecure,
		MaxAge:   int(j.config.TokenExpiry.Seconds()),
		SameSite: http.SameSiteLaxMode,
	})
}

// SetRefreshTokenCookie 设置刷新令牌Cookie
func (j *JWTAuth) SetRefreshTokenCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     j.config.RefreshCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: j.config.CookieHTTPOnly,
		Secure:   j.config.CookieSecure,
		MaxAge:   int(j.config.RefreshExpiry.Seconds()),
		SameSite: http.SameSiteLaxMode,
	})
}

// ClearTokenCookies 清除令牌Cookie
func (j *JWTAuth) ClearTokenCookies(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     j.config.CookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: j.config.CookieHTTPOnly,
		Secure:   j.config.CookieSecure,
		MaxAge:   -1,
		SameSite: http.SameSiteLaxMode,
	})

	http.SetCookie(w, &http.Cookie{
		Name:     j.config.RefreshCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: j.config.CookieHTTPOnly,
		Secure:   j.config.CookieSecure,
		MaxAge:   -1,
		SameSite: http.SameSiteLaxMode,
	})
}

// Middleware JWT中间件
func (j *JWTAuth) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 从请求中获取令牌
		tokenString := j.extractToken(r)
		if tokenString == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// 验证令牌
		claims, err := j.ValidateToken(tokenString)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// 将用户信息添加到请求上下文
		ctx := r.Context()
		ctx = WithUserID(ctx, claims.UserID)
		ctx = WithRole(ctx, claims.Role)

		// 继续处理请求
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// extractToken 从请求中提取令牌
func (j *JWTAuth) extractToken(r *http.Request) string {
	// 从Cookie中获取
	cookie, err := r.Cookie(j.config.CookieName)
	if err == nil {
		return cookie.Value
	}

	// 从Authorization头获取
	bearerToken := r.Header.Get("Authorization")
	if len(bearerToken) > 7 && strings.ToUpper(bearerToken[0:7]) == "BEARER " {
		return bearerToken[7:]
	}

	// 从查询参数获取
	return r.URL.Query().Get("token")
}
