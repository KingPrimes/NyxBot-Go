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
	MissionReward *AlertReward           // 奖励
}

// AlertReward 警报奖励（对齐 Java MissionReward）。
type AlertReward struct {
	Credits *int     // 星币（指针可空）
	Items   []string // 物品列表（已翻译）
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

	// 奖励文字（超宽截断加 "..")
	rewardText := buildRewardText(alert)
	if rewardText != "" {
		canvas.SetColor(accentGoldColor).SetFontSize(24)
		maxRewardW := innerX + innerW - cursorX - 6
		for canvas.StringWidth(rewardText) > float64(maxRewardW) && len(rewardText) > 3 {
			rewardText = rewardText[:len(rewardText)-1]
		}
		if canvas.StringWidth(rewardText) > float64(maxRewardW) {
			rewardText += ".."
		}
		canvas.AddText(rewardText, float64(cursorX), float64(badgeY+1))
	}
}

// buildRewardText 拼接奖励文本：星币 + 前 5 个物品（对齐 Java buildRewardText）。
func buildRewardText(alert *Alert) string {
	if alert.MissionInfo == nil || alert.MissionInfo.MissionReward == nil {
		return ""
	}
	reward := alert.MissionInfo.MissionReward
	var sb []string
	if reward.Credits != nil && *reward.Credits > 0 {
		sb = append(sb, strconv.Itoa(*reward.Credits)+"星币")
	}
	count := 0
	for _, it := range reward.Items {
		if count >= 5 {
			break
		}
		if it != "" {
			sb = append(sb, it)
			count++
		}
	}
	return strings.Join(sb, "  ")
}
