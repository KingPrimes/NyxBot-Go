package rivenanalyse

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"nyxbot-go/internal/draw"
	"nyxbot-go/internal/logging"
	modelwarframe "nyxbot-go/internal/model/warframe"
)

// 综合评分阈值（对齐 Java RivenWeaponAnalyzer 常量）。
const (
	lowCrit         = 0.15 // 低暴击阈值（低于此值暴击词条价值有限）
	highCrit        = 0.18 // 高暴击阈值（高于此值暴击词条价值显著）
	critFatal       = 0.25 // 暴击致命阈值
	highProc        = 0.20 // 高触发阈值
	procFatal       = 0.25 // 触发致命阈值（霰弹枪）
	lowCritFallback = 0.10 // 霰弹枪无暴击时基伤价值更高
	highFireRate    = 5.0  // 高射速武器阈值（发/秒）
	lowMagFatal     = 10   // 低弹匣致命阈值
)

// analyzeWeapon 结合武器基础数据，对紫卡全部词条进行综合分析，
// 写入比率（ratio）、评分（grade）、致命度（lethalLevel）与评语（analysis）。
// 对齐 Java RivenWeaponAnalyzer.analyze。
func analyzeWeapon(_ *Calculator, weapon *modelwarframe.Weapons, model *draw.RivenAnalyseTrend) {
	if weapon == nil || model == nil {
		return
	}
	cat := effectiveCategory(weapon)
	baseCrit := weapon.CriticalChance
	baseProc := weapon.ProcChance
	sniper := isSniper(weapon)

	for i := range model.Attributes {
		attr := &model.Attributes[i]
		var attrVal float64
		if attr.Attr != nil {
			attrVal = *attr.Attr
		}
		isNegative := isNegativeAttribute(attr.Name, attrVal)

		attr.Ratio = calcRatio(weapon, attr.Name, math.Abs(attrVal))
		attr.Grade = calcGrade(cat, attr.Name, baseCrit, baseProc, sniper)
		if isNegative {
			attr.LethalLevel = calcLethalLevel(cat, attr.Name, weapon)
		}
		attr.Analysis = buildAnalysis(attr)
		logging.DebugPack("riven", "武器 %q 词条 %q 分析：比率=%s 评分=%s 致命度=%s",
			weapon.Name, attr.Name, attr.Ratio, attr.Grade, attr.LethalLevel)
	}
}

// isSniper 狙击枪判定（对齐 Java：描述包含「狙击」）。
func isSniper(weapon *modelwarframe.Weapons) bool {
	return strings.Contains(weapon.Description, "狙击")
}

// calcRatio 计算紫卡词条值相对武器基值的倍率（对齐 Java calcRatio；基值无效输出 "-"）。
func calcRatio(weapon *modelwarframe.Weapons, trendName string, rivenValue float64) string {
	var base float64
	switch trendName {
	case "暴击几率":
		base = weapon.CriticalChance * 100
	case "暴击伤害":
		base = (weapon.CriticalMultiplier - 1) * 100
	case "触发几率":
		base = weapon.ProcChance * 100
	case "射速/攻击速度":
		base = weapon.FireRate * 100
	case "伤害/近战伤害":
		base = float64(weapon.TotalDamage)
	case "弹匣容量":
		base = float64(weapon.MagazineSize) * 100
	case "装填速度":
		base = weapon.ReloadTime * 100
	}
	if base <= 0 {
		return "-"
	}
	return fmt.Sprintf("%.2f", rivenValue/base)
}

// calcGrade 按武器类别分发词条评分（对齐 Java calcGrade）。
func calcGrade(cat int, trendName string, baseCrit, baseProc float64, sniper bool) string {
	switch cat {
	case catShotguns:
		return shotgunGrade(trendName, baseCrit, baseProc)
	case catPistols:
		return pistolGrade(trendName, baseCrit)
	case catMelee:
		return meleeGrade(trendName, baseCrit)
	case catLongGuns:
		return rifleGrade(trendName, baseCrit, sniper)
	default:
		return "C"
	}
}

