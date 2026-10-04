// Package config 加载和管理应用配置，支持 config.yaml 和环境变量覆盖。
// 启动时自动检测配置文件，缺失则生成默认配置。
package config

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
	"nyxbot-go/internal/logging"
)

// Config 顶层应用配置，包含 Server、Database、Log、Bot、Auth、Warframe 六个子配置项。
// 各字段的 comment 标签是生成 config.yaml 时写入对应条目头注释的文本，
// 新增字段必须同步填写 comment 标签，否则生成的配置文件缺少注释。
type Config struct {
	Server   ServerConfig   `yaml:"server" comment:"HTTP 服务器配置"`
	Database DatabaseConfig `yaml:"database" comment:"SQLite 数据库配置"`
	Log      LogConfig      `yaml:"log" comment:"日志系统配置"`
	Bot      BotConfig      `yaml:"bot" comment:"OneBot 连接配置"`
	Auth     AuthConfig     `yaml:"auth" comment:"认证鉴权配置"`
	Warframe WarframeConfig `yaml:"warframe" comment:"Warframe 数据层配置"`
}

// ServerConfig HTTP 服务器配置。
// 监听主机地址固定为本机地址（见 listenHost）；前端静态文件在打包时内嵌进
// 可执行文件，目录路径同样不开放配置（见 internal/web）。
type ServerConfig struct {
	Port           string   `yaml:"port" comment:"监听端口"`                                // 监听端口
	GinMode        string   `yaml:"gin_mode" comment:"Gin 运行模式：debug / release / test"` // Gin 框架运行模式（debug/release/test）
	RequestLog     bool     `yaml:"request_log" comment:"是否记录 HTTP 请求日志"`               // 是否记录 HTTP 请求日志
	TrustedProxies []string `yaml:"trusted_proxies" comment:"信任的代理 IP 列表"`              // 信任的代理 IP 列表
}

// DatabaseConfig SQLite 数据库配置。
type DatabaseConfig struct {
	Path string `yaml:"path" comment:"数据库文件路径"` // 数据库文件路径
}

// LogConfig 日志系统配置。
type LogConfig struct {
	Level         string `yaml:"level" comment:"写入日志文件的最低等级：TRACE / DEBUG / INFO / WARN / ERROR / PANIC（控制台输出全部等级）"` // 日志文件最低等级，控制台与 SSE 不受影响
	Startup       bool   `yaml:"startup" comment:"启动时是否打印初始化日志"`                                                     // 启动时是否打印初始化日志
	Console       bool   `yaml:"console" comment:"是否输出日志到控制台"`                                                       // 是否输出到控制台
	Dir           string `yaml:"dir" comment:"日志文件目录"`                                                               // 日志文件目录
	MaxFileSizeMB int    `yaml:"max_file_size_mb" comment:"单个日志文件最大体积（MB），超出后轮转"`                                    // 单个日志文件最大体积（MB）
	MaxAgeDays    int    `yaml:"max_age_days" comment:"日志文件保留天数"`                                                    // 日志文件保留天数
	HistorySize   int    `yaml:"history_size" comment:"内存中保留的历史日志条数"`                                                // 内存中保留的历史日志条数
	// SQLLevel GORM SQL 日志级别，取值 silent/error/warn/info（见 comment 标签与 config.yaml 注释）；
	// 空值或非法值由 database.ParseSQLLogLevel 回退 warn。
	SQLLevel string `yaml:"sql_level" comment:"GORM SQL 日志级别：silent/error/warn/info（warn=仅错误与慢查询；info=输出全部 SQL，DEBUG 级）"`
	// SQLSlowMS GORM 慢查询告警阈值（毫秒）：耗时超过它的 SQL 记 WARN（pack=database.sql）。
	// <=0（含老配置文件没有该字段）由 database.ParseSQLSlowThreshold 回退默认 600ms；
	// 想完全不记慢查询请把 sql_level 设为 error，而不是靠这个阈值。
	SQLSlowMS int `yaml:"sql_slow_ms" comment:"GORM 慢查询告警阈值（毫秒），0=默认 600；不想看慢查询告警请把 sql_level 设为 error"`
}

