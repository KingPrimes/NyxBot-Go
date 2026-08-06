// Warframe 数据管理接口测试（阶段 8）：
// 覆盖 /data/warframe/** 分页、过滤、CRUD、update 异步任务、枚举接口与鉴权
package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"nyxbot-go/internal/auth"
	"nyxbot-go/internal/config"
	"nyxbot-go/internal/database"
	modelwarframe "nyxbot-go/internal/model/warframe"
	"nyxbot-go/internal/server"
	"nyxbot-go/internal/warframe"
)

// setupWarframeDataRouter 初始化 Warframe 数据表 + 带 warframe 组件的完整路由。
func setupWarframeDataRouter(t *testing.T) (*gin.Engine, string) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "warframe-data.db")), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&modelwarframe.Alias{},
		&modelwarframe.Ephemera{},
		&modelwarframe.LichSisterWeapon{},
		&modelwarframe.OrdersItem{},
		&modelwarframe.RivenItem{},
		&modelwarframe.RivenTion{},
		&modelwarframe.RivenTionAlias{},
		&modelwarframe.RivenAnalyseTrend{},
		&modelwarframe.StateTranslation{},
		&modelwarframe.Nodes{},
		&modelwarframe.NightWave{},
		&modelwarframe.Relics{},
		&modelwarframe.RelicRewards{},
		&modelwarframe.RewardPool{},
		&modelwarframe.Reward{},
		&modelwarframe.Warframes{},
		&modelwarframe.WarframesAbility{},
		&modelwarframe.Weapons{},
		&modelwarframe.MissionSubscribe{},
		&modelwarframe.MissionSubscribeUser{},
		&modelwarframe.MissionSubscribeUserCheckType{},
	); err != nil {
		t.Fatal(err)
	}
	database.DB = db
	t.Cleanup(func() { closeGlobalDB(t) })

	exporter := warframe.NewExportFilePath(nil, "zh")
	marketAPI := warframe.NewMarketAPI(nil)
	importer := warframe.NewDataImporter(exporter, marketAPI, db)
	updater := warframe.NewDataUpdater(importer)
	dataHandler := warframe.NewDataHandler(importer, updater)

	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	rt := config.NewRuntime(config.LoadFrom(cfgPath), cfgPath)
	r := server.NewRouter(rt, dataHandler, updater)

	token, err := auth.NewManager(rt.Config().Auth.JwtSecret).GenerateAccessToken(1, "admin")
	if err != nil {
		t.Fatal(err)
	}
	return r, token
}

// warpagedData 分页响应结构。
type warpagedData struct {
	Total   int64             `json:"total"`
	Size    int               `json:"size"`
	Current int               `json:"current"`
	Records []json.RawMessage `json:"records"`
}

