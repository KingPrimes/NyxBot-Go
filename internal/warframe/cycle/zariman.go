// 扎里曼学派循环计算，对应 Java draw-image-plugin 的 ZarimanCycle
// 基于集团赏金任务结束时间和 Corpus 纪元起始时间 (1655182800000) 计算
// 周期 18000000ms，每阶段 9000000ms，轮换 Grineer/Corpus
//
// 公式参考：https://github.com/WFCD/warframe-worldstate-parser
package cycle

import "time"

type ZarimanCycle struct {
	Activation string `json:"activation"` // 当前状态开始时间
	Expiry     string `json:"expiry"`     // 当前状态结束时间
	IsCorpus   bool   `json:"isCorpus"`   // 当前是否为 Corpus
	State      string `json:"state"`      // 状态（Grineer/Corpus）
	TimeLeft   string `json:"timeLeft"`   // 剩余时间
	Expired    bool   `json:"expired"`    // 是否已过期
}

const (
	zarimanCorpusTime int64 = 1655182800000 // Corpus 纪元起始时间（毫秒）
	zarimanFullCycle  int64 = 18000000      // 完整周期（毫秒）
	zarimanStateMax   int64 = 9000000       // 每阶段最大持续时间（毫秒）
)

// ComputeZarimanCycle 计算扎里曼循环
// bountiesEndDate: 坚守者集团赏金任务结束时间戳（毫秒）
func ComputeZarimanCycle(bountiesEndDate int64) *ZarimanCycle {
	now := time.Now().UnixMilli()
	bountiesClone := bountiesEndDate - 5000
	millisLeft := bountiesClone - now

	cycleTimeElapsed := ((bountiesClone-zarimanCorpusTime)%zarimanFullCycle + zarimanFullCycle) % zarimanFullCycle
	cycleTimeLeft := zarimanFullCycle - cycleTimeElapsed
	isCorpus := cycleTimeLeft > zarimanStateMax

	state := "Grineer"
	if isCorpus {
		state = "Corpus"
	}

	minutesCoef := int64(1000 * 60)
	expiry := ((now + millisLeft) / minutesCoef) * minutesCoef
	activation := expiry - zarimanStateMax

	return &ZarimanCycle{
		Activation: time.UnixMilli(activation).Format(time.RFC3339),
		Expiry:     time.UnixMilli(expiry).Format(time.RFC3339),
		IsCorpus:   isCorpus,
		State:      state,
		TimeLeft:   timeDeltaToString(millisLeft),
		Expired:    time.UnixMilli(expiry).Before(time.Now()),
	}
}
