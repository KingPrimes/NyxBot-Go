// 市场玄骸/姐妹拍卖卡片图：三列卡片网格 + 看板娘，列流式布局
// 对齐 Java DefaultDrawMarketLichSisterImage（Liches 与 Sister 两个入口共用同一渲染实现）
package draw

import (
	"fmt"
	"image/color"
	"strconv"
	"time"

	"nyxbot-go/internal/enum/drawplugin"
)

// MarketLichSister 市场玄骸/姐妹拍卖查询结果（对齐 Java model.market.MarketLichSister 原始字段）。
type MarketLichSister struct {
	Payload *MarketLichSisterPayload `json:"payload"` // 响应数据载荷
}

// MarketLichSisterPayload 拍卖载荷（对齐 Java MarketLichSister.Payload 内部类，展开为独立结构）。
type MarketLichSisterPayload struct {
	Auctions []*MarketLichSisterAuction `json:"auctions"`  // 拍卖列表
	ItemName string                     `json:"item_name"` // 拍卖物品名称
}

// MarketLichSisterAuction 单条拍卖信息（对齐 Java MarketLichSister.Auctions 内部类，展开为独立结构）。
type MarketLichSisterAuction struct {
	BuyoutPrice       *int                   `json:"buyout_price"`        // 买断价格（nil 表示无买断价）
	Note              string                 `json:"note"`                // 备注信息
	Visible           *bool                  `json:"visible"`             // 是否可见
	Item              *MarketLichSisterItem  `json:"item"`                // 物品信息
	StartingPrice     *int                   `json:"starting_price"`      // 起拍价
	MinimalReputation *int                   `json:"minimal_reputation"`  // 参与竞拍所需最低声望
	Owner             *MarketLichSisterOwner `json:"owner"`               // 卖家信息
	Platform          string                 `json:"platform"`            // 平台（PC/PS/Xbox 等）
	Closed            *bool                  `json:"closed"`              // 是否已关闭
	TopBid            *int                   `json:"top_bid"`             // 最高出价
	Winner            any                    `json:"winner"`              // 赢家信息（竞拍中为 nil）
	IsMarkedFor       any                    `json:"is_marked_for"`       // 标记信息
	MarkedOperationAt any                    `json:"marked_operation_at"` // 标记操作时间
	Created           time.Time              `json:"created"`             // 创建时间
	Updated           time.Time              `json:"updated"`             // 更新时间
	NoteRaw           string                 `json:"note_raw"`            // 原始备注
	IsDirectSell      *bool                  `json:"is_direct_sell"`      // 是否直接出售
	ID                string                 `json:"id"`                  // 拍卖 ID
	Private           *bool                  `json:"private"`             // 是否为私人拍卖
}

// MarketLichSisterItem 拍卖物品信息（对齐 Java MarketLichSister.Item 内部类，展开为独立结构）。
type MarketLichSisterItem struct {
	Type           string             `json:"type"`            // 物品类型
	Damage         *int               `json:"damage"`          // 武器伤害加成数值
	WeaponURLName  string             `json:"weapon_url_name"` // 武器 URL 名称
	HavingEphemera *bool              `json:"having_ephemera"` // 是否携带幻纹
	Element        drawplugin.Element `json:"element"`         // 元素属性
}

// MarketLichSisterOwner 卖家信息（对齐 Java model.market.Owner 原始字段）。
type MarketLichSisterOwner struct {
	Reputation *int      `json:"reputation"`  // 声望
	Locale     string    `json:"locale"`      // 区服
	Avatar     string    `json:"avatar"`      // 玩家头像
	LastSeen   time.Time `json:"last_seen"`   // 上次登录时间
	IngameName string    `json:"ingame_name"` // 游戏内名称
	Status     string    `json:"status"`      // 用户状态
	ID         string    `json:"id"`          // 用户 ID
	Region     string    `json:"region"`      // 语言地区
}

