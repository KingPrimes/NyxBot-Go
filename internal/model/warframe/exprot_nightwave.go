// 午夜电波导出数据表，对应 Java NyxBot 的 exprot.NightWave 实体（表 night_wave）
// 存储从官方 API 导出的午夜电波挑战数据
package warframe

import (
	"encoding/json"
	"strings"
)

// NightWave 午夜电波挑战条目。uniqueName 为字符串主键。
// Description 入库保留原始文本；JSON 输出时 |COUNT| 替换为 Required（对齐 Java getter 行为）。
// JSON 字段对齐前端 Api.LocalData.NightWave。
type NightWave struct {
	UniqueName  string `gorm:"primaryKey" json:"uniqueName"`          // 主键（唯一标识）
	Name        string `gorm:"column:name" json:"name"`               // 挑战名称（中文）
	Description string `gorm:"column:description" json:"description"` // 挑战描述（含 |COUNT| 占位）
	Standing    int    `gorm:"column:standing" json:"standing"`       // 声望值
	Required    int    `gorm:"column:required" json:"required"`       // 需要完成次数
}

func (NightWave) TableName() string {
	return "night_wave"
}

// MarshalJSON 输出时替换 description 中的 |COUNT| 占位（对齐 Java getDescription()）。
func (record NightWave) MarshalJSON() ([]byte, error) {
	type plain NightWave
	aliased := plain(record)
	aliased.Description = strings.ReplaceAll(record.Description, "|COUNT|", toString(record.Required))
	return json.Marshal(aliased)
}
