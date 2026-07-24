// 节点/掉落源导出数据表，对应 Java NyxBot 的 exprot.Nodes 实体
// 存储从官方 API 导出的星球节点/掉落来源数据
package warframe

type Nodes struct {
	ID          uint   `gorm:"primaryKey"`          // 主键
	Name        string `gorm:"column:name"`         // 节点名称（中文）
	EnName      string `gorm:"column:en_name"`      // 英文名
	ImageName   string `gorm:"column:image_name"`   // 图片文件名
	Tradable    string `gorm:"column:tradable"`     // 可交易状态
	Tags        string `gorm:"column:tags"`         // 标签
	Description string `gorm:"column:description"`  // 描述
	Exclude     bool   `gorm:"column:exclude"`      // 是否排除
}

func (Nodes) TableName() string {
	return "exprot_nodes"
}
