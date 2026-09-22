// 阶段10 管理类指令测试：权限元数据、执行器接口契约、命令挂载存在性。
// handler 的异步群消息回发依赖 ZeroBot 真实运行（黑线），此处覆盖可静态验证的部分。
package tests

import (
	"testing"

	"nyxbot-go/internal/enum/nyxbot"
	"nyxbot-go/internal/onebot"
	"nyxbot-go/internal/warframe"
)

// TestDataImporterImplementsDataUpdateExecutor 编译期断言：DataImporter 满足管理更新接口。
func TestDataImporterImplementsDataUpdateExecutor(t *testing.T) {
	var _ onebot.DataUpdateExecutor = (*warframe.DataImporter)(nil)
	var executor onebot.DataUpdateExecutor = (*warframe.DataImporter)(nil)
	if executor == nil {
		t.Fatal("接口断言不应为 nil")
	}
}

// TestAdminCommandPermissions 管理类指令必须为 SUPER_ADMIN 且命令元数据存在。
func TestAdminCommandPermissions(t *testing.T) {
	adminCodes := []nyxbot.Codes{
		nyxbot.CmdUpdateWfResMarketItems,
		nyxbot.CmdUpdateWfResMarketRiven,
		nyxbot.CmdUpdateWfSister,
		nyxbot.CmdUpdateWfTar,
	}
	for _, code := range adminCodes {
		info, ok := nyxbot.CodesInfo[code]
		if !ok {
			t.Fatalf("%s 缺少命令元数据", code)
		}
		if info.Permissions != nyxbot.PermSuperAdmin {
			t.Fatalf("%s 权限应为超级管理员, 实际 %v", code, info.Permissions)
		}
	}
}
