// Package logging 自实现的日志系统，支持 slog 风格分级、文件轮转、控制台输出
// 和 SSE 实时日志订阅。替代 Go 标准 log 包和第三方日志库。
package logging

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Level 日志级别类型。
type Level string

const (
	LevelTrace Level = "TRACE" // 跟踪级别
	LevelDebug Level = "DEBUG" // 调试级别
	LevelInfo  Level = "INFO"  // 信息级别
	LevelWarn  Level = "WARN"  // 警告级别
	LevelError Level = "ERROR" // 错误级别
	LevelPanic Level = "PANIC" // 恐慌级别
	LevelHTTP  Level = "HTTP"  // HTTP 请求日志
)

// Entry 单条日志条目，包含时间、级别、来源包、协程和消息。
type Entry struct {
	Time    time.Time `json:"time"`
	Level   Level     `json:"level"`
	Pack    string    `json:"pack"`   // 来源包名
	Thread  string    `json:"thread"` // goroutine ID
	Message string    `json:"message"`
	Fields  string    `json:"fields,omitempty"` // 扩展字段
}

// Subscriber 日志订阅者通道，用于 SSE 实时推送。
type Subscriber chan Entry

// Config 日志初始配置参数（Init 时传入）。
type Config struct {
	Dir            string
	MaxFileSize    int64
	MaxAgeDays     int
	HistorySize    int
	ConsoleEnabled bool
}

var (
	mu          sync.RWMutex
	subscribers = map[Subscriber]struct{}{}
	history     []Entry
	std         = log.New(os.Stdout, "", 0)
	current     = LevelInfo
	logger      *fileLogger
	config      = Config{
		Dir:            filepath.Join("data", "logs"),
		MaxFileSize:    5 * 1024 * 1024,
		MaxAgeDays:     7,
		HistorySize:    50,
		ConsoleEnabled: true,
	}
)

// Init 初始化日志系统，可多次调用以重新配置。
func Init(cfg Config) {
	mu.Lock()
	defer mu.Unlock()
	if logger != nil {
		_ = logger.Close()
		logger = nil
	}

	if cfg.Dir != "" {
		config.Dir = cfg.Dir
	}
	if cfg.MaxFileSize > 0 {
		config.MaxFileSize = cfg.MaxFileSize
	}
	if cfg.MaxAgeDays > 0 {
		config.MaxAgeDays = cfg.MaxAgeDays
	}
	if cfg.HistorySize > 0 {
		config.HistorySize = cfg.HistorySize
	}
	config.ConsoleEnabled = cfg.ConsoleEnabled

	log.SetFlags(0)
	log.SetOutput(os.Stdout)

	var err error
	logger, err = newFileLogger(config.Dir, config.MaxFileSize, config.MaxAgeDays)
	if err != nil {
		log.Printf("[WARN] file logger init failed: %v", err)
		logger = nil
	}
}

// Subscribe 创建一个新的日志订阅者通道，用于实时接收日志条目。
func Subscribe(buffer int) Subscriber {
	if buffer <= 0 {
		buffer = 64
	}

	ch := make(Subscriber, buffer)
	mu.Lock()
	subscribers[ch] = struct{}{}
	mu.Unlock()
	return ch
}

// Unsubscribe 取消订阅并关闭通道。
func Unsubscribe(ch Subscriber) {
	mu.Lock()
	if _, ok := subscribers[ch]; ok {
		delete(subscribers, ch)
		close(ch)
	}
	mu.Unlock()
}

// Info 记录 INFO 级别日志，自动从调用栈推断来源包名。
func Info(message string, args ...any) {
	write(LevelInfo, 2, message, args...)
}

// InfoPack 记录 INFO 级别日志，显式指定来源包名。
func InfoPack(pack string, message string, args ...any) {
	writePack(LevelInfo, pack, message, args...)
}

// Debug 记录 DEBUG 级别日志，自动推断来源包名。
func Debug(message string, args ...any) {
	write(LevelDebug, 2, message, args...)
}

// DebugPack 记录 DEBUG 级别日志，显式指定来源包名。
func DebugPack(pack string, message string, args ...any) {
	writePack(LevelDebug, pack, message, args...)
}

// Trace 记录 TRACE 级别日志，自动推断来源包名。
func Trace(message string, args ...any) {
	write(LevelTrace, 2, message, args...)
}

// TracePack 记录 TRACE 级别日志，显式指定来源包名。
func TracePack(pack string, message string, args ...any) {
	writePack(LevelTrace, pack, message, args...)
}

