// NyxBot Warframe 相关枚举
package nyxbot

// MissionType 订阅任务类型（27 种）
type MissionType string

const (
	MTExtermination  MissionType = "MT_EXTERMINATION"
	MTSurvival       MissionType = "MT_SURVIVAL"
	MTRescue         MissionType = "MT_RESCUE"
	MTSabotage       MissionType = "MT_SABOTAGE"
	MTCapture        MissionType = "MT_CAPTURE"
	MTIntel          MissionType = "MT_INTEL"
	MTDefense        MissionType = "MT_DEFENSE"
	MTMobileDefense  MissionType = "MT_MOBILE_DEFENSE"
	MTTerritory      MissionType = "MT_TERRITORY"
	MTHive           MissionType = "MT_HIVE"
	MTRetrieval      MissionType = "MT_RETRIEVAL"
	MTExcavate       MissionType = "MT_EXCAVATE"
	MTSalvage        MissionType = "MT_SALVAGE"
	MTPursuit        MissionType = "MT_PURSUIT"
	MTAssault        MissionType = "MT_ASSAULT"
	MTEvacuation     MissionType = "MT_EVACUATION"
	MTDisruption     MissionType = "MT_DISRUPTION"
	MTVoidFlood      MissionType = "MT_VOID_FLOOD"
	MTVoidCascade    MissionType = "MT_VOID_CASCADE"
	MTVoidArmageddon MissionType = "MT_VOID_ARMAGEDDON"
	MTAlchemy        MissionType = "MT_ALCHEMY"
	MTCambire        MissionType = "MT_CAMBIRE"
	MTSkirmish       MissionType = "MT_SKIRMISH"
	MTVolatile       MissionType = "MT_VOLATILE"
	MTOrpheus        MissionType = "MT_ORPHEUS"
	MTAscension      MissionType = "MT_ASCENSION"
	MTCorruption     MissionType = "MT_CORRUPTION"
)

// Name 返回任务类型的中文名称。
func (m MissionType) Name() string {
	mp := map[MissionType]string{
		MTExtermination: "歼灭", MTSurvival: "生存", MTRescue: "救援",
		MTSabotage: "破坏", MTCapture: "捕获", MTIntel: "间谍",
		MTDefense: "防御", MTMobileDefense: "移动防御", MTTerritory: "拦截",
		MTHive: "清巢", MTRetrieval: "劫持", MTExcavate: "挖掘",
		MTSalvage: "资源回收", MTPursuit: "追击", MTAssault: "强袭",
		MTEvacuation: "叛逃", MTDisruption: "中断", MTVoidFlood: "虚空洪流",
		MTVoidCascade: "虚空覆涌", MTVoidArmageddon: "虚空决战",
		MTAlchemy: "元素转换", MTCambire: "异化区", MTSkirmish: "前哨战",
		MTVolatile: "爆发", MTOrpheus: "奧菲斯", MTAscension: "扬升",
		MTCorruption: "虚空腐蚀",
	}
	return mp[m]
}

// SubscribeType 订阅类型（14 种）
type SubscribeType string

const (
	SubAlerts      SubscribeType = "ALERTS"
	SubArbitration SubscribeType = "ARBITRATION"
	SubCetusCycle  SubscribeType = "CETUS_CYCLE"
	SubDailyDeals  SubscribeType = "DAILY_DEALS"
	SubEvents      SubscribeType = "EVENTS"
	SubInvasions   SubscribeType = "INVASIONS"
	SubSteelPath   SubscribeType = "STEEL_PATH"
	SubVoid        SubscribeType = "VOID"
	SubFissures    SubscribeType = "FISSURES"
	SubNews        SubscribeType = "NEWS"
	SubNightwave   SubscribeType = "NIGHTWAVE"
	SubSortie      SubscribeType = "SORTIE"
	SubArchonHunt  SubscribeType = "ARCHON_HUNT"
	SubDuviriCycle SubscribeType = "DUVIRI_CYCLE"
)

// Name 返回订阅类型的中文名称。
func (s SubscribeType) Name() string {
	mp := map[SubscribeType]string{
		SubAlerts: "警报", SubArbitration: "仲裁", SubCetusCycle: "夜灵平野",
		SubDailyDeals: "每日特惠", SubEvents: "活动", SubInvasions: "入侵",
		SubSteelPath: "钢铁之路", SubVoid: "奸商", SubFissures: "裂缝",
		SubNews: "新闻", SubNightwave: "电波", SubSortie: "突击",
		SubArchonHunt: "执刑官猎杀", SubDuviriCycle: "双衍王境",
	}
	return mp[s]
}

// FissureTypeEnum 裂隙过滤类型
type FissureTypeEnum string

const (
	FissureSteelPath     FissureTypeEnum = "STEEL_PATH"
	FissureVoidStorms    FissureTypeEnum = "VOID_STORMS"
	FissureActiveMission FissureTypeEnum = "ACTIVE_MISSION"
)

// InvasionReward 入侵特殊奖励
type InvasionReward string

const (
	InvRewardNone             InvasionReward = "NONE"
	InvRewardDetoniteInjector InvasionReward = "DETONITE_INJECTOR"
	InvRewardFieldron         InvasionReward = "FIELDRON"
	InvRewardMutagenMass      InvasionReward = "MUTAGEN_MASS"
	InvRewardOrokinCatalyst   InvasionReward = "OROKIN_CATALYST"
	InvRewardOrokinReactor    InvasionReward = "OROKIN_REACTOR"
	InvRewardForma            InvasionReward = "FORMA"
	InvRewardExilusAdapter    InvasionReward = "EXILUS_ADAPTER"
)

// Name 返回入侵特殊奖励的中文名称。
func (i InvasionReward) Name() string {
	mp := map[InvasionReward]string{
		InvRewardNone:             "无",
		InvRewardDetoniteInjector: "突变原聚合物",
		InvRewardFieldron:         "力场装置样本",
		InvRewardMutagenMass:      "突变原聚合物",
		InvRewardOrokinCatalyst:   "金土豆",
		InvRewardOrokinReactor:    "银土豆",
		InvRewardForma:            "Forma",
		InvRewardExilusAdapter:    "Exilus 槽位适配器",
	}
	return mp[i]
}

// RivenTrendTypeEnum 紫卡倾向武器类型
type RivenTrendType string

const (
	RivenTrendRifle   RivenTrendType = "RIFLE"
	RivenTrendShotgun RivenTrendType = "SHOTGUN"
	RivenTrendPistol  RivenTrendType = "PISTOL"
	RivenTrendArchgun RivenTrendType = "ARCHGUN"
	RivenTrendMelee   RivenTrendType = "MELEE"
)

// Desc 返回紫卡倾向武器类型的中文描述。
func (r RivenTrendType) Desc() string {
	mp := map[RivenTrendType]string{
		RivenTrendRifle:   "步枪-狙击枪",
		RivenTrendShotgun: "霰弹枪",
		RivenTrendPistol:  "手枪",
		RivenTrendArchgun: "Archwing枪械",
		RivenTrendMelee:   "近战",
	}
	return mp[r]
}

// Value 返回紫卡倾向武器类型的枚举数值。
func (r RivenTrendType) Value() int {
	mp := map[RivenTrendType]int{
		RivenTrendRifle: 0, RivenTrendShotgun: 1,
		RivenTrendPistol: 2, RivenTrendArchgun: 3, RivenTrendMelee: 4,
	}
	return mp[r]
}
