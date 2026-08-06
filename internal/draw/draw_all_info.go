// 系统信息图片：CPU/JVM/系统/版本/磁盘信息卡片两列流式布局 + 看板娘
// 对齐 Java DefaultDrawAllInfoImage
package draw

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"
)

// 系统信息图布局常量（对齐 Java DefaultDrawAllInfoImage 私有常量）。
const (
	allInfoContentX      float64 = 50  // 内容区左边界
	allInfoColGap        float64 = 20  // 卡片列间距
	allInfoCardRadius    float64 = 14  // 卡片圆角半径
	allInfoCardPad       float64 = 20  // 卡片内边距
	allInfoTitleY        float64 = 80  // 标题基线 Y
	allInfoContentStartY float64 = 150 // 首行卡片起始 Y
	allInfoCols          int     = 2   // 卡片列数
)

// AllInfo 系统综合信息绘图输入（对齐 Java model.AllInfo 原始字段）。
type AllInfo struct {
	CpuInfo        *CpuInfo        // CPU 信息
	JvmInfo        *JvmInfo        // JVM 信息
	SystemInfo     *SystemInfo     // 系统信息
	SysFileInfos   *SysFileInfos   // 盘符信息
	PackageVersion *PackageVersion // 项目版本
}

// CpuInfo CPU 信息（对齐 Java model.CpuInfo）。
type CpuInfo struct {
	Model     string  // CPU 型号
	Cores     int     // CPU 核心数
	Threads   int     // CPU 线程数
	Frequency float64 // CPU 频率（GHz）
	CacheSize int     // CPU 缓存大小（KB）
	UserUsage float64 // CPU 用户使用率
	WaitUsage float64 // CPU 等待率
	SysUsage  float64 // CPU 系统使用率
	IdleUsage float64 // CPU 空闲率
}

// JvmInfo JVM 信息（对齐 Java model.JvmInfo）。
type JvmInfo struct {
	Version         string  // JVM 版本
	MaxMemory       int64   // JVM 最大内存（字节）
	UsedMemory      int64   // JVM 已用内存（字节）
	FreeMemory      int64   // JVM 空闲内存（字节）
	UsedMemoryRatio float64 // JVM 内存使用率
	FreeMemoryRatio float64 // JVM 空闲内存占比
}

// SystemInfo 系统信息（对齐 Java model.SystemInfo）。
type SystemInfo struct {
	OsName       string // 操作系统名称
	OsArch       string // 操作系统架构
	ComputerName string // 计算机名称
	ComputerIp   string // 计算机 IP 地址
}

// SysFileInfos 系统盘符信息（对齐 Java model.SysFileInfos）。
type SysFileInfos struct {
	SysFileInfos []*SysFileInfo // 盘符列表
}

// SysFileInfo 单个盘符信息（对齐 Java model.SysFileInfos.SysFileInfo）。
type SysFileInfo struct {
	DirName  string // 盘符路径
	TypeName string // 盘符类型
	FileType string // 文件类型
	Total    *int64 // 总大小（字节，可空）
	Used     *int64 // 已使用（字节，可空）
}

// PackageVersion 项目版本信息（对齐 Java AllInfo.PackageVersion record）。
type PackageVersion struct {
	Name    string // 项目名称
	Version string // 项目版本
}

