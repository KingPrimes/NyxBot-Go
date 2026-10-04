// 枚举字段 JSON 序列化辅助，对应 Java Jackson 的枚举默认序列化行为
// Java 端：数据库列存 ORDINAL int（JPA @Enumerated 默认），JSON 输出枚举名（WRITE_ENUMS_USING_TO_STRING=false）
// Go 端：DB 列保持 int（对齐 ORDINAL），JSON 输出枚举名字符串（对齐前端契约）
package warframe

import "strconv"

// toString 整数转字符串（供 JSON 序列化辅助使用）。
func toString(value int) string {
	return strconv.Itoa(value)
}

// rarityOrdinalToName RarityEnum ORDINAL 序数 -> 枚举名（对齐 draw-image-plugin RarityEnum）。
func rarityOrdinalToName(ordinal int) string {
	names := []string{"COMMON", "UNCOMMON", "RARE", "LEGENDARY"}
	if ordinal >= 0 && ordinal < len(names) {
		return names[ordinal]
	}
	return "COMMON"
}

// rarityNameToOrdinal RarityEnum 枚举名 -> ORDINAL 序数；未知回退 COMMON(0)。
func rarityNameToOrdinal(name string) int {
	switch name {
	case "COMMON":
		return 0
	case "UNCOMMON":
		return 1
	case "RARE":
		return 2
	case "LEGENDARY":
		return 3
	default:
		return 0
	}
}

// productCategoryOrdinalToName ProductCategory ORDINAL 序数 -> 枚举名（对齐 Weapons.ProductCategory）。
func productCategoryOrdinalToName(ordinal int) string {
	names := []string{
		"Pistols", "LongGuns", "Melee", "SpaceGuns",
		"SpaceMelee", "SpecialItems", "CrewShipWeapons", "SentinelWeapons", "Shotguns",
	}
	if ordinal >= 0 && ordinal < len(names) {
		return names[ordinal]
	}
	return "LongGuns"
}

// productCategoryNameToOrdinal ProductCategory 枚举名 -> ORDINAL 序数；未知回退 LongGuns(1)。
func productCategoryNameToOrdinal(name string) int {
	switch name {
	case "Pistols":
		return 0
	case "LongGuns":
		return 1
	case "Melee":
		return 2
	case "SpaceGuns":
		return 3
	case "SpaceMelee":
		return 4
	case "SpecialItems":
		return 5
	case "CrewShipWeapons":
		return 6
	case "SentinelWeapons":
		return 7
	case "Shotguns":
		return 8
	default:
		return 1
	}
}

// StateTypeDefinition StateTypeEnum 单项定义（对齐 Java StateTypeEnum 的 (KEY, NAME) 与声明顺序）。
type StateTypeDefinition struct {
	Name  string // 枚举名：JSON value、前端下拉 value、数据库 ORDINAL 对应的标识
	Label string // 中文名：Java NAME 字段，前端下拉 label
	Key   string // Java KEY 正则（整串匹配 uniqueName）；空串表示该枚举只作来源默认类型，永不命中正则
}

