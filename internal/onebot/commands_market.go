// 阶段10 市场类指令：/WM（市场订单）、/WR（紫卡拍卖）、/CD（赤毒玄骸武器）、
// /XT（信条武器）、金垃圾、银垃圾。
// 对齐 Java MarketOrdersPlugin / MarketRivenPlugin / MarketLichesPlugin /
// MarketSistersPlugin / MarketGodDumpPlugin / MarketSilverDumpPlugin。
package onebot

import (
	"sort"
	"strings"

	zero "github.com/wdvxdr1123/ZeroBot"

	"nyxbot-go/internal/draw"
	"nyxbot-go/internal/logging"
	"nyxbot-go/internal/warframe"
)

// 市场指令的平台关键字（对齐 Java MarketPlatformEnum 的 platform 字段）。
// 平台关键字按长度降序匹配，避免 "PC" 命中 "PC" 之外的短前缀歧义。
var marketPlatformKeywords = []struct {
	Keyword  string
	Platform string
}{
	{"SWITCH", "switch"},
	{"MOBILE", "mobile"},
	{"XBOX", "xbox"},
	{"PS4", "ps4"},
	{"PC", "pc"},
}

// 市场订单买卖方向关键字（对齐 Java processBuySellParam）。
var (
	marketBuyerKeywords = []string{"购买", "买家", "BUY", "buy"}
	marketMaxKeywords   = []string{"满级", "MAX", "max"}
)

// wfMarketOrders 处理「/WM/市场」：市场订单查询，未命中物品时给出候选列表。
func (registry *CommandRegistry) wfMarketOrders(ctx *zero.Ctx, parameter string) error {
	keyword := stripMarketAlias(parameter, "/WM", "WM", "MARKET", "/市场", "市场", "/wm", "wm", "market")
	buyer, maxRank, platform, keyword := parseMarketOrderParams(keyword)
	if keyword == "" {
		return ReplyText(ctx, "请输入正确的名称！")
	}

	result, err := warframe.QueryMarketOrders(warframe.DefaultMarketAPI(), keyword, buyer, maxRank, platform)
	if err != nil {
		logging.WarnPack("onebot.command", "query market orders %q failed: %v", keyword, err)
		return ReplyText(ctx, warframe.MarketErrorText(err))
	}
	if len(result.PossibleItems) > 0 {
		return replyPossibleItems(ctx, result.PossibleItems, "未找到该物品，你可能想查询：")
	}
	if result.Item == nil || len(result.Orders) == 0 {
		return ReplyText(ctx, "未找到符合条件的订单，请稍后再试")
	}
	return ReplyImage(ctx, draw.DrawMarketOrders(result.Item))
}

// wfMarketRiven 处理「/WR」：紫卡拍卖查询。
func (registry *CommandRegistry) wfMarketRiven(ctx *zero.Ctx, parameter string) error {
	keyword := stripMarketAlias(parameter, "/WR", "WR", "WMR", "/wr", "wr", "wmr")
	if keyword == "" {
		return ReplyText(ctx, "请输入正确的指令！")
	}

	result, err := warframe.QueryRivenAuctions(warframe.DefaultMarketAPI(), keyword)
	if err != nil {
		logging.WarnPack("onebot.command", "query riven auctions %q failed: %v", keyword, err)
		return ReplyText(ctx, warframe.MarketErrorText(err))
	}
	if len(result.PossibleItems) > 0 {
		return replyPossibleItems(ctx, result.PossibleItems, "未找到该武器，你可能想查询：")
	}
	if result.DTO == nil || result.DTO.Payload == nil || len(result.DTO.Payload.Auctions) == 0 {
		return ReplyText(ctx, "未查询到符合条件的紫卡拍卖")
	}
	return ReplyImage(ctx, draw.DrawMarketRiven(result.DTO))
}

// wfRivenMarket 处理「/RM」：对齐 Java 本体，暂未实现具体查询。
func (registry *CommandRegistry) wfRivenMarket(ctx *zero.Ctx, _ string) error {
	return ReplyText(ctx, "该功能暂未实现！")
}

