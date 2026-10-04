// 配置文件的最小写入：保存配置时把落盘改动限制到最小范围。
//
// 背景：原先 Runtime.Update 一律把整份配置重新序列化写回，用户手写的注释、空行、自定义键与键序
// 都会被生成格式覆盖，而结构体里新增的配置项又不会出现在已有文件里。现在的落盘规则：
//  1. 值发生变化的已有键 → 只替换该键对应的原始行（保留缩进与行内注释；该行按标准 YAML 格式重渲染，
//     因此注释前的对齐空格会归一化，行前注释与相邻行则原样不动）；
//  2. 结构体里有、YAML 树里缺失的键 → 按 comment 标签补写（含注释行）到所属映射末尾（只补缺失键）；
//  3. 其余内容逐字节保留；补丁结果会重新解析并校验与期望配置一致，否则拒绝写入。
package config

import (
	"bytes"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// patchEdit 一处最小改动：把原始文件的 [start, end] 行（1-based、含端点）替换为 lines。
// 插入操作用 end = start-1（空区间），即在 end 行之后插入。
// depth 为所属映射的嵌套层级：同一行上既有内层插入又有外层追加时，先应用外层（见 applyPatches）。
type patchEdit struct {
	start int
	end   int
	depth int
	lines []string
}

// syncConfigFile 依据 before → after 的差异最小化写回 path：
// 只改动值变化的键所在行，并补齐结构体里有而文件里缺的键；文件不存在时退回整份生成。
func syncConfigFile(path string, before, after Config) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return writeConfig(path, after)
		}
		return err
	}

	beforeNode, err := configNode(before)
	if err != nil {
		return err
	}
	afterNode, err := configNode(after)
	if err != nil {
		return err
	}
	var fileNode yaml.Node
	if err := yaml.Unmarshal(raw, &fileNode); err != nil {
		return fmt.Errorf("%s 解析失败，为免覆盖现有内容未写入: %w", path, err)
	}

	lineEnding := configLineEnding(raw)
	lines, trailingNewline := configLines(raw)
	edits := make([]patchEdit, 0, 8)
	if err := collectPatches(documentMapping(&fileNode), documentMapping(afterNode), documentMapping(beforeNode), "", 0, lines, &edits); err != nil {
		return err
	}

	patched := applyPatches(lines, edits)
	content := []byte(strings.Join(patched, lineEnding))
	if trailingNewline {
		content = append(content, lineEnding...)
	}
	// 校验：补丁结果必须能解析成与 after 等价的配置，否则宁可不写（避免写出坏配置）
	var check Config
	if err := yaml.Unmarshal(content, &check); err != nil {
		return fmt.Errorf("配置补丁校验失败，未写入: %w", err)
	}
	if !sameConfig(check, after) {
		return fmt.Errorf("配置补丁校验失败：写回结果与期望配置不一致，未写入")
	}
	return writeFileAtomic(path, content)
}

// collectPatches 递归比对一个映射节点，收集最小改动：
// 已有键中值发生变化的只替换该键的原始行，after 里有而文件里没有的键整体补写到映射末尾。
func collectPatches(file, after, before *yaml.Node, indent string, depth int, lines []string, edits *[]patchEdit) error {
	if file.Kind != yaml.MappingNode || file.Style == yaml.FlowStyle {
		// 文件里这个键不是块状映射（标量/null/流式 {}）：整体替换由上层负责，这里不动
		return nil
	}

	missing := make([]*yaml.Node, 0, 4)
	for i := 0; i+1 < len(after.Content); i += 2 {
		afterKey, afterValue := after.Content[i], after.Content[i+1]
		fileKey, fileValue, found := mappingPair(file, afterKey.Value)
		if !found {
			// 结构体里有、文件里缺：整项（含注释与嵌套键）补写
			missing = append(missing, afterKey, afterValue)
			continue
		}

		beforeValue := mappingValue(before, afterKey.Value)
		if afterValue.Kind == yaml.MappingNode && fileValue.Kind == yaml.MappingNode {
			if fileValue.Style == yaml.FlowStyle {
				// 文件里这一节写成了流式 {…}：无法逐行补丁，退化为「只替换这一节」的块状渲染
				if err := appendReplacement(edits, fileKey, fileValue, afterValue, indent, depth); err != nil {
					return err
				}
				continue
			}
			if err := collectPatches(fileValue, afterValue, beforeValue, indent+"  ", depth+1, lines, edits); err != nil {
				return err
			}
			continue
		}
		if beforeValue != nil && sameNode(afterValue, beforeValue) {
			continue // 值没有变化
		}
		if err := appendReplacement(edits, fileKey, fileValue, afterValue, indent, depth); err != nil {
			return err
		}
	}

	if len(missing) > 0 {
		rendered, err := renderEntries(missing, indent)
		if err != nil {
			return err
		}
		anchor := maxNodeLine(file)
		if anchor == 0 {
			anchor = len(lines)
		}
		*edits = append(*edits, patchEdit{start: anchor + 1, end: anchor, depth: depth, lines: rendered})
	}
	return nil
}