// DrawAllInfo 绘制系统信息图（info 为空或无信息卡片时返回 nil，对齐 Java drawAllInfoImage）。
func DrawAllInfo(info *AllInfo) []byte {
	if info == nil {
		return nil
	}
	cards := buildInfoCards(info)
	if len(cards) == 0 {
		return nil
	}

	n := len(cards)
	isOdd := n%allInfoCols != 0
	cardW := 562.0
	textW := cardW - allInfoCardPad*2
	canvasW := allInfoContentX + float64(allInfoCols)*cardW + allInfoColGap + allInfoContentX

	colX := [2]float64{allInfoContentX, allInfoContentX + cardW + allInfoColGap}

	// 预计算每张卡片高度（对齐 Java cardHeights）
	cardHeights := make([]float64, n)
	for i, card := range cards {
		cardHeights[i] = card.height()
	}

	// 列流式 Y 终点（对齐 Java colEndY 累加后回退一列间距）
	colEndY := [2]float64{allInfoContentStartY, allInfoContentStartY}
	for i := 0; i < n; i++ {
		col := i % allInfoCols
		colEndY[col] += cardHeights[i] + allInfoColGap
	}
	for c := 0; c < allInfoCols; c++ {
		if colEndY[c] > allInfoContentStartY {
			colEndY[c] -= allInfoColGap
		}
	}

	var canvasH float64
	if isOdd {
		tallerEnd := maxF(colEndY[0], colEndY[1])
		canvasH = maxF(tallerEnd, colEndY[1]+cardW)
	} else {
		canvasH = maxF(colEndY[0], colEndY[1]) + 10 + cardW
	}

	canvas := NewCanvas(int(canvasW), int(canvasH))
	canvas.SetColor(pageBackgroundColor).FillRect(0, 0, canvasW, canvasH)
	canvas.DrawTooRoundRect()

	// 标题
	canvas.SetColor(titleColor).SetFontSize(44)
	canvas.AddCenteredText("系统信息", allInfoTitleY)
	canvas.SetColor(dividerColor).DrawLine(allInfoContentX, allInfoTitleY+50, canvasW-allInfoContentX, allInfoTitleY+50)

	// 逐列绘制卡片
	drawY := [2]float64{allInfoContentStartY, allInfoContentStartY}
	for i, card := range cards {
		col := i % allInfoCols
		card.draw(canvas, colX[col], drawY[col], cardW, cardHeights[i], textW)
		drawY[col] += cardHeights[i] + allInfoColGap
	}

	// 看板娘 + 底部签名
	standingX := colX[1]
	standingY := canvasH - cardW
	if isOdd {
		standingY = colEndY[1]
	}
	canvas.DrawStandingAt(standingX, standingY, cardW, cardW)
	canvas.AddFooter(canvasH - 25)
	data, err := canvas.PNG()
	if err != nil {
		return nil
	}
	return data
}

// buildInfoCards 收集待绘制信息卡片（对齐 Java getInfoCards：仅非空模型入列）。
func buildInfoCards(info *AllInfo) []infoCard {
	var cards []infoCard
	if info.CpuInfo != nil {
		cards = append(cards, cpuInfoCard{cpu: info.CpuInfo})
	}
	if info.PackageVersion != nil {
		cards = append(cards, versionInfoCard{pkg: info.PackageVersion})
	}
	if info.JvmInfo != nil {
		cards = append(cards, jvmInfoCard{jvm: info.JvmInfo})
	}
	if info.SystemInfo != nil {
		cards = append(cards, systemInfoCard{si: info.SystemInfo})
	}
	if info.SysFileInfos != nil {
		for _, fi := range info.SysFileInfos.SysFileInfos {
			cards = append(cards, diskInfoCard{fi: fi})
		}
	}
	return cards
}

// infoCard 信息卡片绘制单元（对齐 Java InfoCard：先算高度、再定点绘制）。
type infoCard interface {
	height() float64
	draw(canvas *Canvas, x, y, w, h, tw float64)
}

// infoCardHeight 卡片高度（对齐 Java cardPad：CARD_PAD*2 + 10 + 行数*34 + 40）。
func infoCardHeight(lines int) float64 {
	return allInfoCardPad*2 + 10 + float64(lines)*34 + 40
}

// drawInfoCardHeader 绘制卡片头部（对齐 Java drawHeader：背景 + 顶部强调条 + 边框 + 标题）。
func drawInfoCardHeader(canvas *Canvas, x, y, w, h float64, title string, accent color.RGBA) {
	canvas.SetColor(cardBackgroundColor).FillRoundRect(x, y, w, h, allInfoCardRadius)
	canvas.SetColor(accent).FillRect(x+allInfoCardRadius, y+2, w-allInfoCardRadius*2, 5)
	canvas.SetColor(dividerColor).SetStroke(1).DrawRoundRect(x, y, w, h, allInfoCardRadius)
	canvas.SetColor(accent).SetFontSize(24)
	canvas.AddText(title, x+allInfoCardPad, y+allInfoCardPad+28)
}

