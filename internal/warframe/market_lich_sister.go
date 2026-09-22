// 赤毒/信条武器拍卖与杜卡德币（金/银垃圾）查询数据层，
// 对齐 Java NyxBot 的 MarketLichSisterUtils / MarketDucatsUtils 与三个对应插件。
package warframe

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"nyxbot-go/internal/database"
	"nyxbot-go/internal/draw"
	"nyxbot-go/internal/enum/drawplugin"
	"nyxbot-go/internal/logging"
	modelwarframe "nyxbot-go/internal/model/warframe"
)

// WfMarketLichSisterType 赤毒/信条拍卖搜索类型（对齐 Java SearchType）。
type WfMarketLichSisterType string

const (
	// LichSisterLich 赤毒玄骸武器（type=lich）
	LichSisterLich WfMarketLichSisterType = "lich"
	// LichSisterSister 帕尔沃斯姐妹武器（type=sister）
	LichSisterSister WfMarketLichSisterType = "sister"
)

// 赤毒/信条拍卖搜索参数（对齐 Java MarketSearchResult 的 damageMin/Max 与 limit(10)）。
const (
	lichSisterDamageMin = 25
	lichSisterDamageMax = 60
	lichSisterLimit     = 10
)

// 杜卡德币筛选区间（对齐 Java getSilverDump / getGodDump）。
const (
	silverDumpMinDucats = 45
	silverDumpMaxDucats = 100
	godDumpDucats       = 100
	ducatsDumpLimit     = 10
)

// LichSisterAuctionsResult 赤毒/信条武器拍卖查询结果。
type LichSisterAuctionsResult struct {
	PossibleItems []string
	DTO           *draw.MarketLichSister
}

// QueryLichSisterAuctions 查询赤毒/信条武器拍卖（对齐 Java MarketLichSisterUtils.getAuctions）。
func QueryLichSisterAuctions(api *MarketAPI, keyword string, searchType WfMarketLichSisterType) (*LichSisterAuctionsResult, error) {
	if api == nil {
		api = DefaultMarketAPI()
	}
	weapon := findLichSisterWeaponByKeyword(keyword)
	if weapon == nil {
		return &LichSisterAuctionsResult{PossibleItems: lichSisterCandidates(keyword, keyword)}, nil
	}

	query := buildLichSisterSearchQuery(string(searchType), weapon.Slug)
	body, err := api.Get(api.baseURL + "/v1/auctions/search?" + query)
	if err != nil {
		return nil, err
	}
	dto, err := parseLichSisterAuctions(body, weapon.Name)
	if err != nil {
		return nil, err
	}
	return &LichSisterAuctionsResult{DTO: dto}, nil
}

// findLichSisterWeaponByKeyword 按 Java queryLichSisterWeapons 顺序匹配本地武器：
// 精确名 → 模糊名 → 别名后精确名 → 别名后模糊名 → 前后缀正则（原文与别名各一次）。
func findLichSisterWeaponByKeyword(keyword string) *modelwarframe.LichSisterWeapon {
	key := strings.TrimSpace(keyword)
	if key == "" {
		return nil
	}
	if hit := findLichSisterByName(key); hit != nil {
		return hit
	}
	if hit := findLichSisterByNameLike(key); hit != nil {
		return hit
	}
	aliased := processMarketAliases(key)
	if hit := findLichSisterByName(aliased); hit != nil {
		return hit
	}
	if hit := findLichSisterByNameLike(aliased); hit != nil {
		return hit
	}
	if hit := findLichSisterByNameRegex(key); hit != nil {
		return hit
	}
	return findLichSisterByNameRegex(aliased)
}

// buildLichSisterSearchQuery 构造赤毒/信条拍卖查询串（对齐 Java MarketSearchResult.getUrl）：
// 固定 has_ephemera=false、价格升序、伤害区间钳制到 [25,60]；元素为 any 时不附加。
func buildLichSisterSearchQuery(searchType, slug string) string {
	parts := []string{
		"type=" + searchType,
		"has_ephemera=false",
		"sort_by=price_asc",
		"weapon_url_name=" + slug,
		fmt.Sprintf("damage_min=%d", lichSisterDamageMin),
		fmt.Sprintf("damage_max=%d", lichSisterDamageMax),
	}
	return strings.Join(parts, "&")
}

// lichSisterAuctionEnvelope 赤毒/信条拍卖响应（{"payload":{"auctions":[...]}}）。
type lichSisterAuctionEnvelope struct {
	Payload struct {
		Auctions []*draw.MarketLichSisterAuction `json:"auctions"`
	} `json:"payload"`
}

// parseLichSisterAuctions 解析并过滤拍卖（对齐 Java processAuctionData）：
// 未关闭 → 可见 → 卖家在线/游戏内 → 按买断/起拍/最高出价排序 → 截断 10 条。
func parseLichSisterAuctions(body []byte, itemName string) (*draw.MarketLichSister, error) {
	var envelope lichSisterAuctionEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("parse lich sister auctions: %w", err)
	}
	dto := &draw.MarketLichSister{
		Payload: &draw.MarketLichSisterPayload{ItemName: itemName},
	}
	for _, auction := range envelope.Payload.Auctions {
		if auction == nil || !lichSisterAuctionUsable(auction) {
			continue
		}
		normalizeLichSisterAuction(auction)
		dto.Payload.Auctions = append(dto.Payload.Auctions, auction)
	}
	sort.SliceStable(dto.Payload.Auctions, func(i, j int) bool {
		return compareLichSisterPrice(dto.Payload.Auctions[i], dto.Payload.Auctions[j]) < 0
	})
	if len(dto.Payload.Auctions) > lichSisterLimit {
		dto.Payload.Auctions = dto.Payload.Auctions[:lichSisterLimit]
	}
	return dto, nil
}