// Warn 记录 WARN 级别日志，自动推断来源包名。
func Warn(message string, args ...any) {
	write(LevelWarn, 2, message, args...)
}

// WarnPack 记录 WARN 级别日志，显式指定来源包名。
func WarnPack(pack string, message string, args ...any) {
	writePack(LevelWarn, pack, message, args...)
}

// Error 记录 ERROR 级别日志，自动推断来源包名。
func Error(message string, args ...any) {
	write(LevelError, 2, message, args...)
}

// ErrorPack 记录 ERROR 级别日志，显式指定来源包名。
func ErrorPack(pack string, message string, args ...any) {
	writePack(LevelError, pack, message, args...)
}

// Panic 记录 PANIC 级别日志，自动推断来源包名。
func Panic(message string, args ...any) {
	write(LevelPanic, 2, message, args...)
}

// PanicPack 记录 PANIC 级别日志，显式指定来源包名。
func PanicPack(pack string, message string, args ...any) {
	writePack(LevelPanic, pack, message, args...)
}

// HTTP 记录 HTTP 请求日志（固定 pack="http"），由中间件调用。
func HTTP(status int, method, path string, latency time.Duration, clientIP string) {
	entry := Entry{
		Time:    time.Now(),
		Level:   LevelHTTP,
		Pack:    "http",
		Thread:  currentThread(),
		Message: fmt.Sprintf("[%d] %s %s | %v | %s", status, method, path, latency, clientIP),
	}
	output(entry)
}

func write(level Level, skip int, message string, args ...any) {
	if len(args) > 0 {
		message = fmt.Sprintf(message, args...)
	}
	pack, thread := callerInfo(skip)
	output(Entry{Time: time.Now(), Level: level, Pack: pack, Thread: thread, Message: message})
}

func writePack(level Level, pack string, message string, args ...any) {
	if len(args) > 0 {
		message = fmt.Sprintf(message, args...)
	}
	if strings.TrimSpace(pack) == "" {
		pack = "unknown"
	}
	output(Entry{Time: time.Now(), Level: level, Pack: pack, Thread: currentThread(), Message: message})
}

func output(entry Entry) {
	formatted := format(entry)
	if config.ConsoleEnabled {
		std.Println(formatted)
	}
	if logger != nil {
		_ = logger.Write([]byte(formatted + "\n"))
	}
	appendHistory(entry)
	broadcast(entry)
}

// Recent 返回内存中缓存的、级别不低于 min 的历史日志。
func Recent(min Level) []Entry {
	mu.RLock()
	defer mu.RUnlock()

	logs := make([]Entry, 0, len(history))
	for _, entry := range history {
		if levelAllowed(entry.Level, min) {
			logs = append(logs, entry)
		}
	}
	return logs
}

// HistorySize 返回内存历史日志的最大缓存条数（对应 Java LogCacheManager 的 MAX_CACHE_SIZE）。
func HistorySize() int {
	mu.RLock()
	defer mu.RUnlock()
	return config.HistorySize
}

func format(entry Entry) string {
	timestamp := entry.Time.Format("2006-01-02 15:04:05")
	base := fmt.Sprintf("%s [%s] [%s] [%s] %s", timestamp, entry.Level, entry.Thread, entry.Pack, entry.Message)
	if entry.Fields != "" {
		return base + " " + entry.Fields
	}
	return base
}

func broadcast(entry Entry) {
	mu.RLock()
	defer mu.RUnlock()

	for ch := range subscribers {
		select {
		case ch <- entry:
		default:
		}
	}
}

func appendHistory(entry Entry) {
	mu.Lock()
	defer mu.Unlock()

	if config.HistorySize <= 0 {
		config.HistorySize = 50
	}
	if len(history) >= config.HistorySize {
		history = history[1:]
	}
	history = append(history, entry)
}

// SetLevel 设置全局最低日志级别。
func SetLevel(level Level) {
	current = level
}

// LevelEnabled 判断指定级别是否被当前全局级别允许。
func LevelEnabled(level Level) bool {
	return levelAllowed(level, current)
}

// TrimLevel 将字符串解析为 Level，不合法时返回 LevelInfo。
func TrimLevel(s string) Level {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "TRACE":
		return LevelTrace
	case "DEBUG":
		return LevelDebug
	case "WARN", "WARNING":
		return LevelWarn
	case "ERROR":
		return LevelError
	case "PANIC":
		return LevelPanic
	case "HTTP":
		return LevelHTTP
	default:
		return LevelInfo
	}
}

func levelAllowed(actual Level, min Level) bool {
	return levelValue(actual) >= levelValue(min)
}

