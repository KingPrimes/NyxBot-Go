package rivenanalyse

import (
	"fmt"
	"math"
	"slices"
	"sort"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"nyxbot-go/internal/draw"
	"nyxbot-go/internal/logging"
	modelwarframe "nyxbot-go/internal/model/warframe"
)

// Calculator 紫卡分析器（持有数据库连接，对齐 Java RivenAttributeCompute）。
type Calculator struct {
	db *gorm.DB
}

// NewCalculator 创建紫卡分析器。
func NewCalculator(db *gorm.DB) *Calculator {
	return &Calculator{db: db}
}

// parsedAttribute 解析出的单个属性词条（对齐 RivenAnalyseTrendCompute.Attribute）。
type parsedAttribute struct {
	name          string  // 分析名（用于查 trend 表）
	attributeName string  // 显示名（含「效果加倍」「重击时 x2」等特殊后缀）
	attr          float64 // OCR 原始数值
	nag           bool    // 整卡是否携带负属性（供修正系数使用）
}

// parsedRiven 单张图的解析结果（对齐 RivenAnalyseTrendCompute，武器名改为候选列表）。
type parsedRiven struct {
	// weaponsCandidates 候选武器名（按行序去重）。截图底部的星级乱码可能也是纯中文行，
	// Java 原逻辑（后写覆盖）会导致武器名被乱码覆盖而分析失败；这里保留全部候选，
	// 计算阶段取「首个能查到武器的候选」，乱码候选自然落空跳过。
	weaponsCandidates []string
	rivenName         string
	attributes        []parsedAttribute
}

// Analyse 分析单张截图的 OCR 文本行，返回每个匹配武器一张结果卡
// （对齐 Java ocrRivenCompute 的单图流程 + setAttributeNumber）。
func (c *Calculator) Analyse(rawLines []string) []*draw.RivenAnalyseTrend {
	lines := cleanLines(rawLines)
	logging.DebugPack("riven", "OCR 清洗：%d 行 → %d 行：%v", len(rawLines), len(lines), lines)
	if len(lines) == 0 {
		return nil
	}
	p := parseRiven(lines)
	logging.DebugPack("riven", "解析结果：武器候选=%v 紫卡名=%q 词条数=%d",
		p.weaponsCandidates, p.rivenName, len(p.attributes))
	for i, a := range p.attributes {
		logging.DebugPack("riven", "词条[%d]：显示名=%q 分析名=%q 数值=%.2f 整卡带负=%v",
			i, a.attributeName, a.name, a.attr, a.nag)
	}
	if len(p.weaponsCandidates) == 0 {
		return nil
	}
	return c.computeAttributes(p)
}

// parseRiven 从清洗后的 OCR 行解析武器名候选、紫卡名与属性词条（对齐 Java getRiven）。
func parseRiven(lines []string) parsedRiven {
	var p parsedRiven
	var attrTexts []string

	for _, line := range lines {
		if len([]rune(line)) < 3 {
			continue
		}
		if isWeaponsName(line) {
			// & 双武器名：& 前非空格时插入空格（对齐 Java "&" 处理）
			if idx := strings.Index(line, "&"); idx > 0 && line[idx-1:idx] != " " {
				line = strings.ReplaceAll(line, "&", " & ")
			}
			if cand := extractChinese(line); cand != "" && !slices.Contains(p.weaponsCandidates, cand) {
				p.weaponsCandidates = append(p.weaponsCandidates, cand)
			}
			if isRivenNameEx(line) {
				if name := rivenNameEnglish(line); name != "" {
					p.rivenName = name
				}
			}
		}
		if isRivenNameEx(line) {
			if name := rivenNameEnglish(line); name != "" {
				if p.rivenName == "" {
					p.rivenName = name
				} else if p.rivenName != name {
					// OCR 把紫卡名拆成多行（如 "Argi-" + "fevacron"）时拼接
					p.rivenName += name
				}
			}
		}
		if isAttribute(line) {
			cleaned := strings.ReplaceAll(strings.TrimSpace(line), " ", "")
			attrTexts = append(attrTexts, strings.ReplaceAll(cleaned, "入", ""))
		}
	}

	// 整卡负属性判定：≥3 条词条且任一条为负数值/歧视词条/反向负属性（对齐 nag 计算）
	nag := false
	if len(attrTexts) >= 3 {
		for _, t := range attrTexts {
			if attributeNum(t) < 0 || isDiscrimination(t) ||
				isNegativeAttribute(attributeName(t), attributeNum(t)) {
				nag = true
				break
			}
		}
	}
	for _, t := range attrTexts {
		p.attributes = append(p.attributes, buildParsedAttribute(t, nag))
	}
	return p
}

