package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"nyxbot-go/internal/database"
	"nyxbot-go/internal/enum/nyxbot"
	"nyxbot-go/internal/logging"
	"nyxbot-go/internal/model/system"
)

// setupLogDB 在临时目录初始化仅含 log_info 表的测试数据库，并注入全局 database.DB。
func setupLogDB(t *testing.T) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "test.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&system.LogInfo{}); err != nil {
		t.Fatal(err)
	}
	database.DB = db
	t.Cleanup(func() { closeGlobalDB(t) })
}

// seedLogInfos 写入三条标题/指令/群组各异的操作日志。
func seedLogInfos(t *testing.T) []system.LogInfo {
	t.Helper()

	rows := []system.LogInfo{
		{Title: string(nyxbot.LogTitlePlugin), Code: "帮助", GroupUID: 1001, BotUID: 2001, UserUID: 3001, RunTime: 12, Status: 1, LogTime: time.Now()},
		{Title: string(nyxbot.LogTitlePlugin), Code: "警报", GroupUID: 1002, BotUID: 2001, UserUID: 3002, RunTime: 34, Status: 1, LogTime: time.Now()},
		{Title: string(nyxbot.LogTitleController), Code: "帮助", GroupUID: 1003, BotUID: 2001, UserUID: 3003, RunTime: 56, Status: 0, LogTime: time.Now()},
	}
	if err := database.DB.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	return rows
}

// TestLogCodesRequiresAuth 验证未携带 token 访问 GET /log/codes 返回 401。
func TestLogCodesRequiresAuth(t *testing.T) {
	r, _ := setupConfigRouter(t)

	recorder := doRequest(t, r, http.MethodGet, "/log/codes", "", "")
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected HTTP 401, got %d", recorder.Code)
	}
}

// TestLogCodesReturnsOptions 验证 /log/codes 返回与指令声明顺序一致、取首别名的下拉选项。
func TestLogCodesReturnsOptions(t *testing.T) {
	r, rt := setupConfigRouter(t)

	recorder := doRequest(t, r, http.MethodGet, "/log/codes", issueToken(t, rt), "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d", recorder.Code)
	}

	var resp struct {
		Code int `json:"code"`
		Data []struct {
			Label string `json:"label"`
			Value string `json:"value"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Data) != len(nyxbot.CodesOrder) {
		t.Fatalf("expected %d options, got %d", len(nyxbot.CodesOrder), len(resp.Data))
	}
	// 首个选项对应 CmdHelp：Comm 为 ^(帮助|指令|...)$，首别名应为“帮助”（不含正则元字符）
	if resp.Data[0].Label != "帮助" || resp.Data[0].Value != "帮助" {
		t.Errorf("first option mismatch: %+v", resp.Data[0])
	}
	for _, opt := range resp.Data {
		if opt.Label == "" || strings.ContainsAny(opt.Label, "^$()") {
			t.Errorf("option label contains regex meta or empty: %q", opt.Label)
		}
	}
}

// TestLogTitlesReturnsOptions 验证 /log/titles 返回中文 label + 枚举名 value 的三个选项。
func TestLogTitlesReturnsOptions(t *testing.T) {
	r, rt := setupConfigRouter(t)

	recorder := doRequest(t, r, http.MethodGet, "/log/titles", issueToken(t, rt), "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d", recorder.Code)
	}

	var resp struct {
		Data []struct {
			Label string `json:"label"`
			Value string `json:"value"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Data) != 3 {
		t.Fatalf("expected 3 title options, got %d", len(resp.Data))
	}
	if resp.Data[1].Label != "插件" || resp.Data[1].Value != "PLUGIN" {
		t.Errorf("PLUGIN option mismatch: %+v", resp.Data[1])
	}
}