// drawInfoCardLine 绘制卡片内容行（对齐 Java drawLine：左列文本 + 可选右列文本）。
func drawInfoCardLine(canvas *Canvas, x, y, tw float64, label, value, label2, value2 string, c color.RGBA) {
	ix := x + allInfoCardPad
	canvas.SetColor(c).AddText(label+value, ix, y)
	if label2 != "" {
		canvas.AddText(label2+value2, ix+tw/2+20, y)
	}
}

// infoFormat 数值格式化（对齐 Java DecimalFormat "0.00"：ALL_INFO_PERCENT_FORMAT / ALL_INFO_MEMORY_FORMAT）。
func infoFormat(v float64) string {
	return fmt.Sprintf("%.2f", v)
}

// javaDouble 浮点转字符串（对齐 Java String.valueOf(double)：最短表示，整数值补 ".0"）。
func javaDouble(v float64) string {
	s := strconv.FormatFloat(v, 'g', -1, 64)
	if !strings.ContainsAny(s, ".eE") {
		return s + ".0"
	}
	return s
}

// cpuInfoCard CPU 信息卡片（对齐 Java CpuCard，6 行内容）。
type cpuInfoCard struct {
	cpu *CpuInfo
}

// height 卡片高度。
func (c cpuInfoCard) height() float64 { return infoCardHeight(6) }

// draw 绘制 CPU 卡片内容。
func (c cpuInfoCard) draw(canvas *Canvas, x, y, w, h, tw float64) {
	drawInfoCardHeader(canvas, x, y, w, h, "CPU 信息", titleColor)
	f := 18.0
	cy := y + allInfoCardPad + 42
	model := c.cpu.Model
	if model == "" {
		model = "未知"
	}
	if r := []rune(model); len(r) > 28 {
		model = string(r[:26]) + ".."
	}
	canvas.SetColor(textColor).SetFontSize(f)
	cy += 34 // 第一行：型号
	canvas.AddText("型号 "+model, x+allInfoCardPad, cy)
	cy += 34 // 第二行：核心/线程
	drawInfoCardLine(canvas, x, cy, tw, "核心 ", strconv.Itoa(c.cpu.Cores), "线程 ", strconv.Itoa(c.cpu.Threads), textColor)
	cy += 34 // 第三行：频率/缓存
	drawInfoCardLine(canvas, x, cy, tw, "频率 ", javaDouble(c.cpu.Frequency)+" GHz", "缓存 ", strconv.Itoa(c.cpu.CacheSize)+" KB", textSecondaryColor)
	cy += 34 // 第四行：用户/系统使用率
	drawInfoCardLine(canvas, x, cy, tw, "用户 ", infoFormat(c.cpu.UserUsage)+"%", "系统 ", infoFormat(c.cpu.SysUsage)+"%", accentGoldColor)
	cy += 34 // 第五行：等待/空闲
	drawInfoCardLine(canvas, x, cy, tw, "等待 ", infoFormat(c.cpu.WaitUsage)+"%", "空闲 ", infoFormat(c.cpu.IdleUsage)+"%", textSecondaryColor)
}

// jvmInfoCard JVM 信息卡片（对齐 Java JvmCard，4 行内容）。
type jvmInfoCard struct {
	jvm *JvmInfo
}

// height 卡片高度。
func (c jvmInfoCard) height() float64 { return infoCardHeight(4) }

// draw 绘制 JVM 卡片内容。
func (c jvmInfoCard) draw(canvas *Canvas, x, y, w, h, tw float64) {
	drawInfoCardHeader(canvas, x, y, w, h, "JVM 信息", color.RGBA{0xE6, 0x7E, 0x22, 0xff})
	f := 18.0
	cy := y + allInfoCardPad + 42
	version := c.jvm.Version
	if version == "" {
		version = "未知"
	}
	canvas.SetColor(textColor).SetFontSize(f)
	cy += 34 // 第一行：版本
	canvas.AddText("版本 "+version, x+allInfoCardPad, cy)
	maxMB := c.jvm.MaxMemory / (1024 * 1024)
	usedMB := c.jvm.UsedMemory / (1024 * 1024)
	freeMB := c.jvm.FreeMemory / (1024 * 1024)
	cy += 34 // 第二行：最大/已用内存
	drawInfoCardLine(canvas, x, cy, tw, "最大 ", strconv.FormatInt(maxMB, 10)+" MB", "已用 ", strconv.FormatInt(usedMB, 10)+" MB", textColor)
	canvas.SetColor(textSecondaryColor)
	cy += 34 // 第三行：空闲内存
	canvas.AddText("空闲 "+strconv.FormatInt(freeMB, 10)+" MB", x+allInfoCardPad, cy)
	cy += 34 // 第四行：使用率/空闲率
	drawInfoCardLine(canvas, x, cy, tw, "使用率 ", infoFormat(c.jvm.UsedMemoryRatio)+"%", "空闲率 ", infoFormat(c.jvm.FreeMemoryRatio)+"%", accentGoldColor)
}