// buildParsedAttribute 构建属性词条并按需重写特殊显示名（对齐 getRiven 的属性构建分支）。
func buildParsedAttribute(text string, nag bool) parsedAttribute {
	a := parsedAttribute{attr: attributeNum(text), nag: nag}
	switch {
	case strings.Contains(text, "射速"):
		a.attributeName = text + " 效果加倍）"
		a.name = attributeName(text) + " 效果加倍）"
	case strings.Contains(text, "暴击几率（") || strings.Contains(text, "重击"):
		head := text
		if idx := strings.Index(text, "率"); idx >= 0 {
			head = text[:idx+len("率")]
		}
		a.attributeName = head + "(重击时 x2)"
		a.name = "暴击几率（重击时 x2）"
	case strings.Contains(text, "滑行攻击"):
		// 对齐 Java s.replaceAll("滑.*+", "滑行攻击暴击几率")：仅替换「滑」之后的部分，
		// 保留其前段的数值前缀（如 "-13.2%"）
		if idx := strings.Index(text, "滑"); idx > 0 {
			a.attributeName = text[:idx] + "滑行攻击暴击几率"
		} else {
			a.attributeName = "滑行攻击暴击几率"
		}
		a.name = "滑行攻击暴击几率"
	default:
		a.attributeName = text
		a.name = attributeName(text)
	}
	return a
}

// computeAttributes 逐武器计算满级等效值与低/高区间（对齐 Java setAttributeNumber）。
// 武器名取首个能查到武器的候选（乱码候选自动跳过，见 parsedRiven.weaponsCandidates）。
func (c *Calculator) computeAttributes(p parsedRiven) []*draw.RivenAnalyseTrend {
	var weapons []modelwarframe.Weapons
	for _, cand := range p.weaponsCandidates {
		if weapons = c.findWeapons(cand); len(weapons) > 0 {
			logging.DebugPack("riven", "武器候选 %q 命中 %d 条", cand, len(weapons))
			break
		}
		logging.DebugPack("riven", "武器候选 %q 未命中，继续下一个候选", cand)
	}
	if len(weapons) == 0 {
		return nil
	}

	sourceDisp := weapons[0].OmegaAttenuation
	rankScale := c.estimateRankScale(p, &weapons[0])
	logging.DebugPack("riven", "倾向基准：来源武器=%q sourceDisp=%.2f rankScale=%.4f",
		weapons[0].Name, sourceDisp, rankScale)

	models := make([]*draw.RivenAnalyseTrend, 0, len(weapons))
	for i := range weapons {
		weapon := &weapons[i]
		omega := weapon.OmegaAttenuation
		dispScale := 1.0
		if sourceDisp > 0 {
			dispScale = omega / sourceDisp
		}
		maxEquivScale := rankScale * dispScale

		cat := effectiveCategory(weapon)
		num := omega
		model := &draw.RivenAnalyseTrend{
			WeaponName: weapon.Name,
			RivenName:  p.rivenName,
			Num:        &num,
			WeaponType: weaponCategoryNames[cat],
		}

		for _, attr := range p.attributes {
			trend := c.findTrendByAnalyseName(attr.name)
			if trend == nil {
				logging.DebugPack("riven", "词条 %q 未命中趋势表（分析名=%q），低高区间将显示为 ?", attr.attributeName, attr.name)
			}

			// 满级等效值（对齐：歧视词条按百分比线性缩放，显示值非线性）
			scaled := attr.attr * maxEquivScale
			if isDiscrimination(attr.attributeName) {
				ocr := attr.attr
				pct := ocr*100 - 100
				if ocr > 1 {
					pct = (ocr - 1) * 100
				}
				scaled = 1 + pct*maxEquivScale/100
			}

			m := draw.RivenAnalyseTrendAttribute{
				Name:          attr.name,
				AttributeName: attr.attributeName,
				Attr:          &scaled,
			}

			// 低/高区间：0.9/1.1 × 基准值 × 倾向 × 修正系数（对齐 getLowAttribute/getHighAttribute）
			if baseVal, ok := trendValue(trend, cat); ok {
				isNeg := isNegativeAttribute(attr.attributeName, attr.attr)
				low := lowHighValueWithFactor(baseVal, omega, len(p.attributes), attr.nag, isNeg, 0.9)
				high := lowHighValueWithFactor(baseVal, omega, len(p.attributes), attr.nag, isNeg, 1.1)
				// 负属性（修正系数为负）时 0.9/1.1 系数的结果天然反序（0.9 反而数值更大），
				// 统一按数值从小到大展示区间
				if low > high {
					low, high = high, low
				}
				m.LowAttr = formatAttrBound(low, isDiscrimination(attr.attributeName))
				m.HighAttr = formatAttrBound(high, isDiscrimination(attr.attributeName))
				m.AttrDiff = buildAttrDiff(attr.name, scaled, low, high, isNeg)
				logging.DebugPack("riven", "词条 %q：基准=%.2f 满级等效=%.2f 区间=[%s, %s] 偏差=%s",
					attr.attributeName, baseVal, scaled, m.LowAttr, m.HighAttr, m.AttrDiff)
			} else if trend != nil {
				logging.DebugPack("riven", "词条 %q：类别 %q 无基准值（趋势表该列为 0）",
					attr.attributeName, weaponCategoryNames[cat])
			}

			model.Attributes = append(model.Attributes, m)
		}

		analyzeWeapon(c, weapon, model)
		models = append(models, model)
	}
	return models
}

