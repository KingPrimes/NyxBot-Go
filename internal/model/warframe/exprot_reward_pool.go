// 奖励池导出数据表，对应 Java NyxBot 的 exprot.reward.RewardPool + Reward 实体
// RewardPool（表 reward_pool） - 存储奖励池信息
// Reward（表 reward） - 存储奖池内的具体奖励项（外键 pool_id）
package warframe

import (
	"encoding/json"
	"fmt"
)

// RewardPool 奖励池条目。uniqueName 为字符串主键。
// JSON 字段对齐前端 Api.LocalData.RewardPool。
type RewardPool struct {
	UniqueName string   `gorm:"primaryKey" json:"uniqueName"`     // 主键（唯一标识）
	Rewards    []Reward `gorm:"foreignKey:PoolID" json:"rewards"` // 奖励列表（一对多）
}

func (RewardPool) TableName() string {
	return "reward_pool"
}

// Reward 奖励池内奖励项。id 为 UUID 字符串主键。
// Rarity 数据库列存 RarityEnum ORDINAL 序数，JSON 输出枚举名（对齐 Jackson）。
// JSON 字段对齐前端 Api.LocalData.Reward。
type Reward struct {
	ID        string `gorm:"primaryKey" json:"id"`               // 主键（UUID）
	PoolID    string `gorm:"column:pool_id" json:"-"`            // 所属奖励池外键（不输出）
	Item      string `gorm:"column:item" json:"item"`            // 奖励名称
	Rarity    int    `gorm:"column:rarity" json:"-"`             // 稀有度（RarityEnum ORDINAL，JSON 由 MarshalJSON 输出枚举名）
	ItemCount int    `gorm:"column:item_count" json:"itemCount"` // 数量
}

func (Reward) TableName() string {
	return "reward"
}

// MarshalJSON 输出 rarity 为枚举名字符串（对齐 Jackson 枚举序列化）。
func (record Reward) MarshalJSON() ([]byte, error) {
	type plain Reward
	aliased := plain(record)
	return json.Marshal(struct {
		plain
		Rarity string `json:"rarity"`
	}{plain: aliased, Rarity: rarityOrdinalToName(record.Rarity)})
}

// UnmarshalJSON 接受 rarity 为枚举名或序数，统一转 ORDINAL 入库。
func (record *Reward) UnmarshalJSON(data []byte) error {
	var raw struct {
		ID        string          `json:"id"`
		PoolID    string          `json:"poolId"`
		Item      string          `json:"item"`
		Rarity    json.RawMessage `json:"rarity"`
		ItemCount int             `json:"itemCount"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	record.ID = raw.ID
	record.PoolID = raw.PoolID
	record.Item = raw.Item
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
	return fmt.Errorf("invalid reward rarity %q", string(raw.Rarity))
}
