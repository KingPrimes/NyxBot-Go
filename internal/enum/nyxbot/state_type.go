// NyxBot StateTypeEnum —— Warframe 物品状态/分类枚举（38 种）
package nyxbot

type StateType string

const (
	StateAll                   StateType = "ALL"
	StateGear                  StateType = "GEAR"
	StateKeys                  StateType = "KEYS"
	StateResources             StateType = "RESOURCES"
	StateSentinels             StateType = "SENTINELS"
	StateOther                 StateType = "OTHER"
	StateMods                  StateType = "MODS"
	StateWarframes             StateType = "WARFRAMES"
	StateWeapons               StateType = "WEAPONS"
	StateRelicBronze           StateType = "RELIC_BRONZE"
	StateRelicPlatinum         StateType = "RELIC_PLATINUM"
	StateRelicGold             StateType = "RELIC_GOLD"
	StateRelicSilver           StateType = "RELIC_SILVER"
	StateEnhancers             StateType = "ENHANCERS"
	StateSkins                 StateType = "SKINS"
	StateShip                  StateType = "SHIP"
	StateTennoAccessoryScarves StateType = "TENNO_ACCESSORY_SCARVES"
	StateWeaponsMeleeSkin      StateType = "WEAPONS_TENNO_MELEE_SKIN"
	StateKubrowPatterns        StateType = "KUBROW_PET_PATTERNS"
	StateCatbrowPatterns       StateType = "CATBROW_PET_PATTERNS"
	StateInfestedKavatPatterns StateType = "INFESTED_KAVAT_PET_PATTERNS"
	StateInfestedPredators     StateType = "INFESTED_PREDATORS_PET_PATTERNS"
	StateBackgrounds           StateType = "BACKGROUNDS"
	StateCursors               StateType = "CURSORS"
	StateSounds                StateType = "SOUNDS"
	StateCustomUI              StateType = "CUSTOM_UI_STYLE"
	StateActionFigureDioramas  StateType = "ACTION_FIGURE_DIORAMAS"
	StateColors                StateType = "COLORS"
	StateNotePacks             StateType = "NOTE_PACkS"
	StatePoseSets              StateType = "POSE_SETS"
	StateQuarterWallpapers     StateType = "QUARTERS_WALLPAPERS"
	StateArcade                StateType = "ARCADE"
	StateEmotes                StateType = "EMOTES"
	StateVideoWallBackdrops    StateType = "VIDEO_WALL_BACKDROPS"
	StateVideoWallSoundscapes  StateType = "VIDEO_WALL_SOUNDSCAPES"
	StateAvatarImages          StateType = "AVATAR_IMAGES"
	StateSuitCustomizations    StateType = "SUIT_CUSTOMIZATIONS"
	StatePackages              StateType = "PACKAGES"
	StateShipScenes            StateType = "SHIP_SCENES"
	StateBlueprint             StateType = "BLUEPRINT"
)

type stateTypeInfo struct {
	Name string
	Key  string
}

var StateTypeInfoMap = map[StateType]stateTypeInfo{
	StateAll:                   {"未知", ""},
	StateGear:                  {"道具", "/Lotus/Types/Game/Consumable"},
	StateKeys:                  {"钥匙", "/Lotus/Types/Keys"},
	StateResources:             {"资源", "/Lotus/Types/Resources"},
	StateSentinels:             {"守护", "/Lotus/Types/Sentinels"},
	StateOther:                 {"其他", ""},
	StateMods:                  {"MOD", "/Lotus/Upgrades/Mods"},
	StateWarframes:             {"战甲", "/Lotus/Powersuits"},
	StateWeapons:               {"武器", "/Lotus/Weapons"},
	StateRelicBronze:           {"古纪遗物", "/Lotus/Types/Game/Projections/T1Projection"},
	StateRelicPlatinum:         {"前纪遗物", "/Lotus/Types/Game/Projections/T2Projection"},
	StateRelicGold:             {"后纪遗物", "/Lotus/Types/Game/Projections/T3Projection"},
	StateRelicSilver:           {"中纪遗物", "/Lotus/Types/Game/Projections/T4Projection"},
	StateEnhancers:             {"强化", "/Lotus/Types/Recipes"},
	StateSkins:                 {"外观", "/Lotus/Types/Game/CosmeticEnhancers"},
	StateShip:                  {"飞船", "/Lotus/Types/Ships"},
	StateTennoAccessoryScarves: {"围巾", "/Lotus/Types/Game/CosmeticEnhancers/OperatorCosmetics/Scarves"},
	StateWeaponsMeleeSkin:      {"近战外观", "/Lotus/Types/Game/CosmeticEnhancers/WeaponSkins/Melee"},
	StateKubrowPatterns:        {"库柏毛色", "/Lotus/Types/Game/CosmeticEnhancers/KubrowPetPatterns"},
	StateCatbrowPatterns:       {"薇娜丽毛色", "/Lotus/Types/Game/CosmeticEnhancers/CatbrowPetPatterns"},
	StateInfestedKavatPatterns: {"感染库娃毛色", "/Lotus/Types/Game/CosmeticEnhancers/InfestedKavatPetPatterns"},
	StateInfestedPredators:     {"感染掠食者", "/Lotus/Types/Game/CosmeticEnhancers/InfestedPredators"},
	StateBackgrounds:           {"背景", "/Lotus/Types/Game/CosmeticEnhancers/DrifterUI/Backgrounds"},
	StateCursors:               {"光标", "/Lotus/Types/Game/CosmeticEnhancers/DrifterUI/Cursors"},
	StateSounds:                {"音效", "/Lotus/Types/Game/CosmeticEnhancers/Sounds"},
	StateCustomUI:              {"UI 样式", "/Lotus/Types/Game/CosmeticEnhancers/DrifterUI/Style"},
	StateActionFigureDioramas:  {"手办", "/Lotus/Types/Game/CosmeticEnhancers/ActionFigureDioramas"},
	StateColors:                {"颜色", "/Lotus/Types/Game/CosmeticEnhancers/ColorPalettes"},
	StateNotePacks:             {"音效包", "/Lotus/Types/Game/CosmeticEnhancers/NotePacks"},
	StatePoseSets:              {"表情", "/Lotus/Types/Game/CosmeticEnhancers/Poses"},
	StateQuarterWallpapers:     {"壁纸", "/Lotus/Types/Game/CosmeticEnhancers/DrifterUI/QuarterWallpapers"},
	StateArcade:                {"街机", "/Lotus/Types/Game/CosmeticEnhancers/Arcade"},
	StateEmotes:                {"表情动作", "/Lotus/Types/Game/CosmeticEnhancers/Emotes"},
	StateVideoWallBackdrops:    {"视频背景", "/Lotus/Types/Game/CosmeticEnhancers/DrifterUI/VideoWall/Backdrops"},
	StateVideoWallSoundscapes:  {"视频音效", "/Lotus/Types/Game/CosmeticEnhancers/DrifterUI/VideoWall/Soundscapes"},
	StateAvatarImages:          {"头像", "/Lotus/Types/Game/CosmeticEnhancers/DrifterUI/AvatarImages"},
	StateSuitCustomizations:    {"套服自定义", ""},
	StatePackages:              {"组合包", "/Lotus/StoreItems/Packages"},
	StateShipScenes:            {"飞船场景", "/Lotus/Types/Game/CosmeticEnhancers/LisetArt"},
	StateBlueprint:             {"蓝图", "/Lotus/Types/Recipes"},
}
