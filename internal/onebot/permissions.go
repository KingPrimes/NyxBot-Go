package onebot

import (
	"errors"

	zero "github.com/wdvxdr1123/ZeroBot"
	"gorm.io/gorm"

	"nyxbot-go/internal/database"
	"nyxbot-go/internal/enum/nyxbot"
	"nyxbot-go/internal/logging"
	modelbot "nyxbot-go/internal/model/bot"
)

// AccessChecker 按黑白名单、艾特配置和数据库管理员权限过滤指令。
type AccessChecker struct {
	pluginPrefixProvider func() bool
}

// NewAccessChecker 创建消息访问检查器。
func NewAccessChecker(pluginPrefix bool) *AccessChecker {
	return NewAccessCheckerWithProvider(func() bool { return pluginPrefix })
}

// NewAccessCheckerWithProvider 创建动态读取艾特开关的消息访问检查器。
func NewAccessCheckerWithProvider(provider func() bool) *AccessChecker {
	if provider == nil {
		provider = func() bool { return false }
	}
	return &AccessChecker{pluginPrefixProvider: provider}
}

// Allows 判断事件能否执行所需权限等级的指令。
func (checker *AccessChecker) Allows(event *zero.Event, required nyxbot.PermissionsEnums) bool {
	if event == nil {
		return false
	}
	if checker.pluginPrefixProvider() && event.GroupID != 0 && !event.IsToMe {
		return false
	}
	allowed, err := listAllows(event.GroupID, event.UserID)
	if err != nil {
		logging.ErrorPack("onebot.permission", "check black/white lists failed: %v", err)
		return false
	}
	if !allowed {
		return false
	}

	switch required {
	case nyxbot.PermUser, nyxbot.PermOther:
		return true
	case nyxbot.PermAdmin:
		if event.Sender != nil && (event.Sender.Role == "owner" || event.Sender.Role == "admin") {
			return true
		}
		permission, err := databasePermission(event.SelfID, event.UserID)
		if err != nil {
			logging.ErrorPack("onebot.permission", "query administrator permission failed: %v", err)
			return false
		}
		return permission == nyxbot.PermAdmin || permission == nyxbot.PermSuperAdmin
	case nyxbot.PermSuperAdmin:
		permission, err := databasePermission(event.SelfID, event.UserID)
		if err != nil {
			logging.ErrorPack("onebot.permission", "query super administrator permission failed: %v", err)
			return false
		}
		return permission == nyxbot.PermSuperAdmin
	default:
		return false
	}
}

func (checker *AccessChecker) rule(required nyxbot.PermissionsEnums) zero.Rule {
	return func(ctx *zero.Ctx) bool {
		return ctx != nil && checker.Allows(ctx.Event, required)
	}
}

func listAllows(groupUID, userUID int64) (bool, error) {
	if database.DB == nil {
		return true, nil
	}
	whiteEnabled, err := anyListEntry(&modelbot.GroupWhite{}, &modelbot.ProveWhite{})
	if err != nil {
		return false, err
	}
	blackEnabled, err := anyListEntry(&modelbot.GroupBlack{}, &modelbot.ProveBlack{})
	if err != nil {
		return false, err
	}
	whiteMatch, err := matchesList(groupUID, userUID, &modelbot.GroupWhite{}, &modelbot.ProveWhite{})
	if err != nil {
		return false, err
	}
	blackMatch, err := matchesList(groupUID, userUID, &modelbot.GroupBlack{}, &modelbot.ProveBlack{})
	if err != nil {
		return false, err
	}

	switch {
	case whiteEnabled && blackEnabled:
		return whiteMatch || !blackMatch, nil
	case whiteEnabled:
		return whiteMatch, nil
	case blackEnabled:
		return !blackMatch, nil
	default:
		return true, nil
	}
}

func anyListEntry(groupModel, userModel any) (bool, error) {
	var groupCount, userCount int64
	if err := database.DB.Model(groupModel).Count(&groupCount).Error; err != nil {
		return false, err
	}
	if err := database.DB.Model(userModel).Count(&userCount).Error; err != nil {
		return false, err
	}
	return groupCount > 0 || userCount > 0, nil
}

func matchesList(groupUID, userUID int64, groupModel, userModel any) (bool, error) {
	if groupUID != 0 {
		var count int64
		if err := database.DB.Model(groupModel).Where("group_uid = ?", groupUID).Count(&count).Error; err != nil {
			return false, err
		}
		if count > 0 {
			return true, nil
		}
	}
	if userUID != 0 {
		var count int64
		if err := database.DB.Model(userModel).Where("prove_uid = ?", userUID).Count(&count).Error; err != nil {
			return false, err
		}
		return count > 0, nil
	}
	return false, nil
}

func databasePermission(botUID, userUID int64) (nyxbot.PermissionsEnums, error) {
	if database.DB == nil {
		return nyxbot.PermOther, errors.New("database is not initialized")
	}
	var admins []modelbot.BotAdmin
	err := database.DB.Where("bot_uid = ? AND admin_uid = ?", botUID, userUID).Find(&admins).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nyxbot.PermOther, err
	}
	best := nyxbot.PermOther
	for _, admin := range admins {
		permission := nyxbot.PermissionsEnums(admin.Permissions)
		if permission == nyxbot.PermSuperAdmin {
			return permission, nil
		}
		if permission == nyxbot.PermAdmin {
			best = permission
		}
	}
	return best, nil
}
