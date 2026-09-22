// 世界状态查询：解析 Raw WorldState JSON（对齐 Java WorldStateUtils.getFissure /
// WarframeCache.getWarframeStatus），并将数据转换为 draw 绘图 DTO。
// 注意：真实 WorldState API 的 activeMissions 数组字段为 Java 命名风格（大写首字母），
// 与 model.WorldState（NoManifest 用途）不同，故此处自定义 envelope。
package warframe

import (
	"encoding/json"
	"errors"
	"sort"
	"time"

	"gorm.io/gorm"

	"nyxbot-go/internal/database"
	"nyxbot-go/internal/draw"
	"nyxbot-go/internal/enum/drawplugin"
	"nyxbot-go/internal/logging"
	modelwarframe "nyxbot-go/internal/model/warframe"
)

var (
	// ErrWorldStateNotReady 表示世界状态原始数据尚未成功拉取。
	ErrWorldStateNotReady = errors.New("世界状态数据尚未就绪，请稍后再试")
	errWorldStateParse    = errors.New("世界状态数据解析失败")
)

// wsEnvelope 对齐 Java WorldState 顶层 JSON 字段（仅声明本批次用到的数组）。
type wsEnvelope struct {
	ActiveMissions []wsActiveMission `json:"activeMissions"`
	VoidStorms     []wsVoidStorm     `json:"voidStorms"`
	Invasions      []wsInvasion      `json:"invasions"`
	VoidTraders    []wsVoidTrader    `json:"VoidTraders"`
	DailyDeals     []wsDailyDeal     `json:"DailyDeals"`
	Sorties        []wsSortie        `json:"Sorties"`
}

// wsActiveMission 对齐 Java model.ActiveMission（继承 BastWorldState）的 JSON 字段。
type wsActiveMission struct {
	ID          string `json:"_id"`
	Activation  string `json:"Activation"`
	Expiry      string `json:"Expiry"`
	MissionType string `json:"MissionType"`
	Modifier    string `json:"Modifier"`
	Node        string `json:"Node"`
	Faction     string `json:"Faction"`
	Region      int    `json:"Region"`
	Seed        int    `json:"Seed"`
	Hard        bool   `json:"Hard"`
	VoidStorms  bool   `json:"voidStorms"`
}

// wsVoidStorm 对齐 Java WorldState.voidStorms 数组元素（九重天数据源）。
type wsVoidStorm struct {
	ID         string `json:"_id"`
	Activation string `json:"Activation"`
	Expiry     string `json:"Expiry"`
	Node       string `json:"Node"`
	Tier       string `json:"Tier"`
}

// wsInvasion 对齐 Java model.worldstate.Invasion 的 JSON 字段。
type wsInvasion struct {
	ID              string     `json:"_id"`
	Activation      string     `json:"Activation"`
	Expiry          string     `json:"Expiry"`
	Faction         string     `json:"Faction"`
	DefenderFaction string     `json:"DefenderFaction"`
	Node            string     `json:"Node"`
	Count           float64    `json:"Count"`
	Goal            float64    `json:"Goal"`
	LocTag          string     `json:"LocTag"`
	Completed       bool       `json:"Completed"`
	ChainID         string     `json:"ChainID"`
	AttackerReward  []wsReward `json:"AttackerReward"`
	DefenderReward  wsReward   `json:"DefenderReward"`
}

// wsReward 对齐 Java model.worldstate.Reward 的 JSON 字段。
type wsReward struct {
	Credits      int            `json:"credits"`
	Xp           int            `json:"xp"`
	Items        []string       `json:"items"`
	CountedItems []wsRewardItem `json:"countedItems"`
}

// wsRewardItem 对齐 Java Reward.Item 的 JSON 字段。
type wsRewardItem struct {
	ItemType  string `json:"ItemType"`
	ItemCount int    `json:"ItemCount"`
}

// wsVoidTrader 对齐 Java model.worldstate.VoidTrader 的 JSON 字段。
type wsVoidTrader struct {
	ID         string       `json:"_id"`
	Activation string       `json:"Activation"`
	Expiry     string       `json:"Expiry"`
	Character  string       `json:"Character"`
	Node       string       `json:"Node"`
	Manifest   []wsManifest `json:"Manifest"`
}

