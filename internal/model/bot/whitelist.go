package bot

// GroupWhite 表示一个群组白名单条目。
type GroupWhite struct {
	ID       uint  `gorm:"primaryKey" json:"id"`                         // 主键
	BotUID   int64 `gorm:"column:bot_uid" json:"botUid"`                 // 机器人 QQ
	GroupUID int64 `gorm:"column:group_uid;uniqueIndex" json:"groupUid"` // 白名单群号（全局唯一）
}

// TableName 返回群组白名单表名。
func (GroupWhite) TableName() string {
	return "group_white"
}

// ProveWhite 表示一个个人白名单条目。
type ProveWhite struct {
	ID       uint  `gorm:"primaryKey" json:"id"`                         // 主键
	BotUID   int64 `gorm:"column:bot_uid" json:"botUid"`                 // 机器人 QQ
	ProveUID int64 `gorm:"column:prove_uid;uniqueIndex" json:"proveUid"` // 白名单用户 QQ（全局唯一）
}

// TableName 返回个人白名单表名。
func (ProveWhite) TableName() string {
	return "prove_white"
}
