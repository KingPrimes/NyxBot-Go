// 统一响应封装，对应 Java NyxBot 的 ApiResponse
// 所有 API 响应格式：{ code, msg, data }
package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Response struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data"`
}

type PageData struct {
	Total   int64 `json:"total"`
	Size    int   `json:"size"`
	Current int   `json:"current"`
	Records any   `json:"records"`
}

func Success(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Response{
		Code: 200,
		Msg:  "success",
		Data: data,
	})
}

func SuccessMsg(c *gin.Context, msg string, data any) {
	c.JSON(http.StatusOK, Response{
		Code: 200,
		Msg:  msg,
		Data: data,
	})
}

func Page(c *gin.Context, total int64, size, current int, records any) {
	Success(c, PageData{
		Total:   total,
		Size:    size,
		Current: current,
		Records: records,
	})
}

func Fail(c *gin.Context, code int, msg string) {
	c.JSON(code, Response{
		Code: code,
		Msg:  msg,
		Data: nil,
	})
}

func Unauthorized(c *gin.Context, msg string) {
	if msg == "" {
		msg = "未登录或 token 已过期"
	}
	Fail(c, http.StatusUnauthorized, msg)
	c.Abort()
}

func Forbidden(c *gin.Context, msg string) {
	if msg == "" {
		msg = "无权限访问"
	}
	Fail(c, http.StatusForbidden, msg)
	c.Abort()
}

func Error(c *gin.Context, msg string) {
	Fail(c, http.StatusInternalServerError, msg)
}
