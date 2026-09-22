// 每日特惠图片：固定尺寸标签-值信息卡（物品名/价格/折扣/库存/剩余时间）
// 对齐 Java DefaultDrawDailyDealsImage / model.DailyDeals
package draw

import (
	"image/color"
	"strconv"
	"time"
)

// DailyDeals 每日特惠绘图输入（对齐 Java model.DailyDeals 原始字段）。
// Count 为折扣百分比，TimeLeft 由 Expiry 推导。
type DailyDeals struct {
	Item          string    // 物品名称（已翻译）
	OriginalPrice *int      // 原价
	SalePrice     *int      // 现价
	Count         *float64  // 折扣百分比
	Total         *int      // 总数量
	Sold          *int      // 已售数量
	Expiry        time.Time // 结束时间
}

// 价格颜色（对齐 Java 常量）。
var (
	dailyOriginalColor = color.RGBA{0xff, 0x6b, 0x6b, 0xff} // 原价 - 红
	dailySaleColor     = color.RGBA{0x4c, 0xaf, 0x50, 0xff} // 现价 - 绿
	dailyDiscountColor = color.RGBA{0xff, 0x6b, 0x6b, 0xff} // 折扣 - 红
	dailyRemainColor   = color.RGBA{0x4c, 0xaf, 0x50, 0xff} // 剩余 - 绿
)

// DrawDailyDeals 绘制每日特惠图（对齐 Java drawDailyDealsImage）。
func DrawDailyDeals(deal *DailyDeals) []byte {
	if deal == nil {
		return nil
	}
	const (
		canvasW = 1000
		canvasH = 600
	)
	canvas := NewCanvas(canvasW, canvasH)
	canvas.SetColor(pageBackgroundColor).FillRect(0, 0, canvasW, canvasH)
	canvas.DrawTooRoundRect()
	szW, szH := scaleByPct(canvasW, canvasW, standardRatio)
	canvas.DrawStandingAt(canvasW-float64(szW), canvasH-float64(szH), float64(szW), float64(szH))

	// 标题
	canvas.SetColor(titleColor).SetFontSize(36)
	canvas.AddCenteredText("每日特惠", 80)

	startY := 130
	labelX := float64(imageMargin)

	// 物品名称
	itemName := deal.Item
	if itemName == "" {
		itemName = "未知物品"
	}
	canvas.SetColor(textColor).SetFontSize(28)
	canvas.AddText("物品名称:", labelX, float64(startY))
	canvas.SetColor(textColor).SetFontSize(28)
	canvas.AddText(itemName, labelX+220, float64(startY))
	startY += imageRowHeight

	// 原价/现价
	canvas.SetColor(textColor).SetFontSize(28)
	canvas.AddText("原价/现价:", labelX, float64(startY))
	orig := priceText(deal.OriginalPrice)
	canvas.SetColor(dailyOriginalColor).SetFontSize(28)
	canvas.AddText(orig, labelX+220, float64(startY))
	// 删除线（对齐 Java drawLine 于基线上方 13px）
	origW := canvas.StringWidth(orig)
	canvas.SetColor(dailyOriginalColor).SetStroke(2).DrawLine(labelX+220, float64(startY)-13, labelX+220+origW, float64(startY)-13)
	// 斜杠分隔 + 现价
	saleText := priceText(deal.SalePrice)
	canvas.SetColor(textColor).SetFontSize(28)
	canvas.AddText(" / ", labelX+220+origW, float64(startY))
	canvas.SetColor(dailySaleColor).SetFontSize(28)
	canvas.AddText(saleText, labelX+220+origW+40, float64(startY))
	startY += imageRowHeight

	// 折扣比
	canvas.SetColor(textColor).SetFontSize(28)
	canvas.AddText("折扣比:", labelX, float64(startY))
	discountText := "0 %"
	if deal.Count != nil {
		discountText = floatTrim(*deal.Count) + " %"
	}
	canvas.SetColor(dailyDiscountColor).SetFontSize(28)
	canvas.AddText(discountText, labelX+220, float64(startY))
	startY += imageRowHeight

	// 总数/剩余
	canvas.SetColor(textColor).SetFontSize(28)
	canvas.AddText("总/余:", labelX, float64(startY))
	totalText := "0"
	if deal.Total != nil {
		totalText = strconv.Itoa(*deal.Total)
	}
	totalW := canvas.StringWidth(totalText)
	canvas.SetColor(textColor).SetFontSize(28)
	canvas.AddText(totalText, labelX+220, float64(startY))
	canvas.AddText(" / ", labelX+220+totalW, float64(startY))
	remainText := "0"
	if deal.Total != nil && deal.Sold != nil {
		remainText = strconv.Itoa(*deal.Total - *deal.Sold)
	}
	canvas.SetColor(dailyRemainColor).SetFontSize(28)
	canvas.AddText(remainText, labelX+220+totalW+40, float64(startY))
	startY += imageRowHeight

	// 剩余时间
	canvas.SetColor(textColor).SetFontSize(28)
	canvas.AddText("剩余时间:", labelX, float64(startY))
	timeLeft := timeDeltaString(time.Until(deal.Expiry))
	if timeLeft == "" {
		timeLeft = "未知"
	}
	canvas.SetColor(textColor).SetFontSize(28)
	canvas.AddText(timeLeft, labelX+220, float64(startY))

	// 底部署名
	canvas.AddFooter(canvasH - 25)
	data, err := canvas.PNG()
	if err != nil {
		return nil
	}
	return data
}

// priceText 格式化价格文本（指针为空回退 "0"）。
func priceText(p *int) string {
	if p == nil {
		return "0"
	}
	return strconv.Itoa(*p)
}

// floatTrim 格式化折扣百分比（去掉多余小数，对齐 Java Double.toString）。
func floatTrim(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}
