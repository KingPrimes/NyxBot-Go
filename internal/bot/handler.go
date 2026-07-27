package bot

import (
	"bytes"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"nyxbot-go/internal/database"
	"nyxbot-go/internal/enum/nyxbot"
	"nyxbot-go/internal/logging"
	modelbot "nyxbot-go/internal/model/bot"
	"nyxbot-go/internal/response"
)

const (
	messageInvalidParam   = "请求参数无效！"
	messageOperationFail  = "操作失败!"
	messageNoBot          = "请链接机器人后操作！\n注意：官方机器人无法获取好友与群列表。"
	messageBotOffline     = "此机器人未链接"
	messagePermissionBan  = "不可使用此权限！"
	messageSuperAdminOnly = "超级管理员已存在！且只能有一个。"
	messageBlackExists    = "已在黑名单中存在！"
	minQQUID              = int64(10000)
	maxQQUID              = int64(9999999999999)
)

// Handler 提供 Bot 在线信息、管理员和黑白名单管理接口。
type Handler struct {
	directory *Directory
}

// NewHandler 创建 Bot 配置管理 Handler。
func NewHandler(directory *Directory) *Handler {
	if directory == nil {
		directory = DefaultDirectory
	}
	return &Handler{directory: directory}
}

// Bots 返回当前已连接的 Bot 下拉选项。
func (h *Handler) Bots(c *gin.Context) {
	options := h.directory.Bots()
	if len(options) == 0 {
		response.Error(c, messageNoBot)
		return
	}
	response.Success(c, options)
}

// Friends 返回指定在线 Bot 的好友下拉选项。
func (h *Handler) Friends(c *gin.Context) {
	h.directoryOptions(c, true)
}

// Groups 返回指定在线 Bot 的群组下拉选项。
func (h *Handler) Groups(c *gin.Context) {
	h.directoryOptions(c, false)
}

func (h *Handler) directoryOptions(c *gin.Context, friends bool) {
	uid, err := strconv.ParseInt(c.Param("botUid"), 10, 64)
	if err != nil || uid <= 0 {
		response.Fail(c, http.StatusBadRequest, messageInvalidParam)
		return
	}
	if len(h.directory.Bots()) == 0 {
		response.Error(c, messageNoBot)
		return
	}

	var options []Option
	var online bool
	if friends {
		options, online = h.directory.Friends(uid)
	} else {
		options, online = h.directory.Groups(uid)
	}
	if !online {
		response.FailWithHTTP(c, http.StatusOK, http.StatusNoContent, messageBotOffline)
		return
	}
	response.Success(c, options)
}

// Permissions 返回可分配给 Bot 管理员的权限选项。
func (h *Handler) Permissions(c *gin.Context) {
	permissions := []nyxbot.PermissionsEnums{nyxbot.PermSuperAdmin, nyxbot.PermAdmin, nyxbot.PermUser}
	options := make([]Option, 0, len(permissions))
	for _, permission := range permissions {
		options = append(options, Option{Label: permission.Name(), Value: string(permission)})
	}
	response.Success(c, options)
}

// AdminList 分页返回 Bot 管理员，可按 botUid 等值过滤。
func (h *Handler) AdminList(c *gin.Context) {
	var request adminRequest
	if !bindJSON(c, &request) || !requireDatabase(c) {
		return
	}

	query := database.DB.Model(&modelbot.BotAdmin{})
	if request.BotUID.Set {
		query = query.Where("bot_uid = ?", request.BotUID.Value)
	}
	current, size := request.pageRequest.normalize()
	var total int64
	if err := query.Count(&total).Error; err != nil {
		databaseError(c, "list bot admins", err)
		return
	}
	rows := make([]modelbot.BotAdmin, 0)
	if err := query.Order("id ASC").Offset((current - 1) * size).Limit(size).Find(&rows).Error; err != nil {
		databaseError(c, "list bot admins", err)
		return
	}
	response.Page(c, total, size, current, rows)
}

