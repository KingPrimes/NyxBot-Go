// 绘图排版回归黑盒测试：以 PNG 像素与画布几何断言无法直接观测的排版行为。
//   - 1999 日历（奇数月卡布局）：页脚不得压住最后一张卡（对齐后在卡底之下）
//   - 突击图：Boss 行渲染结束时间（零值不渲染），且右对齐不越出内容边界
//   - 集团赏金（三列）：看板娘取全部列最大底部，不得压住第三列卡片
//
// 说明：几何常量（contentX/cardW 等）在测试内按绘制规格硬编码，属「规格 vs 实现」比对；
// 颜色通过只含页脚的最小画布采样获得，避免依赖包内未导出的颜色常量。
package tests

import (
	"bytes"
	"image"
	"image/png"
	"testing"
	"time"

	"nyxbot-go/internal/draw"
	"nyxbot-go/internal/enum/drawplugin"
)

// decodePNG 断言绘图输出非空并解码为图像。
func decodePNG(t *testing.T, data []byte) image.Image {
	t.Helper()
	if len(data) == 0 {
		t.Fatal("绘图输出为空")
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("PNG 解码失败: %v", err)
	}
	return img
}

// mustPNG 编码画布并返回字节（供颜色采样）。
func mustPNG(t *testing.T, canvas *draw.Canvas) []byte {
	t.Helper()
	data, err := canvas.PNG()
	if err != nil {
		t.Fatalf("PNG 编码失败: %v", err)
	}
	return data
}

// rgbAt 取像素的 16 位 RGBA 分量（用于精确比对颜色）。
func rgbAt(img image.Image, x, y int) [4]uint32 {
	r, g, b, a := img.At(x, y).RGBA()
	return [4]uint32{r, g, b, a}
}

// footerColor 取底部署名文字颜色的取样点：渲染一张仅含页脚的画布，采样文字笔画像素。
// 页脚用 textMutedColor = #787878，与卡片/文字/分割线颜色均不同（见 constants.go）。
func footerColor(t *testing.T) [4]uint32 {
	t.Helper()
	canvas := draw.NewCanvas(400, 60)
	canvas.AddFooter(40)
	img := decodePNG(t, mustPNG(t, canvas))
	center := img.Bounds().Dx() / 2
	color := rgbAt(img, center, 34)
	if color[3] == 0 {
		t.Fatal("页脚颜色取样失败")
	}
	return color
}

// TestDrawKnownCalendarSeasonsFooterClearsLastCard 验证奇数月卡布局下页脚不再压住卡片。
//
// 回归背景：奇数布局最后一张卡在左列且为最高列，totalHeight 原先不含底部间距，
// 页脚基线（totalHeight-25）落在最后一张卡内部。修复后多留 40px，页脚整体位于卡底之下。
func TestDrawKnownCalendarSeasonsFooterClearsLastCard(t *testing.T) {
	const (
		contentX      = 50  // contentX
		contentStartY = 220 // contentStartY
		cardW         = 562 // cardW
		colGap        = 20  // colGap
		pad           = 20  // calendarCardPad
		months        = 5   // 奇数布局：左列 3 张（i=0,2,4），右列 2 张（i=1,3）
		daysPerMonth  = 30
		eventsPerDay  = 1
	)
	monthDays := map[int][]*draw.CalendarDay{}
	for m := 1; m <= months; m++ {
		days := make([]*draw.CalendarDay, 0, daysPerMonth)
		for d := 1; d <= daysPerMonth; d++ {
			days = append(days, &draw.CalendarDay{
				Month: m, Day: d,
				Events: []*draw.CalendarEvent{{Type: draw.CETChallenge, Challenge: "A"}},
			})
		}
		monthDays[m] = days
	}
	img := decodePNG(t, draw.DrawKnownCalendarSeasons([]*draw.KnownCalendarSeasons{{
		Season: "测试季节", YearIteration: 1, Version: "1.0", MonthDays: monthDays,
	}}))

	// 规格：月卡高 = 上下内边距(20+20) + 月标题(40) + 分隔线(8) + Σ(日期行 30 + 事件 30)
	cardH := pad*2 + 40 + 8 + daysPerMonth*(30+eventsPerDay*30)
	wantLastCardBottom := contentStartY + 2*(cardH+colGap) + cardH

	// 实测左列最后一张卡底边：x=300 处卡内无文字（日期/事件文字起于 x=70/85 且很短），
	// 该列卡片背景色最后出现的位置即卡底；右列看板娘（x>=632）不在此列。
	cardFill := rgbAt(img, 300, 400)
	measuredBottom := -1
	for y := contentStartY; y < img.Bounds().Dy(); y++ {
		if rgbAt(img, 300, y) == cardFill {
			measuredBottom = y
		}
	}
	if measuredBottom < wantLastCardBottom-4 || measuredBottom > wantLastCardBottom+1 {
		t.Fatalf("左列最后一张卡底边实测 %d, 期望约 %d", measuredBottom, wantLastCardBottom)
	}

	// 页脚颜色像素不得出现在卡片范围内（x 限定在左卡内，避免右列看板娘干扰）
	footer := footerColor(t)
	rowsInside, rowsBelow := 0, 0
	for y := 0; y < img.Bounds().Dy(); y++ {
		hit := false
		for x := contentX; x <= contentX+cardW; x++ {
			if rgbAt(img, x, y) == footer {
				hit = true
				break
			}
		}
		if !hit {
			continue
		}
		if y <= measuredBottom {
			rowsInside++
		} else {
			rowsBelow++
		}
	}
	if rowsInside != 0 {
		t.Fatalf("页脚有 %d 行落在最后一张卡内（卡底 %d），应完全在卡底之下", rowsInside, measuredBottom)
	}
	if rowsBelow == 0 {
		t.Fatal("卡底之下未找到页脚文字，颜色取样或绘制可能失效")
	}
}