// wsManifest 对齐 Java VoidTrader.Manifest 的 JSON 字段。
type wsManifest struct {
	Item         string `json:"ItemType"`
	PrimePrice   *int   `json:"PrimePrice"`
	RegularPrice *int   `json:"RegularPrice"`
	Limit        *int   `json:"Limit"`
}

// wsDailyDeal 对齐 Java model.worldstate.DailyDeals 的 JSON 字段。
type wsDailyDeal struct {
	Item          string   `json:"StoreItem"`
	Activation    string   `json:"Activation"`
	Expiry        string   `json:"Expiry"`
	Discount      *float64 `json:"Discount"`
	OriginalPrice *int     `json:"OriginalPrice"`
	SalePrice     *int     `json:"SalePrice"`
	Total         *int     `json:"AmountTotal"`
	Sold          *int     `json:"AmountSold"`
}

// wsSortie 对齐 Java model.worldstate.Sortie 的 JSON 字段。
type wsSortie struct {
	ID         string            `json:"_id"`
	Activation string            `json:"Activation"`
	Expiry     string            `json:"Expiry"`
	Boss       string            `json:"Boss"`
	Variants   []wsSortieVariant `json:"Variants"`
}

// wsSortieVariant 对齐 Java model.worldstate.Variant 的 JSON 字段。
type wsSortieVariant struct {
	MissionType  string `json:"missionType"`
	ModifierType string `json:"modifierType"`
	Node         string `json:"node"`
}

// GetActiveMissions 解析裂隙任务（普通 hard=false / 钢铁 hard=true），
// 对齐 Java WorldStateUtils.getFissure：过滤 hard → 翻译节点/派系 → 按遗物等级排序。
func GetActiveMissions(hard bool) ([]*draw.ActiveMission, error) {
	env, err := parseWorldState()
	if err != nil {
		return nil, err
	}
	missions := make([]*draw.ActiveMission, 0, 8)
	for i := range env.ActiveMissions {
		if env.ActiveMissions[i].Hard != hard {
			continue
		}
		missions = append(missions, translateActiveMission(&env.ActiveMissions[i]))
	}
	sortActiveMissions(missions)
	return missions, nil
}

// GetVoidStorms 解析九重天风暴（对齐 Java getFissure(VOID_STORMS)：
// 从 voidStorms 数组构造，节点任务类型与派系取自 nodes 表）。
func GetVoidStorms() ([]*draw.ActiveMission, error) {
	env, err := parseWorldState()
	if err != nil {
		return nil, err
	}
	missions := make([]*draw.ActiveMission, 0, len(env.VoidStorms))
	for i := range env.VoidStorms {
		missions = append(missions, voidStormToActiveMission(&env.VoidStorms[i]))
	}
	sortActiveMissions(missions)
	return missions, nil
}

// GetInvasions 解析进行中的入侵（对齐 Java WorldStateUtils.getInvasions：
// 过滤已完成 → 翻译节点与奖励物品名称）。
func GetInvasions() ([]*draw.Invasion, error) {
	env, err := parseWorldState()
	if err != nil {
		return nil, err
	}
	invasions := make([]*draw.Invasion, 0, 8)
	for i := range env.Invasions {
		if env.Invasions[i].Completed {
			continue
		}
		invasions = append(invasions, translateInvasion(&env.Invasions[i]))
	}
	return invasions, nil
}

// translateInvasion 翻译节点与奖励物品名（对齐 Java translateInvasion）。
func translateInvasion(m *wsInvasion) *draw.Invasion {
	act := &draw.Invasion{
		ID:              m.ID,
		Activation:      parseTime(m.Activation),
		Expiry:          parseTime(m.Expiry),
		Faction:         drawplugin.Faction(m.Faction),
		DefenderFaction: drawplugin.Faction(m.DefenderFaction),
		Node:            TranslateNode(m.Node),
		Count:           float64Ptr(m.Count),
		Goal:            float64Ptr(m.Goal),
		LocTag:          m.LocTag,
		Completed:       m.Completed,
		ChainID:         m.ChainID,
		DefenderReward:  translateReward(&m.DefenderReward),
	}
	for i := range m.AttackerReward {
		act.AttackerReward = append(act.AttackerReward, translateReward(&m.AttackerReward[i]))
	}
	return act
}

