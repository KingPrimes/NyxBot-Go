// draw-image-plugin 集团 & 遗物等级 & 极性 & 稀有度 枚举
package drawplugin

type SyndicateInfo struct {
	Name string
	Icon string
}

type Syndicate string

const (
	SyndArbiters       Syndicate = "ArbitersSyndicate"
	SyndCephalonSuda   Syndicate = "CephalonSudaSyndicate"
	SyndNewLoka        Syndicate = "NewLokaSyndicate"
	SyndPerrin         Syndicate = "PerrinSyndicate"
	SyndSteelMeridian  Syndicate = "SteelMeridianSyndicate"
	SyndRedVeil        Syndicate = "RedVeilSyndicate"
	SyndCetus          Syndicate = "CetusSyndicate"
	SyndQuills         Syndicate = "QuillsSyndicate"
	SyndSolaris        Syndicate = "SolarisSyndicate"
	SyndVox            Syndicate = "VoxSyndicate"
	SyndVentKids       Syndicate = "VentKidsSyndicate"
	SyndEntrati        Syndicate = "EntratiSyndicate"
	SyndEntratiLab     Syndicate = "EntratiLabSyndicate"
	SyndHex            Syndicate = "HexSyndicate"
	SyndNecraloid      Syndicate = "NecraloidSyndicate"
	SyndKahl           Syndicate = "KahlSyndicate"
	SyndZariman        Syndicate = "ZarimanSyndicate"
)

var SyndicateMap = map[Syndicate]SyndicateInfo{
	SyndArbiters:      {"均衡仲裁者", "\ue500"},
	SyndCephalonSuda:  {"中枢苏达", "\ue501"},
	SyndNewLoka:       {"新世间", "\ue502"},
	SyndPerrin:        {"佩兰数列", "\ue505"},
	SyndSteelMeridian: {"钢铁防线", "\ue504"},
	SyndRedVeil:       {"血色面纱", "\ue503"},
	SyndCetus:         {"Ostron", "\ue508"},
	SyndQuills:        {"夜羽", "\ue509"},
	SyndSolaris:       {"索拉里斯联盟", "\ue510"},
	SyndVox:           {"索拉里斯之声", "\ue512"},
	SyndVentKids:      {"通风小子", "\ue511"},
	SyndEntrati:       {"英择谛", "\ue514"},
	SyndEntratiLab:    {"科维兽", "\ue517"},
	SyndHex:           {"六人组", "\ue519"},
	SyndNecraloid:     {"殁世械灵", "\ue515"},
	SyndKahl:          {"卡尔驻军", "\ue518"},
	SyndZariman:       {"坚守者", "\ue516"},
}

// VoidEnum 遗物等级
type VoidTier string

const (
	VoidT1 VoidTier = "VoidT1"
	VoidT2 VoidTier = "VoidT2"
	VoidT3 VoidTier = "VoidT3"
	VoidT4 VoidTier = "VoidT4"
	VoidT5 VoidTier = "VoidT5"
	VoidT6 VoidTier = "VoidT6"
)

var VoidTierMap = map[VoidTier]string{
	VoidT1: "古纪", VoidT2: "前纪", VoidT3: "中纪",
	VoidT4: "后纪", VoidT5: "安魂", VoidT6: "全能",
}

// PolarityEnum 极性
type Polarity string

const (
	PolarityMadurai Polarity = "MADURAI"
	PolarityVazarin Polarity = "VAZARIN"
	PolarityNaramon Polarity = "NARAMON"
	PolarityZenurik Polarity = "ZENURIK"
	PolarityUnairu  Polarity = "UNAIRU"
	PolarityPenjaga Polarity = "PENJAGA"
	PolarityUmbra   Polarity = "UMBRA"
	PolarityAny     Polarity = "Any"
)

var PolarityIconMap = map[Polarity]string{
	PolarityMadurai: "\ue200", PolarityVazarin: "\ue202",
	PolarityNaramon: "\ue203", PolarityZenurik: "\ue204",
	PolarityUnairu: "\ue205", PolarityPenjaga: "\ue206",
	PolarityUmbra: "\ue207", PolarityAny: "",
}

// Rarity 稀有度
type Rarity string

const (
	RarityCommon   Rarity = "COMMON"
	RarityUncommon Rarity = "UNCOMMON"
	RarityRare     Rarity = "RARE"
	RarityLegendary Rarity = "LEGENDARY"
)

var RarityMap = map[Rarity]string{
	RarityCommon: "常见", RarityUncommon: "罕见",
	RarityRare: "稀有", RarityLegendary: "传奇",
}

// RivenTrend 紫卡倾向（视觉显示）
type RivenTrend string

const (
	RivenTrend1 RivenTrend = "RIVEN_TREND_1"
	RivenTrend2 RivenTrend = "RIVEN_TREND_2"
	RivenTrend3 RivenTrend = "RIVEN_TREND_3"
	RivenTrend4 RivenTrend = "RIVEN_TREND_4"
	RivenTrend5 RivenTrend = "RIVEN_TREND_5"
)

func (r RivenTrend) Doc() string {
	mp := map[RivenTrend]string{
		RivenTrend1: "●○○○○", RivenTrend2: "●●○○○",
		RivenTrend3: "●●●○○", RivenTrend4: "●●●●○",
		RivenTrend5: "●●●●●",
	}
	return mp[r]
}

type Icon string

const (
	IconSmile    Icon = "SMILE"
	IconMeh      Icon = "MEH"
	IconCubes    Icon = "CUBES"
	IconDucats   Icon = "DUCATS"
	IconCredits  Icon = "CREDITS"
	IconAyan     Icon = "AYAN"
	IconPlatinum Icon = "PLATINUM"
	IconRefresh   Icon = "REFRESH"
	IconCold     Icon = "COLD"
	IconSun      Icon = "SUN"
	IconNight    Icon = "NIGHT"
)

var IconMap = map[Icon]string{
	IconSmile: "\ue300", IconMeh: "\ue302", IconCubes: "\ue303",
	IconDucats: "\ue304", IconCredits: "\ue305", IconAyan: "\ue306",
	IconPlatinum: "\ue307", IconRefresh: "\ue308", IconCold: "\ue309",
	IconSun: "\ue30a", IconNight: "\ue30b",
}