// elementColorMap 元素主题色（对齐 Java ElementEnum.COLOR 十六进制值）。
// 元素名称与图标复用 enum/drawplugin 的 ElementMap；ElementInfo 未含 Color 字段，故在此补齐颜色映射。
var elementColorMap = map[drawplugin.Element]color.RGBA{
	drawplugin.ElemElectricity: {0x95, 0x5a, 0xb9, 0xff}, // 电击
	drawplugin.ElemImpact:      {0x92, 0x95, 0x95, 0xff}, // 冲击
	drawplugin.ElemRadiation:   {0x9e, 0x94, 0x0c, 0xff}, // 辐射
	drawplugin.ElemMagnetic:    {0x50, 0x57, 0x5e, 0xff}, // 磁力
	drawplugin.ElemCold:        {0x2c, 0x53, 0xb5, 0xff}, // 冰冻
	drawplugin.ElemToxin:       {0x4f, 0x82, 0x2f, 0xff}, // 毒素
	drawplugin.ElemHeat:        {0xe5, 0x6e, 0x0c, 0xff}, // 火焰
	drawplugin.ElemPuncture:    {0x8d, 0x86, 0x76, 0xff}, // 穿刺
	drawplugin.ElemSlash:       {0x87, 0x67, 0x69, 0xff}, // 切割
	drawplugin.ElemBlast:       {0xa8, 0x66, 0x68, 0xff}, // 爆炸
	drawplugin.ElemCorrosive:   {0xb2, 0xca, 0x01, 0xff}, // 腐蚀
	drawplugin.ElemGas:         {0x3a, 0xc4, 0x90, 0xff}, // 气体
	drawplugin.ElemViral:       {0xe3, 0x74, 0xb6, 0xff}, // 病毒
	drawplugin.ElemVoid:        {0x01, 0x64, 0x68, 0xff}, // 虚空
	drawplugin.ElemTau:         {0xdf, 0x19, 0x1e, 0xff}, // TAU
	drawplugin.ElemTrue:        {0x66, 0x58, 0x2e, 0xff}, // TRUE
}

// ephemeraColor 幻纹标记颜色（对齐 Java drawCard 中 new Color(0x9B59B6)）。
var ephemeraColor = color.RGBA{0x9b, 0x59, 0xb6, 0xff}

// elementColor 返回元素主题色；未知元素回退主文本色（对齐 Java ElementEnum.getCOLOR 的 null 回退 TEXT_COLOR）。
func elementColor(e drawplugin.Element) color.RGBA {
	if c, ok := elementColorMap[e]; ok {
		return c
	}
	return textColor
}

// startingText 起拍价文本（对齐 Java "起拍 " + startingPrice 拼接）。
func (a *MarketLichSisterAuction) startingText() string {
	if a.StartingPrice != nil {
		return "起拍 " + strconv.Itoa(*a.StartingPrice)
	}
	return "起拍 -"
}

// buyoutText 买断价文本；无买断价返回 "-"。
func (a *MarketLichSisterAuction) buyoutText() string {
	if a.BuyoutPrice != nil {
		return strconv.Itoa(*a.BuyoutPrice)
	}
	return "-"
}

// damageText 伤害加成文本；物品或加成缺失时返回 "伤害 未知"。
func (a *MarketLichSisterAuction) damageText() string {
	if a.Item != nil && a.Item.Damage != nil {
		return fmt.Sprintf("伤害 +%d%%", *a.Item.Damage)
	}
	return "伤害 未知"
}

// havingEphemera 是否携带幻纹。
func (a *MarketLichSisterAuction) havingEphemera() bool {
	return a.Item != nil && a.Item.HavingEphemera != nil && *a.Item.HavingEphemera
}

// sellerName 卖家游戏内名称；缺失时返回 "未知"。
func (a *MarketLichSisterAuction) sellerName() string {
	if a.Owner != nil && a.Owner.IngameName != "" {
		return a.Owner.IngameName
	}
	return "未知"
}