func levelValue(level Level) int {
	switch level {
	case LevelTrace:
		return 0
	case LevelDebug:
		return 1
	case LevelInfo, LevelHTTP:
		return 2
	case LevelWarn:
		return 3
	case LevelError, LevelPanic:
		return 4
	default:
		return 2
	}
}

func callerInfo(skip int) (string, string) {
	pc, file, _, ok := runtime.Caller(skip)
	if !ok {
		return "unknown", currentThread()
	}

	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return filepath.Base(file), currentThread()
	}

	name := fn.Name()
	pack := shortPackage(name)
	if pack == "" {
		pack = filepath.Base(file)
	}
	return pack, currentThread()
}

func shortPackage(name string) string {
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		name = name[idx+1:]
	}
	if idx := strings.LastIndex(name, "."); idx >= 0 {
		return name[:idx]
	}
	return name
}

func currentThread() string {
	buf := make([]byte, 64)
	n := runtime.Stack(buf, false)
	line := string(buf[:n])
	line = strings.TrimPrefix(line, "goroutine ")
	if idx := strings.IndexByte(line, ' '); idx > 0 {
		return "goroutine-" + line[:idx]
	}
	return "goroutine-unknown"
}

type fileLogger struct {
	mu      sync.Mutex
	dir     string
	maxSize int64
	maxAge  int
	current string
	file    *os.File
	size    int64
	date    string
	index   int
	paths   map[string]struct{}
}

func newFileLogger(dir string, maxSize int64, maxAge int) (*fileLogger, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	fl := &fileLogger{
		dir:     dir,
		maxSize: maxSize,
		maxAge:  maxAge,
		paths:   map[string]struct{}{},
	}
	if err := fl.cleanupOldFiles(); err != nil {
		return nil, err
	}
	return fl, nil
}

func (f *fileLogger) Write(p []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.rotateIfNeeded(); err != nil {
		return err
	}
	if f.file == nil {
		return nil
	}
	if _, err := f.file.Write(p); err != nil {
		return err
	}
	f.size += int64(len(p))
	return nil
}

func (f *fileLogger) rotateIfNeeded() error {
	today := time.Now().Format("2006-01-02")
	if f.file != nil && f.date == today && f.size < f.maxSize {
		return nil
	}

	if f.file != nil {
		_ = f.file.Close()
		f.file = nil
	}

	f.date = today
	f.index = f.nextIndex(today)
	path := filepath.Join(f.dir, fmt.Sprintf("%s-%d.log", today, f.index))
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	stat, err := file.Stat()
	if err == nil {
		f.size = stat.Size()
	} else {
		f.size = 0
	}
	f.file = file
	f.current = path
	f.paths[path] = struct{}{}
	return nil
}

func (f *fileLogger) nextIndex(today string) int {
	pattern := filepath.Join(f.dir, fmt.Sprintf("%s-*.log", today))
	matches, _ := filepath.Glob(pattern)
	maxIndex := -1
	latestPath := ""
	for _, match := range matches {
		if match == f.current {
			continue
		}
		base := filepath.Base(match)
		parts := strings.Split(strings.TrimSuffix(base, ".log"), "-")
		if len(parts) < 4 {
			continue
		}
		indexPart := parts[len(parts)-1]
		idx, err := strconv.Atoi(indexPart)
		if err == nil && idx > maxIndex {
			maxIndex = idx
			latestPath = match
		}
	}
	if maxIndex >= 0 && latestPath != "" {
		if info, err := os.Stat(latestPath); err == nil && info.Size() < f.maxSize {
			return maxIndex
		}
	}
	return maxIndex + 1
}

func (f *fileLogger) cleanupOldFiles() error {
	entries, err := os.ReadDir(f.dir)
	if err != nil {
		return err
	}
	cutoff := time.Now().AddDate(0, 0, -f.maxAge)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".log") {
			continue
		}
		fullPath := filepath.Join(f.dir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			_ = os.Remove(fullPath)
		}
	}
	return nil
}

func (f *fileLogger) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.file != nil {
		err := f.file.Close()
		f.file = nil
		return err
	}
	return nil
}

// FileLoggerDir 返回日志文件目录路径。
func FileLoggerDir() string {
	return config.Dir
}

// FileLoggerPattern 返回日志文件的 Glob 匹配模式。
func FileLoggerPattern() string {
	return filepath.Join(config.Dir, "*.log")
}

// LogFilePaths 返回所有日志文件的路径列表（按文件名排序）。
func LogFilePaths() []string {
	files, _ := filepath.Glob(FileLoggerPattern())
	sort.Strings(files)
	return files
}
