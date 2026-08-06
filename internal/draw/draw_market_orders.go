// 市场订单图片：订单详情表（标题 + 平台/买卖/杜卡币/交易税信息区 + 订单数据表）与可能要查询的物品列表
// 对齐 Java DefaultDrawMarketOrdersImage
package draw

import (
	"image"
	"image/color"
	"strconv"
	"strings"
	"time"

	"nyxbot-go/internal/enum/drawplugin"
)

// 市场订单图布局常量（对齐 DefaultDrawMarketOrdersImage 私有常量）。
const (
	marketOrdersImageWidth  float64 = 1550 // 图像宽度
	marketOrdersImageMargin float64 = 40   // 图像边距
	marketOrdersTitleH      float64 = 60   // 标题高度
	marketOrdersHeaderH     float64 = 50   // 表头高度
	marketOrdersRowH        float64 = 60   // 数据行高度
	marketOrdersFooterH     float64 = 40   // 底部高度
)

// 市场订单图局部颜色（对齐 Java 私有常量；未激活按钮背景复用 cardBackgroundColor，不可见状态复用 textMutedColor）。
var (
	marketPlatformBgColor = color.RGBA{0x6c, 0x5c, 0xe7, 0xff} // 平台标签背景 — 紫色
	marketActiveBgColor   = color.RGBA{0x27, 0xae, 0x60, 0xff} // 激活按钮背景 — 绿色
	marketPriceColor      = color.RGBA{0xe8, 0xd5, 0xa3, 0xff} // 价格文本 — 浅金
	marketOnlineColor     = color.RGBA{0x27, 0xae, 0x60, 0xff} // 状态：在线 — 绿
	marketIngameColor     = color.RGBA{0x34, 0x98, 0xdb, 0xff} // 状态：游戏中 — 蓝
	marketOfflineColor    = color.RGBA{0xe7, 0x4c, 0x3c, 0xff} // 状态：离线 — 红
)

// MarketPlatform 市场平台（对齐 Java MarketPlatformEnum）。
// drawplugin 枚举包暂无平台枚举，此处按 Java 原始定义补齐。
type MarketPlatform string

const (
	MarketPlatformPC     MarketPlatform = "PC"     // PC 平台
	MarketPlatformPS4    MarketPlatform = "PS4"    // PS4 平台
	MarketPlatformXbox   MarketPlatform = "XBOX"   // Xbox 平台
	MarketPlatformSwitch MarketPlatform = "SWITCH" // Switch 平台
	MarketPlatformMobile MarketPlatform = "MOBILE" // 移动端平台
)

// name 返回平台显示名（大写枚举名，对齐 Java name()；兼容小写输入）。
func (p MarketPlatform) name() string {
	return strings.ToUpper(string(p))
}

// Orders 市场订单绘图输入（对齐 Java model.market.Orders 原始字段）。
type Orders struct {
	Name           string           // 物品名称
	Form           MarketPlatform   // 平台类型
	IsBy           *bool            // true: 购买 false: 出售
	IsMax          *bool            // true: 满级 false: 未满级
	Ducats         *int             // 物品价值杜卡币
	Vaulted        *bool            // 遗物是否入库
	MaxAmberStars  *int             // 阿耶檀识 黄星星
	MaxCyanStars   *int             // 阿耶檀识 蓝星星
	BaseEndo       *int             // 阿耶檀识 基础 内融核心
	ReqMasteryRank *int             // 段位等级限制
	TradingTax     *int             // 交易税
	Icon           image.Image      // 物品图标（可为 nil，为 nil 时不绘制）
	Orders         []*OrderWithUser // 订单列表
}

// OrderWithUser 市场订单行数据（对齐 Java model.market.OrderWithUser 原始字段）。
type OrderWithUser struct {
	ID         string                     // 订单ID
	Type       drawplugin.TransactionType // 交易类型
	Platinum   *int                       // 订单价格
	Quantity   *int                       // 订单数量
	PerTrade   *int                       // 单价
	Rank       *int                       // 等级
	Charges    *int                       // 槽位
	Subtype    string                     // 子类型
	AmberStars *int                       // 黄星数
	CyanStars  *int                       // 蓝星数
	Vosfor     *int                       // 阿耶檀识宝物分解后获得的内融核心数
	Visible    *bool                      // 订单是否可见
	CreatedAt  time.Time                  // 创建时间
	UpdatedAt  time.Time                  // 修改时间
	ItemID     string                     // 物品ID
	User       *MarketUser                // 订单用户信息
}

// MarketUser 市场用户信息（对齐 Java model.market.MarketUser 原始字段）。
type MarketUser struct {
	ID         string                  // 用户ID
	IngameName string                  // 游戏昵称
	Avatar     string                  // 头像
	Reputation *int                    // 评分
	Locale     string                  // 地区
	Platform   MarketPlatform          // 平台
	Status     drawplugin.MarketStatus // 状态
	Activity   *MarketUserActivity     // 活动
	LastSeen   time.Time               // 最后登录时间
}