// shotgunGrade 霰弹枪词条评分（对齐 Java shotgunGrade）。
func shotgunGrade(name string, baseCrit, baseProc float64) string {
	switch name {
	case "伤害/近战伤害":
		if baseCrit > lowCritFallback {
			return "B"
		}
		return "A"
	case "多重射击":
		return "S"
	case "暴击几率", "暴击伤害":
		if baseCrit > lowCrit {
			return "A"
		}
		return "C"
	case "触发几率", "切割伤害":
		if baseProc > highProc {
			return "A"
		}
		return "B"
	case "射速/攻击速度":
		return "B"
	case "冰冻伤害", "毒素伤害", "电击伤害", "火焰伤害":
		if baseCrit > lowCrit {
			return "B"
		}
		return "A"
	case "穿刺伤害", "冲击伤害", "装填速度", "投射物飞行速度":
		return "C"
	default:
		return "-"
	}
}

// pistolGrade 手枪词条评分（对齐 Java pistolGrade）。
func pistolGrade(name string, baseCrit float64) string {
	switch name {
	case "伤害/近战伤害":
		return "S"
	case "多重射击":
		if baseCrit > lowCrit {
			return "B"
		}
		return "A"
	case "暴击几率", "暴击伤害":
		if baseCrit > highCrit {
			return "A"
		}
		return "B"
	case "触发几率", "冰冻伤害", "毒素伤害", "电击伤害", "穿刺伤害", "冲击伤害", "切割伤害":
		return "B"
	case "射速/攻击速度", "装填速度":
		return "C"
	case "火焰伤害":
		if baseCrit > highCrit {
			return "B"
		}
		return "A"
	default:
		return "-"
	}
}

// meleeGrade 近战词条评分（对齐 Java meleeGrade）。
func meleeGrade(name string, baseCrit float64) string {
	switch name {
	case "伤害/近战伤害":
		return "B"
	case "暴击几率":
		return "C"
	case "暴击伤害":
		return "S"
	case "触发几率", "射速/攻击速度":
		return "B"
	case "攻击范围", "初始连击", "切割伤害":
		return "A"
	case "连击持续时间":
		if baseCrit > lowCrit {
			return "A"
		}
		return "B"
	case "冰冻伤害", "毒素伤害", "电击伤害", "火焰伤害":
		if baseCrit > highCrit {
			return "B"
		}
		return "A"
	case "处决伤害", "重击效率", "冲击伤害", "穿刺伤害":
		return "C"
	case "滑行攻击暴击几率":
		return "D"
	default:
		return "-"
	}
}

// rifleGrade 步枪词条评分（对齐 Java rifleGrade）。
func rifleGrade(name string, baseCrit float64, sniper bool) string {
	switch name {
	case "伤害/近战伤害", "触发几率", "冰冻伤害", "毒素伤害", "电击伤害", "火焰伤害", "穿刺伤害", "冲击伤害":
		return "B"
	case "多重射击":
		if sniper {
			return "S"
		}
		return "A"
	case "暴击几率", "暴击伤害":
		if baseCrit > highCrit {
			return "S"
		}
		return "A"
	case "射速/攻击速度", "装填速度", "弹匣容量":
		return "C"
	case "切割伤害":
		if baseCrit > highCrit {
			return "B"
		}
		return "A"
	default:
		return "-"
	}
}

