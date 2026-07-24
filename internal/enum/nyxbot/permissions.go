// NyxBot 权限 & 业务状态枚举
package nyxbot

// BusinessStatus 业务执行状态
type BusinessStatus string

const (
	BusinessStatusSuccess BusinessStatus = "SUCCESS"
	BusinessStatusFail    BusinessStatus = "FAIL"
)

// Type 返回业务状态的描述文字。
func (s BusinessStatus) Type() string {
	m := map[BusinessStatus]string{
		BusinessStatusSuccess: "成功",
		BusinessStatusFail:    "失败",
	}
	return m[s]
}

// PermissionsEnums 权限等级
type PermissionsEnums string

const (
	PermOther      PermissionsEnums = "OTHER"
	PermSuperAdmin PermissionsEnums = "SUPER_ADMIN"
	PermAdmin      PermissionsEnums = "ADMIN"
	PermUser       PermissionsEnums = "USER"
	PermManage     PermissionsEnums = "MANAGE"
)

// Name 返回权限等级的中文名称。
func (p PermissionsEnums) Name() string {
	m := map[PermissionsEnums]string{
		PermOther:      "其他用户",
		PermSuperAdmin: "超级管理员用户",
		PermAdmin:      "管理员用户",
		PermUser:       "普通用户",
		PermManage:     "后台用户",
	}
	return m[p]
}

// Level 返回权限等级的数值（越小权限越高）。
func (p PermissionsEnums) Level() int {
	m := map[PermissionsEnums]int{
		PermOther:      -1,
		PermSuperAdmin: 0,
		PermAdmin:      1,
		PermUser:       2,
		PermManage:     3,
	}
	return m[p]
}

// BusinessType 操作业务类型
type BusinessType string

const (
	BizOther  BusinessType = "OTHER"
	BizSelect BusinessType = "SELECT"
	BizInsert BusinessType = "INSERT"
	BizUpdate BusinessType = "UPDATE"
	BizDelete BusinessType = "DELETE"
	BizExport BusinessType = "EXPORT"
	BizImport BusinessType = "IMPORT"
	BizClean  BusinessType = "CLEAN"
	BizImage  BusinessType = "IMAGE"
	BizPlugin BusinessType = "PLUGIN"
)

// Type 返回业务类型的中文描述。
func (b BusinessType) Type() string {
	m := map[BusinessType]string{
		BizOther:  "其它",
		BizSelect: "查询",
		BizInsert: "新增",
		BizUpdate: "修改",
		BizDelete: "删除",
		BizExport: "导出",
		BizImport: "导入",
		BizClean:  "清空",
		BizImage:  "生成图片",
		BizPlugin: "插件",
	}
	return m[b]
}

// LogTitleEnum 日志标题类型
type LogTitleEnum string

const (
	LogTitleOther     LogTitleEnum = "OTHER"
	LogTitlePlugin    LogTitleEnum = "PLUGIN"
	LogTitleController LogTitleEnum = "CONTROLLER"
)

// Title 返回日志标题枚举的中文名。
func (l LogTitleEnum) Title() string {
	m := map[LogTitleEnum]string{
		LogTitleOther:     "其它",
		LogTitlePlugin:    "插件",
		LogTitleController: "控制器",
	}
	return m[l]
}
