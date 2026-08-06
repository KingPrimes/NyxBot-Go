// 紫卡分析趋势图：两列卡片 + 右侧看板娘，卡片高度自适应属性数量
// 对齐 Java DefaultDrawRivenAnalyseTrendImage
package draw

import (
	"fmt"
	"image/color"
	"strings"

	"nyxbot-go/internal/enum/drawplugin"
)

// 紫卡趋势图布局常量（对齐 Java DefaultDrawRivenAnalyseTrendImage 各静态字段）。
const (
	rivenCanvasW       float64 = 1750 // 画布宽度
	rivenContentX      float64 = 50   // 内容左边距
	rivenCols          int     = 2    // 卡片列数
	rivenColGap        float64 = 50   // 列间距
	rivenCardPad       float64 = 24   // 卡片内边距
	rivenCardRad       float64 = 14   // 卡片圆角
	rivenTitleY        float64 = 80   // 标题基线 Y
	rivenContentStartY float64 = 150  // 内容区起始 Y
)

// 趋势属性颜色（对齐 Java UP_COLOR / DOWN_COLOR）。
var (
	trendUpColor   = color.RGBA{0x3b, 0x9a, 0x21, 0xff} // 属性上升 — 绿
	trendDownColor = color.RGBA{0xac, 0x18, 0x18, 0xff} // 属性下降 — 红
	// lethalColor 对齐 Java LETHAL_COLOR（Java 中已定义但未使用，保留以备扩展）。
	lethalColor = color.RGBA{0xcc, 0x66, 0x00, 0xff}
)

// RivenAnalyseTrend 紫卡分析趋势绘图输入（对齐 Java model.RivenAnalyseTrendModel）。
type RivenAnalyseTrend struct {
	WeaponName string                       // 武器名称
	RivenName  string                       // 紫卡名称
	Dot        string                       // 倾向点文本（Num 为空时回退）
	Num        *float64                     // 倾向数值（非空时按五档换算倾向点）
	WeaponType string                       // 武器类型
	Attributes []RivenAnalyseTrendAttribute // 属性分析
}

// RivenAnalyseTrendAttribute 紫卡属性分析行（对齐 Java RivenAnalyseTrendModel.Attribute）。
type RivenAnalyseTrendAttribute struct {
	Name          string   // 属性名称
	AttributeName string   // 属性全称
	Attr          *float64 // 属性数值
	LowAttr       string   // 低属性数值
	HighAttr      string   // 高属性数值
	AttrDiff      string   // 属性数值差异
	Ratio         string   // 比率（紫卡值与该武器类型现有 MOD 的比值）
	Grade         string   // 评分 S/A/B/C/D
	LethalLevel   string   // 致命度（仅负属性有值: fatal/serious/harmful/beneficial）
	Analysis      string   // 综合分析文本
}

// dotText 倾向点文本：Num 非空按五档数值换算，否则回退 Dot，再回退 1 点（对齐 Java getDot）。
func (r *RivenAnalyseTrend) dotText() string {
	if r.Num != nil {
		return rivenTrendDot(*r.Num)
	}
	if r.Dot != "" {
		return r.Dot
	}
	return drawplugin.RivenTrend1.Doc()
}

// rivenTrendDot 将倾向数值换算为 1-5 点倾向文本（对齐 Java RivenTrendEnum.getRivenTrendDot）。
func rivenTrendDot(num float64) string {
	switch {
	case num < 0.7:
		return drawplugin.RivenTrend1.Doc()
	case num < 0.9:
		return drawplugin.RivenTrend2.Doc()
	case num < 1.15:
		return drawplugin.RivenTrend3.Doc()
	case num < 1.3:
		return drawplugin.RivenTrend4.Doc()
	default:
		return drawplugin.RivenTrend5.Doc()
	}
}

