// 世界状态查询（批次 D）：双衍轮换 / 电波 / 1999 日历。
//
// 对齐 Java：
//   - 双衍轮换 WorldStateUtils.getDuvalierCycle + translateDuvalierCycle（情绪本地计算，
//     选项取自 WorldState.EndlessXpSchedule 的 CategoryChoices，钢铁项按 weapons 表英文名译中文）
//   - 电波 WorldStateUtils.getSeasonInfo（Challenge 关联 night_wave 表取名称/描述/声望；
//     daily/weekly/elite 标记直接来自 WorldState 载荷，night_wave 表无对应列）
//   - 1999 日历 WorldStateUtils.getKnownCalendarSeasons（Days 的 day 为一年中第几天，
//     换算为月/日并按月分组；事件字段按类型查 state_translation）
package warframe

import (
	"errors"
	"sort"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"nyxbot-go/internal/database"
	"nyxbot-go/internal/draw"
	"nyxbot-go/internal/logging"
	modelwarframe "nyxbot-go/internal/model/warframe"
	"nyxbot-go/internal/warframe/cycle"
)

// —— 双衍轮换 ——

// wsDuviriEnvelope 双衍轮换所需字段的 WorldState 视图。
type wsDuviriEnvelope struct {
	EndlessXpSchedule []wsEndlessXpSchedule `json:"EndlessXpSchedule"`
}

// wsEndlessXpSchedule 对齐 Java model.worldstate.EndlessXpSchedule 的 JSON 字段。
type wsEndlessXpSchedule struct {
	CategoryChoices []wsEndlessXpChoice `json:"CategoryChoices"`
}

// wsEndlessXpChoice 对齐 Java EndlessXpChoices 的 JSON 字段。
type wsEndlessXpChoice struct {
	Category string   `json:"Category"`
	Choices  []string `json:"Choices"`
}

// 双衍选择分类（对齐 Java EndlessXpChoices.Category）。
const (
	duviriCategoryNormal = "EXC_NORMAL"
	duviriCategoryHard   = "EXC_HARD"
)

// GetDuviriCycle 组装双衍王境轮换（对齐 Java getDuvalierCycle）：
// 情绪由本地时间推算；选项取自 EndlessXpSchedule 首项（缺失时仅有情绪卡）。
func GetDuviriCycle() (*draw.DuvalierCycle, error) {
	computed := cycle.ComputeDuvalierCycle()
	dto := &draw.DuvalierCycle{
		State:    computed.State,
		TimeLeft: computed.TimeLeft,
	}

	env, err := parseWorldStateEnvelope[wsDuviriEnvelope]("duviri")
	if err != nil {
		// 情绪可本地推算，WorldState 不可用时仍返回情绪卡
		return dto, nil
	}
	if len(env.EndlessXpSchedule) == 0 {
		return dto, nil
	}

	for _, choice := range env.EndlessXpSchedule[0].CategoryChoices {
		dto.Choices = append(dto.Choices, draw.DuviriChoice{
			Category: duviriCategory(choice.Category),
			Choices:  translateDuviriChoices(choice.Category, choice.Choices),
		})
	}
	return dto, nil
}

// duviriCategory 分类字符串 → 绘图枚举（未命中按普通处理）。
func duviriCategory(category string) draw.DuviriCategory {
	if category == duviriCategoryHard {
		return draw.DuviriHard
	}
	return draw.DuviriNormal
}

// translateDuviriChoices 翻译双衍选项（对齐 Java translateDuvalierCycle：
// 仅钢铁分类按 weapons 表英文名译中文，普通分类原样保留）。
func translateDuviriChoices(category string, choices []string) []string {
	if category != duviriCategoryHard {
		return choices
	}
	translated := make([]string, 0, len(choices))
	for _, choice := range choices {
		translated = append(translated, weaponChineseName(choice))
	}
	return translated
}

// weaponChineseName 按英文名查 weapons 表中文名（未命中回退原文）。
func weaponChineseName(englishName string) string {
	if englishName == "" || database.DB == nil {
		return englishName
	}
	var weapon modelwarframe.Weapons
	if err := database.DB.Where("english_name = ?", englishName).First(&weapon).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			logging.DebugPack("warframe.duviri", "query weapon %q failed: %v", englishName, err)
		}
		return englishName
	}
	if weapon.Name == "" {
		return englishName
	}
	return weapon.Name
}

// —— 电波 ——

// wsSeasonEnvelope 电波所需字段的 WorldState 视图。
type wsSeasonEnvelope struct {
	SeasonInfo *wsSeasonInfo `json:"SeasonInfo"`
}

