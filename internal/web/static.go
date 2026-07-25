// Package web 托管前端静态文件并提供 SPA 路由兜底（NoRoute 返回 index.html）。
package web

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

// staticDir 前端静态文件目录。前端 dist 产物在打包时会内嵌进可执行文件，
// 目录路径不开放配置项，统一使用该常量。
const staticDir = "./resources/static"

// RegisterStaticRoutes 注册静态文件路由和 Vue SPA 兜底路由。
// 已构建的前端 dist 目录应存放于 staticDir 下。
func RegisterStaticRoutes(r *gin.Engine) {
	r.Static("/static", filepath.Join(staticDir, "static"))
	r.StaticFile("/favicon.ico", filepath.Join(staticDir, "favicon.ico"))
	r.StaticFile("/favicon.svg", filepath.Join(staticDir, "favicon.svg"))

	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{
				"code": 404,
				"msg":  "api not found",
				"data": nil,
			})
			return
		}

		indexFile := resolveIndexFile()
		if _, err := os.Stat(indexFile); err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"code": 404,
				"msg":  "frontend build not found, please put Vue dist files into resources",
				"data": nil,
			})
			return
		}

		c.File(indexFile)
	})
}

func resolveIndexFile() string {
	indexFile := filepath.Join(staticDir, "index.html")
	if _, err := os.Stat(indexFile); err == nil {
		return indexFile
	}

	return filepath.Join(filepath.Dir(staticDir), "templates", "index.html")
}
