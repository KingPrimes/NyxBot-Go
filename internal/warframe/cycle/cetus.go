// 夜灵平原昼夜循环计算，对应 Java draw-image-plugin 的 CetusCycle
// 基于赏金任务结束时间计算：夜晚持续 3000s，白天/夜晚各有限值
//
// 公式参考：https://github.com/WFCD/warframe-worldstate-parser
package cycle

import (
	"math"
	"time"
)

type CetusCycle struct {
	IsDay    bool   `json:"isDay"`    // 是否为白昼
	Expiry   string `json:"expiry"`   // 当前状态结束时间
	Activation string `json:"activation"` // 当前状态开始时间
	State    string `json:"state"`    // 状态（白昼/夜晚）
	Cycle    string `json:"cycle"`    // 状态英文（day/night）
	TimeLeft string `json:"timeLeft"` // 剩余时间
}

const (
	cetusNightTime     = 3000               // 夜晚持续时间（秒）
	cetusDayMax  int64 = 6000000            // 白天最大持续时间（毫秒）
	cetusNightMax int64 = 3000000           // 夜晚最大持续时间（毫秒）
)

// ComputeCetusCycle 计算夜灵平原循环
// bountiesEndDate: Cetus 集团赏金任务结束时间戳（毫秒）
func ComputeCetusCycle(bountiesEndDate int64) *CetusCycle {
	now := time.Now().UnixMilli()
	bountiesClone := (bountiesEndDate / 1000) * 1000 // truncate to seconds

	millisLeft := bountiesClone - now
	secondsToNightEnd := int64(math.Round(float64(millisLeft) / 1000))
	dayTime := secondsToNightEnd > cetusNightTime

	var secondsRemainingInCycle int64
	if dayTime {
		secondsRemainingInCycle = secondsToNightEnd - cetusNightTime
	} else {
		secondsRemainingInCycle = secondsToNightEnd
	}
	millisLeft = secondsRemainingInCycle * 1000

	expiryUnix := truncateToMinutes(now + millisLeft)
	expiry := time.UnixMilli(expiryUnix)

	var state, cycle string
	if dayTime {
		state = "白昼"
		cycle = "day"
	} else {
		state = "夜晚"
		cycle = "night"
	}

	var activationMax int64
	if dayTime {
		activationMax = cetusDayMax
	} else {
		activationMax = cetusNightMax
	}
	activation := expiry.Add(-time.Duration(activationMax) * time.Millisecond)

	return &CetusCycle{
		IsDay:      dayTime,
		Expiry:     expiry.Format(time.RFC3339),
		Activation: activation.Format(time.RFC3339),
		State:      state,
		Cycle:      cycle,
		TimeLeft:   timeDeltaToString(millisLeft),
	}
}
