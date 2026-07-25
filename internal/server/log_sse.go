// Package server Gin 路由、中间件、响应格式和 SSE 日志流推送。
package server

import (
	"io"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"nyxbot-go/internal/logging"
	"nyxbot-go/internal/response"
)

type logSSESession struct {
	level  logging.Level
	filter logFilterConfig
}

type logFilterConfig struct {
	Enabled         bool     `json:"enabled"`
	MinLevel        string   `json:"minLevel"`
	IncludeKeywords []string `json:"includeKeywords"`
	ExcludeKeywords []string `json:"excludeKeywords"`
	IncludePackages []string `json:"includePackages"`
	ExcludePackages []string `json:"excludePackages"`
	IncludeThreads  []string `json:"includeThreads"`
	UseRegex        bool     `json:"useRegex"`
}

type logSSEData struct {
	Live   string `json:"live"`
	Time   string `json:"time"`
	Thread string `json:"thread"`
	Pack   string `json:"pack"`
	Log    string `json:"log"`
}

var (
	logSSEMu       sync.RWMutex
	logSSESessions = map[string]logSSESession{}
)

func streamLogs(c *gin.Context) {
	level := logging.TrimLevel(c.DefaultQuery("level", "INFO"))
	sessionID := uuid.NewString()
	filter := defaultLogFilter(level)
	logs := logging.Subscribe(128)

	logSSEMu.Lock()
	logSSESessions[sessionID] = logSSESession{level: level, filter: filter}
	logSSEMu.Unlock()
	logging.DebugPack("server.sse", "SSE log stream connected: sessionId=%s level=%s", sessionID, level)

	defer func() {
		logging.Unsubscribe(logs)
		removeLogSSESession(sessionID)
		logging.DebugPack("server.sse", "SSE log stream disconnected: sessionId=%s", sessionID)
	}()

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	c.SSEvent("session", gin.H{"sessionId": sessionID})
	if history := filteredDTOList(sessionID, logging.Recent(logging.LevelTrace)); len(history) > 0 {
		c.SSEvent("history", history)
	}
	c.Writer.Flush()

	c.Stream(func(w io.Writer) bool {
		select {
		case entry, ok := <-logs:
			if !ok {
				return false
			}
			if !matchesSession(sessionID, entry) {
				return true
			}
			c.SSEvent("log", []logSSEData{toLogSSEData(entry)})
			return true
		case <-time.After(25 * time.Second):
			c.SSEvent("ping", gin.H{"time": time.Now().Unix()})
			return true
		case <-c.Request.Context().Done():
			return false
		}
	})
}

func logSSEStats(c *gin.Context) {
	logSSEMu.RLock()
	connections := len(logSSESessions)
	logSSEMu.RUnlock()

	response.Success(c, gin.H{
		"connections":       connections,
		"activeConnections": connections,
		"protocol":          "SSE (Server-Sent Events)",
	})
}

func updateLogSSEFilter(c *gin.Context) {
	sessionID := c.Query("sessionId")
	if sessionID == "" {
		response.Fail(c, 400, "sessionId不能为空")
		return
	}

	var filter logFilterConfig
	if err := c.ShouldBindJSON(&filter); err != nil {
		response.Fail(c, 400, "无效的过滤配置")
		return
	}

	logSSEMu.Lock()
	session, ok := logSSESessions[sessionID]
	if ok {
		session.filter = normalizeLogFilter(filter, session.level)
		logSSESessions[sessionID] = session
	}
	logSSEMu.Unlock()

	if !ok {
		response.Fail(c, 400, "会话不存在")
		return
	}

	logging.InfoPack("server.sse", "SSE filter updated: sessionId=%s", sessionID)
	response.Success(c, gin.H{"config": session.filter})
}

func resetLogSSEFilter(c *gin.Context) {
	sessionID := c.Query("sessionId")
	if sessionID == "" {
		response.Fail(c, 400, "sessionId不能为空")
		return
	}

	logSSEMu.Lock()
	session, ok := logSSESessions[sessionID]
	if ok {
		session.filter = normalizeLogFilter(session.filter, session.level)
		logSSESessions[sessionID] = session
	}
	logSSEMu.Unlock()

	if !ok {
		response.Fail(c, 400, "会话不存在")
		return
	}

	logging.InfoPack("server.sse", "SSE filter reset: sessionId=%s", sessionID)
	response.Success(c, gin.H{"config": session.filter})
}

