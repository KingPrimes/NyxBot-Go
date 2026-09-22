// 杜卡德币（金/银垃圾）图片：4 列卡片网格（左两列「当天」、右两列「每小时」）+ 看板娘
// 对齐 Java DefaultDrawMarketDucatsImage
package draw

import (
	"fmt"
	"image/color"
)

// 杜卡币图布局常量（对齐 DefaultDrawMarketDucatsImage 私有常量）。
const (
	ducatsContentX      float64 = 50  // 内容起始 X
	ducatsCols          int     = 4   // 卡片列数（当天 2 列 + 每小时 2 列）
	ducatsColGap        float64 = 20  // 列间距
	ducatsCardRadius    float64 = 14  // 卡片圆角
	ducatsCardPad       float64 = 20  // 卡片内边距
	ducatsTitleY        float64 = 80  // 标题 Y
	ducatsContentStartY float64 = 150 // 内容起始 Y
	ducatsCardW         float64 = 400 // 卡片宽度
	ducatsCardH         float64 = 180 // 卡片高度
	ducatsNameMaxRunes  int     = 22  // 物品名最长字符数（超出截断）
)

// 杜卡币图局部颜色（对齐 Java 私有常量）。
var (
	ducatsDayLabelColor  = color.RGBA{0x6c, 0x5c, 0xe7, 0xff} // 「当天」标签 — 紫
	ducatsHourLabelColor = color.RGBA{0xe6, 0x7e, 0x22, 0xff} // 「每小时」标签 — 橙
)

// Ducats 杜卡币绘图输入（对齐 Java Ducats.Ducat 的展示字段）。
// 当天与每小时各一份列表，分别占用前两列与后两列。
type Ducats struct {
	Day  []*DucatsEntry // 「当天」榜单
	Hour []*DucatsEntry // 「每小时」榜单
}

// DucatsEntry 单条杜卡币排行（对齐 Java Ducats.Ducat 展开的展示字段）。
type DucatsEntry struct {
	Item                string   // 物品名
	Ducats              *int     // 杜卡币数量
	WaPrice             *float64 // 均价
	Volume              *int     // 库存
	DucatsPerPlatinumWa *float64 // 实时「1 白金 = ? 杜卡币」
}

// DrawMarketDucats 绘制金/银垃圾图（空输入返回 nil）。
// title 为图标题（如「金垃圾」/「银垃圾」）。
func DrawMarketDucats(dump *Ducats, title string) []byte {
	if dump == nil || (len(dump.Day) == 0 && len(dump.Hour) == 0) {
		return nil
	}

	textW := ducatsCardW - ducatsCardPad*2
	canvasW := ducatsContentX + float64(ducatsCols)*ducatsCardW +
		float64(ducatsCols-1)*ducatsColGap + ducatsColGap + ducatsCardW + ducatsContentX

	colX := make([]float64, ducatsCols)
	for c := 0; c < ducatsCols; c++ {
		colX[c] = ducatsContentX + float64(c)*(ducatsCardW+ducatsColGap)
	}

	dayEndY := ducatsColumnEnd(len(dump.Day))
	hourEndY := ducatsColumnEnd(len(dump.Hour))
	totalHeight := maxF(maxF(dayEndY, hourEndY), ducatsCardW) + 60

	canvas := NewCanvas(int(canvasW), int(totalHeight))
	canvas.SetColor(pageBackgroundColor).FillRect(0, 0, canvasW, totalHeight)
	canvas.DrawTooRoundRect()
	canvas.SetColor(titleColor).SetFontSize(44)
	canvas.AddCenteredText(title, ducatsTitleY)
	canvas.SetColor(dividerColor).DrawLine(ducatsContentX, ducatsTitleY+50, canvasW-ducatsContentX, ducatsTitleY+50)

	// 当天：col0/col1 交替；每小时：col2/col3 交替（对齐 Java dayY / hourY 数组）
	drawDucatsCards(canvas, dump.Day, "当天", colX[:2], textW, ducatsDayLabelColor)
	drawDucatsCards(canvas, dump.Hour, "每小时", colX[2:4], textW, ducatsHourLabelColor)

	// 看板娘固定在右侧独立列，底部对齐
	standingX := canvasW - ducatsContentX - ducatsCardW
	canvas.DrawStandingAt(standingX, totalHeight-ducatsCardW, ducatsCardW, ducatsCardW)
	canvas.AddFooter(totalHeight - 25)

	data, err := canvas.PNG()
	if err != nil {
		return nil
	}
	return data
}

