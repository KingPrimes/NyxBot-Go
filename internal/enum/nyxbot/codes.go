// NyxBot Codes 指令枚举 —— 指令 -> 所需权限映射
// CodeInfo.Comm 为指令匹配正则（Java shiro 沿用而来），阶段 7 将经 zero.OnRegex 注册：
// ZeroBot RegexRule 使用 Go RE2 的 FindStringSubmatch（find 语义），与 Java shiro
// MessageHandlerFilter 的 Matcher.find 完全一致，故全部模式无需改写；
// 匹配结果（含捕获组）存于 ctx.State["regex_matched"]。RE2 兼容性由 tests/command_match_test.go 守护。
package nyxbot

type CodeInfo struct {
	Comm        string
	Permissions PermissionsEnums
}

type Codes string

const (
	CmdHelp                     Codes = "HELP"
	CmdCheckVersion             Codes = "CHECK_VERSION"
	CmdUpdateWfResMarketItems   Codes = "UPDATE_WARFRAME_RES_MARKET_ITEMS"
	CmdUpdateWfResMarketRiven   Codes = "UPDATE_WARFRAME_RES_MARKET_RIVEN"
	CmdUpdateWfSister           Codes = "UPDATE_WARFRAME_SISTER"
	CmdUpdateWfTar              Codes = "UPDATE_WARFRAME_TAR"
	CmdWfAlerts                 Codes = "WARFRAME_ALERTS_PLUGIN"
	CmdWfSorties                Codes = "WARFRAME_SORTIES_PLUGIN"
	CmdWfLiteSortie             Codes = "WARFRAME_LITE_SORITE_PLUGIN"
	CmdWfVoid                   Codes = "WARFRAME_VOID_PLUGIN"
	CmdWfArbitrationEx          Codes = "WARFRAME_ARBITRATION_EX_PLUGIN"
	CmdWfArbitration            Codes = "WARFRAME_ARBITRATION_PLUGIN"
	CmdWfDailyDeals             Codes = "WARFRAME_DAILY_DEALS_PLUGIN"
	CmdWfInvasions              Codes = "WARFRAME_INVASIONS_PLUGIN"
	CmdWfActiveMission          Codes = "WARFRAME_ACTIVE_MISSION_PLUGIN"
	CmdWfVoidStorms             Codes = "WARFRAME_VOID_STORMS_PLUGIN"
	CmdWfActiveMissionPath      Codes = "WARFRAME_ACTIVE_MISSION_PATH_PLUGIN"
	CmdWfSteelPath              Codes = "WARFRAME_STEEL_PATH_PLUGIN"
	CmdWfAllCycle               Codes = "WARFRAME_ALL_CYCLE_PLUGIN"
	CmdWfSyndicateOstrons       Codes = "WARFRAME_SYNDICATE_OSTRONS"
	CmdWfSyndicateEntrati       Codes = "WARFRAME_SYNDICATE_ENTRATI"
	CmdWfSyndicateSolarisUnited Codes = "WARFRAME_SYNDICATE_SOLARIS_UNITED"
	CmdWfDuviriCycle            Codes = "WARFRAME_DUVIRI_CYCLE"
	CmdWfNightWave              Codes = "WARFRAME_NIGH_WAVE_PLUGIN"
	CmdWfRivenDisUpdate         Codes = "WARFRAME_RIVEN_DIS_UPDATE_PLUGIN"
	CmdWfTra                    Codes = "WARFRAME_TRA_PLUGIN"
	CmdWfMarketRiven            Codes = "WARFRAME_MARKET_RIVEN_PLUGIN"
	CmdWfMarketOrders           Codes = "WARFRAME_MARKET_ORDERS_PLUGIN"
	CmdWfRivenMarket            Codes = "WARFRAME_RIVEN_MARKET_PLUGIN"
	CmdWfLichs                  Codes = "WARFRAME_LICHES_PLUGIN"
	CmdWfSisters                Codes = "WARFRAME_SISTERS_PLUGIN"
	CmdWfPerlinSequence         Codes = "WARFRAME_THE_PERLIN_SEQUENCE_PLUGIN"
	CmdWfMarketGodDump          Codes = "WARFRAME_MARKET_GOD_DUMP"
	CmdWfMarketSilverDump       Codes = "WARFRAME_MARKET_SILVER_DUMP"
	CmdWfRelics                 Codes = "WARFRAME_RELICS_PLUGIN"
	CmdWfOpenRelics             Codes = "WARFRAME_OPEN_RELICS_PLUGIN"
	CmdWfRivenAnalyse           Codes = "WARFRAME_RIVEN_ANALYSE"
	CmdWfSubscribe              Codes = "WARFRAME_SUBSCRIBE"
	CmdWfUnsubscribe            Codes = "WARFRAME_UNSUBSCRIBE"
	CmdWfKnownCalendarSeasons   Codes = "WARFRAME_KNOWN_CALENDAR_SEASONS_PLUGIN"
)