func removeLogSSESession(sessionID string) {
	logSSEMu.Lock()
	delete(logSSESessions, sessionID)
	logSSEMu.Unlock()
}

func defaultLogFilter(level logging.Level) logFilterConfig {
	return logFilterConfig{Enabled: false, MinLevel: string(level)}
}

func normalizeLogFilter(filter logFilterConfig, fallback logging.Level) logFilterConfig {
	if filter.MinLevel == "" {
		filter.MinLevel = string(fallback)
	}
	return filter
}

func matchesSession(sessionID string, entry logging.Entry) bool {
	logSSEMu.RLock()
	session, ok := logSSESessions[sessionID]
	logSSEMu.RUnlock()
	if !ok {
		return false
	}

	filter := session.filter
	minLevel := session.level
	if filter.Enabled {
		minLevel = logging.TrimLevel(filter.MinLevel)
	}
	if !logLevelAllowed(entry.Level, minLevel) {
		return false
	}
	if !filter.Enabled {
		return true
	}

	dto := toLogSSEData(entry)
	text := dto.Log + " " + dto.Pack + " " + dto.Thread
	return matchKeywords(text, filter.IncludeKeywords, true, filter.UseRegex) &&
		matchKeywords(text, filter.ExcludeKeywords, false, filter.UseRegex) &&
		matchKeywords(dto.Pack, filter.IncludePackages, true, filter.UseRegex) &&
		matchKeywords(dto.Pack, filter.ExcludePackages, false, filter.UseRegex) &&
		matchKeywords(dto.Thread, filter.IncludeThreads, true, false)
}

func matchKeywords(text string, keywords []string, include bool, useRegex bool) bool {
	cleaned := make([]string, 0, len(keywords))
	for _, keyword := range keywords {
		if strings.TrimSpace(keyword) != "" {
			cleaned = append(cleaned, keyword)
		}
	}
	if len(cleaned) == 0 {
		return true
	}

	matched := false
	for _, keyword := range cleaned {
		if useRegex {
			re, err := regexp.Compile("(?i)" + keyword)
			if err == nil && re.MatchString(text) {
				matched = true
				break
			}
			continue
		}
		if strings.Contains(strings.ToLower(text), strings.ToLower(keyword)) {
			matched = true
			break
		}
	}

	if include {
		return matched
	}
	return !matched
}

func dtoList(entries []logging.Entry) []logSSEData {
	logs := make([]logSSEData, 0, len(entries))
	for _, entry := range entries {
		logs = append(logs, toLogSSEData(entry))
	}
	return logs
}

func filteredDTOList(sessionID string, entries []logging.Entry) []logSSEData {
	logs := make([]logSSEData, 0, len(entries))
	for _, entry := range entries {
		if matchesSession(sessionID, entry) {
			logs = append(logs, toLogSSEData(entry))
		}
	}
	return logs
}

func toLogSSEData(entry logging.Entry) logSSEData {
	level := entry.Level
	if level == logging.LevelHTTP {
		level = logging.LevelInfo
	}
	if level == logging.LevelPanic {
		level = logging.LevelError
	}
	thread := entry.Thread
	if thread == "" {
		thread = "goroutine-unknown"
	}
	pack := entry.Pack
	if pack == "" {
		pack = "unknown"
	}

	return logSSEData{
		Live:   string(level),
		Time:   entry.Time.Format("2006-01-02 15:04:05"),
		Thread: thread,
		Pack:   pack,
		Log:    entry.Message,
	}
}

func logLevelAllowed(actual logging.Level, min logging.Level) bool {
	order := map[logging.Level]int{
		logging.LevelTrace: 0,
		logging.LevelDebug: 1,
		logging.LevelInfo:  2,
		logging.LevelHTTP:  2,
		logging.LevelWarn:  3,
		logging.LevelError: 4,
		logging.LevelPanic: 4,
	}
	return order[actual] >= order[min]
}
