package tests

import (
	"io/fs"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"nyxbot-go/internal/web"
	"nyxbot-go/resources"
)

// staticAssetRef 从 index.html 中提取 /static/ 资源地址，用于验证内嵌资源的可访问性。
var staticAssetRef = regexp.MustCompile(`(?:src|href)="(/static/[^"]+)"`)

// TestWebStaticRoutesEmbedded 校验内嵌前端产物的路由语义：
// SPA 入口能返回、前端路由兜底到 index.html、/api/ 未命中仍是 JSON 404。
func TestWebStaticRoutesEmbedded(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	web.RegisterStaticRoutes(r)

	index := doRequest(t, r, http.MethodGet, "/", "", "")
	if index.Code != http.StatusOK || index.Body.Len() == 0 {
		t.Fatalf("GET / status = %d (len %d), want the embedded index.html", index.Code, index.Body.Len())
	}
	if contentType := index.Header().Get("Content-Type"); !strings.Contains(contentType, "text/html") {
		t.Fatalf("GET / content-type = %q, want text/html", contentType)
	}

	t.Run("spa fallback", func(t *testing.T) {
		recorder := doRequest(t, r, http.MethodGet, "/dashboard", "", "")
		if recorder.Code != http.StatusOK || recorder.Body.String() != index.Body.String() {
			t.Fatalf("GET /dashboard = %d (len %d), want the same index.html as /", recorder.Code, recorder.Body.Len())
		}
	})

	t.Run("api 404", func(t *testing.T) {
		recorder := doRequest(t, r, http.MethodGet, "/api/not-exist", "", "")
		if recorder.Code != http.StatusNotFound || !strings.Contains(recorder.Body.String(), "api not found") {
			t.Fatalf("GET /api/not-exist = %d %q, want 404 with api not found", recorder.Code, recorder.Body.String())
		}
	})

	t.Run("embedded asset", func(t *testing.T) {
		// 干净检出（CI 只跑测试、没有 WebUI 产物）时内嵌资源目录为空，此时无从断言。
		if _, err := fs.Stat(resources.Static, "static/static"); err != nil {
			t.Skip("no embedded JS/CSS assets in this build")
		}

		match := staticAssetRef.FindStringSubmatch(index.Body.String())
		if match == nil {
			t.Skip("embedded index.html references no /static asset")
		}

		recorder := doRequest(t, r, http.MethodGet, match[1], "", "")
		if recorder.Code != http.StatusOK || recorder.Body.Len() == 0 {
			t.Fatalf("GET %s = %d (len %d), want the embedded asset", match[1], recorder.Code, recorder.Body.Len())
		}
	})
}
