// 入侵图片：单列宽卡片布局（进攻/防守双色进度条 + 阵营对抗 + 奖励）
// 对齐 Java DefaultDrawInvasionImage / model.Invasion / model.Reward
package draw

import (
	"fmt"
	"image/color"
	"math"
	"sort"
	"time"

	"nyxbot-go/internal/enum/drawplugin"
)

// Invasion 入侵任务绘图输入（对齐 Java model.Invasion 原始字段）。
type Invasion struct {
	ID              string             // 唯一标识
	Activation      time.Time          // 开始时间
	Expiry          time.Time          // 结束时间
	Faction         drawplugin.Faction // 进攻方阵营
	DefenderFaction drawplugin.Faction // 防守方阵营
	Node            string             // 星图节点
	Count           *float64           // 当前进度计数（负数表示防守方领先）
	Goal            *float64           // 目标值
	LocTag          string             // 本地化标签
	Completed       bool               // 是否完成
	ChainID         string             // 入侵任务链唯一标识
	AttackerReward  []*Reward          // 进攻方奖励列表
	DefenderReward  *Reward            // 防守方奖励
}

// Reward 任务奖励绘图输入（对齐 Java model.Reward 原始字段）。
type Reward struct {
	Credits      int           // 现金奖励数量
	Xp           int           // 经验值奖励数量
	Items        []string      // 物品名称列表
	CountedItems []*RewardItem // 带数量的物品奖励列表
}

// RewardItem 带数量的奖励物品（对齐 Java Reward.Item）。
type RewardItem struct {
	Name  string // 物品名称
	Count *int   // 物品数量
}

// DrawInvasion 绘制入侵任务列表图（单列宽卡片，按目标值降序排列）。
func DrawInvasion(invasions []*Invasion) []byte {
	if len(invasions) == 0 {
		return nil
	}
	const (
		canvasW       float64 = 1300
		contentX      float64 = 60
		contentW      float64 = 1180
		cardW         float64 = 1180
		cardH         float64 = 200
		cardRad       float64 = 14
		cardPad       float64 = 24
		rowGap        float64 = 20
		stripH        float64 = 6
		titleY        float64 = 80
		contentStartY float64 = 150
	)

	// 过滤空条目并按目标值降序排序（nil 目标排最后，对齐 Java nullsLast(reverseOrder)）
	sorted := make([]*Invasion, 0, len(invasions))
	for _, inv := range invasions {
		if inv != nil {
			sorted = append(sorted, inv)
		}
	}
	if len(sorted) == 0 {
		return nil
	}
	sort.Slice(sorted, func(i, j int) bool {
		gi, gj := sorted[i].Goal, sorted[j].Goal
		switch {
		case gi == nil:
			return false // nil 排最后
		case gj == nil:
			return true
		default:
			return *gi > *gj
		}
	})

	n := len(sorted)
	cardsH := n*int(cardH) + (n-1)*int(rowGap)
	szW, szH := scaleByPct(canvasW, canvasW, standardRatio)
	canvasH := contentStartY + float64(cardsH) + float64(szH)

	canvas := NewCanvas(int(canvasW), int(canvasH))
	canvas.SetColor(pageBackgroundColor).FillRect(0, 0, canvasW, canvasH)
	canvas.DrawTooRoundRect()

	canvas.SetColor(accentColor).SetFontSize(48)
	canvas.AddCenteredText("入侵任务", titleY)
	canvas.SetColor(dividerColor).DrawLine(contentX, titleY+55, contentX+contentW, titleY+55)

	for i, inv := range sorted {
		drawInvasionCard(canvas, inv, contentX, contentStartY+float64(i*(int(cardH)+int(rowGap))),
			cardW, cardH, cardRad, cardPad, stripH, 32, 26, 22, 24)
	}

	canvas.DrawStandingAt(canvasW-float64(szW), canvasH-float64(szH), float64(szW), float64(szH))
	canvas.AddFooter(canvasH - 25)
	data, err := canvas.PNG()
	if err != nil {
		return nil
	}
	return data
}

