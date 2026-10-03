// Package web 托管内嵌的前端构建产物并提供 SPA 路由兜底（NoRoute 返回 index.html）。
//
// 前端产物在编译期由 resources 包的 go:embed 打进可执行文件（见 resources/assets.go），
// 运行期不读磁盘，因此可执行文件单独拷到任意目录、以任意工作目录启动、或放进容器，
// 都能直接提供 Web 界面。
//
// 目录布局与 NyxBot-WebUI 的 Vite 构建输出一致（该仓库 build/plugins/index.ts 的
// move-index-html 插件会把 resources/static/index.html 移到 resources/templates）：
//
//	resources/static/static/**      JS/CSS 资源，对应 URL 前缀 /static
//	resources/static/favicon.svg    站点图标
//	resources/templates/index.html  SPA 入口
package web

import (
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/gin-gonic/gin"

	"nyxbot-go/internal/logging"
	"nyxbot-go/resources"
)

const (
	// assetsDir /static 前缀对应的内嵌目录（Vite outDir=resources/static、assetsDir=static）。
	assetsDir = "static/static"
)

// indexCandidates SPA 入口 index.html 的内嵌路径候选，按优先级排列。
// templates/index.html 是 NyxBot-WebUI 构建的规范输出；static/index.html 兼容
// 未做 index 移动的构建（例如直接使用 Vite 默认输出布局）。
var indexCandidates = []string{"templates/index.html", "static/index.html"}

// faviconCandidates 需要映射到站点根路径的图标文件（存在才注册路由）。
var faviconCandidates = []string{"static/favicon.ico", "static/favicon.svg"}

// RegisterStaticRoutes 注册静态文件路由和 Vue SPA 兜底路由。
// 所有资源都取自编译期内嵌的前端产物，不依赖运行目录。
func RegisterStaticRoutes(r *gin.Engine) {
	if isDir(assetsDir) {
		sub, err := fs.Sub(resources.Static, assetsDir)
		if err != nil {
			logging.ErrorPack("web", "open embedded assets %s failed: %v", assetsDir, err)
		} else {
			r.StaticFS("/static", http.FS(sub))
		}
	} else {
		logging.WarnPack("web", "embedded %s not found: page will load without JS/CSS, rebuild with the WebUI dist in resources", assetsDir)
	}

	for _, icon := range faviconCandidates {
		if isFile(icon) {
			r.StaticFileFS("/"+path.Base(icon), icon, http.FS(resources.Static))
		}
	}

	if indexPath, ok := findIndexFile(); ok {
		logging.InfoPack("web", "embedded frontend index: %s", indexPath)
	} else {
		logging.WarnPack("web", "embedded frontend index.html not found: rebuild with the WebUI dist in resources")
	}

	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{
				"code": 404,
				"msg":  "api not found",
				"data": nil,
			})
			return
		}

		indexPath, ok := findIndexFile()
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{
				"code": 404,
				"msg":  "frontend build not found, please rebuild the binary with the Vue dist files in resources",
				"data": nil,
			})
			return
		}

		// 直接读出内容返回，不用 c.File/c.FileFromFS：net/http 会把任何以
		// /index.html 结尾的请求路径 301 重定向到 ./，而 SPA 兜底必须原样返回页面。
		index, err := fs.ReadFile(resources.Static, indexPath)
		if err != nil {
			logging.ErrorPack("web", "read embedded %s failed: %v", indexPath, err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"code": 500,
				"msg":  "frontend index read failed",
				"data": nil,
			})
			return
		}

		c.Data(http.StatusOK, "text/html; charset=utf-8", index)
	})
}

// findIndexFile 返回 SPA 入口 index.html 的内嵌路径，未找到时 ok 为 false。
func findIndexFile() (string, bool) {
	for _, candidate := range indexCandidates {
		if isFile(candidate) {
			return candidate, true
		}
	}

	return "", false
}

// isDir 判断内嵌文件系统中是否存在该目录。
func isDir(name string) bool {
	info, err := fs.Stat(resources.Static, name)

	return err == nil && info.IsDir()
}

// isFile 判断内嵌文件系统中是否存在该普通文件。
func isFile(name string) bool {
	info, err := fs.Stat(resources.Static, name)

	return err == nil && !info.IsDir()
}
