// 仲裁缓存测试：arbys.txt 解析与窗口裁剪、本地文件降级、Reload 落盘与刷新、评级过滤与查表。
// 数据源已由 wf.555590.xyz 的 JSON API 切换为 browse.wf/arbys.txt（逐行 "unix秒,节点ID"）。
package tests

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"nyxbot-go/internal/database"
	modelwarframe "nyxbot-go/internal/model/warframe"
	"nyxbot-go/internal/warframe"
)

// stubTransport 按固定状态码与响应体响应 HTTP 请求（测试用，不触网）。
// 失败用例统一用 404：非重试状态码，避免测试等待指数退避。
type stubTransport struct {
	status int
	body   string
	calls  atomic.Int32
}

func (transport *stubTransport) RoundTrip(*http.Request) (*http.Response, error) {
	transport.calls.Add(1)
	return &http.Response{
		StatusCode: transport.status,
		Body:       io.NopCloser(strings.NewReader(transport.body)),
		Header:     make(http.Header),
	}, nil
}

// useTempWorkdir 切换工作目录到临时目录（仲裁缓存为相对路径 ./data/arbys.txt），测试结束恢复。
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

// writeArbysFile 在临时工作目录写入 arbys.txt 原文。
func writeArbysFile(t *testing.T, content string) {
	t.Helper()
	if err := os.MkdirAll("data", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join("data", "arbys.txt"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// arbysLine 生成一行 "unix秒,节点ID"，offset 为相对当前的开始时间偏移。
func arbysLine(offset time.Duration, nodeKey string) string {
	return fmt.Sprintf("%d,%s", time.Now().Add(offset).Unix(), nodeKey)
}

// setupArbitrationTierDB 迁移 nodes 与 arbitration_tier 表并置入 global DB。
func setupArbitrationTierDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "arbitration_tier.db")), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&modelwarframe.Nodes{}, &modelwarframe.ArbitrationTier{}); err != nil {
		t.Fatal(err)
	}
	database.DB = db
	t.Cleanup(func() { closeGlobalDB(t) })
}

// TestArbitrationParseWindowAndMalformedLines 解析本地 arbys.txt：畸形行（空行/缺列/时间戳非数字）
// 直接跳过，已过期行不入结果，进行中的仲裁可被取到且到期时间恒为开始时间 + 3600 秒。
func TestArbitrationParseWindowAndMalformedLines(t *testing.T) {
	useTempWorkdir(t)
	writeArbysFile(t, strings.Join([]string{
		"",
		"   ",
		"not-a-number,SolNode1",                // 时间戳非数字
		"12345",                                // 缺列
		"12345,",                               // 节点为空
		",SolNode2",                            // 时间戳为空
		arbysLine(-2*time.Hour, "ExpiredNode"), // 已过期
		arbysLine(40*24*time.Hour, "FarNode"),  // 超出 30 天窗口
		arbysLine(-30*time.Minute, "CurrentNode"), // 进行中
		arbysLine(2*time.Hour, "FutureNode"),      // 窗口内的未来场次
	}, "\n"))

	cache := warframe.NewArbitrationCache(nil)
	current, ok := cache.GetArbitration()
	if !ok {
		t.Fatal("应能从本地文件解析出进行中的仲裁")
	}
	if current.NodeKey != "CurrentNode" {
		t.Fatalf("进行中的仲裁应为 CurrentNode, got %q", current.NodeKey)
	}
	if current.Expiry-current.Activation != 3600 {
		t.Fatalf("到期时间应为开始时间 + 3600s, got %+v", current)
	}
}

// TestArbitrationLoadFallsBackToLocalFile 消费路径只读本地文件：即使直连不可用也不发起网络请求。
func TestArbitrationLoadFallsBackToLocalFile(t *testing.T) {
	useTempWorkdir(t)
	writeArbysFile(t, arbysLine(-10*time.Minute, "SolNode64"))

	transport := &stubTransport{status: http.StatusNotFound}
	cache := warframe.NewArbitrationCache(&http.Client{Transport: transport})
	if _, ok := cache.GetArbitration(); !ok {
		t.Fatal("应能从本地文件恢复当前仲裁")
	}
	if got := transport.calls.Load(); got != 0 {
		t.Fatalf("消费路径不应触网, got %d 次请求", got)
	}
}

