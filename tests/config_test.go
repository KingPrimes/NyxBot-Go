package tests

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
	"nyxbot-go/internal/config"
	"nyxbot-go/internal/logging"
)

// neutralizeConfigEnv 清空会影响配置加载的环境变量，保证测试结果与运行环境无关。
func neutralizeConfigEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{"APP_PORT", "GIN_MODE", "APP_REQUEST_LOG", "DB_PATH", "APP_STARTUP_LOG", "APP_LOG_CONSOLE", "APP_LOG_LEVEL", "JWT_SECRET"} {
		t.Setenv(key, "")
	}
}

// TestLoadFromGeneratesCommentedDefault 验证配置文件缺失时自动生成的默认配置：
// 每个条目携带 comment 标签定义的头注释，且带注释的内容可无损解析回已加载的配置。
func TestLoadFromGeneratesCommentedDefault(t *testing.T) {
	neutralizeConfigEnv(t)
	path := filepath.Join(t.TempDir(), "config.yaml")
	loaded := config.LoadFrom(path)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("默认配置未生成: %v", err)
	}
	t.Logf("生成的配置内容：\n%s", data)

	text := string(data)
	// 抽查顶层分节注释与关键字段注释
	for _, want := range []string{
		"# HTTP 服务器配置",
		"# 监听端口",
		"# SQLite 数据库配置",
		"# OneBot 连接配置",
		"# 写入日志文件的最低等级：TRACE / DEBUG / INFO / WARN / ERROR / PANIC（控制台输出全部等级）",
		"# JWT 签名密钥（首次启动时随机生成，请勿泄露）",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("生成配置缺少注释 %q", want)
		}
	}
	if loaded.Server.Port != "8080" {
		t.Errorf("端口默认值异常: %q", loaded.Server.Port)
	}
	if loaded.Log.Level != "INFO" {
		t.Errorf("日志文件等级默认值应为 INFO，实际: %q", loaded.Log.Level)
	}

	// 注释不影响解析：写出的内容应能无损读回加载结果
	var roundTrip config.Config
	if err := yaml.Unmarshal(data, &roundTrip); err != nil {
		t.Fatalf("带注释配置解析失败: %v", err)
	}
	// YAML 中的 [] 反序列化为空切片而非 nil，比较前需归一化
	loaded.Server.TrustedProxies = []string{}
	if !reflect.DeepEqual(roundTrip, loaded) {
		t.Fatalf("配置往返不一致:\n%+v\n期望:\n%+v", roundTrip, loaded)
	}
}

// TestLoadFromGeneratesRandomJwtSecret 验证首启生成配置时 JWT 密钥为随机值并已落盘：
// 不同实例生成的密钥互不相同，重新加载同一文件时密钥保持不变。
func TestLoadFromGeneratesRandomJwtSecret(t *testing.T) {
	neutralizeConfigEnv(t)
	path1 := filepath.Join(t.TempDir(), "config.yaml")
	path2 := filepath.Join(t.TempDir(), "config.yaml")

	cfg1 := config.LoadFrom(path1)
	cfg2 := config.LoadFrom(path2)

	if len(cfg1.Auth.JwtSecret) != 64 {
		t.Errorf("jwt secret 长度异常（期望 64 位十六进制字符）: %q", cfg1.Auth.JwtSecret)
	}
	if cfg1.Auth.JwtSecret == cfg2.Auth.JwtSecret {
		t.Error("两次首启生成的 jwt secret 相同")
	}
	if again := config.LoadFrom(path1); again.Auth.JwtSecret != cfg1.Auth.JwtSecret {
		t.Error("重新加载已有配置后 jwt secret 发生变化")
	}
}

// TestLoadFromEmptyJwtSecretFallback 验证已有配置缺失 jwt_secret 时，
// 在内存中生成随机密钥兜底且不覆盖用户文件。
func TestLoadFromEmptyJwtSecretFallback(t *testing.T) {
	neutralizeConfigEnv(t)
	path := filepath.Join(t.TempDir(), "config.yaml")
	original := []byte("server:\n  port: \"8080\"\n")
	if err := os.WriteFile(path, original, 0644); err != nil {
		t.Fatal(err)
	}

	cfg := config.LoadFrom(path)
	if cfg.Auth.JwtSecret == "" {
		t.Error("缺失 jwt_secret 时未生成兜底密钥")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(original) {
		t.Error("兜底逻辑不应改写用户已有的配置文件")
	}
}

// TestLoadFromEnvLogLevelOverride 验证环境变量 APP_LOG_LEVEL 覆盖日志文件等级，
// 且配置里的等级可被日志系统解析（大小写不敏感）。
func TestLoadFromEnvLogLevelOverride(t *testing.T) {
	neutralizeConfigEnv(t)
	t.Setenv("APP_LOG_LEVEL", "debug")

	cfg := config.LoadFrom(filepath.Join(t.TempDir(), "config.yaml"))
	if cfg.Log.Level != "debug" {
		t.Fatalf("APP_LOG_LEVEL 未生效: %q", cfg.Log.Level)
	}
	if level, ok := logging.ParseLevel(cfg.Log.Level); !ok || level != logging.LevelDebug {
		t.Fatalf("配置的日志等级无法解析: %q -> %v (ok=%v)", cfg.Log.Level, level, ok)
	}
}

// TestLoadFromInvalidLogLevelWarns 验证非法的 log.level 会回退到 INFO 并打印告警，
// 避免用户把等级写错后无从察觉。
func TestLoadFromInvalidLogLevelWarns(t *testing.T) {
	neutralizeConfigEnv(t)
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("log:\n  level: verbose\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := config.LoadFrom(path)
	if cfg.Log.Level != "verbose" {
		t.Fatalf("配置原值应保留: %q", cfg.Log.Level)
	}
	if _, ok := logging.ParseLevel(cfg.Log.Level); ok {
		t.Fatal("verbose 不应被解析为合法等级")
	}

	warned := false
	for _, entry := range logging.Recent(logging.LevelInfo) {
		if entry.Level == logging.LevelWarn && strings.Contains(entry.Message, "log.level") && strings.Contains(entry.Message, "verbose") {
			warned = true
		}
	}
	if !warned {
		t.Error("非法 log.level 未产生告警日志")
	}
}

// TestRuntimeUpdatePreservesComments 验证运行期保存配置（Update 落盘）时注释不丢失。
func TestRuntimeUpdatePreservesComments(t *testing.T) {
	neutralizeConfigEnv(t)
	path := filepath.Join(t.TempDir(), "config.yaml")
	rt := config.NewRuntime(config.LoadFrom(path), path)
	if err := rt.Update(func(c *config.Config) { c.Server.Port = "9090" }); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "# 监听端口") {
		t.Error("Update 写回的配置丢失字段注释")
	}
	if !strings.Contains(text, `port: "9090"`) {
		t.Errorf("Update 写回的端口异常:\n%s", text)
	}
}
