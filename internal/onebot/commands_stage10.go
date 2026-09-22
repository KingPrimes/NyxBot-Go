// 阶段10 Warframe Bot 指令实现（首批：仲裁、仲裁表、遗物查询）。
// 依赖阶段 7 的 CommandRegistry 框架、阶段 8 的 warframe 数据层、阶段 9 的 draw 绘图包。
package onebot

import (
	"errors"
	"strings"

	zero "github.com/wdvxdr1123/ZeroBot"
	"gorm.io/gorm"

	"nyxbot-go/internal/database"
	"nyxbot-go/internal/draw"
	"nyxbot-go/internal/enum/nyxbot"
	"nyxbot-go/internal/logging"
	modelwarframe "nyxbot-go/internal/model/warframe"
	"nyxbot-go/internal/warframe"
)

// installStageCommands 把已实现的阶段 10 指令处理器挂到注册表（Register 前）。
// 未实现的指令保持 notImplemented，待后续批次补齐。
func (registry *CommandRegistry) installStageCommands() {
	handlers := map[nyxbot.Codes]CommandHandler{
		nyxbot.CmdWfArbitration:          registry.wfArbitration,
		nyxbot.CmdWfArbitrationEx:        registry.wfArbitrationEx,
		nyxbot.CmdWfRelics:               registry.wfRelics,
		nyxbot.CmdWfActiveMission:        registry.wfActiveMission,
		nyxbot.CmdWfActiveMissionPath:    registry.wfActiveMissionPath,
		nyxbot.CmdWfVoidStorms:           registry.wfVoidStorms,
		nyxbot.CmdWfInvasions:            registry.wfInvasions,
		nyxbot.CmdWfVoid:                 registry.wfVoidTrader,
		nyxbot.CmdWfDailyDeals:           registry.wfDailyDeals,
		nyxbot.CmdWfSorties:              registry.wfSorties,
		nyxbot.CmdWfSubscribe:            registry.wfSubscribe,
		nyxbot.CmdWfUnsubscribe:          registry.wfUnsubscribe,
		nyxbot.CmdUpdateWfResMarketItems: registry.wfUpdateMarketItems,
		nyxbot.CmdUpdateWfResMarketRiven: registry.wfUpdateMarketRiven,
		nyxbot.CmdUpdateWfSister:         registry.wfUpdateLichSister,
		nyxbot.CmdUpdateWfTar:            registry.wfUpdateTranslation,
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	for code, handler := range handlers {
		if _, ok := nyxbot.CodesInfo[code]; ok {
			registry.handlers[code] = handler
		}
	}
}

// wfArbitration 处理「仲裁」：展示当前正在进行且最接近到期的仲裁图。
func (registry *CommandRegistry) wfArbitration(ctx *zero.Ctx, _ string) error {
	arb, ok := warframe.DefaultArbitration().GetArbitration()
	if !ok {
		return ReplyText(ctx, "暂无当前仲裁数据")
	}
	return ReplyImage(ctx, draw.DrawArbitration(warframe.ArbitrationToDraw(arb)))
}

// wfArbitrationEx 处理「仲裁表」：展示未来值得参与的仲裁列表图。
func (registry *CommandRegistry) wfArbitrationEx(ctx *zero.Ctx, _ string) error {
	list := warframe.DefaultArbitration().GetArbitrationList()
	if len(list) == 0 {
		return ReplyText(ctx, "暂无待参与的仲裁数据")
	}
	dtos := make([]*draw.Arbitration, 0, len(list))
	for i := range list {
		dtos = append(dtos, warframe.ArbitrationToDraw(list[i]))
	}
	return ReplyImage(ctx, draw.DrawArbitrations(dtos))
}

// wfRelics 处理「核桃/查核桃」：按名称查询遗物（纯本地 relics 表）并绘图。
// 参数为换取指令前缀后剩余内容（遗物名称）。
func (registry *CommandRegistry) wfRelics(ctx *zero.Ctx, parameter string) error {
	name := strings.TrimSpace(parameter)
	if name == "" {
		return ReplyText(ctx, "请带上要查询的遗物名称，如：核桃 无垢 后纪")
	}
	var relic modelwarframe.Relics
	err := database.DB.Preload("RelicRewards").Where("name = ?", name).First(&relic).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ReplyText(ctx, "未找到该遗物，请确认名称（如：愚龙 后纪）")
		}
		logging.ErrorPack("onebot.command", "query relic %q failed: %v", name, err)
		return ReplyText(ctx, "查询遗物数据失败，请稍后再试")
	}
	return ReplyImage(ctx, draw.DrawRelics([]*draw.Relics{warframe.RelicsToDraw(&relic)}))
}

