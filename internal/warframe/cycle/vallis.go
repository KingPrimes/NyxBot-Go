// 奥布山谷冷暖循环计算，对应 Java draw-image-plugin 的 VallisCycle
// 基于固定纪元时间 L_START (2018-11-10T08:13:48Z) 取模计算
// 周期 1600000ms，温暖期 400000ms，寒冷期 1200000ms
//
// 公式参考：https://github.com/WFCD/warframe-worldstate-parser
package cycle

import (
	"time"
)

type VallisCycle struct {
	Activation string `json:"activation"` // 当前状态开始时间
	Expiry     string `json:"expiry"`     // 当前状态结束时间
	IsWarm     bool   `json:"isWarm"`     // 是否为温暖期
	State      string `json:"state"`      // 状态（温暖/寒冷）
	TimeLeft   string `json:"timeLeft"`   // 剩余时间
	Expired    bool   `json:"expired"`    // 是否已过期
}

const (
	vallisLoopTime int64 = 1600000                         // 总周期时长（毫秒）
	vallisWarmTime int64 = 400000                          // 温暖期持续时间（毫秒）
	vallisColdTime int64 = vallisLoopTime - vallisWarmTime // 寒冷期持续时间
)

var vallisStart = time.Date(2018, 11, 10, 8, 13, 48, 0, time.UTC)

func ComputeVallisCycle() *VallisCycle {
	now := time.Now().UnixMilli()
	startMillis := vallisStart.UnixMilli()

	sinceLast := (now - startMillis) % vallisLoopTime
	toNextFull := vallisLoopTime - sinceLast

	state := "寒冷"
	if toNextFull > vallisColdTime {
		state = "温暖"
	}

	var toNextMinor int64
	if toNextFull < vallisColdTime {
		toNextMinor = toNextFull
	} else {
		toNextMinor = toNextFull - vallisColdTime
	}

	timeAtNext := time.UnixMilli(now + toNextMinor)
	var timeAtPrevious time.Time
	if state == "温暖" {
		timeAtPrevious = time.UnixMilli(now + toNextFull - vallisLoopTime)
	} else {
		timeAtPrevious = time.UnixMilli(now + toNextFull - vallisColdTime)
	}

	expiry := timeAtNext.Truncate(time.Second)
	activation := timeAtPrevious.Truncate(time.Second)

	return &VallisCycle{
		Activation: activation.Format(time.RFC3339),
		Expiry:     expiry.Format(time.RFC3339),
		IsWarm:     state == "温暖",
		State:      state,
		TimeLeft:   timeDeltaToString(toNextMinor),
		Expired:    expiry.Before(time.Now()),
	}
}
