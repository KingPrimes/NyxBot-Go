// 提示信息模型，对应 Java NyxBot 的 Hint 实体
// 存储 Bot 指令帮助/提示文本，仅包含一条长文本
package system

type Hint struct {
	ID   uint   `gorm:"primaryKey"`          // 主键
	Hint string `gorm:"column:hint;type:longtext"` // 提示文本内容
}

func (Hint) TableName() string {
	return "hint"
}
