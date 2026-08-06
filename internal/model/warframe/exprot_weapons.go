// 武器导出数据表，对应 Java NyxBot 的 exprot.Weapons 实体（表 weapons）
// 存储从官方 API 导出的全量武器数据（含伤害、暴击、触发等完整属性）
package warframe

import (
	"encoding/json"
	"fmt"
)

// Weapons 武器条目。uniqueName 为字符串主键。
// ProductCategory 数据库列存 ProductCategory 枚举 ORDINAL 序数，JSON 输出枚举名（对齐 Jackson）。
// DamagePerShot 为 20 个伤害值的 JSON 数组字符串（对齐 Java @ElementCollection）。
// JSON 字段对齐前端 Api.LocalData.Weapon。
type Weapons struct {
	UniqueName         string  `gorm:"primaryKey" json:"uniqueName"`                          // 主键（唯一标识）
	Name               string  `gorm:"column:name" json:"name"`                               // 武器名称
	CodexSecret        bool    `gorm:"column:codex_secret" json:"codexSecret"`                // 是否法典机密
	DamagePerShot      string  `gorm:"column:damage_per_shot;type:text" json:"damagePerShot"` // 分伤害数组（JSON）
	TotalDamage        int     `gorm:"column:total_damage" json:"totalDamage"`                // 总伤害
	Description        string  `gorm:"column:description;type:text" json:"description"`       // 武器描述
	EnglishName        string  `gorm:"column:english_name" json:"englishName"`                // 英文名
	CriticalChance     float64 `gorm:"column:critical_chance" json:"criticalChance"`          // 暴击几率
	CriticalMultiplier float64 `gorm:"column:critical_multiplier" json:"criticalMultiplier"`  // 暴击倍率
	ProcChance         float64 `gorm:"column:proc_chance" json:"procChance"`                  // 触发几率
	FireRate           float64 `gorm:"column:fire_rate" json:"fireRate"`                      // 射速
	MasteryReq         int     `gorm:"column:mastery_req" json:"masteryReq"`                  // 段位要求
	ProductCategory    int     `gorm:"column:product_category" json:"-"`                      // 产品分类（ORDINAL，JSON 由 MarshalJSON 输出枚举名）
	Slot               int     `gorm:"column:slot" json:"slot"`                               // 槽位
	Accuracy           float64 `gorm:"column:accuracy" json:"accuracy"`                       // 精准度
	OmegaAttenuation   float64 `gorm:"column:omega_attenuation" json:"omegaAttenuation"`      // Omega 衰减
	MaxLevelCap        int     `gorm:"column:max_level_cap" json:"maxLevelCap"`               // 最高等级上限（默认 30）
	Noise              string  `gorm:"column:noise" json:"noise"`                             // 噪音等级
	Trigger            string  `gorm:"column:trigger" json:"trigger"`                         // 扳机类型
	MagazineSize       int     `gorm:"column:magazine_size" json:"magazineSize"`              // 弹匣容量
	ReloadTime         float64 `gorm:"column:reload_time" json:"reloadTime"`                  // 换弹时间
	Sentinel           bool    `gorm:"column:sentinel" json:"sentinel"`                       // 是否为守护武器
	Multishot          int     `gorm:"column:multishot" json:"multishot"`                     // 多重射击
}

func (Weapons) TableName() string {
	return "weapons"
}

// MarshalJSON 输出 productCategory 为枚举名字符串（对齐 Jackson 枚举序列化）。
func (record Weapons) MarshalJSON() ([]byte, error) {
	type plain Weapons
	aliased := plain(record)
	return json.Marshal(struct {
		plain
		ProductCategory string `json:"productCategory"`
	}{plain: aliased, ProductCategory: productCategoryOrdinalToName(record.ProductCategory)})
}

// UnmarshalJSON 接受 productCategory 为枚举名或序数，统一转 ORDINAL 入库。
func (record *Weapons) UnmarshalJSON(data []byte) error {
	type plain Weapons
	aliased := (*plain)(record)
	// 先按原结构反序列化（productCategory 为 int 时可直接解析）
	var base plain
	if err := json.Unmarshal(data, &base); err != nil {
		return err
	}
	*aliased = base

	var raw struct {
		ProductCategory json.RawMessage `json:"productCategory"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	var name string
	if err := json.Unmarshal(raw.ProductCategory, &name); err == nil {
		record.ProductCategory = productCategoryNameToOrdinal(name)
		return nil
	}
	var ordinal int
	if err := json.Unmarshal(raw.ProductCategory, &ordinal); err == nil {
		record.ProductCategory = ordinal
		return nil
	}
	return fmt.Errorf("invalid weapon product category %q", string(raw.ProductCategory))
}
