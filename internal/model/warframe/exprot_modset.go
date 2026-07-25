// Mod 套装导出数据表，对应 Java NyxBot 的 exprot.ModSet 实体
// 存储 Mod 套装信息（套装名称、包含的 Mod 列表）
package warframe

type ModSet struct {
	ID          uint   `gorm:"primaryKey"`          // 主键
	Name        string `gorm:"column:name"`         // 套装名称（中文）
	EnName      string `gorm:"column:en_name"`      // 英文名
	ImageName   string `gorm:"column:image_name"`   // 图片文件名
	Tags        string `gorm:"column:tags"`         // 标签
	Description string `gorm:"column:description"`  // 描述
	Exclude     bool   `gorm:"column:exclude"`      // 是否排除
	ModSetMods  string `gorm:"column:mod_set_mods"` // 套装包含的 Mod 列表（JSON）
}

func (ModSet) TableName() string {
	return "exprot_mod_set"
}
