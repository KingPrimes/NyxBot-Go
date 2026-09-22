// 市场紫卡拍卖查询数据层（/WR 指令），对齐 Java NyxBot 的 MarketRivenUtils + MarketRivenPlugin。
//
// 关键字格式：「武器名」或「武器名-正面词条1,正面词条2-有/无」。
// 流程：本地 riven_items 匹配武器 → 词条中文/别名转 url_name → /v1/auctions/search
// → 过滤（未关闭/可见/在线）→ 按买断价、起拍价、最高出价依次排序 → 截断 10 条
// → 词条 url_name 回译为中文效果。
package warframe

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"gorm.io/gorm"

	"nyxbot-go/internal/database"
	"nyxbot-go/internal/draw"
	"nyxbot-go/internal/logging"
	modelwarframe "nyxbot-go/internal/model/warframe"
)

// rivenAuctionsLimit 紫卡拍卖最大返回条数（对齐 Java limit(10)）。
const rivenAuctionsLimit = 10

// rivenSearchMasteryMin/Max 紫卡搜索段位区间（对齐 Java masteryRankMin/Max 默认与钳制）。
const (
	rivenSearchMasteryMin = 7
	rivenSearchMasteryMax = 16
)

// RivenAuctionsResult 紫卡拍卖查询结果。
// PossibleItems 非空表示未命中武器；否则 DTO 为可直接绘制的紫卡拍卖数据。
type RivenAuctionsResult struct {
	PossibleItems []string
	DTO           *draw.MarketRiven
}

// QueryRivenAuctions 查询紫卡拍卖（对齐 Java MarketRivenUtils.marketRivenParameter）。
// keyword 为去掉指令前缀后的原文。
func QueryRivenAuctions(api *MarketAPI, keyword string) (*RivenAuctionsResult, error) {
	if api == nil {
		api = DefaultMarketAPI()
	}
	key := strings.TrimSpace(keyword)
	if key == "" {
		return &RivenAuctionsResult{}, nil
	}

	weaponKey := key
	var positiveStats, negativeStats string
	if strings.Contains(key, "-") {
		segments := strings.Split(key, "-")
		weaponKey = strings.TrimSpace(segments[0])
		positiveStats = resolveStatURLNames(strings.Split(strings.ReplaceAll(segments[1], "，", ","), ","))
		negativeStats = parseNegativeStat(segments)
	}

	weapon := findRivenWeaponByKeyword(weaponKey)
	if weapon == nil {
		return &RivenAuctionsResult{PossibleItems: rivenItemCandidates(normalizeMarketInput(weaponKey), weaponKey)}, nil
	}

	searchURL := buildRivenSearchQuery(weapon.Slug, positiveStats, negativeStats)
	body, err := api.Get(api.baseURL + "/v1/auctions/search?" + searchURL)
	if err != nil {
		return nil, err
	}

	dto, err := parseRivenAuctions(body, weapon.Name)
	if err != nil {
		return nil, err
	}
	return &RivenAuctionsResult{DTO: dto}, nil
}

// findRivenWeaponByKeyword 按 Java getRiveItems 顺序匹配本地紫卡武器：
// Prime 展开 → 精确名 → 别名后精确名 → 前后缀正则。
func findRivenWeaponByKeyword(keyword string) *modelwarframe.RivenItem {
	key := strings.ToLower(strings.TrimSpace(keyword))
	if key == "" {
		return nil
	}
	primed := processPrimeKeyword(key)
	if hit := findRivenItemByName(primed); hit != nil {
		return hit
	}
	if aliased := processMarketAliases(primed); aliased != primed {
		if hit := findRivenItemByName(aliased); hit != nil {
			return hit
		}
		primed = aliased
	}
	return findRivenItemByNameRegex(primed)
}

// parseNegativeStat 解析负面词条参数（对齐 Java parseNegativeStat）：
// 第三段存在时「有」→ has、「无」→ none，否则为空（不附加参数）。
func parseNegativeStat(segments []string) string {
	if len(segments) <= 2 {
		return ""
	}
	return strings.NewReplacer("有", "has", "无", "none").Replace(strings.TrimSpace(segments[2]))
}

// resolveStatURLNames 把词条中文/别名解析为 url_name（对齐 Java resolveStatURLNames）：
// 先按 effect 精确匹配 riven_tion，再按 cn 匹配 riven_tion_alias；无法识别的词条丢弃。
func resolveStatURLNames(stats []string) string {
	if database.DB == nil {
		return ""
	}
	names := make([]string, 0, len(stats))
	for _, stat := range stats {
		effect := strings.TrimSpace(stat)
		if effect == "" {
			continue
		}
		if name := rivenTionURLName(effect); name != "" {
			names = append(names, name)
			continue
		}
		if name := rivenTionAliasURLName(effect); name != "" {
			names = append(names, name)
		}
	}
	return strings.Join(names, ",")
}

// rivenTionURLName 按 effect 查紫卡词条的 url_name。
func rivenTionURLName(effect string) string {
	if database.DB == nil {
		return ""
	}
	var tion modelwarframe.RivenTion
	if err := database.DB.Where("effect = ?", effect).First(&tion).Error; err != nil {
		if err != gorm.ErrRecordNotFound {
			logging.DebugPack("warframe.market", "query riven tion %q failed: %v", effect, err)
		}
		return ""
	}
	return tion.URLName
}

