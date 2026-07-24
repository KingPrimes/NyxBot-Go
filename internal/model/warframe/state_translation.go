// 状态翻译表，对应 Java NyxBot 的 StateTranslation 实体
// 存储 WorldState 任务状态的中文翻译映射（如 "Fissure" -> "裂缝"）
package warframe

type StateTranslation struct {
	ID          uint   `gorm:"primaryKey"`          // 主键
	State       string `gorm:"column:state"`         // 原始状态名（英文）
	ChineseName string `gorm:"column:chinese_name"`  // 中文翻译
	Description string `gorm:"column:description"`   // 状态描述说明
}

func (StateTranslation) TableName() string {
	return "state_translation"
}
