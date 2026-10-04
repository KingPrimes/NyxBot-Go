// 世界状态查询（批次 B）：平原循环 AllCycle。
//
// 对齐 Java WorldStateUtils.getAllCycle：五项周期中只有需要外部时间的部分从 WorldState 读，
// 其余为纯本地计算，因此这里统一由 internal/warframe/cycle 的计算函数产出，再转换为 draw DTO。
//
// 数据来源（对齐 Java WorldState.getXxxCycle）：
//   - 夜灵平原 CetusCycle：由 SyndicateMissions 中 CetusSyndicate 的 Expiry 推出赏金结束时间
//   - 扎里曼 ZarimanCycle：由 SyndicateMissions 中 ZarimanSyndicate 的 Expiry 推出
//   - 魔胎之境 CambionCycle：由 CetusCycle 派生
//   - 地球 EarthCycle / 奥布山谷 VallisCycle：纯本地计算
package warframe

import (
	"time"

	"nyxbot-go/internal/draw"
	"nyxbot-go/internal/warframe/cycle"
)

// 赏金集团 Tag（对齐 Java SyndicateEnum 的枚举名，用于在 SyndicateMissions 中定位赏金结束时间）。
const (
	syndicateTagCetus   = "CetusSyndicate"
	syndicateTagZariman = "ZarimanSyndicate"
)

// wsCycleEnvelope 平原循环所需字段的 WorldState 视图。
type wsCycleEnvelope struct {
	SyndicateMissions []wsSyndicateMission `json:"SyndicateMissions"`
}

// wsSyndicateMission 对齐 Java model.worldstate.SyndicateMission 的 JSON 字段
// （本批次仅用到标签与结束时间；Nodes/Jobs 由三赏金批次解析）。
type wsSyndicateMission struct {
	Tag    string `json:"Tag"`
	Expiry wsTime `json:"Expiry"`
}

// GetAllCycle 组装五张平原周期卡片（对齐 Java WorldStateUtils.getAllCycle）。
// 任何一项计算失败都不影响其它项；WorldState 未就绪时仍需返回结果（各周期可本地推算）。
func GetAllCycle() (*draw.AllCycle, error) {
	// 赏金结束时间缺失时回退为当前时间（对齐 Java getBountiesEndDate 的 orElse(Instant.now())）
	cetusExpiry := syndicateBountyExpiry(syndicateTagCetus)
	zarimanExpiry := syndicateBountyExpiry(syndicateTagZariman)

	cetus := cycle.ComputeCetusCycle(cetusExpiry)
	earth := cycle.ComputeEarthCycle()
	vallis := cycle.ComputeVallisCycle()
	cambion := cycle.ComputeCambionCycle(cetus)
	zariman := cycle.ComputeZarimanCycle(zarimanExpiry)

	return &draw.AllCycle{
		EarthCycle:   earthCycleToDraw(earth),
		CetusCycle:   cetusCycleToDraw(cetus),
		CambionCycle: cambionCycleToDraw(cambion),
		VallisCycle:  vallisCycleToDraw(vallis),
		ZarimanCycle: zarimanCycleToDraw(zariman),
	}, nil
}

// syndicateBountyExpiry 返回指定集团赏金的结束时间戳（毫秒）；未找到时回退当前时间。
// 读取失败（WorldState 未就绪/解析失败）同样回退，保证平原查询不因缺数据整体失败。
func syndicateBountyExpiry(tag string) int64 {
	env, err := parseWorldStateEnvelope[wsCycleEnvelope]("cycle")
	if err != nil {
		return time.Now().UnixMilli()
	}
	for i := range env.SyndicateMissions {
		mission := &env.SyndicateMissions[i]
		if mission.Tag != tag {
			continue
		}
		if expiry := mission.Expiry.Time(); !expiry.IsZero() {
			return expiry.UnixMilli()
		}
	}
	return time.Now().UnixMilli()
}

// earthCycleToDraw 地球循环 → 绘图 DTO。
func earthCycleToDraw(src *cycle.EarthCycle) *draw.EarthCycle {
	if src == nil {
		return nil
	}
	return &draw.EarthCycle{
		Activation: parseTime(src.Activation),
		Expiry:     parseTime(src.Expiry),
		IsDay:      src.IsDay,
		State:      src.State,
		TimeLeft:   src.TimeLeft,
		Rounded:    parseTime(src.Rounded),
		Start:      parseTime(src.Start),
		Expired:    src.Expired,
	}
}

// cetusCycleToDraw 夜灵平原循环 → 绘图 DTO。
func cetusCycleToDraw(src *cycle.CetusCycle) *draw.CetusCycle {
	if src == nil {
		return nil
	}
	return &draw.CetusCycle{
		IsDay:      src.IsDay,
		Expiry:     parseTime(src.Expiry),
		Activation: parseTime(src.Activation),
		State:      src.State,
		Cycle:      src.Cycle,
		TimeLeft:   src.TimeLeft,
	}
}

// cambionCycleToDraw 魔胎之境循环 → 绘图 DTO。
func cambionCycleToDraw(src *cycle.CambionCycle) *draw.CambionCycle {
	if src == nil {
		return nil
	}
	return &draw.CambionCycle{
		Active:     src.Active,
		TimeLeft:   src.TimeLeft,
		Expiry:     parseTime(src.Expiry),
		Activation: parseTime(src.Activation),
	}
}

// vallisCycleToDraw 奥布山谷循环 → 绘图 DTO。
func vallisCycleToDraw(src *cycle.VallisCycle) *draw.VallisCycle {
	if src == nil {
		return nil
	}
	return &draw.VallisCycle{
		Activation: parseTime(src.Activation),
		Expiry:     parseTime(src.Expiry),
		IsWarm:     src.IsWarm,
		State:      src.State,
		TimeLeft:   src.TimeLeft,
		Expired:    src.Expired,
	}
}

// zarimanCycleToDraw 扎里曼循环 → 绘图 DTO。
func zarimanCycleToDraw(src *cycle.ZarimanCycle) *draw.ZarimanCycle {
	if src == nil {
		return nil
	}
	return &draw.ZarimanCycle{
		Activation: parseTime(src.Activation),
		Expiry:     parseTime(src.Expiry),
		IsCorpus:   src.IsCorpus,
		State:      src.State,
		TimeLeft:   src.TimeLeft,
		Expired:    src.Expired,
	}
}
