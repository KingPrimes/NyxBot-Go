// 订阅指令领域服务，对齐 Java SubscriptionApplicationService + WarframeTaskSubscribePlugin
// 的订阅/取消/列表文案与枚举 ordinal 契约：用户输入 code = Java 枚举声明下标（0-based）。
package warframe

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"nyxbot-go/internal/database"
	"nyxbot-go/internal/enum/nyxbot"
	"nyxbot-go/internal/logging"
	modelwarframe "nyxbot-go/internal/model/warframe"
)

// subscribeOrder 对齐 Java SubscribeType 声明顺序（下标即 ordinal/code）。
var subscribeOrder = []nyxbot.SubscribeType{
	nyxbot.SubAlerts, nyxbot.SubArbitration, nyxbot.SubCetusCycle, nyxbot.SubDailyDeals,
	nyxbot.SubEvents, nyxbot.SubInvasions, nyxbot.SubSteelPath, nyxbot.SubVoid,
	nyxbot.SubFissures, nyxbot.SubNews, nyxbot.SubNightwave, nyxbot.SubSortie,
	nyxbot.SubArchonHunt, nyxbot.SubDuviriCycle,
}

// missionOrder 对齐 Java MissionType 枚举声明顺序（订阅子参数）。
var missionOrder = []nyxbot.MissionType{
	nyxbot.MTExtermination, nyxbot.MTSurvival, nyxbot.MTRescue, nyxbot.MTSabotage,
	nyxbot.MTCapture, nyxbot.MTIntel, nyxbot.MTDefense, nyxbot.MTMobileDefense,
	nyxbot.MTTerritory, nyxbot.MTHive, nyxbot.MTRetrieval, nyxbot.MTExcavate,
	nyxbot.MTSalvage, nyxbot.MTPursuit, nyxbot.MTAssault, nyxbot.MTEvacuation,
	nyxbot.MTDisruption, nyxbot.MTVoidFlood, nyxbot.MTVoidCascade, nyxbot.MTVoidArmageddon,
	nyxbot.MTAlchemy, nyxbot.MTCambire, nyxbot.MTSkirmish, nyxbot.MTVolatile,
	nyxbot.MTOrpheus, nyxbot.MTAscension, nyxbot.MTCorruption,
}

// rewardOrder 对齐 Java InvasionReward 枚举声明顺序（NONE=0）。
var rewardOrder = []nyxbot.InvasionReward{
	nyxbot.InvRewardNone, nyxbot.InvRewardDetoniteInjector, nyxbot.InvRewardFieldron,
	nyxbot.InvRewardMutagenMass, nyxbot.InvRewardOrokinCatalyst, nyxbot.InvRewardOrokinReactor,
	nyxbot.InvRewardForma, nyxbot.InvRewardExilusAdapter,
}

// subscribeName SubscribeType 显示名（对齐 Java enum name）。
func subscribeName(t nyxbot.SubscribeType) string {
	switch t {
	case nyxbot.SubArbitration:
		return "仲裁"
	case nyxbot.SubCetusCycle:
		return "夜灵平野"
	case nyxbot.SubDailyDeals:
		return "每日特惠"
	case nyxbot.SubEvents:
		return "活动"
	case nyxbot.SubInvasions:
		return "入侵"
	case nyxbot.SubSteelPath:
		return "钢铁兑换"
	case nyxbot.SubVoid:
		return "奸商"
	case nyxbot.SubFissures:
		return "裂隙"
	case nyxbot.SubNews:
		return "新闻"
	case nyxbot.SubNightwave:
		return "电波"
	case nyxbot.SubSortie:
		return "突击"
	case nyxbot.SubArchonHunt:
		return "执政官突击"
	case nyxbot.SubDuviriCycle:
		return "双衍王境"
	default:
		return "警报"
	}
}

