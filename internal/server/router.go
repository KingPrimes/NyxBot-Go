// Package server 路由注册与 Gin Engine 初始化。
package server

import (
	"github.com/gin-gonic/gin"
	"nyxbot-go/internal/auth"
	"nyxbot-go/internal/config"
	"nyxbot-go/internal/logging"
	"nyxbot-go/internal/response"
	"nyxbot-go/internal/system"
	"nyxbot-go/internal/web"
)

// NewRouter 创建并配置 Gin Engine，注册认证路由、系统配置路由、API 路由和静态文件托管。
func NewRouter(rt *config.Runtime) *gin.Engine {
	cfg := rt.Config()
	gin.SetMode(cfg.Server.GinMode)
	gin.DebugPrintRouteFunc = func(httpMethod, absolutePath, handlerName string, nuHandlers int) {
		logging.DebugPack("server.router", "ROUTE %s %s %s %d", httpMethod, absolutePath, handlerName, nuHandlers)
	}

	r := gin.New()
	if err := r.SetTrustedProxies(cfg.Server.TrustedProxies); err != nil {
		panic(err)
	}
	r.Use(CORS(), Recovery())
	if cfg.Server.RequestLog {
		r.Use(Logger())
	}

	tokenManager := auth.NewManager(cfg.Auth.JwtSecret)
	authHandler := auth.NewHandler(tokenManager)
	authMiddleware := auth.NewMiddleware(tokenManager)

	registerAuthRoutes(r, authHandler, authMiddleware)
	registerConfigRoutes(r, authMiddleware, rt)
	registerAPIRoutes(r)
	web.RegisterStaticRoutes(r)

	return r
}

// registerAuthRoutes 注册 /auth/* 认证相关路由。
func registerAuthRoutes(r *gin.Engine, h *auth.Handler, mw *auth.Middleware) {
	r.POST("/auth/login", h.Login)
	r.POST("/auth/refreshToken", mw.AllowExpired(), h.RefreshToken)

	authGroup := r.Group("/auth")
	authGroup.Use(mw.RequireAuth())
	{
		authGroup.GET("/info", h.GetUserInfo)
		authGroup.POST("/restorePassword", h.RestorePassword)
		authGroup.POST("/changeUsername", h.ChangeUsername)
		authGroup.POST("/logout", h.Logout)
	}
}

// registerConfigRoutes 注册 /config/* 系统配置路由，全部要求有效 access token。
func registerConfigRoutes(r *gin.Engine, mw *auth.Middleware, rt *config.Runtime) {
	h := system.NewConfigHandler(rt)

	configGroup := r.Group("/config")
	configGroup.Use(mw.RequireAuth())
	{
		configGroup.GET("/loading", h.GetLoading)
		configGroup.POST("/loading", h.SaveLoading)
	}
}

// registerAPIRoutes 注册 /api/* 通用路由（健康检查等）和 SSE 日志流路由。
func registerAPIRoutes(r *gin.Engine) {
	r.GET("/sse/log-now", streamLogs)
	r.GET("/sse/stats", logSSEStats)
	r.POST("/sse/filter/update", updateLogSSEFilter)
	r.POST("/sse/filter/reset", resetLogSSEFilter)

	api := r.Group("/api")
	{
		api.GET("/health", func(c *gin.Context) {
			response.Success(c, gin.H{"status": "ok"})
		})
	}
}
