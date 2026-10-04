// 遗物查询：按遗物名或奖励物品名检索（对齐 Java RelicsService.findAllByRelicNameOrRewardsItemName）。
package warframe

import (
	"sort"
	"strings"

	"gorm.io/gorm"

	"nyxbot-go/internal/database"
	modelwarframe "nyxbot-go/internal/model/warframe"
)

// FindRelicsByNameOrReward 按遗物名或奖励物品名查询遗物
// （对齐 Java RelicsService.findAllByRelicNameOrRewardsItemName 的四级回退）：
// ① 遗物名等值（忽略大小写）→ ② 遗物名模糊 → ③ 奖励物品名模糊 →
// ④ 别名（中文→英文）替换后奖励物品名模糊。命中即按名称排序返回；全部未命中返回 nil。
func FindRelicsByNameOrReward(keyword string) []modelwarframe.Relics {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" || database.DB == nil {
		return nil
	}
	if relics := queryRelics(database.DB.Where("LOWER(name) = LOWER(?)", keyword)); len(relics) > 0 {
		return sortRelicsByName(relics)
	}
	if relics := queryRelics(database.DB.Where("LOWER(name) LIKE LOWER(?)", "%"+keyword+"%")); len(relics) > 0 {
		return sortRelicsByName(relics)
	}
	if relics := queryRelicsByRewardName(keyword); len(relics) > 0 {
		return sortRelicsByName(relics)
	}
	// 别名回退：把输入里的中文别名替换为英文（对齐 Java 遍历 alias 表做 replace）
	if replaced := replaceAliasToEnglish(keyword); replaced != keyword {
		if relics := queryRelicsByRewardName(replaced); len(relics) > 0 {
			return sortRelicsByName(relics)
		}
	}
	return nil
}

// queryRelics 按条件查询遗物并预载奖励列表。
func queryRelics(query *gorm.DB) []modelwarframe.Relics {
	var relics []modelwarframe.Relics
	if err := query.Preload("RelicRewards").Find(&relics).Error; err != nil {
		return nil
	}
	return relics
}

// queryRelicsByRewardName 按奖励物品名模糊匹配遗物（对齐 Java
// RelicsRepository.findByRelicRewardsRewardNameContainingIgnoreCase，JPQL 的 join 语义用子查询等价实现）。
func queryRelicsByRewardName(keyword string) []modelwarframe.Relics {
	subQuery := database.DB.Model(&modelwarframe.RelicRewards{}).
		Select("relics_id").
		Where("LOWER(reward_name) LIKE LOWER(?)", "%"+keyword+"%")
	return queryRelics(database.DB.Where("unique_name IN (?)", subQuery))
}

// replaceAliasToEnglish 用 alias 表把输入中的中文别名替换为英文
// （对齐 Java：key.contains(cn) 时 key.replace(cn, en)，逐条按表内顺序累积替换）。
func replaceAliasToEnglish(keyword string) string {
	var aliases []modelwarframe.Alias
	if err := database.DB.Order("id").Find(&aliases).Error; err != nil {
		return keyword
	}
	result := keyword
	for _, alias := range aliases {
		if alias.Cn == "" || alias.En == "" || !strings.Contains(result, alias.Cn) {
			continue
		}
		result = strings.ReplaceAll(result, alias.Cn, alias.En)
	}
	return result
}

// sortRelicsByName 按遗物名排序（对齐 Java RelicsService.sort：Comparator.comparing(Relics::getName)）。
func sortRelicsByName(relics []modelwarframe.Relics) []modelwarframe.Relics {
	sort.SliceStable(relics, func(i, j int) bool { return relics[i].Name < relics[j].Name })
	return relics
}