// rivenTionAliasURLName 按中文别名查紫卡词条的 url_name。
func rivenTionAliasURLName(effect string) string {
	if database.DB == nil {
		return ""
	}
	var alias modelwarframe.RivenTionAlias
	if err := database.DB.Where("cn = ?", effect).First(&alias).Error; err != nil {
		if err != gorm.ErrRecordNotFound {
			logging.DebugPack("warframe.market", "query riven tion alias %q failed: %v", effect, err)
		}
		return ""
	}
	return alias.En
}

// buildRivenSearchQuery 构造 /v1/auctions/search 查询串（对齐 Java MarketSearchResult.getUrl）：
// 固定 type=riven、段位钳制到 [7,16]、极性 any、价格升序；buyout_policy 为 any 时不附加。
func buildRivenSearchQuery(slug, positiveStats, negativeStats string) string {
	parts := []string{
		"type=riven",
		"weapon_url_name=" + slug,
	}
	if strings.TrimSpace(positiveStats) != "" {
		parts = append(parts, "positive_stats="+positiveStats)
	}
	if strings.TrimSpace(negativeStats) != "" {
		parts = append(parts, "negative_stats="+negativeStats)
	}
	parts = append(parts,
		fmt.Sprintf("mastery_rank_min=%d", rivenSearchMasteryMin),
		fmt.Sprintf("mastery_rank_max=%d", rivenSearchMasteryMax),
		"polarity=any",
		"sort_by=price_asc",
	)
	// Java 对整个查询串调用 toLowerCase，此处逐段小写化以保持一致
	return strings.ToLower(strings.Join(parts, "&"))
}

// rivenAuctionEnvelope 紫卡拍卖响应（/v1/auctions/search 返回 {"payload":{"auctions":[...]}}）。
// 嵌套结构复用 draw 包的 DTO，字段标签与 API 一致，故直接反序列化。
type rivenAuctionEnvelope struct {
	Payload struct {
		Auctions []*draw.MarketRivenAuction `json:"auctions"`
	} `json:"payload"`
}

// parseRivenAuctions 解析并过滤紫卡拍卖（对齐 Java stream + mapAttributeEffects）。
func parseRivenAuctions(body []byte, itemName string) (*draw.MarketRiven, error) {
	var envelope rivenAuctionEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("parse riven auctions: %w", err)
	}
	dto := &draw.MarketRiven{
		ItemName: itemName,
		Payload:  &draw.MarketRivenPayload{},
	}
	for _, auction := range envelope.Payload.Auctions {
		if auction == nil || !rivenAuctionUsable(auction) {
			continue
		}
		mapAttributeEffects(auction)
		dto.Payload.Auctions = append(dto.Payload.Auctions, auction)
	}
	sort.SliceStable(dto.Payload.Auctions, func(i, j int) bool {
		return compareRivenPrice(dto.Payload.Auctions[i], dto.Payload.Auctions[j]) < 0
	})
	if len(dto.Payload.Auctions) > rivenAuctionsLimit {
		dto.Payload.Auctions = dto.Payload.Auctions[:rivenAuctionsLimit]
	}
	return dto, nil
}

// rivenAuctionUsable 判定拍卖是否可用（对齐 Java stream 过滤）：
// 未关闭、可见、卖家在线或游戏内。
func rivenAuctionUsable(auction *draw.MarketRivenAuction) bool {
	if auction.Closed != nil && *auction.Closed {
		return false
	}
	if auction.Visible != nil && !*auction.Visible {
		return false
	}
	if auction.Owner == nil {
		return false
	}
	status := strings.ToLower(auction.Owner.Status)
	return status == "online" || status == "ingame"
}

// compareRivenPrice 按买断价 → 起拍价 → 最高出价依次比较（对齐 Java compareByPrice）。
// 任一层双方都有值时即返回结果；三层都无法比较时返回 0。
func compareRivenPrice(left, right *draw.MarketRivenAuction) int {
	if left.BuyoutPrice != nil && right.BuyoutPrice != nil {
		return *left.BuyoutPrice - *right.BuyoutPrice
	}
	if left.StartingPrice != nil && right.StartingPrice != nil {
		return *left.StartingPrice - *right.StartingPrice
	}
	if left.TopBid != nil && right.TopBid != nil {
		return *left.TopBid - *right.TopBid
	}
	return 0
}

// mapAttributeEffects 把词条 url_name 回译为中文效果（对齐 Java mapAttributeEffects：
// 查 riven_tion.byUrlName 取 effect，未命中置空）。
func mapAttributeEffects(auction *draw.MarketRivenAuction) {
	if auction.Item == nil {
		return
	}
	for _, attribute := range auction.Item.Attributes {
		if attribute == nil {
			continue
		}
		attribute.URLName = rivenTionEffect(attribute.URLName)
	}
}

// rivenTionEffect 按 url_name 查中文效果（未命中返回空串，对齐 Java orElse(new RivenTion())）。
func rivenTionEffect(urlName string) string {
	if database.DB == nil || urlName == "" {
		return ""
	}
	var tion modelwarframe.RivenTion
	if err := database.DB.Where("url_name = ?", urlName).First(&tion).Error; err != nil {
		if err != gorm.ErrRecordNotFound {
			logging.DebugPack("warframe.market", "query riven effect %q failed: %v", urlName, err)
		}
		return ""
	}
	return tion.Effect
}