// MarketUserActivity 用户活动信息（对齐 Java MarketUser.Activity 嵌套类展开）。
type MarketUserActivity struct {
	Type      drawplugin.MarketActivityType // 活动类型
	Details   string                        // 活动详情
	StartedAt time.Time                     // 活动开始时间
}

// intOrDash 整数指针转文本，nil 显示 "-"（对齐 Java 判空三元表达式）。
func intOrDash(v *int) string {
	if v == nil {
		return "-"
	}
	return strconv.Itoa(*v)
}

// marketStatusColor 返回订单用户状态对应的颜色（对齐 Java getStatusColor；未知状态回退弱化色）。
func marketStatusColor(status drawplugin.MarketStatus) color.RGBA {
	switch status {
	case drawplugin.MarketStatusOnline:
		return marketOnlineColor
	case drawplugin.MarketStatusIngame:
		return marketIngameColor
	case drawplugin.MarketStatusOffline:
		return marketOfflineColor
	case drawplugin.MarketStatusInvisible:
		return textMutedColor
	}
	return textMutedColor
}

// marketButtonColor 返回买卖切换按钮背景色（激活绿 / 未激活卡片背景，对齐 Java 三元表达式）。
func marketButtonColor(active bool) color.RGBA {
	if active {
		return marketActiveBgColor
	}
	return cardBackgroundColor
}

// marketButtonTextColor 返回买卖切换按钮文字颜色（激活白 / 未激活浅灰）。
func marketButtonTextColor(active bool) color.RGBA {
	if active {
		return textColor
	}
	return textSecondaryColor
}

// DrawMarketOrders 绘制市场订单详情图（订单列表为 nil 或空返回 nil）。
func DrawMarketOrders(orders *Orders) []byte {
	if orders == nil || orders.Orders == nil {
		return nil
	}

	orderList := orders.Orders

	szW, szH := scaleByPct(marketOrdersImageWidth, marketOrdersImageWidth, standardRatio)
	contentHeight := marketOrdersHeaderH + float64(len(orderList))*marketOrdersRowH
	startY := marketOrdersTitleH + 20

	// 预计算：标题下方 info 区域占 Y: startY ~ startY+100（对齐 Java 注释）
	infoEndY := startY + 100
	tableStartY := infoEndY + 30
	standingY := tableStartY + contentHeight + 50
	totalHeight := standingY + float64(szH)

	canvas := NewCanvas(int(marketOrdersImageWidth), int(totalHeight))
	canvas.SetColor(pageBackgroundColor).FillRect(0, 0, marketOrdersImageWidth, totalHeight)
	canvas.DrawTooRoundRect()

	// 绘制标题
	currentY := startY
	canvas.SetColor(titleColor).SetFontSize(48)
	canvas.AddCenteredText(orders.Name, currentY)
	canvas.SetFontSize(defaultFontSize)
	lineX := 50.0

	if orders.Icon != nil {
		w := float64(orders.Icon.Bounds().Dx())
		h := float64(orders.Icon.Bounds().Dy())
		lineX += w / 3
		canvas.DrawImageScaled(orders.Icon, 40, 25, w/3, h/3)
	}

	currentY += 80
	canvas.SetColor(dividerColor).DrawLine(lineX, currentY-70, marketOrdersImageWidth-30, currentY-70)

	// ---- 平台标签 ----
	lineX += 50
	canvas.SetColor(marketPlatformBgColor).FillRoundRect(lineX, currentY-40, 120, 60, 20)
	canvas.SetColor(textColor).AddText(orders.Form.name(), lineX+20, currentY)

	// ---- 卖家/买家切换按钮 ----
	isBuy := orders.IsBy != nil && *orders.IsBy
	// 卖家
	lineX += 250
	canvas.SetColor(marketButtonColor(isBuy)).FillRoundRect(lineX, currentY-40, 120, 60, 20)
	canvas.SetColor(marketButtonTextColor(isBuy)).AddText("卖家", lineX+25, currentY)
	// 买家
	lineX += 120
	canvas.SetColor(marketButtonColor(!isBuy)).FillRoundRect(lineX, currentY-40, 120, 60, 20)
	canvas.SetColor(marketButtonTextColor(!isBuy)).AddText("买家", lineX+25, currentY)

	// ---- 杜卡币 ----
	lineX += 280
	canvas.SetColor(accentGoldColor).SetFontSize(defaultFontSize)
	canvas.AddText("杜卡币", lineX, currentY-32)
	canvas.SetIconFontSize(defaultFontSize)
	canvas.AddText(drawplugin.IconMap[drawplugin.IconDucats], lineX, currentY+15)
	canvas.SetFontSize(defaultFontSize)
	lineX += 50
	canvas.AddText(intOrDash(orders.Ducats), lineX, currentY+15)

	// ---- 交易税 ----
	lineX += 120
	canvas.SetColor(textSecondaryColor).SetFontSize(defaultFontSize)
	canvas.AddText("交易税", lineX, currentY-32)
	canvas.SetIconFontSize(defaultFontSize)
	canvas.AddText(drawplugin.IconMap[drawplugin.IconCredits], lineX, currentY+15)
	canvas.SetFontSize(defaultFontSize)
	canvas.AddText(intOrDash(orders.TradingTax), lineX+50, currentY+15)

	// ---- 表头 ----
	currentY += 30
	canvas.SetColor(dividerColor).DrawLine(30, currentY, marketOrdersImageWidth-30, currentY)
	currentY += 20
	headerY := currentY + marketOrdersHeaderH/2 + 8
	canvas.SetColor(textSecondaryColor).SetFontSize(20)
	canvas.AddText("价格", marketOrdersImageMargin+10, headerY)
	canvas.AddText("数量", marketOrdersImageMargin+180, headerY)
	canvas.AddText("等级", marketOrdersImageMargin+380, headerY)
	canvas.AddText("卖家", marketOrdersImageMargin+600, headerY)
	canvas.AddText("状态", marketOrdersImageMargin+1000, headerY)

	currentY += marketOrdersHeaderH + 5

	// ---- 数据行 ----
	for _, order := range orderList {
		if order == nil {
			currentY += marketOrdersRowH
			continue
		}
		platinum := intOrDash(order.Platinum)
		quantity := intOrDash(order.Quantity)
		rank := intOrDash(order.Rank)

		sellerName := "-"
		statusText := "-"
		var status drawplugin.MarketStatus
		if order.User != nil {
			if order.User.IngameName != "" {
				sellerName = order.User.IngameName
			}
			status = order.User.Status
			if s, ok := drawplugin.MarketStatusMap[status]; ok {
				statusText = s
			}
		}
		rowY := currentY + marketOrdersRowH/2 + 8

		// 价格 — 白金色
		canvas.SetColor(marketPriceColor).SetIconFontSize(defaultFontSize)
		canvas.AddText(drawplugin.IconMap[drawplugin.IconPlatinum], marketOrdersImageMargin, rowY)
		canvas.SetFontSize(defaultFontSize)
		canvas.AddText(platinum, marketOrdersImageMargin+40, rowY)

		// 数量 — 浅灰色
		canvas.SetColor(textSecondaryColor).AddText(quantity, marketOrdersImageMargin+180, rowY)

		// 等级
		canvas.SetColor(textColor).AddText(rank, marketOrdersImageMargin+380, rowY)

		// 卖家
		canvas.AddText(sellerName, marketOrdersImageMargin+600, rowY)

		// 状态 — 颜色跟随状态
		canvas.SetColor(marketStatusColor(status)).AddText(statusText, marketOrdersImageMargin+1000, rowY)

		currentY += marketOrdersRowH
	}

	canvas.DrawStandingAt(marketOrdersImageWidth-float64(szW), totalHeight-float64(szH), float64(szW), float64(szH))
	canvas.AddFooter(totalHeight - 25)
	data, err := canvas.PNG()
	if err != nil {
		return nil
	}
	return data
}

