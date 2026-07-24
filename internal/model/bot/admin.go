// Bot 管理员模型，对应 Java NyxBot 的 BotAdmin 实体 + BaseEntity
// 记录每个 Bot 下的管理员列表及其权限等级
package bot

type BotAdmin struct {
	ID          uint   `gorm:"primaryKey"`                             // 主键
	BotUID      int64  `gorm:"column:bot_uid"`                         // 机器人 QQ
	AdminUID    int64  `gorm:"column:admin_uid"`                       // 管理员 QQ
	Permissions string `gorm:"column:permissions"`                     // 权限等级（如 admin/super_admin）
}

func (BotAdmin) TableName() string {
	return "bot_admin"
}
