// 市场订单查询数据层（/WM 指令），对齐 Java NyxBot 的 MarketOrderUtils + MarketOrdersPlugin。
//
// 流程：本地 orders_items 匹配关键字 → warframe.market /v2/item/{slug}/set 补全段位与交易税
// → /v2/orders/item/{slug} 拉取订单 → 过滤（离线用户/物品 ID/买卖方向/满级）→ 排序截断 8 条。
// 未命中本地条目时返回候选物品名列表，由指令层绘制候选图。
package warframe

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"nyxbot-go/internal/draw"
	"nyxbot-go/internal/enum/drawplugin"
	"nyxbot-go/internal/logging"
	modelwarframe "nyxbot-go/internal/model/warframe"
)

// marketOrdersLimit 市场订单最大返回条数（对齐 Java limit(8)）。
const marketOrdersLimit = 8

// MarketOrdersResult 市场订单查询结果。
// PossibleItems 非空表示未命中本地物品（需展示候选列表）；否则 Item 与 Orders 为查询结果。
type MarketOrdersResult struct {
	Slug          string                // 命中的市场 slug
	PossibleItems []string              // 候选物品名（未命中时）
	Item          *draw.Orders          // 绘图输入（Orders 为空表示直接展示候选列表）
	Orders        []*draw.OrderWithUser // 过滤排序后的订单
}

// defaultMarketMu/defaultMarket 进程级默认市场 API 客户端（复用 120s URL 缓存）。
// 用互斥锁而非 sync.Once：SetDefaultMarketAPI(nil) 之后必须能按需重建，
// 否则 once 已触发、defaultMarket 为 nil，后续调用方会解引用空指针。
var (
	defaultMarketMu sync.Mutex
	defaultMarket   *MarketAPI
)

// DefaultMarketAPI 返回进程级默认市场 API 客户端；为 nil 时按需创建，永不返回 nil。
// 单次请求超时由 doRequestWithRetry 统一控制（config.yaml 的 warframe.http_retry_*）。
func DefaultMarketAPI() *MarketAPI {
	defaultMarketMu.Lock()
	defer defaultMarketMu.Unlock()
	if defaultMarket == nil {
		defaultMarket = NewMarketAPI(nil)
	}
	return defaultMarket
}

// SetDefaultMarketAPI 替换进程级默认市场 API 客户端（供黑盒测试注入本地服务器）。
// 传 nil 时清除当前实例，下次 DefaultMarketAPI() 会重新按需创建。
func SetDefaultMarketAPI(api *MarketAPI) {
	defaultMarketMu.Lock()
	defer defaultMarketMu.Unlock()
	defaultMarket = api
}

// QueryMarketOrders 查询市场订单（对齐 Java MarketOrdersPlugin.postMarketOrdersImage）。
// keyword 为去掉指令前缀后的原文；buyer 为 true 表示查询买家（否则卖家）；
// maxRank 为 true 表示只保留满级/满星订单；platform 为空时按默认平台请求。
func QueryMarketOrders(api *MarketAPI, keyword string, buyer, maxRank bool, platform string) (*MarketOrdersResult, error) {
	if api == nil {
		api = DefaultMarketAPI()
	}
	item := findOrdersItemByKeyword(keyword)
	if item == nil {
		return &MarketOrdersResult{
			PossibleItems: ordersItemCandidates(normalizeMarketInput(keyword), keyword),
		}, nil
	}

	// 补全段位要求与交易税（对齐 Java toSet：/v2/item/{slug}/set 的 items 数组中按 id 命中）
	reqMastery, tradingTax := fetchOrdersItemDetail(api, item)
	orders, err := fetchMarketOrders(api, item.Slug, platform)
	if err != nil {
		return nil, err
	}

	result := &MarketOrdersResult{Slug: item.Slug}
	result.Orders = filterMarketOrders(orders, item.ID, buyer, maxRank, item.MaxRank, item.MaxAmberStars, item.MaxCyanStars)
	result.Item = buildOrdersDrawInput(item, result.Orders, reqMastery, tradingTax, buyer, maxRank, platform)
	return result, nil
}

// buildOrdersDrawInput 组装绘图输入（对齐 Java getOrders：杜卡币/入库/星级/内融核心缺省为 0）。
func buildOrdersDrawInput(
	item *modelwarframe.OrdersItem,
	orders []*draw.OrderWithUser,
	reqMastery, tradingTax int,
	buyer, maxRank bool,
	platform string,
) *draw.Orders {
	form := draw.MarketPlatform(strings.ToUpper(platform))
	if form == "" {
		form = draw.MarketPlatformPC
	}
	buyerFlag, maxFlag := buyer, maxRank
	return &draw.Orders{
		Name:           item.Name,
		Form:           form,
		IsBy:           &buyerFlag,
		IsMax:          &maxFlag,
		Ducats:         intPtr(item.Ducats),
		Vaulted:        boolPtr(item.Vaulted),
		MaxAmberStars:  intPtr(item.MaxAmberStars),
		MaxCyanStars:   intPtr(item.MaxCyanStars),
		BaseEndo:       intPtr(item.BaseEndo),
		ReqMasteryRank: intPtr(reqMastery),
		TradingTax:     intPtr(tradingTax),
		Orders:         orders,
	}
}

