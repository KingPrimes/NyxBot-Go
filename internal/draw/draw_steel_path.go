// 钢铁奖励图片：三行文本信息（当前奖励/下一个奖励/剩余时间）
// 对齐 Java DefaultDrawSteelPathImage / model.SteelPathOffering
package draw

// SteelPathOffering 钢铁奖励绘图输入（对齐 Java model.SteelPathOffering）。
type SteelPathOffering struct {
	CurrentReward string // 当前奖励
	NextReward    string // 下一个奖励
	Remaining     string // 剩余时间文本
}

// DrawSteelPath 绘制钢铁奖励图（对齐 Java drawSteelPathImage）。
func DrawSteelPath(sp *SteelPathOffering) []byte {
	if sp == nil {
		return nil
	}
	const (
		canvasW       = 1200
		contentX      = 60
		contentW      = 1080
		rowH          = 55
		titleY        = 80
		dividerY      = 115
		contentStartY = 155
	)
	rows := 0
	if sp.CurrentReward != "" {
		rows++
	}
	if sp.NextReward != "" {
		rows++
	}
	if sp.Remaining != "" {
		rows++
	}
	canvasH := maxI(contentStartY+rows*rowH+360, 400)

	canvas := NewCanvas(canvasW, canvasH)
	canvas.SetColor(pageBackgroundColor).FillRect(0, 0, float64(canvasW), float64(canvasH))
	canvas.DrawTooRoundRect()

	canvas.SetColor(titleColor).SetFontSize(44)
	canvas.AddCenteredText("钢铁奖励", titleY)
	canvas.SetColor(dividerColor).DrawLine(contentX, dividerY, contentX+contentW, dividerY)

	y := contentStartY
	if sp.CurrentReward != "" {
		canvas.SetColor(textColor).SetFontSize(28)
		canvas.AddText("当前奖励: "+sp.CurrentReward, contentX, float64(y+18))
		y += rowH
	}
	if sp.NextReward != "" {
		canvas.SetColor(textColor).SetFontSize(28)
		canvas.AddText("下一个奖励: "+sp.NextReward, contentX, float64(y+18))
		y += rowH
	}
	if sp.Remaining != "" {
		canvas.SetColor(accentGoldColor).SetFontSize(28)
		canvas.AddText("剩余时间: "+sp.Remaining, contentX, float64(y+18))
	}

	szW, szH := scaleByPct(canvasW, canvasW, standardRatio)
	canvas.DrawStandingAt(float64(canvasW-szW), float64(canvasH-szH), float64(szW), float64(szH))
	canvas.AddFooter(float64(canvasH) - 25)
	data, err := canvas.PNG()
	if err != nil {
		return nil
	}
	return data
}