// wfActiveMission 处理「裂隙/裂缝」：展示当前进行中的普通裂隙图。
func (registry *CommandRegistry) wfActiveMission(ctx *zero.Ctx, _ string) error {
	missions, err := warframe.GetActiveMissions(false)
	if err != nil {
		return ReplyText(ctx, err.Error())
	}
	if len(missions) == 0 {
		return ReplyText(ctx, "暂无进行中的普通裂隙")
	}
	return ReplyImage(ctx, draw.DrawActiveMission(missions))
}

// wfActiveMissionPath 处理「钢铁裂隙/钢铁裂缝」：展示钢铁之路裂隙图。
func (registry *CommandRegistry) wfActiveMissionPath(ctx *zero.Ctx, _ string) error {
	missions, err := warframe.GetActiveMissions(true)
	if err != nil {
		return ReplyText(ctx, err.Error())
	}
	if len(missions) == 0 {
		return ReplyText(ctx, "暂无进行中的钢铁裂隙")
	}
	return ReplyImage(ctx, draw.DrawActiveMission(missions))
}

// wfInvasions 处理「入侵」：展示当前进行中的入侵图。
func (registry *CommandRegistry) wfInvasions(ctx *zero.Ctx, _ string) error {
	invasions, err := warframe.GetInvasions()
	if err != nil {
		return ReplyText(ctx, err.Error())
	}
	if len(invasions) == 0 {
		return ReplyText(ctx, "暂无进行中的入侵")
	}
	return ReplyImage(ctx, draw.DrawInvasion(invasions))
}

// wfVoidTrader 处理「奸商」：展示当前虚空商人及其商品清单图。
func (registry *CommandRegistry) wfVoidTrader(ctx *zero.Ctx, _ string) error {
	traders, err := warframe.GetVoidTraders()
	if err != nil {
		return ReplyText(ctx, err.Error())
	}
	if len(traders) == 0 {
		return ReplyText(ctx, "暂无虚空商人数据")
	}
	return ReplyImage(ctx, draw.DrawVoidTrader(traders))
}

// wfDailyDeals 处理「每日特惠」：展示当前每日特惠图。
func (registry *CommandRegistry) wfDailyDeals(ctx *zero.Ctx, _ string) error {
	deals, err := warframe.GetDailyDeals()
	if err != nil {
		return ReplyText(ctx, err.Error())
	}
	if len(deals) == 0 {
		return ReplyText(ctx, "暂无每日特惠数据")
	}
	return ReplyImage(ctx, draw.DrawDailyDeals(deals[0]))
}

// wfSorties 处理「突击」：展示当前突击任务图。
func (registry *CommandRegistry) wfSorties(ctx *zero.Ctx, _ string) error {
	sorties, err := warframe.GetSorties()
	if err != nil {
		return ReplyText(ctx, err.Error())
	}
	if len(sorties) == 0 {
		return ReplyText(ctx, "暂无突击任务数据")
	}
	return ReplyImage(ctx, draw.DrawSorties(sorties[0]))
}

// wfVoidStorms 处理「九重天」：展示当前九重天虚空风暴图。
func (registry *CommandRegistry) wfVoidStorms(ctx *zero.Ctx, _ string) error {
	missions, err := warframe.GetVoidStorms()
	if err != nil {
		return ReplyText(ctx, err.Error())
	}
	if len(missions) == 0 {
		return ReplyText(ctx, "暂无进行中的九重天风暴")
	}
	return ReplyImage(ctx, draw.DrawActiveMission(missions))
}
