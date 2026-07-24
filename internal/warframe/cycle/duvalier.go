// 双衍王境情绪循环计算，对应 Java draw-image-plugin 的 DuvalierCycle
// 基于时间戳取模：周期 36000s，每个情绪 7200s，共 5 种情绪
// 情绪顺序：悲伤 -> 恐惧 -> 喜悦 -> 愤怒 -> 嫉妒
//
// 公式参考：https://github.com/WFCD/warframe-worldstate-parser
package cycle

import (
	"time"
)

type DuvalierCycle struct {
	Activation string `json:"activation"` // 当前状态开始时间
	Expiry     string `json:"expiry"`     // 当前状态结束时间
	State      string `json:"state"`      // 当前情绪
	Schedules  any    `json:"schedules"`  // 当前循环可选内容
	TimeLeft   string `json:"timeLeft"`   // 剩余时间
}

type DuvalierSchedule struct {
	CategoryChoices []DuvalierChoice `json:"CategoryChoices"`
}

type DuvalierChoice struct {
	Category string   `json:"Category"`
	Choices  []string `json:"Choices"`
}

const (
	duvalierCycleTime  int64 = 36000 // 总周期时长（秒）
	duvalierStateTime  int64 = 7200  // 每个阶段持续时间（秒）
)

var duvalierStates = []string{"悲伤", "恐惧", "喜悦", "愤怒", "嫉妒"}

func ComputeDuvalierCycle() *DuvalierCycle {
	now := time.Now()
	nowSeconds := now.Unix()

	cycleDelta := (nowSeconds - 52) % duvalierCycleTime
	stateInd := cycleDelta / duvalierStateTime
	stateDelta := cycleDelta % duvalierStateTime
	untilNext := duvalierStateTime - stateDelta

	state := duvalierStates[stateInd]
	expiry := now.Add(time.Duration(untilNext) * time.Second).Truncate(time.Second)
	activation := expiry.Add(-time.Duration(duvalierStateTime) * time.Second)

	return &DuvalierCycle{
		Activation: activation.Format(time.RFC3339),
		Expiry:     expiry.Format(time.RFC3339),
		State:      state,
		TimeLeft:   timeDeltaToString(expiry.UnixMilli() - time.Now().UnixMilli()),
	}
}
