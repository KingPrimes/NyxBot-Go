package server

import (
	"github.com/gin-gonic/gin"
	"nyxbot-go/internal/config"
	"nyxbot-go/internal/logging"
	"nyxbot-go/internal/web"
)

func NewRouter(cfg config.Config) *gin.Engine {
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

	registerAPIRoutes(r)
	web.RegisterStaticRoutes(r, cfg.Server.StaticDir)

	return r
}

func registerAPIRoutes(r *gin.Engine) {
	r.GET("/sse/log-now", streamLogs)
	r.GET("/sse/stats", logSSEStats)
	r.POST("/sse/filter/update", updateLogSSEFilter)
	r.POST("/sse/filter/reset", resetLogSSEFilter)

	api := r.Group("/api")
	{
		api.GET("/health", func(c *gin.Context) {
			Success(c, gin.H{"status": "ok"})
		})
	}
}
