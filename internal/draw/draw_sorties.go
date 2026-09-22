// 突击图片：单卡片布局（Boss + 任务列表，每行任务类型着色 + modifier 子行）
// 对齐 Java DefaultDrawSortiesImage / model.Sortie / model.Variant
package draw

import (
	"time"

	"nyxbot-go/internal/enum/drawplugin"
)

// Sortie 突击绘图输入（对齐 Java model.Sortie 原始字段）。
type Sortie struct {
	Boss     string           // Boss 名称（已翻译）
	Expiry   time.Time        // 结束时间
	Variants []*SortieVariant // 任务阶段列表
}

// SortieVariant 突击任务阶段（对齐 Java model.Variant）。
type SortieVariant struct {
	MissionType  drawplugin.MissionType  // 任务类型
	ModifierType drawplugin.ModifierType // modifier 类型
	Node         string                  // 节点（已翻译）
}

// DrawSorties 绘制突击任务图（对齐 Java drawSortiesImage）。
func DrawSorties(sorties *Sortie) []byte {
	if sorties == nil {
		return nil
	}
	const canvasW = 900
	const minHeight = 600
	const rowHeight = 50

	variantCount := len(sorties.Variants)
	modifierCount := 0
	for _, v := range sorties.Variants {
		if v != nil && v.ModifierType != "" {
			modifierCount++
		}
	}
	canvasH := 420 + variantCount*rowHeight + modifierCount*28
	if canvasH < minHeight {
		canvasH = minHeight
	}

	canvas := NewCanvas(canvasW, canvasH)
	canvas.SetColor(pageBackgroundColor).FillRect(0, 0, float64(canvasW), float64(canvasH))
	canvas.DrawTooRoundRect()

	// 标题
	canvas.SetColor(titleColor).SetFontSize(44)
	canvas.AddCenteredText("突击任务", 70)

	// 分割线
	contentX := imageMargin + 30
	contentW := canvasW - 2*(imageMargin+30)
	canvas.SetColor(dividerColor).DrawLine(float64(contentX), 138, float64(contentX+contentW), 138)

	y := 160

	// Boss + 结束时间
	y += rowHeight
	canvas.SetColor(textColor).SetFontSize(28)
	boss := sorties.Boss
	if boss == "" {
		boss = "未知"
	}
	canvas.AddText("Boss: "+boss, float64(imageMargin), float64(y))

	// 任务列表标题
	y += rowHeight + 10
	canvas.SetColor(titleColor).SetFontSize(28)
	canvas.AddText("任务列表:", float64(imageMargin), float64(y))

	if len(sorties.Variants) > 0 {
		y += 10
		for _, v := range sorties.Variants {
			if v == nil {
				continue
			}
			y += rowHeight
			node := v.Node
			if node == "" {
				node = "未知节点"
			}
			mtInfo, mtOK := drawplugin.MissionTypeMap[v.MissionType]
			mtName := "未知"
			if mtOK {
				mtName = mtInfo.Name
			}
			canvas.SetColor(missionTypeColor(v.MissionType)).SetFontSize(26)
			canvas.AddText("• "+mtName+" - "+node, float64(imageMargin+20), float64(y))
			if v.ModifierType != "" {
				y += 40
				modStr := drawplugin.ModifierMap[v.ModifierType]
				if modStr == "" {
					modStr = "未知"
				}
				canvas.SetColor(textSecondaryColor).SetFontSize(24)
				canvas.AddText("    • "+modStr, float64(imageMargin+40), float64(y))
			}
		}
	} else {
		y += rowHeight
		canvas.SetColor(textColor).SetFontSize(26)
		canvas.AddText("暂无任务信息", float64(imageMargin+20), float64(y))
	}

	// 看板娘 + 底部署名
	szW, szH := scaleByPct(canvasW, canvasW, standardRatio)
	canvas.DrawStandingAt(canvasW-float64(szW), float64(canvasH)-float64(szH), float64(szW), float64(szH))
	canvas.AddFooter(float64(canvasH) - 25)
	data, err := canvas.PNG()
	if err != nil {
		return nil
	}
	return data
}
