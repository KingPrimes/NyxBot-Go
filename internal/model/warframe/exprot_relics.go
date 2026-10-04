// 遗物导出数据表，对应 Java NyxBot 的 exprot.Relics + RelicRewards 实体
// Relics（表 relics） - 存储遗物基本信息
// RelicRewards（表 relic_rewards） - 存储遗物内物品掉落（外键 relics_id）
package warframe

import (
	"encoding/json"
	"fmt"
	"strconv"
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

// RelicRewards 遗物掉落物品条目。id 为
// "{遗物uniqueName}|{奖励原始uniqueName}|{稀有度}|{等级}|{数量}" 的确定性主键。
// 官方导出文件的 relicRewards 不含 id，且同一奖励物品（如赤毒、Forma 蓝图）会被多个遗物共用；
// 若直接拿奖励物品名当主键，不同遗物会互相覆盖（对齐 Java @GeneratedValue(UUID) 的「每个遗物各自成行」语义，
// 但保持可重复导入的幂等性）。
// Rarity 数据库列存 RarityEnum ORDINAL 序数，JSON 输出枚举名（对齐 Jackson）。
// JSON 字段对齐前端 Api.LocalData.RelicReward。
type RelicRewards struct {
	ID         string `gorm:"primaryKey" json:"id"`                 // 主键（遗物uniqueName|奖励uniqueName|稀有度|等级|数量）
	RelicsID   string `gorm:"column:relics_id" json:"-"`            // 所属遗物外键（不输出）
	RewardName string `gorm:"column:reward_name" json:"rewardName"` // 物品名称（已翻译为中文，未命中保留原始 uniqueName）
	Rarity     int    `gorm:"column:rarity" json:"-"`               // 稀有度（RarityEnum ORDINAL，JSON 由 MarshalJSON 输出枚举名）
	Tier       int    `gorm:"column:tier" json:"tier"`              // 等级
	ItemCount  int    `gorm:"column:item_count" json:"itemCount"`   // 数量
}

func (RelicRewards) TableName() string {
	return "relic_rewards"
}

// MarshalJSON 输出 rarity 为枚举名字符串（对齐 Jackson 枚举序列化），
// 并在 itemCount > 1 时把数量前缀进 rewardName（对齐 Java RelicRewards.getRewardName）：
// 例如 1200X赤毒。
func (record RelicRewards) MarshalJSON() ([]byte, error) {
	type plain RelicRewards
	aliased := plain(record)
	rewardName := record.RewardName
	if record.ItemCount > 1 {
		rewardName = strconv.Itoa(record.ItemCount) + "X" + rewardName
	}
	return json.Marshal(struct {
		plain
		RewardName string `json:"rewardName"`
		Rarity     string `json:"rarity"`
	}{plain: aliased, RewardName: rewardName, Rarity: rarityOrdinalToName(record.Rarity)})
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
