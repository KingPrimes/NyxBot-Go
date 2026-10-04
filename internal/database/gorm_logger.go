// GORM 日志适配器：把 GORM 的 SQL 日志转发到项目统一日志 internal/logging。
// GORM 自带 logger（logger.Default）直接写 stdout 并带 ANSI 颜色，会绕过 pack/级别/文件轮转与 SSE 推送，
// 因此这里实现 logger.Interface，把所有输出交给 logging.*Pack（pack = database.sql）。
package database

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"nyxbot-go/internal/logging"
)

// gormLogPack GORM 日志统一使用的 pack 名，SSE/前端可按 pack = database.sql 过滤。
const gormLogPack = "database.sql"

// DefaultSQLSlowThreshold 慢查询告警默认阈值（对应 config.yaml 的 log.sql_slow_ms 缺省值）。
// 600ms 而非 GORM 默认的 200ms：遗物导入的子表批量 INSERT（3002 行 / 18012 个绑定参数）
// 实测约 200ms（纯 Go 驱动 modernc.org/sqlite 的参数绑定随参数个数超线性），
// 用 200ms 会在每次数据更新时刷出无意义的告警。
const DefaultSQLSlowThreshold = 600 * time.Millisecond

// ParseSQLSlowThreshold 把 config.yaml 的 log.sql_slow_ms 解析为慢查询阈值：
// <=0（字段缺失/未设置/非法）回退 DefaultSQLSlowThreshold，保证老配置文件升级后行为不突变。
// 想完全不记慢查询请用 ParseSQLLogLevel 的 error/silent，而不是把阈值当作开关。
func ParseSQLSlowThreshold(ms int) time.Duration {
	if ms <= 0 {
		return DefaultSQLSlowThreshold
	}
	return time.Duration(ms) * time.Millisecond
}

// gormMaxSQLLength 单条 SQL 日志的最大长度（字节）：批量 INSERT（500 行实测 58KB）与多条件 LIKE
// 会把一条日志撑到几十 KB，挤压日志文件与 SSE 推送，超长部分截断。
const gormMaxSQLLength = 1000

// gormLogger 实现 gorm.io/gorm/logger.Interface，并把日志转发到 internal/logging。
type gormLogger struct {
	level                logger.LogLevel // 明细级别：Silent/Error/Warn/Info
	slowThreshold        time.Duration   // 慢查询阈值（<=0 表示不记录慢查询）
	ignoreRecordNotFound bool            // 是否忽略 gorm.ErrRecordNotFound（本项目大量查询是可选命中）
}

// ParseSQLLogLevel 把 config.yaml 的 log.sql_level 字符串解析为 GORM 日志级别：
// silent（含 off/none）/ error / warn（默认）/ info（含 debug/all，输出全部 SQL）。
// 空值与无法识别的值回退 warn，保证默认只记录错误与慢查询。
func ParseSQLLogLevel(level string) logger.LogLevel {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "silent", "off", "none":
		return logger.Silent
	case "error":
		return logger.Error
	case "info", "debug", "all":
		return logger.Info
	default:
		return logger.Warn
	}
}

// NewGORMLogger 构造转发到统一日志（pack=database.sql）的 GORM logger。
// level 控制明细程度；slowThreshold 以上的慢查询记 WARN；ignoreRecordNotFound 为 true 时
// 「记录不存在」不再按 ERROR 输出（本项目大量查询是可选命中，未命中由业务回退默认值）。
func NewGORMLogger(level logger.LogLevel, slowThreshold time.Duration, ignoreRecordNotFound bool) logger.Interface {
	return gormLogger{level: level, slowThreshold: slowThreshold, ignoreRecordNotFound: ignoreRecordNotFound}
}

// LogMode 返回切换级别后的 logger 副本（对齐 GORM logger.Interface 的链式语义）。
func (l gormLogger) LogMode(level logger.LogLevel) logger.Interface {
	l.level = level
	return l
}

// Info 记录 GORM 内部信息（迁移提示等），级别达标时以 INFO 输出。
func (l gormLogger) Info(_ context.Context, message string, args ...any) {
	if l.level >= logger.Info {
		logging.InfoPack(gormLogPack, message, args...)
	}
}

// Warn 记录 GORM 警告，级别达标时以 WARN 输出。
func (l gormLogger) Warn(_ context.Context, message string, args ...any) {
	if l.level >= logger.Warn {
		logging.WarnPack(gormLogPack, message, args...)
	}
}

// Error 记录 GORM 错误，级别达标时以 ERROR 输出。
func (l gormLogger) Error(_ context.Context, message string, args ...any) {
	if l.level >= logger.Error {
		logging.ErrorPack(gormLogPack, message, args...)
	}
}

// Trace 记录单条 SQL：执行错误（未忽略时）→ ERROR，超过慢查询阈值 → WARN，
// 其余仅在 Info 级别下以 DEBUG 输出，保证默认（Warn）配置不刷 SQL。
func (l gormLogger) Trace(_ context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.level <= logger.Silent {
		return
	}
	elapsed := time.Since(begin)
	sql, rows := fc()
	sql = truncateSQL(sql, gormMaxSQLLength)
	elapsedMS := float64(elapsed.Nanoseconds()) / 1e6

	// 「记录不存在」是业务预期（可选命中后回退原值），忽略时不占用 ERROR 级别
	ignored := l.ignoreRecordNotFound && errors.Is(err, gorm.ErrRecordNotFound)

	switch {
	case err != nil && !ignored && l.level >= logger.Error:
		logging.ErrorPack(gormLogPack, "%v [%.3fms] [rows:%d] %s", err, elapsedMS, rows, sql)
	case l.slowThreshold > 0 && elapsed > l.slowThreshold && l.level >= logger.Warn:
		logging.WarnPack(gormLogPack, "SLOW SQL >= %v [%.3fms] [rows:%d] %s", l.slowThreshold, elapsedMS, rows, sql)
	case l.level >= logger.Info:
		logging.DebugPack(gormLogPack, "[%.3fms] [rows:%d] %s", elapsedMS, rows, sql)
	}
}

// truncateSQL 截断过长 SQL 文本：截断点回退到完整 rune 边界，避免产生非法 UTF-8（SSE 走 JSON 编码）。
func truncateSQL(sql string, limit int) string {
	if limit <= 0 || len(sql) <= limit {
		return sql
	}
	cut := limit
	for cut > 0 && !utf8.RuneStart(sql[cut]) {
		cut--
	}
	return sql[:cut] + fmt.Sprintf("...(%d bytes truncated)", len(sql)-cut)
}
