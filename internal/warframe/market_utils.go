// 市场 API 封装，对应 Java NyxBot 的 HttpUtils.marketSendGet
// 提供 warframe.market 各端点的请求与 429/超时错误文案，供数据导入与市场指令使用
// 字段解析对齐 Java：gameRef 为顶层字段，name/icon/thumb 从 i18n.zh-hans 嵌套读取
package warframe

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"nyxbot-go/internal/logging"
)

// marketBaseURL warframe.market API 根地址。
const marketBaseURL = "https://api.warframe.market"

// MarketAPI 封装 warframe.market 的 HTTP 请求与进程内缓存。
// 对齐 Java HttpUtils.marketSendGet 的 120s 缓存语义（按 URL 缓存）。
type MarketAPI struct {
	client  *http.Client
	baseURL string // API 根地址（默认 marketBaseURL，测试可注入）
	mu      sync.Mutex
	cache   map[string]marketCacheEntry // URL -> 缓存条目
}

// marketCacheEntry 单条 URL 缓存：响应 JSON + 拉取时间。
type marketCacheEntry struct {
	body      []byte
	fetchedAt time.Time
}

// marketCacheTTL 市场 API 缓存有效期（对齐 Java 120s）。
const marketCacheTTL = 2 * time.Minute

// NewMarketAPI 创建市场 API 客户端；client 为 nil 时使用 http.DefaultClient。
func NewMarketAPI(client *http.Client) *MarketAPI {
	if client == nil {
		client = http.DefaultClient
	}
	return &MarketAPI{client: client, baseURL: marketBaseURL, cache: make(map[string]marketCacheEntry)}
}

// NewMarketAPIWithBaseURL 创建使用指定根地址的市场 API 客户端（供测试注入本地服务器）。
func NewMarketAPIWithBaseURL(client *http.Client, baseURL string) *MarketAPI {
	api := NewMarketAPI(client)
	api.baseURL = strings.TrimRight(baseURL, "/")
	return api
}

// Get 发起市场 API GET 请求（进程内缓存 120s），返回响应 JSON 字节。
// url 需为完整 URL（含 /v2/items 等路径）；httpStatus 为 429 时返回限速错误。
func (api *MarketAPI) Get(url string) ([]byte, error) {
	api.mu.Lock()
	if entry, ok := api.cache[url]; ok && time.Since(entry.fetchedAt) < marketCacheTTL {
		body := entry.body
		api.mu.Unlock()
		return body, nil
	}
	api.mu.Unlock()

	body, err := api.doGet(url)
	if err != nil {
		return nil, err
	}

	api.mu.Lock()
	api.cache[url] = marketCacheEntry{body: body, fetchedAt: time.Now()}
	api.mu.Unlock()
	return body, nil
}

// Invalidate 清除全部缓存（数据更新任务开始前调用，确保任务拉取最新数据）。
func (api *MarketAPI) Invalidate() {
	api.mu.Lock()
	api.cache = make(map[string]marketCacheEntry)
	api.mu.Unlock()
}

// doGet 执行实际请求（带网络重试），请求头对齐 Java marketSendGet（Language/Platform/Crossplay）。
// 网络错误（TLS 超时等）与 429/5xx 自动重试 3 次，指数退避。
func (api *MarketAPI) doGet(url string) ([]byte, error) {
	headers := map[string]string{
		"Accept":          "application/json",
		"Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
		"Language":        "zh-hans",
		"Platform":        "pc",
		"Pragma":          "no-cache",
		"Crossplay":       "true",
	}
	body, err := doRequestWithRetry(context.Background(), api.client, http.MethodGet, url, headers)
	if err != nil {
		return nil, fmt.Errorf("market request failed: %w", err)
	}
	return body, nil
}

// marketPayload 市场 API 通用响应外壳 { data: [...] }。
// 经真实 API 验证（v0.25.0）：响应为 {"apiVersion":"...","data":[...]}，无 payload 包裹。
type marketPayload struct {
	Data []json.RawMessage `json:"data"`
}

// fetchItemsRaw 拉取指定端点的 data 原始数组。
func (api *MarketAPI) fetchItemsRaw(url string) ([]json.RawMessage, error) {
	body, err := api.Get(url)
	if err != nil {
		return nil, err
	}
	var envelope marketPayload
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("parse %s: %w", url, err)
	}
	return envelope.Data, nil
}

// marketEntry 市场条目通用反序列化结构（对齐 Java 各 Service 的 build* 方法）：
// 顶层字段 + i18n.zh-hans 嵌套（name/icon/thumb）。
type marketEntry struct {
	ID             string  `json:"id"`
	Slug           string  `json:"slug"`
	GameRef        string  `json:"gameRef"`
	Group          string  `json:"group"`
	RivenType      string  `json:"rivenType"`
	Disposition    float64 `json:"disposition"`
	ReqMasteryRank int     `json:"reqMasteryRank"`
	BulkTradable   bool    `json:"bulkTradable"`
	MaxRank        int     `json:"maxRank"`
	Ducats         int     `json:"ducats"`
	Vaulted        bool    `json:"vaulted"`
	MaxAmberStars  int     `json:"maxAmberStars"`
	MaxCyanStars   int     `json:"maxCyanStars"`
	BaseEndo       int     `json:"baseEndo"`
	Animation      string  `json:"animation"`
	Element        string  `json:"element"`
	Name           string  `json:"-"`
	Icon           string  `json:"-"`
	Thumb          string  `json:"-"`

	I18N marketI18N `json:"i18n"`
}