// OneBot 连接模式常量（BotConfig.Mode 的合法取值）。
const (
	BotModeServer = "server" // 反向 WS：本服务作为 WS 服务端等待 Bot 接入
	BotModeClient = "client" // 正向 WS：本服务主动连接 OneBot 实现的 WS 服务端
)

// BotConfig OneBot 连接配置，字段对齐 ZeroBot SDK 的 driver 参数。
// ZeroBot 原生 WSServer 仅取 URL 的 Host 独立监听（丢弃路径），本项目为保持
// 与前端/Java 一致的“主服务端口 + 路径”形态，反向 WS 通过自定义 zero.Driver
// 挂载到 Gin 主服务的 WsServerPath 上。
type BotConfig struct {
	Mode         string `yaml:"mode" comment:"连接模式：server=反向 WS（等待 Bot 接入）/ client=正向 WS（主动连接 OneBot）"` // 连接模式：server=反向 WS / client=正向 WS（driver.NewWebSocketClient）
	WsServerPath string `yaml:"ws_server_path" comment:"反向 WS 挂载路径（mode=server 时生效）"`                   // 反向 WS 挂载路径（对应 ZeroBot WSServer 端点，挂 Gin 主服务）
	WsClientURL  string `yaml:"ws_client_url" comment:"正向 WS 连接地址（mode=client 时生效）"`                    // 正向 WS 地址，对应 driver.NewWebSocketClient 的 url 参数
	AccessToken  string `yaml:"access_token" comment:"OneBot 鉴权令牌"`                                     // OneBot 鉴权令牌，对应 ZeroBot driver 的 accessToken 参数
	WaitN        int    `yaml:"wait_n" comment:"反向 WS 并发等待数"`                                           // 反向 WS 并发等待数，对应 driver.NewWebSocketServer 的 waitn 参数
	PluginPrefix bool   `yaml:"plugin_prefix" comment:"是否使用艾特触发指令"`                                     // 是否使用艾特触发指令（NyxBot 业务语义）
}

// AuthConfig 认证鉴权配置。
type AuthConfig struct {
	JwtSecret string `yaml:"jwt_secret" comment:"JWT 签名密钥（首次启动时随机生成，请勿泄露）"` // JWT 签名密钥，首启随机生成
}

// WarframeConfig Warframe 数据层配置：远程数据源的 HTTP 请求重试参数。
// 覆盖 worldState.php / 仲裁 / warframe.market / 官方导出 / CDN 数据源，
// 网络错误（TLS 握手超时、连接拒绝等）与 HTTP 429/5xx 会按次数自动重试，指数退避。
type WarframeConfig struct {
	// HTTPRetryAttempts 单个请求的最大尝试次数（含首次，1 表示不重试）。
	HTTPRetryAttempts int `yaml:"http_retry_attempts" comment:"远程数据源请求重试次数（含首次，1=不重试）"`
	// HTTPRetryBaseWaitSeconds 首次重试前的等待秒数，之后按 2 的幂指数退避。
	HTTPRetryBaseWaitSeconds int `yaml:"http_retry_base_wait_seconds" comment:"首次重试等待秒数（指数退避基准）"`
	// HTTPRetryTimeoutSeconds 单次请求超时秒数。
	HTTPRetryTimeoutSeconds int `yaml:"http_retry_timeout_seconds" comment:"单次请求超时秒数"`
}

// listenHost 监听主机地址，固定为本机地址（0.0.0.0 表示监听全部网卡），不开放配置项。
const listenHost = "0.0.0.0"

// Addr 返回监听地址字符串（host:port），host 固定为本机地址。
func (c Config) Addr() string {
	return listenHost + ":" + c.Server.Port
}

// Runtime 持有运行期配置，提供并发安全的读取与原子更新（改内存 + 落盘 YAML）。
// 保存接口等运行期修改通过 Update 完成，使新值立即对后续 Config() 调用生效；
// 监听端口等启动期参数仍需重启进程才会应用。
// cfg 是生效配置（含环境变量覆盖），base 是文件派生配置（不含环境变量覆盖）：
// 最小写入与补丁校验都以 base 为基准，环境变量只影响运行期行为，既不写进文件、也不会让保存失败。
type Runtime struct {
	mu   sync.RWMutex
	cfg  Config
	base Config
	path string
}

