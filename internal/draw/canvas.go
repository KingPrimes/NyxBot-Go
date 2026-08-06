// Canvas 绘图合成器：对齐 Java ImageCombiner 的链式绘制 API
// 基于 fogleman/gg 实现矩形/圆角/椭圆/线条/文本/渐变/图片/看板娘等绘制能力
package draw

import (
	"bytes"
	"image"
	"image/color"
	"math"
	"math/rand"
	"sync"

	"github.com/fogleman/gg"
	"golang.org/x/image/font"
)

// Canvas 目标画布，所有绘制操作的载体（尺寸不可变），方法支持链式调用。
type Canvas struct {
	dc     *gg.Context
	width  int
	height int
	face   font.Face // 当前激活字体 face
	size   float64   // 当前字号
}

// canvasMu 限制并发绘图数量（对齐 Java DRAW_SEMAPHORE max 2）。
var canvasMu = make(chan struct{}, 2)

// NewCanvas 创建指定尺寸的画布并铺白底。各 Draw 入口随后用页面背景色 FillRect 铺满覆盖，
// 此处白色仅作为所有绘图路径一致的初始底色，保证透明区域稳定。
func NewCanvas(width, height int) *Canvas {
	canvasMu <- struct{}{}
	dc := gg.NewContext(width, height)
	dc.SetColor(textColor)
	dc.Clear()
	return &Canvas{dc: dc, width: width, height: height, face: textFace(defaultFontSize), size: defaultFontSize}
}

// release 释放画布（Encode 后调用）。
func (c *Canvas) release() { <-canvasMu }

// SetColor 设置当前绘图颜色。
func (c *Canvas) SetColor(rgba color.RGBA) *Canvas {
	c.dc.SetRGBA255(int(rgba.R), int(rgba.G), int(rgba.B), int(rgba.A))
	return c
}

// SetFontSize 设置当前正文字体字号（基于思源宋体）。
func (c *Canvas) SetFontSize(size float64) *Canvas {
	face := textFace(size)
	if face == nil {
		return c
	}
	c.face = face
	c.size = size
	c.dc.SetFontFace(face)
	return c
}

// SetIconFontSize 设置当前图标字体字号（基于 Warframe 图标字体）。
func (c *Canvas) SetIconFontSize(size float64) *Canvas {
	face := iconFace(size)
	if face == nil {
		return c
	}
	c.dc.SetFontFace(face)
	return c
}

// SetStroke 设置描边宽度。
func (c *Canvas) SetStroke(width float64) *Canvas {
	c.dc.SetLineWidth(width)
	return c
}

// Face 返回当前字体 face（供文本测量）。
func (c *Canvas) Face() font.Face { return c.face }

// Width 返回画布宽度。
func (c *Canvas) Width() int { return c.width }

// Height 返回画布高度。
func (c *Canvas) Height() int { return c.height }

// FillRect 填充矩形。
func (c *Canvas) FillRect(x, y, w, h float64) *Canvas {
	c.dc.DrawRectangle(x, y, w, h)
	c.dc.Fill()
	return c
}

// DrawRoundRect 绘制圆角矩形边框。
func (c *Canvas) DrawRoundRect(x, y, w, h, arc float64) *Canvas {
	c.dc.DrawRoundedRectangle(x, y, w, h, arc)
	c.dc.Stroke()
	return c
}

// FillRoundRect 填充圆角矩形。
func (c *Canvas) FillRoundRect(x, y, w, h, arc float64) *Canvas {
	c.dc.DrawRoundedRectangle(x, y, w, h, arc)
	c.dc.Fill()
	return c
}

// DrawLine 绘制直线。
func (c *Canvas) DrawLine(x1, y1, x2, y2 float64) *Canvas {
	c.dc.DrawLine(x1, y1, x2, y2)
	c.dc.Stroke()
	return c
}

// DrawOval 绘制椭圆边框。
func (c *Canvas) DrawOval(x, y, w, h float64) *Canvas {
	c.dc.DrawEllipse(x+w/2, y+h/2, w/2, h/2)
	c.dc.Stroke()
	return c
}

// FillOval 填充椭圆。
func (c *Canvas) FillOval(x, y, w, h float64) *Canvas {
	c.dc.DrawEllipse(x+w/2, y+h/2, w/2, h/2)
	c.dc.Fill()
	return c
}

// ClearRect 清除矩形区域为透明。
func (c *Canvas) ClearRect(x, y, w, h float64) *Canvas {
	c.dc.Push()
	c.dc.SetColor(color.RGBA{0, 0, 0, 0})
	c.dc.ClearPath()
	c.dc.DrawRectangle(x, y, w, h)
	c.dc.Fill()
	c.dc.Pop()
	return c
}

