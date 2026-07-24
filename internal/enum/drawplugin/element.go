// draw-image-plugin 元素伤害枚举 —— 中文名 + 颜色 + 字体图标
package drawplugin

type ElementInfo struct {
	Name string
	Icon string
}

type Element string

const (
	ElemElectricity Element = "ELECTRICITY"
	ElemImpact      Element = "IMPACT"
	ElemRadiation   Element = "RADIATION"
	ElemMagnetic    Element = "MAGNETIC"
	ElemCold        Element = "COLD"
	ElemToxin       Element = "TOXIN"
	ElemHeat        Element = "HEAT"
	ElemPuncture    Element = "PUNCTURE"
	ElemSlash       Element = "SLASH"
	ElemBlast       Element = "BLAST"
	ElemCorrosive   Element = "CORROSIVE"
	ElemGas         Element = "GAS"
	ElemViral       Element = "VIRAL"
	ElemVoid        Element = "VOID"
	ElemTau         Element = "TAU"
	ElemTrue        Element = "TRUE"
)

var ElementMap = map[Element]ElementInfo{
	ElemElectricity: {"电击", "\ue601"},
	ElemImpact:      {"冲击", "\ue602"},
	ElemRadiation:   {"辐射", "\ue603"},
	ElemMagnetic:    {"磁力", "\ue604"},
	ElemCold:        {"冰冻", "\ue605"},
	ElemToxin:       {"毒素", "\ue606"},
	ElemHeat:        {"火焰", "\ue607"},
	ElemPuncture:    {"穿刺", "\ue608"},
	ElemSlash:       {"切割", "\ue609"},
	ElemBlast:       {"爆炸", "\ue60a"},
	ElemCorrosive:   {"腐蚀", "\ue60b"},
	ElemGas:         {"气体", "\ue60c"},
	ElemViral:       {"病毒", "\ue60d"},
	ElemVoid:        {"虚空", "\ue60e"},
	ElemTau:         {"Tau", "\ue60f"},
	ElemTrue:        {"真实", "\ue610"},
}
