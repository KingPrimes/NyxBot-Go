// 集团任务图片：节点视图（垂直居中列表）或赏金任务三列卡片流式视图
// 对齐 Java DefaultDrawSyndicateImage / model.SyndicateMission
package draw

import (
	"image/color"
	"strconv"

	"nyxbot-go/internal/enum/drawplugin"
)

// syndicateCard 卡片固定参数（对齐 Java CARD_MIN_HEIGHT / IMAGE_*）。
const (
	syndicateCardMinH = 250
	syndicateImageW   = 1600
)

// SyndicateTag 集团标签（对齐 Java SyndicateMission.Tag）。
type SyndicateTag struct {
	Name string // 集团名称
}

// SyndicateJob 赏金任务（对齐 Java model.worldstate.Job）。
type SyndicateJob struct {
	Type       string             // 任务类型
	IsVault    bool               // 是否保险库
	Endless    bool               // 是否无尽
	MinLevel   int                // 最低敌人等级
	MaxLevel   int                // 最高敌人等级
	MasteryReq int                // 段位要求
	Desc       string             // 任务描述
	Rewards    []*SyndicateReward // 奖励列表
	XpAmounts  []int              // 声望值
}

// SyndicateReward 赏金奖励（对齐 Java RewardPool.Reward）。
type SyndicateReward struct {
	Rarity    drawplugin.Rarity // 稀有度
	Item      string            // 物品名
	ItemCount int               // 数量
}

// SyndicateMission 集团任务绘图输入（对齐 Java model.SyndicateMission）。
type SyndicateMission struct {
	Tag   *SyndicateTag   // 集团标签
	Nodes []string        // 节点列表
	Jobs  []*SyndicateJob // 赏金任务列表
}

// DrawSyndicateImage 绘制集团任务图：存在节点画节点视图，否则画卡片视图（对齐 Java drawSyndicateImage）。
func DrawSyndicateImage(sm *SyndicateMission) []byte {
	if sm == nil {
		return nil
	}
	if len(sm.Nodes) > 0 {
		return drawSyndicateNodesView(sm)
	}
	if len(sm.Jobs) > 0 {
		return drawSyndicateJobsView(sm)
	}
	return nil
}

// drawSyndicateNodesView 绘制 Nodes 视图（垂直居中节点列表）。
func drawSyndicateNodesView(sm *SyndicateMission) []byte {
	const (
		imageWidth   = 1200
		imageMarginT = 60
		imageTitleH  = 50
		imageFooterH = 40
	)
	nodes := sm.Nodes
	if len(nodes) == 0 {
		return nil
	}
	nodeHeight := len(nodes) * 50
	totalHeight := imageMarginT + imageTitleH + nodeHeight + imageFooterH

	canvas := NewCanvas(imageWidth, totalHeight)
	canvas.SetColor(pageBackgroundColor).FillRect(0, 0, float64(imageWidth), float64(totalHeight))
	canvas.DrawTooRoundRect()

	title := "集团任务 - 节点"
	if sm.Tag != nil && sm.Tag.Name != "" {
		title = sm.Tag.Name + " - 节点"
	}
	canvas.SetColor(titleColor).SetFontSize(32)
	canvas.AddCenteredText(title, float64(imageMarginT+30))

	y := imageMarginT + imageTitleH + 50
	for _, n := range nodes {
		canvas.SetColor(textColor).SetFontSize(24)
		canvas.AddCenteredText("• "+n, float64(y))
		y += 50
	}

	canvas.AddFooter(float64(totalHeight - imageFooterH))
	data, err := canvas.PNG()
	if err != nil {
		return nil
	}
	return data
}

