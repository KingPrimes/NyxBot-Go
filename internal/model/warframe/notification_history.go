// 通知历史记录表，对应 Java NyxBot 的 NotificationHistory 实体
// 存储已推送的通知消息、时间、类型等历史数据
package warframe

import "time"

type NotificationHistory struct {
	ID      uint      `gorm:"primaryKey"`     // 主键
	Message string    `gorm:"column:message"` // 通知消息内容
	Time    time.Time `gorm:"column:time"`    // 通知时间
	IsPush  bool      `gorm:"column:is_push"` // 是否已推送
	Type    string    `gorm:"column:type"`    // 通知类型（如警报/入侵/活动等）
}

func (NotificationHistory) TableName() string {
	return "notification_history"
}