// drawDucatsCards 按列流式绘制一组卡片（i%2 决定列，逐张累加 Y）。
func drawDucatsCards(canvas *Canvas, entries []*DucatsEntry, label string, groupX []float64, textW float64, labelColor color.RGBA) {
	if len(entries) == 0 || len(groupX) == 0 {
		return
	}
	drawY := make([]float64, len(groupX))
	for i := range drawY {
		drawY[i] = ducatsContentStartY
	}
	for i, entry := range entries {
		if entry == nil {
			continue
		}
		col := i % len(groupX)
		drawDucatsCard(canvas, label, entry, groupX[col], drawY[col], textW, labelColor)
		drawY[col] += ducatsCardH + ducatsColGap
	}
}

// ducatsColumnEnd 计算一组卡片按两列流式排布后的 Y 终点（空组为内容起始 Y）。
func ducatsColumnEnd(count int) float64 {
	if count == 0 {
		return ducatsContentStartY
	}
	rows := (count + 1) / 2
	return ducatsContentStartY + float64(rows)*(ducatsCardH+ducatsColGap) - ducatsColGap
}

// drawDucatsCard 绘制单张杜卡币卡片（对齐 Java drawCard：标签+物品名 / 杜卡币+比率 / 均价+库存）。
func drawDucatsCard(canvas *Canvas, label string, entry *DucatsEntry, cardX, cardY, textW float64, labelColor color.RGBA) {
	innerX := cardX + ducatsCardPad
	rightX := innerX + textW

	canvas.SetColor(cardBackgroundColor).FillRoundRect(cardX, cardY, ducatsCardW, ducatsCardH, ducatsCardRadius)
	canvas.SetColor(labelColor).FillRect(cardX+ducatsCardRadius, cardY+2, ducatsCardW-2*ducatsCardRadius, 5)
	canvas.SetColor(dividerColor).SetStroke(1).DrawRoundRect(cardX, cardY, ducatsCardW, ducatsCardH, ducatsCardRadius)

	cy := cardY + ducatsCardPad

	// 标签 + 物品名
	canvas.SetColor(labelColor).SetFontSize(18)
	canvas.AddText("["+label+"]", innerX, cy+22)
	canvas.SetColor(textColor).SetFontSize(22)
	canvas.AddText(truncateRunes(entry.Item, ducatsNameMaxRunes), innerX+70, cy+22)
	cy += 42

	// 杜卡币 + 比率
	canvas.SetColor(accentGoldColor).SetFontSize(20)
	canvas.AddText(ducatsText(entry.Ducats), innerX, cy+20)
	canvas.SetColor(textSecondaryColor).SetFontSize(18)
	canvas.AddText(ratioText(entry.DucatsPerPlatinumWa), innerX+textW/2, cy+20)
	cy += 36

	// 均价 + 库存
	canvas.SetColor(textColor).SetFontSize(18)
	canvas.AddText(avgPriceText(entry.WaPrice), innerX, cy+20)
	canvas.SetColor(textSecondaryColor).SetFontSize(18)
	volume := volumeText(entry.Volume)
	canvas.AddText(volume, rightX-canvas.StringWidth(volume), cy+20)
}

// ducatsText 杜卡币文本；缺失时显示 "杜卡币 -"（对齐 Java 判空三元表达式）。
func ducatsText(ducats *int) string {
	if ducats == nil {
		return "杜卡币 -"
	}
	return "杜卡币 " + fmt.Sprintf("%d", *ducats)
}

// ratioText 实时比率文本；缺失时显示 "比率 -"。
func ratioText(ratio *float64) string {
	if ratio == nil {
		return "比率 -"
	}
	return fmt.Sprintf("比率 %.1f/P", *ratio)
}

// avgPriceText 均价文本；缺失时显示 "均价 -"。
func avgPriceText(price *float64) string {
	if price == nil {
		return "均价 -"
	}
	return fmt.Sprintf("均价 %.1f", *price)
}

// volumeText 库存文本；缺失时显示 "库存 -"。
func volumeText(volume *int) string {
	if volume == nil {
		return "库存 -"
	}
	return fmt.Sprintf("库存 %d", *volume)
}

// truncateRunes 按字符数截断文本（对齐 Java substring：超过 22 字符保留前 20 并加 ".."）。
func truncateRunes(text string, max int) string {
	if text == "" {
		return "未知"
	}
	runes := []rune(text)
	if len(runes) <= max {
		return text
	}
	return string(runes[:max-2]) + ".."
}