// estimateRankScale 通过 OCR 值与预期满级中位数的比值估算紫卡当前等级，
// 返回缩放至满级 (rank 8) 的倍率（对齐 Java estimateRankScale：取中位数、过滤单条 OCR 偏差）。
func (c *Calculator) estimateRankScale(p parsedRiven, source *modelwarframe.Weapons) float64 {
	sourceDisp := source.OmegaAttenuation
	cat := effectiveCategory(source)
	n := len(p.attributes)
	nag := false
	for _, a := range p.attributes {
		if a.nag {
			nag = true
			break
		}
	}

	var ratios []float64
	for _, attr := range p.attributes {
		trend := c.findTrendByAnalyseName(attr.name)
		baseVal, ok := trendValue(trend, cat)
		if !ok || baseVal == 0 {
			continue
		}
		isNeg := isNegativeAttribute(attr.attributeName, attr.attr)
		expected := math.Abs(baseVal) * sourceDisp * math.Abs(correctionFactor(n, nag, isNeg))
		if expected <= 0 {
			continue
		}
		// 歧视词条 OCR 值为 x1.53 格式，转为百分比后再比较
		val := math.Abs(attr.attr)
		if isDiscrimination(attr.attributeName) {
			if val > 1 {
				val = (val - 1) * 100
			} else {
				val = 100 - val*100
			}
		}
		ratios = append(ratios, val/expected)
	}

	if len(ratios) == 0 {
		return 1.0
	}
	sort.Float64s(ratios)
	var median float64
	if len(ratios)%2 == 0 {
		median = (ratios[len(ratios)/2-1] + ratios[len(ratios)/2]) / 2
	} else {
		median = ratios[len(ratios)/2]
	}
	scale := math.Min(math.Max(1.0/median, 0.9), 9.0)
	logging.DebugPack("riven", "等级估算：ratios=%v median=%.4f → rankScale=%.4f", ratios, median, scale)
	if scale > 2.5 {
		// 对齐 Java 的告警；低等级紫卡（0-1 级）本身可达 8-9 倍，属正常范围
		logging.WarnPack("riven", "紫卡缩放倍率偏大（rankScale=%.4f），请确认截图等级或 OCR 数据是否清晰", scale)
	}
	return scale
}

