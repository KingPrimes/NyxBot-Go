// 市场紫卡图片：紫卡拍卖三列卡片网格（列流式布局）+ 看板娘
// 对齐 Java DefaultDrawMarketRivenImage
package draw

import (
	"image/color"
	"math"
	"strconv"
	"strings"
	"time"

	"nyxbot-go/internal/enum/drawplugin"
)

// 紫卡图布局常量（对齐 DefaultDrawMarketRivenImage 私有常量；
// 前缀 Market 用于区分 draw_riven_analyse_trend.go 中的同名趋势图常量）。
const (
	rivenMarketContentX      float64 = 50  // 内容起始 X
	rivenMarketCols          int     = 3   // 卡片列数
	rivenMarketColGap        float64 = 20  // 列间距
	rivenMarketCardRadius    float64 = 14  // 卡片圆角
	rivenMarketCardPad       float64 = 20  // 卡片内边距
	rivenMarketTitleY        float64 = 80  // 标题 Y
	rivenMarketContentStartY float64 = 150 // 内容起始 Y
	rivenMarketCardW         float64 = 562 // 卡片宽度
)

// 紫卡图局部颜色（对齐 Java 私有常量）。
var (
	rivenColor         = color.RGBA{0x86, 0x69, 0xa7, 0xff} // 紫卡名称 — 紫色
	rivenPositiveColor = color.RGBA{0x3b, 0x9a, 0x21, 0xff} // 正向词条 — 绿色
	rivenNegativeColor = color.RGBA{0xac, 0x18, 0x18, 0xff} // 负向词条 — 红色
)

// 元素颜色复用 draw_market_lich_sister.go 的 elementColorMap（同 Java ElementEnum.COLOR 映射）。

// rivenElementOrder 元素匹配顺序（对齐 Java ElementEnum 声明顺序）。
var rivenElementOrder = []drawplugin.Element{
	drawplugin.ElemElectricity,
	drawplugin.ElemImpact,
	drawplugin.ElemRadiation,
	drawplugin.ElemMagnetic,
	drawplugin.ElemCold,
	drawplugin.ElemToxin,
	drawplugin.ElemHeat,
	drawplugin.ElemPuncture,
	drawplugin.ElemSlash,
	drawplugin.ElemBlast,
	drawplugin.ElemCorrosive,
	drawplugin.ElemGas,
	drawplugin.ElemViral,
	drawplugin.ElemVoid,
	drawplugin.ElemTau,
	drawplugin.ElemTrue,
}

// MarketRiven 市场紫卡拍卖数据（对齐 Java model.market.MarketRiven 原始字段）。
type MarketRiven struct {
	ItemName string              // 物品名称
	Payload  *MarketRivenPayload // 数据负载
}

// MarketRivenPayload 紫卡拍卖负载（对齐 Java MarketRiven.Payload 嵌套类展开）。
type MarketRivenPayload struct {
	Auctions []*MarketRivenAuction // 拍卖列表
}

// MarketRivenAuction 单条紫卡拍卖（对齐 Java MarketRiven.Auctions 嵌套类展开）。
type MarketRivenAuction struct {
	BuyoutPrice       *int              // 买断价格
	Note              string            // 备注
	Visible           *bool             // 是否可见
	Item              *MarketRivenItem  // 物品
	StartingPrice     *int              // 起拍价
	MinimalReputation *int              // 声望
	Owner             *MarketRivenOwner // 卖家/买家
	Platform          string            // 平台
	Closed            *bool             // 顶点是否关闭
	TopBid            *int              // 最高出价
	Winner            any               // 获胜者
	IsMarkedFor       any               // 标记对象
	MarkedOperationAt any               // 标记操作时间
	Created           time.Time         // 创建时间
	Updated           time.Time         // 修改时间
	NoteRaw           string            // 原始备注
	IsDirectSell      *bool             // 是否买断
	ID                string            // 订单ID
	Private           *bool             // 是否是私人
}

