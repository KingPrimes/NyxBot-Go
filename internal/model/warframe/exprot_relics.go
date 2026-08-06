// 遗物导出数据表，对应 Java NyxBot 的 exprot.Relics + RelicRewards 实体
// Relics（表 relics） - 存储遗物基本信息
// RelicRewards（表 relic_rewards） - 存储遗物内物品掉落（外键 relics_id）
package warframe

import (
	"encoding/json"
	"fmt"
)

// Relics 遗物条目。uniqueName 为字符串主键。
// JSON 字段对齐前端 Api.LocalData.Relic。
type Relics struct {
	UniqueName   string         `gorm:"primaryKey" json:"uniqueName"`            // 主键（唯一标识）
	Name         string         `gorm:"column:name" json:"name"`                 // 遗物名称（list 过滤字段，等值匹配）
	CodexSecret  bool           `gorm:"column:codex_secret" json:"codexSecret"`  // 是否法典机密
	Description  string         `gorm:"column:description" json:"description"`   // 描述
	RelicRewards []RelicRewards `gorm:"foreignKey:RelicsID" json:"relicRewards"` // 遗物内含物品（一对多）
}

func (Relics) TableName() string {
	return "relics"
}

// RelicRewards 遗物掉落物品条目。id 为 UUID 字符串主键。
// Rarity 数据库列存 RarityEnum ORDINAL 序数，JSON 输出枚举名（对齐 Jackson）。
// JSON 字段对齐前端 Api.LocalData.RelicReward。
type RelicRewards struct {
	ID         string `gorm:"primaryKey" json:"id"`                 // 主键（UUID）
	RelicsID   string `gorm:"column:relics_id" json:"-"`            // 所属遗物外键（不输出）
	RewardName string `gorm:"column:reward_name" json:"rewardName"` // 物品名称
	Rarity     int    `gorm:"column:rarity" json:"-"`               // 稀有度（RarityEnum ORDINAL，JSON 由 MarshalJSON 输出枚举名）
	Tier       int    `gorm:"column:tier" json:"tier"`              // 等级
	ItemCount  int    `gorm:"column:item_count" json:"itemCount"`   // 数量
}

func (RelicRewards) TableName() string {
	return "relic_rewards"
}

// MarshalJSON 输出 rarity 为枚举名字符串（对齐 Jackson 枚举序列化）。
func (record RelicRewards) MarshalJSON() ([]byte, error) {
	type plain RelicRewards
	aliased := plain(record)
	return json.Marshal(struct {
		plain
		Rarity string `json:"rarity"`
	}{plain: aliased, Rarity: rarityOrdinalToName(record.Rarity)})
}

// UnmarshalJSON 接受 rarity 为枚举名或序数，统一转 ORDINAL 入库。
func (record *RelicRewards) UnmarshalJSON(data []byte) error {
	var raw struct {
		ID         string          `json:"id"`
		RelicsID   string          `json:"relicsId"`
		RewardName string          `json:"rewardName"`
		Rarity     json.RawMessage `json:"rarity"`
		Tier       int             `json:"tier"`
		ItemCount  int             `json:"itemCount"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	record.ID = raw.ID
	record.RelicsID = raw.RelicsID
	record.RewardName = raw.RewardName
	record.Tier = raw.Tier
	record.ItemCount = raw.ItemCount

	var name string
	if err := json.Unmarshal(raw.Rarity, &name); err == nil {
		record.Rarity = rarityNameToOrdinal(name)
		return nil
	}
	var ordinal int
	if err := json.Unmarshal(raw.Rarity, &ordinal); err == nil {
		record.Rarity = ordinal
		return nil
	}
	return fmt.Errorf("invalid relic rarity %q", string(raw.Rarity))
}
