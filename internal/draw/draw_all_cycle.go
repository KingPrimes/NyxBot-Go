// 平原循环图片：五张平原周期卡片（三列网格）+ 右侧看板娘
// 对齐 Java DefaultDrawAllCycleImage
package draw

import (
	"image/color"
	"time"

	"nyxbot-go/internal/enum/drawplugin"
)

// 图标字体字符（对齐 Java IconEnum 码点；注意 Go 侧 IconMap 的历史码点与此不同，此处以 Java 为准）。
const (
	iconSun   = "" // 太阳图标（白昼/温暖）
	iconNight = "" // 月亮图标（夜晚/寒冷）
	iconCold  = "" // 雪花图标（温暖期结束）
)

// 平原循环卡片布局常量（对齐 Java DefaultDrawAllCycleImage 类常量）。
const (
	cycleCanvasW       float64 = 1450 // CANVAS_W 画布宽度
	cycleContentX      float64 = 50   // CONTENT_X 内容左边距
	cycleCols          int     = 3    // COLS 列数
	cycleColGap        float64 = 20   // COL_GAP 列间距
	cycleCardRadius    float64 = 14   // CARD_RADIUS 卡片圆角
	cycleCardPad       float64 = 20   // CARD_PAD 卡片内边距
	cycleRowGap        float64 = 20   // ROW_GAP 行间距
	cycleTitleY        float64 = 80   // TITLE_Y 标题基线 Y
	cycleContentStartY float64 = 150  // CONTENT_START_Y 卡片区起始 Y
	cycleCardH         float64 = 210  // CARD_H 卡片高度
)

// AllCycle 平原循环绘图输入（对齐 Java model.worldstate.AllCycle 原始字段）。
type AllCycle struct {
	EarthCycle   *EarthCycle   // 地球循环
	CetusCycle   *CetusCycle   // 夜灵平原
	CambionCycle *CambionCycle // 魔胎之境
	VallisCycle  *VallisCycle  // 奥布山谷
	ZarimanCycle *ZarimanCycle // 扎里曼
}

// EarthCycle 地球昼夜循环（对齐 Java model.worldstate.EarthCycle 原始字段）。
type EarthCycle struct {
	Activation time.Time // 当前状态开始时间
	Expiry     time.Time // 当前状态结束时间
	IsDay      bool      // 当前状态是否为白昼
	State      string    // 白昼/夜晚
	TimeLeft   string    // 剩余时间
	Rounded    time.Time // 圆整时间
	Start      time.Time // 循环开始时间
	Expired    bool      // 循环是否已结束
}

// CetusCycle 夜灵平野昼夜循环（对齐 Java model.worldstate.CetusCycle 原始字段）。
type CetusCycle struct {
	IsDay      bool      // 当前时间是否是白昼
	Expiry     time.Time // 当前状态结束时间
	Activation time.Time // 当前状态开始时间
	State      string    // 白昼/夜晚
	Cycle      string    // day/night
	TimeLeft   string    // 剩余时间
}

// CambionCycle 魔胎之境循环（对齐 Java model.worldstate.CambionCycle 原始字段）。
type CambionCycle struct {
	Active     string    // 活动状态 FASS/VOME
	TimeLeft   string    // 剩余时间
	Expiry     time.Time // 结束时间
	Activation time.Time // 开始时间
}

// VallisCycle 奥布山谷冷暖循环（对齐 Java model.worldstate.VallisCycle 原始字段）。
type VallisCycle struct {
	Activation time.Time // 活动开始时间
	Expiry     time.Time // 活动结束时间
	IsWarm     bool      // 活动是否处于温暖期
	State      string    // 温暖/寒冷
	TimeLeft   string    // 活动剩余时间
	Expired    bool      // 活动是否已结束
}

// ZarimanCycle 扎里曼派系循环（对齐 Java model.worldstate.ZarimanCycle 原始字段）。
type ZarimanCycle struct {
	Activation time.Time // 开始时间
	Expiry     time.Time // 结束时间
	IsCorpus   bool      // 当前派系是否为 Corpus
	State      string    // Grineer/Corpus
	TimeLeft   string    // 剩余时间
	Expired    bool      // 是否已结束
}

// cycleCard 单张平原循环卡片数据（对齐 Java 内部类 CycleCard）。
type cycleCard struct {
	name     string     // 区域名称
	state    string     // 当前状态文本
	timeLeft string     // 剩余时间文本
	icon     string     // 状态图标字符
	color    color.RGBA // 状态颜色（温暖/寒冷）
}