// translateReward 翻译带数量奖励的物品名（对齐 Java translateInvasion 的 countedItems 处理）。
func translateReward(r *wsReward) *draw.Reward {
	if r == nil {
		return nil
	}
	out := &draw.Reward{
		Credits: r.Credits,
		Xp:      r.Xp,
		Items:   r.Items,
	}
	for i := range r.CountedItems {
		count := r.CountedItems[i].ItemCount
		out.CountedItems = append(out.CountedItems, &draw.RewardItem{
			Name:  TranslateStateName(r.CountedItems[i].ItemType),
			Count: &count,
		})
	}
	return out
}

// GetVoidTraders 解析虚空商人（对齐 Java WorldStateUtils.getVoidTraders：
// 翻译节点与商品名，保留 manifest 排序）。
func GetVoidTraders() ([]*draw.VoidTrader, error) {
	env, err := parseWorldState()
	if err != nil {
		return nil, err
	}
	traders := make([]*draw.VoidTrader, 0, len(env.VoidTraders))
	for i := range env.VoidTraders {
		traders = append(traders, translateVoidTrader(&env.VoidTraders[i]))
	}
	return traders, nil
}

// translateVoidTrader 翻译虚空商人节点与商品名（对齐 Java translateVoidTraders）。
func translateVoidTrader(v *wsVoidTrader) *draw.VoidTrader {
	act := &draw.VoidTrader{
		ID:        v.ID,
		Character: v.Character,
		Node:      TranslateNode(v.Node),
		Expiry:    parseTime(v.Expiry),
	}
	for i := range v.Manifest {
		item := &draw.VoidTraderItem{
			Item:         TranslateStateName(v.Manifest[i].Item),
			PrimePrice:   v.Manifest[i].PrimePrice,
			RegularPrice: v.Manifest[i].RegularPrice,
			Limit:        v.Manifest[i].Limit,
		}
		act.Manifest = append(act.Manifest, item)
	}
	return act
}

// GetDailyDeals 解析每日特惠（对齐 Java WorldStateUtils.getDailyDeals：
// 翻译物品名，保留列表顺序）。
func GetDailyDeals() ([]*draw.DailyDeals, error) {
	env, err := parseWorldState()
	if err != nil {
		return nil, err
	}
	deals := make([]*draw.DailyDeals, 0, len(env.DailyDeals))
	for i := range env.DailyDeals {
		deals = append(deals, translateDailyDeal(&env.DailyDeals[i]))
	}
	return deals, nil
}

// translateDailyDeal 翻译每日特惠物品名（对齐 Java translateDailyDeals）。
func translateDailyDeal(d *wsDailyDeal) *draw.DailyDeals {
	return &draw.DailyDeals{
		Item:          TranslateStateName(d.Item),
		OriginalPrice: d.OriginalPrice,
		SalePrice:     d.SalePrice,
		Count:         d.Discount,
		Total:         d.Total,
		Sold:          d.Sold,
		Expiry:        parseTime(d.Expiry),
	}
}

// GetSorties 解析突击任务（对齐 Java WorldStateUtils.getSorties：
// 翻译 variants 节点，Boss 由枚举映射中文）。
func GetSorties() ([]*draw.Sortie, error) {
	env, err := parseWorldState()
	if err != nil {
		return nil, err
	}
	sorties := make([]*draw.Sortie, 0, len(env.Sorties))
	for i := range env.Sorties {
		sorties = append(sorties, translateSortie(&env.Sorties[i]))
	}
	return sorties, nil
}

// translateSortie 翻译突击任务（节点 + Boss 名）。
func translateSortie(s *wsSortie) *draw.Sortie {
	act := &draw.Sortie{
		Boss:   bossName(s.Boss),
		Expiry: parseTime(s.Expiry),
	}
	for i := range s.Variants {
		act.Variants = append(act.Variants, &draw.SortieVariant{
			MissionType:  drawplugin.MissionType(s.Variants[i].MissionType),
			ModifierType: drawplugin.ModifierType(s.Variants[i].ModifierType),
			Node:         TranslateNode(s.Variants[i].Node),
		})
	}
	return act
}

