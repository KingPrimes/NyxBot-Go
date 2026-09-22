// 世界状态查询（批次 A）：警报 / 执刑官猎杀 / 钢铁奖励。
//
// 与 worldstate_query.go 同源：真实 WorldState API 的数组字段为 Java 命名风格（大写首字母），
// 故沿用自定义 envelope 逐个声明所需字段，而不复用 model.WorldState。
//
// 钢铁奖励为**纯本地计算**（对齐 Java SteelPathOffering：由固定基准日 + 8 周轮换表推出
// 当前/下一个奖励与本周剩余时间），不读 WorldState、不发网络请求。
package warframe

import (
	"time"

	"nyxbot-go/internal/draw"
	"nyxbot-go/internal/enum/drawplugin"
	"nyxbot-go/internal/warframe/cycle"
)

// wsAlertEnvelope 警报 / 执刑官猎杀所需字段的 WorldState 视图。
type wsAlertEnvelope struct {
	Alerts      []wsAlert      `json:"Alerts"`
	LiteSorties []wsLiteSorite `json:"LiteSorties"`
}

// wsAlert 对齐 Java model.Alert 的 JSON 字段。
type wsAlert struct {
	MissionInfo *wsAlertMissionInfo `json:"MissionInfo"`
}

// wsAlertMissionInfo 对齐 Java Alert.MissionInfo 的 JSON 字段。
type wsAlertMissionInfo struct {
	Location      string         `json:"location"`
	MissionType   string         `json:"missionType"`
	Faction       string         `json:"faction"`
	MissionReward *wsAlertReward `json:"missionReward"`
}

// wsAlertReward 对齐 Java Alert.MissionInfo.Reward 的 JSON 字段。
type wsAlertReward struct {
	Credits *int     `json:"credits"`
	Items   []string `json:"items"`
}

// wsLiteSorite 对齐 Java model.LiteSorite 的 JSON 字段。
type wsLiteSorite struct {
	ID       string                `json:"_id"`
	Boss     string                `json:"Boss"`
	Expiry   string                `json:"Expiry"`
	Missions []wsLiteSoriteMission `json:"Missions"`
}

// wsLiteSoriteMission 对齐 Java LiteSorite.Mission 的 JSON 字段。
type wsLiteSoriteMission struct {
	MissionType string `json:"missionType"`
	Node        string `json:"node"`
}

// GetAlerts 解析警报列表（对齐 Java WorldStateUtils.getAlerts）：
// 过滤空条目与缺失 MissionInfo 的条目 → 翻译节点、派系与奖励物品名。
func GetAlerts() ([]*draw.Alert, error) {
	env, err := parseAlertEnvelope()
	if err != nil {
		return nil, err
	}
	alerts := make([]*draw.Alert, 0, len(env.Alerts))
	for i := range env.Alerts {
		if translated := translateAlert(&env.Alerts[i]); translated != nil {
			alerts = append(alerts, translated)
		}
	}
	return alerts, nil
}

// translateAlert 翻译单条警报（对齐 Java translateAlerts：
// 奖励物品名查 state_translation，location 查 nodes 表拼「名称(星系)」）。
// 缺少 MissionInfo 时返回 nil（Java 侧同一过滤条件）。
func translateAlert(alert *wsAlert) *draw.Alert {
	if alert == nil || alert.MissionInfo == nil {
		return nil
	}
	info := alert.MissionInfo

	reward := &draw.AlertReward{Credits: alertRewardCredits(info.MissionReward)}
	for _, item := range alertRewardItems(info.MissionReward) {
		// 对齐 Java translateAlerts：按完整 uniqueName 直接查表（不做 last3 截取）
		reward.Items = append(reward.Items, TranslateStateNameDirect(item))
	}

	return &draw.Alert{
		MissionInfo: &draw.AlertMissionInfo{
			// 派系不在 Alert 载荷中，改由节点表 factionIndex 推导（对齐 nodes 表用法）
			Location:      TranslateNode(info.Location),
			MissionType:   drawplugin.MissionType(info.MissionType),
			Faction:       nodeFactionByKey(info.Location),
			MissionReward: reward,
		},
	}
}

// alertRewardCredits 返回星币指针（奖励结构为 nil 时为 nil）。
func alertRewardCredits(reward *wsAlertReward) *int {
	if reward == nil {
		return nil
	}
	return reward.Credits
}

// alertRewardItems 返回奖励物品列表（奖励结构为 nil 时为 nil）。
func alertRewardItems(reward *wsAlertReward) []string {
	if reward == nil {
		return nil
	}
	return reward.Items
}