// drawInvasionCard 绘制单张入侵卡片（对齐 Java drawInvasionCard 三行布局）。
func drawInvasionCard(canvas *Canvas, inv *Invasion, cardX, cardY, cardW, cardH, radius, pad, stripH, nodeSize, factionSize, rewardSize, progressSize float64) {
	innerX := cardX + pad
	innerW := cardW - pad*2

	// 完成度（对齐 Java：goal 非空非零且 count 非空时计算，并钳制到 1.0）
	progress := 0.0
	if inv.Goal != nil && *inv.Goal != 0 && inv.Count != nil {
		progress = math.Min(math.Abs(*inv.Count) / *inv.Goal, 1.0)
	}

	// 卡片背景
	canvas.SetColor(cardBackgroundColor).FillRoundRect(cardX, cardY, cardW, cardH, radius)

	// 顶部双色进度条（进攻红 / 防守绿，两端避开圆角）
	splitX := cardX + float64(int(cardW*progress))
	if splitX-cardX > radius {
		canvas.SetColor(attackerColor).FillRect(cardX+radius, cardY+2, splitX-cardX-radius, stripH)
	}
	if cardX+cardW-splitX > radius {
		canvas.SetColor(defenderColor).FillRect(splitX, cardY+2, cardX+cardW-splitX-radius, stripH)
	}

	// 行 1：节点（左）+ 进度（右）
	node := inv.Node
	if node == "" {
		node = "未知节点"
	}
	canvas.SetColor(textColor).SetFontSize(nodeSize)
	canvas.AddText(node, innerX, cardY+40)
	pct := fmt.Sprintf("进度: %.1f%%", progress*100)
	canvas.SetColor(titleColor).SetFontSize(progressSize)
	canvas.AddText(pct, innerX+innerW-canvas.StringWidth(pct), cardY+42)

	// 行 2：阵营对抗（居中）
	row2Y := cardY + 88
	atkName, atkCol, atkIcon := factionSideInfo(inv.Faction)
	defName, defCol, defIcon := factionSideInfo(inv.DefenderFaction)
	sep := "  VS  "

	canvas.SetFontSize(factionSize)
	atkW := canvas.StringWidth(atkName)
	defW := canvas.StringWidth(defName)
	sepW := canvas.StringWidth(sep)
	atkIconW, defIconW := 0.0, 0.0
	if atkIcon != "" {
		canvas.SetIconFontSize(32)
		atkIconW = canvas.StringWidth(atkIcon) + 4
		canvas.SetFontSize(factionSize)
	}
	if defIcon != "" {
		canvas.SetIconFontSize(32)
		defIconW = canvas.StringWidth(defIcon) + 4
		canvas.SetFontSize(factionSize)
	}
	totalW := atkIconW + atkW + sepW + defIconW + defW
	curX := innerX + (innerW-totalW)/2

	if atkIcon != "" {
		canvas.SetColor(atkCol).SetIconFontSize(32)
		canvas.AddText(atkIcon, curX, row2Y)
		curX += atkIconW
	}
	canvas.SetColor(atkCol).SetFontSize(factionSize)
	canvas.AddText(atkName, curX, row2Y+3)
	curX += atkW
	canvas.SetColor(textMutedColor).SetFontSize(factionSize)
	canvas.AddText(sep, curX, row2Y+3)
	curX += sepW
	if defIcon != "" {
		canvas.SetColor(defCol).SetIconFontSize(32)
		canvas.AddText(defIcon, curX, row2Y)
		curX += defIconW
	}
	canvas.SetColor(defCol).SetFontSize(factionSize)
	canvas.AddText(defName, curX, row2Y+3)

	// 行 3：奖励（进攻左 / 防守右对齐）
	rewardY := cardY + 140
	atkReward := firstRewardText(inv.AttackerReward)
	if atkReward != "" {
		canvas.SetColor(attackerColor).SetFontSize(rewardSize)
		canvas.AddText("进攻: "+atkReward, innerX, rewardY+3)
	}
	defReward := ""
	if inv.DefenderReward != nil && len(inv.DefenderReward.CountedItems) > 0 {
		defReward = rewardItemText(inv.DefenderReward.CountedItems[0])
	}
	if defReward != "" {
		canvas.SetColor(defenderColor).SetFontSize(rewardSize)
		labelW := canvas.StringWidth("防守: " + defReward)
		canvas.AddText("防守: "+defReward, innerX+innerW-labelW, rewardY+3)
	}
}

// firstRewardText 取奖励列表中第一个带数量物品的文本（对齐 Java getFirstRewardText）。
func firstRewardText(rewards []*Reward) string {
	for _, rw := range rewards {
		if rw != nil && len(rw.CountedItems) > 0 {
			return rewardItemText(rw.CountedItems[0])
		}
	}
	return ""
}

// rewardItemText 格式化带数量奖励为 "Nx 名称"（对齐 Java 内联拼接逻辑，缺失值显示 ?）。
func rewardItemText(item *RewardItem) string {
	if item == nil {
		return ""
	}
	count := "?"
	if item.Count != nil {
		count = fmt.Sprintf("%d", *item.Count)
	}
	name := item.Name
	if name == "" {
		name = "?"
	}
	return count + "x " + name
}

// factionSideInfo 阵营显示信息（对齐 Java 空阵营回退：名称"未知"、弱化灰、无图标）。
func factionSideInfo(key drawplugin.Faction) (string, color.RGBA, string) {
	info, ok := factionKeyInfo(key)
	if !ok {
		return "未知", textMutedColor, ""
	}
	return info.Name, hexToColor(info.Color), info.Icon
}
