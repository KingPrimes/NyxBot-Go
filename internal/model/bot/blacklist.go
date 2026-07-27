package bot

// GroupBlack 表示一个群组黑名单条目。
type GroupBlack struct {
	ID       uint  `gorm:"primaryKey" json:"id"`                         // 主键
	BotUID   int64 `gorm:"column:bot_uid" json:"botUid"`                 // 机器人 QQ
	GroupUID int64 `gorm:"column:group_uid;uniqueIndex" json:"groupUid"` // 黑名单群号（全局唯一）
}

// TableName 返回群组黑名单表名。
func (GroupBlack) TableName() string {
	return "group_black"
}

// ProveBlack 表示一个个人黑名单条目。
type ProveBlack struct {
	ID       uint  `gorm:"primaryKey" json:"id"`                         // 主键
	BotUID   int64 `gorm:"column:bot_uid" json:"botUid"`                 // 机器人 QQ
	ProveUID int64 `gorm:"column:prove_uid;uniqueIndex" json:"proveUid"` // 黑名单用户 QQ（全局唯一）
}

// TableName 返回个人黑名单表名。
func (ProveBlack) TableName() string {
	return "prove_black"
}
