// Package rivenanalyse 紫卡截图分析链路：OCR 文本行 → 解析（武器名/紫卡名/属性词条）
// → 数值计算（等级估算、倾向缩放、低高区间）→ 综合分析（比率/评分/致命度）
// → 绘图输入（draw.RivenAnalyseTrend）。
//
// 对齐 Java NyxBot 的 RivenAnalysePlugin 链路：
// RivenOcrFilter（噪音过滤）+ RivenMatcherUtil（解析）+
// RivenAttributeCompute（计算）+ RivenWeaponAnalyzer（综合分析）。
package rivenanalyse