// missionName MissionType 显示名（对齐 Java MissionType name）。
func missionName(t nyxbot.MissionType) string {
	switch t {
	case nyxbot.MTExtermination:
		return "歼灭"
	case nyxbot.MTSurvival:
		return "生存"
	case nyxbot.MTRescue:
		return "救援"
	case nyxbot.MTSabotage:
		return "破坏"
	case nyxbot.MTCapture:
		return "捕获"
	case nyxbot.MTIntel:
		return "间谍"
	case nyxbot.MTDefense:
		return "防御"
	case nyxbot.MTMobileDefense:
		return "移动防御"
	case nyxbot.MTTerritory:
		return "拦截"
	case nyxbot.MTHive:
		return "清巢"
	case nyxbot.MTRetrieval:
		return "劫持"
	case nyxbot.MTExcavate:
		return "挖掘"
	case nyxbot.MTSalvage:
		return "资源回收"
	case nyxbot.MTPursuit:
		return "追击"
	case nyxbot.MTAssault:
		return "强袭"
	case nyxbot.MTEvacuation:
		return "叛逃"
	case nyxbot.MTDisruption:
		return "中断"
	case nyxbot.MTVoidFlood:
		return "虚空洪流"
	case nyxbot.MTVoidCascade:
		return "虚空覆涌"
	case nyxbot.MTVoidArmageddon:
		return "虚空决战"
	case nyxbot.MTAlchemy:
		return "元素转换"
	case nyxbot.MTCambire:
		return "异化区"
	case nyxbot.MTSkirmish:
		return "前哨战"
	case nyxbot.MTVolatile:
		return "爆发"
	case nyxbot.MTOrpheus:
		return "奥菲斯"
	case nyxbot.MTAscension:
		return "扬升"
	case nyxbot.MTCorruption:
		return "虚空腐蚀"
	default:
		return ""
	}
}

// invasionRewardName InvasionReward 显示名（NONE=无）。
func invasionRewardName(r nyxbot.InvasionReward) string {
	switch r {
	case nyxbot.InvRewardDetoniteInjector:
		return "突变原聚合物"
	case nyxbot.InvRewardFieldron:
		return "力场装置样本"
	case nyxbot.InvRewardMutagenMass:
		return "诱变剂物质"
	case nyxbot.InvRewardOrokinCatalyst:
		return "Orokin 催化剂"
	case nyxbot.InvRewardOrokinReactor:
		return "Orokin 反应堆"
	case nyxbot.InvRewardForma:
		return "Forma"
	case nyxbot.InvRewardExilusAdapter:
		return "特殊功能槽连接器"
	default:
		return "无"
	}
}

// tierName 遗物等级显示名（对齐 Java getTierName）。
func tierName(tier int) string {
	switch tier {
	case 1:
		return "古纪 (Lith)"
	case 2:
		return "前纪 (Meso)"
	case 3:
		return "中纪 (Neo)"
	case 4:
		return "后纪 (Axi)"
	case 5:
		return "安魂 (Requiem)"
	default:
		return fmt.Sprintf("Tier %d", tier)
	}
}

// parseByCode 解析用户输入 code（对齐 Java values()[code]：0 基下标，范围 [1, len-1]）。
func parseByCode[T any](code int, list []T) (T, bool) {
	var zero T
	if code <= 0 || code >= len(list) {
		return zero, false
	}
	return list[code], true
}

// ParseSubscribeType 解析订阅类型数字。
func ParseSubscribeType(code int) (nyxbot.SubscribeType, bool) {
	return parseByCode(code, subscribeOrder)
}

// ParseMissionType 解析任务类型数字。
func ParseMissionType(code int) (nyxbot.MissionType, bool) {
	return parseByCode(code, missionOrder)
}

// ParseInvasionReward 解析入侵奖励数字。
func ParseInvasionReward(code int) (nyxbot.InvasionReward, bool) {
	return parseByCode(code, rewardOrder)
}

// SubscribeCommand 订阅命令值对象（对齐 Java SubscriptionCommand record）。
type SubscribeCommand struct {
	BotUID    int64                  // Bot UID
	GroupID   int64                  // 群号
	GroupName string                 // 群名称
	UserID    int64                  // 用户 ID
	UserName  string                 // 用户昵称
	SubType   nyxbot.SubscribeType   // 订阅类型（解析后注入）
	Mission   *nyxbot.MissionType    // 任务类型（可空 = 全部）
	Tier      *int                   // 遗物等级（可空 = 全部）
	Reward    *nyxbot.InvasionReward // 入侵奖励（可空 = 全部）
}

// parseSubParams 解析订阅子参数（按类型：入侵-奖励 / 裂隙-任务+等级 / 仲裁-任务）。
func parseSubParams(subType nyxbot.SubscribeType, parts []string) (*nyxbot.MissionType, *int, *nyxbot.InvasionReward, bool) {
	switch subType {
	case nyxbot.SubInvasions:
		if len(parts) > 1 {
			r, ok := ParseInvasionReward(atoiOrZero(parts[1]))
			if !ok {
				return nil, nil, nil, false
			}
			return nil, nil, &r, true
		}
	case nyxbot.SubFissures:
		if len(parts) > 1 {
			m, ok := ParseMissionType(atoiOrZero(parts[1]))
			if !ok {
				return nil, nil, nil, false
			}
			if len(parts) > 2 {
				t, err := strconv.Atoi(parts[2])
				if err != nil {
					return nil, nil, nil, false
				}
				return &m, &t, nil, true
			}
			return &m, nil, nil, true
		}
	case nyxbot.SubArbitration:
		if len(parts) > 1 {
			m, ok := ParseMissionType(atoiOrZero(parts[1]))
			if !ok {
				return nil, nil, nil, false
			}
			return &m, nil, nil, true
		}
	}
	return nil, nil, nil, true
}

