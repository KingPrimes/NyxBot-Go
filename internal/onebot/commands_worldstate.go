// 阶段10 世界状态类指令（批次 A-D）：警报、执刑官猎杀、钢铁奖励、平原、
// 三赏金（希图斯/英择谛/索拉里斯）、双衍轮换、电波、1999 日历。
// 对齐 Java AlertsPlugin / LiteSoritePlugin / SteelPathPlugin / AllCyclePlugin /
// Syndicate*Plugin / DuviriCyclePlugin / NightWavePlugin / KnownCalendarSeasonsPlugin。
package onebot

import (
	zero "github.com/wdvxdr1123/ZeroBot"

	"nyxbot-go/internal/draw"
	"nyxbot-go/internal/warframe"
)

// wfAlerts 处理「警报」：展示当前警报列表图。
func (registry *CommandRegistry) wfAlerts(ctx *zero.Ctx, _ string) error {
	alerts, err := warframe.GetAlerts()
	if err != nil {
		return ReplyText(ctx, err.Error())
	}
	if len(alerts) == 0 {
		return ReplyText(ctx, "暂无进行中的警报")
	}
	return ReplyImage(ctx, draw.DrawAlerts(alerts))
}

// wfLiteSortie 处理「执刑官猎杀/猎杀/执行官/执政官/执刑官」：展示当前执刑官猎杀图。
func (registry *CommandRegistry) wfLiteSortie(ctx *zero.Ctx, _ string) error {
	list, err := warframe.GetLiteSorite()
	if err != nil {
		return ReplyText(ctx, err.Error())
	}
	if len(list) == 0 {
		return ReplyText(ctx, "暂无执刑官猎杀数据")
	}
	return ReplyImage(ctx, draw.DrawLiteSorite(list[0]))
}

// wfSteelPath 处理「钢铁奖励」：展示本周钢铁奖励轮换（纯本地计算，无需 WorldState）。
func (registry *CommandRegistry) wfSteelPath(ctx *zero.Ctx, _ string) error {
	image := draw.DrawSteelPath(warframe.GetSteelPath())
	if len(image) == 0 {
		return ReplyText(ctx, "生成钢铁奖励图片失败")
	}
	return ReplyImage(ctx, image)
}

// wfAllCycle 处理「平原/夜灵平原/福尔图娜/魔胎之境/扎里曼」：展示五张平原周期卡片。
func (registry *CommandRegistry) wfAllCycle(ctx *zero.Ctx, _ string) error {
	allCycle, err := warframe.GetAllCycle()
	if err != nil {
		return ReplyText(ctx, err.Error())
	}
	image := draw.DrawAllCycle(allCycle)
	if len(image) == 0 {
		return ReplyText(ctx, "生成平原周期图片失败")
	}
	return ReplyImage(ctx, image)
}

// wfSyndicateOstrons 处理「希图斯/地球赏金」：展示 Ostrons 集团赏金图。
func (registry *CommandRegistry) wfSyndicateOstrons(ctx *zero.Ctx, _ string) error {
	return registry.syndicateImage(ctx, warframe.SyndicateOstrons, "希图斯")
}

// wfSyndicateEntrati 处理「英择谛/火卫二赏金」：展示英择谛集团赏金图。
func (registry *CommandRegistry) wfSyndicateEntrati(ctx *zero.Ctx, _ string) error {
	return registry.syndicateImage(ctx, warframe.SyndicateEntrati, "英择谛")
}

// wfSyndicateSolarisUnited 处理「索拉里斯/金星赏金」：展示索拉里斯联盟赏金图。
func (registry *CommandRegistry) wfSyndicateSolarisUnited(ctx *zero.Ctx, _ string) error {
	return registry.syndicateImage(ctx, warframe.SyndicateSolarisUnited, "索拉里斯")
}

// syndicateImage 三赏金共用实现（对齐 Java AbstractSyndicatePlugin.handle）。
func (registry *CommandRegistry) syndicateImage(ctx *zero.Ctx, syndicate warframe.WfSyndicate, displayName string) error {
	mission, err := warframe.GetSyndicate(syndicate)
	if err != nil {
		return ReplyText(ctx, err.Error())
	}
	if mission == nil || (len(mission.Nodes) == 0 && len(mission.Jobs) == 0) {
		return ReplyText(ctx, "暂无"+displayName+"赏金数据")
	}
	image := draw.DrawSyndicateImage(mission)
	if len(image) == 0 {
		return ReplyText(ctx, "生成"+displayName+"赏金图片失败")
	}
	return ReplyImage(ctx, image)
}

// wfDuviriCycle 处理「轮换/双衍王境」：展示双衍情绪与选择卡。
func (registry *CommandRegistry) wfDuviriCycle(ctx *zero.Ctx, _ string) error {
	duviri, err := warframe.GetDuviriCycle()
	if err != nil {
		return ReplyText(ctx, err.Error())
	}
	image := draw.DrawDuviriCycle(duviri)
	if len(image) == 0 {
		return ReplyText(ctx, "生成双衍轮换图片失败")
	}
	return ReplyImage(ctx, image)
}

// wfNightWave 处理「电波」：展示当前电波挑战双列卡片。
func (registry *CommandRegistry) wfNightWave(ctx *zero.Ctx, _ string) error {
	info, err := warframe.GetSeasonInfo()
	if err != nil {
		return ReplyText(ctx, err.Error())
	}
	if info == nil || len(info.ActiveChallenges) == 0 {
		return ReplyText(ctx, "暂无电波挑战数据")
	}
	image := draw.DrawSeasonInfo(info)
	if len(image) == 0 {
		return ReplyText(ctx, "生成电波图片失败")
	}
	return ReplyImage(ctx, image)
}

// wfKnownCalendarSeasons 处理「1999/日历」：展示 1999 日历月卡（多季节时取首个）。
func (registry *CommandRegistry) wfKnownCalendarSeasons(ctx *zero.Ctx, _ string) error {
	list, err := warframe.GetKnownCalendarSeasons()
	if err != nil {
		return ReplyText(ctx, err.Error())
	}
	if len(list) == 0 {
		return ReplyText(ctx, "暂无 1999 日历数据")
	}
	image := draw.DrawKnownCalendarSeasons(list)
	if len(image) == 0 {
		return ReplyText(ctx, "生成 1999 日历图片失败")
	}
	return ReplyImage(ctx, image)
}
