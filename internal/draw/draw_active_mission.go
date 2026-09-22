// 裂隙图片：两列卡片网格布局（居中标题 + 每张卡片顶部 tier 颜色强调条）
// 对齐 Java DefaultDrawActiveMission / model.ActiveMission
package draw

import (
	"image/color"
	"strconv"
	"strings"
	"time"

	"nyxbot-go/internal/enum/drawplugin"
)

// ActiveMission 裂隙任务绘图输入（对齐 Java model.ActiveMission 原始字段）。
type ActiveMission struct {
	ID          string                 // 唯一标识
	Activation  time.Time              // 开始时间
	Expiry      time.Time              // 结束时间
	MissionType drawplugin.MissionType // 任务类型
	Modifier    drawplugin.VoidTier    // 遗物等级
	Node        string                 // 节点
	Faction     drawplugin.Faction     // 派系
	Region      int                    // 地区
	Seed        int                    // 种子
	Hard        bool                   // 是否是钢铁模式
	VoidStorms  bool                   // 是否是虚空风暴模式
}

// timeLeft 剩余时间文本（对齐 Java getTimeLeft）。
func (m *ActiveMission) timeLeft() string {
	return timeDeltaString(time.Until(m.Expiry))
}

// missionTypeName 任务类型显示名（对齐 Java getMissionTypeName，未知回退"未知"）。
func (m *ActiveMission) missionTypeName() string {
	if info, ok := drawplugin.MissionTypeMap[m.MissionType]; ok {
		return info.Name
	}
	return "未知"
}

// factionInfo 派系显示信息（空派系不显示，未知键回退未知派系）。
func (m *ActiveMission) factionInfo() (drawplugin.FactionInfo, bool) {
	return factionKeyInfo(m.Faction)
}

// modifierName 遗物等级显示名（对齐 Java getModifierName，空值回退 VoidT1"古纪"）。
func (m *ActiveMission) modifierName() string {
	if name, ok := drawplugin.VoidTierMap[m.Modifier]; ok {
		return name
	}
	return "古纪"
}

// voidTierColor 遗物等级颜色（对齐 Java VoidEnum.color；VoidT6 全能色在 draw 包内补齐）。
// 返回颜色与是否有效；未知等级 ok 为 false。
func voidTierColor(tier drawplugin.VoidTier) (color.RGBA, bool) {
	switch tier {
	case drawplugin.VoidT1:
		return voidT1Color, true
	case drawplugin.VoidT2:
		return voidT2Color, true
	case drawplugin.VoidT3:
		return voidT3Color, true
	case drawplugin.VoidT4:
		return voidT4Color, true
	case drawplugin.VoidT5:
		return voidT5Color, true
	case drawplugin.VoidT6:
		return color.RGBA{0x7f, 0x8c, 0x8d, 0xff}, true // 全能
	}
	return color.RGBA{}, false
}

// voidEnName 遗物等级英文名（对齐 Java getVoidEnName）。
func voidEnName(name string) string {
	switch name {
	case "古纪":
		return "Lith"
	case "前纪":
		return "Meso"
	case "中纪":
		return "Neo"
	case "后纪":
		return "Axi"
	case "安魂":
		return "Requiem"
	case "全能":
		return "Omnia"
	}
	return ""
}

// timeColor 裂隙剩余时间颜色（对齐 Java getTimeColor：含 h 为充裕，含 m 且数字 < 10 为紧急）。
func timeColor(t string) color.RGBA {
	if t == "" {
		return activeTimeLowColor
	}
	lo := strings.ToLower(t)
	if strings.Contains(lo, "h") {
		return activeTimeHighColor
	}
	if strings.Contains(lo, "m") {
		// 对齐 Java replaceAll("\\D","")：去掉所有非数字后整体解析
		digits := strings.Map(func(r rune) rune {
			if r >= '0' && r <= '9' {
				return r
			}
			return -1
		}, lo)
		if n, err := strconv.Atoi(digits); err == nil {
			if n < 10 {
				return activeTimeLowColor
			}
			return activeTimeMediumColor
		}
	}
	return activeTimeLowColor
}

// lighten 颜色向白色提亮 45%（对齐 Java DefaultDrawActiveMission.lighten）。
func lighten(c color.RGBA) color.RGBA {
	return color.RGBA{
		uint8(float64(c.R) + (255-float64(c.R))*0.45),
		uint8(float64(c.G) + (255-float64(c.G))*0.45),
		uint8(float64(c.B) + (255-float64(c.B))*0.45),
		255,
	}
}