// decodePage 解析分页响应。
func decodePage(t *testing.T, body []byte) (int, warpagedData) {
	t.Helper()
	var envelope struct {
		Code int          `json:"code"`
		Msg  string       `json:"msg"`
		Data warpagedData `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("decode page response %q: %v", string(body), err)
	}
	return envelope.Code, envelope.Data
}

// TestWarframeDataRoutesRequireAuth 验证未携带 token 访问 /data/warframe/** 返回 401。
func TestWarframeDataRoutesRequireAuth(t *testing.T) {
	r, _ := setupWarframeDataRouter(t)

	postPaths := []string{
		"/data/warframe/alias/list",
		"/data/warframe/market/list",
		"/data/warframe/nodes/list",
	}
	for _, path := range postPaths {
		recorder := doRequest(t, r, http.MethodPost, path, "", `{"current":1,"size":15}`)
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("%s: expected 401, got %d", path, recorder.Code)
		}
	}
	getPaths := []string{
		"/data/warframe/subscribe/sub",
		"/data/warframe/state-translation/types",
	}
	for _, path := range getPaths {
		recorder := doRequest(t, r, http.MethodGet, path, "", "")
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("%s: expected 401, got %d", path, recorder.Code)
		}
	}
}

// TestAliasListPaginationAndFilter 验证 alias 分页与 cn 模糊过滤。
func TestAliasListPaginationAndFilter(t *testing.T) {
	r, token := setupWarframeDataRouter(t)

	// 种子数据
	db := database.DB
	for index, cn := range []string{"战甲", "武器", "守护", "战甲Prime"} {
		db.Create(&modelwarframe.Alias{En: "en" + string(rune('A'+index)), Cn: cn})
	}

	recorder := doRequest(t, r, http.MethodPost, "/data/warframe/alias/list", token, `{"current":1,"size":2}`)
	code, page := decodePage(t, recorder.Body.Bytes())
	if code != 200 || page.Total != 4 || page.Current != 1 || len(page.Records) != 2 {
		t.Fatalf("pagination mismatch: code=%d page=%+v", code, page)
	}

	// cn 模糊过滤：like %战甲%
	recorder = doRequest(t, r, http.MethodPost, "/data/warframe/alias/list", token, `{"current":1,"size":10,"cn":"战甲"}`)
	code, page = decodePage(t, recorder.Body.Bytes())
	if code != 200 || page.Total != 2 {
		t.Fatalf("filter mismatch: code=%d total=%d", code, page.Total)
	}
}

// TestAliasSaveValidation 验证 alias 保存校验：空值、英文格式、中文格式、查重。
func TestAliasSaveValidation(t *testing.T) {
	r, token := setupWarframeDataRouter(t)

	// 空 cn（对齐 Java BaseController.error：HTTP 500 + code 500）
	recorder := doRequest(t, r, http.MethodPost, "/data/warframe/alias/save", token, `{"en":"Prime","cn":""}`)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected HTTP 500 for empty cn, got %d", recorder.Code)
	}

	// 英文非法（含中文）
	recorder = doRequest(t, r, http.MethodPost, "/data/warframe/alias/save", token, `{"en":"武器","cn":"武器"}`)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected HTTP 500 for invalid en, got %d", recorder.Code)
	}

	// 合法保存
	recorder = doRequest(t, r, http.MethodPost, "/data/warframe/alias/save", token, `{"en":"Prime","cn":"战甲"}`)
	if code, _ := decodeData(t, recorder); code != 200 {
		t.Fatalf("expected 200 for valid alias, got %d", code)
	}

	// 查重
	recorder = doRequest(t, r, http.MethodPost, "/data/warframe/alias/save", token, `{"en":"Prime","cn":"战甲"}`)
	if code, _ := decodeData(t, recorder); code != 500 {
		t.Fatalf("expected 500 for duplicate alias, got %d", code)
	}
}

// TestAliasEditAndRemove 验证 alias edit（key=alias）与 remove。
func TestAliasEditAndRemove(t *testing.T) {
	r, token := setupWarframeDataRouter(t)
	db := database.DB

	alias := modelwarframe.Alias{En: "Prime", Cn: "战甲"}
	db.Create(&alias)

	// edit 存在
	recorder := doRequest(t, r, http.MethodGet, "/data/warframe/alias/edit/1", token, "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("edit: expected 200, got %d", recorder.Code)
	}
	var envelope struct {
		Code int `json:"code"`
		Data struct {
			Alias modelwarframe.Alias `json:"alias"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data.Alias.Cn != "战甲" {
		t.Fatalf("edit should return alias key with record: %+v", envelope.Data)
	}

	// remove
	recorder = doRequest(t, r, http.MethodDelete, "/data/warframe/alias/remove/1", token, "")
	if code, _ := decodeData(t, recorder); code != 200 {
		t.Fatalf("remove: expected 200, got %d", code)
	}
	var count int64
	db.Model(&modelwarframe.Alias{}).Count(&count)
	if count != 0 {
		t.Fatalf("alias should be deleted, remaining %d", count)
	}
}

// TestNodesCRUD 验证 nodes 的 save/edit/remove（uniqueName 主键）。
func TestNodesCRUD(t *testing.T) {
	r, token := setupWarframeDataRouter(t)
	db := database.DB

	// save
	recorder := doRequest(t, r, http.MethodPost, "/data/warframe/nodes/save", token,
		`{"uniqueName":"Node1","name":"节点一","systemName":"地球","minEnemyLevel":5,"maxEnemyLevel":10}`)
	if code, _ := decodeData(t, recorder); code != 200 {
		t.Fatalf("save: expected 200, got %d", code)
	}

	// edit 返回 data 键
	recorder = doRequest(t, r, http.MethodGet, "/data/warframe/nodes/edit/Node1", token, "")
	var envelope struct {
		Code int `json:"code"`
		Data struct {
			Data modelwarframe.Nodes `json:"data"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data.Data.Name != "节点一" {
		t.Fatalf("edit should return node: %+v", envelope.Data)
	}

	// list 全量分页
	recorder = doRequest(t, r, http.MethodPost, "/data/warframe/nodes/list", token, `{"current":1,"size":15}`)
	code, page := decodePage(t, recorder.Body.Bytes())
	if code != 200 || page.Total != 1 {
		t.Fatalf("list mismatch: code=%d total=%d", code, page.Total)
	}

	// remove
	recorder = doRequest(t, r, http.MethodDelete, "/data/warframe/nodes/remove/Node1", token, "")
	if code, _ := decodeData(t, recorder); code != 200 {
		t.Fatalf("remove: expected 200, got %d", code)
	}
	var count int64
	db.Model(&modelwarframe.Nodes{}).Count(&count)
	if count != 0 {
		t.Fatalf("node should be deleted, remaining %d", count)
	}
}

// TestMarketListFilter 验证 market 列表 name 模糊过滤。
func TestMarketListFilter(t *testing.T) {
	r, token := setupWarframeDataRouter(t)
	db := database.DB

	db.Create(&modelwarframe.OrdersItem{ID: "1", Name: "Rhino Prime Set", Slug: "rhino_prime_set"})
	db.Create(&modelwarframe.OrdersItem{ID: "2", Name: "Soma Prime", Slug: "soma_prime"})
	db.Create(&modelwarframe.OrdersItem{ID: "3", Name: "Vectis", Slug: "vectis"})

	recorder := doRequest(t, r, http.MethodPost, "/data/warframe/market/list", token, `{"current":1,"size":10,"name":"prime"}`)
	code, page := decodePage(t, recorder.Body.Bytes())
	if code != 200 || page.Total != 2 {
		t.Fatalf("market filter mismatch: code=%d total=%d", code, page.Total)
	}
}

// TestRelicsListExactMatch 验证 relics 列表 name 等值过滤（对齐 Java 等值语义）。
func TestRelicsListExactMatch(t *testing.T) {
	r, token := setupWarframeDataRouter(t)
	db := database.DB

	db.Create(&modelwarframe.Relics{UniqueName: "/R1", Name: "Lith A1"})
	db.Create(&modelwarframe.Relics{UniqueName: "/R2", Name: "Lith A1 Prime"})

	// like 会命中两条，等值只命中一条
	recorder := doRequest(t, r, http.MethodPost, "/data/warframe/relics/list", token, `{"current":1,"size":10,"name":"lith a1"}`)
	code, page := decodePage(t, recorder.Body.Bytes())
	if code != 200 || page.Total != 1 {
		t.Fatalf("relics exact match mismatch: code=%d total=%d", code, page.Total)
	}
}

// TestSubscribeEnumsAndPagination 验证订阅枚举、三级分页与删除。
func TestSubscribeEnumsAndPagination(t *testing.T) {
	r, token := setupWarframeDataRouter(t)
	db := database.DB

	// 枚举
	recorder := doRequest(t, r, http.MethodGet, "/data/warframe/subscribe/sub", token, "")
	var envelope struct {
		Code int             `json:"code"`
		Data []enumOptionTest `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if len(envelope.Data) != 14 {
		t.Fatalf("SubscribeType should have 14 entries, got %d", len(envelope.Data))
	}

	// 三级数据
	group := modelwarframe.MissionSubscribe{GroupName: "测试群", SubBotUID: 10001, SubGroup: 30001}
	db.Create(&group)
	user := modelwarframe.MissionSubscribeUser{SubID: group.ID, UserID: 20001, UserName: "用户A"}
	db.Create(&user)
	db.Create(&modelwarframe.MissionSubscribeUserCheckType{
		SubuID: user.ID, Subscribe: "ALERTS", MissionTypeEnum: "MT_EXTERMINATION", TierNum: 4,
	})

	// 组分页
	recorder = doRequest(t, r, http.MethodPost, "/data/warframe/subscribe/list", token, `{"current":1,"size":10,"subGroup":30001}`)
	code, page := decodePage(t, recorder.Body.Bytes())
	if code != 200 || page.Total != 1 {
		t.Fatalf("subscribe list mismatch: code=%d total=%d", code, page.Total)
	}

	// 用户分页（按组 id）
	recorder = doRequest(t, r, http.MethodPost, "/data/warframe/subscribe/user/list", token, `{"current":1,"size":10,"id":1}`)
	code, page = decodePage(t, recorder.Body.Bytes())
	if code != 200 || page.Total != 1 {
		t.Fatalf("subscribe user list mismatch: code=%d total=%d", code, page.Total)
	}

	// 删除组（级联清理用户）
	recorder = doRequest(t, r, http.MethodDelete, "/data/warframe/subscribe/1", token, "")
	if code, _ := decodeData(t, recorder); code != 200 {
		t.Fatalf("subscribe remove: expected 200, got %d", code)
	}
	var userCount int64
	db.Model(&modelwarframe.MissionSubscribeUser{}).Count(&userCount)
	if userCount != 0 {
		t.Fatalf("subscribe user should cascade delete, remaining %d", userCount)
	}
}

// enumOptionTest 订阅枚举选项结构。
type enumOptionTest struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// TestStateTranslationTypes 验证 state-translation/types 返回全部枚举。
func TestStateTranslationTypes(t *testing.T) {
	r, token := setupWarframeDataRouter(t)

	recorder := doRequest(t, r, http.MethodGet, "/data/warframe/state-translation/types", token, "")
	var envelope struct {
		Code int              `json:"code"`
		Data []enumOptionTest `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if len(envelope.Data) == 0 {
		t.Fatal("state translation types should not be empty")
	}
	foundWarframes := false
	for _, option := range envelope.Data {
		if option.Value == "WARFRAMES" {
			foundWarframes = true
			if option.Label != "战甲" {
				t.Fatalf("WARFRAMES label mismatch: %+v", option)
			}
		}
	}
	if !foundWarframes {
		t.Fatal("WARFRAMES should exist in types")
	}
}

// TestUpdateTriggersAsyncTask 验证 update 接口立即返回且不阻塞（任务异步执行）。
func TestUpdateTriggersAsyncTask(t *testing.T) {
	r, token := setupWarframeDataRouter(t)

	recorder := doRequest(t, r, http.MethodPost, "/data/warframe/alias/update", token, "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("update: expected 200, got %d", recorder.Code)
	}
	var envelope struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Code != 200 {
		t.Fatalf("update business code mismatch: %+v", envelope)
	}
}

// TestDataRefreshSSE 验证 /sse/data-refresh 可建立连接并收到 connected 事件。
func TestDataRefreshSSE(t *testing.T) {
	r, _ := setupWarframeDataRouter(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := httptest.NewRequest(http.MethodGet, "/sse/data-refresh", nil).WithContext(ctx)
	recorder := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		r.ServeHTTP(recorder, req)
		close(done)
	}()
	// SSE 连接会持续，验证首帧后取消连接
	waitFor(t, func() bool { return strings.Contains(recorder.Body.String(), "sessionId") }, "sse should emit connected event")
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("sse handler should exit after request context cancel")
	}
}
