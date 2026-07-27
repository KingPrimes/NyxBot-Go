// 统一响应封装，对应 Java NyxBot 的 ApiResponse
// 所有 API 响应格式：{ code, msg, data }
package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Response 统一 API 响应结构 { code, msg, data }。
type Response struct {
	Code int    `json:"code"` // 业务状态码（200 成功）
	Msg  string `json:"msg"`  // 提示信息
	Data any    `json:"data"` // 响应数据
}

// PageData 分页响应结构 { total, size, current, records }。
type PageData struct {
	Total   int64 `json:"total"`   // 总记录数
	Size    int   `json:"size"`    // 每页大小
	Current int   `json:"current"` // 当前页码
	Records any   `json:"records"` // 当前页数据列表
}

// Success 返回成功响应（code=200, msg="success"）。
func Success(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Response{
		Code: 200,
		Msg:  "success",
		Data: data,
	})
}

// SuccessMsg 返回成功响应，自定义 msg。
func SuccessMsg(c *gin.Context, msg string, data any) {
	c.JSON(http.StatusOK, Response{
		Code: 200,
		Msg:  msg,
		Data: data,
	})
}

// Page 返回分页成功响应。
func Page(c *gin.Context, total int64, size, current int, records any) {
	Success(c, PageData{
		Total:   total,
		Size:    size,
		Current: current,
		Records: records,
	})
}

// Fail 返回失败响应，设置指定的状态码和消息。
func Fail(c *gin.Context, code int, msg string) {
	FailWithHTTP(c, code, code, msg)
}

// FailWithHTTP 返回失败响应，并允许 HTTP 状态码与业务状态码分别指定。
func FailWithHTTP(c *gin.Context, httpStatus, code int, msg string) {
	c.JSON(httpStatus, Response{
		Code: code,
		Msg:  msg,
		Data: nil,
	})
}

// Unauthorized 返回 401 未授权响应，并终止请求链。
func Unauthorized(c *gin.Context, msg string) {
	if msg == "" {
		msg = "未登录或 token 已过期"
	}
	Fail(c, http.StatusUnauthorized, msg)
	c.Abort()
}

// Forbidden 返回 403 禁止访问响应，并终止请求链。
func Forbidden(c *gin.Context, msg string) {
	if msg == "" {
		msg = "无权限访问"
	}
	Fail(c, http.StatusForbidden, msg)
	c.Abort()
}

// Error 返回 500 服务器内部错误响应。
func Error(c *gin.Context, msg string) {
	Fail(c, http.StatusInternalServerError, msg)
}
