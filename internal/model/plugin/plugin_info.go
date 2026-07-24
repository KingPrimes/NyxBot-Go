// 插件信息模型，对应 Java NyxBot 的 PluginInfo 实体
// 记录已安装/可用的绘图或功能插件的元数据
package plugin

import "time"

type PluginInfo struct {
	ID          uint      `gorm:"primaryKey"`                     // 主键
	PluginName  string    `gorm:"column:plugin_name;uniqueIndex"` // 插件唯一标识名
	DisplayName string    `gorm:"column:display_name"`            // 插件显示名
	Version     string    `gorm:"column:version"`                 // 版本号
	Description string    `gorm:"column:description;type:text"`   // 插件描述
	Author      string    `gorm:"column:author"`                  // 作者
	Type        string    `gorm:"column:type"`                    // 插件类型（jar / native）
	IconURL     string    `gorm:"column:icon_url;type:text"`      // 图标链接
	FilePath    string    `gorm:"column:file_path"`               // 文件路径
	FileSize    int64     `gorm:"column:file_size"`               // 文件大小（字节）
	DownloadURL string    `gorm:"column:download_url;type:text"`  // 下载链接
	Repository  string    `gorm:"column:repository;type:text"`    // 仓库地址
	License     string    `gorm:"column:license"`                 // 许可证
	Homepage    string    `gorm:"column:homepage;type:text"`      // 主页
	Enabled     bool      `gorm:"column:enabled"`                 // 是否启用
	Tags        string    `gorm:"column:tags"`                    // 标签（JSON 数组字符串）
	InstallAt   time.Time `gorm:"column:install_at;autoCreateTime"` // 安装时间
	UpdatedAt   time.Time `gorm:"column:updated_at;autoUpdateTime"` // 更新时间
}

func (PluginInfo) TableName() string {
	return "plugin_info"
}
