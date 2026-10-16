// 仲裁图片：单条仲裁卡片（左卡片 + 右看板娘）与仲裁前瞻列表（两列网格）
// 对齐 Java DefaultDrawArbitrationImage / DefaultDrawArbitrationsImage
package draw

import (
	"image/color"
	"strings"
	"time"

	"nyxbot-go/internal/enum/drawplugin"
)

// arbitrationCardLimit 前瞻列表图最多渲染的卡片数（对齐 Java filter(isWorth).limit(5)）。
const arbitrationCardLimit = 5

// Arbitration 仲裁任务绘图输入。节点名（含星系）/ 任务类型 / 评级等显示值
// 由 warframe.ArbitrationToDraw 在转换时补齐，draw 层只渲染不判定。
type Arbitration struct {
	Activation  time.Time          // 开始时间
	Expiry      time.Time          // 结束时间
	Node        string             // 节点显示名「名称(星系)」
	MissionName string             // 任务类型中文名
	Faction     drawplugin.Faction // 派系枚举
	Tier        string             // 评级 S/A/B/C/D/F（空值或未知值按 F 渲染）
}

// factionInfo 派系显示信息（颜色/图标）；空派系回退未知派系。
func (a *Arbitration) factionInfo() drawplugin.FactionInfo {
	info, ok := factionKeyInfo(a.Faction)
	if !ok {
		return drawplugin.FactionMap[drawplugin.FactionNone]
	}
	return info
}

// activationFormat 开始时间文本（本地时区）。
func (a *Arbitration) activationFormat() string {
	return FormatTimestamp(a.Activation)
}

// timeLeft 剩余时间文本。
func (a *Arbitration) timeLeft() string {
	return timeDeltaString(time.Until(a.Expiry))
}

// tierColors 仲裁评级 → 颜色（S 最优，F 最差）。
var tierColors = map[string]color.RGBA{
	"S": tierSColor,
	"A": tierAColor,
	"B": tierBColor,
	"C": tierCColor,
	"D": tierDColor,
	"F": tierFColor,
}

// tierLabel 评级显示文本：空值与未知值一律按 F 渲染。
func tierLabel(tier string) string {
	normalized := strings.ToUpper(strings.TrimSpace(tier))
	if _, ok := tierColors[normalized]; !ok {
		return "F"
	}
	return normalized
}

// tierColor 评级对应颜色；空值与未知值按 F 渲染。
func tierColor(tier string) color.RGBA {
	return tierColors[tierLabel(tier)]
}

// DrawArbitration 绘制单条仲裁卡片图。
func DrawArbitration(a *Arbitration) []byte {
	if a == nil {
		return nil
	}
	const (
		canvasW  float64 = 1450
		contentX float64 = 50
		cardW    float64 = 850
		cardH    float64 = 210
		cardRad  float64 = 14
		cardPad  float64 = 24
		rowH     float64 = 52
		titleY   float64 = 80
		cardY    float64 = 150
	)

	szW, szH := scaleByPct(canvasW, canvasW, standardRatio)
	standingX := canvasW - float64(szW)
	standingY := cardY
	canvasH := standingY + float64(maxI(int(cardH), szH))

	canvas := NewCanvas(int(canvasW), int(canvasH))
	canvas.SetColor(pageBackgroundColor).FillRect(0, 0, canvasW, canvasH)
	canvas.DrawTooRoundRect()

	// 标题
	canvas.SetColor(titleColor).SetFontSize(36)
	canvas.AddText("仲裁任务", contentX, titleY)
	canvas.SetColor(dividerColor).DrawLine(contentX, titleY+55, contentX+cardW, titleY+55)

	drawArbitrationCard(canvas, a, contentX, cardY, cardW, cardH, cardRad, cardPad, rowH, 20, 24)

	canvas.DrawStandingAt(standingX, standingY, float64(szW), float64(szH))
	canvas.AddFooter(canvasH - 25)
	data, err := canvas.PNG()
	if err != nil {
		return nil
	}
	return data
}

