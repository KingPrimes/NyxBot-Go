package web

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

func RegisterStaticRoutes(r *gin.Engine, staticDir string) {
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

		indexFile := resolveIndexFile(staticDir)
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

func resolveIndexFile(staticDir string) string {
	indexFile := filepath.Join(staticDir, "index.html")
	if _, err := os.Stat(indexFile); err == nil {
		return indexFile
	}

	return filepath.Join(filepath.Dir(staticDir), "templates", "index.html")
}
