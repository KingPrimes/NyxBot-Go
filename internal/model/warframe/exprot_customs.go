// 自定义分类导出数据表，对应 Java NyxBot 的 exprot.Customs 实体
// 存储扩展的自定义分类数据（如外观、装饰等额外分类物品）
package warframe

type Customs struct {
	ID           uint   `gorm:"primaryKey"`           // 主键
	Name         string `gorm:"column:name"`          // 物品名称（中文）
	ImageName    string `gorm:"column:image_name"`    // 图片文件名
	Description  string `gorm:"column:description"`   // 描述
	EnName       string `gorm:"column:en_name"`       // 英文名
	Tags         string `gorm:"column:tags"`          // 标签
	Tradable     string `gorm:"column:tradable"`      // 可交易状态
	Exclude      bool   `gorm:"column:exclude"`       // 是否排除
	Category     string `gorm:"column:category"`      // 分类
	ItemCount    int    `gorm:"column:item_count"`    // 物品数量
	Xp           int    `gorm:"column:xp"`            // 经验值
	CategoryName string `gorm:"column:category_name"` // 分类名称
}

func (Customs) TableName() string {
	return "exprot_customs"
}
