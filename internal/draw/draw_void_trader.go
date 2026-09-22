// 虚空商人图片：单列表格布局（标题 + 位置/时间信息 + 物品/杜卡/星币三列）
// 对齐 Java DefaultDrawVoidTraderImage / model.VoidTrader
package draw

import (
	"image/color"
	"strconv"
	"time"

	"nyxbot-go/internal/enum/drawplugin"
)

// VoidTrader 虚空商人绘图输入（对齐 Java model.VoidTrader 原始字段）。
// Node 与 Manifest[].Item 均为已翻译后的中文名。
type VoidTrader struct {
	ID        string            // 唯一标识
	Character string            // 商人名称
	Node      string            // 出现位置（已翻译）
	Expiry    time.Time         // 结束时间
	Manifest  []*VoidTraderItem // 商品清单
}

// VoidTraderItem 虚空商人单个商品（对齐 Java VoidTrader.Manifest）。
type VoidTraderItem struct {
	Item         string // 物品名称（已翻译）
	PrimePrice   *int   // 杜卡币价格
	RegularPrice *int   // 星币价格
	Limit        *int   // 限购数量
}

// voidTraderLocationColor 出现位置颜色（对齐 Java LOCATION_COLOR 0x9B59B6）。
var voidTraderLocationColor = color.RGBA{0x9b, 0x59, 0xb6, 0xff}

// DrawVoidTrader 绘制虚空商人列表图（对齐 Java drawVoidTraderImage）。
func DrawVoidTrader(voidTraders []*VoidTrader) []byte {
	const canvasW = 1400
	const minHeight = 800

	// 过滤空条目并统计商品总数
	list := make([]*VoidTrader, 0, len(voidTraders))
	totalItems := 0
	for _, vt := range voidTraders {
		if vt == nil {
			continue
		}
		list = append(list, vt)
		totalItems += len(vt.Manifest)
	}
	if len(list) == 0 {
		return nil
	}

	// 计算图像高度（对齐 Java calculateImageHeight）
	height := calculateVoidTraderHeight(len(list), totalItems)
	if height < minHeight {
		height = minHeight
	}

	canvas := NewCanvas(canvasW, height)
	canvas.SetColor(pageBackgroundColor).FillRect(0, 0, float64(canvasW), float64(height))
	canvas.DrawTooRoundRect()

	// 标题
	canvas.SetColor(titleColor).SetFontSize(40)
	canvas.AddCenteredText("虚空商人", 80)

	// 绘制虚空商人列表
	startY := 130
	for _, vt := range list {
		startY = drawVoidTraderSection(canvas, vt, startY)
	}

	// 看板娘 + 底部署名
	szW, szH := scaleByPct(canvasW, canvasW, standardRatio)
	canvas.DrawStandingAt(canvasW-float64(szW), float64(height)-float64(szH), float64(szW), float64(szH))
	canvas.AddFooter(float64(height) - 25)
	data, err := canvas.PNG()
	if err != nil {
		return nil
	}
	return data
}

// calculateVoidTraderHeight 计算虚空商人图像高度（对齐 Java calculateImageHeight）。
func calculateVoidTraderHeight(traderCount, itemCount int) int {
	headerHeight := 140 // 标题/顶部区域高度
	footerHeight := imageFooterH + 50
	traderInfoHeight := traderCount * 80 // 每个商人信息区域
	tableHeaderHeight := traderCount * 60
	itemsHeight := itemCount * 45
	return headerHeight + traderInfoHeight + tableHeaderHeight + itemsHeight + footerHeight
}

// drawVoidTraderSection 绘制单个虚空商人区块（信息 + 商品清单），返回下一区块起始 Y。
func drawVoidTraderSection(canvas *Canvas, vt *VoidTrader, startY int) int {
	currentY := drawTraderInfo(canvas, vt, startY)
	if len(vt.Manifest) > 0 {
		currentY = drawManifestTable(canvas, vt.Manifest, currentY)
	}
	return currentY + 30
}

// drawTraderInfo 绘制虚空商人基本信息（出现位置 + 剩余时间），返回下一行 Y。
func drawTraderInfo(canvas *Canvas, vt *VoidTrader, startY int) int {
	location := vt.Node
	if location == "" {
		location = "未知位置"
	}
	canvas.SetColor(voidTraderLocationColor).SetFontSize(28)
	canvas.AddText("位置: "+location, imageMargin+200, float64(startY))

	timeLeft := timeDeltaString(time.Until(vt.Expiry))
	if timeLeft == "" {
		timeLeft = "未知"
	}
	canvas.SetColor(textColor).SetFontSize(28)
	canvas.AddText("剩余时间: "+timeLeft, imageMargin+640, float64(startY))

	return startY + 50
}

// drawManifestTable 绘制商品清单表格（表头 + 各行），返回表格结束 Y。
func drawManifestTable(canvas *Canvas, manifest []*VoidTraderItem, startY int) int {
	const (
		tableX     = imageMargin // 40
		tableWidth = 1400 - 2*imageMargin
	)
	colWidths := [3]int{600, 200, 200}

	// 表头背景
	canvas.SetColor(cardBackgroundColor).FillRect(float64(tableX), float64(startY), float64(tableWidth), 60)

	// 表头文字
	headers := [3]string{"物品名称", "杜卡币价格", "星币价格"}
	canvas.SetColor(textColor).SetFontSize(26)
	headerX := tableX
	for i := 0; i < 3; i++ {
		canvas.AddText(headers[i], float64(headerX+20), float64(startY+30))
		headerX += colWidths[i]
	}

	currentY := startY + 60
	for _, item := range manifest {
		drawManifestRow(canvas, item, currentY, tableX, colWidths)
		currentY += 45
	}
	return currentY
}

// drawManifestRow 绘制单个商品行（名称 + 杜卡币 + 星币）。
func drawManifestRow(canvas *Canvas, item *VoidTraderItem, rowY, startX int, colWidths [3]int) {
	const rowH = 45
	// 物品名称
	name := item.Item
	if name == "" {
		name = "未知物品"
	}
	canvas.SetColor(titleColor).SetFontSize(26)
	canvas.AddText(name, float64(startX+20), float64(rowY+rowH/2+6))

	// 杜卡币价格（图标 + 数值）
	ducatsPrice := "0"
	if item.PrimePrice != nil {
		ducatsPrice = strconv.Itoa(*item.PrimePrice)
	}
	canvas.SetColor(accentGoldColor).SetIconFontSize(24)
	canvas.AddText(drawplugin.IconMap[drawplugin.IconDucats], float64(startX+colWidths[0]+20), float64(rowY+rowH/2+6))
	canvas.SetFontSize(26)
	canvas.AddText(ducatsPrice, float64(startX+colWidths[0]+60), float64(rowY+rowH/2+6))

	// 星币价格（图标 + 千分位 "K"）
	regularText := "0"
	if item.RegularPrice != nil {
		regularText = strconv.Itoa(*item.RegularPrice/1000) + "K"
	}
	canvas.SetColor(titleColor).SetIconFontSize(24)
	canvas.AddText(drawplugin.IconMap[drawplugin.IconCredits], float64(startX+colWidths[0]+colWidths[1]+20), float64(rowY+rowH/2+6))
	canvas.SetFontSize(26)
	canvas.AddText(regularText, float64(startX+colWidths[0]+colWidths[1]+60), float64(rowY+rowH/2+6))
}
