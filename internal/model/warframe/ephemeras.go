// 幻纹数据表，对应 Java NyxBot 的 Ephemeras 实体
// 存储 Warframe 幻纹（翅膀特效）的名称、图片链接等信息
package warframe

type Ephemera struct {
	ID       uint   `gorm:"primaryKey"`          // 主键
	URL      string `gorm:"column:url"`           // 物品详情链接
	ItemName string `gorm:"column:item_name"`     // 幻纹名称
	Thumb    string `gorm:"column:thumb"`         // 缩略图链接
}

func (Ephemera) TableName() string {
	return "ephemeras"
}
