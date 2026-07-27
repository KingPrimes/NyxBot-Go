// Package bot 定义 Bot 管理员、白名单和黑名单的持久化模型。
package bot

// BotAdmin 记录每个 Bot 下的管理员及其权限等级。
type BotAdmin struct {
	ID          uint   `gorm:"primaryKey" json:"id"`                                             // 主键
	BotUID      int64  `gorm:"column:bot_uid;uniqueIndex:uidx_bot_admin" json:"botUid"`          // 机器人 QQ
	AdminUID    int64  `gorm:"column:admin_uid;uniqueIndex:uidx_bot_admin" json:"adminUid"`      // 管理员 QQ
	Permissions string `gorm:"column:permissions;uniqueIndex:uidx_bot_admin" json:"permissions"` // 权限等级
}

// TableName 返回 Bot 管理员表名。
func (BotAdmin) TableName() string {
	return "bot_admin"
}
