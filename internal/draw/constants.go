// 绘图常量：暗色主题颜色与标准尺寸（对齐 Java DefaultDraw.DrawConstants）
package draw

import "image/color"

// 底部署名文本。
const footerText = "Posted by: KingPrimes"

// 标准图像尺寸。
const (
	imageWidth     = 1200 // 标准图像宽度
	imageMargin    = 40   // 标准图像边距
	imageTitleH    = 50   // 标准图像标题高度
	imageRowHeight = 50   // 标准图像行高度
	imageMarginTop = 60   // 标准图像上边距
	imageFooterH   = 40   // 标准图像底部高度
	standardRatio  = 0.3  // 看板娘绘制比例
)

// 暗色主题颜色（对齐 DrawConstants 十六进制值）。
var (
	pageBackgroundColor = color.RGBA{0x1a, 0x1a, 0x2e, 0xff} // 页面背景 — 深海军蓝
	cardBackgroundColor = color.RGBA{0x16, 0x21, 0x3e, 0xff} // 卡片背景
	titleColor          = color.RGBA{0x45, 0x85, 0xe9, 0xff} // 标题/强调 — 蓝
	textColor           = color.RGBA{0xff, 0xff, 0xff, 0xff} // 主文本 — 白
	textSecondaryColor  = color.RGBA{0xaa, 0xaa, 0xaa, 0xff} // 次要文本 — 浅灰
	textMutedColor      = color.RGBA{0x78, 0x78, 0x78, 0xff} // 弱化文本 — 灰
	accentColor         = color.RGBA{0xe9, 0x45, 0x60, 0xff} // 强调 — 红粉
	accentGreenColor    = color.RGBA{0x45, 0xe9, 0x96, 0xff} // 绿色强调
	accentGoldColor     = color.RGBA{0xff, 0xd7, 0x00, 0xff} // 金色强调（奖励）
	dividerColor        = color.RGBA{0x3c, 0x3c, 0x5a, 0xff} // 分割线
	attackerColor       = color.RGBA{0xff, 0x6b, 0x6b, 0xff} // 进攻方/红色状态
	defenderColor       = color.RGBA{0x4c, 0xaf, 0x50, 0xff} // 防守方/绿色状态
	worthColor          = color.RGBA{0x27, 0xae, 0x60, 0xff} // 值得参与
	notWorthColor       = color.RGBA{0xe7, 0x4c, 0x3c, 0xff} // 不值得参与
	borderOuterColor    = color.RGBA{0x3c, 0x3c, 0x5a, 0xff} // 双层边框外层
	borderInnerColor    = color.RGBA{0x4a, 0x4a, 0x6a, 0xff} // 双层边框内层

	// 双衍王境情绪颜色
	emotionSadColor   = color.RGBA{74, 144, 217, 255}
	emotionFearColor  = color.RGBA{155, 89, 182, 255}
	emotionJoyColor   = color.RGBA{241, 196, 15, 255}
	emotionAngerColor = color.RGBA{231, 76, 60, 255}
	emotionEnvyColor  = color.RGBA{46, 204, 113, 255}

	// 循环冷暖状态颜色
	allCycleWarmColor = color.RGBA{0xff, 0x80, 0x00, 0xff}
	allCycleColdColor = color.RGBA{0x00, 0xb4, 0xff, 0xff}

	// 裂隙剩余时间颜色（紧急/中等/充裕）
	activeTimeLowColor    = color.RGBA{0xff, 0x6b, 0x6b, 0xff}
	activeTimeMediumColor = color.RGBA{0xaa, 0x7e, 0x1e, 0xff}
	activeTimeHighColor   = color.RGBA{0x1d, 0xd1, 0xa1, 0xff}

	// 虚空遗物等级颜色 T1-T5
	voidT1Color = color.RGBA{0x51, 0x42, 0x34, 0xff}
	voidT2Color = color.RGBA{0x75, 0x56, 0x2b, 0xff}
	voidT3Color = color.RGBA{0x9f, 0x9e, 0x9e, 0xff}
	voidT4Color = color.RGBA{0xc1, 0xbe, 0x39, 0xff}
	voidT5Color = color.RGBA{0x87, 0x2a, 0x2c, 0xff}

	// 订阅四色
	subscribeBlueColor   = color.RGBA{0x1c, 0x84, 0xc6, 0xff}
	subscribePurpleColor = color.RGBA{0x61, 0x10, 0xf3, 0xff}
	subscribeRedColor    = color.RGBA{0xff, 0x00, 0x00, 0xff}
	subscribeBrownColor  = color.RGBA{0xaf, 0x52, 0x44, 0xff}
)
