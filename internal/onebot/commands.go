package onebot

import (
	"errors"
	"fmt"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	zero "github.com/wdvxdr1123/ZeroBot"

	"nyxbot-go/internal/database"
	"nyxbot-go/internal/enum/nyxbot"
	"nyxbot-go/internal/logging"
	modelsystem "nyxbot-go/internal/model/system"
	"nyxbot-go/internal/version"
)

const (
	helpDocumentURL     = "https://kingprimes.top/posts/1bb16eb"
	commandStatusOK     = 0
	commandStatusFailed = 1
)

// CommandHandler 处理已通过正则和权限检查的指令，parameter 为移除指令前缀后的参数。
type CommandHandler func(ctx *zero.Ctx, parameter string) error

// CommandRegistry 按 Java Codes 枚举顺序注册 ZeroBot 指令与统一执行日志。
type CommandRegistry struct {
	engine  *zero.Engine
	access  *AccessChecker
	started time.Time

	mu         sync.RWMutex
	handlers   map[nyxbot.Codes]CommandHandler
	registered bool
}

// NewCommandRegistry 创建基础指令注册器。
func NewCommandRegistry(pluginPrefix bool) *CommandRegistry {
	return newCommandRegistry(func() bool { return pluginPrefix })
}

func newCommandRegistry(pluginPrefixProvider func() bool) *CommandRegistry {
	registry := &CommandRegistry{
		engine:   zero.New().SetBlock(true),
		access:   NewAccessCheckerWithProvider(pluginPrefixProvider),
		started:  time.Now(),
		handlers: make(map[nyxbot.Codes]CommandHandler, len(nyxbot.CodesOrder)),
	}
	for _, code := range nyxbot.CodesOrder {
		registry.handlers[code] = registry.notImplemented
	}
	registry.handlers[nyxbot.CmdHelp] = registry.help
	registry.handlers[nyxbot.CmdCheckVersion] = registry.systemInfo
	return registry
}