// versionInfoCard 版本信息卡片（对齐 Java VersionCard，1 行内容）。
type versionInfoCard struct {
	pkg *PackageVersion
}

// height 卡片高度。
func (c versionInfoCard) height() float64 { return infoCardHeight(1) }

// draw 绘制版本卡片内容。
func (c versionInfoCard) draw(canvas *Canvas, x, y, w, h, tw float64) {
	drawInfoCardHeader(canvas, x, y, w, h, "版本信息", color.RGBA{0x34, 0x98, 0xDB, 0xff})
	f := 20.0
	cy := y + allInfoCardPad + 42
	canvas.SetColor(textColor).SetFontSize(f)
	cy += 34 // 第一行：名称/版本
	canvas.AddText("名称 "+c.pkg.Name, x+allInfoCardPad, cy)
	canvas.SetColor(textSecondaryColor)
	canvas.AddText("版本 "+c.pkg.Version, x+allInfoCardPad+tw/2+20, cy)
}

// systemInfoCard 系统信息卡片（对齐 Java SystemCard，2 行内容）。
type systemInfoCard struct {
	si *SystemInfo
}

// height 卡片高度。
func (c systemInfoCard) height() float64 { return infoCardHeight(2) }

// draw 绘制系统卡片内容。
func (c systemInfoCard) draw(canvas *Canvas, x, y, w, h, tw float64) {
	drawInfoCardHeader(canvas, x, y, w, h, "系统信息", color.RGBA{0x27, 0xAE, 0x60, 0xff})
	f := 20.0
	cy := y + allInfoCardPad + 42
	canvas.SetFontSize(f)
	cy += 34 // 第一行：OS/架构
	drawInfoCardLine(canvas, x, cy, tw, "OS ", c.si.OsName, "架构 ", c.si.OsArch, textColor)
	cy += 34 // 第二行：计算机/IP
	drawInfoCardLine(canvas, x, cy, tw, "计算机 ", c.si.ComputerName, "IP ", c.si.ComputerIp, textSecondaryColor)
}

// diskInfoCard 磁盘信息卡片（对齐 Java DiskCard，2 行内容）。
type diskInfoCard struct {
	fi *SysFileInfo
}

// height 卡片高度。
func (c diskInfoCard) height() float64 { return infoCardHeight(2) }

// draw 绘制磁盘卡片内容。
func (c diskInfoCard) draw(canvas *Canvas, x, y, w, h, tw float64) {
	drawInfoCardHeader(canvas, x, y, w, h, "磁盘 "+c.fi.DirName, color.RGBA{0x9B, 0x59, 0xB6, 0xff})
	f := 20.0
	cy := y + allInfoCardPad + 42
	canvas.SetFontSize(f)
	cy += 34 // 第一行：类型/文件系统
	drawInfoCardLine(canvas, x, cy, tw, "类型 ", c.fi.TypeName, "文件系统 ", c.fi.FileType, textColor)
	// 总大小与已使用换算为 GB（对齐 Java 按 1024^3 换算，可空字段按 0 处理）
	var total, used float64
	if c.fi.Total != nil {
		total = float64(*c.fi.Total) / (1024.0 * 1024.0 * 1024.0)
	}
	if c.fi.Used != nil {
		used = float64(*c.fi.Used) / (1024.0 * 1024.0 * 1024.0)
	}
	pct := 0.0
	if total > 0 {
		pct = used / total * 100
	}
	cy += 34 // 第二行：总量/已用
	drawInfoCardLine(canvas, x, cy, tw, "总 ", infoFormat(total)+" GB", "已用 ", infoFormat(used)+" GB ("+infoFormat(pct)+"%)", accentGoldColor)
}
