// 市场查询结果 DTO，对应 Java NyxBot 的 MarketResult 实体
// 非数据库表，用于 Warframe.Market 搜索结果的 JSON 序列化返回
package warframe

type MarketResult struct {
	URL        string   `json:"url"`         // 物品详情链接
	ItemName   string   `json:"item_name"`   // 物品名称
	Thumb      string   `json:"thumb"`       // 缩略图链接
	SubType    string   `json:"sub_type"`    // 子类型
	EnName     string   `json:"en_name"`     // 英文名
	ZhName     string   `json:"zh_name"`     // 中文名
	Icon       string   `json:"icon"`        // 图标链接
	IconFormat string   `json:"icon_format"` // 图标格式
	Group      string   `json:"group"`       // 分组标识
	Tags       []string `json:"tags"`        // 标签列表
}