// ParseUnsubscribeParams 解析取消订阅的类型与子参数（通配子参数传 nil）。
func ParseUnsubscribeParams(code int, parts []string) (nyxbot.SubscribeType, *nyxbot.MissionType, *int, *nyxbot.InvasionReward, bool) {
	subType, ok := ParseSubscribeType(code)
	if !ok {
		return "", nil, nil, nil, false
	}
	mission, tier, reward, ok := parseSubParams(subType, parts)
	return subType, mission, tier, reward, ok
}

// ParseSubscribeCommand 将已去「订阅」前缀、去空格的内容（如 "9-2-4"）解析到命令。
func ParseSubscribeCommand(command *SubscribeCommand, raw string) bool {
	if command == nil {
		return false
	}
	parts := make([]string, 0, 3)
	for _, part := range strings.Split(raw, "-") {
		part = strings.TrimSpace(part)
		if part != "" {
			parts = append(parts, part)
		}
	}
	if len(parts) == 0 {
		return false
	}
	code, err := strconv.Atoi(parts[0])
	if err != nil {
		return false
	}
	subType, ok := ParseSubscribeType(code)
	if !ok {
		return false
	}
	command.SubType = subType
	switch subType {
	case nyxbot.SubInvasions:
		if len(parts) > 1 {
			r, ok := ParseInvasionReward(atoiOrZero(parts[1]))
			if !ok {
				return false
			}
			command.Reward = &r
		}
	case nyxbot.SubFissures:
		if len(parts) > 1 {
			m, ok := ParseMissionType(atoiOrZero(parts[1]))
			if !ok {
				return false
			}
			command.Mission = &m
		}
		if len(parts) > 2 {
			t, err := strconv.Atoi(parts[2])
			if err != nil {
				return false
			}
			command.Tier = &t
		}
	case nyxbot.SubArbitration:
		if len(parts) > 1 {
			m, ok := ParseMissionType(atoiOrZero(parts[1]))
			if !ok {
				return false
			}
			command.Mission = &m
		}
	}
	return true
}

// Subscribe 新增订阅规则并返回回复文案。
func Subscribe(cmd *SubscribeCommand) string {
	if database.DB == nil {
		return "订阅失败：数据库不可用"
	}
	subscription, err := findOrCreateSubscription(cmd)
	if err != nil {
		logging.ErrorPack("warframe.subscribe", "create subscription failed: %v", err)
		return "订阅失败，请稍后再试"
	}
	user, err := findOrCreateUser(subscription.ID, cmd)
	if err != nil {
		logging.ErrorPack("warframe.subscribe", "create subscription user failed: %v", err)
		return "订阅失败，请稍后再试"
	}
	exists, err := hasMatchingRule(user.ID, cmd)
	if err != nil {
		logging.ErrorPack("warframe.subscribe", "query existing rule failed: %v", err)
		return "订阅失败，请稍后再试"
	}
	if exists {
		return "你已订阅过当前规则了！\n" + buildSubscriptionInfo(cmd)
	}
	rule := &modelwarframe.MissionSubscribeUserCheckType{
		SubuID:          user.ID,
		Subscribe:       string(cmd.SubType),
		MissionTypeEnum: missionToStr(cmd.Mission),
		TierNum:         tierOrZero(cmd.Tier),
		InvasionReward:  invasionToStr(cmd.Reward),
	}
	if err := database.DB.Create(rule).Error; err != nil {
		logging.ErrorPack("warframe.subscribe", "create rule failed: %v", err)
		return "订阅失败，请稍后再试"
	}
	return "订阅成功！\n" + buildSubscriptionInfo(cmd)
}

