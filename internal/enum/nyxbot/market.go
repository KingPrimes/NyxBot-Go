// NyxBot 市场相关枚举
package nyxbot

// MarketSortBy 市场排序方式
type MarketSortBy string

const (
	MarketSortPriceAsc   MarketSortBy = "PRICE_ASC"
	MarketSortPriceDesc  MarketSortBy = "PRICE_DESC"
	MarketSortDamageAsc  MarketSortBy = "DAMAGE_ASC"
	MarketSortDamageDesc MarketSortBy = "DAMAGE_DESC"
)

// Value 返回排序方式的 API 参数字符串。
func (m MarketSortBy) Value() string {
	mp := map[MarketSortBy]string{
		MarketSortPriceAsc: "price_asc", MarketSortPriceDesc: "price_desc",
		MarketSortDamageAsc: "damage_asc", MarketSortDamageDesc: "damage_desc",
	}
	return mp[m]
}

// MarketSearchPolicy 市场搜索策略
type MarketSearchPolicy string

const (
	MarketSearchAny     MarketSearchPolicy = "ANY"
	MarketSearchDirect  MarketSearchPolicy = "DIRECT"
	MarketSearchAuction MarketSearchPolicy = "AUCTION"
)

// Value 返回搜索策略的 API 参数字符串。
func (m MarketSearchPolicy) Value() string {
	mp := map[MarketSearchPolicy]string{
		MarketSearchAny: "any", MarketSearchDirect: "direct", MarketSearchAuction: "auction",
	}
	return mp[m]
}

// MarketSearchElementEnum 玄骸/姐妹武器元素类型
type MarketSearchElement string

const (
	MarketElemCold       MarketSearchElement = "COLD"
	MarketElemRadiation  MarketSearchElement = "RADIATION"
	MarketElemHeat       MarketSearchElement = "HEAT"
	MarketElemMagnetic   MarketSearchElement = "MAGNETIC"
	MarketElemToxin      MarketSearchElement = "TOXIN"
	MarketElemElectricity MarketSearchElement = "ELECTRICITY"
	MarketElemImpact     MarketSearchElement = "IMPACT"
	MarketElemAny        MarketSearchElement = "ANY"
)

// Element 返回元素类型的小写 API 参数。
func (m MarketSearchElement) Element() string {
	mp := map[MarketSearchElement]string{
		MarketElemCold: "cold", MarketElemRadiation: "radiation", MarketElemHeat: "heat",
		MarketElemMagnetic: "magnetic", MarketElemToxin: "toxin",
		MarketElemElectricity: "electricity", MarketElemImpact: "impact", MarketElemAny: "",
	}
	return mp[m]
}

// MarketPlatformEnum 市场平台
type MarketPlatform string

const (
	MarketPlatformPC     MarketPlatform = "PC"
	MarketPlatformPS4    MarketPlatform = "PS4"
	MarketPlatformXBOX   MarketPlatform = "XBOX"
	MarketPlatformSwitch MarketPlatform = "SWITCH"
	MarketPlatformMobile MarketPlatform = "MOBILE"
)

// Platform 返回平台的小写 API 参数。
func (m MarketPlatform) Platform() string {
	mp := map[MarketPlatform]string{
		MarketPlatformPC: "pc", MarketPlatformPS4: "ps4",
		MarketPlatformXBOX: "xbox", MarketPlatformSwitch: "switch",
		MarketPlatformMobile: "mobile",
	}
	return mp[m]
}
