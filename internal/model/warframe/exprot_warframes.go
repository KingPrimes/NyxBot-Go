// 战甲导出数据表，对应 Java NyxBot 的 exprot.Warframes 实体
// 存储从官方 API 导出的全量战甲数据（含基础属性、极性、Prime 状态等）
package warframe

type Warframes struct {
	ID             uint    `gorm:"primaryKey"`             // 主键
	Name           string  `gorm:"column:name"`            // 战甲名称（中文）
	Description    string  `gorm:"column:description"`     // 战甲描述
	Health         int     `gorm:"column:health"`          // 生命值
	Shield         int     `gorm:"column:shield"`          // 护盾值
	Armor          int     `gorm:"column:armor"`           // 护甲
	Energy         int     `gorm:"column:energy"`          // 能量上限
	Sprint         float64 `gorm:"column:sprint"`          // 冲刺速度
	Mastery        int     `gorm:"column:mastery"`         // 段位要求
	ImageName      string  `gorm:"column:image_name"`      // 图片文件名
	EnName         string  `gorm:"column:en_name"`         // 英文名
	Tradable       string  `gorm:"column:tradable"`        // 可交易状态
	Tags           string  `gorm:"column:tags"`            // 标签
	Aura           string  `gorm:"column:aura"`            // 光环极性
	Polarity       string  `gorm:"column:polarity"`        // 整体极性
	Introduced     string  `gorm:"column:introduced"`      // 引入版本
	Conclave       bool    `gorm:"column:conclave"`        // 是否在武形秘仪可用
	Vaulted        bool    `gorm:"column:vaulted"`         // 是否入库（Prime）
	WikiaThumbnail string  `gorm:"column:wikia_thumbnail"` // Wikia 缩略图
	WikiaURL       string  `gorm:"column:wikia_url"`       // Wikia 页面链接
	Exclude        bool    `gorm:"column:exclude"`         // 是否排除
	IsPrime        bool    `gorm:"column:is_prime"`        // 是否为 Prime 版本
}

func (Warframes) TableName() string {
	return "exprot_warframes"
}