// MissionTypeColor 任务类型颜色（对齐 Java MissionTypeEnum.getColor，未知/空类型回退黑色）。
// 导出供 warframe 查询层为其它含任务类型的图（如执刑官猎杀）取同一套配色。
func MissionTypeColor(mt drawplugin.MissionType) color.RGBA {
	switch mt {
	case drawplugin.MTAssassination:
		return color.RGBA{0xff, 0x6b, 0x6b, 0xff} // 刺杀 - 红色
	case drawplugin.MTExtermination:
		return color.RGBA{0xff, 0x9f, 0x43, 0xff} // 歼灭 - 橙色
	case drawplugin.MTSurvival:
		return color.RGBA{0xcd, 0xa2, 0x41, 0xff} // 生存 - 黄色
	case drawplugin.MTRescue:
		return color.RGBA{0x1d, 0xd1, 0xa1, 0xff} // 救援 - 绿色
	case drawplugin.MTSabotage:
		return color.RGBA{0x54, 0xa0, 0xff, 0xff} // 破坏 - 蓝色
	case drawplugin.MTCapture:
		return color.RGBA{0x5f, 0x27, 0xcd, 0xff} // 捕获 - 紫色
	case drawplugin.MTIntel:
		return color.RGBA{0x00, 0xd2, 0xd3, 0xff} // 间谍 - 青色
	case drawplugin.MTDefense:
		return color.RGBA{0xff, 0x9f, 0xf3, 0xff} // 防御 - 粉色
	case drawplugin.MTMobileDefense:
		return color.RGBA{0x75, 0xb1, 0xe6, 0xff} // 移动防御 - 灰蓝
	case drawplugin.MTTerritory:
		return color.RGBA{0x01, 0xa3, 0xa4, 0xff} // 拦截 - 深青色
	case drawplugin.MTHive:
		return color.RGBA{0x83, 0x95, 0xa7, 0xff} // 清巢 - 石板灰
	case drawplugin.MTRetrieval:
		return color.RGBA{0xff, 0xea, 0xa7, 0xff} // 劫持 - 浅黄
	case drawplugin.MTExcavate:
		return color.RGBA{0xdd, 0xa0, 0xdd, 0xff} // 挖掘 - 梅花色
	case drawplugin.MTSalvage:
		return color.RGBA{0xa2, 0x9b, 0xfe, 0xff} // 资源回收 - 紫罗兰
	case drawplugin.MTAssault:
		return color.RGBA{0xff, 0x76, 0x75, 0xff} // 强袭 - 浅红
	case drawplugin.MTEvacuation:
		return color.RGBA{0xfd, 0xcb, 0x6e, 0xff} // 叛逃 - 金色
	case drawplugin.MTArtifact, drawplugin.MTDisruption:
		return color.RGBA{0xe1, 0x70, 0x55, 0xff} // 中断 - 橘红
	case drawplugin.MTVoidFlood:
		return color.RGBA{0x6c, 0x5c, 0xe7, 0xff} // 虚空洪流 - 靛蓝
	case drawplugin.MTVoidCascade:
		return color.RGBA{0xa2, 0x9b, 0xfe, 0xff} // 虚空覆涌 - 熏衣草
	case drawplugin.MTVoidArmageddon:
		return color.RGBA{0x2d, 0x34, 0x36, 0xff} // 虚空决战 - 深灰
	case drawplugin.MTAlchemy:
		return color.RGBA{0x00, 0xb8, 0x94, 0xff} // 元素转换 - 绿松石
	case drawplugin.MTCambire:
		return color.RGBA{0xe8, 0x43, 0x93, 0xff} // 异化区 - 热粉
	case drawplugin.MTShrineDefense:
		return color.RGBA{0xfd, 0x79, 0xa8, 0xff} // 祈运坛防御 - 粉红
	case drawplugin.MTFaceoff:
		return color.RGBA{0xe6, 0x7e, 0x22, 0xff} // 对战 - 胡萝卜色
	case drawplugin.MTSkirmish:
		return color.RGBA{0xe7, 0x4c, 0x3c, 0xff} // 前哨战 - 珊瑚红
	case drawplugin.MTVolatile:
		return color.RGBA{0xf3, 0x9c, 0x12, 0xff} // 爆发 - 太阳花黄
	case drawplugin.MTOrpheus:
		return color.RGBA{0x9b, 0x59, 0xb6, 0xff} // 奥菲斯 - 紫水晶
	case drawplugin.MTAscension:
		return color.RGBA{0x34, 0x98, 0xdb, 0xff} // 扬升 - 湛蓝
	case drawplugin.MTCorruption:
		return color.RGBA{0x8e, 0x44, 0xad, 0xff} // 虚空腐蚀 - 紫罗兰
	case drawplugin.MTEndlessCapture:
		return color.RGBA{0xe8, 0x43, 0x93, 0xff} // 无尽捕获 - 热粉
	default:
		return color.RGBA{0x00, 0x00, 0x00, 0xff} // 默认黑色
	}
}