// drawSyndicateJobsView 绘制 Jobs 三列卡片流式视图。
func drawSyndicateJobsView(sm *SyndicateMission) []byte {
	const (
		contentX     = 60
		cols         = 3
		cardMarginX  = 35
		cardMarginY  = 30
		imageMarginT = 60
		imageTitleH  = 50
		imageFooterH = 40
	)
	imageWidth := syndicateImageW
	jobs := sm.Jobs
	if len(jobs) == 0 {
		return nil
	}
	n := len(jobs)
	isOdd := n%cols != 0

	szW, szHBox := scaleByPct(float64(imageWidth), float64(imageWidth), standardRatio)
	cardsContentW := imageWidth - contentX*2
	cardW := (cardsContentW - cardMarginX*(cols-1)) / cols
	textMaxW := cardW - 40

	colX := make([]int, cols)
	for c := 0; c < cols; c++ {
		colX[c] = contentX + c*(cardW+cardMarginX)
	}

	cardHeights := make([]int, n)
	for i := range jobs {
		cardHeights[i] = calcSyndicateCardHeight(jobs[i], textMaxW)
	}

	startY := imageMarginT + imageTitleH + 25
	colEndY := make([]int, cols)
	for c := range colEndY {
		colEndY[c] = startY
	}
	for i := 0; i < n; i++ {
		col := i % cols
		colEndY[col] += cardHeights[i] + cardMarginY
	}
	for c := range colEndY {
		if colEndY[c] > startY {
			colEndY[c] -= cardMarginY
		}
	}

	var standingY int
	if isOdd {
		standingY = maxI(colEndY[0], colEndY[1])
	} else {
		maxEnd := startY
		for c := range colEndY {
			maxEnd = maxI(maxEnd, colEndY[c])
		}
		standingY = maxEnd + 10
	}
	standingX := imageWidth - szW
	totalHeight := standingY + szHBox

	canvas := NewCanvas(imageWidth, totalHeight)
	canvas.SetColor(pageBackgroundColor).FillRect(0, 0, float64(imageWidth), float64(totalHeight))
	canvas.DrawTooRoundRect()

	title := "集团任务"
	if sm.Tag != nil && sm.Tag.Name != "" {
		title = sm.Tag.Name + " - 赏金任务"
	}
	canvas.SetColor(titleColor).SetFontSize(32)
	canvas.AddCenteredText(title, float64(imageMarginT+30))

	drawY := make([]int, cols)
	for c := range drawY {
		drawY[c] = startY
	}
	for i := 0; i < n; i++ {
		col := i % cols
		drawSyndicateJobCard(canvas, jobs[i], colX[col], drawY[col], cardW, cardHeights[i])
		drawY[col] += cardHeights[i] + cardMarginY
	}

	canvas.DrawStandingAt(float64(standingX), float64(standingY), float64(szW), float64(szHBox))
	canvas.AddFooter(float64(totalHeight - imageFooterH))
	data, err := canvas.PNG()
	if err != nil {
		return nil
	}
	return data
}