// TestLogListFiltersAndPages 验证 /log/list 的无过滤全量、等值过滤与分页语义。
func TestLogListFiltersAndPages(t *testing.T) {
	setupLogDB(t)
	seedLogInfos(t)
	r, rt := setupConfigRouter(t)
	token := issueToken(t, rt)

	// 无过滤：全量 3 条
	_, data := decodeData(t, doRequest(t, r, http.MethodPost, "/log/list", token, `{"current":1,"size":10}`))
	if data["total"] != float64(3) {
		t.Fatalf("expected total 3, got %v", data["total"])
	}
	records, ok := data["records"].([]any)
	if !ok || len(records) != 3 {
		t.Fatalf("expected 3 records, got %v", data["records"])
	}
	// JSON 字段名为 camelCase（对齐 WebUI Api.SystemLog.LogInfo）
	first, _ := records[0].(map[string]any)
	if _, ok := first["botUid"]; !ok {
		t.Errorf("record missing camelCase field botUid: %v", first)
	}

	// code 等值过滤：帮助 × 2
	_, data = decodeData(t, doRequest(t, r, http.MethodPost, "/log/list", token, `{"current":1,"size":10,"code":"帮助"}`))
	if data["total"] != float64(2) {
		t.Fatalf("code filter: expected total 2, got %v", data["total"])
	}

	// groupUid 等值过滤：1001 × 1
	_, data = decodeData(t, doRequest(t, r, http.MethodPost, "/log/list", token, `{"current":1,"size":10,"groupUid":1001}`))
	if data["total"] != float64(1) {
		t.Fatalf("groupUid filter: expected total 1, got %v", data["total"])
	}

	// title 过滤：CONTROLLER × 1
	_, data = decodeData(t, doRequest(t, r, http.MethodPost, "/log/list", token, `{"current":1,"size":10,"title":"CONTROLLER"}`))
	if data["total"] != float64(1) {
		t.Fatalf("title filter: expected total 1, got %v", data["total"])
	}

	// 分页：size=2 第二页剩 1 条，total 仍为 3
	_, data = decodeData(t, doRequest(t, r, http.MethodPost, "/log/list", token, `{"current":2,"size":2}`))
	if data["total"] != float64(3) || data["size"] != float64(2) || data["current"] != float64(2) {
		t.Fatalf("pagination meta mismatch: %v", data)
	}
	if records, _ := data["records"].([]any); len(records) != 1 {
		t.Fatalf("page 2 with size 2: expected 1 record, got %v", data["records"])
	}
}

// TestLogListRequiresAuth 验证未携带 token 访问 POST /log/list 返回 401。
func TestLogListRequiresAuth(t *testing.T) {
	r, _ := setupConfigRouter(t)

	recorder := doRequest(t, r, http.MethodPost, "/log/list", "", `{"current":1,"size":10}`)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected HTTP 401, got %d", recorder.Code)
	}
}

// TestLogDetail 验证 /log/detail/:id 的命中、未命中（data=null）与非法 ID（400）。
func TestLogDetail(t *testing.T) {
	setupLogDB(t)
	rows := seedLogInfos(t)
	r, rt := setupConfigRouter(t)
	token := issueToken(t, rt)

	_, data := decodeData(t, doRequest(t, r, http.MethodGet, fmt.Sprintf("/log/detail/%d", rows[0].ID), token, ""))
	if data["code"] != "帮助" || data["groupUid"] != float64(1001) {
		t.Fatalf("detail mismatch: %v", data)
	}
	if _, ok := data["logTime"].(string); !ok || data["logTime"] == "" {
		t.Errorf("logTime should be a non-empty string, got %v", data["logTime"])
	}

	// 不存在：成功响应且 data 为 null（对齐 Java success() 空载语义）
	_, data = decodeData(t, doRequest(t, r, http.MethodGet, "/log/detail/99999", token, ""))
	if data != nil {
		t.Fatalf("missing id should return null data, got %v", data)
	}

	// 非法 ID：400
	recorder := doRequest(t, r, http.MethodGet, "/log/detail/abc", token, "")
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected HTTP 400, got %d", recorder.Code)
	}
}

// TestLogSearchFiltersHistory 验证 /api/logs/search 的关键词、级别与时间范围过滤。
func TestLogSearchFiltersHistory(t *testing.T) {
	r, rt := setupConfigRouter(t)
	token := issueToken(t, rt)

	marker := "logsearch-marker-7f3a9c"
	logging.InfoPack("tests", "%s hello", marker)
	logging.WarnPack("tests", "%s-warn", marker)

	// 关键词命中（消息或包名）
	_, data := decodeData(t, doRequest(t, r, http.MethodGet, "/api/logs/search?keyword="+marker, token, ""))
	if data["total"].(float64) < 2 {
		t.Fatalf("keyword search: expected >= 2 hits, got %v", data["total"])
	}

	// 无命中关键词
	_, data = decodeData(t, doRequest(t, r, http.MethodGet, "/api/logs/search?keyword=nonexistent-zzz", token, ""))
	if data["total"] != float64(0) {
		t.Fatalf("no-hit keyword: expected 0, got %v", data["total"])
	}

	// 级别过滤：WARN 命中 warn 标记，ERROR 不命中
	_, data = decodeData(t, doRequest(t, r, http.MethodGet, "/api/logs/search?keyword="+marker+"&levels=WARN", token, ""))
	if data["total"] != float64(1) {
		t.Fatalf("levels=WARN: expected 1, got %v", data["total"])
	}
	_, data = decodeData(t, doRequest(t, r, http.MethodGet, "/api/logs/search?keyword="+marker+"&levels=ERROR", token, ""))
	if data["total"] != float64(0) {
		t.Fatalf("levels=ERROR: expected 0, got %v", data["total"])
	}

	// 时间范围：未来起点不命中，当前闭区间命中
	future := time.Now().Add(time.Hour).UnixMilli()
	_, data = decodeData(t, doRequest(t, r, http.MethodGet, fmt.Sprintf("/api/logs/search?keyword=%s&startTime=%d", marker, future), token, ""))
	if data["total"] != float64(0) {
		t.Fatalf("future startTime: expected 0, got %v", data["total"])
	}

	// 正则模式（+ 需 URL 编码为 %2B，否则 query 解码为空格）
	_, data = decodeData(t, doRequest(t, r, http.MethodGet, "/api/logs/search?keyword=logsearch-marker-[0-9a-f]%2B&useRegex=true", token, ""))
	if data["total"].(float64) < 2 {
		t.Fatalf("regex search: expected >= 2 hits, got %v", data["total"])
	}
}

