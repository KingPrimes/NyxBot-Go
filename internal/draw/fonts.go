// 字体加载：思源宋体粗体（正文）+ Warframe 图标字体（图标字符）
// 对齐 Java Fonts 类：FONT_TEXT = SourceHanSerifCN-Bold.ttf，FONT_WARFRAME_ICON = warframe_font_icon.ttf
package draw

import (
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
)

// 默认字号（对齐 Java Fonts.LARGE_FONT_SIZE）。
const (
	defaultFontSize = 32
	smallFontSize   = 18
	largeFontSize   = 32
)

var (
	fontOnce   sync.Once
	textParsed *opentype.Font
	iconParsed *opentype.Font
	fontErr    error
	textFaces  sync.Map // float64 字号 -> font.Face（思源宋体）
	iconFaces  sync.Map // float64 字号 -> font.Face（图标字体）
)

// loadFonts 解析嵌入字体文件（懒加载，失败时 fontErr 非 nil，绘制文本将跳过）。
func loadFonts() {
	textRaw, err := fontFile("SourceHanSerifCN-Bold.ttf")
	if err != nil {
		fontErr = err
		return
	}
	textParsed, err = opentype.Parse(textRaw)
	if err != nil {
		fontErr = err
		return
	}
	iconRaw, err := fontFile("warframe_font_icon.ttf")
	if err != nil {
		fontErr = err
		return
	}
	iconParsed, err = opentype.Parse(iconRaw)
	if err != nil {
		fontErr = err
		return
	}
}

// newFace 按字号创建字体 face（无缓存版本，供内部派生调用）。
func newFace(parsed *opentype.Font, size float64) font.Face {
	if parsed == nil {
		return nil
	}
	face, err := opentype.NewFace(parsed, &opentype.FaceOptions{
		Size:    size,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return nil
	}
	return face
}

// textFace 获取指定字号的正文（思源宋体）face，带缓存。
func textFace(size float64) font.Face {
	fontOnce.Do(loadFonts)
	if textParsed == nil {
		return nil
	}
	if cached, ok := textFaces.Load(size); ok {
		return cached.(font.Face)
	}
	face := newFace(textParsed, size)
	textFaces.Store(size, face)
	return face
}

// iconFace 获取指定字号的 Warframe 图标 face，带缓存。
func iconFace(size float64) font.Face {
	fontOnce.Do(loadFonts)
	if iconParsed == nil {
		return nil
	}
	if cached, ok := iconFaces.Load(size); ok {
		return cached.(font.Face)
	}
	face := newFace(iconParsed, size)
	iconFaces.Store(size, face)
	return face
}
