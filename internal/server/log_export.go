package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"nyxbot-go/internal/logging"
	"nyxbot-go/internal/response"
)

// logSearchParams /api/logs 搜索、统计、导出共用的过滤参数（query 传参），
// 对齐 Java LogExportController 的 RequestParam 定义。
type logSearchParams struct {
	keyword   string
	startTime *int64 // 毫秒时间戳
	endTime   *int64
	levels    []string
	useRegex  bool
}

// parseLogSearchParams 解析搜索参数。levels 兼容重复参数与逗号分隔两种形式
// （对齐 Spring 对 List<String> 的绑定行为）；useRegex 默认 false。
func parseLogSearchParams(c *gin.Context) logSearchParams {
	params := logSearchParams{
		keyword:  c.Query("keyword"),
		useRegex: c.DefaultQuery("useRegex", "false") == "true",
	}
	if v := c.Query("startTime"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			params.startTime = &n
		}
	}
	if v := c.Query("endTime"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			params.endTime = &n
		}
	}
	for _, raw := range c.QueryArray("levels") {
		for _, level := range strings.Split(raw, ",") {
			if level = strings.TrimSpace(level); level != "" {
				params.levels = append(params.levels, level)
			}
		}
	}
	return params
}

// filterLogEntries 对齐 Java LogSearchService.searchLogs：时间范围 → 级别 → 关键词依次过滤。
func filterLogEntries(entries []logging.Entry, params logSearchParams) []logging.Entry {
	filtered := make([]logging.Entry, 0, len(entries))
	for _, entry := range entries {
		if !matchLogTimeRange(entry, params.startTime, params.endTime) {
			continue
		}
		if !matchLogLevels(entry, params.levels) {
			continue
		}
		if !matchLogKeyword(entry, params.keyword, params.useRegex) {
			continue
		}
		filtered = append(filtered, entry)
	}
	return filtered
}

// matchLogTimeRange 毫秒时间戳范围匹配，上下界均可选（闭区间）。
func matchLogTimeRange(entry logging.Entry, startTime, endTime *int64) bool {
	ms := entry.Time.UnixMilli()
	if startTime != nil && ms < *startTime {
		return false
	}
	return endTime == nil || ms <= *endTime
}

// matchLogLevels 级别精确匹配（不区分大小写），levels 为空时全部通过。
func matchLogLevels(entry logging.Entry, levels []string) bool {
	if len(levels) == 0 {
		return true
	}
	for _, level := range levels {
		if strings.EqualFold(level, string(entry.Level)) {
			return true
		}
	}
	return false
}

// matchLogKeyword 在「日志内容 + 包名」上匹配关键词：默认不区分大小写子串匹配；
// useRegex 时按不区分大小写正则查找，无效正则不匹配任何条目
// （对齐 Java matchText 的 PatternSyntaxException 分支）。
func matchLogKeyword(entry logging.Entry, keyword string, useRegex bool) bool {
	if keyword == "" {
		return true
	}
	text := entry.Message + " " + entry.Pack
	if useRegex {
		re, err := regexp.Compile("(?i)" + keyword)
		if err != nil {
			return false
		}
		return re.MatchString(text)
	}
	return strings.Contains(strings.ToLower(text), strings.ToLower(keyword))
}

// buildLogSearchFilter 构造响应中的过滤条件回显，缺失的时间参数以空字符串占位
// （对齐 Java buildFilter 的 null -> "" 行为）。
func buildLogSearchFilter(params logSearchParams) gin.H {
	var startTime, endTime any = "", ""
	if params.startTime != nil {
		startTime = *params.startTime
	}
	if params.endTime != nil {
		endTime = *params.endTime
	}
	levels := params.levels
	if levels == nil {
		levels = []string{}
	}
	return gin.H{
		"keyword":   params.keyword,
		"startTime": startTime,
		"endTime":   endTime,
		"levels":    levels,
		"useRegex":  params.useRegex,
	}
}

