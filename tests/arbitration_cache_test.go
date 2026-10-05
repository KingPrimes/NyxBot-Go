// 仲裁缓存可用性判定与坏缓存自愈测试（评审跟进）：
// 覆盖 Init/load 对「未过期但任务类型全空」与「类型非空但会被裁剪」两类坏缓存的拒绝与重拉。
package tests

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"nyxbot-go/internal/warframe"
)

// countingTransport 统计 HTTP 调用次数并返回不可重试的 404，避免测试触网与重试退避等待。
type countingTransport struct{ calls atomic.Int32 }

func (transport *countingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	transport.calls.Add(1)
	return &http.Response{
		StatusCode: http.StatusNotFound,
		Body:       io.NopCloser(strings.NewReader("")),
		Header:     make(http.Header),
	}, nil
}

// useTempWorkdir 切换工作目录到临时目录（仲裁缓存为相对路径 ./data/arbitration），测试结束恢复。
func useTempWorkdir(t *testing.T) {
	t.Helper()
	original, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(original) })
}

// writeArbitrationFile 在临时工作目录写入 Base64 编码的仲裁缓存文件。
func writeArbitrationFile(t *testing.T, list []warframe.Arbitration) {
	t.Helper()
	if err := os.MkdirAll("data", 0o755); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(list)
	if err != nil {
		t.Fatal(err)
	}
	encoded := base64.StdEncoding.EncodeToString(raw)
	if err := os.WriteFile(filepath.Join("data", "arbitration"), []byte(encoded), 0o644); err != nil {
		t.Fatal(err)
	}
}

// arbitrationEntry 构造一条仲裁条目，minutesToExpiry 为相对当前的到期分钟数（负数表示已过期）。
func arbitrationEntry(minutesToExpiry int, taskType string) warframe.Arbitration {
	now := time.Now()
	return warframe.Arbitration{
		ID:         strconv.FormatInt(now.Unix(), 10),
		Activation: now.Add(-time.Hour).Format(time.RFC3339),
		Expiry:     now.Add(time.Duration(minutesToExpiry) * time.Minute).Format(time.RFC3339),
		Node:       "Stöfler (谷神星)",
		Enemy:      "Grineer",
		Type:       taskType,
	}
}

// TestArbitrationInitRestoresUsableFile 可用文件（未过期 + 类型非空）直接恢复，不触网。
func TestArbitrationInitRestoresUsableFile(t *testing.T) {
	useTempWorkdir(t)
	writeArbitrationFile(t, []warframe.Arbitration{arbitrationEntry(60, "防御")})

	transport := &countingTransport{}
	cache := warframe.NewArbitrationCache(&http.Client{Transport: transport})
	if err := cache.Init(); err != nil {
		t.Fatalf("可用文件不应触发重拉: %v", err)
	}
	if got := transport.calls.Load(); got != 0 {
		t.Fatalf("可用文件不应发起网络请求, got %d", got)
	}
	arb, ok := cache.GetArbitration()
	if !ok || arb.Type != "防御" {
		t.Fatalf("应从文件恢复当前仲裁: ok=%v arb=%+v", ok, arb)
	}
}

// TestArbitrationInitRejectsEmptyTypeFile 未过期但类型全空的坏缓存应被拒绝并触发重拉。
func TestArbitrationInitRejectsEmptyTypeFile(t *testing.T) {
	useTempWorkdir(t)
	writeArbitrationFile(t, []warframe.Arbitration{arbitrationEntry(60, "")})

	transport := &countingTransport{}
	cache := warframe.NewArbitrationCache(&http.Client{Transport: transport})
	if err := cache.Init(); err == nil {
		t.Fatal("类型全空的缓存应触发重拉（网络禁用时应返回错误）")
	}
	if got := transport.calls.Load(); got == 0 {
		t.Fatal("坏缓存应发起重拉请求")
	}
}

// TestArbitrationInitRejectsTypeOnlyInPrunedEntries 类型非空的条目会被裁剪时同样视为坏数据。
func TestArbitrationInitRejectsTypeOnlyInPrunedEntries(t *testing.T) {
	useTempWorkdir(t)
	writeArbitrationFile(t, []warframe.Arbitration{
		arbitrationEntry(60, ""),    // 未过期但类型为空（保留集）
		arbitrationEntry(-30, "防御"), // 类型非空但已过期（会被裁剪）
	})

	transport := &countingTransport{}
	cache := warframe.NewArbitrationCache(&http.Client{Transport: transport})
	if err := cache.Init(); err == nil {
		t.Fatal("保留集无可用类型时应触发重拉")
	}
	if got := transport.calls.Load(); got == 0 {
		t.Fatal("坏缓存应发起重拉请求")
	}
}

// TestArbitrationLoadRefetchesBadFileWithoutInit 未调用 Init 时 load 也不应接受坏缓存。
func TestArbitrationLoadRefetchesBadFileWithoutInit(t *testing.T) {
	useTempWorkdir(t)
	writeArbitrationFile(t, []warframe.Arbitration{arbitrationEntry(60, "")})

	transport := &countingTransport{}
	cache := warframe.NewArbitrationCache(&http.Client{Transport: transport})
	if list := cache.GetArbitrationList(); len(list) != 0 {
		t.Fatalf("坏缓存不应产出前瞻列表, got %d", len(list))
	}
	if got := transport.calls.Load(); got == 0 {
		t.Fatal("load 遇到坏缓存应发起重拉请求")
	}
}
