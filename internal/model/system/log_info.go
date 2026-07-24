// 操作日志模型，对应 Java NyxBot 的 LogInfo 实体 + BaseEntity
// 记录 Bot 指令执行历史、用户行为、请求参数与结果
package system

import "time"

type LogInfo struct {
	ID            uint      `gorm:"primaryKey"`                    // 主键
	Title         string    `gorm:"column:title"`                  // 日志标题（枚举映射为字符串）
	Code          string    `gorm:"column:code"`                   // 操作指令（如 /warframe cycle）
	Permissions   string    `gorm:"column:permissions"`            // 操作用户等级
	BusinessType  string    `gorm:"column:business_type"`          // 操作类型
	BotUID        int64     `gorm:"column:bot_uid"`                // 机器人 ID（QQ）
	GroupUID      int64     `gorm:"column:group_uid"`              // 请求来源群组
	UserUID       int64     `gorm:"column:user_uid"`               // 请求人 QQ
	RawMsg        string    `gorm:"column:raw_msg;type:text"`      // 原始消息内容
	URL           string    `gorm:"column:url;type:text"`          // 请求 URL
	Method        string    `gorm:"column:method"`                 // HTTP 方法（POST/GET）
	RequestMethod string    `gorm:"column:request_method"`         // 请求方法体
	RunTime       int64     `gorm:"column:run_time"`               // 执行耗时（毫秒）
	Param         string    `gorm:"column:param;type:text"`        // 请求参数（JSON）
	Result        string    `gorm:"column:result;type:text"`       // 返回结果（JSON）
	Status        int       `gorm:"column:status"`                 // 执行状态（0=失败/1=成功）
	ErrorMsg      string    `gorm:"column:error_msg;type:longtext"` // 错误信息
	LogTime       time.Time `gorm:"column:log_time"`               // 日志记录时间
}

func (LogInfo) TableName() string {
	return "log_info"
}