// bossName Boss 枚举键 → 中文名（对齐 Java BossEnum.name，未命中回退原文）。
func bossName(key string) string {
	if info, ok := drawplugin.BossMap[drawplugin.Boss(key)]; ok {
		return info.Name
	}
	return key
}

// parseWorldState 从默认缓存读取原始 JSON 并解析 envelope。
func parseWorldState() (*wsEnvelope, error) {
	return parseWorldStateEnvelope[wsEnvelope]("world state")
}

// parseWorldStateEnvelope 从默认缓存解析指定的 WorldState 视图。
// 各查询只声明自身所需字段，但共享相同的就绪检查、错误处理与日志格式。
func parseWorldStateEnvelope[T any](name string) (*T, error) {
	raw := DefaultWorldState().Raw()
	if len(raw) == 0 {
		return nil, ErrWorldStateNotReady
	}
	var env T
	if err := json.Unmarshal(raw, &env); err != nil {
		logging.WarnPack("warframe.status", "parse %s envelope failed: %v", name, err)
		return nil, errWorldStateParse
	}
	return &env, nil
}

// translateActiveMission 翻译节点与派系（对齐 Java translateActiveMission，
// 仅覆盖 node/faction，其余字段透传）。
func translateActiveMission(m *wsActiveMission) *draw.ActiveMission {
	act := &draw.ActiveMission{
		ID:          m.ID,
		Activation:  parseTime(m.Activation),
		Expiry:      parseTime(m.Expiry),
		MissionType: drawplugin.MissionType(m.MissionType),
		Modifier:    drawplugin.VoidTier(m.Modifier),
		Node:        TranslateNode(m.Node),
		Region:      m.Region,
		Seed:        m.Seed,
		Hard:        m.Hard,
		VoidStorms:  m.VoidStorms,
	}
	if act.Node == m.Node && m.Faction != "" {
		act.Faction = drawplugin.Faction(m.Faction)
	} else {
		act.Faction = nodeFactionByKey(m.Node)
	}
	return act
}

// voidStormToActiveMission 构造九重天任务（对齐 Java VOID_STORMS 分支）。
func voidStormToActiveMission(v *wsVoidStorm) *draw.ActiveMission {
	return &draw.ActiveMission{
		ID:          v.ID,
		Activation:  parseTime(v.Activation),
		Expiry:      parseTime(v.Expiry),
		MissionType: nodeMissionTypeByKey(v.Node),
		Modifier:    drawplugin.VoidTier(v.Tier),
		Node:        TranslateNode(v.Node),
		Faction:     nodeFactionByKey(v.Node),
		VoidStorms:  true,
	}
}

// sortActiveMissions 按遗物等级排序（对齐 Java sorted(comparing(getModifier))）。
func sortActiveMissions(missions []*draw.ActiveMission) {
	sort.SliceStable(missions, func(i, j int) bool {
		return voidTierOrder(missions[i].Modifier) < voidTierOrder(missions[j].Modifier)
	})
}

// voidTierOrder 返回遗物等级的展示顺序（VoidT1 最小，未知名次回退最大）。
func voidTierOrder(tier drawplugin.VoidTier) int {
	switch tier {
	case drawplugin.VoidT1:
		return 1
	case drawplugin.VoidT2:
		return 2
	case drawplugin.VoidT3:
		return 3
	case drawplugin.VoidT4:
		return 4
	case drawplugin.VoidT5:
		return 5
	case drawplugin.VoidT6:
		return 6
	default:
		return 99
	}
}

// nodeFactionByKey 查询节点表派系序号并映射为枚举（对齐 Java Nodes.getFactionName）。
func nodeFactionByKey(nodeKey string) drawplugin.Faction {
	if nodeKey == "" || database.DB == nil {
		return drawplugin.FactionNone
	}
	var node modelwarframe.Nodes
	if err := database.DB.Where("unique_name = ?", nodeKey).First(&node).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			logging.DebugPack("warframe.translate", "query node faction %q failed: %v", nodeKey, err)
		}
		return drawplugin.FactionNone
	}
	return factionFromIndex(node.FactionIndex)
}

