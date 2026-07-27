// Package system 系统管理相关 HTTP 接口（系统配置、日志查询等）。
package system

import (
	"net/http"
	"regexp"
	"strconv"

	"github.com/gin-gonic/gin"
	"nyxbot-go/internal/config"
	"nyxbot-go/internal/response"
)

// LoadingConfig 系统配置视图，JSON 字段与 WebUI 的 Api.SystemConfig.LoadingConfig 对齐。
// 内部存储按 ZeroBot SDK 参数组织（见 config.BotConfig），本结构负责双向映射。
type LoadingConfig struct {
	ServerPort       int    `json:"serverPort"`       // 服务端口（1-65535）
	IsServerOrClient bool   `json:"isServerOrClient"` // true=反向 WS 服务端模式，false=正向 WS 客户端模式
	WsClientURL      string `json:"wsClientUrl"`      // 正向 WebSocket 地址
	WsServerURL      string `json:"wsServerUrl"`      // 反向 WebSocket 路径
	Token            string `json:"token"`            // OneBot 连接令牌（accessToken）
	PluginPrefix     bool   `json:"pluginPrefix"`     // 是否使用艾特触发指令
	PluginName       string `json:"pluginName"`       // 当前选中的绘图插件名称
}

// loadingConfigUpdate 保存系统配置的请求体。
// 全部字段为指针类型，用于区分“未提交”与“零值”，仅覆盖非 nil 字段，
// 对齐 Java NyxConfig.mergeInto 的部分更新语义；未声明的字段（如旧版代理配置）静默忽略。
type loadingConfigUpdate struct {
	ServerPort       *int    `json:"serverPort"`
	IsServerOrClient *bool   `json:"isServerOrClient"`
	WsClientURL      *string `json:"wsClientUrl"`
	WsServerURL      *string `json:"wsServerUrl"`
	Token            *string `json:"token"`
	PluginPrefix     *bool   `json:"pluginPrefix"`
	PluginName       *string `json:"pluginName"`
}

var (
	// wsServerURLPattern 对齐 Java 的一至两段字母路径，并拒绝尾随非法内容。
	wsServerURLPattern = regexp.MustCompile(`^/[A-Za-z]+(?:/[A-Za-z]+)?$`)
	// wsClientURLPattern 对齐 Java isValidateClientUrl。
	wsClientURLPattern = regexp.MustCompile(`^(ws|wss)://[\w.-]+(:\d+)?(/([\w/_.-]*(\?\S+)?)?)?$`)
)

// ConfigHandler 系统配置接口处理器，依赖运行时配置实现读取与热更新。
type ConfigHandler struct {
	rt *config.Runtime
}

// NewConfigHandler 创建系统配置处理器。
func NewConfigHandler(rt *config.Runtime) *ConfigHandler {
	return &ConfigHandler{rt: rt}
}

// GetLoading 处理 GET /config/loading，返回当前系统配置。
func (h *ConfigHandler) GetLoading(c *gin.Context) {
	response.Success(c, loadingConfigFromConfig(h.rt.Config()))
}

// SaveLoading 处理 POST /config/loading，校验后按非 nil 字段合并配置并写回 YAML。
// 校验失败返回 500（对齐 Java BaseController.error），成功返回 { code: 200 }。
// 端口、连接模式、WS 地址和令牌修改后需重启进程生效；pluginPrefix 即时生效。
func (h *ConfigHandler) SaveLoading(c *gin.Context) {
	var req loadingConfigUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数格式错误")
		return
	}

	if req.ServerPort != nil && (*req.ServerPort < 1 || *req.ServerPort > 65535) {
		response.Error(c, "端口号必须在1到65535之间")
		return
	}
	if req.WsServerURL != nil && !wsServerURLPattern.MatchString(*req.WsServerURL) {
		response.Error(c, "服务端URL格式不正确")
		return
	}
	if req.WsClientURL != nil && !wsClientURLPattern.MatchString(*req.WsClientURL) {
		response.Error(c, "客户端URL格式不正确")
		return
	}

	if err := h.rt.Update(req.applyTo); err != nil {
		response.Error(c, "配置保存失败: "+err.Error())
		return
	}

	response.Success(c, nil)
}

// loadingConfigFromConfig 将内部配置转换为前端视图结构。
// 存储侧字段对应 ZeroBot driver 参数，此处映射回前端契约字段。
func loadingConfigFromConfig(cfg config.Config) LoadingConfig {
	port, err := strconv.Atoi(cfg.Server.Port)
	if err != nil {
		port = 8080
	}

	return LoadingConfig{
		ServerPort:       port,
		IsServerOrClient: cfg.Bot.Mode != config.BotModeClient,
		WsClientURL:      cfg.Bot.WsClientURL,
		WsServerURL:      cfg.Bot.WsServerPath,
		Token:            cfg.Bot.AccessToken,
		PluginPrefix:     cfg.Bot.PluginPrefix,
		PluginName:       cfg.Bot.PluginName,
	}
}

// applyTo 将请求中非 nil 的字段合并到目标配置（存储侧为 ZeroBot 参数语义）。
func (u *loadingConfigUpdate) applyTo(cfg *config.Config) {
	if u.ServerPort != nil {
		cfg.Server.Port = strconv.Itoa(*u.ServerPort)
	}
	if u.IsServerOrClient != nil {
		if *u.IsServerOrClient {
			cfg.Bot.Mode = config.BotModeServer
		} else {
			cfg.Bot.Mode = config.BotModeClient
		}
	}
	if u.WsClientURL != nil {
		cfg.Bot.WsClientURL = *u.WsClientURL
	}
	if u.WsServerURL != nil {
		cfg.Bot.WsServerPath = *u.WsServerURL
	}
	if u.Token != nil {
		cfg.Bot.AccessToken = *u.Token
	}
	if u.PluginPrefix != nil {
		cfg.Bot.PluginPrefix = *u.PluginPrefix
	}
	if u.PluginName != nil {
		cfg.Bot.PluginName = *u.PluginName
	}
}