// drawSyndicateJobCard 绘制单个赏金卡片。
func drawSyndicateJobCard(canvas *Canvas, job *SyndicateJob, cardX, cardY, cardW, cardHeight int) {
	border := syndicateJobBorderColor(job)

	canvas.SetColor(pageBackgroundColor).FillRect(float64(cardX), float64(cardY), float64(cardW), float64(cardHeight))
	canvas.SetColor(cardBackgroundColor).FillRoundRect(float64(cardX), float64(cardY), float64(cardW), float64(cardHeight), 15)
	canvas.SetColor(border).SetStroke(5).DrawRoundRect(float64(cardX), float64(cardY), float64(cardW), float64(cardHeight), 15)

	y := cardY + 30
	typeText := job.Type
	if typeText == "" {
		typeText = "未知任务"
	}
	if job.IsVault {
		typeText += " [保险库]"
	} else if job.Endless {
		typeText += " [无尽]"
	}
	canvas.SetColor(titleColor).SetFontSize(20)
	canvas.AddText(typeText, float64(cardX+(cardW-int(canvas.StringWidth(typeText)))/2), float64(y))
	y += 40

	// 敌人等级
	if job.MaxLevel > 0 {
		canvas.SetColor(textColor).SetFontSize(18)
		canvas.AddText("敌人等级: Lv."+strconv.Itoa(job.MinLevel)+" - Lv."+strconv.Itoa(job.MaxLevel), float64(cardX+20), float64(y))
		y += 30
	}

	// 段位要求
	if job.MasteryReq > 0 {
		canvas.SetColor(color.RGBA{0xE6, 0x7E, 0x22, 0xff}).SetFontSize(18)
		canvas.AddText("段位要求: MR "+strconv.Itoa(job.MasteryReq), float64(cardX+20), float64(y))
		y += 30
	}

	// 任务描述（自动换行）
	if job.Desc != "" {
		canvas.SetColor(textColor).SetFontSize(18)
		for _, line := range wrapText(canvas, job.Desc, float64(cardW-40)) {
			canvas.AddText(line, float64(cardX+20), float64(y))
			y += 25
		}
		y += 10
	}

	// 奖励列表
	if len(job.Rewards) > 0 {
		canvas.SetColor(titleColor).SetFontSize(20)
		canvas.AddText("奖励:", float64(cardX+20), float64(y))
		y += 25
		for _, r := range job.Rewards {
			canvas.SetColor(syndicateRarityColor(r.Rarity)).SetFontSize(18)
			canvas.AddText("  • "+r.Item+" x"+strconv.Itoa(r.ItemCount), float64(cardX+20), float64(y))
			y += 25
		}
	}

	// 声望奖励（卡片底部固定）
	canvas.SetColor(color.RGBA{0x27, 0xAE, 0x60, 0xff}).SetFontSize(16)
	canvas.AddText("声望奖励: "+strconv.Itoa(syndicateXP(job)), float64(cardX+20), float64(cardY+cardHeight-14))
}

// calcSyndicateCardHeight 计算单个赏金卡片高度（对齐 Java calculateJobCardHeight）。
func calcSyndicateCardHeight(job *SyndicateJob, textMaxW int) int {
	height := 30 + 40 // 任务类型 + 敌人等级
	if job.MasteryReq > 0 {
		height += 30
	}
	if job.Desc != "" {
		height += syndicateTextLines(job.Desc, textMaxW)*25 + 10
	}
	height += 30 // 奖励标题
	height += len(job.Rewards) * 25
	height += 40 // 经验值
	height += 20 // 内边距
	return maxI(height, syndicateCardMinH)
}

// syndicateTextLines 文本按最大宽度环绕后的行数（对齐 Java calculateTextLines）。
func syndicateTextLines(text string, maxWidth int) int {
	if text == "" || maxWidth <= 0 {
		return 0
	}
	return textWrapLineCount(text, 18, float64(maxWidth))
}

// syndicateJobBorderColor 任务边框颜色（对齐 Java getJobBorderColor）。
func syndicateJobBorderColor(job *SyndicateJob) color.RGBA {
	if job.IsVault {
		return color.RGBA{0x9B, 0x59, 0xB6, 0xff} // 紫色 - 保险库
	}
	if job.Endless {
		return color.RGBA{0xE6, 0x7E, 0x22, 0xFF} // 橙色 - 无尽
	}
	return color.RGBA{0x95, 0xA5, 0xA6, 0xFF} // 灰色 - 普通
}

// syndicateRarityColor 稀有度颜色（对齐 Java getRarityColor）。
func syndicateRarityColor(rarity drawplugin.Rarity) color.RGBA {
	switch rarity {
	case drawplugin.RarityCommon:
		return voidT2Color
	case drawplugin.RarityUncommon:
		return voidT3Color
	case drawplugin.RarityRare:
		return voidT4Color
	case drawplugin.RarityLegendary:
		return voidT5Color
	default:
		return textColor
	}
}

// syndicateXP 声望值求和（对齐 Java xpAmounts 求和）。
func syndicateXP(job *SyndicateJob) int {
	total := 0
	for _, xp := range job.XpAmounts {
		total += xp
	}
	return total
}