// TestArbitrationLoadWithoutLocalFile 本地文件缺失且未拉取时返回空结果而非报错。
func TestArbitrationLoadWithoutLocalFile(t *testing.T) {
	useTempWorkdir(t)

	transport := &stubTransport{status: http.StatusNotFound}
	cache := warframe.NewArbitrationCache(&http.Client{Transport: transport})
	if _, ok := cache.GetArbitration(); ok {
		t.Fatal("无本地文件时不应产出仲裁")
	}
	if list := cache.GetArbitrationList(); len(list) != 0 {
		t.Fatalf("无本地文件时前瞻列表应为空, got %d", len(list))
	}
	if got := transport.calls.Load(); got != 0 {
		t.Fatalf("消费路径不应触网, got %d 次请求", got)
	}
}

// TestArbitrationReloadPersistsAndRefreshes Reload 应拉取远程原文、原样覆写本地文件并刷新内存缓存。
func TestArbitrationReloadPersistsAndRefreshes(t *testing.T) {
	useTempWorkdir(t)
	body := arbysLine(-5*time.Minute, "SolNode64")
	transport := &stubTransport{status: http.StatusOK, body: body}
	cache := warframe.NewArbitrationCache(&http.Client{Transport: transport})

	if err := cache.Reload(); err != nil {
		t.Fatalf("Reload 失败: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join("data", "arbys.txt"))
	if err != nil {
		t.Fatalf("Reload 应落盘 arbys.txt: %v", err)
	}
	if strings.TrimSpace(string(raw)) != body {
		t.Fatalf("落盘内容应为远程原文, got %q", string(raw))
	}
	if arb, ok := cache.GetArbitration(); !ok || arb.NodeKey != "SolNode64" {
		t.Fatalf("Reload 后应刷新内存缓存: ok=%v arb=%+v", ok, arb)
	}
}

// TestArbitrationReloadRejectsNonArbysBody 响应非 arbys.txt 格式（如 CDN 错误页）时视为拉取失败，不落盘。
func TestArbitrationReloadRejectsNonArbysBody(t *testing.T) {
	useTempWorkdir(t)
	transport := &stubTransport{status: http.StatusOK, body: "<html>not arbys</html>"}
	cache := warframe.NewArbitrationCache(&http.Client{Transport: transport})

	if err := cache.Reload(); err == nil {
		t.Fatal("非 arbys.txt 内容应视为拉取失败")
	}
	if _, err := os.Stat(filepath.Join("data", "arbys.txt")); !os.IsNotExist(err) {
		t.Fatalf("失败不应写入本地文件, stat err=%v", err)
	}
}

// TestArbitrationReloadKeepsLocalFileOnFailure 拉取失败时不覆写已有的本地文件。
func TestArbitrationReloadKeepsLocalFileOnFailure(t *testing.T) {
	useTempWorkdir(t)
	existing := arbysLine(-5*time.Minute, "SolNode64")
	writeArbysFile(t, existing)

	transport := &stubTransport{status: http.StatusNotFound}
	cache := warframe.NewArbitrationCache(&http.Client{Transport: transport})
	if err := cache.Reload(); err == nil {
		t.Fatal("拉取失败时 Reload 应返回错误")
	}
	raw, err := os.ReadFile(filepath.Join("data", "arbys.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(raw)) != existing {
		t.Fatalf("失败不应覆写本地文件, got %q", string(raw))
	}
}

// TestArbitrationListFiltersByTier 前瞻列表按评级阈值过滤：S/A/B 入选，C/D 与无分级被排除；
// 已开始的场次不入列表；超出 30 天窗口的场次不入列表。
func TestArbitrationListFiltersByTier(t *testing.T) {
	setupArbitrationTierDB(t)
	useTempWorkdir(t)
	if err := database.DB.Create(&[]modelwarframe.ArbitrationTier{
		{Key: "NodeS", Tier: "S"},
		{Key: "NodeA", Tier: "A"},
		{Key: "NodeB", Tier: "B"},
		{Key: "NodeC", Tier: "C"},
		{Key: "NodeD", Tier: "D"},
		{Key: "NodeStarted", Tier: "S"},
		{Key: "NodeFar", Tier: "S"},
		{Key: "NodeJustInWindow", Tier: "S"},
	}).Error; err != nil {
		t.Fatal(err)
	}
	writeArbysFile(t, strings.Join([]string{
		arbysLine(1*time.Hour, "NodeS"),
		arbysLine(2*time.Hour, "NodeA"),
		arbysLine(3*time.Hour, "NodeB"),
		arbysLine(4*time.Hour, "NodeC"),
		arbysLine(5*time.Hour, "NodeD"),
		arbysLine(6*time.Hour, "NodeNoTier"),           // 无分级 → 视为 F
		arbysLine(-10*time.Minute, "NodeStarted"),      // 已开始 → 不入前瞻列表
		arbysLine(29*24*time.Hour, "NodeJustInWindow"), // 窗口边界内
		arbysLine(40*24*time.Hour, "NodeFar"),          // 超出窗口
	}, "\n"))

	cache := warframe.NewArbitrationCache(nil)
	list := cache.GetArbitrationList()
	want := []string{"NodeS", "NodeA", "NodeB", "NodeJustInWindow"}
	if len(list) != len(want) {
		t.Fatalf("应仅 S/A/B 三级且窗口内入选, got %d 条: %+v", len(list), list)
	}
	for index, key := range want {
		if list[index].NodeKey != key {
			t.Errorf("第 %d 条应为 %s, got %s", index, key, list[index].NodeKey)
		}
	}
}

// TestArbitrationTierRankBoundary 评级序数与阈值判定边界：S/A/B 值得参与，C/D/F/空/未知不值得。
func TestArbitrationTierRankBoundary(t *testing.T) {
	cases := []struct {
		tier  string
		rank  int
		worth bool
	}{
		{"S", 5, true},
		{"A", 4, true},
		{"B", 3, true},
		{"C", 2, false},
		{"D", 1, false},
		{"F", 0, false},
		{"", 0, false},
		{"unknown", 0, false},
		{" s ", 5, true},
	}
	for _, tc := range cases {
		if got := warframe.TierRank(tc.tier); got != tc.rank {
			t.Errorf("TierRank(%q)=%d, want %d", tc.tier, got, tc.rank)
		}
		if got := warframe.IsWorthTier(tc.tier); got != tc.worth {
			t.Errorf("IsWorthTier(%q)=%v, want %v", tc.tier, got, tc.worth)
		}
	}
}

// TestArbitrationTierLookupWithoutDatabase 数据库不可用时评级查询回退 F。
func TestArbitrationTierLookupWithoutDatabase(t *testing.T) {
	saved := database.DB
	database.DB = nil
	t.Cleanup(func() { database.DB = saved })

	if got := warframe.ArbitrationTier("SolNode450"); got != "F" {
		t.Fatalf("数据库不可用时应回退 F, got %q", got)
	}
}

// TestArbitrationTierLookup 评级查表：命中返回表内评级（大小写归一），未收录与空节点回退 F。
func TestArbitrationTierLookup(t *testing.T) {
	setupArbitrationTierDB(t)
	if err := database.DB.Create(&[]modelwarframe.ArbitrationTier{
		{Key: "SolNode450", Tier: "S"},
		{Key: "SolNode106", Tier: "b"},
	}).Error; err != nil {
		t.Fatal(err)
	}

	if got := warframe.ArbitrationTier("SolNode450"); got != "S" {
		t.Fatalf("命中节点应返回表内评级, got %q", got)
	}
	if got := warframe.ArbitrationTier("SolNode106"); got != "B" {
		t.Fatalf("评级应大小写归一, got %q", got)
	}
	if got := warframe.ArbitrationTier("SolNode999"); got != "F" {
		t.Fatalf("未收录节点应回退 F, got %q", got)
	}
	if got := warframe.ArbitrationTier(""); got != "F" {
		t.Fatalf("空节点应回退 F, got %q", got)
	}
}
