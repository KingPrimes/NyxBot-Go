// 订单物品表，对应 Java NyxBot 的 OrdersItems 实体（表 orders_items）
// 存储 Warframe.Market 上可交易的物品条目信息，供市场查询指令与本地数据管理使用
package warframe

// OrdersItem 市场物品条目。id 为字符串主键。
// reqMasteryRank / tradingTax 在 Java 中为 @Transient（不入库，list 恒为 null），故不建列。
// JSON 字段对齐前端 Api.LocalData.Market。
type OrdersItem struct {
	ID            string `gorm:"primaryKey" json:"id"`                        // 主键（唯一标识）
	Slug          string `gorm:"column:slug" json:"slug"`                     // 市场 slug
	GameRef       string `gorm:"column:game_ref" json:"gameRef"`              // 游戏内引用
	BulkTradable  bool   `gorm:"column:bulk_tradable" json:"bulkTradable"`    // 是否可批量交易
	MaxRank       int    `gorm:"column:max_rank" json:"maxRank"`              // 最大等级
	Ducats        int    `gorm:"column:ducats" json:"ducats"`                 // 杜卡德币价格
	Name          string `gorm:"column:name" json:"name"`                     // 物品名称（list 过滤字段）
	Icon          string `gorm:"column:icon" json:"icon"`                     // 图标链接
	Thumb         string `gorm:"column:thumb" json:"thumb"`                   // 缩略图链接
	Vaulted       bool   `gorm:"column:vaulted" json:"vaulted"`               // 是否入库
	MaxAmberStars int    `gorm:"column:max_amber_stars" json:"maxAmberStars"` // 最大琥珀星数
	MaxCyanStars  int    `gorm:"column:max_cyan_stars" json:"maxCyanStars"`   // 最大靛蓝星数
	BaseEndo      int    `gorm:"column:base_endo" json:"baseEndo"`            // 基础 Endo
}

func (OrdersItem) TableName() string {
	return "orders_items"
}
