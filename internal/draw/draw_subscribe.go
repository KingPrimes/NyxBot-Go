// 订阅指令说明图：用法说明卡片 + 订阅类型/任务类型/入侵奖励三张数值表（四色分组）
// 对齐 Java DefaultDrawWarframeSubscribeImage
package draw

import (
	"fmt"
	"image/color"
	"sort"
)

// 订阅图布局常量（对齐 Java DefaultDrawWarframeSubscribeImage 各静态字段）。
const (
	subscribeCanvasW    float64 = 1600 // 画布宽度
	subscribeContentX   float64 = 50   // 内容左边距
	subscribeCardPad    float64 = 24   // 卡片内边距
	subscribeCardRad    float64 = 14   // 卡片圆角
	subscribeSectionGap float64 = 20   // 卡片节间距
	subscribeTitleY     float64 = 80   // 标题基线 Y
)

// DrawWarframeSubscribe 绘制订阅指令说明图（subscribe/missionType/invasionReward 全空返回 nil）。
func DrawWarframeSubscribe(subscribe, missionType, invasionReward map[int]string) []byte {
	if len(subscribe) == 0 && len(missionType) == 0 && len(invasionReward) == 0 {
		return nil
	}
	const (
		bodySize  float64 = 26 // 用法说明正文
		tableSize float64 = 24 // 表格标题
		smallSize float64 = 22 // 参数说明
	)

	contentW := subscribeCanvasW - subscribeContentX*2

	// 预计算各区域高度（对齐 Java 行高公式）
	currentY := subscribeContentX + 50 // 标题区域后开始

	// 用法说明卡片高度（5 行说明）
	usageH := subscribeCardPad + 40 + 5*36 + subscribeCardPad

	// 订阅类型表卡片高度
	subscribeRows := (len(subscribe) + 4) / 5
	subscribeH := subscribeCardPad + 42 + 8 + float64(subscribeRows)*38 + subscribeCardPad

	// 任务类型表卡片高度
	missionRows := (len(missionType) + 4) / 5
	missionH := subscribeCardPad + 42 + 8 + float64(missionRows)*38 + subscribeCardPad

	// 入侵奖励表卡片高度（无行则不绘制）
	invasionRows := (len(invasionReward) + 4) / 5
	var invasionH float64
	if invasionRows > 0 {
		invasionH = subscribeCardPad + 42 + 8 + float64(invasionRows)*38 + subscribeCardPad
	}

	// 构建各节卡片 Y
	sectionHeights := [4]float64{usageH, subscribeH, missionH, invasionH}
	var sectionY [4]float64
	cy := currentY
	for i, h := range sectionHeights {
		sectionY[i] = cy
		cy += h + subscribeSectionGap
	}
	contentEnd := cy - subscribeSectionGap
	standingY := contentEnd + 10
	szW, szH := scaleByPct(subscribeCanvasW, subscribeCanvasW, standardRatio)
	totalHeight := maxF(standingY+float64(szH), contentEnd+float64(szH))

	canvas := NewCanvas(int(subscribeCanvasW), int(totalHeight))
	canvas.SetColor(pageBackgroundColor).FillRect(0, 0, subscribeCanvasW, totalHeight)
	canvas.DrawTooRoundRect()

	// 标题
	canvas.SetColor(titleColor).SetFontSize(44)
	canvas.AddCenteredText("订阅指令说明", subscribeTitleY)
	canvas.SetColor(dividerColor).DrawLine(subscribeContentX, subscribeTitleY+50, subscribeCanvasW-subscribeContentX, subscribeTitleY+50)

	// 用法说明卡片
	drawSubscribeUsageCard(canvas, sectionY[0], contentW, usageH, bodySize, smallSize)
	// 订阅类型表（红色分组）
	drawSubscribeTableCard(canvas, sectionY[1], contentW, subscribeH,
		"订阅内容类型数值", subscribe, subscribeRedColor, tableSize)
	// 任务类型表（紫色分组）
	drawSubscribeTableCard(canvas, sectionY[2], contentW, missionH,
		"订阅任务类型数值", missionType, subscribePurpleColor, tableSize)
	// 入侵奖励表（蓝色分组）
	if invasionH > 0 {
		drawSubscribeTableCard(canvas, sectionY[3], contentW, invasionH,
			"入侵奖励类型数值", invasionReward, subscribeBlueColor, tableSize)
	}

	canvas.DrawStandingAt(subscribeCanvasW-float64(szW), totalHeight-float64(szH), float64(szW), float64(szH))
	canvas.AddFooter(totalHeight - 25)
	data, err := canvas.PNG()
	if err != nil {
		return nil
	}
	return data
}