// GetLiteSorite 解析执刑官猎杀（对齐 Java WorldStateUtils.getLiteSorite）：
// 逐任务翻译节点，Boss 由枚举映射中文名，剩余时间由 Expiry 计算。
func GetLiteSorite() ([]*draw.LiteSorite, error) {
	env, err := parseAlertEnvelope()
	if err != nil {
		return nil, err
	}
	list := make([]*draw.LiteSorite, 0, len(env.LiteSorties))
	for i := range env.LiteSorties {
		list = append(list, translateLiteSorite(&env.LiteSorties[i]))
	}
	return list, nil
}

// translateLiteSorite 翻译单条执刑官猎杀记录。
func translateLiteSorite(ls *wsLiteSorite) *draw.LiteSorite {
	dto := &draw.LiteSorite{
		Boss:   bossName(ls.Boss),
		Expiry: remainingUntil(ls.Expiry),
	}
	for i := range ls.Missions {
		mission := &ls.Missions[i]
		dto.Missions = append(dto.Missions, &draw.LiteSoriteMission{
			TypeName:  missionTypeName(mission.MissionType),
			TypeColor: draw.MissionTypeColor(drawplugin.MissionType(mission.MissionType)),
			Node:      TranslateNode(mission.Node),
		})
	}
	return dto
}

// missionTypeName 任务类型枚举键 → 中文名（未命中回退原文）。
func missionTypeName(key string) string {
	if info, ok := drawplugin.MissionTypeMap[drawplugin.MissionType(key)]; ok {
		return info.Name
	}
	return key
}

// remainingUntil 计算到指定 RFC3339 时间的剩余时间文本（解析失败返回空串）。
func remainingUntil(expiry string) string {
	parsed := parseTime(expiry)
	if parsed.IsZero() {
		return ""
	}
	return remainingFrom(parsed)
}

// remainingFrom 剩余时间文本（对齐 Java timeDeltaToString 的 "Xd Xh Xm Xs" 形态）。
func remainingFrom(expiry time.Time) string {
	return cycle.TimeDeltaToString(expiry.Sub(time.Now()).Milliseconds())
}

// parseAlertEnvelope 从默认缓存读取原始 JSON 并解析本批次的 envelope。
func parseAlertEnvelope() (*wsAlertEnvelope, error) {
	return parseWorldStateEnvelope[wsAlertEnvelope]("alert")
}

// —— 钢铁奖励（本地计算，对齐 Java SteelPathOffering）——

// steelPathStartDate 轮换基准日（对齐 Java START_DATE = 2020-11-16 00:00 UTC）。
var steelPathStartDate = time.Date(2020, time.November, 16, 0, 0, 0, 0, time.UTC)

// steelPathRotation 8 周轮换奖励表（顺序即轮换序，对齐 Java SteelPathOffering.rotation）。
var steelPathRotation = []string{
	"Umbra Forma 蓝图",
	"50,000 赤毒",
	"组合枪裂罅 Mod",
	"3x Forma",
	"Zaw 裂罅 Mod",
	"30,000 内融核心",
	"步枪裂罅 Mod",
	"霰弹枪裂罅 Mod",
}

// 轮换周期常量（对齐 Java sevenDays / eightWeeks）。
const (
	steelPathWeekSeconds  int64 = 604800
	steelPathCycleSeconds int64 = 4838400
)

// GetSteelPath 计算当前钢铁奖励轮换（对齐 Java SteelPathPlugin.postSteelPathImage）。
// 纯本地计算，不读 WorldState、不发网络请求。
func GetSteelPath() *draw.SteelPathOffering {
	now := time.Now()
	secondsSinceStart := int64(now.UTC().Sub(steelPathStartDate).Seconds())
	index := (secondsSinceStart % steelPathCycleSeconds) / steelPathWeekSeconds

	size := int64(len(steelPathRotation))
	current := steelPathRotation[index%size]
	next := steelPathRotation[(index+1)%size]

	expiry := endOfSteelPathWeek(now)
	return &draw.SteelPathOffering{
		CurrentReward: current,
		NextReward:    next,
		Remaining:     remainingFrom(expiry),
	}
}

// endOfSteelPathWeek 返回本周结束时刻（下周一 00:00 本地时间，对齐 Java TimeUtils.getLastDayOfWeek）。
func endOfSteelPathWeek(t time.Time) time.Time {
	weekday := int(t.Weekday()) // Sunday = 0
	if weekday == 0 {
		weekday = 7 // 把周日当作一周第 7 天
	}
	daysUntilNextMonday := 8 - weekday
	startOfDay := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	return startOfDay.AddDate(0, 0, daysUntilNextMonday)
}
