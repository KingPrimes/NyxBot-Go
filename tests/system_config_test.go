// Package tests 黑盒测试包：所有测试代码集中于此，仅调用各模块的导出 API，
// 通过显式临时路径与 httptest 全路由验证配置、数据库、HTTP 接口等行为。
package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
	"nyxbot-go/internal/auth"
	"nyxbot-go/internal/config"
	"nyxbot-go/internal/server"
)

// setupConfigRouter 在临时目录中初始化运行时配置与完整路由，避免污染仓库根目录的 config.yaml。
func setupConfigRouter(t *testing.T) (*gin.Engine, *config.Runtime) {
	t.Helper()

	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	rt := config.NewRuntime(config.LoadFrom(cfgPath), cfgPath)

	return server.NewRouter(rt), rt
}

// issueToken 使用运行时配置中的 JWT 密钥为测试用户签发 access token。
func issueToken(t *testing.T, rt *config.Runtime) string {
	t.Helper()

	token, err := auth.NewManager(rt.Config().Auth.JwtSecret).GenerateAccessToken(1, "admin")
	if err != nil {
		t.Fatalf("generate access token: %v", err)
	}
	return token
}

// doRequest 发送测试请求；body 非空时自动携带 application/json，token 非空时携带 Bearer 头。
func doRequest(t *testing.T, r *gin.Engine, method, path, token, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	recorder := httptest.NewRecorder()
	r.ServeHTTP(recorder, req)
	return recorder
}

// decodeData 解析统一响应 { code, msg, data }，返回业务码与 data 对象（data 为 null 时返回 nil）。
func decodeData(t *testing.T, recorder *httptest.ResponseRecorder) (int, map[string]any) {
	t.Helper()

	var resp struct {
		Code int            `json:"code"`
		Msg  string         `json:"msg"`
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response body %q: %v", recorder.Body.String(), err)
	}
	return resp.Code, resp.Data
}

// TestGetLoadingConfigRequiresAuth 验证未携带 token 访问 GET /config/loading 返回 401。
func TestGetLoadingConfigRequiresAuth(t *testing.T) {
	r, _ := setupConfigRouter(t)

	recorder := doRequest(t, r, http.MethodGet, "/config/loading", "", "")
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected HTTP 401, got %d, body: %s", recorder.Code, recorder.Body.String())
	}
	if code, _ := decodeData(t, recorder); code != http.StatusUnauthorized {
		t.Fatalf("expected body code 401, got %d", code)
	}
}

// TestGetLoadingConfigReturnsDefaults 验证认证后 GET /config/loading 返回默认配置且字段齐全。
func TestGetLoadingConfigReturnsDefaults(t *testing.T) {
	r, rt := setupConfigRouter(t)

	recorder := doRequest(t, r, http.MethodGet, "/config/loading", issueToken(t, rt), "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d, body: %s", recorder.Code, recorder.Body.String())
	}

	code, data := decodeData(t, recorder)
	if code != 200 {
		t.Fatalf("expected body code 200, got %d", code)
	}

	expects := map[string]any{
		"serverPort":       float64(8080),
		"isServerOrClient": true,
		"wsServerUrl":      "/ws/shiro",
		"wsClientUrl":      "ws://localhost:3001",
		"token":            "",
		"pluginPrefix":     false,
		"pluginName":       "",
	}
	for key, want := range expects {
		if got, ok := data[key]; !ok || got != want {
			t.Fatalf("field %s: expected %v (present=%v), got %v", key, want, ok, got)
		}
	}
}

// TestSaveLoadingConfigMergesAndPersists 验证全量保存后：接口可读回、运行时内存热更新、YAML 落盘。
// 请求体中夹带的旧版代理字段（httpProxy）应被静默忽略且不落盘（ZeroBot 不支持代理配置）。
func TestSaveLoadingConfigMergesAndPersists(t *testing.T) {
	r, rt := setupConfigRouter(t)
	token := issueToken(t, rt)

	body := `{"serverPort":9090,"isServerOrClient":false,"wsClientUrl":"wss://example.com:3001/ws","wsServerUrl":"/ws/onebot","token":"abc123","httpProxy":"http://127.0.0.1:7890","pluginPrefix":true,"pluginName":"draw-core"}`
	recorder := doRequest(t, r, http.MethodPost, "/config/loading", token, body)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d, body: %s", recorder.Code, recorder.Body.String())
	}
	if code, _ := decodeData(t, recorder); code != 200 {
		t.Fatalf("expected body code 200, got %d", code)
	}

	// 接口读回
	_, data := decodeData(t, doRequest(t, r, http.MethodGet, "/config/loading", token, ""))
	expects := map[string]any{
		"serverPort":       float64(9090),
		"isServerOrClient": false,
		"wsClientUrl":      "wss://example.com:3001/ws",
		"wsServerUrl":      "/ws/onebot",
		"token":            "abc123",
		"pluginPrefix":     true,
		"pluginName":       "draw-core",
	}
	for key, want := range expects {
		if got := data[key]; got != want {
			t.Fatalf("field %s: expected %v, got %v", key, want, got)
		}
	}

	// 运行时内存热更新（存储侧为 ZeroBot 参数语义）
	current := rt.Config()
	if current.Server.Port != "9090" {
		t.Fatalf("expected runtime port 9090, got %s", current.Server.Port)
	}
	if current.Bot.Mode != config.BotModeClient {
		t.Fatalf("expected bot mode client, got %s", current.Bot.Mode)
	}
	if !current.Bot.PluginPrefix || current.Bot.AccessToken != "abc123" {
		t.Fatalf("runtime bot config not hot-updated: %+v", current.Bot)
	}

	// YAML 落盘（ZeroBot 语义字段名）
	raw, err := os.ReadFile(rt.Path())
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}
	var saved config.Config
	if err := yaml.Unmarshal(raw, &saved); err != nil {
		t.Fatalf("parse saved config: %v", err)
	}
	if saved.Bot.Mode != config.BotModeClient || saved.Bot.WsServerPath != "/ws/onebot" ||
		saved.Bot.WsClientURL != "wss://example.com:3001/ws" || saved.Bot.AccessToken != "abc123" || saved.Server.Port != "9090" {
		t.Fatalf("saved yaml mismatch: %+v", saved.Bot)
	}
	if strings.Contains(string(raw), "http_proxy") || strings.Contains(string(raw), "httpProxy") {
		t.Fatalf("legacy proxy field should not be persisted, yaml:\n%s", string(raw))
	}
}

