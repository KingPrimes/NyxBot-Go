// Package auth 提供 Gin 中间件用于 JWT 认证校验。
package auth

import (
	"strings"

	"github.com/gin-gonic/gin"
	"nyxbot-go/internal/response"
)

// Middleware 认证中间件，依赖 JWT Manager 进行令牌校验。
type Middleware struct {
	manager *Manager
}

// NewMiddleware 创建认证中间件实例。
func NewMiddleware(manager *Manager) *Middleware {
	return &Middleware{manager: manager}
}

// RequireAuth 返回要求有效 access token 的 Gin 中间件。
func (m *Middleware) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := BearerToken(c.GetHeader("Authorization"))
		if raw == "" {
			response.Unauthorized(c, "未提供认证令牌")
			return
		}

		claims, err := m.manager.ParseAccessToken(raw)
		if err != nil {
			switch err {
			case ErrExpiredToken:
				response.Unauthorized(c, "令牌已过期，请重新登录")
			case ErrBlacklistedToken:
				response.Unauthorized(c, "令牌已被注销")
			default:
				response.Unauthorized(c, "无效的认证令牌")
			}
			return
		}

		c.Set(ClaimsKey, claims)
		c.Next()
	}
}

// AllowExpired 返回允许过期 access token 的中间件（用于 refresh 端点）。
func (m *Middleware) AllowExpired() gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := BearerToken(c.GetHeader("Authorization"))
		if raw == "" {
			response.Unauthorized(c, "未提供认证令牌")
			return
		}

		claims, err := m.manager.ParseAccessTokenAllowExpired(raw)
		if err != nil {
			switch err {
			case ErrBlacklistedToken:
				response.Unauthorized(c, "令牌已被注销")
			default:
				response.Unauthorized(c, "无效的认证令牌")
			}
			return
		}

		if strings.TrimSpace(claims.UserName) == "" {
			response.Unauthorized(c, "无效的认证令牌")
			return
		}

		c.Set(ClaimsKey, claims)
		c.Next()
	}
}