// AdminSave 新增或更新 Bot 管理员。
func (h *Handler) AdminSave(c *gin.Context) {
	var request adminRequest
	if !bindJSON(c, &request) || !requireDatabase(c) {
		return
	}
	if !validQQ(request.BotUID) || !validQQ(request.AdminUID) {
		response.Fail(c, http.StatusBadRequest, "QQ号有错误!")
		return
	}
	permission := nyxbot.PermissionsEnums(request.Permissions)
	if permission != nyxbot.PermSuperAdmin && permission != nyxbot.PermAdmin && permission != nyxbot.PermUser {
		response.Error(c, messagePermissionBan)
		return
	}

	if permission == nyxbot.PermSuperAdmin {
		query := database.DB.Model(&modelbot.BotAdmin{}).
			Where("bot_uid = ? AND permissions = ?", request.BotUID.Value, string(nyxbot.PermSuperAdmin))
		if request.ID != 0 {
			query = query.Where("id <> ?", request.ID)
		}
		var count int64
		if err := query.Count(&count).Error; err != nil {
			databaseError(c, "validate super admin", err)
			return
		}
		if count > 0 {
			response.Error(c, messageSuperAdminOnly)
			return
		}
	}

	admin := modelbot.BotAdmin{
		ID:          request.ID,
		BotUID:      request.BotUID.Value,
		AdminUID:    request.AdminUID.Value,
		Permissions: request.Permissions,
	}
	if err := database.DB.Save(&admin).Error; err != nil {
		databaseError(c, "save bot admin", err)
		return
	}
	response.Success(c, nil)
}

// AdminRemove 删除指定 Bot 管理员。
func (h *Handler) AdminRemove(c *gin.Context) {
	h.remove(c, &modelbot.BotAdmin{})
}

// WhiteGroupList 分页返回群组白名单。
func (h *Handler) WhiteGroupList(c *gin.Context) {
	h.listGroupWhite(c)
}

// WhiteGroupSave 新增或更新群组白名单。
func (h *Handler) WhiteGroupSave(c *gin.Context) {
	var request groupRequest
	if !bindJSON(c, &request) || !requireDatabase(c) {
		return
	}
	if !validPositive(request.BotUID) || !validPositive(request.GroupUID) {
		response.Fail(c, http.StatusBadRequest, messageInvalidParam)
		return
	}
	blackExists, err := exists(&modelbot.GroupBlack{}, "group_uid = ?", request.GroupUID.Value)
	if err != nil {
		databaseError(c, "check group blacklist", err)
		return
	}
	if blackExists {
		response.Fail(c, http.StatusBadRequest, messageBlackExists)
		return
	}
	row := modelbot.GroupWhite{ID: request.ID, BotUID: request.BotUID.Value, GroupUID: request.GroupUID.Value}
	if err := saveByBusinessID(&row, &modelbot.GroupWhite{}, "group_uid = ?", request.GroupUID.Value); err != nil {
		databaseError(c, "save group whitelist", err)
		return
	}
	response.Success(c, nil)
}

// WhiteGroupRemove 删除指定群组白名单。
func (h *Handler) WhiteGroupRemove(c *gin.Context) {
	h.remove(c, &modelbot.GroupWhite{})
}

// WhiteProveList 分页返回个人白名单。
func (h *Handler) WhiteProveList(c *gin.Context) {
	h.listProveWhite(c)
}

// WhiteProveSave 新增或更新个人白名单。
func (h *Handler) WhiteProveSave(c *gin.Context) {
	var request proveRequest
	if !bindJSON(c, &request) || !requireDatabase(c) {
		return
	}
	if !validPositive(request.BotUID) || !validPositive(request.ProveUID) {
		response.Fail(c, http.StatusBadRequest, messageInvalidParam)
		return
	}
	blackExists, err := exists(&modelbot.ProveBlack{}, "prove_uid = ?", request.ProveUID.Value)
	if err != nil {
		databaseError(c, "check prove blacklist", err)
		return
	}
	if blackExists {
		response.Fail(c, http.StatusBadRequest, messageBlackExists)
		return
	}
	row := modelbot.ProveWhite{ID: request.ID, BotUID: request.BotUID.Value, ProveUID: request.ProveUID.Value}
	if err := saveByBusinessID(&row, &modelbot.ProveWhite{}, "prove_uid = ?", request.ProveUID.Value); err != nil {
		databaseError(c, "save prove whitelist", err)
		return
	}
	response.Success(c, nil)
}