// StringWidth 测量文本宽度（当前字体）。
func (c *Canvas) StringWidth(text string) float64 {
	if c.face == nil || text == "" {
		return 0
	}
	w, _ := c.dc.MeasureString(text)
	return w
}

// ascent 当前字体的上伸高度（基线到字形顶部的距离，对齐 FontMetrics.getAscent）。
func (c *Canvas) ascent() float64 {
	if c.face == nil {
		return 0
	}
	return float64(c.face.Metrics().Ascent.Ceil())
}

// AddText 绘制文本（y 为基线，对齐 AWT drawString；
// gg 的 DrawStringAnchored(ax=0, ay=0) 中 y 本身即为基线，无需再做 ascent 修正）。
func (c *Canvas) AddText(text string, x, y float64) *Canvas {
	if text == "" || c.face == nil {
		return c
	}
	c.dc.DrawStringAnchored(text, x, y, 0, 0)
	return c
}

// AddCenteredText 绘制水平居中文本（指定基线 y）。
func (c *Canvas) AddCenteredText(text string, y float64) *Canvas {
	if text == "" {
		return c
	}
	c.AddText(text, (float64(c.width)-c.StringWidth(text))/2, y)
	return c
}

// AddCenteredXText 绘制水平居中文本（带 X 偏移）。
func (c *Canvas) AddCenteredXText(text string, y, xOffset float64) *Canvas {
	if text == "" {
		return c
	}
	c.AddText(text, (float64(c.width)-c.StringWidth(text))/2+xOffset, y)
	return c
}

// AddCenteredXYText 绘制完全居中（X、Y 轴）文本。
func (c *Canvas) AddCenteredXYText(text string) *Canvas {
	if text == "" {
		return c
	}
	c.AddText(text, (float64(c.width)-c.StringWidth(text))/2, (float64(c.height)+c.ascent())/2)
	return c
}

// AddMultilineText 绘制多行文本（手动换行 \n）。
func (c *Canvas) AddMultilineText(text string, x, startY, lineHeight, spacing float64) *Canvas {
	if text == "" {
		return c
	}
	lines := splitLines(text)
	currentY := startY
	for _, line := range lines {
		c.AddText(line, x, currentY)
		currentY += lineHeight + spacing
	}
	return c
}

// AddMultilineCenteredText 绘制多行居中文本（手动换行 \n）。
func (c *Canvas) AddMultilineCenteredText(text string, startY, lineHeight, spacing float64) *Canvas {
	if text == "" {
		return c
	}
	lines := splitLines(text)
	currentY := startY
	for _, line := range lines {
		c.AddCenteredText(line, currentY)
		currentY += lineHeight + spacing
	}
	return c
}

// AddMultilineTextWithWrap 绘制自动换行多行文本。
func (c *Canvas) AddMultilineTextWithWrap(text string, x, startY, maxWidth, lineHeight, spacing float64) *Canvas {
	if text == "" {
		return c
	}
	lines := wrapText(c, text, maxWidth)
	currentY := startY
	for _, line := range lines {
		c.AddText(line, x, currentY)
		currentY += lineHeight + spacing
	}
	return c
}

// DrawGradientBackground 绘制渐变背景填充画布。
func (c *Canvas) DrawGradientBackground(start, end color.RGBA, vertical bool) *Canvas {
	var grad gg.Gradient
	if vertical {
		grad = gg.NewLinearGradient(0, 0, 0, float64(c.height))
	} else {
		grad = gg.NewLinearGradient(0, 0, float64(c.width), 0)
	}
	grad.AddColorStop(0, start)
	grad.AddColorStop(1, end)
	c.dc.SetFillStyle(grad)
	c.dc.DrawRectangle(0, 0, float64(c.width), float64(c.height))
	c.dc.Fill()
	return c
}

// DrawTooRoundRect 绘制双层圆角边框（对齐 ImageCombiner.drawTooRoundRect()）。
func (c *Canvas) DrawTooRoundRect() *Canvas {
	padding := 20.0
	w := float64(c.width)
	h := float64(c.height)
	c.SetColor(borderOuterColor).SetStroke(4).
		DrawRoundRect(padding-8, padding, w-(padding-8)*2, h-padding*2, 20)
	c.SetColor(borderInnerColor).
		DrawRoundRect(padding, padding, w-padding*2, h-padding*2, 20)
	return c
}

