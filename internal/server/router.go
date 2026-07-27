// Package server 路由注册与 Gin Engine 初始化。
package server

import (
	"github.com/gin-gonic/gin"
	"nyxbot-go/internal/auth"
	botconfig "nyxbot-go/internal/bot"
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
	registerLogRoutes(r, authMiddleware)
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
	configHandler := system.NewConfigHandler(rt)
	botHandler := botconfig.NewHandler(botconfig.DefaultDirectory)

	configGroup := r.Group("/config")
	configGroup.Use(mw.RequireAuth())
	{
		configGroup.GET("/loading", configHandler.GetLoading)
		configGroup.POST("/loading", configHandler.SaveLoading)

		botGroup := configGroup.Group("/bot")
		botGroup.GET("/bots", botHandler.Bots)
		botGroup.GET("/friend/:botUid", botHandler.Friends)
		botGroup.GET("/group/:botUid", botHandler.Groups)
		botGroup.GET("/admin/permissions", botHandler.Permissions)
		botGroup.POST("/admin/list", botHandler.AdminList)
		botGroup.POST("/admin/save", botHandler.AdminSave)
		botGroup.DELETE("/admin/remove/:id", botHandler.AdminRemove)
		botGroup.POST("/white/group/list", botHandler.WhiteGroupList)
		botGroup.POST("/white/group/save", botHandler.WhiteGroupSave)
		botGroup.DELETE("/white/group/remove/:id", botHandler.WhiteGroupRemove)
		botGroup.POST("/white/prove/list", botHandler.WhiteProveList)
		botGroup.POST("/white/prove/save", botHandler.WhiteProveSave)
		botGroup.DELETE("/white/prove/remove/:id", botHandler.WhiteProveRemove)
		botGroup.POST("/black/group/list", botHandler.BlackGroupList)
		botGroup.POST("/black/group/save", botHandler.BlackGroupSave)
		botGroup.DELETE("/black/group/remove/:id", botHandler.BlackGroupRemove)
		botGroup.POST("/black/prove/list", botHandler.BlackProveList)
		botGroup.POST("/black/prove/save", botHandler.BlackProveSave)
		botGroup.DELETE("/black/prove/remove/:id", botHandler.BlackProveRemove)
	}
}

// registerLogRoutes 注册 /log/** 操作日志查询路由和 /api/logs/** 日志搜索/统计/导出路由，
// 全部要求有效 access token（对齐 Java 侧后台接口的鉴权语义）。
func registerLogRoutes(r *gin.Engine, mw *auth.Middleware) {
	h := system.NewLogHandler()

	logGroup := r.Group("/log")
	logGroup.Use(mw.RequireAuth())
	{
		logGroup.GET("/codes", h.Codes)
		logGroup.GET("/titles", h.Titles)
		logGroup.POST("/list", h.List)
		logGroup.GET("/detail/:id", h.Detail)
	}

	apiLogs := r.Group("/api/logs")
	apiLogs.Use(mw.RequireAuth())
	{
		apiLogs.GET("/search", searchLogs)
		apiLogs.GET("/stats", logStats)
		apiLogs.GET("/export/txt", exportLogsTxt)
		apiLogs.GET("/export/json", exportLogsJSON)
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
