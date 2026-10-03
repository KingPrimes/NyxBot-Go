// 市场指令的本地物品匹配工具，对齐 Java NyxBot 的 MarketCommonUtils / MarketOrderUtils。
// 匹配在本地数据库完成（orders_items / riven_items / lich_sister_weapons），
// 命中后返回条目 slug 供 warframe.market API 查询；未命中则返回候选名称列表。
//
// 正则匹配说明：Java 用 JPA 的 findByNameRegex（底层 REGEXP），Go 侧
// glebarez/sqlite 无 REGEXP 扩展，故改为「LIKE 缩小候选 + Go regexp 逐条匹配」，
// 结果与 Java 的前后缀正则语义一致，且不依赖数据库方言。
package warframe

import (
	"regexp"
	"strings"

	"nyxbot-go/internal/database"
	"nyxbot-go/internal/logging"
	modelwarframe "nyxbot-go/internal/model/warframe"
)

// normalizeMarketInput 标准化输入关键字（对齐 Java MarketCommonUtils.normalizeInput）：
// 转小写 + 「总图」→「蓝图」。
func normalizeMarketInput(input string) string {
	return strings.ReplaceAll(strings.ToLower(input), "总图", "蓝图")
}

// processMarketAliases 按记录顺序做首个命中的中文别名替换（对齐 Java processAliases：
// 遍历全部别名，命中即替换并返回）。表为空或数据库不可用时原样返回。
func processMarketAliases(input string) string {
	if input == "" || database.DB == nil {
		return input
	}
	var aliases []modelwarframe.Alias
	if err := database.DB.Order("id").Find(&aliases).Error; err != nil {
		logging.DebugPack("warframe.market", "load aliases failed: %v", err)
		return input
	}
	for _, alias := range aliases {
		if alias.Cn != "" && strings.Contains(input, alias.Cn) {
			return strings.ReplaceAll(input, alias.Cn, alias.En)
		}
	}
	return input
}

// processPrimeKeyword 将 Prime 简写 "p" 展开为 "Prime"（对齐 Java processPrimeKeyword：
// 已含 prime 时不处理；否则把全部 "p" 逐字替换）。
func processPrimeKeyword(key string) string {
	if !strings.Contains(key, "prime") && strings.Contains(key, "p") {
		return strings.ReplaceAll(key, "p", "Prime")
	}
	return key
}

// regexNamePatterns 生成前后缀正则候选模式（对齐 Java tryRegexNameMatch）：
// P 简写形态（首.*?p.*?尾.*?）优先，随后为前缀长度 4→1 递减的「前缀.*?尾.*?」。
// 调用方保证 key 已小写化；模式均按大小写不敏感编译。
func regexNamePatterns(key string) []*regexp.Regexp {
	runes := []rune(key)
	if len(runes) < 2 {
		return nil
	}
	end := regexp.QuoteMeta(string(runes[len(runes)-1]))
	patterns := make([]*regexp.Regexp, 0, 5)

	appendPattern := func(prefix string) {
		compiled, err := regexp.Compile("(?i)^" + regexp.QuoteMeta(prefix) + ".*?" + end + ".*?")
		if err != nil {
			return
		}
		patterns = append(patterns, compiled)
	}

	if strings.Contains(key, "p") && string(runes[len(runes)-1]) != "p" {
		appendPattern(string(runes[0]) + ".*?p.*?")
	}
	maxPrefix := len(runes) - 1
	if maxPrefix > 4 {
		maxPrefix = 4
	}
	for prefixLen := maxPrefix; prefixLen >= 1; prefixLen-- {
		appendPattern(string(runes[:prefixLen]))
	}
	return patterns
}

// matchByRegex 在候选条目上依次套用前后缀正则，返回首个命中的条目。
// likePrefix 为 LIKE 缩小范围的键（取 key 首字符），避免全表扫描。
func matchByRegex[T any](key string, likePrefix string, loadCandidates func(prefix string) []T, nameOf func(*T) string) *T {
	patterns := regexNamePatterns(key)
	if len(patterns) == 0 {
		return nil
	}
	candidates := loadCandidates(likePrefix)
	for _, pattern := range patterns {
		for i := range candidates {
			if pattern.MatchString(nameOf(&candidates[i])) {
				return &candidates[i]
			}
		}
	}
	return nil
}