var CodesInfo = map[Codes]CodeInfo{
	CmdHelp:                     {Comm: `^(帮助|指令|命令|菜单|help|HELP)$`, Permissions: PermUser},
	CmdCheckVersion:             {Comm: `^(检查版本|版本|运行状态|状态)$`, Permissions: PermUser},
	CmdUpdateWfResMarketItems:   {Comm: `^更新WM物品$`, Permissions: PermSuperAdmin},
	CmdUpdateWfResMarketRiven:   {Comm: `^更新WM紫卡$`, Permissions: PermSuperAdmin},
	CmdUpdateWfSister:           {Comm: `^更新信条$`, Permissions: PermSuperAdmin},
	CmdUpdateWfTar:              {Comm: `^更新翻译$`, Permissions: PermSuperAdmin},
	CmdWfAlerts:                 {Comm: `^警报$`, Permissions: PermUser},
	CmdWfSorties:                {Comm: `^突击$`, Permissions: PermUser},
	CmdWfLiteSortie:             {Comm: `^(执刑官猎杀|猎杀|执行官|执政官|执刑官)$`, Permissions: PermUser},
	CmdWfVoid:                   {Comm: `^奸商$`, Permissions: PermUser},
	CmdWfArbitrationEx:          {Comm: `^仲裁表$`, Permissions: PermUser},
	CmdWfArbitration:            {Comm: `^仲裁$`, Permissions: PermUser},
	CmdWfDailyDeals:             {Comm: `^(每日特惠|特惠)$`, Permissions: PermUser},
	CmdWfInvasions:              {Comm: `^入侵$`, Permissions: PermUser},
	CmdWfActiveMission:          {Comm: `^(裂隙|裂缝)$`, Permissions: PermUser},
	CmdWfVoidStorms:             {Comm: `^(九重天裂隙|九重天|九重天裂缝)$`, Permissions: PermUser},
	CmdWfActiveMissionPath:      {Comm: `^(钢铁裂隙|钢铁裂缝)$`, Permissions: PermUser},
	CmdWfSteelPath:              {Comm: `^钢铁奖励$`, Permissions: PermUser},
	CmdWfAllCycle:               {Comm: `^(平原|夜灵平原|夜灵平野|福尔图娜|魔胎之境|扎里曼)$`, Permissions: PermUser},
	CmdWfSyndicateOstrons:       {Comm: `^(希图斯|地球赏金)$`, Permissions: PermUser},
	CmdWfSyndicateEntrati:       {Comm: `^(英择谛|火卫二赏金)$`, Permissions: PermUser},
	CmdWfSyndicateSolarisUnited: {Comm: `^(索拉里斯|金星赏金)$`, Permissions: PermUser},
	CmdWfDuviriCycle:            {Comm: `^(轮换|双衍王境)$`, Permissions: PermUser},
	CmdWfNightWave:              {Comm: `^电波$`, Permissions: PermUser},
	CmdWfRivenDisUpdate:         {Comm: `^(紫卡倾向变动|倾向变动)$`, Permissions: PermUser},
	CmdWfTra:                    {Comm: `^翻译`, Permissions: PermUser},
	CmdWfMarketRiven:            {Comm: `^(/WR|WR|WMR|/wr|wr|wmr)`, Permissions: PermUser},
	CmdWfMarketOrders:           {Comm: `^(/WM|WM|MARKET|/市场|市场|/wm|wm|market)`, Permissions: PermUser},
	CmdWfRivenMarket:            {Comm: `^(/RM|RM|/rm|rm)`, Permissions: PermUser},
	CmdWfLichs:                  {Comm: `^(/CD|CD|赤毒|/cd|cd)`, Permissions: PermUser},
	CmdWfSisters:                {Comm: `^(/XT|XT|信条|/xt|xt)`, Permissions: PermUser},
	CmdWfPerlinSequence:         {Comm: `^(佩兰|佩兰数列|信条武器)$`, Permissions: PermUser},
	CmdWfMarketGodDump:          {Comm: `^金垃圾$`, Permissions: PermUser},
	CmdWfMarketSilverDump:       {Comm: `^银垃圾$`, Permissions: PermUser},
	CmdWfRelics:                 {Comm: `^(核桃|查核桃)`, Permissions: PermUser},
	CmdWfOpenRelics:             {Comm: `^(开核桃|砸核桃)$`, Permissions: PermUser},
	CmdWfRivenAnalyse:           {Comm: `^(紫卡分析|分析紫卡)`, Permissions: PermUser},
	CmdWfKnownCalendarSeasons:   {Comm: `^(1999|日历)$`, Permissions: PermUser},
	CmdWfSubscribe:              {Comm: `^订阅`, Permissions: PermUser},
	CmdWfUnsubscribe:            {Comm: `^取消订阅`, Permissions: PermUser},
}

// CodesOrder 按 Java Codes 枚举声明顺序排列的全部指令，
// 新增指令常量时必须同步追加（/log/codes 选项顺序依赖此切片）。
var CodesOrder = []Codes{
	CmdHelp,
	CmdCheckVersion,
	CmdUpdateWfResMarketItems,
	CmdUpdateWfResMarketRiven,
	CmdUpdateWfSister,
	CmdUpdateWfTar,
	CmdWfAlerts,
	CmdWfSorties,
	CmdWfLiteSortie,
	CmdWfVoid,
	CmdWfArbitrationEx,
	CmdWfArbitration,
	CmdWfDailyDeals,
	CmdWfInvasions,
	CmdWfActiveMission,
	CmdWfVoidStorms,
	CmdWfActiveMissionPath,
	CmdWfSteelPath,
	CmdWfAllCycle,
	CmdWfSyndicateOstrons,
	CmdWfSyndicateEntrati,
	CmdWfSyndicateSolarisUnited,
	CmdWfDuviriCycle,
	CmdWfNightWave,
	CmdWfRivenDisUpdate,
	CmdWfTra,
	CmdWfMarketRiven,
	CmdWfMarketOrders,
	CmdWfRivenMarket,
	CmdWfLichs,
	CmdWfSisters,
	CmdWfPerlinSequence,
	CmdWfMarketGodDump,
	CmdWfMarketSilverDump,
	CmdWfRelics,
	CmdWfOpenRelics,
	CmdWfRivenAnalyse,
	CmdWfSubscribe,
	CmdWfUnsubscribe,
	CmdWfKnownCalendarSeasons,
}
