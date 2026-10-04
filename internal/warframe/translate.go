// 世界状态指令翻译工具：节点 / 物品名 中文化（对齐 Java WorldStateUtils 的
// nodesRepository + StateTranslationRepository 查询语义）。
package warframe

import (
	"errors"
	"strings"

	"gorm.io/gorm"

	"nyxbot-go/internal/database"
	"nyxbot-go/internal/logging"
	modelwarframe "nyxbot-go/internal/model/warframe"
)

// TranslateNode 将节点唯一标识翻译为 "名称(星系)"（对齐 Java WorldStateUtils：
// nodesRepository.findById(node) → name + "(" + systemName + ")"）。
// 未命中（或节点没有名称）时回退原文并登记到未翻译清单，供后续补数据。
func TranslateNode(nodeKey string) string {
	if nodeKey == "" || database.DB == nil {
		return nodeKey
	}
	var node modelwarframe.Nodes
	if err := database.DB.Where("unique_name = ?", nodeKey).First(&node).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			logging.DebugPack("warframe.translate", "query node %q failed: %v", nodeKey, err)
			return nodeKey
		}
		recordTranslationMiss(nodeKey)
		return nodeKey
	}
	name := node.Name
	if name == "" {
		recordTranslationMiss(nodeKey)
		return nodeKey
	}
	if node.SystemName != "" {
		return name + "(" + node.SystemName + ")"
	}
	return name
}

// TranslateStateName 将物品唯一标识翻译为中文名（对齐 Java StateTranslationService.getName：
// 先按 StringUtils.getLastThreeSegments 截取末三段，再用 StateTranslationRepository.findByUniqueName
// 的后缀语义查询 state_translation）。
// 未命中时回退原文并登记到未翻译清单，供后续补翻译。
func TranslateStateName(uniqueName string) string {
	if uniqueName == "" {
		return uniqueName
	}
	if name := TranslateStateNameSuffix(getLastThreeSegments(uniqueName)); name != "" {
		return name
	}
	recordTranslationMiss(uniqueName)
	return uniqueName
}

// TranslateStateNameDirect 按原始 uniqueName 查 state_translation（不做 last3 段截取），
// 同样使用 Java 的后缀语义。用于 Java 侧直接 findByUniqueName(完整路径) 的场景：
// 警报奖励物品、1999 日历事件字段等。未命中时回退原文并登记到未翻译清单。
func TranslateStateNameDirect(uniqueName string) string {
	if uniqueName == "" {
		return uniqueName
	}
	if name := TranslateStateNameSuffix(uniqueName); name != "" {
		return name
	}
	recordTranslationMiss(uniqueName)
	return uniqueName
}

// recordTranslationMiss 登记一次翻译未命中（uniqueName 为空时不登记）。
// 数据库不可用时说明翻译查询压根没有执行（如单元测试、启动早期），此时不登记，
// 否则会把整套原始数据写进未翻译清单。
func recordTranslationMiss(uniqueName string) {
	if uniqueName == "" || database.DB == nil {
		return
	}
	RecordUntranslated(uniqueName)
}

// TranslateStateNameSuffix 按 Java StateTranslationRepository.findByUniqueName 的后缀语义取中文名：
// JPQL 为 RIGHT(st.uniqueName, LENGTH(:key)) = :key（state_translation 存的是完整 uniqueName，
// 调用方通常只传末三段）。这里用 LIKE 缩小候选 + Go 侧 strings.HasSuffix 精确校验，
// 取最长（最具体）的命中行；未命中或 name 为空返回空串。
// 用 Limit(1).Find 而非 First：未命中是常态（业务回退原值），不应产生 ErrRecordNotFound 错误路径。
func TranslateStateNameSuffix(key string) string {
	if key == "" || database.DB == nil {
		return ""
	}
	var record modelwarframe.StateTranslation
	err := database.DB.
		Where("name IS NOT NULL AND name <> ''").
		Where("unique_name LIKE ? ESCAPE '\\'", "%"+escapeLikeKey(key)).
		Order("length(unique_name) DESC").
		Limit(1).
		Find(&record).Error
	if err != nil {
		logging.DebugPack("warframe.translate", "query state_translation %q failed: %v", key, err)
		return ""
	}
	if record.UniqueName == "" || !strings.HasSuffix(record.UniqueName, key) {
		return ""
	}
	return record.Name
}

// escapeLikeKey 转义 LIKE 通配符（配合 ESCAPE '\'），保证 key 中的 _ / % 按字面量参与粗筛。
func escapeLikeKey(key string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(key)
}

// LoadStateTranslationMap 批量加载「uniqueName 末段关键词 -> 中文名」映射
// （对齐 Java RelicsImportUtil.loadTranslationMap：关键词取末三段，按 endsWith 归并）。
// 实现上一次性取出 (unique_name, name) 候选集，再在 Go 侧用 uniqueName 的末 1/2/3 段做后缀匹配：
// SQLite 的前导通配 LIKE 无法走索引，实测 200 个 OR 条件的一批就要 570ms+（全表扫描 ×200），
// 整表扫一次稳定在几十毫秒。同一关键词保留首个命中（与表扫描顺序一致）。
func LoadStateTranslationMap(keywords []string) map[string]string {
	result := make(map[string]string, len(keywords))
	if len(keywords) == 0 || database.DB == nil {
		return result
	}
	wanted := make(map[string]struct{}, len(keywords))
	for _, keyword := range keywords {
		if keyword != "" {
			wanted[keyword] = struct{}{}
		}
	}
	if len(wanted) == 0 {
		return result
	}

	var rows []modelwarframe.StateTranslation
	if err := database.DB.
		Model(&modelwarframe.StateTranslation{}).
		Select("unique_name", "name").
		Where("name IS NOT NULL AND name <> ''").
		Find(&rows).Error; err != nil {
		logging.WarnPack("warframe.translate", "load state translation map failed: %v", err)
		return result
	}
	for _, row := range rows {
		for _, tail := range pathTails(row.UniqueName) {
			if _, ok := wanted[tail]; !ok || result[tail] != "" {
				continue
			}
			result[tail] = row.Name
		}
	}
	return result
}

// pathTails 返回路径的末 1/2/3 段（不足则返回现有段），用于后缀匹配翻译关键词。
// 关键词总是不超过三段（getLastThreeSegments 的结果），因此检查三个候选即可覆盖全部后缀情况。
func pathTails(value string) []string {
	parts := make([]string, 0, 8)
	for _, segment := range strings.Split(value, "/") {
		if strings.TrimSpace(segment) != "" {
			parts = append(parts, segment)
		}
	}
	tails := make([]string, 0, 3)
	for length := 1; length <= 3 && length <= len(parts); length++ {
		tails = append(tails, strings.Join(parts[len(parts)-length:], "/"))
	}
	return tails
}

// getLastThreeSegments 取 uniqueName 最后三段（对齐 Java StringUtils.getLastThreeSegments，
// 用于 state_translation 翻译命中：/Lotus/StoreItems/A/B → StoreItems/A/B）。
func getLastThreeSegments(value string) string {
	parts := make([]string, 0, 8)
	for _, segment := range strings.Split(value, "/") {
		if strings.TrimSpace(segment) != "" {
			parts = append(parts, segment)
		}
	}
	if len(parts) <= 3 {
		return strings.Join(parts, "/")
	}
	return strings.Join(parts[len(parts)-3:], "/")
}