// wsSeasonInfo 对齐 Java model.worldstate.SeasonInfo 的 JSON 字段。
type wsSeasonInfo struct {
	Season           int                 `json:"Season"`
	Phase            int                 `json:"Phase"`
	ActiveChallenges []wsActiveChallenge `json:"ActiveChallenges"`
}

// wsActiveChallenge 对齐 Java SeasonInfo.ActiveChallenges 的 JSON 字段。
// daily/weekly/elite 由载荷直接给出（night_wave 表只存名称/描述/声望/次数）。
type wsActiveChallenge struct {
	Challenge string `json:"Challenge"`
	Daily     bool   `json:"Daily"`
	Weekly    bool   `json:"Weekly"`
	Elite     bool   `json:"Elite"`
}

// GetSeasonInfo 解析电波（对齐 Java WorldStateUtils.getSeasonInfo）：
// 按 Challenge 关联 night_wave 表补全名称/描述/声望，标记位取自载荷。
func GetSeasonInfo() (*draw.SeasonInfo, error) {
	env, err := parseWorldStateEnvelope[wsSeasonEnvelope]("season")
	if err != nil {
		return nil, err
	}
	if env.SeasonInfo == nil {
		return &draw.SeasonInfo{}, nil
	}

	dto := &draw.SeasonInfo{
		Season: env.SeasonInfo.Season,
		Phase:  env.SeasonInfo.Phase,
	}
	for i := range env.SeasonInfo.ActiveChallenges {
		dto.ActiveChallenges = append(dto.ActiveChallenges, translateActiveChallenge(&env.SeasonInfo.ActiveChallenges[i]))
	}
	return dto, nil
}

// translateActiveChallenge 翻译单条电波挑战（对齐 Java getSeasonInfo 的 peek 逻辑：
// 未命中 night_wave 表时仅保留标记位，绘制层会跳过空名称挑战）。
func translateActiveChallenge(challenge *wsActiveChallenge) *draw.ActiveChallenge {
	dto := &draw.ActiveChallenge{
		Daily:  challenge.Daily,
		Weekly: challenge.Weekly,
		Elite:  challenge.Elite,
	}
	record := nightWaveByChallenge(challenge.Challenge)
	if record == nil {
		return dto
	}
	dto.Name = record.Name
	// 入库保留 |COUNT| 占位，展示时替换为所需次数（对齐 Java getter 行为）
	dto.Description = strings.ReplaceAll(record.Description, "|COUNT|", strconv.Itoa(record.Required))
	dto.Standing = strconv.Itoa(record.Standing)
	return dto
}

// nightWaveByChallenge 按 uniqueName 查 night_wave 表（未命中返回 nil）。
func nightWaveByChallenge(uniqueName string) *modelwarframe.NightWave {
	if uniqueName == "" || database.DB == nil {
		return nil
	}
	var record modelwarframe.NightWave
	if err := database.DB.Where("unique_name = ?", uniqueName).First(&record).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			logging.DebugPack("warframe.nightwave", "query night wave %q failed: %v", uniqueName, err)
		}
		return nil
	}
	return &record
}

// —— 1999 日历 ——

// wsCalendarEnvelope 1999 日历所需字段的 WorldState 视图。
type wsCalendarEnvelope struct {
	KnownCalendarSeasons []wsCalendarSeason `json:"KnownCalendarSeasons"`
}

// wsCalendarSeason 对齐 Java KnownCalendarSeasons 的 JSON 字段。
type wsCalendarSeason struct {
	Season        string          `json:"Season"`
	YearIteration int             `json:"YearIteration"`
	Version       int             `json:"Version"`
	Days          []wsCalendarDay `json:"Days"`
}

// wsCalendarDay 对齐 Java KnownCalendarSeasons.Days 的 JSON 字段。
// day 为「一年中的第几天」，需换算为月/日（对齐 Java processDays）。
type wsCalendarDay struct {
	Day    int               `json:"day"`
	Month  int               `json:"month"`
	Events []wsCalendarEvent `json:"events"`
}

// wsCalendarEvent 对齐 Java KnownCalendarSeasons.Events 的 JSON 字段。
type wsCalendarEvent struct {
	Type      string `json:"type"`
	Challenge string `json:"challenge"`
	Reward    string `json:"reward"`
	Upgrade   string `json:"upgrade"`
}

// calendarMonthDays 月份天数表（对齐 Java processDays 的 monthDays，不做闰年修正）。
var calendarMonthDays = []int{31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}

// calendarSeasonNames 季节枚举 → 中文（对齐 Java SeasonEnum.name）。
var calendarSeasonNames = map[string]string{
	"CST_FALL":   "秋季",
	"CST_SUMMER": "夏季",
	"CST_SPRING": "春季",
	"CST_WINTER": "冬季",
}