// SetHandler 在 Register 前替换指定指令处理器，供后续阶段扩展业务命令。
func (registry *CommandRegistry) SetHandler(code nyxbot.Codes, handler CommandHandler) error {
	if handler == nil {
		return errors.New("command handler is nil")
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if registry.registered {
		return errors.New("commands are already registered")
	}
	if _, ok := nyxbot.CodesInfo[code]; !ok {
		return fmt.Errorf("unknown command code %s", code)
	}
	registry.handlers[code] = handler
	return nil
}

// Register 将全部指令注册到 ZeroBot 全局匹配器列表。
func (registry *CommandRegistry) Register() error {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if registry.registered {
		return nil
	}
	for index, code := range nyxbot.CodesOrder {
		info, ok := nyxbot.CodesInfo[code]
		if !ok {
			return fmt.Errorf("missing command metadata for %s", code)
		}
		pattern, err := regexp.Compile(info.Comm)
		if err != nil {
			return fmt.Errorf("compile command %s: %w", code, err)
		}
		handler := registry.handlers[code]
		if handler == nil {
			handler = registry.notImplemented
		}
		registry.engine.
			OnRegex(info.Comm, registry.access.rule(info.Permissions)).
			SetPriority(index + 10).
			Handle(registry.wrap(code, info, pattern, handler))
	}
	registry.registered = true
	return nil
}

// Close 从 ZeroBot 移除本注册器创建的全部 matcher。
func (registry *CommandRegistry) Close() {
	registry.engine.Delete()
}

func (registry *CommandRegistry) wrap(code nyxbot.Codes, info nyxbot.CodeInfo, pattern *regexp.Regexp, handler CommandHandler) zero.Handler {
	displayCommand := commandAlias(info.Comm)
	return func(ctx *zero.Ctx) {
		started := time.Now()
		rawMessage := ""
		if ctx != nil {
			rawMessage = strings.TrimSpace(ctx.MessageString())
		}
		parameter := strings.TrimSpace(pattern.ReplaceAllString(rawMessage, ""))
		var executionErr error

		defer func() {
			if recovered := recover(); recovered != nil {
				executionErr = fmt.Errorf("%v", recovered)
			}
			if executionErr != nil {
				logging.ErrorPack("onebot.command", "command %s failed: %v", code, executionErr)
				if ctx != nil && ctx.Event != nil {
					_ = ReplyText(ctx, fmt.Sprintf("插件%s执行异常\n异常信息：%s", displayCommand, executionErr.Error()))
				}
			}
			registry.recordLog(ctx, displayCommand, rawMessage, info.Permissions, started, executionErr)
		}()

		if ctx == nil || ctx.Event == nil {
			executionErr = errors.New("message event is unavailable")
			return
		}
		logging.InfoPack("onebot.command", "group=%d user=%d command=%s raw=%q", ctx.Event.GroupID, ctx.Event.UserID, displayCommand, rawMessage)
		executionErr = handler(ctx, parameter)
	}
}

func (registry *CommandRegistry) help(ctx *zero.Ctx, _ string) error {
	commands := make([]string, 0, len(nyxbot.CodesOrder))
	for _, code := range nyxbot.CodesOrder {
		alias := commandAlias(nyxbot.CodesInfo[code].Comm)
		switch code {
		case nyxbot.CmdWfSubscribe:
			alias = "订阅 [0-9] -[0-9]"
		case nyxbot.CmdWfUnsubscribe:
			alias = "取消订阅 [0-9] -[0-9]"
		}
		commands = append(commands, alias)
	}
	text := "NyxBot 指令\n" + strings.Join(commands, "、") + "\n\n指令文档：" + helpDocumentURL
	return ReplyText(ctx, text)
}

func (registry *CommandRegistry) systemInfo(ctx *zero.Ctx, _ string) error {
	uptime := time.Since(registry.started).Round(time.Second)
	text := fmt.Sprintf(
		"%s %s (%s)\nGo %s\n%s/%s\n运行时间：%s",
		version.ProductName,
		version.Version,
		version.Commit,
		runtime.Version(),
		runtime.GOOS,
		runtime.GOARCH,
		uptime,
	)
	return ReplyText(ctx, text)
}

func (registry *CommandRegistry) notImplemented(ctx *zero.Ctx, _ string) error {
	return ReplyText(ctx, "该功能暂未实现！")
}

func (registry *CommandRegistry) recordLog(
	ctx *zero.Ctx,
	command string,
	rawMessage string,
	permission nyxbot.PermissionsEnums,
	started time.Time,
	executionErr error,
) {
	if database.DB == nil || ctx == nil || ctx.Event == nil {
		return
	}
	status := commandStatusOK
	errorMessage := ""
	if executionErr != nil {
		status = commandStatusFailed
		errorMessage = executionErr.Error()
	}
	entry := modelsystem.LogInfo{
		Title:         string(nyxbot.LogTitlePlugin),
		Code:          command,
		Permissions:   string(permission),
		BusinessType:  nyxbot.BizPlugin.Type(),
		BotUID:        ctx.Event.SelfID,
		GroupUID:      ctx.Event.GroupID,
		UserUID:       ctx.Event.UserID,
		RawMsg:        rawMessage,
		Method:        "Plugin",
		RequestMethod: "internal/onebot.CommandRegistry.handle()",
		RunTime:       time.Since(started).Milliseconds(),
		Status:        status,
		ErrorMsg:      errorMessage,
		LogTime:       time.Now(),
	}
	if err := database.DB.Create(&entry).Error; err != nil {
		logging.ErrorPack("onebot.command", "save command log failed: %v", err)
	}
}

func commandAlias(pattern string) string {
	first, _, _ := strings.Cut(pattern, "|")
	first = strings.ReplaceAll(first, ".*?", "")
	first = strings.NewReplacer("^", "", "$", "", "(", "", ")", "").Replace(first)
	return strings.TrimSpace(first)
}
