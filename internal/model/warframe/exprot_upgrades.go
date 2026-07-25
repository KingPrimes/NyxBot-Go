// Mod/升级卡片导出数据表，对应 Java NyxBot 的 exprot.Upgrades 实体
// 存储从官方 API 导出的全量 Mod 数据（含极性、稀有度、类型分类等）
package warframe

type Upgrades struct {
	ID           uint   `gorm:"primaryKey"`          // 主键
	Name         string `gorm:"column:name"`         // Mod 名称（中文）
	EnName       string `gorm:"column:en_name"`      // Mod 英文名
	ImageName    string `gorm:"column:image_name"`   // 图片文件名
	Tradable     string `gorm:"column:tradable"`     // 可交易状态
	Type         string `gorm:"column:type"`         // Mod 大类（Warframe/武器等）
	Tags         string `gorm:"column:tags"`         // 标签
	Category     string `gorm:"column:category"`     // 分类（如步枪/手枪/近战）
	Description  string `gorm:"column:description"`  // Mod 效果描述
	Exclude      bool   `gorm:"column:exclude"`      // 是否排除
	Polarity     string `gorm:"column:polarity"`     // 极性
	Rarity       string `gorm:"column:rarity"`       // 稀有度（常见/罕见/稀有/传说）
	FusionLimit  int    `gorm:"column:fusion_limit"` // 最高等级
	UpgradeType  string `gorm:"column:upgrade_type"` // 升级类型
	Compatibilty string `gorm:"column:compatibilty"` // 兼容性（注意拼写与 Java 一致）
	IsExilus     bool   `gorm:"column:is_exilus"`    // 是否 Exilus 槽
	IsAugment    bool   `gorm:"column:is_augment"`   // 是否集团 Mod
	IsPrime      bool   `gorm:"column:is_prime"`     // 是否 Prime 版本
	IsLegendary  bool   `gorm:"column:is_legendary"` // 是否传说级
	IsParazon    bool   `gorm:"column:is_parazon"`   // 是否 Parazon Mod
}

func (Upgrades) TableName() string {
	return "exprot_upgrades"
}
