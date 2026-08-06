// 节点导出数据表，对应 Java NyxBot 的 exprot.Nodes 实体（表 nodes）
// 存储从官方 API 导出的星球节点数据（任务类型、敌人等级、派系等）
package warframe

// Nodes 星球节点条目。uniqueName 为字符串主键。
// JSON 字段对齐前端 Api.LocalData.Nodes。
type Nodes struct {
	UniqueName    string `gorm:"primaryKey" json:"uniqueName"`                // 主键（唯一标识）
	Name          string `gorm:"column:name" json:"name"`                     // 节点名称（中文）
	SystemName    string `gorm:"column:system_name" json:"systemName"`        // 所在星系名称
	SystemIndex   int    `gorm:"column:system_index" json:"systemIndex"`      // 星系序号
	NodeType      int    `gorm:"column:node_type" json:"nodeType"`            // 节点类型
	MasteryReq    int    `gorm:"column:mastery_req" json:"masteryReq"`        // 段位要求
	MissionIndex  int    `gorm:"column:mission_index" json:"missionIndex"`    // 任务类型序号
	FactionIndex  int    `gorm:"column:faction_index" json:"factionIndex"`    // 派系序号
	MinEnemyLevel int    `gorm:"column:min_enemy_level" json:"minEnemyLevel"` // 最低敌人等级
	MaxEnemyLevel int    `gorm:"column:max_enemy_level" json:"maxEnemyLevel"` // 最高敌人等级
}

func (Nodes) TableName() string {
	return "nodes"
}
