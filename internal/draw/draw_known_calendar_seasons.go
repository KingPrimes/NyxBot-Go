// 1999 日历季节图片：两列月卡片网格 + 看板娘（对齐 Java DefaultDrawKnownCalendarSeasonsImage）
package draw

import (
	"image/color"
	"sort"
	"strconv"
)

// CalendarEventType 日历事件类型（对齐 Java KnownCalendarSeasons.Events.EventTypeEnum）。
type CalendarEventType int

const (
	CETChallenge CalendarEventType = 0 // 任务
	CETReward    CalendarEventType = 1 // 奖励
	CETUpgrade   CalendarEventType = 2 // 加成
)

// CalendarEvent 日历单日事件（对齐 Java KnownCalendarSeasons.Events）。
type CalendarEvent struct {
	Type      CalendarEventType // 事件类型
	Challenge string            // 任务描述
	Reward    string            // 奖励描述
	Upgrade   string            // 加成描述
}

// CalendarDay 日历单日（对齐 Java KnownCalendarSeasons.Days）。
type CalendarDay struct {
	Month  int              // 月份
	Day    int              // 日
	Events []*CalendarEvent // 事件
}

// KnownCalendarSeasons 日历季节绘图输入（对齐 Java model.KnownCalendarSeasons）。
type KnownCalendarSeasons struct {
	Season        string                 // 季节名
	YearIteration int                    // 年份迭代（第 N 次）
	Version       string                 // 版本
	MonthDays     map[int][]*CalendarDay // 月份 → 天数列表
}

// calendarMonthCard 内部月份卡片（对日历按月份分组的载体）。
type calendarMonthCard struct {
	month int
	days  []*CalendarDay
}

// calendar 卡片常量（对齐 Java DefaultDrawKnownCalendarSeasonsImage）。
const (
	calendarCardRadius = 14 // 卡片圆角
	calendarCardPad    = 20 // 卡片内边距
)

// DrawKnownCalendarSeasons 绘制 1999 日历季节图（对齐 drawKnownCalendarSeasonsImage）。
func DrawKnownCalendarSeasons(list []*KnownCalendarSeasons) []byte {
	if len(list) == 0 || list[0] == nil {
		return nil
	}
	calendar := list[0]
	if len(calendar.MonthDays) == 0 {
		return nil
	}
	const (
		contentX      = 50
		cols          = 2
		colGap        = 20
		titleY        = 80
		contentStartY = 220
	)
	cardW := 562
	textW := cardW - calendarCardPad*2
	canvasW := contentX + cols*cardW + (cols-1)*colGap + contentX

	// 按月份排序（对齐 Java TreeMap）
	months := make([]int, 0, len(calendar.MonthDays))
	for m := range calendar.MonthDays {
		months = append(months, m)
	}
	sort.Ints(months)

	cards := make([]calendarMonthCard, 0, len(months))
	for _, m := range months {
		cards = append(cards, calendarMonthCard{month: m, days: calendar.MonthDays[m]})
	}
	n := len(cards)
	isOdd := n%cols != 0

	colX := make([]int, cols)
	for c := 0; c < cols; c++ {
		colX[c] = contentX + c*(cardW+colGap)
	}

	cardHeights := make([]int, n)
	for i := range cards {
		cardHeights[i] = calcMonthCardHeight(cards[i])
	}

	colEndY := make([]int, cols)
	for c := range colEndY {
		colEndY[c] = contentStartY
	}
	for i := 0; i < n; i++ {
		col := i % cols
		colEndY[col] += cardHeights[i] + colGap
	}
	for c := range colEndY {
		if colEndY[c] > contentStartY {
			colEndY[c] -= colGap
		}
	}

	standingX := colX[1]
	var totalHeight int
	if isOdd {
		// 奇数布局最后一张卡在左列且为最高列；页脚画在 totalHeight-25（基线），
		// 多留 40px 底部间距才能让页脚整体落在卡片底边之下（30px 时上沿仍压住卡片约 8px）
		totalHeight = maxI(colEndY[0], colEndY[1]+cardW) + 40
	} else {
		maxEnd := contentStartY
		for c := range colEndY {
			maxEnd = maxI(maxEnd, colEndY[c])
		}
		totalHeight = maxEnd + 10 + cardW + 30
	}

	canvas := NewCanvas(canvasW, totalHeight)
	canvas.SetColor(pageBackgroundColor).FillRect(0, 0, float64(canvasW), float64(totalHeight))
	canvas.DrawTooRoundRect()

	// 标题 + 分割线
	canvas.SetColor(titleColor).SetFontSize(44)
	canvas.AddCenteredText("1999 日历季节信息", titleY)
	canvas.SetColor(dividerColor).DrawLine(float64(contentX), float64(titleY+50), float64(canvasW-contentX), float64(titleY+50))

	// 季节信息
	seasonName := calendar.Season
	if seasonName == "" {
		seasonName = "未知"
	}
	iter := "未知"
	if calendar.YearIteration > 0 {
		iter = "第 " + strconv.Itoa(calendar.YearIteration) + " 次"
	}
	ver := calendar.Version
	if ver == "" {
		ver = "未知"
	}
	info := "季节: " + seasonName + "    |    年份迭代: " + iter + "    |    版本: " + ver
	canvas.SetColor(textSecondaryColor).SetFontSize(22)
	canvas.AddText(info, float64((canvasW-int(canvas.StringWidth(info)))/2), float64(titleY+95))

	drawY := make([]int, cols)
	for c := range drawY {
		drawY[c] = contentStartY
	}
	for i := 0; i < n; i++ {
		col := i % cols
		drawMonthCard(canvas, cards[i].month, cards[i].days, colX[col], drawY[col], cardW, cardHeights[i], textW)
		drawY[col] += cardHeights[i] + colGap
	}

	standingY := colEndY[1]
	if !isOdd {
		standingY = totalHeight - cardW
	}
	canvas.DrawStandingAt(float64(standingX), float64(standingY), float64(cardW), float64(cardW))
	canvas.AddFooter(float64(totalHeight) - 25)
	data, err := canvas.PNG()
	if err != nil {
		return nil
	}
	return data
}