// WhiteProveRemove 删除指定个人白名单。
func (h *Handler) WhiteProveRemove(c *gin.Context) {
	h.remove(c, &modelbot.ProveWhite{})
}

// BlackGroupList 分页返回群组黑名单。
func (h *Handler) BlackGroupList(c *gin.Context) {
	h.listGroupBlack(c)
}

// BlackGroupSave 新增或更新群组黑名单。
func (h *Handler) BlackGroupSave(c *gin.Context) {
	var request groupRequest
	if !bindJSON(c, &request) || !requireDatabase(c) {
		return
	}
	if !validPositive(request.BotUID) || !validPositive(request.GroupUID) {
		response.Fail(c, http.StatusBadRequest, messageInvalidParam)
		return
	}
	row := modelbot.GroupBlack{ID: request.ID, BotUID: request.BotUID.Value, GroupUID: request.GroupUID.Value}
	if err := saveByBusinessID(&row, &modelbot.GroupBlack{}, "group_uid = ?", request.GroupUID.Value); err != nil {
		databaseError(c, "save group blacklist", err)
		return
	}
	response.Success(c, nil)
}

// BlackGroupRemove 删除指定群组黑名单。
func (h *Handler) BlackGroupRemove(c *gin.Context) {
	h.remove(c, &modelbot.GroupBlack{})
}

// BlackProveList 分页返回个人黑名单。
func (h *Handler) BlackProveList(c *gin.Context) {
	h.listProveBlack(c)
}

// BlackProveSave 新增或更新个人黑名单。
func (h *Handler) BlackProveSave(c *gin.Context) {
	var request proveRequest
	if !bindJSON(c, &request) || !requireDatabase(c) {
		return
	}
	if !validPositive(request.BotUID) || !validPositive(request.ProveUID) {
		response.Fail(c, http.StatusBadRequest, messageInvalidParam)
		return
	}
	row := modelbot.ProveBlack{ID: request.ID, BotUID: request.BotUID.Value, ProveUID: request.ProveUID.Value}
	if err := saveByBusinessID(&row, &modelbot.ProveBlack{}, "prove_uid = ?", request.ProveUID.Value); err != nil {
		databaseError(c, "save prove blacklist", err)
		return
	}
	response.Success(c, nil)
}

// BlackProveRemove 删除指定个人黑名单。
func (h *Handler) BlackProveRemove(c *gin.Context) {
	h.remove(c, &modelbot.ProveBlack{})
}

func (h *Handler) listGroupWhite(c *gin.Context) {
	var request groupRequest
	if !bindJSON(c, &request) || !requireDatabase(c) {
		return
	}
	query := database.DB.Model(&modelbot.GroupWhite{})
	if request.GroupUID.Set {
		query = query.Where("group_uid = ?", request.GroupUID.Value)
	}
	current, size := request.pageRequest.normalize()
	rows := make([]modelbot.GroupWhite, 0)
	page(c, query, current, size, &rows)
}

func (h *Handler) listProveWhite(c *gin.Context) {
	var request proveRequest
	if !bindJSON(c, &request) || !requireDatabase(c) {
		return
	}
	query := database.DB.Model(&modelbot.ProveWhite{})
	if request.ProveUID.Set {
		query = query.Where("prove_uid = ?", request.ProveUID.Value)
	}
	current, size := request.pageRequest.normalize()
	rows := make([]modelbot.ProveWhite, 0)
	page(c, query, current, size, &rows)
}

func (h *Handler) listGroupBlack(c *gin.Context) {
	var request groupRequest
	if !bindJSON(c, &request) || !requireDatabase(c) {
		return
	}
	query := database.DB.Model(&modelbot.GroupBlack{})
	if request.GroupUID.Set {
		query = query.Where("group_uid = ?", request.GroupUID.Value)
	}
	current, size := request.pageRequest.normalize()
	rows := make([]modelbot.GroupBlack, 0)
	page(c, query, current, size, &rows)
}