// drawSubscribeUsageCard 绘制用法说明卡片（蓝色顶条 + 使用方式 + 4 行类型参数示例）。
func drawSubscribeUsageCard(canvas *Canvas, cardY, cardW, cardH, bodySize, smallSize float64) {
	innerX := subscribeContentX + subscribeCardPad

	canvas.SetColor(cardBackgroundColor).FillRoundRect(subscribeContentX, cardY, cardW, cardH, subscribeCardRad)
	canvas.SetColor(subscribeBlueColor).FillRect(subscribeContentX+subscribeCardRad, cardY+2, cardW-2*subscribeCardRad, 5)
	canvas.SetColor(dividerColor).SetStroke(1).
		DrawRoundRect(subscribeContentX, cardY, cardW, cardH, subscribeCardRad)

	cy := cardY + subscribeCardPad + 30

	// 使用方式（测量 "使用方式：" 需在切换 smallSize 之前，保持 bodySize 字体）
	canvas.SetColor(textColor).SetFontSize(bodySize)
	canvas.AddText("使用方式：", innerX, cy+8)
	x := innerX + canvas.StringWidth("使用方式：")
	canvas.SetColor(subscribeRedColor).SetFontSize(smallSize)
	canvas.AddText("订阅 ", x, cy+8)
	x += canvas.StringWidth("订阅 ")
	canvas.SetColor(textColor).AddText("<类型编号>[-<子参数>]", x+10, cy+8)
	cy += 36

	// 各类型参数说明（类型名红 / 格式说明白 / 示例棕）
	typeParams := [][3]string{
		{"裂隙 9", "类型编号-任务类型-遗物等级", "9-11-4"},
		{"入侵 6", "类型编号-奖励类型", "6-3"},
		{"仲裁 2", "类型编号-任务类型", "2-1"},
		{"其他", "仅类型编号", "1"},
	}
	for _, row := range typeParams {
		canvas.SetFontSize(smallSize)
		x = innerX
		canvas.SetColor(subscribeRedColor).AddText(row[0], x, cy+8)
		x += canvas.StringWidth(row[0] + "  ")
		canvas.SetColor(textColor).AddText(row[1], x, cy+8)
		x += canvas.StringWidth(row[1] + "  ")
		canvas.SetColor(subscribeBrownColor).AddText("例: "+row[2], x, cy+8)
		cy += 36
	}
}

// drawSubscribeTableCard 绘制单张类型数值表卡片（accent 顶条 + 标题 + 5 列键值单元格）。
func drawSubscribeTableCard(canvas *Canvas, cardY, cardW, cardH float64, title string,
	data map[int]string, accent color.RGBA, tableSize float64) {
	innerX := subscribeContentX + subscribeCardPad
	rightX := subscribeContentX + cardW - subscribeCardPad

	canvas.SetColor(cardBackgroundColor).FillRoundRect(subscribeContentX, cardY, cardW, cardH, subscribeCardRad)
	canvas.SetColor(accent).FillRect(subscribeContentX+subscribeCardRad, cardY+2, cardW-2*subscribeCardRad, 5)
	canvas.SetColor(dividerColor).SetStroke(1).
		DrawRoundRect(subscribeContentX, cardY, cardW, cardH, subscribeCardRad)

	cy := cardY + subscribeCardPad
	canvas.SetColor(accent).SetFontSize(tableSize)
	canvas.AddText(title, innerX, cy+28)
	cy += 42

	canvas.SetColor(dividerColor).DrawLine(innerX, cy+4, rightX, cy+4)
	cy += 12

	if len(data) == 0 {
		return
	}
	entries := sortedMapEntries(data)

	cols := 5
	colW := (cardW - subscribeCardPad*2) / float64(cols)
	for i, kv := range entries {
		col := i % cols
		row := i / cols
		cx := innerX + float64(col)*colW
		rowY := cy + float64(row)*38
		canvas.SetColor(textColor).SetFontSize(20)
		canvas.AddText(fmt.Sprintf("%d = %s", kv.key, kv.value), cx, rowY+24)
	}
}

// mapEntry 订阅表按键排序后的键值对。
type mapEntry struct {
	key   int
	value string
}

// sortedMapEntries 按键升序排序订阅表条目（对齐 Java Map.Entry.comparingByKey）。
func sortedMapEntries(m map[int]string) []mapEntry {
	entries := make([]mapEntry, 0, len(m))
	for k, v := range m {
		entries = append(entries, mapEntry{key: k, value: v})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].key < entries[j].key })
	return entries
}
