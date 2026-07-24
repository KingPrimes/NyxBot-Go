// 系统用户模型，对应 Java NyxBot 的 SysUser 实体
// 存储 WebUI 登录账号的用户名、密码（BCrypt 加密）
package system

type SysUser struct {
	UserID   uint   `gorm:"primaryKey;column:user_id"` // 用户 ID
	UserName string `gorm:"column:user_name"`           // 用户名（登录用）
	Password string `gorm:"column:password"`            // BCrypt 加密后的密码
}

func (SysUser) TableName() string {
	return "sys_user"
}