// wfMarketLichs 处理「/CD/赤毒」：赤毒玄骸武器拍卖查询。
func (registry *CommandRegistry) wfMarketLichs(ctx *zero.Ctx, parameter string) error {
	return registry.marketLichSister(ctx, parameter, warframe.LichSisterLich,
		"/CD", "CD", "赤毒", "/cd", "cd")
}

// wfMarketSisters 处理「/XT/信条」：帕尔沃斯姐妹武器拍卖查询。
func (registry *CommandRegistry) wfMarketSisters(ctx *zero.Ctx, parameter string) error {
	return registry.marketLichSister(ctx, parameter, warframe.LichSisterSister,
		"/XT", "XT", "信条", "/xt", "xt")
}

// marketLichSister 赤毒/信条拍卖查询的共用实现（对齐 Java AbstractMarketLichSisterPlugin.handle）。
func (registry *CommandRegistry) marketLichSister(
	ctx *zero.Ctx,
	parameter string,
	searchType warframe.WfMarketLichSisterType,
	aliases ...string,
) error {
	keyword := stripMarketAlias(parameter, aliases...)
	if keyword == "" {
		return ReplyText(ctx, "请输入正确的指令！")
	}

	result, err := warframe.QueryLichSisterAuctions(warframe.DefaultMarketAPI(), keyword, searchType)
	if err != nil {
		logging.WarnPack("onebot.command", "query %s auctions %q failed: %v", searchType, keyword, err)
		return ReplyText(ctx, warframe.MarketErrorText(err))
	}
	if len(result.PossibleItems) > 0 {
		return replyPossibleItems(ctx, result.PossibleItems, "未找到该武器，你可能想查询：")
	}
	if result.DTO == nil || result.DTO.Payload == nil || len(result.DTO.Payload.Auctions) == 0 {
		return ReplyText(ctx, "未查询到符合条件的拍卖")
	}
	return ReplyImage(ctx, draw.DrawMarketLichSister(result.DTO))
}

// wfMarketGodDump 处理「金垃圾」：杜卡币 100 的遗物部件性价比排行。
func (registry *CommandRegistry) wfMarketGodDump(ctx *zero.Ctx, _ string) error {
	return registry.marketDucatsDump(ctx, warframe.DucatsDumpGod, "金垃圾")
}

// wfMarketSilverDump 处理「银垃圾」：杜卡币 45-99 的遗物部件性价比排行。
func (registry *CommandRegistry) wfMarketSilverDump(ctx *zero.Ctx, _ string) error {
	return registry.marketDucatsDump(ctx, warframe.DucatsDumpSilver, "银垃圾")
}

// marketDucatsDump 金/银垃圾查询的共用实现（对齐 Java AbstractMarketDumpPlugin.handle）。
func (registry *CommandRegistry) marketDucatsDump(ctx *zero.Ctx, dumpType warframe.DucatsDumpType, title string) error {
	result, err := warframe.QueryDucatsDump(warframe.DefaultMarketAPI(), dumpType)
	if err != nil {
		logging.WarnPack("onebot.command", "query ducats %s failed: %v", dumpType, err)
		return ReplyText(ctx, warframe.MarketErrorText(err))
	}
	if result == nil || (len(result.Day) == 0 && len(result.Hour) == 0) {
		return ReplyText(ctx, "获取 ducat 数据失败，请稍后再试")
	}
	image := draw.DrawMarketDucats(ducatsToDraw(result), title)
	if len(image) == 0 {
		return ReplyText(ctx, "生成"+title+"图片失败，请稍后再试")
	}
	return ReplyImage(ctx, image)
}

// ducatsToDraw 将杜卡币查询结果转换为绘图输入。
func ducatsToDraw(result *warframe.DucatsDumpResult) *draw.Ducats {
	convert := func(entries []*warframe.DucatsEntry) []*draw.DucatsEntry {
		out := make([]*draw.DucatsEntry, 0, len(entries))
		for _, entry := range entries {
			if entry == nil {
				continue
			}
			ducats := entry.Ducats
			ratio := entry.DucatsPerPlatinumWa
			out = append(out, &draw.DucatsEntry{
				Item:                entry.Item,
				Ducats:              &ducats,
				DucatsPerPlatinumWa: &ratio,
			})
		}
		return out
	}
	return &draw.Ducats{Day: convert(result.Day), Hour: convert(result.Hour)}
}

