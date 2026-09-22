// 阶段10 订阅指令（订阅/取消订阅/帮助），对齐 Java WarframeTaskSubscribePlugin。
package onebot

import (
	"strconv"
	"strings"

	zero "github.com/wdvxdr1123/ZeroBot"

	"nyxbot-go/internal/draw"
	"nyxbot-go/internal/warframe"
)

// wfSubscribe 处理「订阅」：仅群聊，解析 [类型]-[任务类型][-遗物等级] 并落库。
func (registry *CommandRegistry) wfSubscribe(ctx *zero.Ctx, parameter string) error {
	if ctx.Event.MessageType != "group" {
		return ReplyText(ctx, "该指令只能在群聊中使用!")
	}
	info := commandInfoFromEvent(ctx)
	raw := normalizeSubscribeInput(parameter)
	if raw == "" {
		sub, mission, reward := warframe.SubscribeHelpEnums()
		if img := draw.DrawWarframeSubscribe(sub, mission, reward); img != nil {
			return ReplyImage(ctx, img)
		}
		return ReplyText(ctx, "订阅指令说明：订阅 <类型编号>[-<子参数>]")
	}
	cmd := &warframe.SubscribeCommand{
		BotUID:    info.BotID,
		GroupID:   info.GroupID,
		GroupName: info.GroupName,
		UserID:    info.UserID,
		UserName:  info.UserName,
	}
	if !warframe.ParseSubscribeCommand(cmd, raw) {
		return ReplyText(ctx, "订阅参数错误，格式为：订阅[类型]-[任务类型][-遗物等级]\n"+
			"示例：订阅9 、 订阅9-2 、 订阅9-2-4")
	}
	return ReplyText(ctx, warframe.Subscribe(cmd))
}

// wfUnsubscribe 处理「取消订阅」：仅限群聊；无参列出当前订阅，带参取消指定规则。
func (registry *CommandRegistry) wfUnsubscribe(ctx *zero.Ctx, parameter string) error {
	if ctx.Event.MessageType != "group" {
		return ReplyText(ctx, "取消订阅指令只能在群聊中使用!")
	}
	info := commandInfoFromEvent(ctx)
	raw := normalizeSubscribeInput(parameter)
	if raw == "" {
		if subs := warframe.UserSubscriptionInfo(info.GroupID, info.UserID); subs != "" {
			return ReplyText(ctx, subs+"\n输入「取消订阅+类型编号」取消指定订阅\n"+
				"例：取消订阅9 、 取消订阅9-2")
		}
		return ReplyText(ctx, "你没有订阅任何内容，输入「订阅」查看可订阅的类型")
	}
	parts := make([]string, 0, 3)
	for _, part := range strings.Split(raw, "-") {
		part = strings.TrimSpace(part)
		if part != "" {
			parts = append(parts, part)
		}
	}
	if len(parts) == 0 {
		return ReplyText(ctx, "订阅参数错误，格式为：取消订阅[类型]-[任务类型][-遗物等级]")
	}
	code, err := strconv.Atoi(parts[0])
	if err != nil {
		return ReplyText(ctx, "订阅参数错误，格式为：取消订阅[类型]-[任务类型][-遗物等级]")
	}
	subType, mission, tier, reward, ok := warframe.ParseUnsubscribeParams(code, parts)
	if !ok {
		return ReplyText(ctx, "订阅参数错误，格式为：取消订阅[类型]-[任务类型][-遗物等级]")
	}
	return ReplyText(ctx, warframe.Unsubscribe(info.GroupID, info.UserID, subType, mission, tier, reward))
}

// commandInfo 事件上下文快照（群/用户/Bot 标识）。
type commandInfo struct {
	GroupID   int64
	GroupName string
	UserID    int64
	UserName  string
	BotID     int64
}

// commandInfoFromEvent 从事件提取群/用户/Bot 标识；群名与成员昵称调用 QQ API，失败时回退 ID。
func commandInfoFromEvent(ctx *zero.Ctx) commandInfo {
	info := commandInfo{
		GroupID:   ctx.Event.GroupID,
		UserID:    ctx.Event.UserID,
		BotID:     ctx.Event.SelfID,
		GroupName: strconv.FormatInt(ctx.Event.GroupID, 10),
		UserName:  strconv.FormatInt(ctx.Event.UserID, 10),
	}
	if group := ctx.GetGroupInfo(ctx.Event.GroupID, false); group.Name != "" {
		info.GroupName = group.Name
	}
	if member := ctx.GetGroupMemberInfo(ctx.Event.GroupID, ctx.Event.UserID, false); member.Get("nickname").String() != "" {
		info.UserName = member.Get("nickname").String()
	}
	return info
}

// normalizeSubscribeInput 去除命令前缀后剩余内容中的空白。
func normalizeSubscribeInput(parameter string) string {
	return strings.ReplaceAll(strings.TrimSpace(parameter), " ", "")
}