// TestLogSearchRequiresAuth 验证未携带 token 访问 /api/logs/search 返回 401。
func TestLogSearchRequiresAuth(t *testing.T) {
	r, _ := setupConfigRouter(t)

	recorder := doRequest(t, r, http.MethodGet, "/api/logs/search", "", "")
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected HTTP 401, got %d", recorder.Code)
	}
}

// TestLogStatsShape 验证 /api/logs/stats 的响应字段与 Java 对齐。
func TestLogStatsShape(t *testing.T) {
	r, rt := setupConfigRouter(t)

	_, data := decodeData(t, doRequest(t, r, http.MethodGet, "/api/logs/stats", issueToken(t, rt), ""))
	if data["maxCacheSize"] != float64(50) {
		t.Errorf("maxCacheSize: expected 50, got %v", data["maxCacheSize"])
	}
	if _, ok := data["total"].(float64); !ok {
		t.Errorf("total should be a number, got %v", data["total"])
	}
	if status, _ := data["cacheStatus"].(string); !strings.HasPrefix(status, "LogCache[") {
		t.Errorf("cacheStatus format mismatch: %q", status)
	}
	if stats, _ := data["statistics"].(string); !strings.HasPrefix(stats, "Total: ") {
		t.Errorf("statistics format mismatch: %q", stats)
	}
	if _, ok := data["levelCounts"].(map[string]any); !ok {
		t.Errorf("levelCounts should be an object, got %v", data["levelCounts"])
	}
}

// TestLogExportTxt 验证 /api/logs/export/txt 的附件响应头与文本格式。
func TestLogExportTxt(t *testing.T) {
	r, rt := setupConfigRouter(t)
	token := issueToken(t, rt)

	marker := "logexport-marker-2b8e1d"
	logging.InfoPack("tests", "%s body", marker)

	recorder := doRequest(t, r, http.MethodGet, "/api/logs/export/txt?keyword="+marker, token, "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d", recorder.Code)
	}
	if cd := recorder.Header().Get("Content-Disposition"); !strings.HasPrefix(cd, "attachment; filename=logs_") {
		t.Errorf("Content-Disposition mismatch: %q", cd)
	}
	body := recorder.Body.String()
	for _, want := range []string{"# 日志导出", "# 总条数: 1", marker} {
		if !strings.Contains(body, want) {
			t.Errorf("export body missing %q:\n%s", want, body)
		}
	}
}

// TestLogExportJSON 验证 /api/logs/export/json 的附件响应头与可解析的载荷结构。
func TestLogExportJSON(t *testing.T) {
	r, rt := setupConfigRouter(t)
	token := issueToken(t, rt)

	marker := "logexport-marker-9c4f2a"
	logging.InfoPack("tests", "%s body", marker)

	recorder := doRequest(t, r, http.MethodGet, "/api/logs/export/json?keyword="+marker, token, "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d", recorder.Code)
	}
	if cd := recorder.Header().Get("Content-Disposition"); !strings.HasPrefix(cd, "attachment; filename=logs_") {
		t.Errorf("Content-Disposition mismatch: %q", cd)
	}

	var payload struct {
		ExportTime int64 `json:"exportTime"`
		Total      int   `json:"total"`
		Logs       []struct {
			Live string `json:"live"`
			Pack string `json:"pack"`
			Log  string `json:"log"`
		} `json:"logs"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("export body is not valid JSON: %v", err)
	}
	if payload.Total != 1 || len(payload.Logs) != 1 || payload.ExportTime <= 0 {
		t.Fatalf("export payload mismatch: %+v", payload)
	}
	if !strings.Contains(payload.Logs[0].Log, marker) {
		t.Errorf("exported log missing marker: %+v", payload.Logs[0])
	}
}