// findOrdersItemByName 精确匹配市场物品名（对齐 Java findByItemName）。
func findOrdersItemByName(key string) *modelwarframe.OrdersItem {
	return firstOrdersItem("name = ?", key)
}

// findOrdersItemByNameLike 模糊匹配市场物品名（对齐 Java findByItemNameLike）。
func findOrdersItemByNameLike(key string) *modelwarframe.OrdersItem {
	return firstOrdersItem("name LIKE ?", "%"+key+"%")
}

// findOrdersItemByNameRegex 前后缀正则匹配市场物品名（对齐 Java findByNameRegex）。
func findOrdersItemByNameRegex(key string) *modelwarframe.OrdersItem {
	return matchByRegex(key, firstRune(key), ordersItemsLike, func(item *modelwarframe.OrdersItem) string {
		return item.Name
	})
}

// findRivenItemByName 精确匹配紫卡武器名（对齐 Java RivenItemsRepository.findByName）。
func findRivenItemByName(key string) *modelwarframe.RivenItem {
	return firstRivenItem("name = ?", key)
}

// findRivenItemByNameLike 模糊匹配紫卡武器名。
func findRivenItemByNameLike(key string) *modelwarframe.RivenItem {
	return firstRivenItem("name LIKE ?", "%"+key+"%")
}

// findRivenItemByNameRegex 前后缀正则匹配紫卡武器名。
func findRivenItemByNameRegex(key string) *modelwarframe.RivenItem {
	return matchByRegex(key, firstRune(key), rivenItemsLike, func(item *modelwarframe.RivenItem) string {
		return item.Name
	})
}

// findLichSisterByName 精确匹配赤毒/信条武器名。
func findLichSisterByName(key string) *modelwarframe.LichSisterWeapon {
	return firstLichSisterWeapon("name = ?", key)
}

// findLichSisterByNameLike 模糊匹配赤毒/信条武器名。
func findLichSisterByNameLike(key string) *modelwarframe.LichSisterWeapon {
	return firstLichSisterWeapon("name LIKE ?", "%"+key+"%")
}

// findLichSisterByNameRegex 前后缀正则匹配赤毒/信条武器名。
func findLichSisterByNameRegex(key string) *modelwarframe.LichSisterWeapon {
	return matchByRegex(key, firstRune(key), lichSisterWeaponsLike, func(item *modelwarframe.LichSisterWeapon) string {
		return item.Name
	})
}

// firstOrdersItem 按条件查询单条市场物品（按 id 稳定排序，无结果或库不可用时返回 nil）。
func firstOrdersItem(query string, args ...any) *modelwarframe.OrdersItem {
	return firstMarketItem[modelwarframe.OrdersItem]("orders item", query, args...)
}

// firstRivenItem 按条件查询单条紫卡武器。
func firstRivenItem(query string, args ...any) *modelwarframe.RivenItem {
	return firstMarketItem[modelwarframe.RivenItem]("riven item", query, args...)
}

// firstLichSisterWeapon 按条件查询单条赤毒/信条武器。
func firstLichSisterWeapon(query string, args ...any) *modelwarframe.LichSisterWeapon {
	return firstMarketItem[modelwarframe.LichSisterWeapon]("lich sister weapon", query, args...)
}

// firstMarketItem 按条件查询单条市场模型（按 id 稳定排序）。
func firstMarketItem[T any](name, query string, args ...any) *T {
	if database.DB == nil {
		return nil
	}
	var item T
	if err := database.DB.Where(query, args...).Order("id").First(&item).Error; err != nil {
		logging.DebugPack("warframe.market", "query %s failed: %v", name, err)
		return nil
	}
	return &item
}

// ordersItemsLike 按 name LIKE %prefix% 载入市场物品候选。
func ordersItemsLike(prefix string) []modelwarframe.OrdersItem {
	return marketItemsLike[modelwarframe.OrdersItem]("orders items", prefix)
}

