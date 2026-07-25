// 订单物品表，对应 Java NyxBot 的 OrdersItems 实体
// 存储 Warframe.Market 上可交易的物品条目信息
package warframe

type OrdersItem struct {
	ID         uint   `gorm:"primaryKey"`         // 主键
	URL        string `gorm:"column:url"`         // 物品详情链接
	ItemName   string `gorm:"column:item_name"`   // 物品名称（游戏内名）
	Thumb      string `gorm:"column:thumb"`       // 缩略图链接
	SubType    string `gorm:"column:sub_type"`    // 子类型（如为遗物时的纪元）
	EnName     string `gorm:"column:en_name"`     // 英文名
	ZhName     string `gorm:"column:zh_name"`     // 中文名
	Icon       string `gorm:"column:icon"`        // 图标链接
	IconFormat string `gorm:"column:icon_format"` // 图标格式（如 png）
}

func (OrdersItem) TableName() string {
	return "orders_items"
}
