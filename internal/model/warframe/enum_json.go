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

// stateTypeOrdinalToName StateTypeEnum ORDINAL 序数 -> 枚举名（对齐 StateTypeEnum 声明顺序）。
func stateTypeOrdinalToName(ordinal int) string {
	names := []string{
		"ALL", "GEAR", "KEYS", "RESOURCES", "SENTINELS", "OTHER", "MODS", "WARFRAMES",
		"WEAPONS", "RELIC_BRONZE", "RELIC_PLATINUM", "RELIC_GOLD", "RELIC_SILVER",
		"ENHANCERS", "SKINS", "SHIP", "TENNO_ACCESSORY_SCARVES", "WEAPONS_TENNO_MELEE_SKIN",
		"KUBROW_PET_PATTERNS", "CATBROW_PET_PATTERNS", "INFESTED_KAVAT_PET_PATTERNS",
		"INFESTED_PREDATORS_PET_PATTERNS", "BACKGROUNDS", "CURSORS", "SOUNDS",
		"CUSTOM_UI_STYLE", "ACTION_FIGURE_DIORAMAS", "COLORS", "NOTE_PACkS", "POSE_SETS",
		"QUARTERS_WALLPAPERS", "ARCADE", "EMOTES", "VIDEO_WALL_BACKDROPS",
		"VIDEO_WALL_SOUNDSCAPES", "AVATAR_IMAGES", "SUIT_CUSTOMIZATIONS", "PACKAGES",
		"SHIP_SCENES", "BLUEPRINT",
	}
	if ordinal >= 0 && ordinal < len(names) {
		return names[ordinal]
	}
	return "ALL"
}

// stateTypeNameToOrdinal StateTypeEnum 枚举名 -> ORDINAL 序数；未知回退 RESOURCES(3)。
func stateTypeNameToOrdinal(name string) int {
	for index, candidate := range []string{
		"ALL", "GEAR", "KEYS", "RESOURCES", "SENTINELS", "OTHER", "MODS", "WARFRAMES",
		"WEAPONS", "RELIC_BRONZE", "RELIC_PLATINUM", "RELIC_GOLD", "RELIC_SILVER",
		"ENHANCERS", "SKINS", "SHIP", "TENNO_ACCESSORY_SCARVES", "WEAPONS_TENNO_MELEE_SKIN",
		"KUBROW_PET_PATTERNS", "CATBROW_PET_PATTERNS", "INFESTED_KAVAT_PET_PATTERNS",
		"INFESTED_PREDATORS_PET_PATTERNS", "BACKGROUNDS", "CURSORS", "SOUNDS",
		"CUSTOM_UI_STYLE", "ACTION_FIGURE_DIORAMAS", "COLORS", "NOTE_PACkS", "POSE_SETS",
		"QUARTERS_WALLPAPERS", "ARCADE", "EMOTES", "VIDEO_WALL_BACKDROPS",
		"VIDEO_WALL_SOUNDSCAPES", "AVATAR_IMAGES", "SUIT_CUSTOMIZATIONS", "PACKAGES",
		"SHIP_SCENES", "BLUEPRINT",
	} {
		if candidate == name {
			return index
		}
	}
	return 3
}
