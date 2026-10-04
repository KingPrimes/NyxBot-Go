// 未翻译内容清单：把查不到译名的 uniqueName 汇总到独立 JSON 文件（默认
// ./data/UntranslatedRelicsRewardsName.json），供人工补齐译名后回填 state_translation / nodes。
//
// 两个来源：
//   - 遗物导入：奖励名查不到译名（对齐 Java RelicsImportUtil.exportUntranslatedItems）；
//   - 运行期世界状态：物品/奖励/挑战/赏金/节点的翻译未命中并回退原文时登记（见 recordTranslationMiss）。
//
// 登记前去重：uniqueName 已在文件中（或本次进程内已登记）则跳过。
// 磁盘是权威：每次写回前重新读盘，人工填写的译名/描述不会被覆盖，人工删掉的条目也不会被复活；
// 写入先落 ".tmp" 再原子改名，避免读到半截内容。
package warframe

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode"

	"nyxbot-go/internal/logging"
)

// UntranslatedRelicsRewardsPath 未翻译清单的默认输出路径
// （对齐 Java RelicsImportUtil.exportUntranslatedItems 的 ./data/UntranslatedRelicsRewardsName.json）。
const UntranslatedRelicsRewardsPath = "./data/UntranslatedRelicsRewardsName.json"

