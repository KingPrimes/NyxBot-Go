package rivenanalyse

import (
	"regexp"
	"strconv"
	"strings"
)

// hanRange 中文汉字范围（对齐 Java 正则的 [一-龥]，即 U+4E00-U+9FA5）。
const hanRange = `\x{4e00}-\x{9fa5}`

// 解析正则（逐条对齐 Java RivenMatcherUtil 的 Pattern 定义）。
var (
	// reChinese 提取连续中文片段（对齐 IS_CHINES）。
	reChinese = regexp.MustCompile(`[` + hanRange + `]*? ?·?&? ?[` + hanRange + `]`)
	// reWeaponsName 纯中文武器名（可含 & 与 ·）。
	reWeaponsName = regexp.MustCompile(`^[` + hanRange + `]*?&?·?[` + hanRange + `]*$`)
	// reWeaponsNameSuffix 中文武器名 + 可选英文后缀（如「守望者 Argi-」）。
	reWeaponsNameSuffix = regexp.MustCompile(`^[` + hanRange + `]*?&?·?[` + hanRange + `] *?[A-Za-z]*?-?[A-Za-z]*?$`)
	// reAttribute 属性行：含数值且以中文（可带括号/英文）结尾。
	// 必须全匹配（对齐 Java Matcher.matches()）——否则「用3,500来循环」这类
	// 界面提示会因子串（5 + 00 + 来循环）满足模式被误判为词条。
	reAttribute = regexp.MustCompile(`^(?:.[+-x]?\d+(\.\d+)?%?.?[` + hanRange + `]*?.?（?[a-zA-Z]*?.?[` + hanRange + `]+)$`)
	// reAttributeNum 数值提取（对齐 ATTRIBUTE_NUM）。
	reAttributeNum = regexp.MustCompile(`[+-]?\d+(\.\d+)?%?`)
	// reDiscrimination 歧视词条：x 开头的倍率格式（如「x1.06 对 Grineer 的伤害」）。
	reDiscrimination = regexp.MustCompile(`^x\d+(\.\d+)?%?.?[` + hanRange + `]*?.?（?[a-zA-Z]*?.?[` + hanRange + `]+$`)
	// reAttributeName 属性名提取（中文结尾部分，对齐 IS_ATTRIBUTE_NAME）。
	reAttributeName = regexp.MustCompile(`[` + hanRange + `]*?（?[a-zA-Z]*?[` + hanRange + `]+$`)
	// reRivenNameFull 紫卡名（中文前缀 + 空格 + 英文-英文，对齐 IS_RIVEN_NAME）。
	reRivenNameFull = regexp.MustCompile(`^[` + hanRange + `]*? [a-zA-Z]*-[a-zA-Z]*$`)
	// reRivenNameEx 紫卡名宽松形式（对齐 IS_RIVEN_NAME_EX）。
	reRivenNameEx = regexp.MustCompile(`^[` + hanRange + `]*? ?[a-zA-Z]*-?$`)
	// reRivenNamePart 提取紫卡名英文部分（对齐 RIVEN_NAME = [a-zA-Z]*-?$）。
	reRivenNamePart = regexp.MustCompile(`[a-zA-Z]*-?$`)
	// reRivenNamePattern 提取紫卡名后缀（对齐 RIVEN_NAME_PATTERN）。
	reRivenNamePattern = regexp.MustCompile(`[a-zA-Z]*-[a-zA-Z]*$`)

	// rePureNumber 整行属性数值（简化合并的兜底判定：如拆行后的「+6.5%」独立成行）。
	// 要求带符号（+/-）、x 倍率前缀或百分号结尾——避免把价格等无符号纯数字误判为拆行数值。
	rePureNumber = regexp.MustCompile(`^(?:[+-x]\d+(?:\.\d+)?%?|\d+(?:\.\d+)?%)$`)
)

// extractChinese 提取字符串中的中文片段并拼接（对齐 getChines）。
func extractChinese(s string) string {
	var sb strings.Builder
	for _, part := range reChinese.FindAllString(s, -1) {
		sb.WriteString(part)
	}
	return sb.String()
}

// isWeaponsName 判断是否为武器名称（纯中文 或 中文+英文后缀形式）。
func isWeaponsName(s string) bool {
	t := strings.TrimSpace(s)
	return reWeaponsNameSuffix.MatchString(t) || reWeaponsName.MatchString(t)
}

// isRivenName 紫卡名完整判定（中文前缀 + 英文-英文）。
func isRivenName(s string) bool {
	return reRivenNameFull.MatchString(s)
}

// isRivenNameEx 紫卡名宽松判定（对齐 isRivenNameEx：完整形式或宽松形式）。
func isRivenNameEx(s string) bool {
	return isRivenName(s) || reRivenNameEx.MatchString(s)
}

// rivenNameEnglish 提取紫卡名英文部分（对齐 getRivenNameE）。
func rivenNameEnglish(s string) string {
	var sb strings.Builder
	for _, part := range reRivenNamePart.FindAllString(s, -1) {
		sb.WriteString(part)
	}
	return sb.String()
}

// isAttribute 判断是否为属性词条行（含数值 + 中文）。
func isAttribute(s string) bool {
	return reAttribute.MatchString(s)
}

// attributeNum 提取属性数值，取最后一个匹配（对齐 getAttributeNum：
// 「x1.06 对 Grineer 的伤害」取 1.06；「+19.1% 暴击几率」取 19.1）。
func attributeNum(s string) float64 {
	matches := reAttributeNum.FindAllString(s, -1)
	if len(matches) == 0 {
		return 0
	}
	last := strings.TrimSpace(strings.TrimSuffix(matches[len(matches)-1], "%"))
	v, err := strconv.ParseFloat(last, 64)
	if err != nil {
		return 0
	}
	return v
}

// attributeName 提取属性名（对齐 getAttributeName：中文结尾部分）。
func attributeName(s string) string {
	var sb strings.Builder
	for _, part := range reAttributeName.FindAllString(s, -1) {
		sb.WriteString(part)
	}
	return strings.TrimSpace(sb.String())
}

// isDiscrimination 判断是否为歧视词条（x 倍率格式）。
func isDiscrimination(s string) bool {
	return reDiscrimination.MatchString(s)
}

// isPureNumberLine 判断整行是否为纯数值（简化合并的兜底判定）。
func isPureNumberLine(s string) bool {
	return rePureNumber.MatchString(strings.TrimSpace(s))
}

// isInvertedAttribute 判断是否为「值为负时是正属性」的特殊属性（如武器后坐力）。
// 规则：数值为正数 → 负属性；数值为负数 → 正属性。
func isInvertedAttribute(name string) bool {
	return strings.Contains(name, "后坐力")
}

// isNegativeAttribute 统一判定属性是否为负属性（含特殊属性反转逻辑，对齐同名方法）。
func isNegativeAttribute(name string, value float64) bool {
	if isInvertedAttribute(name) {
		return value > 0
	}
	return value < 0
}
