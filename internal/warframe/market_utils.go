// 市场 API 封装，对应 Java NyxBot 的 HttpUtils.marketSendGet
// 提供 warframe.market 各端点的请求与 429/超时错误文案，供数据导入与市场指令使用
package warframe

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"nyxbot-go/internal/logging"
)

// marketBaseURL warframe.market API 根地址。
const marketBaseURL = "https://api.warframe.market"

// marketTimeout 单次市场 API 请求超时。
const marketTimeout = 15 * time.Second

// MarketAPI 封装 warframe.market 的 HTTP 请求与进程内缓存。
// 对齐 Java HttpUtils.marketSendGet 的 120s 缓存语义（按 URL 缓存）。
type MarketAPI struct {
	client *http.Client
	mu     sync.Mutex
	cache  map[string]marketCacheEntry // URL -> 缓存条目
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
	return &MarketAPI{client: client, cache: make(map[string]marketCacheEntry)}
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

// Invalidate 清除全部缓存（数据更新任务完成后调用）。
func (api *MarketAPI) Invalidate() {
	api.mu.Lock()
	api.cache = make(map[string]marketCacheEntry)
	api.mu.Unlock()
}

// doGet 执行实际请求，请求头对齐 Java marketSendGet（Language/Platform/Crossplay）。
func (api *MarketAPI) doGet(url string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), marketTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	request.Header.Set("Language", "zh-hans")
	request.Header.Set("Platform", "pc")
	request.Header.Set("Pragma", "no-cache")
	request.Header.Set("Crossplay", "true")

	response, err := api.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("market request failed: %w", err)
	}
	defer response.Body.Close()

	switch {
	case response.StatusCode == http.StatusTooManyRequests:
		return nil, fmt.Errorf("market API rate limited (HTTP 429)，请稍后重试")
	case response.StatusCode < 200 || response.StatusCode >= 400:
		return nil, fmt.Errorf("market API returned HTTP %d", response.StatusCode)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("read market response: %w", err)
	}
	return body, nil
}

// marketPayload 市场 API 通用响应外壳 { payload: { ... } }。
type marketPayload struct {
	Payload struct {
		Items []json.RawMessage `json:"items"`
	} `json:"payload"`
}

// MarketItem 市场物品条目（/v2/items 的元素），i18n 取 zh-hans。
type MarketItem struct {
	ID            string `json:"id"`
	Slug          string `json:"url_name"`
	GameRef       string `json:"item_name"`
	Icon          string `json:"icon"`
	Thumb         string `json:"thumb"`
	Ducats        int    `json:"ducats"`
	Vaulted       bool   `json:"vaulted"`
	MaxRank       int    `json:"max_rank"`
	BulkTradable  bool   `json:"bulk_tradable"`
	MaxAmberStars int    `json:"max_amber_stars"`
	MaxCyanStars  int    `json:"max_cyan_stars"`
	BaseEndo      int    `json:"base_endo"`
	TradingTax    int    `json:"trading_tax"`

	Name   string `json:"name"`    // 填充后的中文名
	EnName string `json:"en_name"` // 填充后的英文名
}

// FetchItems 拉取 /v2/items 全量市场物品，返回中文名填充后的条目列表。
// i18n 结构：{ item_name, icon, thumb, ... }，zh-hans 字段在 item 内联。
func (api *MarketAPI) FetchItems() ([]MarketItem, error) {
	body, err := api.Get(marketBaseURL + "/v2/items")
	if err != nil {
		return nil, err
	}
	var envelope struct {
		Payload struct {
			Items []MarketItem `json:"items"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("parse /v2/items: %w", err)
	}
	items := envelope.Payload.Items
	results := make([]MarketItem, 0, len(items))
	for _, item := range items {
		item.EnName = item.GameRef
		item.Name = item.GameRef
		results = append(results, item)
	}
	return results, nil
}

// FetchRivenWeapons 拉取 /v2/riven/weapons 紫卡武器列表。
func (api *MarketAPI) FetchRivenWeapons() ([]json.RawMessage, error) {
	body, err := api.Get(marketBaseURL + "/v2/riven/weapons")
	if err != nil {
		return nil, err
	}
	var envelope marketPayload
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("parse /v2/riven/weapons: %w", err)
	}
	return envelope.Payload.Items, nil
}

// FetchLichSisterWeapons 拉取赤毒/信条武器（/v2/lich/weapons + /v2/sister/weapons）。
func (api *MarketAPI) FetchLichSisterWeapons() ([][]json.RawMessage, error) {
	urls := []string{
		marketBaseURL + "/v2/lich/weapons",
		marketBaseURL + "/v2/sister/weapons",
	}
	results := make([][]json.RawMessage, 0, 2)
	for _, url := range urls {
		body, err := api.Get(url)
		if err != nil {
			return nil, err
		}
		var envelope marketPayload
		if err := json.Unmarshal(body, &envelope); err != nil {
			return nil, fmt.Errorf("parse %s: %w", url, err)
		}
		results = append(results, envelope.Payload.Items)
	}
	return results, nil
}

// FetchLichSisterEphemeras 拉取赤毒/信条幻纹（/v2/lich/ephemeras + /v2/sister/ephemeras）。
func (api *MarketAPI) FetchLichSisterEphemeras() ([][]json.RawMessage, error) {
	urls := []string{
		marketBaseURL + "/v2/lich/ephemeras",
		marketBaseURL + "/v2/sister/ephemeras",
	}
	results := make([][]json.RawMessage, 0, 2)
	for _, url := range urls {
		body, err := api.Get(url)
		if err != nil {
			return nil, err
		}
		var envelope marketPayload
		if err := json.Unmarshal(body, &envelope); err != nil {
			return nil, fmt.Errorf("parse %s: %w", url, err)
		}
		results = append(results, envelope.Payload.Items)
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
	body, err := api.Get(marketBaseURL + "/v1/tools/ducats")
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
