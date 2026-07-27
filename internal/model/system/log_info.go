// 操作日志模型，对应 Java NyxBot 的 LogInfo 实体 + BaseEntity
// 记录 Bot 指令执行历史、用户行为、请求参数与结果
package system

import "time"

// LogInfo 操作日志记录。JSON 字段名与 WebUI 的 Api.SystemLog.LogInfo 逐字段对齐。
type LogInfo struct {
	ID            uint      `gorm:"primaryKey" json:"id"`                           // 主键
	Title         string    `gorm:"column:title" json:"title"`                      // 日志标题（枚举映射为字符串）
	Code          string    `gorm:"column:code" json:"code"`                        // 操作指令（如 /warframe cycle）
	Permissions   string    `gorm:"column:permissions" json:"permissions"`          // 操作用户等级
	BusinessType  string    `gorm:"column:business_type" json:"businessType"`       // 操作类型
	BotUID        int64     `gorm:"column:bot_uid" json:"botUid"`                   // 机器人 ID（QQ）
	GroupUID      int64     `gorm:"column:group_uid" json:"groupUid"`               // 请求来源群组
	UserUID       int64     `gorm:"column:user_uid" json:"userUid"`                 // 请求人 QQ
	RawMsg        string    `gorm:"column:raw_msg;type:text" json:"rawMsg"`         // 原始消息内容
	URL           string    `gorm:"column:url;type:text" json:"url"`                // 请求 URL
	Method        string    `gorm:"column:method" json:"method"`                    // HTTP 方法（POST/GET）
	RequestMethod string    `gorm:"column:request_method" json:"requestMethod"`     // 请求方法体
	RunTime       int64     `gorm:"column:run_time" json:"runTime"`                 // 执行耗时（毫秒）
	Param         string    `gorm:"column:param;type:text" json:"param"`            // 请求参数（JSON）
	Result        string    `gorm:"column:result;type:text" json:"result"`          // 返回结果（JSON）
	Status        int       `gorm:"column:status" json:"status"`                    // 执行状态（0=失败/1=成功）
	ErrorMsg      string    `gorm:"column:error_msg;type:longtext" json:"errorMsg"` // 错误信息
	LogTime       time.Time `gorm:"column:log_time" json:"logTime"`                 // 日志记录时间
}

// TableName 返回 GORM 表名。
func (LogInfo) TableName() string {
	return "log_info"
}