// rivenItemsLike 按 name LIKE %prefix% 载入紫卡武器候选。
func rivenItemsLike(prefix string) []modelwarframe.RivenItem {
	return marketItemsLike[modelwarframe.RivenItem]("riven items", prefix)
}

// lichSisterWeaponsLike 按 name LIKE %prefix% 载入赤毒/信条武器候选。
func lichSisterWeaponsLike(prefix string) []modelwarframe.LichSisterWeapon {
	return marketItemsLike[modelwarframe.LichSisterWeapon]("lich sister weapons", prefix)
}

// marketItemsLike 按名称片段载入市场模型候选。
func marketItemsLike[T any](name, prefix string) []T {
	if database.DB == nil || prefix == "" {
		return nil
	}
	var items []T
	if err := database.DB.Where("name LIKE ?", "%"+prefix+"%").Order("id").Find(&items).Error; err != nil {
		logging.DebugPack("warframe.market", "load %s %q failed: %v", name, prefix, err)
		return nil
	}
	return items
}

// ordersItemCandidates 返回市场物品候选名（对齐 Java getPossibleItems）：
// 先按首字符模糊匹配截断 15 条；为空时退化为「去掉尾字符」后的模糊匹配。
func ordersItemCandidates(key, original string) []string {
	prefix := candidatePrefix(key, original)
	if prefix == "" {
		return nil
	}
	if names := ordersItemNames(ordersItemsLike(prefix), 15); len(names) > 0 {
		return names
	}
	return ordersItemNames(ordersItemsLike(dropLastRune(prefix)), 15)
}

// rivenItemCandidates 返回紫卡武器候选名（对齐 Java MarketRivenUtils.getRiveItems 第 4 步）。
func rivenItemCandidates(key, original string) []string {
	prefix := candidatePrefix(key, original)
	if prefix == "" {
		return nil
	}
	return rivenItemNames(rivenItemsLike(prefix), 15)
}

// lichSisterCandidates 返回赤毒/信条武器候选名（对齐 Java getPossibleItems，Java 未截断）。
func lichSisterCandidates(key, original string) []string {
	prefix := candidatePrefix(key, original)
	if prefix == "" {
		return nil
	}
	return lichSisterNames(lichSisterWeaponsLike(prefix), 0)
}

// ordersItemNames 提取候选名并截断 limit 条（limit<=0 表示不截断）。
func ordersItemNames(items []modelwarframe.OrdersItem, limit int) []string {
	return marketItemNames(items, limit, func(item *modelwarframe.OrdersItem) string { return item.Name })
}

// rivenItemNames 提取紫卡候选名并截断 limit 条。
func rivenItemNames(items []modelwarframe.RivenItem, limit int) []string {
	return marketItemNames(items, limit, func(item *modelwarframe.RivenItem) string { return item.Name })
}

// lichSisterNames 提取赤毒/信条候选名并截断 limit 条。
func lichSisterNames(weapons []modelwarframe.LichSisterWeapon, limit int) []string {
	return marketItemNames(weapons, limit, func(item *modelwarframe.LichSisterWeapon) string { return item.Name })
}

// marketItemNames 提取非空候选名并按需截断。
func marketItemNames[T any](items []T, limit int, nameOf func(*T) string) []string {
	names := make([]string, 0, len(items))
	for i := range items {
		name := nameOf(&items[i])
		if name == "" {
			continue
		}
		names = append(names, name)
		if limit > 0 && len(names) >= limit {
			break
		}
	}
	return names
}

// candidatePrefix 取候选查询前缀：优先用处理后的 key 首字符，key 为空时回退原始输入。
func candidatePrefix(key, original string) string {
	if prefix := firstRune(key); prefix != "" {
		return prefix
	}
	return firstRune(original)
}

// firstRune 返回字符串首字符（空串返回空）。
func firstRune(text string) string {
	runes := []rune(text)
	if len(runes) == 0 {
		return ""
	}
	return string(runes[0])
}

// dropLastRune 去掉末尾一个字符（长度不足时返回空）。
func dropLastRune(text string) string {
	runes := []rune(text)
	if len(runes) <= 1 {
		return ""
	}
	return string(runes[:len(runes)-1])
}
