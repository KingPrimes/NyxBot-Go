// 别名映射表，对应 Java NyxBot 的 Aliases 实体
// 存储物品/任务的中文别名 -> 名称映射关系
package warframe

type Alias struct {
	ID          uint   `gorm:"primaryKey"`          // 主键
	Alias       string `gorm:"column:alias"`        // 别名（输入用）
	ChineseName string `gorm:"column:chinese_name"` // 对应的中文名称
	Message     string `gorm:"column:message"`      // 关联的回复消息
}

func (Alias) TableName() string {
	return "aliases"
}
