// 仲裁相关的两个数据更新入口，挂 *DataImporter（main.go 注入的 executor 即该类型，
// 见 plans/arbitration-tier-plan.md §4.0.6）。指令与 /data/warframe/** 的 update 接口共用：
//   - UpdateArbitrationTiers：从 CDN 重拉节点分级并入库
//   - UpdateArbitration：强制重拉仲裁时间表（覆写本地文件 + 刷内存缓存）
package warframe

import "context"

// UpdateArbitrationTiers 重新拉取仲裁节点分级（纯 CDN）并入库。
func (importer *DataImporter) UpdateArbitrationTiers(ctx context.Context) error {
	return importer.ImportArbitrationTiers(ctx)
}

// UpdateArbitration 强制重拉仲裁时间表：拉取 → 覆写 ./data/arbys.txt → 刷新内存缓存。
// 不依赖 importer，仅为满足注入接口而挂在其上（与其它数据更新任务的落点保持一致）。
func (importer *DataImporter) UpdateArbitration(_ context.Context) error {
	return DefaultArbitration().Reload()
}
