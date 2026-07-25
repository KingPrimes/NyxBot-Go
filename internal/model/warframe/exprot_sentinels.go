// 守护/同伴导出数据表，对应 Java NyxBot 的 exprot.Sentinels 实体
// 存储从官方 API 导出的守护及同伴数据（含基础属性、Prime 状态等）
package warframe

type Sentinels struct {
	ID          uint    `gorm:"primaryKey"`         // 主键
	Name        string  `gorm:"column:name"`        // 名称（中文）
	EnName      string  `gorm:"column:en_name"`     // 英文名
	Description string  `gorm:"column:description"` // 描述
	Health      int     `gorm:"column:health"`      // 生命值
	Shield      int     `gorm:"column:shield"`      // 护盾值
	Armor       int     `gorm:"column:armor"`       // 护甲
	Energy      int     `gorm:"column:energy"`      // 能量上限
	Sprint      float64 `gorm:"column:sprint"`      // 冲刺速度
	Mastery     int     `gorm:"column:mastery"`     // 段位要求
	ImageName   string  `gorm:"column:image_name"`  // 图片文件名
	Tradable    string  `gorm:"column:tradable"`    // 可交易状态
	Tags        string  `gorm:"column:tags"`        // 标签
	Exclude     bool    `gorm:"column:exclude"`     // 是否排除
	IsPrime     bool    `gorm:"column:is_prime"`    // 是否为 Prime 版本
}

func (Sentinels) TableName() string {
	return "exprot_sentinels"
}
