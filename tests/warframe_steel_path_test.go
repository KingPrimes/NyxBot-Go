// 钢铁奖励轮换边界回归测试（GetSteelPath 为纯本地计算，不读写数据库、不访问网络）。
package tests

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"nyxbot-go/internal/warframe"
)

// TestGetSteelPathRemainingUsesUTCBoundary 验证剩余时间以「下周一 00:00 UTC」为界。
//
// 回归背景：轮换基准 steelPathStartDate = 2020-11-16 00:00 UTC（周一），周索引用
// now.UTC() 计算；边界若改用输入时刻的本地时区（旧实现），在非 UTC 机器上会整体
// 偏移一个时区差（如 Asia/Shanghai 提前 8 小时归零）。测试把 time.Local 固定为 UTC+8，
// 使断言在任何机器上都具备区分力（本机若为 UTC，两种实现等价、无法区分）。
func TestGetSteelPathRemainingUsesUTCBoundary(t *testing.T) {
	saved := time.Local
	time.Local = time.FixedZone("TEST+08", 8*3600)
	t.Cleanup(func() { time.Local = saved })

	before := time.Now()
	offering := warframe.GetSteelPath()
	after := time.Now()
	if offering == nil || offering.Remaining == "" {
		t.Fatal("钢铁奖励剩余时间不应为空")
	}

	got := parseDeltaText(t, offering.Remaining)
	// 内部取时介于 before/after 之间，剩余时间按秒取整，故留 2s 容差
	low := untilNextUTCMonday(after) - 2*time.Second
	high := untilNextUTCMonday(before) + 2*time.Second
	if got < low || got > high {
		t.Fatalf("剩余时间 %v (%q) 不在 [%v, %v]：周边界未按 UTC 计算（本地时区 %v）",
			got, offering.Remaining, low, high, time.Local)
	}
	if got > 7*24*time.Hour {
		t.Fatalf("剩余时间 %v 超过一个 7 天轮换周期", got)
	}
}

// untilNextUTCMonday 返回从 t 到下周一 00:00 UTC 的时长（边界规格，用于比对实现）。
func untilNextUTCMonday(t time.Time) time.Duration {
	utc := t.UTC()
	weekday := int(utc.Weekday()) // Sunday = 0
	if weekday == 0 {
		weekday = 7
	}
	startOfDay := time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
	return startOfDay.AddDate(0, 0, 8-weekday).Sub(t)
}

// parseDeltaText 解析 "Xd Xh Xm Xs" 形态的剩余时间文本（对齐 cycle.TimeDeltaToString）。
func parseDeltaText(t *testing.T, text string) time.Duration {
	t.Helper()
	var total time.Duration
	for _, field := range strings.Fields(text) {
		if len(field) < 2 {
			t.Fatalf("剩余时间文本片段非法: %q（原文 %q）", field, text)
		}
		value, err := strconv.Atoi(field[:len(field)-1])
		if err != nil || value < 0 {
			t.Fatalf("剩余时间文本 %q 解析失败: %v", text, err)
		}
		switch field[len(field)-1] {
		case 'd':
			total += time.Duration(value) * 24 * time.Hour
		case 'h':
			total += time.Duration(value) * time.Hour
		case 'm':
			total += time.Duration(value) * time.Minute
		case 's':
			total += time.Duration(value) * time.Second
		default:
			t.Fatalf("剩余时间文本 %q 含未知单位: %q", text, field)
		}
	}
	return total
}
