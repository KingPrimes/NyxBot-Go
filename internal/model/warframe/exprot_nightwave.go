// 午夜电波导出数据表，对应 Java NyxBot 的 exprot.NightWave 实体
// 存储从官方 API 导出的午夜电波挑战数据（含赛季、声望、每日/每周/精英挑战）
package warframe

type NightWave struct {
	ID                  uint   `gorm:"primaryKey"`                    // 主键
	Name                string `gorm:"column:name"`                   // 挑战名称（中文）
	Standing            int    `gorm:"column:standing"`               // 声望值
	Description         string `gorm:"column:description"`            // 挑战描述
	EnName              string `gorm:"column:en_name"`                // 英文名
	ImageName           string `gorm:"column:image_name"`             // 图片文件名
	Tags                string `gorm:"column:tags"`                   // 标签
	Season              int    `gorm:"column:season"`                 // 赛季编号
	SeasonStarted       string `gorm:"column:season_started"`         // 赛季开始时间
	SeasonEnd           string `gorm:"column:season_end"`             // 赛季结束时间
	ActiveChallenges    string `gorm:"column:active_challenges"`      // 当前活跃挑战（JSON）
	DailyChallenges     string `gorm:"column:daily_challenges"`       // 每日挑战（JSON）
	WeeklyChallenges    string `gorm:"column:weekly_challenges"`      // 每周挑战（JSON）
	EliteWeeklyChallenges string `gorm:"column:elite_weekly_challenges"` // 精英每周挑战（JSON）
	Reputation          int    `gorm:"column:reputation"`            // 声望总量
	Exclude             bool   `gorm:"column:exclude"`               // 是否排除
	IsDaily             bool   `gorm:"column:is_daily"`              // 是否为每日挑战
	IsWeekly            bool   `gorm:"column:is_weekly"`             // 是否为每周挑战
	IsElite             bool   `gorm:"column:is_elite"`              // 是否为精英挑战
	Evergreens          string `gorm:"column:evergreens"`            // 常驻奖励（JSON）
	Available           bool   `gorm:"column:available"`             // 是否可用
}

func (NightWave) TableName() string {
	return "exprot_nightwave"
}