// calcLethalLevel 计算负属性对武器的致命度
// （fatal/serious/harmful/acceptable/beneficial，对齐 Java calcLethalLevel）。
func calcLethalLevel(cat int, trendName string, weapon *modelwarframe.Weapons) string {
	if isBeneficialNegative(cat, trendName, weapon) {
		return "beneficial"
	}
	baseCrit := weapon.CriticalChance
	baseProc := weapon.ProcChance
	magSize := weapon.MagazineSize
	if magSize == 0 {
		magSize = 999
	}

	switch trendName {
	case "触发几率":
		if baseProc > procFatal && cat == catShotguns {
			return "fatal"
		}
		if baseProc > highProc {
			return "serious"
		}
		return "harmful"
	case "连击持续时间":
		if cat == catMelee && baseCrit > lowCrit {
			return "fatal"
		}
		return "serious"
	case "初始连击", "多重射击":
		return "serious"
	case "弹匣容量":
		if magSize < lowMagFatal {
			return "fatal"
		}
		return "harmful"
	case "伤害/近战伤害":
		return "harmful"
	case "暴击几率":
		if baseCrit > critFatal {
			return "serious"
		}
		return "acceptable"
	case "暴击伤害":
		if baseCrit > highCrit {
			return "serious"
		}
		return "acceptable"
	case "射速/攻击速度":
		if weapon.FireRate > highFireRate {
			return "serious"
		}
		return "harmful"
	case "攻击范围":
		if cat == catMelee {
			return "serious"
		}
		return "harmful"
	case "后坐力":
		return "harmful"
	case "穿刺伤害", "冲击伤害", "弹药最大值", "投射物飞行速度":
		return "acceptable"
	default:
		return "harmful"
	}
}

// isBeneficialNegative 检测负属性是否实际上对该武器有益（对齐 Java isBeneficialNegative）：
// 负冲击/负穿刺对主切割武器（切割占比 > 50%）有益；负后坐力对霰弹枪有益；负变焦对狙击枪有益。
func isBeneficialNegative(cat int, trendName string, weapon *modelwarframe.Weapons) bool {
	slashRatio := getSlashRatio(weapon)
	switch trendName {
	case "冲击伤害", "穿刺伤害":
		return slashRatio > 0.5
	case "后坐力":
		return cat == catShotguns
	case "变焦":
		return isSniper(weapon)
	default:
		return false
	}
}

// getSlashRatio 计算武器物理伤害中切割的占比（对齐 Java getSlashRatio：
// damagePerShot 数组前三位为 冲击/穿刺/切割）。
func getSlashRatio(weapon *modelwarframe.Weapons) float64 {
	var values []float64
	if err := json.Unmarshal([]byte(weapon.DamagePerShot), &values); err != nil || len(values) < 3 {
		return 0
	}
	total := values[0] + values[1] + values[2]
	if total <= 0 {
		return 0
	}
	return values[2] / total
}

// buildAnalysis 拼接综合分析文本（对齐 Java buildAnalysis）。
func buildAnalysis(attr *draw.RivenAnalyseTrendAttribute) string {
	var sb strings.Builder
	if attr.Grade != "" && attr.Grade != "-" {
		sb.WriteString(gradeToText(attr.Grade))
	}
	if attr.Ratio != "" && attr.Ratio != "-" {
		sb.WriteString("加成/基值: ")
		sb.WriteString(attr.Ratio)
		sb.WriteString("倍；")
	}
	if attr.LethalLevel != "" {
		sb.WriteString(lethalToText(attr.LethalLevel))
	}
	return sb.String()
}

// gradeToText 评分转评语（对齐 Java gradeToText）。
func gradeToText(grade string) string {
	switch grade {
	case "S":
		return "对该武器价值极高；"
	case "A":
		return "对该武器较为优秀；"
	case "B":
		return "对该武器中规中矩；"
	case "C":
		return "对该武器价值偏低；"
	case "D":
		return "对该武器价值很低；"
	default:
		return ""
	}
}

// lethalToText 致命度转评语（对齐 Java lethalToText）。
func lethalToText(lethal string) string {
	switch lethal {
	case "fatal":
		return "负属性致命，直接废卡！"
	case "serious":
		return "负属性严重影响输出/手感；"
	case "harmful":
		return "负属性有一定负面影响；"
	case "acceptable":
		return "该负属性对该武器影响不大；"
	case "beneficial":
		return "该负属性反而对该武器有益！"
	default:
		return ""
	}
}