// correctionFactor 词条修正系数（对齐 Java correctionFactor；不含 grade 浮动因子 0.9~1.1）。
func correctionFactor(totalAttrs int, hasNegative, isNegativeAttr bool) float64 {
	switch totalAttrs {
	case 2:
		if isNegativeAttr {
			return 0
		}
		return 0.99
	case 3:
		if hasNegative {
			if isNegativeAttr {
				return -0.495
			}
			return 1.2375
		}
		if isNegativeAttr {
			return 0
		}
		return 0.75
	case 4:
		if isNegativeAttr {
			return -0.75
		}
		return 0.9375
	default:
		return 1.0
	}
}

// lowHighValueWithFactor 低/高值 = 浮动系数 × 基准值 × 倾向 × 修正系数（2 位小数）。
// 词条数不在 2-4 范围内时返回 0（对齐 Java getLowAttribute/getHighAttribute 的 switch 行为）。
func lowHighValueWithFactor(baseVal, pro float64, count int, nag, isNeg bool, factor float64) float64 {
	switch count {
	case 2, 3, 4:
		return round2(factor * baseVal * pro * correctionFactor(count, nag, isNeg))
	default:
		return 0
	}
}

// trendValue 按武器类别取趋势基准值（对齐 getTrendValue）；
// 显赫武器/星舰武器无紫卡基准，返回 false。
func trendValue(trend *modelwarframe.RivenAnalyseTrend, cat int) (float64, bool) {
	if trend == nil {
		return 0, false
	}
	switch cat {
	case catLongGuns, catSentinelWeapons:
		return trend.Rifle, true
	case catShotguns:
		return trend.Shotgun, true
	case catPistols:
		return trend.Pistol, true
	case catMelee:
		return trend.Melle, true
	case catSpaceGuns, catSpaceMelee:
		return trend.Archwing, true
	default:
		return 0, false
	}
}

// isFactionAttribute 判断是否为派系/歧视词条（对齐 Java attrDiff 的分支条件）。
func isFactionAttribute(name string) bool {
	return strings.Contains(name, "Infested") || strings.Contains(name, "Corpus") ||
		strings.Contains(name, "Grinner") || strings.Contains(name, "Grineer")
}

// buildAttrDiff 生成属性偏差文本（对齐 Java attrDiff + getString/getAttributeDiscriminationDiff）。
// isNegative 为真时按「更强负 = 负向（-）」呈现——Java 原语义只看绝对值偏离方向，
// 会使「负得更多」显示为 +（正向）；这里对负属性翻转方向，使颜色（+绿/-红）符合直觉。
func buildAttrDiff(name string, attr, low, high float64, isNegative bool) string {
	median := (low + high) / 2
	if median == 0 {
		return ""
	}
	abs := math.Abs(attr)
	if isFactionAttribute(name) {
		// 歧视词条按百分比形态参与偏差计算（x1.53 → 53）
		if abs > 1 {
			abs = (abs - 1) * 100
		} else {
			abs = 100 - abs*100
		}
	}
	diff := abs - math.Abs(median)
	if isNegative {
		diff = -diff
	}
	percent := round2(((math.Abs(median) - abs) / math.Abs(median)) * 100)
	switch {
	case diff > 0:
		return "+" + formatNum(math.Abs(percent)) + "%"
	case diff == 0:
		return "0.0%"
	default:
		return "-" + formatNum(math.Abs(percent)) + "%"
	}
}

// formatAttrBound 格式化低/高区间显示值：歧视词条保留 1 位小数，其余完整输出。
func formatAttrBound(v float64, discrimination bool) string {
	if discrimination {
		return fmt.Sprintf("%.1f", v)
	}
	return formatNum(v)
}

// formatNum 数值转字符串（对齐 Java String.valueOf(double) 的展示习惯）。
func formatNum(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// round2 四舍五入到 2 位小数（计算结果统一保留两位小数）。
func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
