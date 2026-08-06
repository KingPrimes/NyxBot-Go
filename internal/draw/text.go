// 文本工具：自动换行与多行处理（对齐 Java TextUtils.wrapText）
package draw

import "strings"

// splitLines 按换行符分割文本。
func splitLines(text string) []string {
	return strings.Split(text, "\n")
}

// wrapText 按最大宽度自动换行（逐字符累积，超宽回退，对齐 Java TextUtils.wrapText）。
func wrapText(c *Canvas, text string, maxWidth float64) []string {
	if maxWidth <= 0 {
		return []string{text}
	}
	lines := make([]string, 0, 4)
	current := &strings.Builder{}
	for _, ch := range text {
		prevLen := current.Len()
		current.WriteRune(ch)
		if c.StringWidth(current.String()) <= maxWidth {
			continue
		}
		// 超宽：回退上一字符，若当前行非空则结算，否则单字符独占一行
		if prevLen == 0 {
			lines = append(lines, string(ch))
			current.Reset()
			continue
		}
		lines = append(lines, current.String()[:prevLen])
		current.Reset()
		current.WriteRune(ch)
	}
	if current.Len() > 0 {
		lines = append(lines, current.String())
	}
	return lines
}