// MarketRivenItem 紫卡物品（对齐 Java MarketRiven.Item 嵌套类展开）。
type MarketRivenItem struct {
	Type          string                  // 物品类型
	ModRank       *int                    // Mod等级
	WeaponURLName string                  // 武器名称
	Attributes    []*MarketRivenAttribute // 紫卡词条
	Name          string                  // 紫卡名称
	ReRolls       *int                    // 紫卡循环次数
	Polarity      drawplugin.Polarity     // 紫卡极性
	MasteryLevel  *int                    // 段位限制
}

// MarketRivenAttribute 紫卡词条（对齐 Java MarketRiven.Attributes 嵌套类展开）。
type MarketRivenAttribute struct {
	Value    *float64 // 词条数值
	Positive *bool    // 是否是正向
	URLName  string   // 词条名称
}

// MarketRivenOwner 拍卖卖家信息（对齐 Java model.market.Owner 原始字段）。
type MarketRivenOwner struct {
	Reputation *int      // 声望
	Locale     string    // 区服
	Avatar     string    // 玩家头像
	LastSeen   time.Time // 上次登录时间
	IngameName string    // 游戏内名称
	Status     string    // 用户状态
	ID         string    // 用户ID
	Region     string    // 所使用的语言
}

// formatRivenDouble 格式化紫卡词条数值（对齐 Java String.valueOf(double)：
// 整数值保留 ".0" 后缀，其余输出最短十进制表示）。
func formatRivenDouble(v float64) string {
	if v == math.Trunc(v) && math.Abs(v) < 1e7 {
		return strconv.FormatFloat(v, 'f', 1, 64)
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// rivenPolarityIcon 返回紫卡极性图标（对齐 Java PolarityEnum.getIcon）。
// drawplugin.PolarityIconMap 的 Any 为空，此处按 Java 原始映射补齐 ""。
func rivenPolarityIcon(p drawplugin.Polarity) string {
	if p == drawplugin.PolarityAny {
		return ""
	}
	return drawplugin.PolarityIconMap[p]
}

// findRivenElement 按词条 url_name 匹配元素（对齐 Java findElement：
// 小写枚举名或中文名包含即命中，按声明顺序返回首个匹配；未命中返回 nil）。
func findRivenElement(urlName string) *drawplugin.Element {
	if urlName == "" {
		return nil
	}
	lower := strings.ToLower(urlName)
	for _, elem := range rivenElementOrder {
		if strings.Contains(lower, strings.ToLower(string(elem))) ||
			strings.Contains(lower, strings.ToLower(drawplugin.ElementMap[elem].Name)) {
			e := elem
			return &e
		}
	}
	return nil
}

// DrawMarketRiven 绘制市场紫卡拍卖图（payload 或拍卖列表为空返回 nil）。
func DrawMarketRiven(marketRiven *MarketRiven) []byte {
	if marketRiven == nil || marketRiven.Payload == nil || marketRiven.Payload.Auctions == nil {
		return nil
	}

	auctions := marketRiven.Payload.Auctions
	if len(auctions) == 0 {
		return nil
	}

	n := len(auctions)
	isOdd := n%rivenMarketCols != 0

	textW := rivenMarketCardW - rivenMarketCardPad*2
	canvasW := rivenMarketContentX + float64(rivenMarketCols)*rivenMarketCardW + float64(rivenMarketCols-1)*rivenMarketColGap + rivenMarketContentX

	colX := make([]float64, rivenMarketCols)
	for c := 0; c < rivenMarketCols; c++ {
		colX[c] = rivenMarketContentX + float64(c)*(rivenMarketCardW+rivenMarketColGap)
	}

	// 预计算卡片高度
	cardHeights := make([]float64, n)
	for i, a := range auctions {
		cardHeights[i] = rivenCardHeight(a)
	}

	// 列流式 Y 终点
	colEndY := make([]float64, rivenMarketCols)
	for c := range colEndY {
		colEndY[c] = rivenMarketContentStartY
	}
	for i, h := range cardHeights {
		colEndY[i%rivenMarketCols] += h + rivenMarketColGap
	}
	for c := 0; c < rivenMarketCols; c++ {
		if colEndY[c] > rivenMarketContentStartY {
			colEndY[c] -= rivenMarketColGap
		}
	}

	standingX := colX[rivenMarketCols-1]
	var totalHeight float64
	if isOdd {
		tallerEnd := maxF(colEndY[0], colEndY[1])
		totalHeight = maxF(tallerEnd, colEndY[rivenMarketCols-1]+rivenMarketCardW)
	} else {
		maxEnd := rivenMarketContentStartY
		for c := 0; c < rivenMarketCols; c++ {
			maxEnd = maxF(maxEnd, colEndY[c])
		}
		totalHeight = maxEnd + 10 + rivenMarketCardW
	}

	canvas := NewCanvas(int(canvasW), int(totalHeight))
	canvas.SetColor(pageBackgroundColor).FillRect(0, 0, canvasW, totalHeight)
	canvas.DrawTooRoundRect()

	canvas.SetColor(titleColor).SetFontSize(44)
	canvas.AddCenteredText("Warframe Market 紫卡市场", rivenMarketTitleY)
	canvas.SetColor(dividerColor).DrawLine(rivenMarketContentX, rivenMarketTitleY+50, canvasW-rivenMarketContentX, rivenMarketTitleY+50)

	itemName := marketRiven.ItemName

	// 列流式绘制
	drawY := make([]float64, rivenMarketCols)
	for c := range drawY {
		drawY[c] = rivenMarketContentStartY
	}
	for i, a := range auctions {
		col := i % rivenMarketCols
		drawRivenCard(canvas, a, itemName, colX[col], drawY[col], rivenMarketCardW, cardHeights[i], textW)
		drawY[col] += cardHeights[i] + rivenMarketColGap
	}

	standingY := totalHeight - rivenMarketCardW
	if isOdd {
		standingY = colEndY[rivenMarketCols-1]
	}
	canvas.DrawStandingAt(standingX, standingY, rivenMarketCardW, rivenMarketCardW)
	canvas.AddFooter(totalHeight - 25)
	data, err := canvas.PNG()
	if err != nil {
		return nil
	}
	return data
}

// rivenCardHeight 预计算紫卡卡片高度（对齐 Java calcCardHeight，最少 210）。
func rivenCardHeight(auction *MarketRivenAuction) float64 {
	h := rivenMarketCardPad
	h += 42 // 紫卡名称
	h += 8  // 分隔线
	h += 32 // 段位 + 循环
	h += 30 // MOD等级（数字）
	h += 8  // 分隔线
	if auction != nil && auction.Item != nil && auction.Item.Attributes != nil {
		h += float64(len(auction.Item.Attributes)) * 32
	}
	h += 8  // 分隔线
	h += 42 // 卖家 + 价格
	h += rivenMarketCardPad
	return maxF(h, 210)
}

// drawRivenCard 绘制单张紫卡卡片（对齐 Java drawCard）。
func drawRivenCard(canvas *Canvas, auction *MarketRivenAuction, itemName string, cardX, cardY, cardW, cardH, textW float64) {
	innerX := cardX + rivenMarketCardPad
	rightX := innerX + textW

	canvas.SetColor(cardBackgroundColor).FillRoundRect(cardX, cardY, cardW, cardH, rivenMarketCardRadius)
	canvas.SetColor(dividerColor).SetStroke(1).DrawRoundRect(cardX, cardY, cardW, cardH, rivenMarketCardRadius)

	if auction == nil || auction.Item == nil {
		return
	}
	item := auction.Item

	cy := cardY + rivenMarketCardPad

	// 紫卡名称 + 极性图标
	name := itemName
	if name == "" {
		name = item.Name
	}
	if name == "" {
		name = "未知紫卡"
	}
	canvas.SetColor(rivenColor).SetFontSize(22)
	canvas.AddText(name, innerX, cy+24)
	if item.Polarity != "" {
		canvas.SetColor(rivenColor).SetIconFontSize(28)
		pIcon := rivenPolarityIcon(item.Polarity)
		pW := canvas.StringWidth(pIcon)
		canvas.AddText(pIcon, rightX-pW, cy+26)
	}
	cy += 42

	// 分隔线
	canvas.SetColor(dividerColor).DrawLine(innerX, cy+4, rightX, cy+4)
	cy += 12

	// 段位 + 循环次数
	mr := "段位 -"
	if item.MasteryLevel != nil {
		mr = "段位 " + strconv.Itoa(*item.MasteryLevel)
	}
	reRolls := "-"
	if item.ReRolls != nil {
		reRolls = strconv.Itoa(*item.ReRolls)
	}
	canvas.SetColor(textColor).SetFontSize(18)
	canvas.AddText(mr, innerX, cy+20)

	// MOD等级 — 纯数字
	rank := "等级 -"
	if item.ModRank != nil {
		rank = "等级 " + strconv.Itoa(*item.ModRank)
	}
	canvas.SetColor(accentGoldColor)
	canvas.AddText(rank, innerX+textW/3, cy+20)

	canvas.SetColor(textSecondaryColor)
	rrText := "循环 " + reRolls
	rrW := canvas.StringWidth(rrText)
	canvas.AddText(rrText, rightX-rrW, cy+20)
	cy += 32

	// 分隔线
	canvas.SetColor(dividerColor).DrawLine(innerX, cy+4, rightX, cy+4)
	cy += 12

	// 属性词条（带元素图标，横向居中）
	if item.Attributes != nil {
		for _, attr := range item.Attributes {
			if attr == nil {
				cy += 32
				continue
			}
			positive := attr.Positive != nil && *attr.Positive
			attrColor := rivenNegativeColor
			prefix := ""
			if positive {
				attrColor = rivenPositiveColor
				prefix = "+"
			}
			val := "?"
			if attr.Value != nil {
				val = formatRivenDouble(*attr.Value)
			}
			attrName := attr.URLName
			if attrName == "" {
				attrName = "?"
			}

			elem := findRivenElement(attrName)
			icon := ""
			iconColor := rivenColor
			hasIcon := false
			if elem != nil {
				icon = drawplugin.ElementMap[*elem].Icon
				if c, ok := elementColorMap[*elem]; ok {
					iconColor = c
					hasIcon = true
				}
			}

			line := prefix + val + "%  " + attrName
			iconW := 0.0
			if icon != "" {
				iconW = canvas.StringWidth(icon) + 6
			}
			lineW := canvas.StringWidth(line)
			totalW := iconW + lineW
			centerX := cardX + (cardW-totalW)/2

			if icon != "" && hasIcon {
				canvas.SetColor(iconColor).SetIconFontSize(22)
				canvas.AddText(icon, centerX, cy+22)
			}
			canvas.SetColor(attrColor).SetFontSize(18)
			canvas.AddText(line, centerX+iconW, cy+20)
			cy += 32
		}
	}

	// 分隔线
	canvas.SetColor(dividerColor).DrawLine(innerX, cy+4, rightX, cy+4)
	cy += 12

	// 卖家 + 买断价
	seller := "未知"
	if auction.Owner != nil && auction.Owner.IngameName != "" {
		seller = auction.Owner.IngameName
	}
	price := "-"
	if auction.BuyoutPrice != nil {
		price = strconv.Itoa(*auction.BuyoutPrice)
	}

	canvas.SetColor(titleColor).SetIconFontSize(24)
	canvas.AddText(drawplugin.FactionMap[drawplugin.FactionTenno].Icon, innerX, cy+22)
	canvas.SetColor(titleColor).SetFontSize(18)
	canvas.AddText(seller, innerX+28, cy+22)

	priceText := price + " "
	priceW := canvas.StringWidth(priceText)
	canvas.SetColor(accentGoldColor).SetIconFontSize(24)
	canvas.AddText(drawplugin.IconMap[drawplugin.IconPlatinum], rightX-priceW-32, cy+22)
	canvas.SetColor(accentGoldColor).SetFontSize(18)
	canvas.AddText(priceText, rightX-priceW, cy+22)
}
