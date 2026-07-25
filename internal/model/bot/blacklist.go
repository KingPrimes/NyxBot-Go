// 黑名单模型，对应 Java NyxBot 的 GroupBlack + ProveBlack 实体
// GroupBlack - 群组黑名单，屏蔽整个群
// ProveBlack - 个人黑名单，屏蔽指定用户
package bot

type GroupBlack struct {
	ID       uint  `gorm:"primaryKey"`                   // 主键
	BotUID   int64 `gorm:"column:bot_uid"`               // 机器人 QQ
	GroupUID int64 `gorm:"column:group_uid;uniqueIndex"` // 黑名单群号（唯一）
}

func (GroupBlack) TableName() string {
	return "group_black"
}

type ProveBlack struct {
	ID       uint  `gorm:"primaryKey"`                   // 主键
	BotUID   int64 `gorm:"column:bot_uid"`               // 机器人 QQ
	ProveUID int64 `gorm:"column:prove_uid;uniqueIndex"` // 黑名单用户 QQ（唯一）
}

func (ProveBlack) TableName() string {
	return "prove_black"
}