// searchLogs 处理 GET /api/logs/search，在内存历史日志中按条件搜索。
func searchLogs(c *gin.Context) {
	params := parseLogSearchParams(c)
	logs := filterLogEntries(logging.Recent(logging.LevelTrace), params)

	response.Success(c, gin.H{
		"total":  len(logs),
		"filter": buildLogSearchFilter(params),
		"logs":   dtoList(logs),
	})
}

// logStats 处理 GET /api/logs/stats，返回内存日志缓存的统计信息，
// 字段对齐 Java LogExportController.getLogStats。
func logStats(c *gin.Context) {
	entries := logging.Recent(logging.LevelTrace)
	maxSize := logging.HistorySize()

	levelCounts := map[string]int64{}
	for _, entry := range entries {
		levelCounts[string(normalizeLogLevel(entry.Level))]++
	}
	usage := 0.0
	if maxSize > 0 {
		usage = float64(len(entries)) * 100.0 / float64(maxSize)
	}

	response.Success(c, gin.H{
		"total":        len(entries),
		"maxCacheSize": maxSize,
		"levelCounts":  levelCounts,
		"cacheStatus":  fmt.Sprintf("LogCache[size=%d, max=%d, usage=%.1f%%]", len(entries), maxSize, usage),
		"statistics": fmt.Sprintf("Total: %d, TRACE: %d, DEBUG: %d, INFO: %d, WARN: %d, ERROR: %d",
			len(entries), levelCounts["TRACE"], levelCounts["DEBUG"], levelCounts["INFO"], levelCounts["WARN"], levelCounts["ERROR"]),
	})
}

// exportLogsTxt 处理 GET /api/logs/export/txt，将过滤后的内存历史日志导出为 TXT 附件，
// 文本格式对齐 Java exportToTxt（useRegex 固定为 false）。
func exportLogsTxt(c *gin.Context) {
	params := parseLogSearchParams(c)
	params.useRegex = false
	logs := filterLogEntries(logging.Recent(logging.LevelTrace), params)

	var buf strings.Builder
	buf.WriteString("# 日志导出\n")
	buf.WriteString("# 导出时间: " + time.Now().Format("Mon Jan 02 15:04:05 MST 2006") + "\n")
	fmt.Fprintf(&buf, "# 总条数: %d\n", len(logs))
	fmt.Fprintf(&buf, "# 过滤条件: keyword=%s, levels=%s\n", params.keyword, formatLevelsForExport(params.levels))
	buf.WriteString("# " + strings.Repeat("=", 80) + "\n\n")
	for _, entry := range logs {
		dto := toLogSSEData(entry)
		fmt.Fprintf(&buf, "%s %s [%s] %s : %s\n", dto.Live, dto.Time, dto.Thread, dto.Pack, dto.Log)
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=logs_%d.txt", time.Now().UnixMilli()))
	c.Data(http.StatusOK, "text/plain;charset=UTF-8", []byte(buf.String()))
}

// exportLogsJSON 处理 GET /api/logs/export/json，将过滤后的内存历史日志导出为 JSON 附件，
// 载荷结构对齐 Java exportToJson（useRegex 固定为 false）。
func exportLogsJSON(c *gin.Context) {
	params := parseLogSearchParams(c)
	params.useRegex = false
	logs := filterLogEntries(logging.Recent(logging.LevelTrace), params)

	payload, err := json.Marshal(gin.H{
		"exportTime": time.Now().UnixMilli(),
		"total":      len(logs),
		"filter":     buildLogSearchFilter(params),
		"logs":       dtoList(logs),
	})
	if err != nil {
		response.Error(c, "日志导出失败: "+err.Error())
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=logs_%d.json", time.Now().UnixMilli()))
	c.Data(http.StatusOK, "application/json;charset=UTF-8", payload)
}

// formatLevelsForExport 对齐 Java 导出文件头中 List 的打印格式：空为 "null"，否则 "[a, b]"。
func formatLevelsForExport(levels []string) string {
	if len(levels) == 0 {
		return "null"
	}
	return "[" + strings.Join(levels, ", ") + "]"
}
