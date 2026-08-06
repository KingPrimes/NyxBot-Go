package tests

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"nyxbot-go/internal/config"
	"nyxbot-go/internal/warframe"
)

// TestWarframeRetryConfig 验证 warframe 重试配置从 config.yaml 读取并注入生效。
func TestWarframeRetryConfig(t *testing.T) {
	// 用临时目录的默认配置（含 warframe 块默认值）
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	rt := config.NewRuntime(config.LoadFrom(cfgPath), cfgPath)
	cfg := rt.Config()

	if cfg.Warframe.HTTPRetryAttempts != 3 {
		t.Fatalf("expected 3 retry attempts, got %d", cfg.Warframe.HTTPRetryAttempts)
	}
	if cfg.Warframe.HTTPRetryBaseWaitSeconds != 1 {
		t.Fatalf("expected 1s base wait, got %d", cfg.Warframe.HTTPRetryBaseWaitSeconds)
	}
	if cfg.Warframe.HTTPRetryTimeoutSeconds != 15 {
		t.Fatalf("expected 15s timeout, got %d", cfg.Warframe.HTTPRetryTimeoutSeconds)
	}

	// 注入配置后，重试次数应生效（服务器始终 500，按配置重试 3 次后失败）
	warframe.SetRetryConfig(cfg.Warframe.HTTPRetryAttempts, cfg.Warframe.HTTPRetryBaseWaitSeconds, cfg.Warframe.HTTPRetryTimeoutSeconds)

	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer server.Close()

	api := warframe.NewMarketAPIWithBaseURL(server.Client(), server.URL)
	_, err := api.FetchItems()
	if err == nil {
		t.Fatal("expected error after all retries")
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts from config, got %d", attempts)
	}
}
