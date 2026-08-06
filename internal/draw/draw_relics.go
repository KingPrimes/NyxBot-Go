// 遗物卡片图：三列卡片网格（遗物名 + 按稀有度着色的奖励列表）+ 看板娘，列流式布局
// 对齐 Java DefaultDrawRelicsImage
package draw

import (
	"fmt"
	"image/color"

	"nyxbot-go/internal/enum/drawplugin"
)

// Relics 遗物查询结果（对齐 Java model.Relics 原始字段）。
type Relics struct {
	Name    string         `json:"name"`    // 遗物名称
	Rewards []*RelicReward `json:"rewards"` // 遗物奖励列表
}

// RelicReward 遗物奖励条目（对齐 Java Relics.Rewards 内部类，展开为独立结构）。
type RelicReward struct {
	Name      string            `json:"name"`      // 奖励名称
	Rarity    drawplugin.Rarity `json:"rarity"`    // 奖励稀有度
	ItemCount *int              `json:"itemCount"` // 奖励数量
}

// displayName 奖励显示名（数量大于 1 时加 "NX" 前缀，对齐 Java Rewards.getName 覆写）。
func (r *RelicReward) displayName() string {
	if r.ItemCount != nil && *r.ItemCount > 1 {
		return fmt.Sprintf("%dX%s", *r.ItemCount, r.Name)
	}
	return r.Name
}

// DrawRelics 绘制遗物查询结果图（空输入返回 nil）。
func DrawRelics(list []*Relics) []byte {
	if len(list) == 0 {
		return nil
	}
	const (
		contentX      float64 = 50  // 内容区左边距
		cols          int     = 3   // 列数
		colGap        float64 = 20  // 列间距
		cardRad       float64 = 14  // 卡片圆角
		cardPad       float64 = 20  // 卡片内边距
		titleY        float64 = 80  // 标题基线 Y
		contentStartY float64 = 150 // 内容区起始 Y
		cardW         float64 = 562 // 卡片宽度
	)
	textW := cardW - cardPad*2
	canvasW := contentX + float64(cols)*cardW + float64(cols-1)*colGap + contentX

	n := len(list)
	isOdd := n%cols != 0

	// 列 X 起点（对齐 Java colX 数组）
	colX := make([]float64, cols)
	for c := 0; c < cols; c++ {
		colX[c] = contentX + float64(c)*(cardW+colGap)
	}

	// 预计算卡片高度（顶部 + 标题 + 奖励行数）
	cardHeights := make([]float64, n)
	for i, r := range list {
		cardHeights[i] = relicCardHeight(r, cardPad)
	}

	// 列流式 Y 终点（对齐 Java 逐列累加后回退一个列间距）
	colEndY := make([]float64, cols)
	for c := range colEndY {
		colEndY[c] = contentStartY
	}
	for i := 0; i < n; i++ {
		colEndY[i%cols] += cardHeights[i] + colGap
	}
	for c := 0; c < cols; c++ {
		if colEndY[c] > contentStartY {
			colEndY[c] -= colGap
		}
	}

	// 画布总高：看板娘占用右列底部区域（奇数时与左中列共享垂直空间）
	var totalHeight float64
	if isOdd {
		tallerEnd := maxF(colEndY[0], colEndY[1])
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

	// 标题
	canvas.SetColor(titleColor).SetFontSize(44)
	canvas.AddCenteredText("遗物查询结果", titleY)
	canvas.SetColor(dividerColor).DrawLine(contentX, titleY+50, canvasW-contentX, titleY+50)

	// 列流式绘制卡片
	drawY := make([]float64, cols)
	for c := range drawY {
		drawY[c] = contentStartY
	}
	for i, r := range list {
		col := i % cols
		drawRelicCard(canvas, r, colX[col], drawY[col], cardW, cardHeights[i], textW, cardRad, cardPad)
		drawY[col] += cardHeights[i] + colGap
	}

	// 看板娘 + 底部署名
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

// relicCardHeight 计算遗物卡片高度（对齐 Java calcCardHeight：顶部 + 标题 + 奖励行数，下限 140）。
func relicCardHeight(r *Relics, pad float64) float64 {
	h := pad + 42 + 10 + pad
	if r != nil && len(r.Rewards) > 0 {
		h += float64(len(r.Rewards)) * 36
	}
	return maxF(h, 140)
}

// drawRelicCard 绘制单张遗物卡片（对齐 Java drawCard 名称 + 奖励列表布局）。
func drawRelicCard(canvas *Canvas, r *Relics, cardX, cardY, cardW, cardH, textW, rad, pad float64) {
	innerX := cardX + pad
	rightX := innerX + textW

	// 卡片背景 + 边框
	canvas.SetColor(cardBackgroundColor).FillRoundRect(cardX, cardY, cardW, cardH, rad)
	canvas.SetColor(dividerColor).SetStroke(1).
		DrawRoundRect(cardX, cardY, cardW, cardH, rad)

	cy := cardY + pad

	// 遗物名称
	name := r.Name
	if name == "" {
		name = "未知遗物"
	}
	canvas.SetColor(titleColor).SetFontSize(22)
	canvas.AddText(name, innerX, cy+24)
	cy += 36

	// 分隔线
	canvas.SetColor(dividerColor).DrawLine(innerX, cy+4, rightX, cy+4)
	cy += 14

	// 奖励列表（稀有度着色）
	for _, reward := range r.Rewards {
		rname := reward.displayName()
		if rname == "" {
			rname = "未知"
		}
		// 超长截断（按 rune 截断，对齐 Java substring 语义）
		if runes := []rune(rname); len(runes) > 26 {
			rname = string(runes[:24]) + ".."
		}
		canvas.SetColor(rarityColor(reward.Rarity)).SetFontSize(18)
		canvas.AddText("● "+rname, innerX+8, cy+22)
		cy += 36
	}
}

// rarityColor 奖励稀有度颜色（对齐 Java getRarityColor：常见/罕见/稀有 → 遗物等级色，其余主文本色）。
func rarityColor(r drawplugin.Rarity) color.RGBA {
	switch r {
	case drawplugin.RarityCommon:
		return voidT2Color
	case drawplugin.RarityUncommon:
		return voidT3Color
	case drawplugin.RarityRare:
		return voidT4Color
	default:
		return textColor
	}
}