// DrawMarketOrdersList 绘制可能要查询的订单物品列表图（possibleItems 为空返回 nil）。
func DrawMarketOrdersList(possibleItems []string) []byte {
	if len(possibleItems) == 0 {
		return nil
	}

	contentHeight := float64(len(possibleItems)) * marketOrdersRowH
	totalHeight := marketOrdersTitleH + contentHeight + marketOrdersFooterH + 150

	canvas := NewCanvas(int(marketOrdersImageWidth), int(totalHeight))
	canvas.SetColor(pageBackgroundColor).FillRect(0, 0, marketOrdersImageWidth, totalHeight)
	canvas.DrawTooRoundRect()

	currentY := marketOrdersTitleH
	canvas.SetColor(titleColor).SetFontSize(defaultFontSize)
	canvas.AddCenteredText("可能要查询的物品列表", currentY)

	currentY += 50
	headerY := currentY + marketOrdersHeaderH/2 + 8
	canvas.SetColor(textSecondaryColor).SetFontSize(20)
	canvas.AddText("序号", marketOrdersImageMargin+10, headerY)
	canvas.AddText("物品名称", marketOrdersImageMargin+150, headerY)

	currentY += marketOrdersHeaderH + 5

	// ---- 数据行 ----
	for i, item := range possibleItems {
		rowY := currentY + marketOrdersRowH/2 + 8
		canvas.SetColor(textColor).SetFontSize(defaultFontSize)
		canvas.AddText(strconv.Itoa(i+1), marketOrdersImageMargin+20, rowY)
		canvas.AddText(item, marketOrdersImageMargin+160, rowY)
		currentY += marketOrdersRowH
	}

	canvas.AddFooter(totalHeight - 40)
	data, err := canvas.PNG()
	if err != nil {
		return nil
	}
	return data
}