// TestDrawSortiesRendersExpiry 验证突击图渲染 Expiry：非零才画、零值省略、画布尺寸不变。
func TestDrawSortiesRendersExpiry(t *testing.T) {
	const (
		contentX    = 70  // imageMargin(40) + 30
		contentW    = 760 // 900 - 2*70
		bossRowFrom = 190 // Boss 行基线 210 附近
		bossRowTo   = 218
	)
	boss := "Councilor Vay Hek" // 最长 Boss 名，用于顺带验证不与结束时间碰撞
	variants := []*draw.SortieVariant{{MissionType: drawplugin.MTExtermination, Node: "节点A"}}

	withExpiry := decodePNG(t, draw.DrawSorties(&draw.Sortie{
		Boss: boss, Expiry: time.Now().Add(90 * time.Minute), Variants: variants,
	}))
	atZero := decodePNG(t, draw.DrawSorties(&draw.Sortie{Boss: boss, Variants: variants}))

	if withExpiry.Bounds() != atZero.Bounds() {
		t.Fatalf("渲染结束时间不应改变画布尺寸: %v vs %v", withExpiry.Bounds(), atZero.Bounds())
	}
	inkWith, rightWith := contentInk(withExpiry, bossRowFrom, bossRowTo)
	inkZero, rightZero := contentInk(atZero, bossRowFrom, bossRowTo)
	if inkWith <= inkZero || rightWith <= rightZero {
		t.Fatalf("Boss 行未渲染结束时间: 含结束时间=(%d px, 最右 %d) 零值=(%d px, 最右 %d)",
			inkWith, rightWith, inkZero, rightZero)
	}
	// 结束时间右对齐到内容右边界，既不能越界也不能与左侧 Boss 文本挤在一起
	if rightWith > contentX+contentW {
		t.Fatalf("结束时间越出内容右边界 %d: 最右墨迹 x=%d", contentX+contentW, rightWith)
	}
	if rightWith < contentX+contentW-40 {
		t.Fatalf("结束时间未右对齐到内容边界 %d: 最右墨迹 x=%d", contentX+contentW, rightWith)
	}
}

// TestDrawSyndicateStandingBelowAllColumns 验证赏金三列布局看板娘让开最高列。
//
// 回归背景：原先 standingY 在 isOdd 分支只取 max(colEndY[0], colEndY[1])，忽略第三列；
// 卡片高度随奖励条数变化（下限 250，40 个奖励时 1160），第三列卡片少但可能更高，
// 看板娘因此会压在第三列卡片上。修复后统一取全部列最大底部 + 10。
func TestDrawSyndicateStandingBelowAllColumns(t *testing.T) {
	mk := func(rewards int) *draw.SyndicateJob {
		job := &draw.SyndicateJob{Type: "捕获"}
		for i := 0; i < rewards; i++ {
			job.Rewards = append(job.Rewards, &draw.SyndicateReward{Item: "奖励", ItemCount: 1})
		}
		return job
	}
	// n=4（i%3）：col0 = i0,i3；col1 = i1；col2 = i2 —— 让 col2 的卡片最高
	jobs := []*draw.SyndicateJob{mk(0), mk(0), mk(40), mk(0)}
	img := decodePNG(t, draw.DrawSyndicateImage(&draw.SyndicateMission{Jobs: jobs}))

	const (
		startY      = 135 // imageMarginT(60) + imageTitleH(50) + 25
		cardMarginY = 30  // cardMarginY
		standingBox = 480 // syndicateImageW(1600) * standardRatio(0.3)
	)
	// 规格：卡片高 = 70(类型+等级) + 30(奖励标题) + 奖励数*25 + 40 + 20，下限 250
	hShort := 250
	hTall := 70 + 30 + 40*25 + 40 + 20 // 1160
	colEnd := []int{
		startY + hShort + hShort + cardMarginY, // col0
		startY + hShort,                        // col1
		startY + hTall,                         // col2 = 1295
	}
	maxColEnd := colEnd[0]
	for _, end := range colEnd {
		if end > maxColEnd {
			maxColEnd = end
		}
	}
	if maxColEnd != colEnd[2] {
		t.Fatalf("用例前提不成立，第三列应为最高列: colEnd=%v", colEnd)
	}
	if got := img.Bounds().Dy(); got < maxColEnd+standingBox {
		t.Fatalf("看板娘压住第三列卡片: 画布高 %d < 最高列底 %d + 看板娘高 %d", got, maxColEnd, standingBox)
	}
	if got, want := img.Bounds().Dy(), maxColEnd+10+standingBox; got != want {
		t.Fatalf("画布高 = %d, 期望 %d(最高列底+10+看板娘高)", got, want)
	}
}

// contentInk 统计行带 [yFrom, yTo] 内、内容区 x∈[40,860] 的墨迹像素数与最右墨迹 x。
// x 上界 860 用于排除画布右侧的装饰边框与看板娘（突击图看板娘位于 x>=630、y>=330）。
func contentInk(img image.Image, yFrom, yTo int) (int, int) {
	background := rgbAt(img, 5, 5)
	count, rightmost := 0, 0
	for y := yFrom; y <= yTo; y++ {
		for x := 40; x <= 860; x++ {
			if rgbAt(img, x, y) != background {
				count++
				if x > rightmost {
					rightmost = x
				}
			}
		}
	}
	return count, rightmost
}
