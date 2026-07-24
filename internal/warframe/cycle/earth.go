// 地球昼夜循环计算，对应 Java draw-image-plugin 的 EarthCycle
// 基于时间戳取模计算：周期 28800s，白天 14400s，每 4 小时轮换
//
// 公式参考：https://github.com/WFCD/warframe-worldstate-parser
package cycle

import (
	"time"
)

type EarthCycle struct {
	Activation string `json:"activation"` // 当前状态开始时间
	Expiry     string `json:"expiry"`     // 当前状态结束时间
	IsDay      bool   `json:"isDay"`      // 是否为白昼
	State      string `json:"state"`      // 状态描述（白昼/夜晚）
	TimeLeft   string `json:"timeLeft"`   // 剩余时间
	Rounded    string `json:"rounded"`    // 圆整后的结束时间
	Start      string `json:"start"`      // 本轮循环开始时间
	Expired    bool   `json:"expired"`    // 是否已过期
}

const (
	earthCycleSeconds = 28800 // 地球周期总时长（秒）
	earthDaytimeLimit = 14400 // 白天最大持续时间（秒）
)

func ComputeEarthCycle() *EarthCycle {
	now := time.Now()
	nowUnix := now.Unix()

	cycleSeconds := nowUnix % earthCycleSeconds
	isDay := cycleSeconds < earthDaytimeLimit

	secondsLeft := earthDaytimeLimit - (cycleSeconds % earthDaytimeLimit)
	millisLeft := secondsLeft * 1000

	expiry := now.UnixMilli() + millisLeft

	minutesCoef := int64(1000 * 60)
	rounded := ((now.UnixMilli() + millisLeft) / minutesCoef) * minutesCoef
	roundedTime := time.UnixMilli(rounded)

	state := "夜晚"
	if isDay {
		state = "白昼"
	}

	start := roundedTime.Add(-4 * time.Hour)

	return &EarthCycle{
		Activation: start.Format(time.RFC3339),
		Expiry:     time.UnixMilli(expiry).Format(time.RFC3339),
		IsDay:      isDay,
		State:      state,
		TimeLeft:   timeDeltaToString(millisLeft),
		Rounded:    roundedTime.Format(time.RFC3339),
		Start:      start.Format(time.RFC3339),
		Expired:    time.UnixMilli(expiry).Before(time.Now()),
	}
}