// lichSisterAuctionUsable 判定拍卖是否可用（对齐 Java processAuctionData 的过滤链）。
func lichSisterAuctionUsable(auction *draw.MarketLichSisterAuction) bool {
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
	return status == "ingame" || status == "online"
}

// normalizeLichSisterAuction 规范元素枚举大小写（API 返回小写，绘图侧按大写常量匹配）。
func normalizeLichSisterAuction(auction *draw.MarketLichSisterAuction) {
	if auction.Item == nil {
		return
	}
	auction.Item.Element = elementFromAPI(string(auction.Item.Element))
}

// compareLichSisterPrice 按买断价 → 起拍价 → 最高出价比较（对齐 Java processAuctionData 排序）。
func compareLichSisterPrice(left, right *draw.MarketLichSisterAuction) int {
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

// elementFromAPI 将 API 的小写元素名转为枚举常量（未知值原样大写返回）。
func elementFromAPI(value string) drawplugin.Element {
	return drawplugin.Element(strings.ToUpper(value))
}

// DucatsDumpType 垃圾类型（对齐 Java DucatsType + DumpType）。
type DucatsDumpType string

const (
	// DucatsDumpSilver 银垃圾（45 ≤ 杜卡币 < 100）
	DucatsDumpSilver DucatsDumpType = "SILVER"
	// DucatsDumpGod 金垃圾（杜卡币 = 100）
	DucatsDumpGod DucatsDumpType = "GOD"
)

// DucatsEntry 杜卡德币排行条目（对齐 Java Ducats.Ducat，仅保留指令所需字段）。
type DucatsEntry struct {
	Item                string  // 物品名（由本地 orders_items 翻译后的展示名）
	ItemID              string  // 物品 ID
	Ducats              int     // 杜卡币
	Platinum            int     // 白金
	DucatsPerPlatinumWa float64 // 实时「1 白金 = ? 杜卡币」
}

// DucatsDumpResult 金/银垃圾查询结果：当天与最近一小时两个榜单。
type DucatsDumpResult struct {
	Day  []*DucatsEntry
	Hour []*DucatsEntry
}

// ducatsEnvelope /v1/tools/ducats 响应（payload.previous_day / previous_hour）。
type ducatsEnvelope struct {
	Payload struct {
		PreviousDay  []*ducatsDTO `json:"previous_day"`
		PreviousHour []*ducatsDTO `json:"previous_hour"`
	} `json:"payload"`
}

// ducatsDTO 杜卡德币原始条目。
type ducatsDTO struct {
	Ducats              int     `json:"ducats"`
	Item                string  `json:"item"`
	DucatsPerPlatinumWa float64 `json:"ducats_per_platinum_wa"`
}

// QueryDucatsDump 查询金/银垃圾榜单（对齐 Java MarketDucatsUtils.getDucats + getDuats）。
func QueryDucatsDump(api *MarketAPI, dumpType DucatsDumpType) (*DucatsDumpResult, error) {
	if api == nil {
		api = DefaultMarketAPI()
	}
	body, err := api.Get(api.baseURL + "/v1/tools/ducats")
	if err != nil {
		return nil, err
	}
	var envelope ducatsEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("parse ducats: %w", err)
	}
	if len(envelope.Payload.PreviousDay) == 0 && len(envelope.Payload.PreviousHour) == 0 {
		return nil, nil
	}
	return &DucatsDumpResult{
		Day:  filterDucats(envelope.Payload.PreviousDay, dumpType),
		Hour: filterDucats(envelope.Payload.PreviousHour, dumpType),
	}, nil
}

// filterDucats 按垃圾类型筛选、翻译物品名、按实时性价比降序并截断 10 条。
func filterDucats(entries []*ducatsDTO, dumpType DucatsDumpType) []*DucatsEntry {
	filtered := make([]*DucatsEntry, 0, len(entries))
	for _, entry := range entries {
		if entry == nil || !ducatsInRange(entry.Ducats, dumpType) {
			continue
		}
		filtered = append(filtered, &DucatsEntry{
			Item:                ordersItemNameByID(entry.Item),
			ItemID:              entry.Item,
			Ducats:              entry.Ducats,
			DucatsPerPlatinumWa: entry.DucatsPerPlatinumWa,
		})
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		return filtered[i].DucatsPerPlatinumWa > filtered[j].DucatsPerPlatinumWa
	})
	if len(filtered) > ducatsDumpLimit {
		filtered = filtered[:ducatsDumpLimit]
	}
	return filtered
}

// ducatsInRange 判定杜卡币数是否属于指定垃圾类型（对齐 Java getSilverDump / getGodDump）。
func ducatsInRange(ducats int, dumpType DucatsDumpType) bool {
	if dumpType == DucatsDumpGod {
		return ducats == godDumpDucats
	}
	return ducats >= silverDumpMinDucats && ducats < silverDumpMaxDucats
}

// ordersItemNameByID 按物品 ID 查本地市场物品名（未命中返回空串，对齐 Java orElse(new OrdersItems())）。
func ordersItemNameByID(id string) string {
	if database.DB == nil || id == "" {
		return ""
	}
	item := firstOrdersItem("id = ?", id)
	if item == nil {
		logging.DebugPack("warframe.market", "ducats item %q not found locally", id)
		return ""
	}
	return item.Name
}
