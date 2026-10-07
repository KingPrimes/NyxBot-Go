package rivenanalyse

import (
	"strings"

	modelwarframe "nyxbot-go/internal/model/warframe"
)

// 武器产品类别（ORDINAL 顺序，对齐 Java Weapons.ProductCategory 枚举与数据库列）。
const (
	catPistols = iota
	catLongGuns
	catMelee
	catSpaceGuns
	catSpaceMelee
	catSpecialItems
	catCrewShipWeapons
	catSentinelWeapons
	catShotguns
)

// weaponCategoryNames 类别显示名（对齐 Java 枚举构造参数）。
var weaponCategoryNames = [...]string{
	"次要武器", "主要武器", "近战武器", "Archwing武器", "Archwing近战武器",
	"显赫武器", "星舰武器", "守护武器", "霰弹枪",
}

// effectiveCategory 计算有效武器类别（对齐 Java getEffectiveCategory）：
// 守护武器按描述细分（霰弹枪/近战/其余归步枪）；其余描述含「霰弹枪/散弹枪」修正为霰弹枪。
func effectiveCategory(weapon *modelwarframe.Weapons) int {
	cat := weapon.ProductCategory
	if cat < 0 || cat >= len(weaponCategoryNames) {
		return catLongGuns
	}
	d := weapon.Description
	if cat == catSentinelWeapons {
		if strings.Contains(d, "霰弹枪") {
			return catShotguns
		}
		if strings.Contains(d, "近战") || strings.Contains(weapon.Name, "分离") {
			return catMelee
		}
		return catLongGuns
	}
	if strings.Contains(d, "霰弹枪") || strings.Contains(d, "散弹枪") {
		return catShotguns
	}
	return cat
}

// charReplacement OCR 错字修正（有序，对齐 Java CHAR_REPLACEMENTS）。
var charReplacement = []struct{ from, to string }{
	{"淞", "凇"},
}

// charAnalyse 词条名规范化映射（**有序**——按序首个命中即采用，对齐 Java LinkedHashMap 的 CHAR_ANALYSE）。
var charAnalyse = []struct{ keyword, target string }{
	{"射速", "射速/攻击速度"},
	{"攻击速度", "射速/攻击速度"},
	{"武器后坐力", "后坐力"},
	{"Infested", "对Infested伤害"},
	{"lnfested", "对Infested伤害"},
	{"Corpus", "对Corpus伤害"},
	{"Grinner", "对Grineer伤害"},
	{"Grineer", "对Grineer伤害"},
	{"滑行", "滑行攻击暴击几率"},
	{"暴击几率", "暴击几率"},
	{"暴击伤害", "暴击伤害"},
	{"秒连击持续时间", "连击持续时间"},
	{"连击数", "几率不获得连击数"},
	{"冰冻", "冰冻伤害"},
	{"毒素", "毒素伤害"},
	{"电击", "电击伤害"},
	{"火焰", "火焰伤害"},
	{"冲击", "冲击伤害"},
	{"切割", "切割伤害"},
	{"穿刺", "穿刺伤害"},
	{"投射物", "投射物飞行速度"},
	{"后坐力", "后坐力"},
	{"伤害", "伤害/近战伤害"},
}

// findWeapons 按名称模糊查询武器（对齐 Java findByFuzzyName：错字修正后 LIKE %名%）。
func (c *Calculator) findWeapons(name string) []modelwarframe.Weapons {
	fixed := name
	for _, r := range charReplacement {
		fixed = strings.ReplaceAll(fixed, r.from, r.to)
	}
	var weapons []modelwarframe.Weapons
	if err := c.db.Where("name LIKE ?", "%"+fixed+"%").Find(&weapons).Error; err != nil {
		return nil
	}
	return weapons
}

// findTrendByAnalyseName 按规范化名查询 trend 条目（对齐 Java findRivenTrendByAnalyseName：
// 逐条关键词映射尝试，命中关键词后按目标名查询、查不到继续下一关键词；全部失败按原名查询）。
func (c *Calculator) findTrendByAnalyseName(analyseName string) *modelwarframe.RivenAnalyseTrend {
	for _, m := range charAnalyse {
		if strings.Contains(analyseName, m.keyword) {
			if trend := c.trendByName(m.target); trend != nil {
				return trend
			}
		}
	}
	return c.trendByName(analyseName)
}

// trendByName 按名称精确查询 trend 条目，未命中返回 nil。
func (c *Calculator) trendByName(name string) *modelwarframe.RivenAnalyseTrend {
	var trend modelwarframe.RivenAnalyseTrend
	if err := c.db.Where("name = ?", name).First(&trend).Error; err != nil {
		return nil
	}
	return &trend
}
