// Package auth 实现基于 JWT 的用户认证体系：登录、令牌刷新、密码修改、登出。
// 包含登录频率限制（IP 级别 5 次/10 分钟）。
package auth

import (
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"nyxbot-go/internal/database"
	"nyxbot-go/internal/logging"
	"nyxbot-go/internal/model/system"
	"nyxbot-go/internal/response"
)

// ClaimsKey 是 Gin Context 中存储认证声明的键名。
const ClaimsKey = "authClaims"

type loginRequest struct {
	UserName string `json:"userName"`
	Password string `json:"password"`
}

type restorePasswordRequest struct {
	OldPassword     string `json:"oldPassword"`
	NewPassword     string `json:"newPassword"`
	ConfirmPassword string `json:"confirmPassword"`
}

type changeUsernameRequest struct {
	NewUsername string `json:"newUsername"`
	Password    string `json:"password"`
}

type loginAttempt struct {
	count   int
	firstAt time.Time
}

// Handler 认证请求处理器，注入 JWT Manager 并管理登录频率限制。
type Handler struct {
	jwt     *Manager
	rateMu  sync.Mutex
	rateMap map[string]*loginAttempt
}

// NewHandler 创建认证处理器实例。
func NewHandler(jwt *Manager) *Handler {
	return &Handler{
		jwt:     jwt,
		rateMap: make(map[string]*loginAttempt),
	}
}

// Login 用户登录，验证用户名密码后返回 access/refresh token 对。
func (h *Handler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 400, "请求参数错误")
		return
	}
	req.UserName = strings.TrimSpace(req.UserName)
	req.Password = strings.TrimSpace(req.Password)
	if req.UserName == "" || req.Password == "" {
		response.Fail(c, 400, "用户名和密码不能为空")
		return
	}

	clientIP := c.ClientIP()
	if !h.allowAttempt(clientIP) {
		response.Fail(c, 429, "登录尝试过于频繁，请10分钟后再试")
		return
	}

	var user system.SysUser
	if err := database.DB.Where("user_name = ?", req.UserName).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			response.Fail(c, 400, "用户名或密码错误")
			return
		}
		logging.ErrorPack("auth.login", "db error: %v", err)
		response.Error(c, "服务器内部错误")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		h.recordAttempt(clientIP)
		response.Fail(c, 400, "用户名或密码错误")
		return
	}

	pair, err := h.jwt.GeneratePair(user.UserID, user.UserName)
	if err != nil {
		logging.ErrorPack("auth.login", "generate token failed: %v", err)
		response.Error(c, "服务器内部错误")
		return
	}

	h.clearAttempts(clientIP)
	logging.InfoPack("auth.login", "user login: %s", user.UserName)
	response.Success(c, pair)
}

// GetUserInfo 获取当前登录用户信息（userId、userName、角色、权限）。
func (h *Handler) GetUserInfo(c *gin.Context) {
	claims := c.MustGet(ClaimsKey).(*Claims)

	var user system.SysUser
	if err := database.DB.First(&user, claims.UserID).Error; err != nil {
		response.Unauthorized(c, "用户不存在")
		return
	}

	response.Success(c, gin.H{
		"userId":   strconv.FormatUint(uint64(user.UserID), 10),
		"userName": user.UserName,
		"roles":    []string{"admin"},
		"buttons":  []string{"*"},
	})
}

// RefreshToken 使用 refresh token 刷新 access/refresh token 对。
func (h *Handler) RefreshToken(c *gin.Context) {
	claims := c.MustGet(ClaimsKey).(*Claims)

	pair, err := h.jwt.GeneratePair(claims.UserID, claims.UserName)
	if err != nil {
		logging.ErrorPack("auth.refresh", "generate token failed: %v", err)
		response.Error(c, "服务器内部错误")
		return
	}

	logging.DebugPack("auth.refresh", "token refreshed for user: %s", claims.UserName)
	response.Success(c, pair)
}

