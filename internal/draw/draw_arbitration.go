// 仲裁图片：单条仲裁卡片（左卡片 + 右看板娘）与值得参与的仲裁列表（两列网格）
// 对齐 Java DefaultDrawArbitrationImage / DefaultDrawArbitrationsImage
package draw

import (
	"strings"
	"time"

	"nyxbot-go/internal/enum/drawplugin"
)

// Arbitration 仲裁任务绘图输入（对齐 Java model.Arbitration 原始字段）。
type Arbitration struct {
	Activation time.Time // 开始时间
	Expiry     time.Time // 结束时间
	ID         string
	Node       string // 节点
	Planet     string // 行星
	Enemy      string // 敌人标识（Grin/Corpus 等）
	Type       string // 任务类型
	Etc        string
}

// isWorth 判断仲裁是否值得参与（对齐 Java Arbitration.isWorth）。
func (a *Arbitration) isWorth() bool {
	isType := strings.Contains(a.Type, "拦截") || strings.Contains(a.Type, "防御")
	isNode := strings.Contains(a.Node, "谷神星") || strings.Contains(a.Node, "水星")
	isEnemy := strings.Contains(a.Enemy, "Grin") || strings.Contains(a.Enemy, "Infest")
	return isType && isNode && isEnemy
}

// enemyInfo 派系信息（颜色/图标）。
func (a *Arbitration) enemyInfo() drawplugin.FactionInfo {
	return matchFaction(a.Enemy)
}

// enemyName 敌人显示名。
func (a *Arbitration) enemyName() string {
	return a.enemyInfo().Name
}

// activationFormat 开始时间文本（本地时区）。
func (a *Arbitration) activationFormat() string {
	return FormatTimestamp(a.Activation)
}

// timeLeft 剩余时间文本。
func (a *Arbitration) timeLeft() string {
	return timeDeltaString(time.Until(a.Expiry))
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

// DrawArbitrations 绘制值得参与的仲裁列表图（两列网格）。
func DrawArbitrations(list []*Arbitration) []byte {
	if len(list) == 0 {
		return nil
	}
	// 仅保留值得参与的前 5 条（对齐 Java filter(isWorth).limit(5)）
	worthList := make([]*Arbitration, 0, minI(len(list), 5))
	for _, a := range list {
		if a != nil && a.isWorth() {
			worthList = append(worthList, a)
			if len(worthList) == 5 {
				break
			}
		}
	}
	if len(worthList) == 0 {
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

	n := len(worthList)
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

	for i, a := range worthList {
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

// drawArbitrationCard 绘制单张仲裁卡片（对齐 Java drawArbitrationCard 三行布局）。
func drawArbitrationCard(canvas *Canvas, a *Arbitration, cardX, cardY, cardW, cardH, radius, pad, rowH, dataSize, worthSize float64) {
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

	info := a.enemyInfo()
	curX := rightX
	if info.Icon != "" {
		canvas.SetColor(hexToColor(info.Color)).SetIconFontSize(32)
		canvas.AddText(info.Icon, curX, row1Y)
		curX += canvas.StringWidth(info.Icon) + 4
	}
	canvas.SetColor(hexToColor(info.Color)).SetFontSize(dataSize)
	canvas.AddText("敌人: "+a.enemyName(), curX, row1Y)

	// 第二行：任务类型 | 开始时间
	missionType := a.Type
	if missionType == "" {
		missionType = "未知"
	}
	canvas.SetColor(textColor).SetFontSize(dataSize)
	canvas.AddText("任务类型: "+missionType, innerX, row2Y)
	canvas.SetColor(textSecondaryColor).SetFontSize(dataSize)
	canvas.AddText("开始: "+a.activationFormat(), rightX, row2Y)

	// 第三行：剩余时间 | 值得参与
	canvas.SetColor(accentGoldColor).SetFontSize(dataSize)
	canvas.AddText("剩余: "+a.timeLeft(), innerX, row3Y)

	worthTextColor := notWorthColor
	worthText := "不值得参与"
	if a.isWorth() {
		worthText = "值得参与"
		worthTextColor = worthColor // 包常量：绿色强调
	}
	canvas.SetColor(worthTextColor).SetFontSize(worthSize)
	canvas.AddText(worthText, rightX, row3Y)
}

// scaleByPct 按百分比缩放宽高（对齐 DrawConstants.scaleByPct）。
func scaleByPct(w, h, pct float64) (int, int) {
	return int(w * pct), int(h * pct)
}
