// 任务订阅数据，对应 Java NyxBot 的 mission 包下三个实体：
//   MissionSubscribe - 订阅的任务条目（任务 ID、名称、类型、QQ 群）
//   MissionSubscribeUser - 订阅用户（用户 ID、名称）
//   MissionSubscribeUserCheckType - 用户的检查类型（如警报/入侵等）
package warframe

type MissionSubscribe struct {
	ID          uint   `gorm:"primaryKey"`                    // 主键
	MissionID   string `gorm:"column:missionid"`              // 任务 ID
	MissionName string `gorm:"column:mission_name"`           // 任务名称
	MissionType string `gorm:"column:mission_type"`           // 任务类型（如警报/刺杀/入侵）
	GroupName   string `gorm:"column:group_name"`             // QQ 群号
	Users       []MissionSubscribeUser `gorm:"foreignKey:SubscribeID"` // 订阅用户列表（一对多）
}

func (MissionSubscribe) TableName() string {
	return "mission_subscribe"
}

type MissionSubscribeUser struct {
	ID          uint   `gorm:"primaryKey"`                          // 主键
	SubscribeID uint   `gorm:"column:subscribe_id"`                 // 所属 MissionSubscribe 外键
	UserID      string `gorm:"column:user_id"`                      // 用户 ID（QQ 号）
	UserName    string `gorm:"column:user_name"`                    // 用户昵称
	CheckTypes  []MissionSubscribeUserCheckType `gorm:"foreignKey:SubscribeUserID"` // 检查类型列表（一对多）
}

func (MissionSubscribeUser) TableName() string {
	return "mission_subscribe_user"
}

type MissionSubscribeUserCheckType struct {
	ID              uint   `gorm:"primaryKey"`            // 主键
	SubscribeUserID uint   `gorm:"column:subscribe_user_id"` // 所属 MissionSubscribeUser 外键
	CheckType       string `gorm:"column:check_type"`      // 检查类型（如 spy/sabotage/excavation 等）
}

func (MissionSubscribeUserCheckType) TableName() string {
	return "mission_subscribe_user_check_type"
}
