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
		"# GORM 慢查询告警阈值（毫秒），0=默认 600；不想看慢查询告警请把 sql_level 设为 error",
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
	// 慢查询告警阈值默认 600ms（遗物导入的 200ms 级批量写入在其下方，不再刷告警）
	if loaded.Log.SQLSlowMS != 600 {
		t.Errorf("慢查询告警阈值默认值应为 600ms，实际: %d", loaded.Log.SQLSlowMS)
	}
	if !strings.Contains(text, "sql_slow_ms: 600") {
		t.Errorf("生成的配置缺少 sql_slow_ms 默认值:\n%s", text)
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

// TestLoadFromMissingJwtSecretGeneratedAndPersisted 验证已有配置缺失 jwt_secret 时：
// 生成随机密钥并在启动补写时落盘（重启后 token 不失效），原有内容逐字节保留。
func TestLoadFromMissingJwtSecretGeneratedAndPersisted(t *testing.T) {
	neutralizeConfigEnv(t)
	path := filepath.Join(t.TempDir(), "config.yaml")
	original := "server:\n  port: \"8080\"\n"
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := config.LoadFrom(path)
	if cfg.Auth.JwtSecret == "" {
		t.Fatal("缺失 jwt_secret 时未生成兜底密钥")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(data), original) {
		t.Errorf("原有内容被改动:\n%s", string(data))
	}
	if !strings.Contains(string(data), "jwt_secret: "+cfg.Auth.JwtSecret) {
		t.Errorf("启动补写未把生成的 jwt_secret 落盘:\n%s", string(data))
	}
	// 重新加载：密钥保持不变（不会因重启让已发 token 失效），端口等既有值也不变
	again := config.LoadFrom(path)
	if again.Auth.JwtSecret != cfg.Auth.JwtSecret {
		t.Error("重新加载后 jwt_secret 发生变化")
	}
	if again.Server.Port != "8080" {
		t.Errorf("重新加载端口异常: %q", again.Server.Port)
	}
}

// TestLoadFromFillsMissingKeys 验证启动加载会检测「结构体里有、文件里缺」的配置项并按注释补写：
// 原有内容（自定义注释、未知键、键序、末尾换行）逐字节保留，检测结果有 WARN 提示，
// 且再次加载不会重复补写（幂等）。
func TestLoadFromFillsMissingKeys(t *testing.T) {
	neutralizeConfigEnv(t)
	path := filepath.Join(t.TempDir(), "config.yaml")
	original := `# 我手写的注释
server:
  port: "18080"
  gin_mode: release
  request_log: false
  trusted_proxies: []

# 自定义键（结构体里没有）必须保留
my_custom_key: hello

log:
  level: INFO
  startup: true
  console: true
  dir: data/logs
  max_file_size_mb: 5
  max_age_days: 7
  history_size: 50
  sql_level: warn
`
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := config.LoadFrom(path)
	if cfg.Log.SQLSlowMS != 600 {
		t.Errorf("缺失项未按默认值生效: sql_slow_ms=%d", cfg.Log.SQLSlowMS)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	// 缺失项按注释补写
	for _, want := range []string{
		"# GORM 慢查询告警阈值（毫秒），0=默认 600；不想看慢查询告警请把 sql_level 设为 error",
		"  sql_slow_ms: 600",
		"# SQLite 数据库配置",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("缺失配置项未补写: %q\n---\n%s", want, text)
		}
	}
	// 原有每一行都在，且末尾换行保持
	for _, line := range strings.Split(strings.TrimRight(original, "\n"), "\n") {
		if !strings.Contains(text, line) {
			t.Errorf("原始行被改动或丢失: %q\n---\n%s", line, text)
		}
	}
	if !strings.HasSuffix(text, "\n") {
		t.Error("末尾换行被改动")
	}
	// 检测结果必须可见（WARN 列出补写的键）
	warned := false
	for _, entry := range logging.Recent(logging.LevelWarn) {
		if entry.Pack == "config" && strings.Contains(entry.Message, "缺少") &&
			strings.Contains(entry.Message, "log.sql_slow_ms") {
			warned = true
		}
	}
	if !warned {
		t.Error("启动补写缺失配置项时未输出 WARN 提示")
	}
	// 幂等：再次加载不改动文件
	config.LoadFrom(path)
	second, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(second) != text {
		t.Errorf("重复加载改动了文件:\n%s", string(second))
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

// TestRuntimeUpdateWritesOnlyChangedLines 验证保存配置只改「值发生变化」的那一行：
// 用户手写的注释、空行、自定义键、行内注释与键序都逐字节保留，缺失的架构键只做追加。
func TestRuntimeUpdateWritesOnlyChangedLines(t *testing.T) {
	neutralizeConfigEnv(t)
	path := filepath.Join(t.TempDir(), "config.yaml")
	original := `# 我手写的文件头注释
server:
  port: "18080"          # 行内注释要保留
  gin_mode: release
  request_log: false
  trusted_proxies: []

# 自定义键（结构体里没有）必须保留
my_custom_key: hello

log:
  level: INFO
  startup: true
  console: true
  dir: data/logs
  max_file_size_mb: 5
  max_age_days: 7
  history_size: 50
  sql_level: warn
`
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}
	// LoadFrom 会在启动时补写缺失键（由 TestLoadFromFillsMissingKeys 覆盖）；
	// 这里把文件复原为「缺键」状态，用于验证保存路径同样只做最小写入并补写缺失键
	cfg := config.LoadFrom(path)
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}
	rt := config.NewRuntime(cfg, path)
	if err := rt.Update(func(c *config.Config) { c.Server.Port = "9090" }); err != nil {
		t.Fatalf("保存配置失败: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)

	// ① 只有 port 行被改写：取值已更新、行内注释保留（该行按标准格式重渲染，注释前间距归一化）
	if !strings.Contains(text, `  port: "9090" # 行内注释要保留`) {
		t.Errorf("port 行未按最小改动重写:\n%s", text)
	}
	if strings.Contains(text, "18080") {
		t.Errorf("旧端口值不应残留:\n%s", text)
	}
	// ② 原有每一行（除被替换的 port 行）都原样保留
	for _, line := range strings.Split(strings.TrimRight(original, "\n"), "\n") {
		if strings.Contains(line, `port: "18080"`) {
			continue
		}
		if !strings.Contains(text, line) {
			t.Errorf("原始行被改动或丢失: %q\n---\n%s", line, text)
		}
	}
	// ③ 缺失的架构键按注释补写（log.sql_slow_ms + 整段缺失的 database/bot/auth/warframe）
	for _, want := range []string{
		"# GORM 慢查询告警阈值（毫秒），0=默认 600；不想看慢查询告警请把 sql_level 设为 error",
		"  sql_slow_ms: 600",
		"# SQLite 数据库配置",
		"# OneBot 连接配置",
		"# Warframe 数据层配置",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("缺失键未补写: %q\n---\n%s", want, text)
		}
	}
	// ④ 补写后仍是合法配置，且解析结果与内存中的配置一致
	reloaded := config.LoadFrom(path)
	if reloaded.Server.Port != "9090" {
		t.Errorf("重新加载端口异常: %q", reloaded.Server.Port)
	}
	if reloaded.Log.SQLSlowMS != 600 {
		t.Errorf("补写的 sql_slow_ms 未生效: %d", reloaded.Log.SQLSlowMS)
	}
	// ⑤ 用户原文件的开头、键序与末尾换行保持不变
	if !strings.HasPrefix(text, "# 我手写的文件头注释\n") {
		t.Errorf("文件头注释位置被改动:\n%s", text)
	}
	if !strings.HasSuffix(text, "\n") {
		t.Errorf("文件末尾换行被改动:\n%q", text)
	}
	customIndex := strings.Index(text, "my_custom_key: hello")
	logIndex := strings.Index(text, "\nlog:\n")
	if customIndex < 0 || logIndex < 0 || customIndex > logIndex {
		t.Errorf("自定义键位置被改动:\n%s", text)
	}
}

// TestRuntimeUpdateRewritesFlowSection 验证文件里某一节被写成流式 {…} 时：
// 只把这一节整体改写为块状（其余内容不动），并在其中补齐缺失键——避免因无法逐行补丁而保存失败。
func TestRuntimeUpdateRewritesFlowSection(t *testing.T) {
	neutralizeConfigEnv(t)
	path := filepath.Join(t.TempDir(), "config.yaml")
	original := `# 顶部注释必须保留
server:
  port: "18080"
log: {level: INFO, startup: true, console: true, dir: data/logs, max_file_size_mb: 5, max_age_days: 7, history_size: 50, sql_level: warn}
`
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}
	// 同上：加载后再把文件复原为含流式分节的旧内容，让 Update 处理这份漂移的文件
	cfg := config.LoadFrom(path)
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}
	rt := config.NewRuntime(cfg, path)
	if err := rt.Update(func(c *config.Config) { c.Log.Level = "DEBUG" }); err != nil {
		t.Fatalf("流式分节的保存不应失败: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if strings.Contains(text, "log: {") {
		t.Errorf("流式分节未被改写为块状:\n%s", text)
	}
	if !strings.HasPrefix(text, "# 顶部注释必须保留\nserver:\n  port: \"18080\"\n") {
		t.Errorf("流式分节之外的内容被改动:\n%s", text)
	}
	for _, want := range []string{"  level: DEBUG", "  sql_slow_ms: 600", "# GORM 慢查询告警阈值"} {
		if !strings.Contains(text, want) {
			t.Errorf("流式分节未按预期重写（缺少 %q）:\n%s", want, text)
		}
	}
	if reloaded := config.LoadFrom(path); reloaded.Log.Level != "DEBUG" || reloaded.Log.SQLSlowMS != 600 {
		t.Errorf("重新加载异常: level=%q sql_slow_ms=%d", reloaded.Log.Level, reloaded.Log.SQLSlowMS)
	}
}

// TestRuntimeUpdateKeepsFileWhenConfigBroken 验证配置文件无法解析时拒绝写入：
// 保存接口报错（不静默整份重写），用户文件逐字节保留。
func TestRuntimeUpdateKeepsFileWhenConfigBroken(t *testing.T) {
	neutralizeConfigEnv(t)
	path := filepath.Join(t.TempDir(), "config.yaml")
	broken := []byte("server:\n  port: \"8080\"\n\t broken: [\n")
	if err := os.WriteFile(path, broken, 0644); err != nil {
		t.Fatal(err)
	}
	rt := config.NewRuntime(config.LoadFrom(path), path)
	if err := rt.Update(func(c *config.Config) { c.Server.Port = "9090" }); err == nil {
		t.Fatal("配置无法解析时应返回错误")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(broken) {
		t.Errorf("解析失败时不应改写用户文件:\n%s", string(data))
	}
}
