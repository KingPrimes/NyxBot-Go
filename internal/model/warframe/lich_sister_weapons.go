// 玄骸/姐妹武器数据表，对应 Java NyxBot 的 LichSisterWeapons 实体
// 存储 Kuva/Tenet 武器的详细属性（伤害、暴击、触发等）
package warframe

type LichSisterWeapon struct {
	ID            uint    `gorm:"primaryKey"`             // 主键
	WeaponName    string  `gorm:"column:weapon_name"`     // 武器名称
	WeaponType    string  `gorm:"column:weapon_type"`     // 武器类型（如步枪/手枪/近战）
	WeaponSubType string  `gorm:"column:weapon_sub_type"` // 武器子类型
	Damage        int     `gorm:"column:damage"`          // 基础伤害
	CritChance    float64 `gorm:"column:crit_chance"`     // 暴击几率
	CritMulti     float64 `gorm:"column:crit_multi"`      // 暴击倍率
	StatusChance  float64 `gorm:"column:status_chance"`   // 触发几率
	AttackSpeed   float64 `gorm:"column:attack_speed"`    // 攻击速度
	Mastery       int     `gorm:"column:mastery"`         // 段位要求
	Magazine      int     `gorm:"column:magazine"`        // 弹匣容量
	Reload        float64 `gorm:"column:reload"`          // 换弹时间（秒）
	Trigger       string  `gorm:"column:trigger"`         // 扳机类型（自动/点射/半自动等）
	Disposition   float64 `gorm:"column:disposition"`     // 紫卡倾向
	Polarities    string  `gorm:"column:polarities"`      // 极性槽
	ImageName     string  `gorm:"column:image_name"`      // 图片文件名
	Impact        float64 `gorm:"column:impact"`          // 冲击伤害
	Puncture      float64 `gorm:"column:puncture"`        // 穿刺伤害
	Slash         float64 `gorm:"column:slash"`           // 切割伤害
	DamageType    string  `gorm:"column:damage_type"`     // 主要伤害类型
	Total         float64 `gorm:"column:total"`           // 总伤害
}

func (LichSisterWeapon) TableName() string {
	return "lich_sister_weapons"
}
