package system

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"nyxbot-go/internal/database"
	"nyxbot-go/internal/enum/nyxbot"
	model "nyxbot-go/internal/model/system"
	"nyxbot-go/internal/response"
)

// LogHandler 操作日志查询接口处理器。
// 数据来自 log_info 表（写入侧为 Bot 指令执行与后续阶段的业务埋点），本处理器只读。
type LogHandler struct{}

// NewLogHandler 创建操作日志查询处理器。
func NewLogHandler() *LogHandler {
	return &LogHandler{}
}

// logOption 下拉选项结构，与 WebUI 的 Api.SystemLog.Option 对齐。
type logOption struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// Codes 处理 GET /log/codes，返回指令下拉选项（label=value=指令正则的第一个别名）。
func (h *LogHandler) Codes(c *gin.Context) {
	options := make([]logOption, 0, len(nyxbot.CodesOrder))
	for _, code := range nyxbot.CodesOrder {
		alias := firstCommandAlias(nyxbot.CodesInfo[code].Comm)
		options = append(options, logOption{Label: alias, Value: alias})
	}
	response.Success(c, options)
}

// Titles 处理 GET /log/titles，返回日志标题下拉选项（label=中文名，value=枚举名）。
func (h *LogHandler) Titles(c *gin.Context) {
	options := make([]logOption, 0, len(nyxbot.LogTitles))
	for _, title := range nyxbot.LogTitles {
		options = append(options, logOption{Label: title.Title(), Value: string(title)})
	}
	response.Success(c, options)
}

// logListRequest POST /log/list 的请求体：分页参数 + 可选过滤条件，
// 字段与 WebUI useTable 提交的 searchParams（current/size/code/groupUid）对齐。
type logListRequest struct {
	Current  int    `json:"current"`
	Size     int    `json:"size"`
	Title    string `json:"title"`
	Code     string `json:"code"`
	GroupUID *int64 `json:"groupUid"`
}

// List 处理 POST /log/list，按标题/指令/群组精确过滤并分页返回操作日志，
// 对齐 Java LogInfoRepository.findAllPageable（等值条件、无显式排序）。
func (h *LogHandler) List(c *gin.Context) {
	var req logListRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数格式错误")
		return
	}
	if req.Current < 1 {
		req.Current = 1
	}
	if req.Size <= 0 {
		req.Size = 15
	}

	db := database.DB.Model(&model.LogInfo{})
	if req.Title != "" {
		db = db.Where("title = ?", req.Title)
	}
	if req.Code != "" {
		db = db.Where("code = ?", req.Code)
	}
	if req.GroupUID != nil {
		db = db.Where("group_uid = ?", *req.GroupUID)
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		response.Error(c, "日志查询失败: "+err.Error())
		return
	}
	records := make([]model.LogInfo, 0, req.Size)
	if err := db.Offset((req.Current - 1) * req.Size).Limit(req.Size).Find(&records).Error; err != nil {
		response.Error(c, "日志查询失败: "+err.Error())
		return
	}

	response.Page(c, total, req.Size, req.Current, records)
}

// Detail 处理 GET /log/detail/:id，返回单条日志详情；
// 记录不存在时返回 data=null 的成功响应（对齐 Java success() 空载语义）。
func (h *LogHandler) Detail(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "无效的日志ID")
		return
	}

	var info model.LogInfo
	if err := database.DB.First(&info, id).Error; err != nil {
		response.Success(c, nil)
		return
	}
	response.Success(c, info)
}

// firstCommandAlias 取指令正则的第一个别名用于下拉展示，
// 对齐 Java StringUtils.removeMatcher 的意图：按 | 分割取首段，
// 去除正则元字符（^ $ ( ) 与 .*?）后 trim。
func firstCommandAlias(comm string) string {
	first, _, _ := strings.Cut(comm, "|")
	first = strings.ReplaceAll(first, ".*?", "")
	first = strings.ReplaceAll(first, "^", "")
	first = strings.ReplaceAll(first, "$", "")
	first = strings.ReplaceAll(first, "(", "")
	first = strings.ReplaceAll(first, ")", "")
	return strings.TrimSpace(first)
}