// lethalLabel 致命度等级转中文标签（对齐 Java lethalLabel 映射）。
func lethalLabel(level string) string {
	switch level {
	case "fatal":
		return "⚡致命"
	case "serious":
		return "⚠严重"
	case "harmful":
		return "△有害"
	case "acceptable":
		return "可接受"
	case "beneficial":
		return "✓有益"
	default:
		return level
	}
}

// DrawRivenAnalyseTrend 绘制紫卡分析趋势图（models 为空返回 nil）。
func DrawRivenAnalyseTrend(models []*RivenAnalyseTrend) []byte {
	if len(models) == 0 {
		return nil
	}
	// 过滤空指针（正常输入不含，防御处理）
	valid := make([]*RivenAnalyseTrend, 0, len(models))
	for _, m := range models {
		if m != nil {
			valid = append(valid, m)
		}
	}
	if len(valid) == 0 {
		return nil
	}
	models = valid

	n := len(models)
	isOdd := n%rivenCols != 0

	szW, szH := scaleByPct(rivenCanvasW, rivenCanvasW, standardRatio)
	cardsContentW := rivenCanvasW - rivenContentX - float64(szW) - 30
	cardW := (cardsContentW - rivenColGap*float64(rivenCols-1)) / float64(rivenCols)
	textW := cardW - rivenCardPad*2

	// 预计算卡片高度
	cardHeights := make([]float64, n)
	for i, m := range models {
		cardHeights[i] = calcRivenTrendCardHeight(m)
	}

	// 行高度（同列卡片取最大）
	rows := (n + rivenCols - 1) / rivenCols
	rowHeights := make([]float64, rows)
	cardsH := 0.0
	for r := 0; r < rows; r++ {
		maxH := 0.0
		for c := 0; c < rivenCols && r*rivenCols+c < n; c++ {
			maxH = maxF(maxH, cardHeights[r*rivenCols+c])
		}
		rowHeights[r] = maxH
		cardsH += maxH
		if r < rows-1 {
			cardsH += rivenColGap
		}
	}

	// 最后一行的起始 Y（奇数行时看板娘与最后一行的卡片对齐）
	lastRowY := rivenContentStartY
	for r := 0; r < rows-1; r++ {
		lastRowY += rowHeights[r] + rivenColGap
	}

	standingX := rivenCanvasW - float64(szW)
	var standingY float64
	if isOdd {
		standingY = lastRowY
	} else {
		standingY = rivenContentStartY + cardsH + 10
	}
	canvasH := standingY + float64(szH)

	colX := [2]float64{rivenContentX, rivenContentX + cardW + rivenColGap}

	canvas := NewCanvas(int(rivenCanvasW), int(canvasH))
	canvas.SetColor(pageBackgroundColor).FillRect(0, 0, rivenCanvasW, canvasH)
	canvas.DrawTooRoundRect()

	// 标题
	canvas.SetColor(titleColor).SetFontSize(44)
	canvas.AddCenteredText("紫卡分析趋势", rivenTitleY)
	canvas.SetColor(dividerColor).DrawLine(rivenContentX, rivenTitleY+50,
		rivenContentX+cardsContentW+float64(szW)+30, rivenTitleY+50)

	currentY := rivenContentStartY
	for i, m := range models {
		row := i / rivenCols
		col := i % rivenCols
		drawRivenTrendCard(canvas, m, colX[col], currentY, cardW, cardHeights[i], textW)
		if col == rivenCols-1 || i == n-1 {
			currentY += rowHeights[row] + rivenColGap
		}
	}

	canvas.DrawStandingAt(standingX, standingY, float64(szW), float64(szH))
	canvas.AddFooter(canvasH - 25)
	data, err := canvas.PNG()
	if err != nil {
		return nil
	}
	return data
}