// marketI18N 市场条目的多语言嵌套结构。
type marketI18N struct {
	ZhHans marketI18NEntry `json:"zh-hans"`
}

// marketI18NEntry 单语言条目（name/icon/thumb）。
type marketI18NEntry struct {
	Name  string `json:"name"`
	Icon  string `json:"icon"`
	Thumb string `json:"thumb"`
}

// resolveI18n 从 i18n.zh-hans 填充 name/icon/thumb（name 缺失回退 slug，对齐 Java）。
func (entry *marketEntry) resolveI18n() {
	if entry.I18N.ZhHans.Name != "" {
		entry.Name = entry.I18N.ZhHans.Name
	} else {
		entry.Name = entry.Slug
	}
	entry.Icon = entry.I18N.ZhHans.Icon
	entry.Thumb = entry.I18N.ZhHans.Thumb
}

// parseMarketItems 解析 items 原始数组为通用条目并回填 i18n。
func parseMarketItems(rawItems []json.RawMessage) []marketEntry {
	results := make([]marketEntry, 0, len(rawItems))
	for _, raw := range rawItems {
		var entry marketEntry
		if err := json.Unmarshal(raw, &entry); err != nil {
			continue
		}
		entry.resolveI18n()
		results = append(results, entry)
	}
	return results
}

// FetchItems 拉取 /v2/items 全量市场物品条目（对齐 OrdersItemsService.initOrdersItemsData）。
func (api *MarketAPI) FetchItems() ([]marketEntry, error) {
	rawItems, err := api.fetchItemsRaw(api.baseURL + "/v2/items")
	if err != nil {
		return nil, err
	}
	return parseMarketItems(rawItems), nil
}

// FetchRivenWeapons 拉取 /v2/riven/weapons 紫卡武器条目（对齐 RivenItemsService）。
func (api *MarketAPI) FetchRivenWeapons() ([]marketEntry, error) {
	rawItems, err := api.fetchItemsRaw(api.baseURL + "/v2/riven/weapons")
	if err != nil {
		return nil, err
	}
	return parseMarketItems(rawItems), nil
}

// FetchLichSisterWeapons 拉取赤毒/信条武器（/v2/lich/weapons + /v2/sister/weapons）。
func (api *MarketAPI) FetchLichSisterWeapons() ([][]marketEntry, error) {
	urls := []string{
		api.baseURL + "/v2/lich/weapons",
		api.baseURL + "/v2/sister/weapons",
	}
	results := make([][]marketEntry, 0, 2)
	for _, url := range urls {
		rawItems, err := api.fetchItemsRaw(url)
		if err != nil {
			return nil, err
		}
		results = append(results, parseMarketItems(rawItems))
	}
	return results, nil
}

// FetchLichSisterEphemeras 拉取赤毒/信条幻纹（/v2/lich/ephemeras + /v2/sister/ephemeras）。
func (api *MarketAPI) FetchLichSisterEphemeras() ([][]marketEntry, error) {
	urls := []string{
		api.baseURL + "/v2/lich/ephemeras",
		api.baseURL + "/v2/sister/ephemeras",
	}
	results := make([][]marketEntry, 0, 2)
	for _, url := range urls {
		rawItems, err := api.fetchItemsRaw(url)
		if err != nil {
			return nil, err
		}
		results = append(results, parseMarketItems(rawItems))
	}
	return results, nil
}

// DucatsResponse 杜卡德币工具响应（/v1/tools/ducats）。
type DucatsResponse struct {
	Payload struct {
		Orders []DucatsOrder `json:"orders"`
	} `json:"payload"`
}

// DucatsOrder 杜卡德币排行条目。
type DucatsOrder struct {
	ID                string  `json:"id"`
	Item              string  `json:"item"`
	ItemName          string  `json:"item_name"`
	Thumb             string  `json:"thumb"`
	Ducats            int     `json:"ducats"`
	Platinum          int     `json:"platinum"`
	DucatsPerPlatinum float64 `json:"ducats_per_platinum"`
}

// FetchDucats 拉取 /v1/tools/ducats 杜卡德币排行。
func (api *MarketAPI) FetchDucats() ([]DucatsOrder, error) {
	body, err := api.Get(api.baseURL + "/v1/tools/ducats")
	if err != nil {
		return nil, err
	}
	var response DucatsResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("parse /v1/tools/ducats: %w", err)
	}
	return response.Payload.Orders, nil
}

// MarketErrorText 将市场请求错误转换为用户可读文案（供指令回复）。
func MarketErrorText(err error) string {
	if err == nil {
		return ""
	}
	text := err.Error()
	if strings.Contains(text, "429") {
		return "market API 请求过于频繁，请稍后重试"
	}
	if strings.Contains(text, "timeout") || strings.Contains(text, "context deadline") {
		return "market API 请求超时，请稍后重试"
	}
	logging.DebugPack("warframe.market", "market error: %v", err)
	return "market API 请求失败，请稍后重试"
}
