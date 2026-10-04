// 状态翻译表，对应 Java NyxBot 的 StateTranslation 实体（表 state_translation）
// 存储 WorldState 物品/任务状态的中文翻译映射，供状态类指令翻译使用
package warframe

import (
	"encoding/json"
	"fmt"
)

// StateTranslation 状态翻译条目。uniqueName 为字符串主键。
// Type 数据库列存 StateTypeEnum 的 ORDINAL 序数（对齐 JPA 默认 @Enumerated ORDINAL），
// JSON 输出枚举名字符串（对齐 Jackson WRITE_ENUMS_USING_TO_STRING=false 与前端 typeMap 契约）。
// JSON 字段对齐前端 Api.LocalData.StateTranslation。
type StateTranslation struct {
	UniqueName  string `gorm:"primaryKey" json:"uniqueName"`                    // 主键（唯一标识）
	Name        string `gorm:"column:name" json:"name"`                         // 中文名（unique + name 联合）
	Description string `gorm:"column:description;type:text" json:"description"` // 描述（Java 兼容 List 时取首元素）
	Type        int    `gorm:"column:type" json:"-"`                            // 状态类型（ORDINAL，JSON 由 MarshalJSON 输出枚举名）
	ParentName  string `gorm:"column:parent_name" json:"parentName"`            // 父级名称
}

func (StateTranslation) TableName() string {
	return "state_translation"
}

// MarshalJSON 输出 type 为枚举名字符串（对齐 Jackson 枚举序列化）。
func (record StateTranslation) MarshalJSON() ([]byte, error) {
	type plain StateTranslation
	aliased := plain(record)
	return json.Marshal(struct {
		plain
		Type string `json:"type"`
	}{plain: aliased, Type: StateTypeOrdinalToName(record.Type)})
}

// UnmarshalJSON 接受 type 为枚举名（前端契约）或序数（防御），统一转 ORDINAL 入库。
func (record *StateTranslation) UnmarshalJSON(data []byte) error {
	var raw struct {
		UniqueName  string          `json:"uniqueName"`
		Name        string          `json:"name"`
		Description json.RawMessage `json:"description"`
		Type        json.RawMessage `json:"type"`
		ParentName  string          `json:"parentName"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	record.UniqueName = raw.UniqueName
	record.Name = raw.Name
	record.Description = firstDescription(raw.Description)
	record.ParentName = raw.ParentName

	var typeName string
	if err := json.Unmarshal(raw.Type, &typeName); err == nil {
		record.Type = StateTypeNameToOrdinal(typeName)
		return nil
	}
	var typeOrdinal int
	if err := json.Unmarshal(raw.Type, &typeOrdinal); err == nil {
		record.Type = typeOrdinal
		return nil
	}
	return fmt.Errorf("invalid state translation type %q", string(raw.Type))
}

// firstDescription 兼容 description 为字符串或数组（对齐 Java 反序列化取首元素）。
func firstDescription(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text
	}
	var list []string
	if err := json.Unmarshal(raw, &list); err == nil && len(list) > 0 {
		return list[0]
	}
	return ""
}
