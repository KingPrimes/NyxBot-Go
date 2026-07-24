// 遗物导出数据表，对应 Java NyxBot 的 exprot.Relics + RelicRewards 实体
// Relics - 存储遗物基本信息（纪元/等级/是否入库）
// RelicRewards - 存储遗物内物品掉落（名称/稀有度/概率）
package warframe

type Relics struct {
	ID          uint   `gorm:"primaryKey"`                    // 主键
	Name        string `gorm:"column:name"`                   // 遗物名称（中文）
	EnName      string `gorm:"column:en_name"`                // 英文名
	ImageName   string `gorm:"column:image_name"`             // 图片文件名
	Tradable    string `gorm:"column:tradable"`               // 可交易状态
	Tags        string `gorm:"column:tags"`                   // 标签
	Description string `gorm:"column:description"`            // 描述
	Exclude     bool   `gorm:"column:exclude"`                // 是否排除
	RelicTier   string `gorm:"column:relic_tier"`             // 遗物等级（Lith/Meso/Neo/Axi）
	IsVaulted   bool   `gorm:"column:is_vaulted"`             // 是否入库
	RelicType   string `gorm:"column:relic_type"`             // 遗物类型（完好/光辉等）
	Rewards     []RelicRewards `gorm:"foreignKey:RelicID"`    // 遗物内含物品（一对多）
}

func (Relics) TableName() string {
	return "exprot_relics"
}

type RelicRewards struct {
	ID       uint    `gorm:"primaryKey"`       // 主键
	RelicID  uint    `gorm:"column:relic_id"`  // 所属遗物外键
	ItemName string  `gorm:"column:item_name"` // 物品名称
	Rarity   string  `gorm:"column:rarity"`    // 稀有度（常见/罕见/稀有/传说）
	Chance   float64 `gorm:"column:chance"`    // 掉落概率
}

func (RelicRewards) TableName() string {
	return "exprot_relic_rewards"
}