// UntranslatedItem 未翻译条目，字段与 Java 导出的 Map 结构一致：
// UniqueName 为原始 uniqueName（可为带 "/" 的物品/任务路径，也可为 SolNodeXX 这类节点键），
// Name 为按 Java 规则猜测的中文名（人工修正后回填），Description 为对应描述的待译文本。
type UntranslatedItem struct {
	UniqueName  string `json:"uniqueName"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// untranslatedRegistry 未翻译清单的进程内缓存与去重索引（运行期可能被多个 HTTP 处理器并发访问）。
// 磁盘是权威：每次写回前重新读一遍文件，人工在文件里填的译名/删掉的条目都不会被覆盖或复活。
type untranslatedRegistry struct {
	mu         sync.Mutex
	path       string              // 清单文件路径
	loaded     bool                // 是否已尝试从磁盘加载
	loadErr    error               // 加载失败原因：非空时不再写回，避免覆盖人工编辑的文件
	loadWarned bool                // 加载失败是否已告警（避免每次未命中都刷日志）
	index      map[string]struct{} // 去重索引（最近一次与磁盘同步的结果）
	pending    []UntranslatedItem  // 尚未成功落盘的新条目，下次写回时一并写入
}

// untranslated 进程内唯一的未翻译清单注册表。
var untranslated = &untranslatedRegistry{
	path:  UntranslatedRelicsRewardsPath,
	index: make(map[string]struct{}),
}

// UntranslatedListPath 返回当前未翻译清单的文件路径。
func UntranslatedListPath() string {
	untranslated.mu.Lock()
	defer untranslated.mu.Unlock()
	return untranslated.path
}

// SetUntranslatedPath 切换未翻译清单文件路径并清空进程内缓存，返回原路径（供测试与工具使用）。
func SetUntranslatedPath(path string) string {
	registry := untranslated
	registry.mu.Lock()
	defer registry.mu.Unlock()
	previous := registry.path
	registry.path = path
	registry.loaded = false
	registry.loadErr = nil
	registry.loadWarned = false
	registry.index = make(map[string]struct{})
	registry.pending = nil
	return previous
}

// RecordUntranslated 登记一次运行期翻译未命中：uniqueName 为空或已在清单中则忽略，
// 新条目按 GuessUntranslatedName 生成草稿名后立即写回文件。
// 写失败只告警，条目留在待写队列里，等下一次写成功时一并落盘。
func RecordUntranslated(uniqueName string) {
	if uniqueName == "" {
		return
	}
	registry := untranslated
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if err := registry.loadLocked(); err != nil {
		if !registry.loadWarned {
			registry.loadWarned = true
			logging.WarnPack("warframe.translate",
				"untranslated list %s unreadable, runtime translation misses are not recorded: %v",
				registry.path, err)
		}
		return
	}
	if !registry.recordLocked(UntranslatedItem{
		UniqueName: uniqueName,
		Name:       GuessUntranslatedName(uniqueName),
	}) {
		return
	}
	if err := registry.flushLocked(); err != nil {
		logging.WarnPack("warframe.translate",
			"write untranslated list %s failed: %v", registry.path, err)
	}
}

// MergeUntranslatedItems 把一批未翻译条目合并进清单（遗物等批量导入场景），返回新增条数。
// 已存在的 uniqueName 跳过，文件中原有条目（含人工填写的译名/描述）保持不变；
// 条目自带 name 为空时按 GuessUntranslatedName 补草稿名。
func MergeUntranslatedItems(items []UntranslatedItem) (int, error) {
	registry := untranslated
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if err := registry.loadLocked(); err != nil {
		return 0, err
	}
	added := 0
	for _, item := range items {
		if item.UniqueName == "" {
			continue
		}
		if item.Name == "" {
			item.Name = GuessUntranslatedName(item.UniqueName)
		}
		if registry.recordLocked(item) {
			added++
		}
	}
	if added == 0 {
		return 0, nil
	}
	if err := registry.flushLocked(); err != nil {
		return added, err
	}
	return added, nil
}

// recordLocked 把一条新条目放入待写队列（uniqueName 为空或已登记时返回 false）。
// 去重索引立即更新，避免同一进程内对同一未命中项反复排队。
func (registry *untranslatedRegistry) recordLocked(item UntranslatedItem) bool {
	if item.UniqueName == "" {
		return false
	}
	if _, exists := registry.index[item.UniqueName]; exists {
		return false
	}
	registry.index[item.UniqueName] = struct{}{}
	registry.pending = append(registry.pending, item)
	return true
}

// flushLocked 把待写队列合并进清单文件：先重新读盘（磁盘为准，人工修改的译名保留、
// 人工删除的条目不会被复活），再按 uniqueName 去重追加待写条目，最后整体原子写入。
// 写成功后清空待写队列并按磁盘结果重建去重索引；失败则保留队列，等下次写入重试。
func (registry *untranslatedRegistry) flushLocked() error {
	diskItems, err := loadUntranslatedFile(registry.path)
	if err != nil {
		return err
	}
	merged := make([]UntranslatedItem, 0, len(diskItems)+len(registry.pending))
	index := make(map[string]struct{}, len(diskItems)+len(registry.pending))
	for _, item := range diskItems {
		if item.UniqueName == "" {
			continue
		}
		if _, exists := index[item.UniqueName]; exists {
			continue
		}
		index[item.UniqueName] = struct{}{}
		merged = append(merged, item)
	}
	for _, item := range registry.pending {
		if item.UniqueName == "" {
			continue
		}
		if _, exists := index[item.UniqueName]; exists {
			continue
		}
		index[item.UniqueName] = struct{}{}
		merged = append(merged, item)
	}
	if err := SaveUntranslatedItems(registry.path, merged); err != nil {
		return err
	}
	registry.index = index
	registry.pending = nil
	return nil
}

// loadLocked 首次访问时把磁盘上的清单读入内存，建立去重索引。
// 解析失败时记录错误并让后续登记全部跳过：宁可少记，也不能覆盖人工编辑过但语法损坏的文件。
func (registry *untranslatedRegistry) loadLocked() error {
	if registry.loaded {
		return registry.loadErr
	}
	registry.loaded = true
	items, err := loadUntranslatedFile(registry.path)
	if err != nil {
		registry.loadErr = err
		return err
	}
	for _, item := range items {
		if item.UniqueName == "" {
			continue
		}
		registry.index[item.UniqueName] = struct{}{}
	}
	return nil
}

// loadUntranslatedFile 读取并解析清单文件；文件不存在或内容为空视为空清单。
func loadUntranslatedFile(path string) ([]UntranslatedItem, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, nil
	}
	var items []UntranslatedItem
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("parse untranslated list %q: %w", path, err)
	}
	return items, nil
}

// CollectUntranslatedItems 把未翻译的 uniqueName 列表整理成清单条目：
// 按 uniqueName 去重（保留首次出现顺序，对齐 Java addToUntranslatedItems 的存在性判断），
// 空名跳过，name 由 GuessUntranslatedName 生成，description 留空。
func CollectUntranslatedItems(uniqueNames []string) []UntranslatedItem {
	items := make([]UntranslatedItem, 0, len(uniqueNames))
	seen := make(map[string]bool, len(uniqueNames))
	for _, uniqueName := range uniqueNames {
		if uniqueName == "" || seen[uniqueName] {
			continue
		}
		seen[uniqueName] = true
		items = append(items, UntranslatedItem{
			UniqueName:  uniqueName,
			Name:        GuessUntranslatedName(uniqueName),
			Description: "",
		})
	}
	return items
}

// GuessUntranslatedName 按 Java addToUntranslatedItems 的规则猜测中文名：
// 取路径最后一段 → splitCamelCase 拆词并规范化大小写 → 依次替换常见部件后缀
// （Blueprint→蓝图 / Systems→系统 / Chassis→机体 / Helmet→头部神经光元）。
// 结果是给人工参考的草稿，不是正式译名。
func GuessUntranslatedName(uniqueName string) string {
	if uniqueName == "" {
		return ""
	}
	guessed := splitCamelCase(lastSegment(uniqueName))
	guessed = strings.ReplaceAll(guessed, "Blueprint", "蓝图")
	guessed = strings.ReplaceAll(guessed, "Systems", "系统")
	guessed = strings.ReplaceAll(guessed, "Chassis", "机体")
	guessed = strings.ReplaceAll(guessed, "Helmet", "头部神经光元")
	return guessed
}

// SaveUntranslatedItems 把整份清单覆盖写入 JSON 数组文件，返回错误时文件内容保持不变。
// 目录不存在时自动创建；先写 "<path>.tmp" 再原子改名，避免人工查看时读到半截文件。
// 编码用 SetEscapeHTML(false)：译名可能含 <...> 标记，转义成 \u003c 会让人工编辑变得困难。
// 空清单写出空数组（表示当前无未翻译项）。
func SaveUntranslatedItems(path string, items []UntranslatedItem) error {
	if items == nil {
		items = []UntranslatedItem{}
	}
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(items); err != nil {
		return fmt.Errorf("marshal untranslated items: %w", err)
	}
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create untranslated directory %q: %w", dir, err)
		}
	}
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, buffer.Bytes(), 0o644); err != nil {
		return fmt.Errorf("write untranslated items %q: %w", path, err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("replace untranslated items %q: %w", path, err)
	}
	return nil
}

// lastSegment 取路径最后一个 "/" 之后的内容（对齐 Java StringUtils.getLastValueAfterSlash）。
// 没有 "/" 时返回原字符串。
func lastSegment(value string) string {
	index := strings.LastIndex(value, "/")
	if index < 0 {
		return value
	}
	return value[index+1:]
}

// splitCamelCase 复刻 Java StringUtils.splitCamelCase：
// 先在大写字母前插入空格（仅当前一个字符不是数字，对应正则 (?<=\D)(?=\p{Upper})），
// 再做 capitalizeFully 规范化（空白分隔的词首字符大写、其余小写）。
func splitCamelCase(input string) string {
	if input == "" {
		return input
	}
	var builder strings.Builder
	builder.Grow(len(input) + 8)
	var previous rune
	for index, current := range input {
		if index > 0 && unicode.IsUpper(current) && !unicode.IsDigit(previous) {
			builder.WriteRune(' ')
		}
		builder.WriteRune(current)
		previous = current
	}
	return capitalizeFully(builder.String())
}

// capitalizeFully 复刻 Apache commons WordUtils.capitalizeFully(String)（默认空白分隔符）：
// 整串先转小写，再让每个空白分隔词的首字符用 toTitleCase 大写。
func capitalizeFully(input string) string {
	var builder strings.Builder
	builder.Grow(len(input))
	atWordStart := true
	for _, current := range input {
		switch {
		case unicode.IsSpace(current):
			builder.WriteRune(current)
			atWordStart = true
		case atWordStart:
			builder.WriteRune(unicode.ToTitle(current))
			atWordStart = false
		default:
			builder.WriteRune(unicode.ToLower(current))
		}
	}
	return builder.String()
}