// findOrdersItemByKeyword 按 Java toDataBase 顺序匹配本地市场物品：
// 标准化 → 精确名 → 别名模糊 → Prime 展开模糊 → 前后缀正则。
func findOrdersItemByKeyword(keyword string) *modelwarframe.OrdersItem {
	normalized := normalizeMarketInput(keyword)
	if normalized == "" {
		return nil
	}
	if hit := findOrdersItemByName(normalized); hit != nil {
		return hit
	}
	aliased := processMarketAliases(normalized)
	if hit := findOrdersItemByNameLike(aliased); hit != nil {
		return hit
	}
	primed := processPrimeKeyword(aliased)
	if hit := findOrdersItemByNameLike(primed); hit != nil {
		return hit
	}
	return findOrdersItemByNameRegex(primed)
}

// fetchOrdersItemDetail 拉取 /v2/item/{slug}/set 取段位要求与交易税。
// 失败时静默降级返回零值（Java 同样尽力而为），不影响主查询。
func fetchOrdersItemDetail(api *MarketAPI, item *modelwarframe.OrdersItem) (int, int) {
	if api == nil || item == nil || item.Slug == "" {
		return 0, 0
	}
	body, err := api.Get(api.baseURL + "/v2/item/" + item.Slug + "/set")
	if err != nil {
		logging.DebugPack("warframe.market", "fetch item set %q failed: %v", item.Slug, err)
		return 0, 0
	}
	var envelope struct {
		Data struct {
			Items []struct {
				ID             string `json:"id"`
				ReqMasteryRank int    `json:"reqMasteryRank"`
				TradingTax     int    `json:"tradingTax"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		logging.DebugPack("warframe.market", "parse item set %q failed: %v", item.Slug, err)
		return 0, 0
	}
	for _, detail := range envelope.Data.Items {
		if detail.ID == item.ID {
			return detail.ReqMasteryRank, detail.TradingTax
		}
	}
	return 0, 0
}

// marketOrderEnvelope 订单接口响应外壳（/v2/orders/item/{slug} 返回 {"data":[...]}）。
type marketOrderEnvelope struct {
	Data []*draw.OrderWithUser `json:"data"`
}

// UnmarshalJSON 反序列化订单，并把 API 的小写枚举值规范化为枚举常量（绘图侧按大写匹配）。
func (envelope *marketOrderEnvelope) UnmarshalJSON(body []byte) error {
	var raw struct {
		Data []*marketOrderDTO `json:"data"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return err
	}
	envelope.Data = make([]*draw.OrderWithUser, 0, len(raw.Data))
	for _, dto := range raw.Data {
		if dto == nil {
			continue
		}
		envelope.Data = append(envelope.Data, dto.toDraw())
	}
	return nil
}

// marketOrderDTO 订单接口原始字段（枚举为小写字符串，与 API 一致）。
type marketOrderDTO struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Platinum   *int   `json:"platinum"`
	Quantity   *int   `json:"quantity"`
	PerTrade   *int   `json:"perTrade"`
	Rank       *int   `json:"rank"`
	Charges    *int   `json:"charges"`
	Subtype    string `json:"subtype"`
	AmberStars *int   `json:"amberStars"`
	CyanStars  *int   `json:"cyanStars"`
	Vosfor     *int   `json:"vosfor"`
	Visible    *bool  `json:"visible"`
	CreatedAt  string `json:"createdAt"`
	UpdatedAt  string `json:"updatedAt"`
	ItemID     string `json:"itemId"`
	User       *struct {
		ID         string `json:"id"`
		IngameName string `json:"ingameName"`
		Avatar     string `json:"avatar"`
		Reputation *int   `json:"reputation"`
		Locale     string `json:"locale"`
		Platform   string `json:"platform"`
		Status     string `json:"status"`
		Activity   *struct {
			Type      string `json:"type"`
			Details   string `json:"details"`
			StartedAt string `json:"startedAt"`
		} `json:"activity"`
		LastSeen string `json:"lastSeen"`
	} `json:"user"`
}

// toDraw 将订单原始字段转换为绘图 DTO（枚举统一大写，缺失值保持 nil）。
func (dto *marketOrderDTO) toDraw() *draw.OrderWithUser {
	order := &draw.OrderWithUser{
		ID:         dto.ID,
		Type:       drawplugin.TransactionType(strings.ToUpper(dto.Type)),
		Platinum:   dto.Platinum,
		Quantity:   dto.Quantity,
		PerTrade:   dto.PerTrade,
		Rank:       dto.Rank,
		Charges:    dto.Charges,
		Subtype:    dto.Subtype,
		AmberStars: dto.AmberStars,
		CyanStars:  dto.CyanStars,
		Vosfor:     dto.Vosfor,
		Visible:    dto.Visible,
		CreatedAt:  parseMarketTime(dto.CreatedAt),
		UpdatedAt:  parseMarketTime(dto.UpdatedAt),
		ItemID:     dto.ItemID,
	}
	if dto.User != nil {
		user := &draw.MarketUser{
			ID:         dto.User.ID,
			IngameName: dto.User.IngameName,
			Avatar:     dto.User.Avatar,
			Reputation: dto.User.Reputation,
			Locale:     dto.User.Locale,
			Platform:   draw.MarketPlatform(strings.ToUpper(dto.User.Platform)),
			Status:     drawplugin.MarketStatus(strings.ToUpper(dto.User.Status)),
			LastSeen:   parseMarketTime(dto.User.LastSeen),
		}
		if dto.User.Activity != nil {
			user.Activity = &draw.MarketUserActivity{
				Type:      drawplugin.MarketActivityType(strings.ToUpper(dto.User.Activity.Type)),
				Details:   dto.User.Activity.Details,
				StartedAt: parseMarketTime(dto.User.Activity.StartedAt),
			}
		}
		order.User = user
	}
	return order
}

// fetchMarketOrders 拉取指定 slug 的订单列表。
func fetchMarketOrders(api *MarketAPI, slug, platform string) ([]*draw.OrderWithUser, error) {
	if api == nil || slug == "" {
		return nil, nil
	}
	url := api.baseURL + "/v2/orders/item/" + slug
	if platform != "" {
		url += "?platform=" + strings.ToLower(platform)
	}
	body, err := api.Get(url)
	if err != nil {
		return nil, err
	}
	var envelope marketOrderEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("parse market orders %q: %w", slug, err)
	}
	return envelope.Data, nil
}