func (h *Handler) listProveBlack(c *gin.Context) {
	var request proveRequest
	if !bindJSON(c, &request) || !requireDatabase(c) {
		return
	}
	query := database.DB.Model(&modelbot.ProveBlack{})
	if request.ProveUID.Set {
		query = query.Where("prove_uid = ?", request.ProveUID.Value)
	}
	current, size := request.pageRequest.normalize()
	rows := make([]modelbot.ProveBlack, 0)
	page(c, query, current, size, &rows)
}

func (h *Handler) remove(c *gin.Context, model any) {
	if !requireDatabase(c) {
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		response.Fail(c, http.StatusBadRequest, messageInvalidParam)
		return
	}
	if err := database.DB.Delete(model, uint(id)).Error; err != nil {
		databaseError(c, "remove bot config record", err)
		return
	}
	response.Success(c, nil)
}

type flexibleInt64 struct {
	Value int64
	Set   bool
}

func (number *flexibleInt64) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if bytes.Equal(data, []byte("null")) || bytes.Equal(data, []byte(`""`)) {
		return nil
	}
	text := string(data)
	if len(text) >= 2 && text[0] == '"' && text[len(text)-1] == '"' {
		unquoted, err := strconv.Unquote(text)
		if err != nil {
			return err
		}
		text = unquoted
	}
	value, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		return err
	}
	number.Value = value
	number.Set = true
	return nil
}

type pageRequest struct {
	Current int `json:"current"`
	Size    int `json:"size"`
}

func (request pageRequest) normalize() (int, int) {
	current, size := request.Current, request.Size
	if current <= 0 {
		current = 1
	}
	if size <= 0 {
		size = 15
	}
	return current, size
}

type adminRequest struct {
	pageRequest
	ID          uint          `json:"id"`
	BotUID      flexibleInt64 `json:"botUid"`
	AdminUID    flexibleInt64 `json:"adminUid"`
	Permissions string        `json:"permissions"`
}

type groupRequest struct {
	pageRequest
	ID       uint          `json:"id"`
	BotUID   flexibleInt64 `json:"botUid"`
	GroupUID flexibleInt64 `json:"groupUid"`
}

type proveRequest struct {
	pageRequest
	ID       uint          `json:"id"`
	BotUID   flexibleInt64 `json:"botUid"`
	ProveUID flexibleInt64 `json:"proveUid"`
}

func bindJSON(c *gin.Context, target any) bool {
	if err := c.ShouldBindJSON(target); err != nil {
		response.Fail(c, http.StatusBadRequest, messageInvalidParam)
		return false
	}
	return true
}

func requireDatabase(c *gin.Context) bool {
	if database.DB == nil {
		response.Error(c, messageOperationFail)
		return false
	}
	return true
}

func validQQ(value flexibleInt64) bool {
	return value.Set && value.Value >= minQQUID && value.Value <= maxQQUID
}

func validPositive(value flexibleInt64) bool {
	return value.Set && value.Value > 0
}

func page(c *gin.Context, query *gorm.DB, current, size int, rows any) {
	var total int64
	if err := query.Count(&total).Error; err != nil {
		databaseError(c, "count bot config records", err)
		return
	}
	if err := query.Order("id ASC").Offset((current - 1) * size).Limit(size).Find(rows).Error; err != nil {
		databaseError(c, "list bot config records", err)
		return
	}
	response.Page(c, total, size, current, rows)
}

func exists(model any, condition string, value any) (bool, error) {
	var count int64
	err := database.DB.Model(model).Where(condition, value).Count(&count).Error
	return count > 0, err
}

func saveByBusinessID(entity, model any, condition string, value any) error {
	var existing struct {
		ID uint
	}
	err := database.DB.Model(model).Select("id").Where(condition, value).Take(&existing).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if err == nil {
		switch row := entity.(type) {
		case *modelbot.GroupWhite:
			row.ID = existing.ID
		case *modelbot.ProveWhite:
			row.ID = existing.ID
		case *modelbot.GroupBlack:
			row.ID = existing.ID
		case *modelbot.ProveBlack:
			row.ID = existing.ID
		}
	}
	return database.DB.Save(entity).Error
}

func databaseError(c *gin.Context, operation string, err error) {
	logging.ErrorPack("bot.config", "%s: %v", operation, err)
	response.Error(c, messageOperationFail)
}
