// 电波赛季图片：两列挑战卡片 + 右下看板娘，卡片高度自适应内容
// 对齐 Java DefaultDrawSeasonInfoImage / model.SeasonInfo
package draw

import (
	"image/color"
	"strconv"
)

// 电波挑战卡片常量（对齐 Java DefaultDrawSeasonInfoImage）。
const (
	ticketCardPad     = 20 // 卡片内边距
	ticketCardRadius  = 14 // 卡片圆角
	ticketRowH        = 40 // 行高
	ticketRowHeight   = 40
	ticketCardMinH    = 200
	ticketDescMaxSize = 22 // 描述字号
)

// ActiveChallenge 电波活跃挑战（对齐 Java SeasonInfo.ActiveChallenges）。
type ActiveChallenge struct {
	Description string // 描述
	Name        string // 任务名称
	Standing    string // 声望值文本
	Daily       bool   // 每日
	Weekly      bool   // 每周
	Elite       bool   // 精英
}

// SeasonInfo 电波赛季绘图输入（对齐 Java model.SeasonInfo）。
type SeasonInfo struct {
	Season           int                // 赛季
	Phase            int                // 阶段
	ActiveChallenges []*ActiveChallenge // 活跃挑战
}

// DrawSeasonInfo 绘制电波赛季图（两列挑战卡片网格 + 右下看板娘）。
func DrawSeasonInfo(info *SeasonInfo) []byte {
	if info == nil || len(info.ActiveChallenges) == 0 {
		return nil
	}
	const (
		canvasW  = 1800
		contentX = 50
		cols     = 2
		colGap   = 20
		rowGap   = 20
		titleY   = 80
	)
	challenges := info.ActiveChallenges
	n := len(challenges)

	szW, szHBox := scaleByPct(canvasW, canvasW, standardRatio)
	cardsContentW := canvasW - contentX*2
	cardW := (cardsContentW - colGap*(cols-1)) / cols
	descMaxW := cardW - ticketCardPad*2

	cardHeights := make([]int, n)
	for i, c := range challenges {
		cardHeights[i] = calcChallengeCardHeight(c.Description, descMaxW)
	}

	rows := (n + cols - 1) / cols
	rowHeights := make([]int, rows)
	cardsH := 0
	for r := 0; r < rows; r++ {
		maxH := 0
		for c := 0; c < cols && r*cols+c < n; c++ {
			maxH = maxI(maxH, cardHeights[r*cols+c])
		}
		rowHeights[r] = maxH
		cardsH += maxH
		if r < rows-1 {
			cardsH += rowGap
		}
	}

	headerH := 120
	contentStartY := titleY + headerH
	standingX := canvasW - szW
	standingY := contentStartY + cardsH + 10
	canvasH := standingY + szHBox

	colX := make([]int, cols)
	for c := 0; c < cols; c++ {
		colX[c] = contentX + c*(cardW+colGap)
	}

	canvas := NewCanvas(canvasW, canvasH)
	canvas.SetColor(pageBackgroundColor).FillRect(0, 0, float64(canvasW), float64(canvasH))
	canvas.DrawTooRoundRect()

	// 标题（全画布居中）
	canvas.SetColor(titleColor).SetFontSize(44)
	canvas.AddCenteredText("电波赛季信息", titleY)
	// 分隔线（横跨内容宽度）
	canvas.SetColor(dividerColor).DrawLine(contentX, titleY+50, canvasW-contentX, titleY+50)
	// 赛季阶段信息
	canvas.SetColor(textSecondaryColor).SetFontSize(20)
	canvas.AddText("赛季 "+strconv.Itoa(info.Season)+"  |  阶段 "+strconv.Itoa(info.Phase), contentX, float64(titleY+85))

	currentY := contentStartY
	for i, c := range challenges {
		row := i / cols
		col := i % cols
		cardH := rowHeights[row]
		drawChallengeCard(canvas, c, colX[col], currentY, cardW, cardH)
		if col == cols-1 || i == n-1 {
			currentY += cardH + rowGap
		}
	}

	canvas.DrawStandingAt(float64(standingX), float64(standingY), float64(szW), float64(szHBox))
	canvas.AddFooter(float64(canvasH) - 25)
	data, err := canvas.PNG()
	if err != nil {
		return nil
	}
	return data
}

