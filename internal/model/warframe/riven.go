// Riven 模块相关数据，对应 Java NyxBot 的 riven 包下四个实体：
//   RivenTion - 紫卡倾向（武器类型 + 兼容性）
//   RivenTionAlias - 紫卡武器别名
//   RivenItems - 紫卡可交易物品
//   RivenAnalyseTrend - 紫卡价格分析趋势
package warframe

type RivenTion struct {
	ID            uint            `gorm:"primaryKey"`            // 主键
	ItemType      string          `gorm:"column:item_type"`      // 物品类型标识
	Compatibility string          `gorm:"column:compatibility"`  // 兼容性描述
	ImageName     string          `gorm:"column:image_name"`     // 图片文件名
	Icon          string          `gorm:"column:icon"`           // 图标链接
	IconFormat    string          `gorm:"column:icon_format"`    // 图标格式
	Aliases       []RivenTionAlias `gorm:"foreignKey:RivenTionID"` // 别名列表（一对多）
}

func (RivenTion) TableName() string {
	return "riven_tion"
}

type RivenTionAlias struct {
	ID          uint   `gorm:"primaryKey"`           // 主键
	Alias       string `gorm:"column:alias"`          // 别名名称
	RivenTionID uint   `gorm:"column:riven_tion_id"` // 所属 RivenTion 外键
}

func (RivenTionAlias) TableName() string {
	return "riven_tion_alias"
}

type RivenItem struct {
	ID         uint   `gorm:"primaryKey"`          // 主键
	URL        string `gorm:"column:url"`           // 物品详情链接
	ItemName   string `gorm:"column:item_name"`     // 物品名称
	Thumb      string `gorm:"column:thumb"`         // 缩略图链接
	SubType    string `gorm:"column:sub_type"`      // 子类型
	EnName     string `gorm:"column:en_name"`       // 英文名
	ZhName     string `gorm:"column:zh_name"`       // 中文名
	Icon       string `gorm:"column:icon"`          // 图标链接
	IconFormat string `gorm:"column:icon_format"`   // 图标格式
	Group      string `gorm:"column:group"`         // 分组标识
}

func (RivenItem) TableName() string {
	return "riven_items"
}

type RivenAnalyseTrend struct {
	ID         uint    `gorm:"primaryKey"`          // 主键
	ItemName   string  `gorm:"column:item_name"`    // 物品名称
	Icon       string  `gorm:"column:icon"`          // 图标链接
	IconFormat string  `gorm:"column:icon_format"`  // 图标格式
	EnName     string  `gorm:"column:en_name"`      // 英文名
	Avg        float64 `gorm:"column:avg"`           // 平均价
	Std        float64 `gorm:"column:std"`           // 标准差
	Min        float64 `gorm:"column:min"`           // 最低价
	Max        float64 `gorm:"column:max"`           // 最高价
	Median     float64 `gorm:"column:median"`        // 中位数
	SampleSize int     `gorm:"column:sample_size"`  // 样本量
}

func (RivenAnalyseTrend) TableName() string {
	return "riven_analyse_trend"
}
