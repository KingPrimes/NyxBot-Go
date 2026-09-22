// 时间格式化工具，对应 Java draw-image-plugin 的 TimeUtils.timeDeltaToString
// 将毫秒差转换为 "Xd Xh Xm Xs" 可读格式
package cycle

import (
	"fmt"
	"time"
)

// TimeDeltaToString 将毫秒差转换为 "Xd Xh Xm Xs" 可读格式（对齐 Java TimeUtils.timeDeltaToString）。
// 导出供 warframe 查询层生成「剩余时间」文本。
func TimeDeltaToString(millis int64) string {
	return timeDeltaToString(millis)
}

func timeDeltaToString(millis int64) string {
	if millis < 0 {
		millis = -millis
	}
	d := time.Duration(millis) * time.Millisecond

	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	var result string
	if days > 0 {
		result += fmt.Sprintf("%dd ", days)
	}
	if hours > 0 {
		result += fmt.Sprintf("%dh ", hours)
	}
	if minutes > 0 {
		result += fmt.Sprintf("%dm ", minutes)
	}
	result += fmt.Sprintf("%ds", seconds)

	return result
}

func truncateToMinutes(millis int64) int64 {
	minutesCoef := int64(1000 * 60)
	return (millis / minutesCoef) * minutesCoef
}