// nodeMissionTypeByKey 查询节点表任务类型序号并映射为枚举（对齐 Java Nodes.getMissionType）。
func nodeMissionTypeByKey(nodeKey string) drawplugin.MissionType {
	if nodeKey == "" || database.DB == nil {
		return drawplugin.MTDefault
	}
	var node modelwarframe.Nodes
	if err := database.DB.Where("unique_name = ?", nodeKey).First(&node).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			logging.DebugPack("warframe.translate", "query node missionType %q failed: %v", nodeKey, err)
		}
		return drawplugin.MTDefault
	}
	return missionFromIndex(node.MissionIndex)
}

// factionFromIndex 派系序号 → 枚举（对齐 Java Nodes.getFactionName 的 switch）。
func factionFromIndex(index int) drawplugin.Faction {
	switch index {
	case 0:
		return drawplugin.FactionGrineer
	case 1:
		return drawplugin.FactionCorpus
	case 2:
		return drawplugin.FactionInfest
	case 3:
		return drawplugin.FactionOrokin
	case 4:
		return drawplugin.FactionCorrupted
	case 5:
		return drawplugin.FactionSentient
	case 6:
		return drawplugin.FactionNarmer
	case 7:
		return drawplugin.FactionMurmur
	case 8:
		return drawplugin.FactionScaldra
	case 9:
		return drawplugin.FactionTechrot
	case 10:
		return drawplugin.FactionDuviri
	case 11:
		return drawplugin.FactionMitw
	case 12:
		return drawplugin.FactionTenno
	default:
		return drawplugin.FactionNone
	}
}

// missionFromIndex 任务类型序号 → 枚举（对齐 Java Nodes.getMissionType 的 switch）。
func missionFromIndex(index int) drawplugin.MissionType {
	switch index {
	case 0:
		return drawplugin.MTAssassination
	case 1:
		return drawplugin.MTExtermination
	case 2:
		return drawplugin.MTSurvival
	case 3:
		return drawplugin.MTRescue
	case 4:
		return drawplugin.MTSabotage
	case 5:
		return drawplugin.MTCapture
	case 7:
		return drawplugin.MTIntel
	case 8:
		return drawplugin.MTDefense
	case 9:
		return drawplugin.MTMobileDefense
	case 10:
		return drawplugin.MTPvp
	case 11:
		return drawplugin.MTSector
	case 13:
		return drawplugin.MTTerritory
	case 14:
		return drawplugin.MTRetrieval
	case 15:
		return drawplugin.MTHive
	case 17:
		return drawplugin.MTExcavate
	case 21:
		return drawplugin.MTSalvage
	case 22:
		return drawplugin.MTArena
	case 24, 25:
		return drawplugin.MTPursuit
	case 26:
		return drawplugin.MTAssault
	case 27:
		return drawplugin.MTEvacuation
	case 28, 31:
		return drawplugin.MTLandscape
	case 32, 33:
		return drawplugin.MTArtifact
	case 34:
		return drawplugin.MTVoidFlood
	case 35:
		return drawplugin.MTVoidCascade
	case 36:
		return drawplugin.MTVoidArmageddon
	case 38:
		return drawplugin.MTAlchemy
	case 39:
		return drawplugin.MTCambire
	case 40:
		return drawplugin.MTLegacyteHarvest
	case 41:
		return drawplugin.MTShrineDefense
	case 42:
		return drawplugin.MTFaceoff
	case 60:
		return drawplugin.MTSkirmish
	case 61:
		return drawplugin.MTVolatile
	case 62:
		return drawplugin.MTOrpheus
	case 90:
		return drawplugin.MTAscension
	case 100:
		return drawplugin.MTRelay
	default:
		return drawplugin.MTDefault
	}
}

// parseTime 解析世界状态时间戳（RFC3339），失败返回零值。
func parseTime(value string) time.Time {
	if value == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}
	}
	return t
}

// float64Ptr 返回 float64 指针（对齐 Java 可空 Double）。
func float64Ptr(value float64) *float64 {
	return &value
}
