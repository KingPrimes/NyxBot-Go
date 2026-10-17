// 警报图片：两列卡片网格布局（居中标题 + 双层装饰边框 + 每张警报独立圆角卡片）
// 对齐 Java DefaultDrawAlertsImage / model.Alert
package draw

import (
	"strconv"
	"strings"

	"nyxbot-go/internal/enum/drawplugin"
)

// alert 卡片固定高度（对齐 Java CARD_H）。
const (
	alertCardH      = 150
	alertCardPad    = 20
	alertCardRadius = 14
)

// alertRewardItemLimit 奖励区最多列出的物品数（对齐 Java buildRewardText 的 limit(5)）。
const alertRewardItemLimit = 5

// Alert 警报绘图输入（对齐 Java model.Alert 原始字段）。
type Alert struct {
	TimeLeft    string            // 剩余时间文本
	MissionInfo *AlertMissionInfo // 任务信息
}

// AlertMissionInfo 警报任务信息（对齐 Java Alert.MissionInfo）。
type AlertMissionInfo struct {
	Location      string                 // 节点地点
	MissionType   drawplugin.MissionType // 任务类型
	Faction       drawplugin.Faction     // 派系
	MissionReward *Reward                // 奖励（复用入侵侧同款 Reward / RewardItem）
}

// DrawAlerts 绘制警报列表图（对齐 Java drawAlertsImage 两列卡片网格）。
func DrawAlerts(alerts []*Alert) []byte {
	const (
		canvasW       = 1200
		contentX      = 60
		contentW      = 1080
		cols          = 2
		colGap        = 20
		rowGap        = 20
		titleY        = 100
		dividerY      = 140
		contentStartY = 170
	)
	cardW := (contentW - colGap) / cols
	colX := [2]int{contentX, contentX + cardW + colGap}

	// 过滤空条目
	list := make([]*Alert, 0, len(alerts))
	for _, a := range alerts {
		if a != nil {
			list = append(list, a)
		}
	}
	if len(list) == 0 {
		return nil
	}

	n := len(list)
	rows := (n + cols - 1) / cols
	cardsH := rows*alertCardH + (rows-1)*rowGap
	isOdd := n%cols != 0
	lastRowY := contentStartY + (rows-1)*(alertCardH+rowGap)

	var canvasH int
	if isOdd {
		canvasH = maxI(contentStartY+cardsH, lastRowY+360)
	} else {
		canvasH = contentStartY + cardsH + 360
	}

	canvas := NewCanvas(canvasW, canvasH)
	canvas.SetColor(pageBackgroundColor).FillRect(0, 0, float64(canvasW), float64(canvasH))
	canvas.DrawTooRoundRect()

	// 标题 + 分割线
	canvas.SetColor(titleColor).SetFontSize(56)
	canvas.AddCenteredText("警报", titleY)
	canvas.SetColor(dividerColor).DrawLine(contentX, dividerY, contentX+contentW, dividerY)

	for i, a := range list {
		row := i / cols
		col := i % cols
		drawAlertCard(canvas, a, colX[col], contentStartY+row*(alertCardH+rowGap), cardW)
	}

	szW, szH := scaleByPct(canvasW, canvasW, standardRatio)
	canvas.DrawStandingAt(float64(canvasW-szW), float64(canvasH-szH), float64(szW), float64(szH))
	canvas.AddFooter(float64(canvasH) - 25)
	data, err := canvas.PNG()
	if err != nil {
		return nil
	}
	return data
}