// Unsubscribe 为用户取消匹配的订阅规则（约束：mission/tier/reward 传 nil 表示全部通配，Java 语义）。
func Unsubscribe(groupID, userID int64, subType nyxbot.SubscribeType, mission *nyxbot.MissionType, tier *int, reward *nyxbot.InvasionReward) string {
	if database.DB == nil {
		return "数据库不可用"
	}
	var subscription modelwarframe.MissionSubscribe
	if err := database.DB.Where("sub_group = ?", groupID).First(&subscription).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "未找到该群订阅"
		}
		return "取消订阅失败，请稍后再试"
	}
	var user modelwarframe.MissionSubscribeUser
	if err := database.DB.Where("sub_id = ? AND user_id = ?", subscription.ID, userID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "该用户没有任何订阅"
		}
		return "取消订阅失败，请稍后再试"
	}
	var rules []modelwarframe.MissionSubscribeUserCheckType
	if err := database.DB.Where("subu_id = ?", user.ID).Find(&rules).Error; err != nil {
		return "取消订阅失败，请稍后再试"
	}
	removed := 0
	for i := range rules {
		if ruleMatchesUnsubscribe(&rules[i], subType, mission, tier, reward) {
			if err := database.DB.Delete(&modelwarframe.MissionSubscribeUserCheckType{}, rules[i].ID).Error; err == nil {
				removed++
			}
		}
	}
	cleanupEmptyUser(user.ID, subscription.ID)
	if removed == 0 {
		return "未找到匹配的订阅规则"
	}
	return fmt.Sprintf("成功取消 %d 条订阅", removed)
}

// UserSubscriptionInfo 返回用户当前订阅列表（无则空串，对齐 Java getUserSubscriptionInfo）。
func UserSubscriptionInfo(groupID, userID int64) string {
	if database.DB == nil {
		return ""
	}
	var subscription modelwarframe.MissionSubscribe
	if err := database.DB.Where("sub_group = ?", groupID).First(&subscription).Error; err != nil {
		return ""
	}
	var user modelwarframe.MissionSubscribeUser
	if err := database.DB.Where("sub_id = ? AND user_id = ?", subscription.ID, userID).First(&user).Error; err != nil {
		return ""
	}
	var rules []modelwarframe.MissionSubscribeUserCheckType
	if err := database.DB.Where("subu_id = ?", user.ID).Find(&rules).Error; err != nil || len(rules) == 0 {
		return ""
	}
	boundary := "━━━━━━━━━━━━━━━━━━"
	var sb strings.Builder
	sb.WriteString("当前订阅：\n" + boundary + "\n")
	for i := range rules {
		sb.WriteString(formatCheckTypeLine(&rules[i]) + "\n")
	}
	sb.WriteString(boundary)
	return sb.String()
}

// SubscribeHelpEnums 返回帮助图三张编号表（编号=subscript MISSION ordinal，排除 0，对齐 Java get*Enums）。
func SubscribeHelpEnums() (subscribe, mission, reward map[int]string) {
	subscribe = make(map[int]string)
	for i := 1; i < len(subscribeOrder); i++ {
		subscribe[i] = subscribeName(subscribeOrder[i])
	}
	mission = make(map[int]string)
	for i := 1; i < len(missionOrder); i++ {
		mission[i] = missionName(missionOrder[i])
	}
	reward = make(map[int]string)
	for i := 1; i < len(rewardOrder); i++ {
		reward[i] = invasionRewardName(rewardOrder[i])
	}
	return subscribe, mission, reward
}