// StateTypes StateTypeEnum 全量定义（对齐 Java StateTypeEnum.java 的 40 项声明顺序）。
// 这是唯一事实来源：切片下标即 ORDINAL，导入分类、/state-translation/types 下拉、
// type 字段的枚举名↔序数转换都从这里取，避免多张平行表下标错位。
var StateTypes = []StateTypeDefinition{
	{"ALL", "未知", ""},
	{"GEAR", "道具", ""},
	{"KEYS", "钥匙", ""},
	{"RESOURCES", "资源", ""},
	{"SENTINELS", "守护/宠物", ""},
	{"OTHER", "加成", ""},
	{"MODS", "MOD", ""},
	{"WARFRAMES", "战甲", ""},
	{"WEAPONS", "武器", ""},
	{"RELIC_BRONZE", "完整遗物", `/Lotus/Types/Game/Projections/.*?(Bronze)$`},
	{"RELIC_PLATINUM", "光辉遗物", `/Lotus/Types/Game/Projections/.*?(Platinum)$`},
	{"RELIC_GOLD", "无暇遗物", `/Lotus/Types/Game/Projections/.*?(Gold)$`},
	{"RELIC_SILVER", "优良遗物", `/Lotus/Types/Game/Projections/.*?(Silver)$`},
	{"ENHANCERS", "赋能", `/Lotus/Upgrades/CosmeticEnhancers/.*`},
	{"SKINS", "外观", `/Lotus/Upgrades/Skins/.*`},
	{"SHIP", "采集机", `/Lotus/Types/Ship/.*`},
	{"TENNO_ACCESSORY_SCARVES", "披饰", `/Lotus/Characters/Tenno/Accessory/Scarves/.*`},
	{"WEAPONS_TENNO_MELEE_SKIN", "武器外观", `/Lotus/Weapons/Tenno/Melee/.*Skin`},
	{"KUBROW_PET_PATTERNS", "库狛花纹", `/Lotus/Types/Game/KubrowPet/Patterns/.*`},
	{"CATBROW_PET_PATTERNS", "库娃花纹", `/Lotus/Types/Game/CatbrowPet/Patterns/.*`},
	{"INFESTED_KAVAT_PET_PATTERNS", "狐帕菲拉花纹", `/Lotus/Types/Game/InfestedKavatPet/Patterns/.*`},
	{"INFESTED_PREDATORS_PET_PATTERNS", "扑猎达赛花纹", `/Lotus/Types/Game/InfestedPredatorPet/Patterns/.*`},
	{"BACKGROUNDS", "背景", `/Lotus/Interface/Graphics/CustomUI/Backgrounds/.*`},
	{"CURSORS", "指针", `/Lotus/Interface/Graphics/CustomUI/Cursors/.*`},
	{"SOUNDS", "登录音效", `/Lotus/Interface/Graphics/CustomUI/Sounds/.*`},
	{"CUSTOM_UI_STYLE", "主题", `/Lotus/Interface/Graphics/CustomUI/.*Style`},
	{"ACTION_FIGURE_DIORAMAS", "景观", `/Lotus/Types/Game/ActionFigureDioramas/.*`},
	{"COLORS", "颜色", `/Lotus/Types/Game/(.*)/?Colors/.*`},
	{"NOTE_PACkS", "乐器", `/Lotus/Types/Game/NotePacks/.*`},
	{"POSE_SETS", "姿势组", `/Lotus/Types/Game/PoseSets/.*`},
	{"QUARTERS_WALLPAPERS", "壁纸模板", `/Lotus/Types/Game/QuartersWallpapers/.*`},
	{"ARCADE", "街机", `/Lotus/Types/Items/Arcade/.*`},
	{"EMOTES", "表情", `/Lotus/Types/Items/Emotes/.*`},
	{"VIDEO_WALL_BACKDROPS", "视频墙背景", `/Lotus/Types/Items/VideoWallBackdrops/.*`},
	{"VIDEO_WALL_SOUNDSCAPES", "视频墙音景", `/Lotus/Types/Items/VideoWallSoundscapes/.*`},
	{"AVATAR_IMAGES", "浮印", `/Lotus/Types/StoreItems/AvatarImages/.*`},
	{"SUIT_CUSTOMIZATIONS", "颜色包", `/Lotus/Types/StoreItems/SuitCustomizations/.*`},
	{"PACKAGES", "组合包", `/Lotus/Types/StoreItems/Packages/.*`},
	{"SHIP_SCENES", "轨道飞行器装饰", `/Lotus/Types/StoreItems/ShipScenes/.*`},
	{"BLUEPRINT", "蓝图", `/Lotus/.*Blueprint`},
}

// StateTypeOrdinalToName StateTypeEnum ORDINAL 序数 -> 枚举名；越界回退 ALL。
func StateTypeOrdinalToName(ordinal int) string {
	if ordinal >= 0 && ordinal < len(StateTypes) {
		return StateTypes[ordinal].Name
	}
	return StateTypes[0].Name
}

// StateTypeOrdinal StateTypeEnum 枚举名 -> ORDINAL 序数；第二个返回值表示是否命中。
func StateTypeOrdinal(name string) (int, bool) {
	for index, definition := range StateTypes {
		if definition.Name == name {
			return index, true
		}
	}
	return 0, false
}

// StateTypeNameToOrdinal StateTypeEnum 枚举名 -> ORDINAL 序数；未知回退 RESOURCES
// （对齐 Java StateTranslation 反序列化与 CDN 初始化的兜底行为）。
func StateTypeNameToOrdinal(name string) int {
	if ordinal, ok := StateTypeOrdinal(name); ok {
		return ordinal
	}
	fallback, _ := StateTypeOrdinal("RESOURCES")
	return fallback
}
