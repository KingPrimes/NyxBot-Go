// Package draw 提供 Warframe 各场景图片绘制能力（迁移自 draw-image-plugin 的 DefaultDraw* 系列），
// 覆盖仲裁、入侵、全信息、市场、裂隙、周期等 12 类图片，并提供通用 Canvas、字体与图标绘制基础。
// 本包仅依赖 internal/enum/drawplugin，不引入任何业务包；指令层负责将领域数据转换为本包定义的 DTO。
package draw
