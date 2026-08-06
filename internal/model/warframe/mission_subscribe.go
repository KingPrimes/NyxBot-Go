// 任务订阅数据，对应 Java NyxBot 的 mission 包下三个实体：
//
//	MissionSubscribe - 订阅组（Bot + 群）
//	MissionSubscribeUser - 订阅用户
//	MissionSubscribeUserCheckType - 用户的检查类型（订阅规则）
package warframe

// MissionSubscribe 订阅组条目。sub_group 唯一。
// JSON 字段对齐前端 Api.LocalData.MissionSubscribe。
type MissionSubscribe struct {
	ID        uint   `gorm:"primaryKey" json:"id"`                // 主键（自增）
	GroupName string `gorm:"column:group_name" json:"groupName"`  // 群名称
	SubBotUID int64  `gorm:"column:sub_bot_uid" json:"subBotUid"` // Bot UID
	SubGroup  int64  `gorm:"column:sub_group" json:"subGroup"`    // 群号（unique，list 过滤字段）
}

func (MissionSubscribe) TableName() string {
	return "mission_subscribe"
}

// MissionSubscribeUser 订阅用户条目（sub_id + user_id 唯一）。
// JSON 字段对齐前端 Api.LocalData.MissionSubscribeUser。
type MissionSubscribeUser struct {
	ID       uint   `gorm:"primaryKey" json:"id"`             // 主键（自增）
	SubID    uint   `gorm:"column:sub_id" json:"-"`           // 所属 MissionSubscribe 外键（不输出）
	UserID   int64  `gorm:"column:user_id" json:"userId"`     // 用户 ID（QQ 号）
	UserName string `gorm:"column:user_name" json:"userName"` // 用户昵称
}

func (MissionSubscribeUser) TableName() string {
	return "mission_subscribe_user"
}

// MissionSubscribeUserCheckType 订阅检查类型条目（subu_id + subscribe + mission_type_enum + tier_num 唯一）。
// 枚举均按 STRING 存储（对齐 Java @Enumerated(STRING)）。
// JSON 字段对齐前端 Api.LocalData.MissionSubscribeUserCheckType。
type MissionSubscribeUserCheckType struct {
	ID              uint   `gorm:"primaryKey" json:"id"`                            // 主键（自增）
	SubuID          uint   `gorm:"column:subu_id" json:"-"`                         // 所属 MissionSubscribeUser 外键（不输出）
	Subscribe       string `gorm:"column:subscribe" json:"subscribe"`               // SubscribeType 枚举名（not null）
	MissionTypeEnum string `gorm:"column:mission_type_enum" json:"missionTypeEnum"` // MissionType 枚举名（可空）
	TierNum         int    `gorm:"column:tier_num" json:"tierNum"`                  // 遗物等级（可空）
	InvasionReward  string `gorm:"column:invasion_reward" json:"invasionReward"`    // InvasionReward 枚举名（可空，null=全部）
}

func (MissionSubscribeUserCheckType) TableName() string {
	return "mission_subscribe_user_check_type"
}