// TestSaveLoadingConfigPartialUpdateKeepsOthers 验证只提交部分字段时其余字段保留原值（merge 语义）。
func TestSaveLoadingConfigPartialUpdateKeepsOthers(t *testing.T) {
	r, rt := setupConfigRouter(t)
	token := issueToken(t, rt)

	full := `{"serverPort":9091,"wsClientUrl":"ws://192.168.1.2:3001","token":"keep-me"}`
	if rec := doRequest(t, r, http.MethodPost, "/config/loading", token, full); rec.Code != http.StatusOK {
		t.Fatalf("setup save failed: %d, body: %s", rec.Code, rec.Body.String())
	}

	recorder := doRequest(t, r, http.MethodPost, "/config/loading", token, `{"pluginPrefix":true}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d, body: %s", recorder.Code, recorder.Body.String())
	}

	_, data := decodeData(t, doRequest(t, r, http.MethodGet, "/config/loading", token, ""))
	if data["pluginPrefix"] != true {
		t.Fatalf("expected pluginPrefix true, got %v", data["pluginPrefix"])
	}
	if data["serverPort"] != float64(9091) || data["wsClientUrl"] != "ws://192.168.1.2:3001" || data["token"] != "keep-me" {
		t.Fatalf("untouched fields should be kept, got: %v", data)
	}
}

// TestSaveLoadingConfigRejectsInvalidServerURL 验证非法 wsServerUrl 返回 500 且配置不被修改。
func TestSaveLoadingConfigRejectsInvalidServerURL(t *testing.T) {
	r, rt := setupConfigRouter(t)
	token := issueToken(t, rt)

	recorder := doRequest(t, r, http.MethodPost, "/config/loading", token, `{"wsServerUrl":"ws://bad"}`)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected HTTP 500, got %d, body: %s", recorder.Code, recorder.Body.String())
	}

	_, data := decodeData(t, doRequest(t, r, http.MethodGet, "/config/loading", token, ""))
	if data["wsServerUrl"] != "/ws/shiro" {
		t.Fatalf("wsServerUrl should stay default after rejected save, got %v", data["wsServerUrl"])
	}
}

// TestSaveLoadingConfigRejectsInvalidClientURL 验证非法 wsClientUrl 返回 500。
func TestSaveLoadingConfigRejectsInvalidClientURL(t *testing.T) {
	r, rt := setupConfigRouter(t)
	token := issueToken(t, rt)

	recorder := doRequest(t, r, http.MethodPost, "/config/loading", token, `{"wsClientUrl":"/not-a-url"}`)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected HTTP 500, got %d, body: %s", recorder.Code, recorder.Body.String())
	}

	_, data := decodeData(t, doRequest(t, r, http.MethodGet, "/config/loading", token, ""))
	if data["wsClientUrl"] != "ws://localhost:3001" {
		t.Fatalf("wsClientUrl should stay default after rejected save, got %v", data["wsClientUrl"])
	}
}

// TestSaveLoadingConfigRejectsInvalidPort 验证端口超出 1-65535 范围返回 500。
func TestSaveLoadingConfigRejectsInvalidPort(t *testing.T) {
	r, rt := setupConfigRouter(t)
	token := issueToken(t, rt)

	for _, body := range []string{`{"serverPort":0}`, `{"serverPort":70000}`} {
		recorder := doRequest(t, r, http.MethodPost, "/config/loading", token, body)
		if recorder.Code != http.StatusInternalServerError {
			t.Fatalf("body %s: expected HTTP 500, got %d, body: %s", body, recorder.Code, recorder.Body.String())
		}
	}

	if port := rt.Config().Server.Port; port != "8080" {
		t.Fatalf("port should stay 8080 after rejected saves, got %s", port)
	}
}

// TestSaveLoadingConfigRequiresAuth 验证未携带 token 保存配置返回 401。
func TestSaveLoadingConfigRequiresAuth(t *testing.T) {
	r, _ := setupConfigRouter(t)

	recorder := doRequest(t, r, http.MethodPost, "/config/loading", "", `{"pluginPrefix":true}`)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected HTTP 401, got %d, body: %s", recorder.Code, recorder.Body.String())
	}
}

// TestSaveLoadingConfigRejectsMalformedJSON 验证非法 JSON 请求体返回 400。
func TestSaveLoadingConfigRejectsMalformedJSON(t *testing.T) {
	r, rt := setupConfigRouter(t)

	recorder := doRequest(t, r, http.MethodPost, "/config/loading", issueToken(t, rt), `{invalid`)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected HTTP 400, got %d, body: %s", recorder.Code, recorder.Body.String())
	}
}
