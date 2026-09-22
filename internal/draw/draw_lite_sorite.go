// 执刑官猎杀图片：标题 + Boss/结束时间 + 任务列表（对齐 Java DefaultDrawLiteSoriteImage）
package draw

import (
	"image/color"
)

// LiteSoriteMission 执刑官猎杀单个任务（对齐 Java model.Mission）。
type LiteSoriteMission struct {
	TypeName  string     // 任务类型名（已翻译）
	TypeColor color.RGBA // 任务类型颜色
	Node      string     // 节点
}

// LiteSorite 执刑官猎杀绘图输入（对齐 Java model.LiteSorite）。
type LiteSorite struct {
	Boss     string               // 执刑官名
	Expiry   string               // 结束时间文本
	Missions []*LiteSoriteMission // 任务列表
}

// DrawLiteSorite 绘制执刑官猎杀图（对齐 Java drawLiteSoriteImage）。
func DrawLiteSorite(ls *LiteSorite) []byte {
	if ls == nil {
		return nil
	}
	const (
		imageWidth  = 900
		imageHeight = 600
	)
	missionCount := len(ls.Missions)
	canvasH := maxI(imageHeight, 400+missionCount*60)

	canvas := NewCanvas(imageWidth, canvasH)
	canvas.SetColor(pageBackgroundColor).FillRect(0, 0, float64(imageWidth), float64(canvasH))
	canvas.DrawTooRoundRect()

	// 标题 + 分割线
	canvas.SetColor(titleColor).SetFontSize(44)
	canvas.AddCenteredText("执刑官猎杀", 70)
	contentX := imageMarginTop + 30
	contentW := imageWidth - 2*(imageMarginTop+30)
	canvas.SetColor(dividerColor).DrawLine(float64(contentX), 138, float64(contentX+contentW), 138)
	y := 160

	// Boss 信息
	y += imageRowHeight
	canvas.SetColor(textColor).SetFontSize(24)
	canvas.AddText("Boss: "+ls.Boss, imageMargin, float64(y))
	if ls.Expiry != "" {
		canvas.SetColor(textColor).SetFontSize(24)
		canvas.AddText("结束时间: "+ls.Expiry, imageWidth/2-60, float64(y))
	}

	// 任务列表标题
	y += imageRowHeight + 10
	canvas.SetColor(titleColor).SetFontSize(24)
	canvas.AddText("任务列表:", imageMargin, float64(y))

	if len(ls.Missions) > 0 {
		y += 10
		for _, m := range ls.Missions {
			y += imageRowHeight
			col := m.TypeColor
			if col == (color.RGBA{}) {
				col = textColor
			}
			canvas.SetColor(col).SetFontSize(24)
			canvas.AddText("• "+m.TypeName+" - "+m.Node, imageMarginTop+20, float64(y))
		}
	} else {
		y += imageRowHeight
		canvas.SetColor(textColor).SetFontSize(24)
		canvas.AddText("暂无任务信息", imageMarginTop+20, float64(y))
	}

	szW, szH := scaleByPct(imageWidth, imageWidth, standardRatio)
	canvas.DrawStandingAt(float64(imageWidth-szW), float64(canvasH-szH), float64(szW), float64(szH))
	canvas.AddFooter(float64(canvasH - imageFooterH))
	data, err := canvas.PNG()
	if err != nil {
		return nil
	}
	return data
}