// calcRivenTrendCardHeight 预计算单张趋势卡片的自适应高度（对齐 Java calcCardHeight）。
func calcRivenTrendCardHeight(m *RivenAnalyseTrend) float64 {
	h := rivenCardPad // 顶部内边距
	h += 34           // 武器名称
	h += 28           // 紫卡名称
	h += 16           // 间距 + 分隔线
	h += 30           // 倾向 + 数值
	h += 26           // 武器类型
	h += 16           // 间距 + 分隔线
	if m.Attributes != nil {
		h += float64(len(m.Attributes)) * 52 // 主行 32 + 副行 20
	}
	h += rivenCardPad
	return maxF(h, 200)
}

// drawRivenTrendCard 绘制单张趋势卡片（对齐 Java drawCard 各行布局与配色）。
func drawRivenTrendCard(canvas *Canvas, m *RivenAnalyseTrend, cardX, cardY, cardW, cardH, textW float64) {
	innerX := cardX + rivenCardPad
	rightX := innerX + textW

	canvas.SetColor(cardBackgroundColor).FillRoundRect(cardX, cardY, cardW, cardH, rivenCardRad)
	canvas.SetColor(dividerColor).SetStroke(1).
		DrawRoundRect(cardX, cardY, cardW, cardH, rivenCardRad)

	cy := cardY + rivenCardPad

	// 武器名称
	weapon := m.WeaponName
	if weapon == "" {
		weapon = "未知"
	}
	canvas.SetColor(titleColor).SetFontSize(24)
	canvas.AddText("武器: "+weapon, innerX, cy+26)
	cy += 34

	// 紫卡名称
	riven := m.RivenName
	if riven == "" {
		riven = "未知"
	}
	canvas.SetColor(textColor).SetFontSize(20)
	canvas.AddText("紫卡: "+riven, innerX, cy+22)
	cy += 30

	// 分隔线
	canvas.SetColor(dividerColor).DrawLine(innerX, cy+6, rightX, cy+6)
	cy += 14

	// 倾向 + 数值
	numStr := "-"
	if m.Num != nil {
		numStr = fmt.Sprintf("%.2f", *m.Num)
	}
	canvas.SetColor(accentGoldColor).SetFontSize(20)
	canvas.AddText("倾向 "+m.dotText()+"  "+numStr, innerX, cy+20)
	cy += 30

	// 武器类型
	typeStr := m.WeaponType
	if typeStr == "" {
		typeStr = "未知"
	}
	canvas.SetColor(textSecondaryColor).SetFontSize(20)
	canvas.AddText("类型 "+typeStr, innerX, cy+20)
	cy += 28

	// 分隔线
	canvas.SetColor(dividerColor).DrawLine(innerX, cy+6, rightX, cy+6)
	cy += 16

	// 属性列表（主行: 名称 (低%-高%) diff grade；副行: 比率 + 分析 + 致命度）
	for _, attr := range m.Attributes {
		diff := attr.AttrDiff
		isUp := !strings.Contains(diff, "-")
		attrColor := trendDownColor
		if isUp {
			attrColor = trendUpColor
		}

		name := attr.AttributeName
		if name == "" {
			name = "?"
		}
		low := attr.LowAttr
		if low == "" {
			low = "?"
		}
		high := attr.HighAttr
		if high == "" {
			high = "?"
		}
		grade := ""
		if attr.Grade != "" && attr.Grade != "-" {
			grade = " [" + attr.Grade + "]"
		}
		line := name + " (" + low + "%-" + high + "%)" + "    " + diff + grade

		canvas.SetColor(attrColor).SetFontSize(20)
		canvas.AddText(line, innerX, cy+20)
		cy += 32

		// 副行：比率 + 综合分析
		ratio := ""
		if attr.Ratio != "" && attr.Ratio != "-" {
			ratio = "比率 " + attr.Ratio
		}
		lethal := ""
		if attr.LethalLevel != "" {
			lethal = " " + lethalLabel(attr.LethalLevel)
		}
		subLine := strings.TrimSpace(ratio + "  " + attr.Analysis + lethal)
		if subLine != "" {
			canvas.SetColor(textSecondaryColor).SetFontSize(16)
			canvas.AddText(subLine, innerX+8, cy+18)
			cy += 20
		}
	}
}
