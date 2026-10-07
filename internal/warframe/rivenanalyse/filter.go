package rivenanalyse

import (
	"regexp"
	"strings"
)

// 噪音过滤正则（对齐 Java RivenOcrFilter）。
var (
	// reNoise 纯噪音行：纯数字 / 段位 / 短字母与标点 / ×数字。
	// 对齐 NOISE = ^(\d{1,3}([\s-]\d{1,3})?|段位\d*|[A-Za-z]?[\p{Punct}]*[A-Za-z]?|×\d+)$
	// （Go 无 \p{Punct}，显式列出 ASCII 标点类以对齐 Java 语义）。
	reNoise = regexp.MustCompile(`^(\d{1,3}([\s-]\d{1,3})?|段位\d*|[A-Za-z]?[!"#$%&'()*+,\-./:;<=>?@\[\]^_` + "`" + `{|}~]*[A-Za-z]?|×\d+)$`)
	// reIconArtifact 元素图标被 OCR 误识别的残留符号。
	reIconArtifact = regexp.MustCompile(`[*＞><∣|★☆●○◆◇▲△▼▽♢♤♧♡]`)
	// reStrayParen 残留在数值与中文之间的半角括号。
	// 对齐 Java (?<=[%\d])\s*[（(](?=[一-龥])（Go 无 lookaround，改用捕获组保留数值与中文）。
	reStrayParen = regexp.MustCompile(`([%\d])\s*[（(]([` + hanRange + `])`)
	// reTrailingPunct OCR 常将相邻标点吸入文本行尾部（如 "fevacron,"、"段位14."）。
	reTrailingPunct = regexp.MustCompile(`[,\.;:，。；：]+$`)
)

// cleanLines 过滤 OCR 噪音并做断行合并兜底（对齐 RivenOcrFilter.clean，按需简化）。
//
// PP-OCRv6 实测输出多为「数值+中文」合并行，Java 针对老 OCR 拆行的 FRAGMENT 追加重合逻辑
// 不再需要；保留两类兜底：① 相邻的「纯中文属性名行 + 纯数值行」互相拼接；
// ② 过滤后仍不成形的碎片行按原样保留，交由解析阶段判定丢弃。
func cleanLines(raw []string) []string {
	filtered := make([]string, 0, len(raw))
	for _, line := range raw {
		s := strings.TrimSpace(line)
		if len([]rune(s)) < 2 {
			continue
		}
		if reNoise.MatchString(s) {
			continue
		}
		s = reIconArtifact.ReplaceAllString(s, "")
		s = reStrayParen.ReplaceAllString(s, "$1$2")
		s = reTrailingPunct.ReplaceAllString(s, "")
		s = strings.TrimSpace(s)
		if len([]rune(s)) < 2 {
			continue
		}
		filtered = append(filtered, s)
	}
	return mergeSplitAttributeLines(filtered)
}

// mergeSplitAttributeLines 合并相邻的「属性名行 + 数值行」（v6 输出万一被拆行时的兜底）。
// 形态一：「装填速度」+「+6.5%」→「+6.5%装填速度」；形态二：「+6.5%」+「装填速度」→ 同上。
func mergeSplitAttributeLines(lines []string) []string {
	out := make([]string, 0, len(lines))
	for i := 0; i < len(lines); i++ {
		cur := lines[i]
		if i+1 < len(lines) {
			next := lines[i+1]
			if isAttrNameOnly(cur) && isPureNumberLine(next) {
				out = append(out, next+cur)
				i++
				continue
			}
			if isPureNumberLine(cur) && isAttrNameOnly(next) {
				out = append(out, cur+next)
				i++
				continue
			}
		}
		out = append(out, cur)
	}
	return out
}

// isAttrNameOnly 整行是纯中文（无数字），疑似被拆出的属性名行。
func isAttrNameOnly(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < 0x4e00 || r > 0x9fa5 {
			return false
		}
	}
	return true
}
