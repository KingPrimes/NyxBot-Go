// 武器导出数据表，对应 Java NyxBot 的 exprot.Weapons 实体
// 存储从官方 API 导出的全量武器数据（含伤害、暴击、触发等完整属性）
package warframe

type Weapons struct {
	ID                uint    `gorm:"primaryKey"`                // 主键
	Name              string  `gorm:"column:name"`               // 武器名称（中文）
	WeaponType        string  `gorm:"column:weapon_type"`        // 武器类型分类
	Trigger           string  `gorm:"column:trigger"`            // 扳机类型
	Damage            int     `gorm:"column:damage"`             // 基础伤害
	CritChance        float64 `gorm:"column:crit_chance"`        // 暴击几率
	CritMulti         float64 `gorm:"column:crit_multi"`         // 暴击倍率
	StatusChance      float64 `gorm:"column:status_chance"`      // 触发几率
	AttackSpeed       float64 `gorm:"column:attack_speed"`       // 攻击速度
	Mastery           int     `gorm:"column:mastery"`            // 段位要求
	Magazine          int     `gorm:"column:magazine"`           // 弹匣容量
	Reload            float64 `gorm:"column:reload"`             // 换弹时间（秒）
	Disposition       float64 `gorm:"column:disposition"`        // 紫卡倾向
	Polarities        string  `gorm:"column:polarities"`         // 极性槽
	ImageName         string  `gorm:"column:image_name"`         // 图片文件名
	Impact            float64 `gorm:"column:impact"`             // 冲击伤害
	Puncture          float64 `gorm:"column:puncture"`           // 穿刺伤害
	Slash             float64 `gorm:"column:slash"`              // 切割伤害
	DamageType        string  `gorm:"column:damage_type"`        // 主要伤害类型
	Total             float64 `gorm:"column:total"`              // 总伤害
	Tags              string  `gorm:"column:tags"`               // 标签
	EnName            string  `gorm:"column:en_name"`            // 英文名
	Tradable          string  `gorm:"column:tradable"`           // 可交易状态
	Accuracy          float64 `gorm:"column:accuracy"`           // 精准度
	FlightSpeed       float64 `gorm:"column:flight_speed"`       // 飞行速度（投射物）
	ChargeAttack      float64 `gorm:"column:charge_attack"`      // 蓄力攻击
	ChargeTime        float64 `gorm:"column:charge_time"`        // 蓄力时间
	ChargeDamage      float64 `gorm:"column:charge_damage"`      // 蓄力伤害
	ChargePenetration float64 `gorm:"column:charge_penetration"` // 蓄力穿透
	ChargeMultiShot   float64 `gorm:"column:charge_multi_shot"`  // 蓄力多重射击
	SniperCombo       float64 `gorm:"column:sniper_combo"`       // 狙击连击
	ComboDuration     float64 `gorm:"column:combo_duration"`     // 连击持续时间
	Ammo              int     `gorm:"column:ammo"`               // 备弹量
	BlockAngles       float64 `gorm:"column:block_angles"`       // 格挡角度
	FollowThrough     float64 `gorm:"column:follow_through"`     // 穿透衰减
	Range             float64 `gorm:"column:range"`              // 近战范围
	Noise             string  `gorm:"column:noise"`              // 噪音等级
	Stamina           float64 `gorm:"column:stamina"`            // 消耗 stamina
	Description       string  `gorm:"column:description"`        // 武器描述
	ItemType          string  `gorm:"column:item_type"`          // 物品类型
	Category          string  `gorm:"column:category"`           // 分类
	Exclude           bool    `gorm:"column:exclude"`            // 是否排除（不参与查询）
	SubType           string  `gorm:"column:sub_type"`           // 子类型
	IsMelee           bool    `gorm:"column:is_melee"`           // 是否为近战
	ContentTag        string  `gorm:"column:content_tag"`        // 内容标签
}

func (Weapons) TableName() string {
	return "exprot_weapons"
}
