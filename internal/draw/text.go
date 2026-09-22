// 文本工具：自动换行与多行处理（对齐 Java TextUtils.wrapText）
package draw

import (
	"strings"

	"github.com/fogleman/gg"
)

// textWrapLineCount 按指定字号与最大宽度计算文本环绕后的行数（不占用画布槽位，用于预估卡片高度）。
// 对齐 Java FontMetrics 语义，逐字符累积直到超宽回退。
func textWrapLineCount(text string, size, maxWidth float64) int {
	if maxWidth <= 0 || text == "" {
		return 0
	}
	lines := 1
	var cur strings.Builder
	for _, ch := range text {
		cur.WriteRune(ch)
		w := measureWidth(size, cur.String())
		if w <= maxWidth {
			continue
		}
		if cur.Len() <= len(string(ch)) {
			lines++
			cur.Reset()
			cur.WriteRune(ch)
			continue
		}
		// 超宽：当前行结算，新行以当前字符开头
		// 对上 wrapText 逐字符语义：先记录上一字符非空则换行
		lines++
		cur.Reset()
		cur.WriteRune(ch)
	}
	return lines
}

// measureWidth 使用指定字号 face 度量文本宽度（对齐 Canvas.StringWidth，不占画布槽位）。
func measureWidth(size float64, text string) float64 {
	c := measureCanvas(size)
	return c.StringWidth(text)
}

// measureCanvas 返回用于测宽的临时画布（直接构造 gg.Context，不注册到全局信号量）。
func measureCanvas(size float64) *Canvas {
	dc := gg.NewContext(1, 1)
	dc.SetFontFace(textFace(size))
	return &Canvas{dc: dc, width: 1, height: 1, face: textFace(size), size: size}
}

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