// appendReplacement 生成「替换某一项」的补丁：保留文件里的键名与行内注释，
// 值按 after 节点重新渲染；行前注释（HeadComment）留在文件里不动，避免重复。
func appendReplacement(edits *[]patchEdit, fileKey, fileValue, afterValue *yaml.Node, indent string, depth int) error {
	keyCopy := *fileKey
	keyCopy.HeadComment, keyCopy.FootComment, keyCopy.LineComment = "", "", ""
	valueCopy := *afterValue
	if fileValue.LineComment != "" {
		valueCopy.LineComment = fileValue.LineComment
	} else if fileKey.LineComment != "" {
		valueCopy.LineComment = fileKey.LineComment
	}
	rendered, err := renderEntries([]*yaml.Node{&keyCopy, &valueCopy}, indent)
	if err != nil {
		return err
	}
	start := fileKey.Line
	end := maxNodeLine(fileValue)
	if end < start {
		end = start
	}
	*edits = append(*edits, patchEdit{start: start, end: end, depth: depth, lines: rendered})
	return nil
}

// renderEntries 把若干「键值对」（按 [key, value, ...] 顺序平铺）渲染成带注释的 YAML 行，
// 并统一加上 indent 缩进。
func renderEntries(pairs []*yaml.Node, indent string) ([]string, error) {
	mapping := &yaml.Node{Kind: yaml.MappingNode, Content: pairs}
	doc := &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{mapping}}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}

	text := strings.TrimRight(buf.String(), "\n")
	if text == "" {
		return nil, fmt.Errorf("渲染配置项失败")
	}
	rendered := strings.Split(text, "\n")
	for i, line := range rendered {
		if line != "" {
			rendered[i] = indent + line
		}
	}
	return rendered, nil
}

// applyPatches 依次应用改动。补丁按起始行倒序应用，因此每个补丁的行号在应用时都仍然有效。
func applyPatches(lines []string, edits []patchEdit) []string {
	sorted := append([]patchEdit(nil), edits...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].start != sorted[j].start {
			return sorted[i].start > sorted[j].start
		}
		// 同一行上既有内层插入又有外层追加：先应用外层，内层才会落在更靠上的位置
		return sorted[i].depth < sorted[j].depth
	})

	result := lines
	for _, edit := range sorted {
		start := edit.start - 1
		end := edit.end // 含端点 → 转成左闭右开
		if start < 0 {
			start = 0
		}
		if end > len(result) {
			end = len(result)
		}
		if start > end {
			start = end
		}
		replaced := make([]string, 0, len(result)-(end-start)+len(edit.lines))
		replaced = append(replaced, result[:start]...)
		replaced = append(replaced, edit.lines...)
		replaced = append(replaced, result[end:]...)
		result = replaced
	}
	return result
}

// documentMapping 取 YAML 文档节点里的根映射（非文档节点直接返回自身）。
func documentMapping(node *yaml.Node) *yaml.Node {
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		return node.Content[0]
	}
	return node
}

// mappingPair 在映射节点里按键名查键值对。
func mappingPair(mapping *yaml.Node, key string) (*yaml.Node, *yaml.Node, bool) {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil, nil, false
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i], mapping.Content[i+1], true
		}
	}
	return nil, nil, false
}

// mappingValue 在映射节点里按键名取值节点（缺失返回 nil）。
func mappingValue(mapping *yaml.Node, key string) *yaml.Node {
	_, value, _ := mappingPair(mapping, key)
	return value
}

// maxNodeLine 返回节点及其后代中最大的原始行号（0 表示节点没有行号信息）。
func maxNodeLine(node *yaml.Node) int {
	if node == nil {
		return 0
	}
	max := node.Line
	for _, child := range node.Content {
		if childMax := maxNodeLine(child); childMax > max {
			max = childMax
		}
	}
	return max
}

// sameNode 比较两个 YAML 节点在结构上是否等价（忽略注释）。
func sameNode(left, right *yaml.Node) bool {
	if left == nil || right == nil {
		return left == right
	}
	if left.Kind != right.Kind || left.Value != right.Value || len(left.Content) != len(right.Content) {
		return false
	}
	for i := range left.Content {
		if !sameNode(left.Content[i], right.Content[i]) {
			return false
		}
	}
	return true
}

// sameConfig 比较两份配置是否等价（nil 切片与空切片视为相同）。
func sameConfig(left, right Config) bool {
	left.Server.TrustedProxies = normalizeSlice(left.Server.TrustedProxies)
	right.Server.TrustedProxies = normalizeSlice(right.Server.TrustedProxies)
	return reflect.DeepEqual(left, right)
}

// normalizeSlice 把空切片归一化为 nil，便于比较。
func normalizeSlice(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	return values
}

// configLineEnding 识别文件使用的换行符（默认 LF），补丁按原样写回。
func configLineEnding(raw []byte) string {
	if bytes.Contains(raw, []byte("\r\n")) {
		return "\r\n"
	}
	return "\n"
}

// configLines 按行拆分配置内容，并报告原文件末尾是否有换行（补丁按原样写回）。
func configLines(raw []byte) ([]string, bool) {
	text := strings.ReplaceAll(string(raw), "\r\n", "\n")
	trailingNewline := strings.HasSuffix(text, "\n")
	text = strings.TrimRight(text, "\n")
	if text == "" {
		return nil, trailingNewline
	}
	return strings.Split(text, "\n"), trailingNewline
}