// RestorePassword 修改当前用户密码，需验证原密码。
func (h *Handler) RestorePassword(c *gin.Context) {
	claims := c.MustGet(ClaimsKey).(*Claims)

	var req restorePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 400, "请求参数错误")
		return
	}

	if req.NewPassword == "" || req.ConfirmPassword == "" {
		response.Fail(c, 400, "密码不能为空")
		return
	}
	if req.NewPassword != req.ConfirmPassword {
		response.Fail(c, 400, "两次输入的新密码不一致")
		return
	}

	var user system.SysUser
	if err := database.DB.First(&user, claims.UserID).Error; err != nil {
		response.Unauthorized(c, "用户不存在")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.OldPassword)); err != nil {
		response.Fail(c, 400, "原密码错误")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		logging.ErrorPack("auth.restore", "bcrypt error: %v", err)
		response.Error(c, "服务器内部错误")
		return
	}

	if err := database.DB.Model(&user).Update("password", string(hash)).Error; err != nil {
		logging.ErrorPack("auth.restore", "db error: %v", err)
		response.Error(c, "服务器内部错误")
		return
	}

	logging.InfoPack("auth.restore", "password changed for user: %s", user.UserName)
	response.SuccessMsg(c, "密码修改成功", nil)
}

// ChangeUsername 修改当前用户名，需验证密码并检查唯一性。
func (h *Handler) ChangeUsername(c *gin.Context) {
	claims := c.MustGet(ClaimsKey).(*Claims)

	var req changeUsernameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 400, "请求参数错误")
		return
	}
	req.NewUsername = strings.TrimSpace(req.NewUsername)
	if req.NewUsername == "" {
		response.Fail(c, 400, "用户名不能为空")
		return
	}

	var user system.SysUser
	if err := database.DB.First(&user, claims.UserID).Error; err != nil {
		response.Unauthorized(c, "用户不存在")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		response.Fail(c, 400, "密码错误")
		return
	}

	var dupCount int64
	database.DB.Model(&system.SysUser{}).Where("user_name = ? AND user_id != ?", req.NewUsername, user.UserID).Count(&dupCount)
	if dupCount > 0 {
		response.Fail(c, 400, "用户名已被使用")
		return
	}

	if err := database.DB.Model(&user).Update("user_name", req.NewUsername).Error; err != nil {
		logging.ErrorPack("auth.rename", "db error: %v", err)
		response.Error(c, "服务器内部错误")
		return
	}

	logging.InfoPack("auth.rename", "username changed: %s -> %s", user.UserName, req.NewUsername)
	response.SuccessMsg(c, "用户名修改成功", nil)
}

// Logout 登出，将当前 access token 加入黑名单。
func (h *Handler) Logout(c *gin.Context) {
	raw := BearerToken(c.GetHeader("Authorization"))
	if raw != "" {
		h.jwt.Blacklist(raw)
	}
	logging.InfoPack("auth.logout", "user logged out")
	response.SuccessMsg(c, "已退出登录", nil)
}

func (h *Handler) allowAttempt(key string) bool {
	h.rateMu.Lock()
	defer h.rateMu.Unlock()

	attempt, ok := h.rateMap[key]
	if !ok {
		return true
	}
	if time.Since(attempt.firstAt) > 10*time.Minute {
		delete(h.rateMap, key)
		return true
	}
	return attempt.count < 5
}

func (h *Handler) recordAttempt(key string) {
	h.rateMu.Lock()
	defer h.rateMu.Unlock()

	attempt, ok := h.rateMap[key]
	if !ok {
		h.rateMap[key] = &loginAttempt{count: 1, firstAt: time.Now()}
		return
	}
	if time.Since(attempt.firstAt) > 10*time.Minute {
		h.rateMap[key] = &loginAttempt{count: 1, firstAt: time.Now()}
		return
	}
	attempt.count++
}

func (h *Handler) clearAttempts(key string) {
	h.rateMu.Lock()
	defer h.rateMu.Unlock()
	delete(h.rateMap, key)
}