// reputationText 卖家声望文本；缺失时返回 "声望 -"。
func (a *MarketLichSisterAuction) reputationText() string {
	if a.Owner != nil && a.Owner.Reputation != nil {
		return "声望 " + strconv.Itoa(*a.Owner.Reputation)
	}
	return "声望 -"
}

// DrawMarketLichSister 绘制玄骸/姐妹拍卖卡片网格图（Java 的 Liches/Sister 两个入口共用；空输入返回 nil）。
func DrawMarketLichSister(m *MarketLichSister) []byte {
	if m == nil || m.Payload == nil || len(m.Payload.Auctions) == 0 {
		return nil
	}
	const (
		contentX      float64 = 50  // 内容区左边距
		cols          int     = 3   // 列数
		colGap        float64 = 20  // 列间距
		cardRad       float64 = 14  // 卡片圆角
		cardPad       float64 = 20  // 卡片内边距
		titleY        float64 = 80  // 标题基线 Y
		contentStartY float64 = 160 // 内容区起始 Y
		cardW         float64 = 562 // 卡片宽度
	)
	textW := cardW - cardPad*2
	canvasW := contentX + float64(cols)*cardW + float64(cols-1)*colGap + contentX

	n := len(m.Payload.Auctions)
	isOdd := n%cols != 0

	// 列 X 起点（对齐 Java colX 数组）
	colX := make([]float64, cols)
	for c := 0; c < cols; c++ {
		colX[c] = contentX + float64(c)*(cardW+colGap)
	}

	// 卡片高度固定（对齐 Java calcCardHeight 无参版本）
	cardH := lichSisterCardHeight(cardPad)

	// 列流式 Y 终点（对齐 Java 逐列累加后回退一个列间距）
	colEndY := make([]float64, cols)
	for c := range colEndY {
		colEndY[c] = contentStartY
	}
	for i := 0; i < n; i++ {
		colEndY[i%cols] += cardH + colGap
	}
	for c := 0; c < cols; c++ {
		if colEndY[c] > contentStartY {
			colEndY[c] -= colGap
		}
	}

	// 画布总高：看板娘占用右列底部区域（奇数时与左中列共享垂直空间）
	var totalHeight float64
	if isOdd {
		tallerEnd := maxF(colEndY[0], colEndY[1])
		totalHeight = maxF(tallerEnd, colEndY[cols-1]+cardW)
	} else {
		maxEnd := contentStartY
		for c := 0; c < cols; c++ {
			maxEnd = maxF(maxEnd, colEndY[c])
		}
		totalHeight = maxEnd + 10 + cardW
	}

	canvas := NewCanvas(int(canvasW), int(totalHeight))
	canvas.SetColor(pageBackgroundColor).FillRect(0, 0, canvasW, totalHeight)
	canvas.DrawTooRoundRect()

	// 标题：物品名 + " 拍卖信息"
	title := "拍卖信息"
	if m.Payload.ItemName != "" {
		title = m.Payload.ItemName + " 拍卖信息"
	}
	canvas.SetColor(titleColor).SetFontSize(44)
	canvas.AddCenteredText(title, titleY)
	canvas.SetColor(dividerColor).DrawLine(contentX, titleY+50, canvasW-contentX, titleY+50)

	// 列流式绘制卡片
	drawY := make([]float64, cols)
	for c := range drawY {
		drawY[c] = contentStartY
	}
	for i, a := range m.Payload.Auctions {
		col := i % cols
		drawLichSisterCard(canvas, a, colX[col], drawY[col], cardW, cardH, textW, cardRad, cardPad)
		drawY[col] += cardH + colGap
	}

	// 看板娘 + 底部署名
	standingX := colX[cols-1]
	standingY := totalHeight - cardW
	if isOdd {
		standingY = colEndY[cols-1]
	}
	canvas.DrawStandingAt(standingX, standingY, cardW, cardW)
	canvas.AddFooter(totalHeight - 25)

	data, err := canvas.PNG()
	if err != nil {
		return nil
	}
	return data
}

