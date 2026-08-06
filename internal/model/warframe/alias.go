// 别名映射表，对应 Java NyxBot 的 Alias 实体（表 alias）
// 存储物品/任务的中文别名 -> 英文名映射，供市场查询与翻译使用
package warframe

// Alias 别名映射记录。cn 列带唯一约束（对齐 Java @Column(unique=true)）。
// JSON 字段对齐前端 Api.LocalData.Alias { id, en, cn }。
type Alias struct {
	ID uint   `gorm:"primaryKey" json:"id"` // 主键（自增）
	En string `gorm:"column:en" json:"en"`  // 英文名/原名
	Cn string `gorm:"column:cn" json:"cn"`  // 中文别名
}

func (Alias) TableName() string {
	return "alias"
}
