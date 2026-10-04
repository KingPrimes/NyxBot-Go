// 世界状态原始 JSON 的解码辅助类型。
//
// 官方 https://api.warframe.com/cdn/worldState.php 返回的是 **MongoDB 扩展 JSON**：
//
//	"_id": {"$oid": "6a735a7e9b3962c8397ea095"}
//	"Activation": {"$date": {"$numberLong": "1785945600000"}}
//
// Java 侧用 Jackson 把 _id 反序列化为 BastWorldState.Id（内含 $oid 字符串）、把时间
// 反序列化为 DateField（内含 $date.$numberLong 毫秒时间戳），并开启了
// ACCEPT_SINGLE_VALUE_AS_ARRAY（JsonNode 形态的单个对象可当单元素数组读）。
// Go 侧若直接声明成 string 会在 Unmarshal 阶段直接报错，导致全部依赖世界状态的指令失效，
// 因此这里统一提供三个容错类型：
//
//   - wsObjectID：_id / ChainID 这类对象 ID
//   - wsTime：Activation / Expiry 这类时间戳
//   - wsRewardList：Java 中声明为 List 但实际载荷为单个对象的奖励字段
//
// 三者都对「旧快照 / 测试构造的简化形态」（纯字符串、RFC3339、数组）保持兼容，且遇到
// 无法识别的形态时按「字段缺失」处理而不返回错误——单个字段格式变化不应让整条指令失败。
package warframe

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

// wsObjectID 世界状态中的对象 ID（真实载荷为 {"$oid":"..."}；兼容纯字符串与数字形态）。
type wsObjectID string

// String 返回 ID 文本（便于直接赋给以 string 描述 ID 的绘图 DTO）。
func (id wsObjectID) String() string {
	return string(id)
}

// UnmarshalJSON 兼容 {"$oid":"..."}、纯字符串、数字与 null；其余形态按缺失处理。
func (id *wsObjectID) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		*id = ""
		return nil
	}
	switch trimmed[0] {
	case '"':
		var text string
		if err := json.Unmarshal(trimmed, &text); err != nil {
			return err
		}
		*id = wsObjectID(text)
	case '{':
		// 扩展 JSON 的对象 ID：{"$oid":"..."}
		var field struct {
			OID json.RawMessage `json:"$oid"`
		}
		if err := json.Unmarshal(trimmed, &field); err != nil {
			return err
		}
		*id = wsObjectID(rawScalarString(field.OID))
	default:
		// 少数接口直接给出数字 ID
		*id = wsObjectID(string(trimmed))
	}
	return nil
}

// wsTime 世界状态时间字段（真实载荷为 {"$date":{"$numberLong":"毫秒"}}）。
// 同时兼容 RFC3339 字符串、{"$date":"RFC3339"}（Mongo 宽松扩展 JSON）与纯毫秒数字。
type wsTime struct {
	at time.Time
}

// Time 返回解析后的时间；字段缺失或形态无法识别时为零值（与 parseTime 的失败语义一致）。
func (t wsTime) Time() time.Time {
	return t.at
}

// UnmarshalJSON 解析扩展 JSON 时间；无法识别的形态按缺失处理（不中断整体解析）。
func (t *wsTime) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		t.at = time.Time{}
		return nil
	}
	switch trimmed[0] {
	case '"':
		var text string
		if err := json.Unmarshal(trimmed, &text); err != nil {
			return err
		}
		t.at = parseWorldStateTimeText(text)
	case '{':
		var field struct {
			Date json.RawMessage `json:"$date"`
		}
		if err := json.Unmarshal(trimmed, &field); err != nil {
			return err
		}
		t.at = parseWorldStateDateField(field.Date)
	default:
		// 纯毫秒时间戳
		t.at = parseWorldStateTimeText(string(trimmed))
	}
	return nil
}

// parseWorldStateDateField 解析 {"$date": ...} 的内容部分（对象 / 字符串 / 数字）。
func parseWorldStateDateField(raw json.RawMessage) time.Time {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return time.Time{}
	}
	switch trimmed[0] {
	case '{':
		var field struct {
			NumberLong json.RawMessage `json:"$numberLong"`
		}
		if err := json.Unmarshal(trimmed, &field); err != nil {
			return time.Time{}
		}
		// $numberLong 既可能是字符串（规范形态），也可能是数字
		return parseWorldStateTimeText(rawScalarString(field.NumberLong))
	default:
		return parseWorldStateTimeText(strings.Trim(string(trimmed), `"`))
	}
}

// parseWorldStateTimeText 解析时间文本：毫秒时间戳（字符串或数字）优先，其次 RFC3339。
func parseWorldStateTimeText(text string) time.Time {
	text = strings.TrimSpace(text)
	if text == "" {
		return time.Time{}
	}
	if millis, err := strconv.ParseInt(text, 10, 64); err == nil {
		return time.UnixMilli(millis)
	}
	// 兼容 "2026-08-07T10:00:00Z" 这类 RFC3339（nanos 版本可保留毫秒精度）
	if parsed, err := time.Parse(time.RFC3339, text); err == nil {
		return parsed
	}
	return time.Time{}
}

// rawScalarString 把 JSON 标量（字符串 / 数字 / null）转为不含引号的文本。
func rawScalarString(raw json.RawMessage) string {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return ""
	}
	return strings.Trim(string(trimmed), `"`)
}

// wsRewardList 奖励列表。Java 侧声明为 List<Reward> 且开启了 ACCEPT_SINGLE_VALUE_AS_ARRAY，
// 而真实载荷（如 Invasions[].AttackerReward）是单个对象，故两种形态都要接受。
type wsRewardList []wsReward

// UnmarshalJSON 兼容单对象（转为一个元素的列表）、数组与 null。
func (list *wsRewardList) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		*list = nil
		return nil
	}
	if trimmed[0] == '[' {
		var items []wsReward
		if err := json.Unmarshal(trimmed, &items); err != nil {
			return err
		}
		*list = items
		return nil
	}
	var single wsReward
	if err := json.Unmarshal(trimmed, &single); err != nil {
		return err
	}
	*list = wsRewardList{single}
	return nil
}
