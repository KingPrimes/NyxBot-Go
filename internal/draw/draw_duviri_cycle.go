// 双衍王境图片：情绪展示卡 + 普通/钢铁双列选择卡（对齐 Java DefaultDrawDuviriCycleImage）
package draw

import (
	"image/color"
)

// DuviriChoice 双衍选择分类（对齐 Java EndlessXpChoices.Category）。
type DuviriChoice struct {
	Category DuviriCategory // 分类
	Choices  []string       // 选项列表
}

// DuviriCategory 双衍选择分类枚举。
type DuviriCategory string

const (
	DuviriNormal DuviriCategory = "EXC_NORMAL" // 普通
	DuviriHard   DuviriCategory = "EXC_HARD"   // 钢铁
)

// DuvalierCycle 双衍王境绘图输入（对齐 Java model.DuvalierCycle）。
type DuvalierCycle struct {
	State    string         // 当前情绪（中文）
	TimeLeft string         // 剩余时间文本
	Choices  []DuviriChoice // 选择分类列表
}

// DrawDuviriCycle 绘制双衍王境图（情绪展示卡 + 普通/钢铁双列选择卡）。
func DrawDuviriCycle(cycle *DuvalierCycle) []byte {
	const (
		canvasW    = 1000
		contentX   = 60
		contentW   = 880
		cardRadius = 14
		colGap     = 20
		titleY     = 70
		maxItems   = 7
		itemH      = 32
	)
	cardW := (contentW - colGap) / 2

	normalItems := []string{}
	hardItems := []string{}
	for _, c := range cycle.Choices {
		if c.Category == DuviriNormal {
			normalItems = c.Choices
		}
		if c.Category == DuviriHard {
			hardItems = c.Choices
		}
	}

	maxItemsVal := len(normalItems)
	if len(hardItems) > maxItemsVal {
		maxItemsVal = len(hardItems)
	}
	displayed := minI(maxItemsVal, maxItems)
	choiceCardsH := 80 + displayed*itemH + 30
	canvasH := 130 + 110 + 40 + choiceCardsH + 300

	canvas := NewCanvas(canvasW, canvasH)
	canvas.SetColor(pageBackgroundColor).FillRect(0, 0, float64(canvasW), float64(canvasH))
	canvas.DrawTooRoundRect()

	canvas.SetColor(titleColor).SetFontSize(48)
	canvas.AddCenteredText("双衍王境", titleY)
	canvas.SetColor(dividerColor).DrawLine(contentX, titleY+40, contentX+contentW, titleY+40)

	// 情绪展示卡
	emotion := cycle.State
	if emotion == "" {
		emotion = "喜悦"
	}
	emotionColorVal := duviriEmotionColor(emotion)
	emotionCardW := 420
	emotionCardH := 110
	emotionCardX := (canvasW - emotionCardW) / 2
	emotionCardY := 130

	canvas.SetColor(cardBackgroundColor).FillRoundRect(float64(emotionCardX), float64(emotionCardY), float64(emotionCardW), float64(emotionCardH), cardRadius)
	canvas.SetColor(emotionColorVal).SetStroke(3).DrawRoundRect(float64(emotionCardX), float64(emotionCardY), float64(emotionCardW), float64(emotionCardH), cardRadius)

	timeLeft := cycle.TimeLeft
	canvas.SetColor(textSecondaryColor).SetFontSize(18)
	canvas.AddCenteredText("当前情绪", float64(emotionCardY+25))
	canvas.SetColor(emotionColorVal).SetFontSize(40)
	canvas.AddCenteredText(emotion, float64(emotionCardY+65))
	if timeLeft != "" {
		canvas.SetColor(accentColor).SetFontSize(18)
		canvas.AddCenteredText("剩余: "+timeLeft, float64(emotionCardY+95))
	}

	// 选择卡
	cardsY := emotionCardY + emotionCardH + 40
	rightX := contentX + cardW + colGap
	drawDuviriChoiceCard(canvas, "普通", normalItems, contentX, cardsY, cardW, accentGoldColor, cardRadius, itemH, maxItems)
	drawDuviriChoiceCard(canvas, "钢铁之路", hardItems, rightX, cardsY, cardW, accentColor, cardRadius, itemH, maxItems)

	szW, szH := scaleByPct(canvasW, canvasW, standardRatio)
	canvas.DrawStandingAt(float64(canvasW-szW), float64(canvasH-szH), float64(szW), float64(szH))
	canvas.AddFooter(float64(canvasH) - 25)
	data, err := canvas.PNG()
	if err != nil {
		return nil
	}
	return data
}

// drawDuviriChoiceCard 绘制单个双衍选择卡（标题 + 顶部强调条 + 选项列表）。
func drawDuviriChoiceCard(canvas *Canvas, title string, items []string, cardX, cardY, cardW int, accent color.RGBA, cardRadius, itemH, maxItems int) {
	if items == nil {
		return
	}
	displayed := minI(len(items), maxItems)
	cardH := 80 + displayed*itemH + 30

	canvas.SetColor(cardBackgroundColor).FillRoundRect(float64(cardX), float64(cardY), float64(cardW), float64(cardH), float64(cardRadius))
	canvas.SetColor(accent).FillRect(float64(cardX+cardRadius), float64(cardY+2), float64(cardW-2*cardRadius), 4)

	canvas.SetColor(titleColor).SetFontSize(24)
	titleW := canvas.StringWidth(title)
	canvas.AddText(title, float64(cardX)+float64(cardW)/2-titleW/2, float64(cardY+35))
	canvas.SetColor(dividerColor).DrawLine(float64(cardX+20), float64(cardY+52), float64(cardX+cardW-20), float64(cardY+52))

	itemY := cardY + 70
	for i := 0; i < displayed; i++ {
		item := items[i]
		if len(item) > 22 {
			item = item[:20] + ".."
		}
		canvas.SetColor(textColor).SetFontSize(20)
		canvas.AddText("• "+item, float64(cardX+25), float64(itemY+8))
		itemY += itemH
	}
	if len(items) == 0 {
		canvas.SetColor(textMutedColor).SetFontSize(20)
		canvas.AddText("暂无", float64(cardX+25), float64(cardY+85))
	}
}

// duviriEmotionColor 情绪中文名 → 颜色（对齐 Java getEmotionColor）。
func duviriEmotionColor(emotion string) color.RGBA {
	switch emotion {
	case "悲伤":
		return emotionSadColor
	case "恐惧":
		return emotionFearColor
	case "喜悦":
		return emotionJoyColor
	case "愤怒":
		return emotionAngerColor
	case "嫉妒":
		return emotionEnvyColor
	default:
		return accentGoldColor
	}
}