// drawAlertCard 绘制单张警报卡片（地点/剩余时间 + 任务类型/派系 + 奖励）。
func drawAlertCard(canvas *Canvas, alert *Alert, cardX, cardY, cardW int) {
	innerX := cardX + alertCardPad
	innerW := cardW - alertCardPad*2
	mi := alert.MissionInfo

	canvas.SetColor(cardBackgroundColor).FillRoundRect(float64(cardX), float64(cardY), float64(cardW), float64(alertCardH), alertCardRadius)

	// 行 1：地点（左）+ 剩余时间（右）
	location := ""
	if mi != nil && mi.Location != "" {
		location = mi.Location
	}
	if location == "" {
		location = "未知节点"
	}
	canvas.SetColor(textColor).SetFontSize(28)
	canvas.AddText(location, float64(innerX), float64(cardY+30))

	eta := ""
	if alert.TimeLeft != "" {
		eta = alert.TimeLeft
	} else {
		eta = "未知"
	}
	canvas.SetColor(accentGoldColor).SetFontSize(20)
	etaW := canvas.StringWidth(eta)
	canvas.AddText(eta, float64(innerX+innerW)-etaW, float64(cardY+30))

	// 行 2：任务类型（有色）+ 派系（有色）+ 奖励
	badgeY := cardY + 74
	cursorX := innerX

	if mi != nil && mi.MissionType != "" {
		mtName := string(mi.MissionType) // 显示名
		if info, ok := drawplugin.MissionTypeMap[mi.MissionType]; ok {
			mtName = info.Name
		}
		canvas.SetColor(MissionTypeColor(mi.MissionType)).SetFontSize(22)
		canvas.AddText(mtName, float64(cursorX), float64(badgeY+3))
		cursorX += int(canvas.StringWidth(mtName)) + 10
	}

	if mi != nil && mi.Faction != "" {
		fInfo, ok := factionKeyInfo(mi.Faction)
		if ok {
			fColor := hexToColor(fInfo.Color)
			if fInfo.Icon != "" {
				canvas.SetColor(fColor).SetIconFontSize(22)
				canvas.AddText(fInfo.Icon, float64(cursorX), float64(badgeY+5))
				cursorX += int(canvas.StringWidth(fInfo.Icon)) + 4
			}
			canvas.SetColor(fColor).SetFontSize(22)
			canvas.AddText(fInfo.Name, float64(cursorX), float64(badgeY+3))
			cursorX += int(canvas.StringWidth(fInfo.Name)) + 14
		}
	}

	// 奖励文字（超宽截断加 ".."；按 rune 截断，避免把 UTF-8 字符切成半个）
	rewardText := buildRewardText(alert)
	if rewardText != "" {
		canvas.SetColor(accentGoldColor).SetFontSize(24)
		maxRewardW := innerX + innerW - cursorX - 6
		rewardText = truncateToWidth(canvas, rewardText, float64(maxRewardW))
		canvas.AddText(rewardText, float64(cursorX), float64(badgeY+1))
	}
}

// truncateToWidth 把文本按 rune 截断到给定像素宽度内，为 ".." 后缀预留宽度，
// 且仅在确实发生截断时追加后缀（对齐 Java DefaultDrawAlertsImage 的奖励文字处理）。
// 文本未超宽时原样返回，保证不会出现「已截断但无后缀」或后缀导致再次溢出。
func truncateToWidth(canvas *Canvas, text string, maxWidth float64) string {
	if canvas.StringWidth(text) <= maxWidth {
		return text
	}
	runes := []rune(text)
	truncated := false
	// 循环中以「候选文本 + ..」测宽，确保加上后缀后仍不超宽
	for len(runes) > 3 && canvas.StringWidth(string(runes)+"..") > maxWidth {
		runes = runes[:len(runes)-1]
		truncated = true
	}
	if !truncated {
		return text
	}
	return string(runes) + ".."
}

// buildRewardText 拼接奖励文本：星币 + 带数量物品 + 普通物品（物品合计最多 alertRewardItemLimit 个）。
//
// 与 Java buildRewardText 的差异：Java 只读 `items`，漏掉了 `countedItems`——而真实警报奖励恰恰以
// `countedItems` 表达（如活动警报的「17000 星币 + 月华脯 (50)」），照搬会导致卡片只显示星币。
// 此处按「带数量物品优先」补齐，格式对齐游戏内显示的「名称 (数量)」。
func buildRewardText(alert *Alert) string {
	if alert.MissionInfo == nil || alert.MissionInfo.MissionReward == nil {
		return ""
	}
	reward := alert.MissionInfo.MissionReward
	var sb []string
	if reward.Credits > 0 {
		sb = append(sb, strconv.Itoa(reward.Credits)+"星币")
	}
	listed := 0
	for _, item := range reward.CountedItems {
		if listed >= alertRewardItemLimit {
			break
		}
		if text := countedItemText(item); text != "" {
			sb = append(sb, text)
			listed++
		}
	}
	for _, name := range reward.Items {
		if listed >= alertRewardItemLimit {
			break
		}
		if name != "" {
			sb = append(sb, name)
			listed++
		}
	}
	return strings.Join(sb, "  ")
}

// countedItemText 带数量物品的显示文本「名称 (数量)」（与游戏内一致）。
// 名称为空返回空串（调用侧跳过）；数量缺失显示 `(?)`。
// 注：入侵卡沿用「50x 名称」（draw_invasion.go 的 rewardItemText），两处格式有意不同——
// 警报卡此处按主人要求对齐游戏内显示。
func countedItemText(item *RewardItem) string {
	if item == nil || item.Name == "" {
		return ""
	}
	if item.Count == nil {
		return item.Name + " (?)"
	}
	return item.Name + " (" + strconv.Itoa(*item.Count) + ")"
}
