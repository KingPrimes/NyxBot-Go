// Riven 模块相关数据，对应 Java NyxBot 的四个实体：
//
//	RivenTion - 紫卡倾向词条（效果、前缀/后缀、单位）
//	RivenTionAlias - 紫卡词条别名（英文 -> 中文）
//	RivenItem - 紫卡可交易物品（market/riven 子模块）
//	RivenAnalyseTrend - 紫卡价格分析趋势
package warframe

// RivenTion 紫卡词条条目。主键字段名为 ids（对齐 Java 的 ids 主键，JSON/列名均为 ids）。
// JSON 字段对齐前端 Api.LocalData.RivenTion。
type RivenTion struct {
	IDs                uint    `gorm:"primaryKey" json:"ids"`                                 // 主键（自增，JSON 名 ids）
	Effect             string  `gorm:"column:effect" json:"effect"`                           // 词条效果
	Group              string  `gorm:"column:group" json:"group"`                             // 词条分组
	NegativeOnly       float64 `gorm:"column:negative_only" json:"negativeOnly"`              // 仅负面
	PositiveIsNegative float64 `gorm:"column:positive_is_negative" json:"positiveIsNegative"` // 正面即负面
	Prefix             string  `gorm:"column:prefix" json:"prefix"`                           // 前缀
	SearchOnly         float64 `gorm:"column:search_only" json:"searchOnly"`                  // 仅搜索
	Suffix             string  `gorm:"column:suffix" json:"suffix"`                           // 后缀
	Units              string  `gorm:"column:units" json:"units"`                             // 单位
	URLName            string  `gorm:"column:url_name" json:"urlName"`                        // 英文 url_name（unique）
	ExclusiveTo        string  `gorm:"column:exclusive_to" json:"exclusiveTo"`                // 专属武器
}

func (RivenTion) TableName() string {
	return "riven_tion"
}

// RivenTionAlias 紫卡词条别名（en + cn 唯一）。
// JSON 字段对齐前端 Api.LocalData.RivenTionAlias { id, en, cn }。
type RivenTionAlias struct {
	ID uint   `gorm:"primaryKey" json:"id"` // 主键（自增）
	En string `gorm:"column:en" json:"en"`  // 英文
	Cn string `gorm:"column:cn" json:"cn"`  // 中文
}

func (RivenTionAlias) TableName() string {
	return "riven_tion_alias"
}

// RivenItem 紫卡可交易物品条目。id 为字符串主键。
// JSON 字段对齐前端 Api.LocalData.MarketRiven。
type RivenItem struct {
	ID             string  `gorm:"primaryKey" json:"id"`                          // 主键（唯一标识）
	Slug           string  `gorm:"column:slug" json:"slug"`                       // 市场 slug
	GameRef        string  `gorm:"column:game_ref" json:"gameRef"`                // 游戏内引用
	Group          string  `gorm:"column:group" json:"group"`                     // 分组
	RivenType      string  `gorm:"column:riven_type" json:"rivenType"`            // 紫卡类型（list 过滤字段）
	Disposition    float64 `gorm:"column:disposition" json:"disposition"`         // 倾向
	ReqMasteryRank int     `gorm:"column:req_mastery_rank" json:"reqMasteryRank"` // 段位要求
	Name           string  `gorm:"column:name" json:"name"`                       // 名称（list 过滤字段）
	Icon           string  `gorm:"column:icon" json:"icon"`                       // 图标链接
	Thumb          string  `gorm:"column:thumb" json:"thumb"`                     // 缩略图链接
}

func (RivenItem) TableName() string {
	return "riven_items"
}

// RivenAnalyseTrend 紫卡价格分析趋势（name 唯一）。
// JSON 字段对齐前端 Api.LocalData.RivenAnalyseTrend。
type RivenAnalyseTrend struct {
	ID       uint    `gorm:"primaryKey" json:"id"`            // 主键（自增）
	Archwing float64 `gorm:"column:archwing" json:"archwing"` // 空战武器倾向
	Melle    float64 `gorm:"column:melle" json:"melle"`       // 近战倾向（对齐 Java 拼写 melle）
	Name     string  `gorm:"column:name" json:"name"`         // 词条效果名（如「暴击几率」，导入时由 tagToTrendName 生成）
	Pistol   float64 `gorm:"column:pistol" json:"pistol"`     // 手枪倾向
	Prefix   string  `gorm:"column:prefix" json:"prefix"`     // 前缀
	Rifle    float64 `gorm:"column:rifle" json:"rifle"`       // 步枪倾向
	Shotgun  float64 `gorm:"column:shotgun" json:"shotgun"`   // 霰弹枪倾向
	Suffix   string  `gorm:"column:suffix" json:"suffix"`     // 后缀
}

func (RivenAnalyseTrend) TableName() string {
	return "riven_analyse_trend"
}
