// 阶段10 市场类指令数据层黑盒测试：
// 覆盖 /WM 订单过滤与排序、/WR 紫卡拍卖、/CD /XT 玄骸信条拍卖、金/银垃圾杜卡币，
// 以及四个绘图入口的非空输出。全部通过 httptest 注入本地市场服务器，不访问真实 API。
package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"nyxbot-go/internal/database"
	"nyxbot-go/internal/draw"
	"nyxbot-go/internal/enum/drawplugin"
	modelbot "nyxbot-go/internal/model/bot"
	modelsystem "nyxbot-go/internal/model/system"
	modelwarframe "nyxbot-go/internal/model/warframe"
	"nyxbot-go/internal/onebot"
	"nyxbot-go/internal/warframe"
)

// 测试用物品 ID 与 slug（与 httptest 返回的订单 itemId 对应）。
const (
	testOrdersItemID  = "item-loki-set"
	testOrdersSlug    = "loki_prime_set"
	testRivenSlug     = "rubico_prime"
	testLichSlug      = "kuva_bramma"
	testSisterSlug    = "tenet_arca_plasmor"
	testDucatGoldID   = "item-gold-part"
	testDucatSilverID = "item-silver-part"
)

// setupMarketDB 初始化临时 sqlite，迁移市场相关表并置入全局 DB。
func setupMarketDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "market.db")), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&modelwarframe.OrdersItem{},
		&modelwarframe.RivenItem{},
		&modelwarframe.RivenTion{},
		&modelwarframe.RivenTionAlias{},
		&modelwarframe.LichSisterWeapon{},
		&modelwarframe.Alias{},
	); err != nil {
		t.Fatal(err)
	}
	database.DB = db
	t.Cleanup(func() { closeGlobalDB(t) })
}

// seedMarketItems 置入市场本地数据（市场物品、紫卡武器、玄骸信条武器、词条、别名）。
func seedMarketItems(t *testing.T) {
	t.Helper()
	orders := []modelwarframe.OrdersItem{
		{ID: testOrdersItemID, Slug: testOrdersSlug, Name: "loki prime set", MaxRank: 0, Ducats: 45},
		{ID: "item-loki-bp", Slug: "loki_prime_blueprint", Name: "loki prime blueprint", MaxRank: 0},
		{ID: "item-rhino-set", Slug: "rhino_prime_set", Name: "rhino prime set", MaxRank: 0},
		{ID: testDucatGoldID, Slug: "gold_part", Name: "金部件", Ducats: 100},
		{ID: testDucatSilverID, Slug: "silver_part", Name: "银部件", Ducats: 45},
	}
	if err := database.DB.Create(&orders).Error; err != nil {
		t.Fatal(err)
	}
	rivenItems := []modelwarframe.RivenItem{
		{ID: "riven-rubico", Slug: testRivenSlug, Name: "rubico prime"},
		{ID: "riven-rubico-raw", Slug: "rubico", Name: "rubico"},
	}
	if err := database.DB.Create(&rivenItems).Error; err != nil {
		t.Fatal(err)
	}
	weapons := []modelwarframe.LichSisterWeapon{
		{ID: "lich-bramma", Slug: testLichSlug, Name: "kuva bramma"},
		{ID: "sister-plasmor", Slug: testSisterSlug, Name: "tenet arca plasmor"},
	}
	if err := database.DB.Create(&weapons).Error; err != nil {
		t.Fatal(err)
	}
	tions := []modelwarframe.RivenTion{
		{Effect: "暴击几率", URLName: "critical_chance"},
		{Effect: "暴击伤害", URLName: "critical_damage"},
	}
	if err := database.DB.Create(&tions).Error; err != nil {
		t.Fatal(err)
	}
	aliases := []modelwarframe.Alias{
		{En: "rhino prime set", Cn: "犀牛p"},
	}
	if err := database.DB.Create(&aliases).Error; err != nil {
		t.Fatal(err)
	}
}

// marketTestServer 构造覆盖订单/物品详情/拍卖/杜卡币四类端点的本地市场服务器。
// 各返回值均可通过参数覆盖，便于逐项断言。
func marketTestServer(t *testing.T, ordersJSON, auctionsJSON, ducatsJSON string) *warframe.MarketAPI {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasPrefix(r.URL.Path, "/v2/item/"):
			fmt.Fprint(w, `{"data":{"items":[{"id":"`+testOrdersItemID+`","reqMasteryRank":5,"tradingTax":1000}]}}`)
		case strings.HasPrefix(r.URL.Path, "/v2/orders/item/"):
			fmt.Fprint(w, `{"data":`+ordersJSON+`}`)
		case r.URL.Path == "/v1/auctions/search":
			fmt.Fprint(w, auctionsJSON)
		case r.URL.Path == "/v1/tools/ducats":
			fmt.Fprint(w, ducatsJSON)
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	return warframe.NewMarketAPIWithBaseURL(server.Client(), server.URL)
}