// filterMarketOrders 过滤并排序订单（对齐 Java MarketOrderUtils.market）：
// 在线用户 → 物品 ID 命中 → 买卖方向 → 满级/满星 → 价格排序（买家取高、卖家取低）→ 截断 8 条。
func filterMarketOrders(orders []*draw.OrderWithUser, itemID string, buyer, maxRank bool, maxRankValue, maxAmberStars, maxCyanStars int) []*draw.OrderWithUser {
	want := drawplugin.TransSell
	if buyer {
		want = drawplugin.TransBuy
	}
	filtered := make([]*draw.OrderWithUser, 0, len(orders))
	for _, order := range orders {
		if order == nil {
			continue
		}
		if !marketUserOnline(order.User) {
			continue
		}
		if itemID != "" && order.ItemID != itemID {
			continue
		}
		if order.Type != want {
			continue
		}
		if !matchesMaxRank(maxRank, maxRankValue, maxAmberStars, maxCyanStars, order) {
			continue
		}
		filtered = append(filtered, order)
	}

	sort.SliceStable(filtered, func(i, j int) bool {
		left, right := orderPlatinum(filtered[i]), orderPlatinum(filtered[j])
		if buyer {
			return left > right
		}
		return left < right
	})
	if len(filtered) > marketOrdersLimit {
		filtered = filtered[:marketOrdersLimit]
	}
	return filtered
}

// matchesMaxRank 判定订单是否满足满级条件（对齐 Java matchesMaxRank 的判空结构）：
// 有 rank 时按物品 maxRank 比对；否则按琥珀星/靛蓝星**各自**的上限比对
// （阿耶檀识塑像的 maxRank 为 0，星际上限存在 maxAmberStars/maxCyanStars 且两者常不相等）；
// 均无则视为满足。
func matchesMaxRank(maxRank bool, maxRankValue, maxAmberStars, maxCyanStars int, order *draw.OrderWithUser) bool {
	if !maxRank {
		return true
	}
	if order.Rank != nil {
		return *order.Rank == maxRankValue
	}
	if order.AmberStars != nil && order.CyanStars != nil {
		return *order.AmberStars == maxAmberStars && *order.CyanStars == maxCyanStars
	}
	return true
}

// marketUserOnline 判定订单用户是否在线（对齐 Java 过滤 OFFLINE）。
func marketUserOnline(user *draw.MarketUser) bool {
	if user == nil {
		return false
	}
	return user.Status != drawplugin.MarketStatusOffline
}

// orderPlatinum 返回订单价格（nil 视为 0，供排序）。
func orderPlatinum(order *draw.OrderWithUser) int {
	if order == nil || order.Platinum == nil {
		return 0
	}
	return *order.Platinum
}

// parseMarketTime 解析市场 API 时间戳；失败返回零值（对齐 Jackson Instant 的宽松解析）。
func parseMarketTime(value string) time.Time {
	if value == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

// intPtr 返回 int 指针（绘图 DTO 的可空字段）。
func intPtr(value int) *int {
	return &value
}

// boolPtr 返回 bool 指针（绘图 DTO 的可空字段）。
func boolPtr(value bool) *bool {
	return &value
}