// calcMonthCardHeight 计算单张月卡高度（对齐 Java calcCardHeight）。
func calcMonthCardHeight(card calendarMonthCard) int {
	h := calendarCardPad // 顶部
	h += 40              // 月份标题
	h += 8               // 分隔线
	for _, day := range card.days {
		h += 30 // 日期行
		h += len(day.Events) * 30
	}
	h += calendarCardPad // 底部
	return maxI(h, 100)
}

// drawMonthCard 绘制单张月份卡片。
func drawMonthCard(canvas *Canvas, month int, days []*CalendarDay, cardX, cardY, cardW, cardH, textW int) {
	innerX := cardX + calendarCardPad
	rightX := innerX + textW
	seasonColor := calendarSeasonColor(month)

	canvas.SetColor(cardBackgroundColor).FillRoundRect(float64(cardX), float64(cardY), float64(cardW), float64(cardH), calendarCardRadius)
	canvas.SetColor(seasonColor).FillRect(float64(cardX+calendarCardRadius), float64(cardY+2), float64(cardW-2*calendarCardRadius), 5)
	canvas.SetColor(dividerColor).SetStroke(1).DrawRoundRect(float64(cardX), float64(cardY), float64(cardW), float64(cardH), calendarCardRadius)

	cy := cardY + calendarCardPad
	canvas.SetColor(seasonColor).SetFontSize(28)
	canvas.AddText(strconv.Itoa(month)+"月", float64(innerX), float64(cy+30))
	cy += 40

	canvas.SetColor(dividerColor).DrawLine(float64(innerX), float64(cy+4), float64(rightX), float64(cy+4))
	cy += 12

	for _, day := range days {
		dayText := strconv.Itoa(day.Month) + "月" + strconv.Itoa(day.Day) + "日"
		canvas.SetColor(textColor).SetFontSize(20)
		canvas.AddText(dayText, float64(innerX), float64(cy+22))
		cy += 30
		for _, ev := range day.Events {
			label, eColor := calendarEventDesc(ev)
			canvas.SetColor(eColor).SetFontSize(18)
			canvas.AddText(label, float64(innerX+15), float64(cy+20))
			cy += 30
		}
	}
}

// calendarSeasonColor 月份 → 季节色（对齐 Java getSeasonColor）。
func calendarSeasonColor(month int) color.RGBA {
	switch {
	case month == 12 || month <= 3:
		return color.RGBA{0x34, 0x98, 0xDB, 0xff} // 冬
	case month <= 6:
		return color.RGBA{0xE9, 0x1E, 0x8C, 0xff} // 春
	case month <= 9:
		return color.RGBA{0xFF, 0x98, 0x00, 0xff} // 夏
	default:
		return color.RGBA{0xC0, 0x39, 0x2B, 0xff} // 秋
	}
}

// calendarEventDesc 事件类型 → 标签前缀 + 颜色（对齐 Java getEventDesc 与事件色）。
func calendarEventDesc(ev *CalendarEvent) (string, color.RGBA) {
	var label, desc string
	var col color.RGBA
	switch ev.Type {
	case CETChallenge:
		label, desc, col = "[任务] ", ev.Challenge, color.RGBA{0xFF, 0x6B, 0x6B, 0xff}
	case CETReward:
		label, desc, col = "[奖励] ", ev.Reward, color.RGBA{0x4C, 0xAF, 0x50, 0xff}
	case CETUpgrade:
		label, desc, col = "[加成] ", ev.Upgrade, color.RGBA{0xB8, 0x86, 0x0B, 0xff}
	default:
		return "未知", textSecondaryColor
	}
	if desc == "" {
		return label, col
	}
	// 路径（含 /）未翻译 → 取尾段
	for i := len(desc) - 1; i >= 0; i-- {
		if desc[i] == '/' {
			desc = desc[i+1:]
			break
		}
	}
	return label + desc, col
}
