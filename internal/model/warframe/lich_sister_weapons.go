// 赤毒/信条武器表，对应 Java NyxBot 的 LichSisterWeapons 实体（表 lich_sister_weapons）
// 存储玄骸/姐妹拍卖用武器条目（url_name + item_name 唯一）
package warframe

// LichSisterWeapon 赤毒/信条武器条目。id 为字符串主键。
// JSON 字段对齐前端 Api.LocalData.LichSisterWeapons。
type LichSisterWeapon struct {
	ID             string `gorm:"primaryKey" json:"id"`                          // 主键（唯一标识）
	Slug           string `gorm:"column:slug" json:"slug"`                       // 市场 slug
	Icon           string `gorm:"column:icon" json:"icon"`                       // 图标链接
	GameRef        string `gorm:"column:game_ref" json:"gameRef"`                // 游戏内引用
	ReqMasteryRank int    `gorm:"column:req_mastery_rank" json:"reqMasteryRank"` // 段位要求
	Name           string `gorm:"column:name" json:"name"`                       // 武器名称
	Thumb          string `gorm:"column:thumb" json:"thumb"`                     // 缩略图链接
}

func (LichSisterWeapon) TableName() string {
	return "lich_sister_weapons"
}
