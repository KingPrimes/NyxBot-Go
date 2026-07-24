package config

import (
	"os"

	"gopkg.in/yaml.v3"
	"nyxbot-go/internal/logging"
)

type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Database  DatabaseConfig  `yaml:"database"`
	Log       LogConfig       `yaml:"log"`
	Bot       BotConfig       `yaml:"bot"`
	Auth      AuthConfig      `yaml:"auth"`
}

type ServerConfig struct {
	Host           string   `yaml:"host"`
	Port           string   `yaml:"port"`
	StaticDir      string   `yaml:"static_dir"`
	GinMode        string   `yaml:"gin_mode"`
	RequestLog     bool     `yaml:"request_log"`
	TrustedProxies []string `yaml:"trusted_proxies"`
}

type DatabaseConfig struct {
	Path string `yaml:"path"`
}

type LogConfig struct {
	Startup        bool   `yaml:"startup"`
	Console        bool   `yaml:"console"`
	Dir            string `yaml:"dir"`
	MaxFileSizeMB  int    `yaml:"max_file_size_mb"`
	MaxAgeDays     int    `yaml:"max_age_days"`
	HistorySize    int    `yaml:"history_size"`
}

type BotConfig struct {
	WsURL string `yaml:"ws_url"`
}

type AuthConfig struct {
	JwtSecret string `yaml:"jwt_secret"`
}

func (c Config) Addr() string {
	return c.Server.Host + ":" + c.Server.Port
}

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

func writeDefaultConfig(path string, cfg Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

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
