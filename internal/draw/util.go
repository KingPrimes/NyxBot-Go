// 绘图通用工具：颜色解析、时间格式化、派系匹配（对齐 Java TimeUtils/TimeZoneUtil/FactionEnum）
package draw

import (
	"fmt"
	"image/color"
	"strings"
	"time"

	"nyxbot-go/internal/enum/drawplugin"
)

// hexToColor 解析 "#RRGGBB" 十六进制颜色；解析失败返回灰色。
func hexToColor(hex string) color.RGBA {
	hex = strings.TrimPrefix(strings.TrimSpace(hex), "#")
	if len(hex) != 6 {
		return color.RGBA{0x78, 0x78, 0x78, 0xff}
	}
	var r, g, b uint8
	if _, err := fmt.Sscanf(hex, "%02x%02x%02x", &r, &g, &b); err != nil {
		return color.RGBA{0x78, 0x78, 0x78, 0xff}
	}
	return color.RGBA{r, g, b, 0xff}
}

// timeDeltaString 将时间差格式化为 "Xd Xh Xm Xs"（对齐 TimeUtils.timeDeltaToString）。
func timeDeltaString(d time.Duration) string {
	if d < 0 {
		d = -d
	}
	days := d / (24 * time.Hour)
	hours := d / time.Hour % 24
	minutes := d / time.Minute % 60
	seconds := d / time.Second % 60

	var sb strings.Builder
	if days > 0 {
		fmt.Fprintf(&sb, "%dd ", days)
	}
	if hours > 0 {
		fmt.Fprintf(&sb, "%dh ", hours)
	}
	if minutes > 0 {
		fmt.Fprintf(&sb, "%dm ", minutes)
	}
	fmt.Fprintf(&sb, "%ds", seconds)
	return strings.TrimSpace(sb.String())
}

// FormatTimestamp 格式化时间戳为 "2006-01-02 15:04:05"，先转换到本地时区
// （对齐 Java TimeZoneUtil.formatTimestamp 取系统时区；API 时间为 UTC，直接 Format 会差一个时区偏移）。
func FormatTimestamp(t time.Time) string {
	return t.In(time.Local).Format("2006-01-02 15:04:05")
}

// factionKeyInfo 按派系键解析显示信息；空键返回无效，未知键回退未知派系。
// 供裂隙/仲裁/入侵等按枚举键取名的绘制使用（对齐 Java FactionEnum 直接取枚举）。
func factionKeyInfo(key drawplugin.Faction) (drawplugin.FactionInfo, bool) {
	if key == "" {
		return drawplugin.FactionInfo{}, false
	}
	info, ok := drawplugin.FactionMap[key]
	if !ok {
		info = drawplugin.FactionMap[drawplugin.FactionNone]
	}
	return info, true
}

// maxF / maxI / minI 数值比较辅助（对应 Java Math.max/min 的常见调用，集中定义便于各绘制文件复用）。
func maxF(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func maxI(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minI(a, b int) int {
	if a < b {
		return a
	}
	return b
}
