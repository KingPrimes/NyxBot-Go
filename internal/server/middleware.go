// Gin 中间件：CORS + 自定义错误恢复 + 请求日志
package server

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"nyxbot-go/internal/logging"
)

// CORS 跨域中间件
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization, X-Requested-With")
		c.Header("Access-Control-Max-Age", "86400")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// Recovery 自定义错误恢复，返回统一响应格式
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				logging.PanicPack("server.recovery", "%v", err)
				c.JSON(http.StatusInternalServerError, Response{
					Code: 500,
					Msg:  "服务器内部错误",
					Data: nil,
				})
				c.Abort()
			}
		}()
		c.Next()
	}
}

// Logger 请求日志中间件
func Logger() gin.HandlerFunc {
	return gin.LoggerWithConfig(gin.LoggerConfig{
		Output: io.Discard,
		Formatter: func(param gin.LogFormatterParams) string {
			logging.HTTP(param.StatusCode, param.Method, param.Path, param.Latency, param.ClientIP)
			return ""
		},
	})
}

func RequestLogLine(status int, method, path string, latency time.Duration, clientIP string) string {
	return fmt.Sprintf("[%d] %s %s | %v | %s", status, method, path, latency, clientIP)
}
