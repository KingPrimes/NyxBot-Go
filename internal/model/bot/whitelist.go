// 白名单模型，对应 Java NyxBot 的 GroupWhite + ProveWhite 实体
// GroupWhite - 群组白名单，群内所有用户不受限制
// ProveWhite - 个人白名单，指定用户不受限制
package bot

type GroupWhite struct {
	ID      uint  `gorm:"primaryKey"`          // 主键
	BotUID  int64 `gorm:"column:bot_uid"`      // 机器人 QQ
	GroupUID int64 `gorm:"column:group_uid;uniqueIndex"` // 白名单群号（唯一）
}

func (GroupWhite) TableName() string {
	return "group_white"
}

type ProveWhite struct {
	ID       uint  `gorm:"primaryKey"`          // 主键
	BotUID   int64 `gorm:"column:bot_uid"`      // 机器人 QQ
	ProveUID int64 `gorm:"column:prove_uid;uniqueIndex"` // 白名单用户 QQ（唯一）
}

func (ProveWhite) TableName() string {
	return "prove_white"
}
