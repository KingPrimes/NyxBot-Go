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
// 未命中或数据库不可用时回退原文。
func TranslateNode(nodeKey string) string {
	if nodeKey == "" || database.DB == nil {
		return nodeKey
	}
	var node modelwarframe.Nodes
	if err := database.DB.Where("unique_name = ?", nodeKey).First(&node).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			logging.DebugPack("warframe.translate", "query node %q failed: %v", nodeKey, err)
		}
		return nodeKey
	}
	name := node.Name
	if name == "" {
		return nodeKey
	}
	if node.SystemName != "" {
		return name + "(" + node.SystemName + ")"
	}
	return name
}

// TranslateStateName 将物品唯一标识翻译为中文名（对齐 Java StateTranslationService.getName：
// 查 state_translation 表取 Name，为空回退原文）。
// key 默认取最后三段（对齐 StringUtils.getLastThreeSegments）。
func TranslateStateName(uniqueName string) string {
	if uniqueName == "" || database.DB == nil {
		return uniqueName
	}
	key := getLastThreeSegments(uniqueName)
	var st modelwarframe.StateTranslation
	if err := database.DB.Where("unique_name = ?", key).First(&st).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			logging.DebugPack("warframe.translate", "query state_translation %q failed: %v", key, err)
		}
		return uniqueName
	}
	if st.Name != "" {
		return st.Name
	}
	return uniqueName
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