// NewRuntime 基于初始配置和落盘路径创建运行时配置持有者。
// cfg 为环境变量覆盖后的生效配置；base 从 path 读取（文件不可读或无法解析时退回 cfg）。
func NewRuntime(cfg Config, path string) *Runtime {
	return &Runtime{cfg: cfg, base: fileConfig(path, cfg), path: path}
}

// fileConfig 读取 path 得到「不含环境变量覆盖」的配置；读不到或无法解析时退回 effective。
func fileConfig(path string, effective Config) Config {
	raw, err := os.ReadFile(path)
	if err != nil {
		return effective
	}
	base := defaultConfig()
	if err := yaml.Unmarshal(raw, &base); err != nil {
		return effective
	}
	// 文件里缺 jwt_secret：沿用生效配置里生成的那个，补写时落盘，使重启后 token 不失效
	if base.Auth.JwtSecret == "" {
		base.Auth.JwtSecret = effective.Auth.JwtSecret
	}
	return base
}

// Config 返回当前配置的快照副本。
func (r *Runtime) Config() Config {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.cfg
}

// Path 返回配置落盘的 YAML 文件路径。
func (r *Runtime) Path() string {
	return r.path
}

// Update 在写锁内通过 fn 修改配置，随后把改动最小化写回 YAML 文件：
// 以文件派生配置为基准比对（环境变量覆盖不参与，避免"文件里没有 env 值 → 校验必然失败"），
// 只更新值发生变化的键，并把结构体里有、文件里缺的键补写回去（见 syncConfigFile），
// 写入了哪些键以 INFO 记录（便于排查"改了没生效"）。
// 写文件失败（或补丁校验不通过）时内存中的修改仍然保留，调用方可据此决定是否向客户端报错。
func (r *Runtime) Update(fn func(*Config)) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	before := r.base
	updated := r.base
	// 回调可能原地修改切片元素：先复制，否则 before/updated 共享底层数组会把差异吞掉
	updated.Server.TrustedProxies = slices.Clone(r.base.Server.TrustedProxies)
	fn(&updated)
	result, err := syncConfigFile(r.path, before, updated)
	if err != nil {
		return err
	}
	r.base = updated
	effective := updated
	overrideFromEnv(&effective)
	r.cfg = effective
	if len(result.Changed) > 0 || len(result.Added) > 0 {
		logging.InfoPack("config", "%s saved: changed [%s], missing filled [%s]",
			r.path, strings.Join(result.Changed, ", "), strings.Join(result.Added, ", "))
	}
	return nil
}

// Load 加载默认路径 config.yaml 的配置文件，返回合并了环境变量覆盖的完整配置。
func Load() Config {
	return LoadFrom("config.yaml")
}

// LoadFrom 加载指定路径的配置文件，缺失时生成默认配置（JWT 密钥随机生成并随文件落盘）。
// 文件已存在时：解析后按结构体检测「有键缺失」，并把缺失项按注释补写到文件里（只补缺，不动其它内容），
// 同时记一条 WARN 列出补写的键——避免新增配置项只能靠代码默认值"隐身"。
// 按优先级：默认值 < YAML 文件 < 环境变量。
func LoadFrom(path string) Config {
	cfg := defaultConfig()

	raw, readErr := os.ReadFile(path)
	switch {
	case readErr == nil:
		if err := yaml.Unmarshal(raw, &cfg); err != nil {
			// 内容不可用：不补写、不改写用户文件，后续 jwt 兜底也只驻留内存
			logging.WarnPack("config", "%s parse error, using defaults: %v", path, err)
			break
		}
		if cfg.Auth.JwtSecret == "" {
			cfg.Auth.JwtSecret = generateJwtSecret()
		}
		// 检测并补写缺失配置项（写失败只告警，不影响启动；环境变量尚未覆盖，不会把 env 值写进文件）
		added, err := fillMissingConfigKeys(path, cfg)
		switch {
		case err != nil:
			logging.WarnPack("config", "检测/补写 %s 的缺失配置项失败: %v", path, err)
		case len(added) > 0:
			logging.WarnPack("config", "%s 缺少 %d 个配置项（%s），已按注释补写；请检查这些默认值是否符合预期",
				path, len(added), strings.Join(added, ", "))
		}
	case os.IsNotExist(readErr):
		cfg.Auth.JwtSecret = generateJwtSecret()
		if err := writeConfig(path, cfg); err != nil {
			logging.ErrorPack("config", "%s not found and create default config failed: %v", path, err)
		} else {
			logging.InfoPack("config", "%s not found, default config created", path)
		}
	default:
		logging.ErrorPack("config", "read %s failed: %v", path, readErr)
	}

	if cfg.Auth.JwtSecret == "" {
		// 配置文件存在但内容损坏（未解析成功）：随机生成一个保证鉴权可用，
		// 仅驻留内存不落盘以避免覆盖用户的文件，重启后旧 token 失效。
		cfg.Auth.JwtSecret = generateJwtSecret()
		logging.WarnPack("config", "jwt_secret not set, random secret generated in memory; set jwt_secret in %s to keep tokens valid after restart", path)
	}

	overrideFromEnv(&cfg)

	// 无效的日志等级会静默回退到 INFO，这里显式提示，避免配置写错后无人察觉
	// （检查放在环境变量覆盖之后，以实际生效的值为准）
	if _, ok := logging.ParseLevel(cfg.Log.Level); !ok {
		logging.WarnPack("config", "log.level %q is not a valid level (TRACE/DEBUG/INFO/WARN/ERROR/PANIC), falling back to INFO", cfg.Log.Level)
	}

	return cfg
}

