// Package version 编译期注入的版本信息，由 CI 的 -ldflags 设置。
// 本地运行时 Version="dev", Commit="unknown"。
package version

var (
	Version     = "dev"           // 语义化版本号
	ProductName = "NyxBot"        // 产品名称
	Description = "NyxBot Server" // 产品描述
	Commit      = "unknown"       // Git commit hash
)

// String 返回版本号字符串。
func String() string {
	return Version
}