// DrawTitle 绘制居中渐变标题（对齐 ImageCombiner.drawTitle）。
func (c *Canvas) DrawTitle(text string) *Canvas {
	if text == "" {
		return c
	}
	lines := wrapText(c, text, float64(c.width)-40)
	totalW := 0.0
	for _, line := range lines {
		if w := c.StringWidth(line); w > totalW {
			totalW = w
		}
	}
	totalH := float64(len(lines))*c.lineHeight() + float64(len(lines)-1)*5
	startX := (float64(c.width) - totalW) / 2
	startY := (float64(c.height)-totalH)/2 + c.lineHeight() + c.ascent() + 15

	bgX, bgY := startX-15, startY-c.ascent()-15
	bgW, bgH := totalW+30, totalH+15
	grad := gg.NewLinearGradient(bgX, bgY, bgX+bgW, bgY)
	grad.AddColorStop(0, color.RGBA{70, 130, 180, 255})
	grad.AddColorStop(1, color.RGBA{100, 149, 237, 255})
	c.dc.SetFillStyle(grad)
	c.dc.DrawRoundedRectangle(bgX, bgY, bgW, bgH, 15)
	c.dc.Fill()
	c.SetColor(color.RGBA{255, 255, 255, 255}).
		DrawRoundRect(bgX, bgY, bgW, bgH, 15)

	// 阴影 + 文字（currentY 为基线，DrawStringAnchored(0,0) y 即基线）
	currentY := startY
	for _, line := range lines {
		lineX := (float64(c.width) - c.StringWidth(line)) / 2
		c.dc.SetColor(color.RGBA{0, 0, 0, 100})
		c.dc.DrawStringAnchored(line, lineX+1, currentY+1, 0, 0)
		c.SetColor(textColor)
		c.dc.DrawStringAnchored(line, lineX, currentY, 0, 0)
		currentY += c.lineHeight() + 5
	}
	return c
}

// lineHeight 当前字体行高（对齐 FontMetrics.getHeight）。
func (c *Canvas) lineHeight() float64 {
	if c.face == nil {
		return c.size
	}
	m := c.face.Metrics()
	return float64(m.Ascent.Ceil() + m.Descent.Ceil())
}

// DrawImage 绘制图片到指定位置（原始尺寸）。
func (c *Canvas) DrawImage(img image.Image, x, y float64) *Canvas {
	if img == nil {
		return c
	}
	c.dc.DrawImage(img, int(x), int(y))
	return c
}

// DrawImageScaled 绘制图片（指定宽高缩放）。
func (c *Canvas) DrawImageScaled(img image.Image, x, y, w, h float64) *Canvas {
	if img == nil {
		return c
	}
	c.dc.Push()
	c.dc.ScaleAbout(w/float64(img.Bounds().Dx()), h/float64(img.Bounds().Dy()), x, y)
	c.dc.DrawImageAnchored(img, int(x), int(y), 0, 0)
	c.dc.Pop()
	return c
}

// DrawImageWithAspectRatio 绘制图片，保持原始比例缩放并居中放置于指定盒子。
func (c *Canvas) DrawImageWithAspectRatio(img image.Image, x, y, maxW, maxH float64) *Canvas {
	if img == nil || maxW <= 0 || maxH <= 0 {
		return c
	}
	imgW := float64(img.Bounds().Dx())
	imgH := float64(img.Bounds().Dy())
	if imgW == 0 || imgH == 0 {
		return c
	}
	scale := math.Min(maxW/imgW, maxH/imgH)
	scaledW := imgW * scale
	scaledH := imgH * scale
	drawX := x + (maxW-scaledW)/2
	drawY := y + (maxH-scaledH)/2
	c.DrawImageScaled(img, drawX, drawY, scaledW, scaledH)
	return c
}

// standingMu 看板娘随机选择的互斥。math/rand 顶层函数内部已带锁（并发安全），
// 此处仍显式加锁，以明确"对同一看板娘池的并发选取"这一共享意图。
var standingMu sync.Mutex

// DrawStandingAt 在指定盒子绘制随机看板娘（保持比例居中，对齐 ImageCombiner.drawStandingAt）。
func (c *Canvas) DrawStandingAt(x, y, w, h float64) *Canvas {
	images := standingImages()
	if len(images) == 0 {
		return c
	}
	standingMu.Lock()
	img := images[rand.Intn(len(images))] //nolint:gosec // 随机看板娘，无需密码学随机
	standingMu.Unlock()
	return c.DrawImageWithAspectRatio(img, x, y, w, h)
}

// AddFooter 添加底部署名文本（对齐 DrawConstants.addFooter）。
func (c *Canvas) AddFooter(y float64) *Canvas {
	c.SetColor(textMutedColor).SetFontSize(smallFontSize)
	return c.AddCenteredText(footerText, y)
}

// PNG 编码画布为 PNG 字节。
func (c *Canvas) PNG() ([]byte, error) {
	defer c.release()
	var buf bytes.Buffer
	if err := c.dc.EncodePNG(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
