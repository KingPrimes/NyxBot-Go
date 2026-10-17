// 仲裁节点分级表（表 arbitration_tier）
// 数据源为 CDN 的 warframe/arbitration_tiers.json，见 plans/arbitration-tier-plan.md。
package warframe

// ArbitrationTier 仲裁节点分级条目。Key 为节点 SolNode ID，与 nodes.unique_name 同键；
// Tier 取 S/A/B/C/D/F，无分级节点由消费侧缺省视为 F。
type ArbitrationTier struct {
	Key  string `gorm:"primaryKey;column:key" json:"key"` // 节点 SolNode ID
	Tier string `gorm:"column:tier" json:"tier"`          // 评级 S/A/B/C/D/F
}

func (ArbitrationTier) TableName() string {
	return "arbitration_tier"
}