// generateJwtSecret 生成随机 JWT 签名密钥，返回 32 字节随机数的十六进制编码（64 字符）。
// Go 1.24 起 crypto/rand.Read 保证不会失败，故忽略其错误返回值。
func generateJwtSecret() string {
	buf := make([]byte, 32)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}

// writeConfig 将配置以带字段注释的 YAML 格式整份写入指定路径（仅在文件不存在或需要重建时使用）。
func writeConfig(path string, cfg Config) error {
	data, err := marshalWithComments(cfg)
	if err != nil {
		return err
	}

	return writeFileAtomic(path, data)
}

// marshalWithComments 将配置编码为带注释的 YAML 字节流。
// 注释文本取自结构体字段的 comment 标签，作为头注释挂到对应条目上方；
// 顶层分节之间插入空行便于阅读。
func marshalWithComments(cfg Config) ([]byte, error) {
	node, err := configNode(cfg)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(node); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return separateSections(buf.Bytes()), nil
}

// configNode 把配置编码为 YAML 节点树，并按 comment 标签挂上各键的头注释。
// 供整份生成（marshalWithComments）与最小写入（syncConfigFile）共用。
func configNode(cfg Config) (*yaml.Node, error) {
	node := &yaml.Node{}
	if err := node.Encode(cfg); err != nil {
		return nil, err
	}
	applyComments(node, "", fieldComments())
	return node, nil
}

// writeFileAtomic 先写同目录临时文件再改名，避免写入中断留下半截配置。
// 临时文件默认权限为 0600，直接改名会把原文件的 0644 降级，故先按原文件权限位 Chmod
// （文件不存在时用 0644；Windows 无 POSIX 权限位，Chmod 只影响只读属性）。
func writeFileAtomic(path string, data []byte) error {
	mode := os.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}

	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}

// fieldComments 通过反射收集 Config 结构体字段的 comment 标签，
// 返回 YAML 点分路径到注释文本的映射（如 "server.port" -> "监听端口"）。
func fieldComments() map[string]string {
	comments := make(map[string]string)
	collectFieldComments(reflect.TypeOf(Config{}), "", comments)
	return comments
}

// collectFieldComments 递归遍历结构体字段，按 YAML 键名拼出点分路径并记录 comment 标签。
func collectFieldComments(t reflect.Type, prefix string, comments map[string]string) {
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		yamlName, _, _ := strings.Cut(field.Tag.Get("yaml"), ",")
		if yamlName == "" || yamlName == "-" {
			continue
		}
		path := yamlName
		if prefix != "" {
			path = prefix + "." + yamlName
		}
		if comment := field.Tag.Get("comment"); comment != "" {
			comments[path] = comment
		}
		if field.Type.Kind() == reflect.Struct {
			collectFieldComments(field.Type, path, comments)
		}
	}
}