// findOrCreateSubscription 查找或创建订阅组。
func findOrCreateSubscription(cmd *SubscribeCommand) (*modelwarframe.MissionSubscribe, error) {
	var subscription modelwarframe.MissionSubscribe
	err := database.DB.Where("sub_group = ?", cmd.GroupID).First(&subscription).Error
	if err == nil {
		return &subscription, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	subscription = modelwarframe.MissionSubscribe{
		GroupName: cmd.GroupName,
		SubBotUID: cmd.BotUID,
		SubGroup:  cmd.GroupID,
	}
	if err := database.DB.Create(&subscription).Error; err != nil {
		return nil, err
	}
	return &subscription, nil
}

// findOrCreateUser 查找或创建订阅用户。
func findOrCreateUser(subscriptionID uint, cmd *SubscribeCommand) (*modelwarframe.MissionSubscribeUser, error) {
	var user modelwarframe.MissionSubscribeUser
	err := database.DB.Where("sub_id = ? AND user_id = ?", subscriptionID, cmd.UserID).First(&user).Error
	if err == nil {
		return &user, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	user = modelwarframe.MissionSubscribeUser{
		SubID:    subscriptionID,
		UserID:   cmd.UserID,
		UserName: cmd.UserName,
	}
	if err := database.DB.Create(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// hasMatchingRule 判断用户是否已存在相同订阅规则。
func hasMatchingRule(subuID uint, cmd *SubscribeCommand) (bool, error) {
	var count int64
	err := database.DB.Model(&modelwarframe.MissionSubscribeUserCheckType{}).
		Where("subu_id = ? AND subscribe = ? AND mission_type_enum = ? AND tier_num = ? AND invasion_reward = ?",
			subuID, string(cmd.SubType), missionToStr(cmd.Mission), tierOrZero(cmd.Tier), invasionToStr(cmd.Reward)).
		Count(&count).Error
	return count > 0, err
}

// ruleMatchesUnsubscribe 判断订阅规则是否匹配取滑条件（对齐 Java matchesUnsubscribeCondition）。
func ruleMatchesUnsubscribe(rule *modelwarframe.MissionSubscribeUserCheckType, subType nyxbot.SubscribeType, mission *nyxbot.MissionType, tier *int, reward *nyxbot.InvasionReward) bool {
	if rule.Subscribe != string(subType) {
		return false
	}
	if mission != nil && rule.MissionTypeEnum != string(*mission) {
		return false
	}
	if tier != nil && rule.TierNum != *tier {
		return false
	}
	if reward != nil && rule.InvasionReward != string(*reward) {
		return false
	}
	return true
}

// formatCheckTypeLine 格式化单条订阅（对齐 Java formatCheckTypeLine）。
func formatCheckTypeLine(rule *modelwarframe.MissionSubscribeUserCheckType) string {
	var sb strings.Builder
	sb.WriteString("[" + strconv.Itoa(ordinalOfSubscribe(nyxbot.SubscribeType(rule.Subscribe))) + "] ")
	sb.WriteString(subscribeName(nyxbot.SubscribeType(rule.Subscribe)))
	if rule.MissionTypeEnum != "" {
		sb.WriteString(" - " + missionName(nyxbot.MissionType(rule.MissionTypeEnum)))
	}
	if rule.TierNum != 0 {
		sb.WriteString(" - " + tierName(rule.TierNum))
	}
	if rule.InvasionReward != "" && rule.InvasionReward != string(nyxbot.InvRewardNone) {
		sb.WriteString(" - " + invasionRewardName(nyxbot.InvasionReward(rule.InvasionReward)))
	}
	return sb.String()
}

// ordinalOfSubscribe 返回订阅类型在 subscribeOrder 中的下标（对齐 Java ordinal）。
func ordinalOfSubscribe(c nyxbot.SubscribeType) int {
	for i, v := range subscribeOrder {
		if v == c {
			return i
		}
	}
	return 0
}

// buildSubscriptionInfo 构造订阅成功/已存在的详情文案（对齐 Java buildSubscriptionInfo）。
func buildSubscriptionInfo(cmd *SubscribeCommand) string {
	boundary := "━━━━━━━━━━━━━━━━━━"
	var sb strings.Builder
	sb.WriteString(boundary + "\n")
	sb.WriteString("类型: " + subscribeName(cmd.SubType))
	if cmd.Mission != nil {
		sb.WriteString("\n任务: " + missionName(*cmd.Mission))
	} else {
		sb.WriteString("\n任务: 全部")
	}
	if cmd.Tier != nil {
		sb.WriteString("\n等级: " + tierName(*cmd.Tier))
	} else if cmd.SubType == nyxbot.SubFissures {
		sb.WriteString("\n等级: 全部")
	}
	sb.WriteString("\n" + boundary)
	return sb.String()
}

// cleanupEmptyUser 删除无规则的用户与无用户的订阅组（对齐 Java）。
func cleanupEmptyUser(subuID uint, subscriptionID uint) {
	var remaining int64
	database.DB.Model(&modelwarframe.MissionSubscribeUserCheckType{}).Where("subu_id = ?", subuID).Count(&remaining)
	if remaining > 0 {
		return
	}
	database.DB.Delete(&modelwarframe.MissionSubscribeUser{}, subuID)
	var users int64
	database.DB.Model(&modelwarframe.MissionSubscribeUser{}).Where("sub_id = ?", subscriptionID).Count(&users)
	if users == 0 {
		database.DB.Delete(&modelwarframe.MissionSubscribe{}, subscriptionID)
	}
}

// 便捷转换辅助。
func missionToStr(m *nyxbot.MissionType) string {
	if m == nil {
		return ""
	}
	return string(*m)
}

func invasionToStr(r *nyxbot.InvasionReward) string {
	if r == nil {
		return ""
	}
	return string(*r)
}

func tierOrZero(t *int) int {
	if t == nil {
		return 0
	}
	return *t
}

func atoiOrZero(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}
