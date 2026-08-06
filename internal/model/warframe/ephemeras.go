// 幻纹表，对应 Java NyxBot 的 Ephemeras 实体（表 ephemeras）
// 存储 Warframe 幻纹（Phantom）条目，来自官方导出/市场数据
package warframe

// Ephemera 幻纹条目。id 为字符串主键（对齐 Java String 主键）。
// JSON 字段对齐前端 Api.LocalData.Phantom { id, slug, gameRef, animation, element, name, icon, thumb }。
type Ephemera struct {
	ID        string `gorm:"primaryKey" json:"id"`              // 主键（唯一标识）
	Slug      string `gorm:"column:slug" json:"slug"`           // 市场 slug
	GameRef   string `gorm:"column:game_ref" json:"gameRef"`    // 游戏内引用
	Animation string `gorm:"column:animation" json:"animation"` // 动画标识
	Element   string `gorm:"column:element" json:"element"`     // 元素属性
	Name      string `gorm:"column:name" json:"name"`           // 幻纹名称（list 过滤字段）
	Icon      string `gorm:"column:icon" json:"icon"`           // 图标链接
	Thumb     string `gorm:"column:thumb" json:"thumb"`         // 缩略图链接
}

func (Ephemera) TableName() string {
	return "ephemeras"
}