// applyComments 递归遍历 YAML 节点树，为映射键节点挂载注释映射表中的头注释。
// path 为父级 YAML 点分路径，空串表示根。
func applyComments(node *yaml.Node, path string, comments map[string]string) {
	switch node.Kind {
	case yaml.DocumentNode:
		for _, child := range node.Content {
			applyComments(child, path, comments)
		}
	case yaml.MappingNode:
		// MappingNode 的 Content 按 [key, value, key, value ...] 成对排列
		for i := 0; i+1 < len(node.Content); i += 2 {
			key, value := node.Content[i], node.Content[i+1]
			childPath := key.Value
			if path != "" {
				childPath = path + "." + key.Value
			}
			if comment, ok := comments[childPath]; ok {
				key.HeadComment = comment
			}
			applyComments(value, childPath, comments)
		}
	}
}

// separateSections 在顶层分节（含其头注释块）之间插入空行，使生成的配置文件分节清晰。
func separateSections(data []byte) []byte {
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	out := make([]string, 0, len(lines)+4)
	for _, line := range lines {
		if isTopLevelKey(line) {
			// 空行需插到头注释块之前，与上一节隔开
			j := len(out)
			for j > 0 && strings.HasPrefix(out[j-1], "#") {
				j--
			}
			if j > 0 && out[j-1] != "" {
				tail := append([]string{""}, out[j:]...)
				out = append(out[:j], tail...)
			}
		}
		out = append(out, line)
	}
	return []byte(strings.Join(out, "\n") + "\n")
}

// isTopLevelKey 判断是否为顶层分节键行（无缩进、非注释、非空行）。
func isTopLevelKey(line string) bool {
	return line != "" && line[0] != ' ' && line[0] != '#'
}

// defaultConfig 返回出厂默认配置。
// JwtSecret 留空：首启生成配置文件时由 LoadFrom 随机生成并落盘。
func defaultConfig() Config {
	return Config{
		Server: ServerConfig{
			Port:           "8080",
			GinMode:        "release",
			RequestLog:     false,
			TrustedProxies: nil,
		},
		Database: DatabaseConfig{
			Path: "data/nyxbot.db",
		},
		Log: LogConfig{
			Level:         "INFO",
			Startup:       true,
			Console:       true,
			Dir:           "data/logs",
			MaxFileSizeMB: 5,
			MaxAgeDays:    7,
			HistorySize:   50,
			SQLLevel:      "warn",
			// 600ms：遗物导入的子表批量 INSERT（3002 行 / 18012 个绑定参数）实测约 200ms，
			// 若沿用 GORM 默认 200ms 会在每次数据更新时刷出无意义的慢查询告警。
			SQLSlowMS: 600,
		},
		Bot: BotConfig{
			Mode:         "server",
			WsServerPath: "/ws/shiro",
			WsClientURL:  "ws://localhost:3001",
			AccessToken:  "",
			WaitN:        16,
			PluginPrefix: false,
		},
		Auth: AuthConfig{
			JwtSecret: "",
		},
		Warframe: WarframeConfig{
			HTTPRetryAttempts:        3,  // 3 次尝试（含首次），网络抖动自动重试
			HTTPRetryBaseWaitSeconds: 1,  // 首次重试等待 1 秒，之后 2s → 4s 指数退避
			HTTPRetryTimeoutSeconds:  15, // 单次请求 15 秒超时
		},
	}
}

// overrideFromEnv 用环境变量覆盖配置字段（仅顶层字段）。
func overrideFromEnv(cfg *Config) {
	if v := os.Getenv("APP_PORT"); v != "" {
		cfg.Server.Port = v
	}
	if v := os.Getenv("GIN_MODE"); v != "" {
		cfg.Server.GinMode = v
	}
	if v := os.Getenv("APP_REQUEST_LOG"); v == "true" || v == "1" {
		cfg.Server.RequestLog = true
	}
	if v := os.Getenv("DB_PATH"); v != "" {
		cfg.Database.Path = v
	}
	if v := os.Getenv("APP_STARTUP_LOG"); v == "false" || v == "0" {
		cfg.Log.Startup = false
	}
	if v := os.Getenv("APP_LOG_LEVEL"); v != "" {
		cfg.Log.Level = v
	}
	if v := os.Getenv("APP_LOG_CONSOLE"); v == "false" || v == "0" {
		cfg.Log.Console = false
	}
	if v := os.Getenv("JWT_SECRET"); v != "" {
		cfg.Auth.JwtSecret = v
	}
}