// DrawAllCycle 绘制平原循环查询结果图（对齐 Java drawAllCycleImage；allCycle 为空返回 nil）。
func DrawAllCycle(allCycle *AllCycle) []byte {
	if allCycle == nil {
		return nil
	}

	// 子结构为空时回退零值避免空指针（Java 构造器保证非空，此处仅防御）
	earth := allCycle.EarthCycle
	if earth == nil {
		earth = &EarthCycle{}
	}
	cetus := allCycle.CetusCycle
	if cetus == nil {
		cetus = &CetusCycle{}
	}
	vallis := allCycle.VallisCycle
	if vallis == nil {
		vallis = &VallisCycle{}
	}
	cambion := allCycle.CambionCycle
	if cambion == nil {
		cambion = &CambionCycle{}
	}
	zariman := allCycle.ZarimanCycle
	if zariman == nil {
		zariman = &ZarimanCycle{}
	}

	// 五张卡片：地球/夜灵平野按昼夜，福尔图娜按冷暖，魔胎之境按 FASS/VOME，扎里曼按派系
	cambionWarm := cambion.Active == "FASS"
	zarimanCorpus := zariman.IsCorpus
	cards := []cycleCard{
		{name: "地球", state: earth.State, timeLeft: earth.TimeLeft,
			icon: dayIcon(earth.IsDay), color: cycleColor(earth.IsDay)},
		{name: "夜灵平野", state: cetus.State, timeLeft: cetus.TimeLeft,
			icon: dayIcon(cetus.IsDay), color: cycleColor(cetus.IsDay)},
		{name: "福尔图娜", state: vallis.State, timeLeft: vallis.TimeLeft,
			icon: warmIcon(vallis.IsWarm), color: cycleColor(vallis.IsWarm)},
		{name: "魔胎之境", state: cambion.Active, timeLeft: cambion.TimeLeft,
			icon: dayIcon(cambionWarm), color: cycleColor(cambionWarm)},
		{name: "扎里曼", state: zariman.State, timeLeft: zariman.TimeLeft,
			icon: zarimanIcon(zarimanCorpus), color: cycleColor(zarimanCorpus)},
	}

	n := len(cards)
	rows := (n + cycleCols - 1) / cycleCols
	lastRowY := cycleContentStartY + float64(rows-1)*(cycleCardH+cycleRowGap)

	szW, szH := scaleByPct(cycleCanvasW, cycleCanvasW, standardRatio)
	// 对齐 Java int 运算：卡片区宽度与单卡宽度均为整数除法
	cardsContentW := int(cycleCanvasW) - int(cycleContentX) - szW - 30
	cardW := float64((cardsContentW - int(cycleColGap)*(cycleCols-1)) / cycleCols)
	colX := make([]float64, cycleCols)
	for c := 0; c < cycleCols; c++ {
		colX[c] = cycleContentX + float64(c)*(cardW+cycleColGap)
	}

	standingX := cycleCanvasW - float64(szW)
	standingY := lastRowY
	canvasH := standingY + float64(szH)

	canvas := NewCanvas(int(cycleCanvasW), int(canvasH))
	canvas.SetColor(pageBackgroundColor).FillRect(0, 0, cycleCanvasW, canvasH)
	canvas.DrawTooRoundRect()

	// 标题
	canvas.SetColor(titleColor).SetFontSize(44)
	canvas.AddCenteredText("平原查询结果", cycleTitleY)
	canvas.SetColor(dividerColor).
		DrawLine(cycleContentX, cycleTitleY+55, cycleContentX+float64(cardsContentW), cycleTitleY+55)

	// 三列卡片网格
	for i, card := range cards {
		row := i / cycleCols
		col := i % cycleCols
		drawCycleCard(canvas, card, colX[col],
			cycleContentStartY+float64(row)*(cycleCardH+cycleRowGap), cardW)
	}

	canvas.DrawStandingAt(standingX, standingY, float64(szW), float64(szH))
	canvas.AddFooter(canvasH - 25)
	data, err := canvas.PNG()
	if err != nil {
		return nil
	}
	return data
}

// drawCycleCard 绘制单张平原循环卡片（对齐 Java drawCycleCard 四段布局：强调条/名称/图标/状态/剩余时间）。
func drawCycleCard(canvas *Canvas, card cycleCard, cardX, cardY, cardW float64) {
	innerX := cardX + cycleCardPad
	innerW := cardW - cycleCardPad*2

	// 卡片背景
	canvas.SetColor(cardBackgroundColor).FillRoundRect(cardX, cardY, cardW, cycleCardH, cycleCardRadius)

	// 顶部强调条
	canvas.SetColor(card.color).FillRect(cardX+cycleCardRadius, cardY+2, cardW-2*cycleCardRadius, 4)

	// 区域名称
	canvas.SetColor(titleColor).SetFontSize(26)
	canvas.AddText(card.name, innerX, cardY+34)

	// 状态图标（居中）
	canvas.SetColor(card.color).SetIconFontSize(80)
	iconW := canvas.StringWidth(card.icon)
	canvas.AddText(card.icon, cardX+(cardW-iconW)/2, cardY+118)

	// 状态文字（图标下方）
	canvas.SetColor(card.color).SetFontSize(22)
	stateW := canvas.StringWidth(card.state)
	canvas.AddText(card.state, cardX+(cardW-stateW)/2, cardY+150)

	// 剩余时间（右下角）
	canvas.SetColor(textSecondaryColor).SetFontSize(20)
	timeW := canvas.StringWidth("剩余: " + card.timeLeft)
	canvas.AddText("剩余: "+card.timeLeft, innerX+innerW-timeW, cardY+cycleCardH-22)
}

// dayIcon 按是否白昼选择图标（白昼太阳/夜晚月亮，对齐 Java IconEnum.SUN/NIGHT）。
func dayIcon(isDay bool) string {
	if isDay {
		return iconSun
	}
	return iconNight
}

// warmIcon 按是否温暖选择图标（温暖太阳/寒冷雪花，对齐 Java IconEnum.SUN/COLD）。
func warmIcon(isWarm bool) string {
	if isWarm {
		return iconSun
	}
	return iconCold
}

// zarimanIcon 按扎里曼当前派系选择图标（Corpus/Grineer，对齐 Java FactionEnum.FC_CORPUS/FC_GRINEER）。
func zarimanIcon(isCorpus bool) string {
	if isCorpus {
		return drawplugin.FactionMap[drawplugin.FactionCorpus].Icon
	}
	return drawplugin.FactionMap[drawplugin.FactionGrineer].Icon
}

// cycleColor 按状态选择循环冷暖颜色（温暖橙/寒冷蓝，对齐 ALL_CYCLE_WARM_COLOR/ALL_CYCLE_COLD_COLOR）。
func cycleColor(warm bool) color.RGBA {
	if warm {
		return allCycleWarmColor
	}
	return allCycleColdColor
}
