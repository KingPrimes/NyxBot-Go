package onebot

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/driver"

	botdirectory "nyxbot-go/internal/bot"
	"nyxbot-go/internal/config"
	"nyxbot-go/internal/logging"
	"nyxbot-go/internal/version"
)

// Runtime 管理 OneBot Driver、ZeroBot 指令和在线目录同步的生命周期。
type Runtime struct {
	config     config.BotConfig
	reverse    *ReverseDriver
	syncer     *DirectorySyncer
	commands   *CommandRegistry
	startOnce  sync.Once
	closeOnce  sync.Once
	startError error
	cancel     context.CancelFunc
	mu         sync.Mutex
}

// NewRuntime 根据 Bot 配置创建 OneBot 运行时，但不会立即连接或启动 goroutine。
func NewRuntime(botConfig config.BotConfig, directory *botdirectory.Directory) (*Runtime, error) {
	return newRuntime(botConfig, directory, func() bool { return botConfig.PluginPrefix })
}

// NewRuntimeFromConfig 基于并发安全的配置持有者创建 OneBot 运行时。
// 连接参数使用启动快照，plugin_prefix 在每次消息检查时读取最新值。
func NewRuntimeFromConfig(configRuntime *config.Runtime, directory *botdirectory.Directory) (*Runtime, error) {
	if configRuntime == nil {
		return nil, fmt.Errorf("config runtime is nil")
	}
	botConfig := configRuntime.Config().Bot
	return newRuntime(botConfig, directory, func() bool {
		return configRuntime.Config().Bot.PluginPrefix
	})
}

func newRuntime(botConfig config.BotConfig, directory *botdirectory.Directory, pluginPrefixProvider func() bool) (*Runtime, error) {
	mode := strings.ToLower(strings.TrimSpace(botConfig.Mode))
	if mode == "" {
		mode = config.BotModeServer
	}
	if mode != config.BotModeServer && mode != config.BotModeClient {
		return nil, fmt.Errorf("unsupported OneBot mode %q", botConfig.Mode)
	}
	botConfig.Mode = mode
	botConfig.WsServerPath = normalizeWebSocketPath(botConfig.WsServerPath)
	if mode == config.BotModeClient && strings.TrimSpace(botConfig.WsClientURL) == "" {
		return nil, fmt.Errorf("ws_client_url is required in client mode")
	}
	if directory == nil {
		directory = botdirectory.DefaultDirectory
	}

	runtime := &Runtime{
		config:   botConfig,
		syncer:   NewDirectorySyncer(directory),
		commands: newCommandRegistry(pluginPrefixProvider),
	}
	if mode == config.BotModeServer {
		runtime.reverse = NewReverseDriver(botConfig.WaitN, botConfig.AccessToken, directory)
		runtime.reverse.SetConnectHook(runtime.syncer.Trigger)
	}
	return runtime, nil
}

// RegisterRoutes 把 server 模式的反向 WebSocket 端点挂到 Gin 主服务。
// 路径与已有路由冲突时返回错误，避免 Gin 的注册 panic 终止进程。
func (runtime *Runtime) RegisterRoutes(routes gin.IRoutes) (err error) {
	if runtime != nil && runtime.reverse != nil {
		defer func() {
			if recovered := recover(); recovered != nil {
				err = fmt.Errorf("register OneBot route %s: %v", runtime.config.WsServerPath, recovered)
			}
		}()
		runtime.reverse.Register(routes, runtime.config.WsServerPath)
	}
	return nil
}

// SetDataExecutor 注入管理类数据更新执行器到指令注册器（Start 前调用）。
func (runtime *Runtime) SetDataExecutor(executor DataUpdateExecutor) {
	if runtime != nil && runtime.commands != nil {
		runtime.commands.SetDataExecutor(executor)
	}
}

// Start 注册指令并异步启动 ZeroBot 与在线目录同步。重复调用不会重复启动。
func (runtime *Runtime) Start(parent context.Context) error {
	if runtime == nil {
		return fmt.Errorf("OneBot runtime is nil")
	}
	runtime.startOnce.Do(func() {
		if err := runtime.commands.Register(); err != nil {
			runtime.startError = err
			return
		}
		if parent == nil {
			parent = context.Background()
		}
		ctx, cancel := context.WithCancel(parent)
		runtime.mu.Lock()
		runtime.cancel = cancel
		runtime.mu.Unlock()
		go runtime.syncer.Run(ctx)

		var communicationDriver zero.Driver
		if runtime.config.Mode == config.BotModeClient {
			communicationDriver = driver.NewWebSocketClient(runtime.config.WsClientURL, runtime.config.AccessToken)
			logging.InfoPack("onebot.runtime", "starting OneBot client: %s", runtime.config.WsClientURL)
		} else {
			communicationDriver = runtime.reverse
			logging.InfoPack("onebot.runtime", "starting OneBot reverse endpoint: %s", runtime.config.WsServerPath)
		}
		go zero.Run(&zero.Config{
			NickName:        []string{version.ProductName},
			KeepAtMeMessage: false,
			Driver:          []zero.Driver{communicationDriver},
		})
	})
	return runtime.startError
}

// Close 停止目录同步、移除指令 matcher，并关闭反向 WebSocket 连接。
// ZeroBot 内置正向 WS 客户端没有关闭接口，client 模式采用进程级生命周期，
// 不支持在同一进程内停止后重启；主程序仅在进程退出流程中调用 Close。
func (runtime *Runtime) Close() error {
	if runtime == nil {
		return nil
	}
	var closeError error
	runtime.closeOnce.Do(func() {
		runtime.mu.Lock()
		cancel := runtime.cancel
		runtime.mu.Unlock()
		if cancel != nil {
			cancel()
		}
		runtime.commands.Close()
		if runtime.reverse != nil {
			closeError = runtime.reverse.Close()
		}
	})
	return closeError
}
