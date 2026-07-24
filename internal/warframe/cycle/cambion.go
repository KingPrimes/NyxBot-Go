// 魔胎之境循环计算，对应 Java draw-image-plugin 的 CambionCycle
// 直接从夜灵平原循环推导：白天 = FASS，夜晚 = VOME
package cycle

type CambionCycle struct {
	Active     string `json:"active"`     // 当前活跃实体（FASS/VOME）
	TimeLeft   string `json:"timeLeft"`   // 剩余时间
	Expiry     string `json:"expiry"`     // 结束时间
	Activation string `json:"activation"` // 开始时间
}

// ComputeCambionCycle 根据夜灵平原循环计算魔胎之境循环
func ComputeCambionCycle(cetus *CetusCycle) *CambionCycle {
	active := "VOME"
	if cetus.IsDay {
		active = "FASS"
	}

	return &CambionCycle{
		Active:     active,
		TimeLeft:   cetus.TimeLeft,
		Expiry:     cetus.Expiry,
		Activation: cetus.Activation,
	}
}
