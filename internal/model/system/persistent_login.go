// 持久化登录模型，对应 Java NyxBot 的 PersistentLogins 实体
// 用于 Remember-me 自动登录的 token 存储（Spring Security 兼容）
package system

import "time"

type PersistentLogin struct {
	Series   string    `gorm:"primaryKey;column:series;size:64"` // 系列标识
	Username string    `gorm:"column:username;size:64"`           // 用户名
	Token    string    `gorm:"column:token;size:64"`              // 登录 token
	LastUsed time.Time `gorm:"column:last_used"`                  // 最后使用时间
}

func (PersistentLogin) TableName() string {
	return "persistent_logins"
}
