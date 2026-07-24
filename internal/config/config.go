// Package config 加载和管理应用配置，支持 config.yaml 和环境变量覆盖。
// 启动时自动检测配置文件，缺失则生成默认配置。
package config

import (
	"os"

	"gopkg.in/yaml.v3"
	"nyxbot-go/internal/logging"
)

// Config 顶层应用配置，包含 Server、Database、Log、Bot、Auth 五个子配置项。
type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Database  DatabaseConfig  `yaml:"database"`
	Log       LogConfig       `yaml:"log"`
	Bot       BotConfig       `yaml:"bot"`
	Auth      AuthConfig      `yaml:"auth"`
}

// ServerConfig HTTP 服务器配置。
type ServerConfig struct {
	Host           string   `yaml:"host"`            // 监听主机地址
	Port           string   `yaml:"port"`            // 监听端口
	StaticDir      string   `yaml:"static_dir"`      // 前端静态文件目录
	GinMode        string   `yaml:"gin_mode"`        // Gin 框架运行模式（debug/release/test）
	RequestLog     bool     `yaml:"request_log"`     // 是否记录 HTTP 请求日志
	TrustedProxies []string `yaml:"trusted_proxies"` // 信任的代理 IP 列表
}

// DatabaseConfig SQLite 数据库配置。
type DatabaseConfig struct {
	Path string `yaml:"path"` // 数据库文件路径
}

// LogConfig 日志系统配置。
type LogConfig struct {
	Startup        bool   `yaml:"startup"`          // 启动时是否打印初始化日志
	Console        bool   `yaml:"console"`          // 是否输出到控制台
	Dir            string `yaml:"dir"`              // 日志文件目录
	MaxFileSizeMB  int    `yaml:"max_file_size_mb"` // 单个日志文件最大体积（MB）
	MaxAgeDays     int    `yaml:"max_age_days"`     // 日志文件保留天数
	HistorySize    int    `yaml:"history_size"`     // 内存中保留的历史日志条数
}

// BotConfig OneBot 机器人连接配置。
type BotConfig struct {
	WsURL string `yaml:"ws_url"` // WebSocket 路径
}

// AuthConfig 认证鉴权配置。
type AuthConfig struct {
	JwtSecret string `yaml:"jwt_secret"` // JWT 签名密钥
}

// Addr 返回监听地址字符串（host:port）。
func (c Config) Addr() string {
	return c.Server.Host + ":" + c.Server.Port
}

// Load 加载配置文件，返回合并了环境变量覆盖的完整配置。
// 按优先级：默认值 < config.yaml < 环境变量。
func Load() Config {
	cfg := defaultConfig()

	if data, err := os.ReadFile("config.yaml"); err == nil {
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			logging.WarnPack("config", "config.yaml parse error, using defaults: %v", err)
		}
	} else {
		if err := writeDefaultConfig("config.yaml", cfg); err != nil {
			logging.ErrorPack("config", "config.yaml not found and create default config failed: %v", err)
		} else {
			logging.InfoPack("config", "config.yaml not found, default config created")
		}
	}

	overrideFromEnv(&cfg)

	return cfg
}

// writeDefaultConfig 将默认配置写入指定路径。
func writeDefaultConfig(path string, cfg Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// defaultConfig 返回出厂默认配置。
func defaultConfig() Config {
	return Config{
		Server: ServerConfig{
			Host:           "0.0.0.0",
			Port:           "8080",
			StaticDir:      "./resources/static",
			GinMode:        "release",
			RequestLog:     false,
			TrustedProxies: nil,
		},
		Database: DatabaseConfig{
			Path: "data/nyxbot.db",
		},
		Log: LogConfig{
			Startup:       true,
			Console:       true,
			Dir:           "data/logs",
			MaxFileSizeMB: 5,
			MaxAgeDays:    7,
			HistorySize:   50,
		},
		Bot: BotConfig{
			WsURL: "/ws/shiro",
		},
		Auth: AuthConfig{
			JwtSecret: "nyxbot-secret-key",
		},
	}
}

// overrideFromEnv 用环境变量覆盖配置字段（仅顶层字段）。
func overrideFromEnv(cfg *Config) {
	if v := os.Getenv("APP_HOST"); v != "" {
		cfg.Server.Host = v
	}
	if v := os.Getenv("APP_PORT"); v != "" {
		cfg.Server.Port = v
	}
	if v := os.Getenv("APP_STATIC_DIR"); v != "" {
		cfg.Server.StaticDir = v
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
	if v := os.Getenv("APP_LOG_CONSOLE"); v == "false" || v == "0" {
		cfg.Log.Console = false
	}
	if v := os.Getenv("JWT_SECRET"); v != "" {
		cfg.Auth.JwtSecret = v
	}
}
