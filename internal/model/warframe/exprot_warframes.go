// 战甲导出数据表，对应 Java NyxBot 的 exprot.Warframes 实体（表 warframes）
// 存储从官方 API 导出的全量战甲数据（含基础属性、技能、Prime 状态等）
package warframe

// Warframes 战甲条目。uniqueName 为字符串主键。
// SprintSpeed 对齐 Java Integer 类型（sprintSpeed）。
// JSON 字段对齐前端 Api.LocalData.Warframes。
type Warframes struct {
	UniqueName      string             `gorm:"primaryKey" json:"uniqueName"`                    // 主键（唯一标识）
	Name            string             `gorm:"column:name" json:"name"`                         // 战甲名称
	ParentName      string             `gorm:"column:parent_name" json:"parentName"`            // 父级名称（Prime 关联基础版）
	Description     string             `gorm:"column:description;type:text" json:"description"` // 战甲描述
	Health          int                `gorm:"column:health" json:"health"`                     // 生命值
	Shield          int                `gorm:"column:shield" json:"shield"`                     // 护盾值
	Armor           int                `gorm:"column:armor" json:"armor"`                       // 护甲
	Stamina         int                `gorm:"column:stamina" json:"stamina"`                   // 耐力
	Power           int                `gorm:"column:power" json:"power"`                       // 能量上限
	CodexSecret     bool               `gorm:"column:codex_secret" json:"codexSecret"`          // 是否法典机密
	MasteryReq      int                `gorm:"column:mastery_req" json:"masteryReq"`            // 段位要求
	SprintSpeed     int                `gorm:"column:sprint_speed" json:"sprintSpeed"`          // 冲刺速度
	Abilities       []WarframesAbility `gorm:"foreignKey:WarframeUniqueName" json:"abilities"`  // 技能列表（一对多）
	ProductCategory string             `gorm:"column:product_category" json:"productCategory"`  // 产品分类
}

func (Warframes) TableName() string {
	return "warframes"
}

// WarframesAbility 战甲技能条目。abilityUniqueName 为字符串主键。
// JSON 字段对齐前端 abilities 子表（abilityName / description）。
type WarframesAbility struct {
	AbilityUniqueName  string `gorm:"primaryKey" json:"abilityUniqueName"`             // 主键（唯一标识）
	AbilityName        string `gorm:"column:ability_name" json:"abilityName"`          // 技能名称
	Description        string `gorm:"column:description;type:text" json:"description"` // 技能描述
	WarframeUniqueName string `gorm:"column:warframe_unique_name" json:"-"`            // 所属战甲外键（不输出）
}

func (WarframesAbility) TableName() string {
	return "abilities"
}