// marketOrderJSON 生成单条订单 JSON（价格/类型/物品/用户状态可配）。
func marketOrderJSON(id, orderType string, platinum, rank int, itemID, status string) string {
	payload := map[string]any{
		"id":        id,
		"type":      orderType,
		"platinum":  platinum,
		"quantity":  1,
		"rank":      rank,
		"itemId":    itemID,
		"visible":   true,
		"createdAt": "2026-08-07T10:00:00Z",
		"updatedAt": "2026-08-07T10:00:00Z",
		"user": map[string]any{
			"id":         "u-" + id,
			"ingameName": "卖家" + id,
			"status":     status,
			"platform":   "pc",
		},
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	return string(encoded)
}

// TestQueryMarketOrdersFilterAndSort 验证订单过滤（离线/物品ID/买卖方向/满级）与价格排序截断。
func TestQueryMarketOrdersFilterAndSort(t *testing.T) {
	setupMarketDB(t)
	seedMarketItems(t)

	orders := "[" + strings.Join([]string{
		marketOrderJSON("s10", "sell", 10, 0, testOrdersItemID, "online"),
		marketOrderJSON("s05", "sell", 5, 0, testOrdersItemID, "ingame"),
		marketOrderJSON("s07", "sell", 7, 0, testOrdersItemID, "online"),
		marketOrderJSON("offline", "sell", 1, 0, testOrdersItemID, "offline"),
		marketOrderJSON("wrongitem", "sell", 2, 0, "other-item", "online"),
		marketOrderJSON("buyer", "buy", 3, 0, testOrdersItemID, "online"),
	}, ",") + "]"
	api := marketTestServer(t, orders, `{"payload":{"auctions":[]}}`, `{"payload":{}}`)

	result, err := warframe.QueryMarketOrders(api, "Loki Prime Set", false, false, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Item == nil {
		t.Fatal("应命中本地物品 loki prime set")
	}
	if result.Item.Name != "loki prime set" {
		t.Fatalf("物品名 = %q", result.Item.Name)
	}
	// 段位要求与交易税由 /v2/item/{slug}/set 补全
	if result.Item.ReqMasteryRank == nil || *result.Item.ReqMasteryRank != 5 {
		t.Fatalf("段位要求补全失败: %v", result.Item.ReqMasteryRank)
	}
	if result.Item.TradingTax == nil || *result.Item.TradingTax != 1000 {
		t.Fatalf("交易税补全失败: %v", result.Item.TradingTax)
	}
	// 只保留 sell + 在线 + 物品 ID 命中，并按价格升序
	wantOrder := []string{"s05", "s07", "s10"}
	if len(result.Orders) != len(wantOrder) {
		t.Fatalf("订单数 = %d, 期望 %d (%+v)", len(result.Orders), len(wantOrder), orderIDs(result.Orders))
	}
	for i, want := range wantOrder {
		if result.Orders[i].ID != want {
			t.Fatalf("第 %d 条订单 = %s, 期望 %s (全部 %v)", i, result.Orders[i].ID, want, orderIDs(result.Orders))
		}
	}
}

// TestQueryMarketOrdersBuyerSort 验证买家模式按价格降序排序。
func TestQueryMarketOrdersBuyerSort(t *testing.T) {
	setupMarketDB(t)
	seedMarketItems(t)

	orders := "[" + strings.Join([]string{
		marketOrderJSON("b3", "buy", 3, 0, testOrdersItemID, "online"),
		marketOrderJSON("b9", "buy", 9, 0, testOrdersItemID, "ingame"),
		marketOrderJSON("s1", "sell", 1, 0, testOrdersItemID, "online"),
	}, ",") + "]"
	api := marketTestServer(t, orders, `{"payload":{"auctions":[]}}`, `{"payload":{}}`)

	result, err := warframe.QueryMarketOrders(api, "loki prime set", true, false, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Orders) != 2 {
		t.Fatalf("买家订单数 = %d, 期望 2", len(result.Orders))
	}
	if result.Orders[0].ID != "b9" || result.Orders[1].ID != "b3" {
		t.Fatalf("买家排序错误: %v", orderIDs(result.Orders))
	}
}

// TestQueryMarketOrdersMaxRankFilter 验证满级过滤只保留 rank 等于物品 maxRank 的订单。
func TestQueryMarketOrdersMaxRankFilter(t *testing.T) {
	setupMarketDB(t)
	// maxRank=5 的物品
	if err := database.DB.Create(&modelwarframe.OrdersItem{
		ID: testOrdersItemID, Slug: testOrdersSlug, Name: "primed mod", MaxRank: 5,
	}).Error; err != nil {
		t.Fatal(err)
	}
	orders := "[" + strings.Join([]string{
		marketOrderJSON("r5", "sell", 20, 5, testOrdersItemID, "online"),
		marketOrderJSON("r0", "sell", 5, 0, testOrdersItemID, "online"),
	}, ",") + "]"
	api := marketTestServer(t, orders, `{"payload":{"auctions":[]}}`, `{"payload":{}}`)

	result, err := warframe.QueryMarketOrders(api, "primed mod", false, true, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Orders) != 1 || result.Orders[0].ID != "r5" {
		t.Fatalf("满级过滤失败: %v", orderIDs(result.Orders))
	}
}

// TestQueryMarketOrdersFuzzyName 验证本地模糊名匹配（LIKE 子串命中物品，不走候选列表）。
func TestQueryMarketOrdersFuzzyName(t *testing.T) {
	setupMarketDB(t)
	// 仅保留单个含 "lok" 的条目，避免多命中时结果依赖 id 排序
	if err := database.DB.Create(&modelwarframe.OrdersItem{
		ID: "a-loki-set", Slug: testOrdersSlug, Name: "loki prime set",
	}).Error; err != nil {
		t.Fatal(err)
	}
	api := marketTestServer(t, `[]`, `{"payload":{"auctions":[]}}`, `{"payload":{}}`)

	result, err := warframe.QueryMarketOrders(api, "Lok", false, false, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Item == nil || result.Item.Name != "loki prime set" {
		t.Fatalf("模糊匹配应命中 loki prime set: %+v", result.Item)
	}
}

// TestQueryMarketOrdersNoMatch 验证完全无匹配的名称不命中物品且无同前缀候选。
func TestQueryMarketOrdersNoMatch(t *testing.T) {
	setupMarketDB(t)
	seedMarketItems(t)
	api := marketTestServer(t, `[]`, `{"payload":{"auctions":[]}}`, `{"payload":{}}`)

	result, err := warframe.QueryMarketOrders(api, "不存在的物品名", false, false, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Item != nil {
		t.Fatal("不应命中物品")
	}
	if len(result.PossibleItems) != 0 {
		t.Fatalf("无同前缀物品时候选应为空, 实际 %v", result.PossibleItems)
	}
}

// TestQueryMarketOrdersCandidateList 验证未命中但存在同首字符物品时返回候选列表。
func TestQueryMarketOrdersCandidateList(t *testing.T) {
	setupMarketDB(t)
	// 两条同首字符 q 的条目，保证 "q" 无精确/子串/正则命中（正则尾字符 q 不在名称中）
	if err := database.DB.Create(&modelwarframe.OrdersItem{
		ID: "a-qtop", Slug: "q_top", Name: "q top prime",
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.DB.Create(&modelwarframe.OrdersItem{
		ID: "a-qblueprint", Slug: "q_top_bp", Name: "q top blueprint",
	}).Error; err != nil {
		t.Fatal(err)
	}
	api := marketTestServer(t, `[]`, `{"payload":{"auctions":[]}}`, `{"payload":{}}`)

	// 关键字 "qz" 不是任何名称的子串，且正则 ^qz.*?z.*? 也不命中（名称中无 z），
	// 故精确/别名/正则三层均不命中，只能落到候选列表分支
	result, err := warframe.QueryMarketOrders(api, "qz", false, false, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Item != nil {
		t.Fatalf("不应命中物品: %+v", result.Item)
	}
	if len(result.PossibleItems) != 2 {
		t.Fatalf("候选数 = %d, 期望 2 (%v)", len(result.PossibleItems), result.PossibleItems)
	}
	for _, name := range result.PossibleItems {
		if !strings.HasPrefix(name, "q top") {
			t.Fatalf("候选名称与首字符不符: %q", name)
		}
	}
}

// TestQueryMarketOrdersAliasMatch 验证中文别名替换后可命中物品。
func TestQueryMarketOrdersAliasMatch(t *testing.T) {
	setupMarketDB(t)
	seedMarketItems(t)
	api := marketTestServer(t, `[]`, `{"payload":{"auctions":[]}}`, `{"payload":{}}`)

	result, err := warframe.QueryMarketOrders(api, "犀牛p", false, false, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Item == nil || result.Item.Name != "rhino prime set" {
		t.Fatalf("别名匹配失败: %+v", result.Item)
	}
}

// TestQueryRivenAuctions 验证紫卡拍卖过滤、排序截断与词条回译。
func TestQueryRivenAuctions(t *testing.T) {
	setupMarketDB(t)
	// 只保留一件紫卡武器，确保关键字唯一命中
	if err := database.DB.Create(&modelwarframe.RivenItem{
		ID: "a-rubico-prime", Slug: testRivenSlug, Name: "rubico prime",
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.DB.Create(&modelwarframe.RivenTion{
		Effect: "暴击伤害", URLName: "critical_damage",
	}).Error; err != nil {
		t.Fatal(err)
	}

	auctions := `{"payload":{"auctions":[
		{"id":"a-closed","closed":true,"visible":true,"buyout_price":1,"owner":{"status":"online"},
		 "item":{"type":"riven","weapon_url_name":"rubico_prime","attributes":[{"url_name":"critical_chance","value":100.0,"positive":true}]}},
		{"id":"a-invisible","closed":false,"visible":false,"buyout_price":2,"owner":{"status":"online"},"item":{"type":"riven"}},
		{"id":"a-offline","closed":false,"visible":true,"buyout_price":3,"owner":{"status":"offline"},"item":{"type":"riven"}},
		{"id":"a-expensive","closed":false,"visible":true,"buyout_price":300,"owner":{"status":"online"},"item":{"type":"riven"}},
		{"id":"a-cheap","closed":false,"visible":true,"buyout_price":150,"owner":{"status":"ingame"},
		 "item":{"type":"riven","weapon_url_name":"rubico_prime","attributes":[{"url_name":"critical_damage","value":80.5,"positive":true}]}}
	]}}`
	api := marketTestServer(t, `[]`, auctions, `{"payload":{}}`)

	result, err := warframe.QueryRivenAuctions(api, "rubico prime")
	if err != nil {
		t.Fatal(err)
	}
	if result.DTO == nil {
		t.Fatal("应返回紫卡拍卖 DTO")
	}
	if result.DTO.ItemName != "rubico prime" {
		t.Fatalf("武器名 = %q", result.DTO.ItemName)
	}
	got := result.DTO.Payload.Auctions
	if len(got) != 2 {
		t.Fatalf("拍卖数 = %d, 期望 2", len(got))
	}
	if got[0].ID != "a-cheap" || got[1].ID != "a-expensive" {
		t.Fatalf("排序错误: %s, %s", got[0].ID, got[1].ID)
	}
	// 词条 url_name 应回译为中文效果
	if len(got[0].Item.Attributes) != 1 || got[0].Item.Attributes[0].URLName != "暴击伤害" {
		t.Fatalf("词条回译失败: %+v", got[0].Item.Attributes)
	}
}

// TestQueryRivenAuctionsNoMatch 验证无匹配武器时返回空结果（不 panic）。
func TestQueryRivenAuctionsNoMatch(t *testing.T) {
	setupMarketDB(t)
	seedMarketItems(t)
	api := marketTestServer(t, `[]`, `{"payload":{"auctions":[]}}`, `{"payload":{}}`)

	result, err := warframe.QueryRivenAuctions(api, "绝路")
	if err != nil {
		t.Fatal(err)
	}
	if result.DTO != nil {
		t.Fatal("不应命中武器")
	}
	if len(result.PossibleItems) != 0 {
		t.Fatalf("无同前缀武器时候选应为空, 实际 %v", result.PossibleItems)
	}
}

// TestQueryRivenAuctionsFuzzyName 验证紫卡武器正则模糊匹配（前缀唯一命中）。
func TestQueryRivenAuctionsFuzzyName(t *testing.T) {
	setupMarketDB(t)
	if err := database.DB.Create(&modelwarframe.RivenItem{
		ID: "a-galatine", Slug: "galatine_prime", Name: "galatine prime",
	}).Error; err != nil {
		t.Fatal(err)
	}
	api := marketTestServer(t, `[]`, `{"payload":{"auctions":[]}}`, `{"payload":{}}`)

	// "galatina" → 正则 ^galatina.*?a.*? 命中 "galatine prime"（非其子串，故必走正则分支）
	result, err := warframe.QueryRivenAuctions(api, "galatina")
	if err != nil {
		t.Fatal(err)
	}
	if result.DTO == nil || result.DTO.ItemName != "galatine prime" {
		t.Fatalf("正则模糊匹配应命中 galatine prime: %+v", result.DTO)
	}
}

// TestQueryRivenAuctionsCandidateList 验证未命中但存在同首字符武器时返回候选列表。
func TestQueryRivenAuctionsCandidateList(t *testing.T) {
	setupMarketDB(t)
	if err := database.DB.Create(&modelwarframe.RivenItem{
		ID: "a-qz-prime", Slug: "qz_prime", Name: "qz prime",
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.DB.Create(&modelwarframe.RivenItem{
		ID: "a-qz-raw", Slug: "qz_raw", Name: "qz raw",
	}).Error; err != nil {
		t.Fatal(err)
	}
	api := marketTestServer(t, `[]`, `{"payload":{"auctions":[]}}`, `{"payload":{}}`)

	// 关键字 "qs" 不是任何名称的子串，且正则尾字符 s 不出现在名称中，
	// 故精确/别名/正则三层均不命中，只能落到候选列表分支
	result, err := warframe.QueryRivenAuctions(api, "qs")
	if err != nil {
		t.Fatal(err)
	}
	if result.DTO != nil {
		t.Fatalf("不应命中武器: %+v", result.DTO)
	}
	if len(result.PossibleItems) != 2 {
		t.Fatalf("候选数 = %d, 期望 2 (%v)", len(result.PossibleItems), result.PossibleItems)
	}
}

// TestRivenSearchQueryParams 验证紫卡搜索查询串参数（段位钳制、极性 any、价格升序）。
func TestRivenSearchQueryParams(t *testing.T) {
	setupMarketDB(t)
	seedMarketItems(t)
	var captured string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"payload":{"auctions":[]}}`)
	}))
	defer server.Close()
	api := warframe.NewMarketAPIWithBaseURL(server.Client(), server.URL)

	if _, err := warframe.QueryRivenAuctions(api, "rubico prime-暴击几率-无"); err != nil {
		t.Fatal(err)
	}
	parsed, err := url.ParseQuery(captured)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Get("type") != "riven" {
		t.Fatalf("type = %q", parsed.Get("type"))
	}
	if parsed.Get("weapon_url_name") != testRivenSlug {
		t.Fatalf("weapon_url_name = %q", parsed.Get("weapon_url_name"))
	}
	if parsed.Get("positive_stats") != "critical_chance" {
		t.Fatalf("positive_stats = %q", parsed.Get("positive_stats"))
	}
	if parsed.Get("negative_stats") != "none" {
		t.Fatalf("negative_stats = %q", parsed.Get("negative_stats"))
	}
	if parsed.Get("mastery_rank_min") != "7" || parsed.Get("mastery_rank_max") != "16" {
		t.Fatalf("段位区间 = %s-%s", parsed.Get("mastery_rank_min"), parsed.Get("mastery_rank_max"))
	}
	if parsed.Get("polarity") != "any" || parsed.Get("sort_by") != "price_asc" {
		t.Fatalf("polarity/sort_by = %s/%s", parsed.Get("polarity"), parsed.Get("sort_by"))
	}
	if parsed.Get("buyout_policy") != "" {
		t.Fatalf("buyout_policy 为 any 时不应附加参数, 实际 %q", parsed.Get("buyout_policy"))
	}
}

// TestQueryLichSisterAuctions 验证玄骸/信条拍卖过滤、排序与元素枚举规范化。
func TestQueryLichSisterAuctions(t *testing.T) {
	setupMarketDB(t)
	seedMarketItems(t)

	auctions := `{"payload":{"auctions":[
		{"id":"l-closed","closed":true,"visible":true,"buyout_price":10,"owner":{"status":"online"},"item":{"element":"heat"}},
		{"id":"l-offline","closed":false,"visible":true,"buyout_price":20,"owner":{"status":"offline"},"item":{"element":"heat"}},
		{"id":"l-online","closed":false,"visible":true,"buyout_price":30,"owner":{"status":"online"},
		 "item":{"element":"heat","damage":40,"weapon_url_name":"kuva_bramma"}}
	]}}`
	api := marketTestServer(t, `[]`, auctions, `{"payload":{}}`)

	result, err := warframe.QueryLichSisterAuctions(api, "kuva bramma", warframe.LichSisterLich)
	if err != nil {
		t.Fatal(err)
	}
	if result.DTO == nil || result.DTO.Payload == nil {
		t.Fatal("应返回玄骸拍卖 DTO")
	}
	if result.DTO.Payload.ItemName != "kuva bramma" {
		t.Fatalf("武器名 = %q", result.DTO.Payload.ItemName)
	}
	if len(result.DTO.Payload.Auctions) != 1 {
		t.Fatalf("拍卖数 = %d, 期望 1", len(result.DTO.Payload.Auctions))
	}
	item := result.DTO.Payload.Auctions[0].Item
	if item == nil || item.Element != drawplugin.ElemHeat {
		t.Fatalf("元素枚举未规范化: %+v", item)
	}
}

// TestLichSisterSearchQueryParams 验证玄骸/信条搜索串（type、幻纹、伤害区间）。
func TestLichSisterSearchQueryParams(t *testing.T) {
	setupMarketDB(t)
	seedMarketItems(t)
	var captured string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"payload":{"auctions":[]}}`)
	}))
	defer server.Close()
	api := warframe.NewMarketAPIWithBaseURL(server.Client(), server.URL)

	if _, err := warframe.QueryLichSisterAuctions(api, "kuva bramma", warframe.LichSisterSister); err != nil {
		t.Fatal(err)
	}
	parsed, err := url.ParseQuery(captured)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"type":            "sister",
		"has_ephemera":    "false",
		"sort_by":         "price_asc",
		"weapon_url_name": testLichSlug,
		"damage_min":      "25",
		"damage_max":      "60",
	}
	for key, value := range want {
		if parsed.Get(key) != value {
			t.Errorf("%s = %q, 期望 %q", key, parsed.Get(key), value)
		}
	}
}

// TestQueryDucatsDump 验证金/银垃圾筛选区间、物品名翻译与性价比排序。
func TestQueryDucatsDump(t *testing.T) {
	setupMarketDB(t)
	seedMarketItems(t)

	ducats := `{"payload":{
		"previous_day":[
			{"item":"` + testDucatGoldID + `","ducats":100,"ducats_per_platinum_wa":3.5},
			{"item":"` + testDucatSilverID + `","ducats":45,"ducats_per_platinum_wa":1.5},
			{"item":"excluded-low","ducats":44,"ducats_per_platinum_wa":9.9},
			{"item":"excluded-high","ducats":101,"ducats_per_platinum_wa":9.9}
		],
		"previous_hour":[
			{"item":"` + testDucatGoldID + `","ducats":100,"ducats_per_platinum_wa":4.0}
		]}}`
	api := marketTestServer(t, `[]`, `{"payload":{"auctions":[]}}`, ducats)

	gold, err := warframe.QueryDucatsDump(api, warframe.DucatsDumpGod)
	if err != nil {
		t.Fatal(err)
	}
	if gold == nil || len(gold.Day) != 1 || len(gold.Hour) != 1 {
		t.Fatalf("金垃圾数量错误: %+v", gold)
	}
	if gold.Day[0].Item != "金部件" || gold.Day[0].Ducats != 100 {
		t.Fatalf("金垃圾条目错误: %+v", gold.Day[0])
	}

	// 银垃圾使用同一份缓存（120s 内命中 URL 缓存），但筛选区间不同
	silver, err := warframe.QueryDucatsDump(api, warframe.DucatsDumpSilver)
	if err != nil {
		t.Fatal(err)
	}
	if silver == nil || len(silver.Day) != 1 {
		t.Fatalf("银垃圾数量错误: %+v", silver)
	}
	if silver.Day[0].Item != "银部件" || silver.Day[0].Ducats != 45 {
		t.Fatalf("银垃圾条目错误: %+v", silver.Day[0])
	}
}

// TestQueryDucatsDumpEmpty 验证空响应返回 nil（供指令层提示获取失败）。
func TestQueryDucatsDumpEmpty(t *testing.T) {
	setupMarketDB(t)
	seedMarketItems(t)
	api := marketTestServer(t, `[]`, `{"payload":{"auctions":[]}}`, `{"payload":{}}`)

	result, err := warframe.QueryDucatsDump(api, warframe.DucatsDumpGod)
	if err != nil {
		t.Fatal(err)
	}
	if result != nil {
		t.Fatalf("空响应应返回 nil, 实际 %+v", result)
	}
}

// TestMarketQueriesRejectMalformedJSON verifies that each changed market query
// surfaces a parse error instead of returning a partial success for corrupt API data.
func TestMarketQueriesRejectMalformedJSON(t *testing.T) {
	setupMarketDB(t)
	seedMarketItems(t)
	api := marketTestServer(t, `{`, `{`, `{`)

	tests := []struct {
		name    string
		wantErr string
		query   func() error
	}{
		{
			name:    "orders",
			wantErr: "parse market orders",
			query: func() error {
				_, err := warframe.QueryMarketOrders(api, "loki prime set", false, false, "")
				return err
			},
		},
		{
			name:    "riven auctions",
			wantErr: "parse riven auctions",
			query: func() error {
				_, err := warframe.QueryRivenAuctions(api, "rubico prime")
				return err
			},
		},
		{
			name:    "lich auctions",
			wantErr: "parse lich sister auctions",
			query: func() error {
				_, err := warframe.QueryLichSisterAuctions(api, "kuva bramma", warframe.LichSisterLich)
				return err
			},
		},
		{
			name:    "ducats",
			wantErr: "parse ducats",
			query: func() error {
				_, err := warframe.QueryDucatsDump(api, warframe.DucatsDumpGod)
				return err
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.query()
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("error = %v, want error containing %q", err, test.wantErr)
			}
		})
	}
}

// TestQueryRivenAuctionsLimitAndSort covers the ten-result contract at its boundary.
func TestQueryRivenAuctionsLimitAndSort(t *testing.T) {
	setupMarketDB(t)
	seedMarketItems(t)

	auctions := make([]map[string]any, 0, 12)
	for price := 112; price >= 101; price-- {
		auctions = append(auctions, map[string]any{
			"id":             fmt.Sprintf("riven-%d", price),
			"closed":         false,
			"visible":        true,
			"buyout_price":   price,
			"starting_price": price + 10,
			"owner":          map[string]any{"status": "online"},
			"item":           map[string]any{"type": "riven", "weapon_url_name": testRivenSlug},
		})
	}
	body, err := json.Marshal(map[string]any{"payload": map[string]any{"auctions": auctions}})
	if err != nil {
		t.Fatal(err)
	}
	api := marketTestServer(t, `[]`, string(body), `{"payload":{}}`)

	result, err := warframe.QueryRivenAuctions(api, "rubico prime")
	if err != nil {
		t.Fatal(err)
	}
	if result.DTO == nil || result.DTO.Payload == nil {
		t.Fatal("expected a riven auction payload")
	}
	got := result.DTO.Payload.Auctions
	if len(got) != 10 {
		t.Fatalf("auction count = %d, want 10", len(got))
	}
	for i, auction := range got {
		wantPrice := 101 + i
		if auction.BuyoutPrice == nil || *auction.BuyoutPrice != wantPrice {
			t.Fatalf("auction[%d] price = %v, want %d", i, auction.BuyoutPrice, wantPrice)
		}
	}
}

// TestQueryLichSisterAuctionsLimitAndSort covers sorting and truncation with
// more than ten otherwise valid auctions.
func TestQueryLichSisterAuctionsLimitAndSort(t *testing.T) {
	setupMarketDB(t)
	seedMarketItems(t)

	auctions := make([]map[string]any, 0, 12)
	for price := 212; price >= 201; price-- {
		auctions = append(auctions, map[string]any{
			"id":           fmt.Sprintf("lich-%d", price),
			"closed":       false,
			"visible":      true,
			"buyout_price": price,
			"owner":        map[string]any{"status": "ingame"},
			"item":         map[string]any{"element": "cold", "weapon_url_name": testLichSlug},
		})
	}
	body, err := json.Marshal(map[string]any{"payload": map[string]any{"auctions": auctions}})
	if err != nil {
		t.Fatal(err)
	}
	api := marketTestServer(t, `[]`, string(body), `{"payload":{}}`)

	result, err := warframe.QueryLichSisterAuctions(api, "kuva bramma", warframe.LichSisterLich)
	if err != nil {
		t.Fatal(err)
	}
	if result.DTO == nil || result.DTO.Payload == nil {
		t.Fatal("expected a lich auction payload")
	}
	got := result.DTO.Payload.Auctions
	if len(got) != 10 {
		t.Fatalf("auction count = %d, want 10", len(got))
	}
	for i, auction := range got {
		wantPrice := 201 + i
		if auction.BuyoutPrice == nil || *auction.BuyoutPrice != wantPrice {
			t.Fatalf("auction[%d] price = %v, want %d", i, auction.BuyoutPrice, wantPrice)
		}
		if auction.Item == nil || auction.Item.Element != drawplugin.ElemCold {
			t.Fatalf("auction[%d] element was not normalized: %+v", i, auction.Item)
		}
	}
}

// TestQueryDucatsDumpBoundariesAndLimit verifies the inclusive/exclusive
// currency boundaries and the ten-entry descending-ratio cap.
func TestQueryDucatsDumpBoundariesAndLimit(t *testing.T) {
	setupMarketDB(t)
	seedMarketItems(t)

	entries := make([]map[string]any, 0, 16)
	for ratio := 1; ratio <= 12; ratio++ {
		entries = append(entries, map[string]any{
			"item": testDucatGoldID, "ducats": 100, "ducats_per_platinum_wa": ratio,
		})
	}
	entries = append(entries,
		map[string]any{"item": testDucatSilverID, "ducats": 45, "ducats_per_platinum_wa": 4.5},
		map[string]any{"item": "silver-upper", "ducats": 99, "ducats_per_platinum_wa": 9.9},
		map[string]any{"item": "too-low", "ducats": 44, "ducats_per_platinum_wa": 99.0},
		map[string]any{"item": "too-high", "ducats": 101, "ducats_per_platinum_wa": 99.0},
	)
	body, err := json.Marshal(map[string]any{
		"payload": map[string]any{"previous_day": entries, "previous_hour": []any{}},
	})
	if err != nil {
		t.Fatal(err)
	}
	api := marketTestServer(t, `[]`, `{"payload":{"auctions":[]}}`, string(body))

	gold, err := warframe.QueryDucatsDump(api, warframe.DucatsDumpGod)
	if err != nil {
		t.Fatal(err)
	}
	if gold == nil || len(gold.Day) != 10 {
		t.Fatalf("gold entries = %+v, want exactly 10", gold)
	}
	for i, entry := range gold.Day {
		wantRatio := float64(12 - i)
		if entry.Ducats != 100 || entry.DucatsPerPlatinumWa != wantRatio {
			t.Fatalf("gold[%d] = %+v, want ducats=100 ratio=%v", i, entry, wantRatio)
		}
	}

	silver, err := warframe.QueryDucatsDump(api, warframe.DucatsDumpSilver)
	if err != nil {
		t.Fatal(err)
	}
	if silver == nil || len(silver.Day) != 2 {
		t.Fatalf("silver entries = %+v, want lower and upper boundary entries", silver)
	}
	if silver.Day[0].Ducats != 99 || silver.Day[1].Ducats != 45 {
		t.Fatalf("silver boundaries or ordering are wrong: %+v", silver.Day)
	}
}

// TestDrawMarketDucatsNonEmpty 验证金/银垃圾图与市场三图的非空绘制。
func TestDrawMarketDucatsNonEmpty(t *testing.T) {
	dump := &draw.Ducats{
		Day: []*draw.DucatsEntry{
			{Item: "金部件", Ducats: intPtr(100), DucatsPerPlatinumWa: floatPtr(3.5), WaPrice: floatPtr(10.5), Volume: intPtr(20)},
			{Item: "另一个部件名称很长很长很长很长很长", Ducats: intPtr(100), DucatsPerPlatinumWa: floatPtr(2.0)},
		},
		Hour: []*draw.DucatsEntry{
			{Item: "银部件", Ducats: intPtr(45), DucatsPerPlatinumWa: floatPtr(1.5)},
		},
	}
	if image := draw.DrawMarketDucats(dump, "金垃圾"); len(image) == 0 {
		t.Fatal("DrawMarketDucats 输出为空")
	}
	if image := draw.DrawMarketDucats(&draw.Ducats{}, "银垃圾"); len(image) != 0 {
		t.Fatal("空输入应返回 nil")
	}
}

// TestDrawMarketOrdersAndAuctionsNonEmpty 验证订单/紫卡/玄骸三张图的非空绘制。
func TestDrawMarketOrdersAndAuctionsNonEmpty(t *testing.T) {
	buyer, maxRank := false, false
	orders := &draw.Orders{
		Name: "loki prime set",
		Form: draw.MarketPlatformPC,
		IsBy: &buyer, IsMax: &maxRank,
		Ducats: intPtr(45), Vaulted: boolPtr(false),
		MaxAmberStars: intPtr(0), MaxCyanStars: intPtr(0), BaseEndo: intPtr(0),
		ReqMasteryRank: intPtr(5), TradingTax: intPtr(1000),
		Orders: []*draw.OrderWithUser{
			{
				ID: "o1", Type: drawplugin.TransSell, Platinum: intPtr(15), Quantity: intPtr(1), Rank: intPtr(0),
				ItemID: "item-loki-set", Visible: boolPtr(true),
				User: &draw.MarketUser{IngameName: "卖家", Status: drawplugin.MarketStatusOnline},
			},
		},
	}
	if image := draw.DrawMarketOrders(orders); len(image) == 0 {
		t.Fatal("DrawMarketOrders 输出为空")
	}
	if image := draw.DrawMarketOrdersList([]string{"loki prime set", "loki prime blueprint"}); len(image) == 0 {
		t.Fatal("DrawMarketOrdersList 输出为空")
	}

	visible := true
	riven := &draw.MarketRiven{
		ItemName: "rubico prime",
		Payload: &draw.MarketRivenPayload{Auctions: []*draw.MarketRivenAuction{
			{
				ID: "a1", BuyoutPrice: intPtr(150), Visible: &visible,
				Owner: &draw.MarketRivenOwner{IngameName: "卖家", Status: "online"},
				Item: &draw.MarketRivenItem{
					Type: "riven", WeaponURLName: testRivenSlug, Name: "绝路 紫卡",
					Polarity: drawplugin.PolarityMadurai,
					Attributes: []*draw.MarketRivenAttribute{
						{URLName: "暴击伤害", Value: floatPtr(80.5), Positive: boolPtr(true)},
					},
				},
			},
		}},
	}
	if image := draw.DrawMarketRiven(riven); len(image) == 0 {
		t.Fatal("DrawMarketRiven 输出为空")
	}

	lich := &draw.MarketLichSister{
		Payload: &draw.MarketLichSisterPayload{
			ItemName: "kuva bramma",
			Auctions: []*draw.MarketLichSisterAuction{
				{
					ID: "l1", BuyoutPrice: intPtr(30), Visible: &visible,
					Owner: &draw.MarketLichSisterOwner{IngameName: "卖家", Status: "online"},
					Item:  &draw.MarketLichSisterItem{Element: drawplugin.ElemHeat, Damage: intPtr(40)},
				},
			},
		},
	}
	if image := draw.DrawMarketLichSister(lich); len(image) == 0 {
		t.Fatal("DrawMarketLichSister 输出为空")
	}
}

// orderIDs 提取订单 ID 列表，便于断言失败时输出可读上下文。
func orderIDs(orders []*draw.OrderWithUser) []string {
	ids := make([]string, 0, len(orders))
	for _, order := range orders {
		if order != nil {
			ids = append(ids, order.ID)
		}
	}
	return ids
}

// TestMarketCommandsEndToEnd 验证市场指令经 CommandRegistry 全链路派发：
// 命令匹配 → 权限校验 → 本地物品匹配 → 注入的本地市场服务器 → 图片回复。
func TestMarketCommandsEndToEnd(t *testing.T) {
	// 指令派发会走权限校验，故需同时具备市场表与 Bot 黑白名单表
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "market-e2e.db")), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&modelwarframe.OrdersItem{},
		&modelwarframe.RivenItem{},
		&modelwarframe.RivenTion{},
		&modelwarframe.RivenTionAlias{},
		&modelwarframe.LichSisterWeapon{},
		&modelwarframe.Alias{},
		&modelbot.BotAdmin{}, &modelbot.GroupWhite{}, &modelbot.ProveWhite{},
		&modelbot.GroupBlack{}, &modelbot.ProveBlack{},
		&modelsystem.LogInfo{},
	); err != nil {
		t.Fatal(err)
	}
	database.DB = db
	t.Cleanup(func() { closeGlobalDB(t) })

	seedMarketItems(t)

	orders := "[" + marketOrderJSON("o1", "sell", 12, 0, testOrdersItemID, "online") + "]"
	api := marketTestServer(t, orders, `{"payload":{"auctions":[]}}`,
		`{"payload":{"previous_day":[{"item":"`+testDucatGoldID+`","ducats":100,"ducats_per_platinum_wa":3.5}],"previous_hour":[]}}`)
	warframe.SetDefaultMarketAPI(api)
	t.Cleanup(func() { warframe.SetDefaultMarketAPI(nil) })

	caller := &recordingCaller{}
	const botUID = int64(12001)
	zero.APICallers.Store(botUID, caller)
	previousConfig := zero.BotConfig
	zero.BotConfig = zero.Config{MaxProcessTime: 5 * time.Second}
	registry := onebot.NewCommandRegistry(false)
	if err := registry.Register(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		registry.Close()
		zero.BotConfig = previousConfig
		zero.APICallers.Delete(botUID)
	})

	bot := zero.GetBot(botUID)
	if bot == nil {
		t.Fatal("test Bot should be available")
	}

	// 1) 正常订单查询：应返回图片元素
	echoPrivateMessage(t, bot, botUID, 22001, "/WM loki prime set")
	waitFor(t, func() bool { return len(caller.snapshot()) >= 1 }, "market orders command should reply")
	requests := caller.snapshot()
	if !replyContainsImage(requests[0].Params["message"]) {
		t.Fatalf("市场订单指令应回复图片: %#v", requests[0].Params["message"])
	}

	// 2) 缺少关键字：应回复提示文本
	echoPrivateMessage(t, bot, botUID, 22001, "/WM")
	waitFor(t, func() bool { return len(caller.snapshot()) >= 2 }, "empty keyword should reply text")
	requests = caller.snapshot()
	if text := replyText(requests[1].Params["message"]); !strings.Contains(text, "请输入正确的名称") {
		t.Fatalf("空关键字应提示名称错误: %q", text)
	}

	// 3) 金垃圾：杜卡币图
	echoPrivateMessage(t, bot, botUID, 22001, "金垃圾")
	waitFor(t, func() bool { return len(caller.snapshot()) >= 3 }, "ducats command should reply")
	requests = caller.snapshot()
	if !replyContainsImage(requests[2].Params["message"]) {
		t.Fatalf("金垃圾指令应回复图片: %#v", requests[2].Params["message"])
	}
}

// replyContainsImage 判定 OneBot 消息参数中是否含非空图片元素。
func replyContainsImage(params any) bool {
	segments, ok := params.(message.Message)
	if !ok {
		return false
	}
	for _, segment := range segments {
		if segment.Type == "image" && segment.Data["file"] != "" {
			return true
		}
	}
	return false
}

// replyText 提取 OneBot 消息参数中的纯文本内容。
func replyText(params any) string {
	if text, ok := params.(string); ok {
		return text
	}
	segments, ok := params.(message.Message)
	if !ok {
		return ""
	}
	var builder strings.Builder
	for _, segment := range segments {
		if segment.Type == "text" {
			builder.WriteString(segment.Data["text"])
		}
	}
	return builder.String()
}