// DrawActiveMission 绘制裂隙任务列表图（两列卡片网格，标题按首条任务模式切换）。
func DrawActiveMission(missions []*ActiveMission) []byte {
	if len(missions) == 0 {
		return nil
	}
	const (
		canvasW       float64 = 1200
		contentX      float64 = 60
		contentW      float64 = 1080
		cols          int     = 2
		colGap        float64 = 20
		cardH         float64 = 160
		cardRad       float64 = 14
		cardPad       float64 = 20
		rowGap        float64 = 20
		stripH        float64 = 4
		titleY        float64 = 80
		contentStartY float64 = 160
	)
	cardW := (contentW - colGap) / float64(cols)
	colX := [2]float64{contentX, contentX + cardW + colGap}

	// 过滤空条目（对齐 Java 直接遍历非空列表）
	list := make([]*ActiveMission, 0, len(missions))
	for _, m := range missions {
		if m != nil {
			list = append(list, m)
		}
	}
	if len(list) == 0 {
		return nil
	}

	n := len(list)
	rows := (n + cols - 1) / cols
	cardsH := rows*int(cardH) + (rows-1)*int(rowGap)
	isOdd := n%cols != 0
	lastRowY := contentStartY + float64((rows-1)*(int(cardH)+int(rowGap)))

	// 看板娘尺寸（以画布宽度为参考，对齐 Java scaleByPct）
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

	// 标题：按首条任务模式切换（对齐 Java 判定逻辑）
	title := "虚空裂隙"
	titleCol := titleColor
	if list[0].VoidStorms {
		title = "虚空风暴"
	} else if list[0].Hard {
		title = "钢铁裂隙"
		titleCol = accentColor
	}
	canvas.SetColor(titleCol).SetFontSize(44)
	canvas.AddCenteredText(title, titleY)
	canvas.SetColor(dividerColor).DrawLine(contentX, titleY+55, contentX+contentW, titleY+55)

	for i, m := range list {
		row := i / cols
		col := i % cols
		drawFissureCard(canvas, m, colX[col], contentStartY+float64(row*(int(cardH)+int(rowGap))),
			cardW, cardH, cardRad, cardPad, stripH, 30, 26, 24, 22)
	}

	canvas.DrawStandingAt(standingX, standingY, float64(szW), float64(szH))
	canvas.AddFooter(canvasH - 25)
	data, err := canvas.PNG()
	if err != nil {
		return nil
	}
	return data
}

// drawFissureCard 绘制单张裂隙卡片（对齐 Java drawFissureCard 三行布局）。
func drawFissureCard(canvas *Canvas, m *ActiveMission, cardX, cardY, cardW, cardH, radius, pad, stripH, tierSize, bodySize, timeSize, locSize float64) {
	innerX := cardX + pad
	innerW := cardW - pad*2

	// 卡片背景
	canvas.SetColor(cardBackgroundColor).FillRoundRect(cardX, cardY, cardW, cardH, radius)

	// tier 强调条（遗物等级颜色）
	tierRgb, hasTier := voidTierColor(m.Modifier)
	if hasTier {
		canvas.SetColor(tierRgb).FillRect(cardX+radius, cardY+2, cardW-2*radius, stripH)
	}

	// 行 1：tier 名称 + 剩余时间
	mn := m.modifierName()
	tierText := mn + " " + voidEnName(mn)
	tierCol := textSecondaryColor
	if hasTier {
		tierCol = lighten(tierRgb)
	}
	canvas.SetColor(tierCol).SetFontSize(tierSize)
	canvas.AddText(tierText, innerX, cardY+42)

	eta := m.timeLeft()
	if eta == "" {
		eta = "未知"
	}
	canvas.SetColor(timeColor(eta)).SetFontSize(timeSize)
	canvas.AddText(eta, innerX+innerW-canvas.StringWidth(eta), cardY+43)

	// 行 2：任务类型 + 派系（居中）
	mt := m.missionTypeName()
	mtCol := MissionTypeColor(m.MissionType)
	fn, fnOK := m.factionInfo()
	row2Y := cardY + 85
	canvas.SetFontSize(bodySize)
	mtW := canvas.StringWidth(mt)
	gap := 0.0
	fnW := 0.0
	if fnOK {
		gap = 8
		fnW = canvas.StringWidth(fn.Name)
	}
	totalW := mtW + gap + fnW
	curX := innerX + (innerW-totalW)/2
	canvas.SetColor(mtCol).AddText(mt, curX, row2Y)
	curX += mtW + gap
	if fnOK {
		if fn.Icon != "" {
			canvas.SetColor(hexToColor(fn.Color)).SetIconFontSize(32)
			canvas.AddText(fn.Icon, curX, row2Y)
			curX += canvas.StringWidth(fn.Icon) + 4
		}
		canvas.SetColor(hexToColor(fn.Color)).SetFontSize(bodySize)
		canvas.AddText(fn.Name, curX, row2Y)
	}

	// 行 3：节点
	node := m.Node
	if node == "" {
		node = "未知节点"
	}
	canvas.SetColor(textColor).SetFontSize(locSize)
	canvas.AddText(node, innerX, cardY+130)
}