// DrawArbitrations 绘制仲裁前瞻列表图（两列网格）。列表已由 warframe 层按评级过滤，
// 此处只渲染不判定，最多取前 arbitrationCardLimit 条。
func DrawArbitrations(list []*Arbitration) []byte {
	if len(list) == 0 {
		return nil
	}
	cards := make([]*Arbitration, 0, minI(len(list), arbitrationCardLimit))
	for _, a := range list {
		if a == nil {
			continue
		}
		cards = append(cards, a)
		if len(cards) == arbitrationCardLimit {
			break
		}
	}
	if len(cards) == 0 {
		return nil
	}

	const (
		canvasW       float64 = 1300
		contentX      float64 = 50
		contentW      float64 = 1200
		cols          int     = 2
		colGap        float64 = 20
		cardH         float64 = 190
		cardRad       float64 = 14
		cardPad       float64 = 20
		rowGap        float64 = 20
		cardRowH      float64 = 44
		titleY        float64 = 80
		contentStartY float64 = 160
	)
	cardW := (contentW - colGap) / float64(cols)
	colX := [2]float64{contentX, contentX + cardW + colGap}

	n := len(cards)
	rows := (n + cols - 1) / cols
	cardsH := rows*int(cardH) + (rows-1)*int(rowGap)
	isOdd := n%cols != 0
	lastRowY := contentStartY + float64((rows-1)*(int(cardH)+int(rowGap)))

	szW, szH := scaleByPct(canvasW, canvasW, standardRatio)
	standingX := canvasW - float64(szW)
	standingY := contentStartY + float64(cardsH) + 10
	if isOdd {
		standingY = lastRowY
	}
	canvasH := standingY + float64(szH)

	canvas := NewCanvas(int(canvasW), int(canvasH))
	canvas.SetColor(pageBackgroundColor).FillRect(0, 0, canvasW, canvasH)
	canvas.DrawTooRoundRect()

	canvas.SetColor(titleColor).SetFontSize(40)
	canvas.AddCenteredText("有价值的仲裁任务", titleY)
	canvas.SetColor(dividerColor).DrawLine(contentX, titleY+55, contentX+contentW, titleY+55)

	for i, a := range cards {
		row := i / cols
		col := i % cols
		drawArbitrationCard(canvas, a, colX[col], contentStartY+float64(row*(int(cardH)+int(rowGap))), cardW, cardH, cardRad, cardPad, cardRowH, 18, 20)
	}

	canvas.DrawStandingAt(standingX, standingY, float64(szW), float64(szH))
	canvas.AddFooter(canvasH - 25)
	data, err := canvas.PNG()
	if err != nil {
		return nil
	}
	return data
}

// drawArbitrationCard 绘制单张仲裁卡片（对齐 Java drawArbitrationCard 三行布局，
// 第三行右列展示节点评级）。
func drawArbitrationCard(canvas *Canvas, a *Arbitration, cardX, cardY, cardW, cardH, radius, pad, rowH, dataSize, tierSize float64) {
	innerX := cardX + pad
	innerW := cardW - pad*2
	rightX := innerX + innerW/2 + 10

	// 卡片背景
	canvas.SetColor(cardBackgroundColor).FillRoundRect(cardX, cardY, cardW, cardH, radius)

	row1Y := cardY + 45
	row2Y := row1Y + rowH
	row3Y := row2Y + rowH

	// 第一行：节点 | 敌人
	node := a.Node
	if node == "" {
		node = "未知节点"
	}
	canvas.SetColor(textColor).SetFontSize(dataSize)
	canvas.AddText("节点: "+node, innerX, row1Y)

	info := a.factionInfo()
	curX := rightX
	if info.Icon != "" {
		canvas.SetColor(hexToColor(info.Color)).SetIconFontSize(32)
		canvas.AddText(info.Icon, curX, row1Y)
		curX += canvas.StringWidth(info.Icon) + 4
	}
	canvas.SetColor(hexToColor(info.Color)).SetFontSize(dataSize)
	canvas.AddText("敌人: "+info.Name, curX, row1Y)

	// 第二行：任务类型 | 开始时间
	missionName := a.MissionName
	if missionName == "" {
		missionName = "未知"
	}
	canvas.SetColor(textColor).SetFontSize(dataSize)
	canvas.AddText("任务类型: "+missionName, innerX, row2Y)
	canvas.SetColor(textSecondaryColor).SetFontSize(dataSize)
	canvas.AddText("开始: "+a.activationFormat(), rightX, row2Y)

	// 第三行：剩余时间 | 评级
	canvas.SetColor(accentGoldColor).SetFontSize(dataSize)
	canvas.AddText("剩余: "+a.timeLeft(), innerX, row3Y)

	canvas.SetColor(tierColor(a.Tier)).SetFontSize(tierSize)
	canvas.AddText("评级: "+tierLabel(a.Tier), rightX, row3Y)
}

// scaleByPct 按百分比缩放宽高（对齐 DrawConstants.scaleByPct）。
func scaleByPct(w, h, pct float64) (int, int) {
	return int(w * pct), int(h * pct)
}