// parseMarketOrderParams 解析市场订单参数（对齐 Java processMaxLevelParam /
// processBuySellParam / processPlatformParam 的固定顺序）：
// 满级 → 买卖方向 → 平台，剩余部分为搜索关键字。
func parseMarketOrderParams(input string) (buyer, maxRank bool, platform, keyword string) {
	processed := input

	if matched, replaced := stripFirstMatch(processed, marketMaxKeywords); matched {
		maxRank = true
		processed = replaced
	}
	if matched, replaced := stripFirstMatch(processed, marketBuyerKeywords); matched {
		buyer = true
		processed = replaced
	}
	for _, candidate := range marketPlatformKeywords {
		if strings.Contains(strings.ToUpper(processed), candidate.Keyword) {
			platform = candidate.Platform
			processed = removeAllFold(processed, candidate.Keyword)
			break
		}
	}
	return buyer, maxRank, platform, strings.Join(strings.Fields(processed), " ")
}

// stripFirstMatch 去除 keywords 中首个命中项，返回是否命中与处理后的字符串。
func stripFirstMatch(input string, keywords []string) (bool, string) {
	for _, keyword := range keywords {
		if strings.Contains(input, keyword) {
			return true, strings.ReplaceAll(input, keyword, "")
		}
	}
	return false, input
}

// removeAllFold 大小写不敏感地移除全部 keyword 出现（保持其余字符原样）。
func removeAllFold(input, keyword string) string {
	if keyword == "" {
		return input
	}
	foldedKeyword := strings.ToUpper(keyword)
	var builder strings.Builder
	for i := 0; i < len(input); {
		if i+len(keyword) <= len(input) && strings.ToUpper(input[i:i+len(keyword)]) == foldedKeyword {
			i += len(keyword)
			continue
		}
		builder.WriteByte(input[i])
		i++
	}
	return builder.String()
}

// stripMarketAlias 去掉参数开头残留的指令别名。
// 指令正则只匹配前缀（如 ^/WM），个别别名（MARKET、赤毒）会与关键字一起落入参数，
// 故在此按别名长度降序再剥一次，得到纯搜索关键字。
func stripMarketAlias(parameter string, aliases ...string) string {
	trimmed := strings.TrimSpace(parameter)
	if trimmed == "" {
		return ""
	}
	upper := strings.ToUpper(trimmed)
	// 先处理带 "/" 的显式前缀
	for _, alias := range aliases {
		if !strings.HasPrefix(alias, "/") {
			continue
		}
		if strings.HasPrefix(upper, strings.ToUpper(alias)) {
			trimmed = strings.TrimSpace(trimmed[len(alias):])
			upper = strings.ToUpper(trimmed)
		}
	}
	// 再处理无 "/" 的别名（按长度降序，避免短别名吃掉长别名）
	sorted := append([]string(nil), aliases...)
	sort.SliceStable(sorted, func(i, j int) bool { return len(sorted[i]) > len(sorted[j]) })
	for _, alias := range sorted {
		if strings.HasPrefix(alias, "/") || alias == "" {
			continue
		}
		if strings.HasPrefix(upper, strings.ToUpper(alias)) {
			trimmed = strings.TrimSpace(trimmed[len(alias):])
			upper = strings.ToUpper(trimmed)
			break
		}
	}
	return trimmed
}

// replyPossibleItems 回复候选物品列表（对齐 Java drawMarketOrdersImage(possibleItems)）。
func replyPossibleItems(ctx *zero.Ctx, items []string, header string) error {
	if len(items) == 0 {
		return ReplyText(ctx, "未找到匹配的物品，请检查名称是否正确")
	}
	if image := draw.DrawMarketOrdersList(items); len(image) > 0 {
		return ReplyImage(ctx, image)
	}
	text := header + "\n" + strings.Join(items, "、")
	return ReplyText(ctx, text)
}