// lichSisterCardHeight 玄骸/姐妹卡片高度（对齐 Java calcCardHeight 固定行高累加，下限 200）。
func lichSisterCardHeight(pad float64) float64 {
	h := pad + 42 + 8 + 30 + 8 + 30 + 28 + 28 + pad
	return maxF(h, 200)
}

// drawLichSisterCard 绘制单张玄骸/姐妹拍卖卡片（对齐 Java drawCard 六行布局）。
func drawLichSisterCard(canvas *Canvas, a *MarketLichSisterAuction, cardX, cardY, cardW, cardH, textW, rad, pad float64) {
	innerX := cardX + pad
	rightX := innerX + textW

	// 元素决定卡片强调色；未知元素回退主文本色
	var element drawplugin.Element
	if a.Item != nil {
		element = a.Item.Element
	}
	accent := elementColor(element)
	info, known := drawplugin.ElementMap[element]

	// 卡片背景 + 顶部元素色条 + 边框
	canvas.SetColor(cardBackgroundColor).FillRoundRect(cardX, cardY, cardW, cardH, rad)
	canvas.SetColor(accent).FillRect(cardX+rad, cardY+2, cardW-rad*2, 5)
	canvas.SetColor(dividerColor).SetStroke(1).
		DrawRoundRect(cardX, cardY, cardW, cardH, rad)

	cy := cardY + pad

	// 元素图标 + 名称
	if known {
		canvas.SetColor(accent).SetIconFontSize(28)
		canvas.AddText(info.Icon, innerX, cy+28)
		canvas.SetColor(accent).SetFontSize(26)
		canvas.AddText(info.Name, innerX+36, cy+28)
	} else {
		canvas.SetColor(textColor).SetFontSize(26)
		canvas.AddText("未知元素", innerX, cy+28)
	}
	cy += 42

	// 分隔线
	canvas.SetColor(dividerColor).DrawLine(innerX, cy+4, rightX, cy+4)
	cy += 12

	// 伤害加成 + 幻纹标记
	damage := a.damageText()
	canvas.SetColor(textColor).SetFontSize(20)
	canvas.AddText(damage, innerX, cy+20)
	if a.havingEphemera() {
		canvas.SetColor(ephemeraColor).SetFontSize(18)
		dw := canvas.StringWidth(damage) // 当前正文字体测量，对齐 Java getFontMetrics(bodyFont)
		canvas.AddText("[幻纹]", innerX+dw+12, cy+20)
	}
	cy += 30

	// 分隔线
	canvas.SetColor(dividerColor).DrawLine(innerX, cy+4, rightX, cy+4)
	cy += 12

	// 起拍价（左） + 买断价（右，白金图标 + 金色数字）
	canvas.SetColor(textSecondaryColor).SetFontSize(20)
	canvas.AddText(a.startingText(), innerX, cy+20)

	buyout := a.buyoutText()
	canvas.SetColor(accentGoldColor).SetIconFontSize(28)
	priceIcon := drawplugin.IconMap[drawplugin.IconPlatinum]
	piW := canvas.StringWidth(priceIcon)
	canvas.SetFontSize(20)
	buyW := canvas.StringWidth(buyout)
	canvas.AddText(priceIcon, rightX-piW-buyW-4, cy+22)
	canvas.SetColor(accentGoldColor)
	canvas.AddText(buyout, rightX-buyW, cy+22)
	cy += 30

	// 卖家（TENNO 图标 + 游戏内名称）
	canvas.SetColor(titleColor).SetIconFontSize(28)
	canvas.AddText(drawplugin.FactionMap[drawplugin.FactionTenno].Icon, innerX, cy+22)
	canvas.SetColor(textColor).SetFontSize(20)
	canvas.AddText(a.sellerName(), innerX+30, cy+20)
	cy += 28

	// 声望
	canvas.SetColor(textSecondaryColor).SetFontSize(18)
	canvas.AddText(a.reputationText(), innerX, cy+20)
}