// calcChallengeCardHeight 计算单张挑战卡高度（对齐 Java calcCardHeight，基于描述换行行数）。
func calcChallengeCardHeight(desc string, descMaxW int) int {
	h := ticketCardPad
	h += ticketRowH // 类型标签
	h += ticketRowH // 名称
	if desc != "" {
		h += challengeDescLines(desc, descMaxW) * 30
	} else {
		h += ticketRowH
	}
	h += ticketRowH    // 声望
	h += ticketCardPad // 底部内边距
	return maxI(h, ticketCardMinH)
}

// challengeDescLines 描述按宽度环绕后的行数。
func challengeDescLines(desc string, maxWidth int) int {
	if desc == "" || maxWidth <= 0 {
		return 0
	}
	return textWrapLineCount(desc, ticketDescMaxSize, float64(maxWidth))
}

// drawChallengeCard 绘制单张挑战卡片。
func drawChallengeCard(canvas *Canvas, c *ActiveChallenge, cardX, cardY, cardW, cardH int) {
	innerX := cardX + ticketCardPad
	innerW := cardW - ticketCardPad*2
	accent := challengeTypeColor(c)

	canvas.SetColor(cardBackgroundColor).FillRoundRect(float64(cardX), float64(cardY), float64(cardW), float64(cardH), ticketCardRadius)
	canvas.SetColor(accent).FillRect(float64(cardX+ticketCardRadius), float64(cardY+2), float64(cardW-2*ticketCardRadius), 5)

	cy := cardY + ticketCardPad
	// 类型标签
	canvas.SetColor(accent).SetFontSize(22)
	canvas.AddText(challengeTypeLabel(c), float64(innerX), float64(cy+28))
	cy += ticketRowH
	// 名称
	name := c.Name
	if name == "" {
		name = "未知任务"
	}
	canvas.SetColor(textColor).SetFontSize(24)
	canvas.AddText(name, float64(innerX), float64(cy+28))
	cy += ticketRowH
	// 描述（自动换行）
	if c.Description != "" {
		canvas.SetColor(textSecondaryColor).SetFontSize(ticketDescMaxSize)
		for _, line := range wrapText(canvas, c.Description, float64(innerW)) {
			canvas.AddText(line, float64(innerX), float64(cy+24))
			cy += 30
		}
	}
	// 声望（卡片右下）
	standing := "声望: " + c.Standing
	if c.Standing == "" {
		standing = "声望: 0"
	}
	canvas.SetColor(accentGoldColor).SetFontSize(22)
	canvas.AddText(standing, float64(innerX+innerW-int(canvas.StringWidth(standing))), float64(cardY+cardH-ticketCardPad-5))
}

// challengeTypeLabel 挑战类型标签（对齐 Java getTypeLabel）。
func challengeTypeLabel(c *ActiveChallenge) string {
	if c.Elite {
		return "精英挑战"
	}
	if c.Weekly {
		return "每周挑战"
	}
	if c.Daily {
		return "每日挑战"
	}
	return "普通挑战"
}

// challengeTypeColor 挑战类型颜色（对齐 Java getTypeColor）。
func challengeTypeColor(c *ActiveChallenge) color.RGBA {
	if c.Elite {
		return color.RGBA{0x9B, 0x59, 0xB6, 0xff} // 精英 - 紫
	}
	if c.Weekly {
		return titleColor // 每周 - 蓝
	}
	if c.Daily {
		return color.RGBA{0xFF, 0x95, 0x00, 0xff} // 每日 - 橙
	}
	return textSecondaryColor // 普通 - 浅灰
}