// GetKnownCalendarSeasons 解析 1999 日历（对齐 Java getKnownCalendarSeasons）：
// 把 Days 的「一年中第几天」换算为月/日并按月分组，事件按类型查 state_translation 翻译。
func GetKnownCalendarSeasons() ([]*draw.KnownCalendarSeasons, error) {
	env, err := parseWorldStateEnvelope[wsCalendarEnvelope]("calendar")
	if err != nil {
		return nil, err
	}
	if len(env.KnownCalendarSeasons) == 0 {
		return nil, errors.New("1999 日历数据为空")
	}

	list := make([]*draw.KnownCalendarSeasons, 0, len(env.KnownCalendarSeasons))
	for i := range env.KnownCalendarSeasons {
		list = append(list, translateCalendarSeason(&env.KnownCalendarSeasons[i]))
	}
	return list, nil
}

// translateCalendarSeason 翻译单个季节：换算月/日、按月分组、翻译事件字段。
func translateCalendarSeason(season *wsCalendarSeason) *draw.KnownCalendarSeasons {
	dto := &draw.KnownCalendarSeasons{
		Season:        calendarSeasonName(season.Season),
		YearIteration: season.YearIteration,
		Version:       strconv.Itoa(season.Version),
		MonthDays:     map[int][]*draw.CalendarDay{},
	}

	days := make([]*draw.CalendarDay, 0, len(season.Days))
	for i := range season.Days {
		days = append(days, translateCalendarDay(&season.Days[i]))
	}
	// 对齐 Java：按 月 → 日 升序，再按月分组
	sort.SliceStable(days, func(i, j int) bool {
		if days[i].Month != days[j].Month {
			return days[i].Month < days[j].Month
		}
		return days[i].Day < days[j].Day
	})
	for _, day := range days {
		dto.MonthDays[day.Month] = append(dto.MonthDays[day.Month], day)
	}
	return dto
}

// translateCalendarDay 换算「一年中第几天」为月/日并翻译事件。
func translateCalendarDay(day *wsCalendarDay) *draw.CalendarDay {
	month, dayOfMonth := dayOfYearToMonthDay(day.Day)
	dto := &draw.CalendarDay{Month: month, Day: dayOfMonth}
	for i := range day.Events {
		dto.Events = append(dto.Events, translateCalendarEvent(&day.Events[i]))
	}
	return dto
}

// dayOfYearToMonthDay 把一年中第几天换算为 (月, 日)（对齐 Java processDays 的累减循环）。
func dayOfYearToMonthDay(dayOfYear int) (int, int) {
	remaining := dayOfYear
	month := 0
	for month < len(calendarMonthDays) && remaining > calendarMonthDays[month] {
		remaining -= calendarMonthDays[month]
		month++
	}
	return month + 1, remaining
}

// translateCalendarEvent 按事件类型翻译对应字段（对齐 Java processEvent）：
// 挑战与加成都查 state_translation 取名，奖励额外去除「<...>」标记。
func translateCalendarEvent(event *wsCalendarEvent) *draw.CalendarEvent {
	dto := &draw.CalendarEvent{
		Type:      calendarEventType(event.Type),
		Challenge: event.Challenge,
		Reward:    event.Reward,
		Upgrade:   event.Upgrade,
	}
	switch event.Type {
	case "CET_CHALLENGE":
		dto.Challenge = TranslateStateNameDirect(event.Challenge)
	case "CET_REWARD":
		dto.Reward = deleteBetweenMarkers(TranslateStateNameDirect(event.Reward), '<', '>')
	case "CET_UPGRADE":
		dto.Upgrade = TranslateStateNameDirect(event.Upgrade)
	}
	return dto
}

// calendarEventType 事件类型字符串 → 绘图枚举（对齐 Java DaysTypeEnum 序数）。
func calendarEventType(eventType string) draw.CalendarEventType {
	switch eventType {
	case "CET_REWARD":
		return draw.CETReward
	case "CET_UPGRADE":
		return draw.CETUpgrade
	default:
		return draw.CETChallenge
	}
}

// calendarSeasonName 季节枚举 → 中文（未命中回退原文）。
func calendarSeasonName(season string) string {
	if name, ok := calendarSeasonNames[season]; ok {
		return name
	}
	return season
}

// deleteBetweenMarkers 删除成对标记及其包裹内容（对齐 Java StringUtils.deleteBetweenAndMarkers）。
// 未找到成对标记时原样返回。
func deleteBetweenMarkers(text string, open, close rune) string {
	start := strings.IndexRune(text, open)
	if start < 0 {
		return text
	}
	openWidth := len(string(open))
	rest := text[start+openWidth:]
	offset := strings.IndexRune(rest, close)
	if offset < 0 {
		return text
	}
	closeWidth := len(string(close))
	return text[:start] + rest[offset+closeWidth:]
}
