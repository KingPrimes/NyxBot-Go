// 奖励池导出数据表，对应 Java NyxBot 的 exprot.RewardPool + Reward 实体
// RewardPool - 存储奖励池信息（如特定掉落表）
// Reward - 存储奖池内的具体奖励项（名称/稀有度/概率）
package warframe

type RewardPool struct {
	ID          uint     `gorm:"primaryKey"`                    // 主键
	Name        string   `gorm:"column:name"`                   // 奖励池名称（中文）
	EnName      string   `gorm:"column:en_name"`                // 英文名
	ImageName   string   `gorm:"column:image_name"`             // 图片文件名
	Tags        string   `gorm:"column:tags"`                   // 标签
	Description string   `gorm:"column:description"`            // 描述
	Exclude     bool     `gorm:"column:exclude"`                // 是否排除
	Rewards     []Reward `gorm:"foreignKey:PoolID"`             // 奖励列表（一对多）
}

func (RewardPool) TableName() string {
	return "exprot_reward_pool"
}

type Reward struct {
	ID     uint    `gorm:"primaryKey"`       // 主键
	PoolID uint    `gorm:"column:pool_id"`   // 所属奖励池外键
	Name   string  `gorm:"column:name"`      // 奖励名称
	Rarity string  `gorm:"column:rarity"`    // 稀有度
	Chance float64 `gorm:"column:chance"`    // 掉落概率
}

func (Reward) TableName() string {
	return "exprot_reward"
}
