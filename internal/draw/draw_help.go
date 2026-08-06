// 帮助中心图片：多列指令标签卡片 + 看板娘，列流式布局
// 对齐 Java DefaultDrawHelpImage
package draw

import "image/color"

// DrawHelp 绘制帮助中心图（commands 为空返回 nil）。
func DrawHelp(commands []string) []byte {
	if len(commands) == 0 {
		return nil
	}
	const (
		contentX      float64 = 50
		cols          int     = 4
		colGap        float64 = 20
		cardRad       float64 = 10
		cardPad       float64 = 16
		titleY        float64 = 80
		contentStartY float64 = 150
		cardW         float64 = 380
		cardH         float64 = 60
	)

	n := len(commands)
	isOdd := n%cols != 0
	canvasW := contentX + float64(cols)*cardW + float64(cols-1)*colGap + contentX

	colX := make([]float64, cols)
	for c := 0; c < cols; c++ {
		colX[c] = contentX + float64(c)*(cardW+colGap)
	}

	// 列末尾 Y（对齐 Java 逐列累加）
	colEndY := make([]float64, cols)
	for i := range colEndY {
		colEndY[i] = contentStartY
	}
	for i := 0; i < n; i++ {
		colEndY[i%cols] += cardH + colGap
	}
	for c := 0; c < cols; c++ {
		if colEndY[c] > contentStartY {
			colEndY[c] -= colGap
		}
	}

	var totalHeight float64
	if isOdd {
		tallerEnd := maxF(maxF(colEndY[0], colEndY[1]), colEndY[2])
		totalHeight = maxF(tallerEnd, colEndY[cols-1]+cardW)
	} else {
		maxEnd := contentStartY
		for c := 0; c < cols; c++ {
			maxEnd = maxF(maxEnd, colEndY[c])
		}
		totalHeight = maxEnd + 10 + cardW
	}

	canvas := NewCanvas(int(canvasW), int(totalHeight))
	canvas.SetColor(pageBackgroundColor).FillRect(0, 0, canvasW, totalHeight)
	canvas.DrawTooRoundRect()

	canvas.SetColor(titleColor).SetFontSize(44)
	canvas.AddCenteredText("帮助中心", titleY)
	canvas.SetColor(dividerColor).DrawLine(contentX, titleY+50, canvasW-contentX, titleY+50)

	drawY := make([]float64, cols)
	for i := range drawY {
		drawY[i] = contentStartY
	}
	dotColor := color.RGBA{0x86, 0x69, 0xA7, 0xff}
	for i, cmd := range commands {
		col := i % cols
		drawHelpTag(canvas, cmd, colX[col], drawY[col], cardW, cardH, cardRad, cardPad, dotColor)
		drawY[col] += cardH + colGap
	}

	standingX := colX[cols-1]
	standingY := totalHeight - cardW
	if isOdd {
		standingY = colEndY[cols-1]
	}
	canvas.DrawStandingAt(standingX, standingY, cardW, cardW)
	canvas.AddFooter(totalHeight - 25)
	data, err := canvas.PNG()
	if err != nil {
		return nil
	}
	return data
}

// drawHelpTag 绘制单个指令标签卡片（背景 + 描边 + 紫色圆点 + 指令文字）。
func drawHelpTag(canvas *Canvas, cmd string, x, y, w, h, radius, pad float64, dot color.RGBA) {
	canvas.SetColor(cardBackgroundColor).FillRoundRect(x, y, w, h, radius)
	canvas.SetColor(dividerColor).SetStroke(1).
		DrawRoundRect(x, y, w, h, radius)

	// 左侧紫色圆点
	canvas.SetColor(dot).FillOval(x+pad, y+h/2-5, 10, 10)

	// 指令文字
	canvas.SetColor(textColor).SetFontSize(22)
	canvas.AddText(cmd, x+pad+20, y+h/2+8)
}

// maxF / minF float64 比较辅助。
func maxF(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func minF(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
