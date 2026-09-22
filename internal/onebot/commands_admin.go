// 阶段10 管理类指令（更新WM物品/紫卡/姐妹/翻译），对齐 Java UpdateAllPlugin + UpdateUtils。
// 执行组件由 main 注入（实现 DataUpdateExecutor），异步执行并回发完成消息。
package onebot

import (
	"context"

	zero "github.com/wdvxdr1123/ZeroBot"

	"nyxbot-go/internal/logging"
)

// DataUpdateExecutor 执行管理类数据更新任务的能力（由外部注入，避免 onebot 直接依赖 warframe 数据层）。
type DataUpdateExecutor interface {
	UpdateOrdersItems(ctx context.Context) error
	UpdateRivenItems(ctx context.Context) error
	UpdateLichSister(ctx context.Context) error
	UpdateTranslation(ctx context.Context) error
}

// SetDataExecutor 注入管理类数据更新执行器（Register 前调用）。
func (registry *CommandRegistry) SetDataExecutor(executor DataUpdateExecutor) {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	registry.dataExecutor = executor
}

// wfUpdateMarketItems 处理「更新WM 物品」：管理员异步刷新市场物品数据。
func (registry *CommandRegistry) wfUpdateMarketItems(ctx *zero.Ctx, _ string) error {
	return registry.dataUpdateRun(ctx, "Market", func(e DataUpdateExecutor) func(context.Context) error {
		return e.UpdateOrdersItems
	})
}

// wfUpdateMarketRiven 处理「更新WM紫卡」：管理员异步刷新紫卡武器数据。
func (registry *CommandRegistry) wfUpdateMarketRiven(ctx *zero.Ctx, _ string) error {
	return registry.dataUpdateRun(ctx, "WM紫卡", func(e DataUpdateExecutor) func(context.Context) error {
		return e.UpdateRivenItems
	})
}

// wfUpdateLichSister 处理「更新信条」：管理员异步刷新信条/赤毒武器数据。
func (registry *CommandRegistry) wfUpdateLichSister(ctx *zero.Ctx, _ string) error {
	return registry.dataUpdateRun(ctx, "信条/赤毒武器", func(e DataUpdateExecutor) func(context.Context) error {
		return e.UpdateLichSister
	})
}

// wfUpdateTranslation 处理「更新翻译」：管理员异步刷新全部翻译数据。
func (registry *CommandRegistry) wfUpdateTranslation(ctx *zero.Ctx, _ string) error {
	return registry.dataUpdateRun(ctx, "翻译数据", func(e DataUpdateExecutor) func(context.Context) error {
		return e.UpdateTranslation
	})
}

// dataUpdateRun 校验执行器已注入后执行通用更新流程。
func (registry *CommandRegistry) dataUpdateRun(ctx *zero.Ctx, name string, pick func(DataUpdateExecutor) func(context.Context) error) error {
	if registry.dataExecutor == nil {
		return ReplyText(ctx, "数据更新功能未启用")
	}
	return registry.runDataUpdate(ctx, pick(registry.dataExecutor), name)
}

// runDataUpdate 通用管理更新流程（对齐 Java updateXxx）：立即回"已发布任务"，异步执行并回发完成/失败。
func (registry *CommandRegistry) runDataUpdate(ctx *zero.Ctx, exec func(context.Context) error, name string) error {
	if exec == nil {
		return ReplyText(ctx, "数据更新功能未启用")
	}
	if err := ReplyText(ctx, "已发布任务，正在更新！"); err != nil {
		return err
	}
	go func() {
		if err := exec(context.Background()); err != nil {
			logging.ErrorPack("onebot.command", "data update %s failed: %v", name, err)
			_ = SendGroupText(ctx.Event.SelfID, ctx.Event.GroupID, name+"更新失败！")
			return
		}
		_ = SendGroupText(ctx.Event.SelfID, ctx.Event.GroupID, name+"更新完成！")
	}()
	return nil
}
