// draw-image-plugin 任务类型 & Modifier 枚举
package drawplugin

type MissionTypeInfo struct {
	Name  string
	Order int
}

type MissionType string

const (
	MTAssassination   MissionType = "MT_ASSASSINATION"
	MTExtermination   MissionType = "MT_EXTERMINATION"
	MTSurvival        MissionType = "MT_SURVIVAL"
	MTRescue          MissionType = "MT_RESCUE"
	MTSabotage        MissionType = "MT_SABOTAGE"
	MTCapture         MissionType = "MT_CAPTURE"
	MTDefault         MissionType = "MT_DEFAULT"
	MTIntel           MissionType = "MT_INTEL"
	MTDefense         MissionType = "MT_DEFENSE"
	MTMobileDefense   MissionType = "MT_MOBILE_DEFENSE"
	MTPvp             MissionType = "MT_PVP"
	MTSector          MissionType = "MT_SECTOR"
	MTTerritory       MissionType = "MT_TERRITORY"
	MTHive            MissionType = "MT_HIVE"
	MTRetrieval       MissionType = "MT_RETRIEVAL"
	MTExcavate        MissionType = "MT_EXCAVATE"
	MTSalvage         MissionType = "MT_SALVAGE"
	MTArena           MissionType = "MT_ARENA"
	MTPursuit         MissionType = "MT_PURSUIT"
	MTAssault         MissionType = "MT_ASSAULT"
	MTEvacuation      MissionType = "MT_EVACUATION"
	MTLandscape       MissionType = "MT_LANDSCAPE"
	MTArtifact        MissionType = "MT_ARTIFACT"
	MTDisruption      MissionType = "MT_DISRUPTION"
	MTVoidFlood       MissionType = "MT_VOID_FLOOD"
	MTVoidCascade     MissionType = "MT_VOID_CASCADE"
	MTVoidArmageddon  MissionType = "MT_VOID_ARMAGEDDON"
	MTAlchemy         MissionType = "MT_ALCHEMY"
	MTCambire         MissionType = "MT_CAMBIRE"
	MTLegacyteHarvest MissionType = "MT_LEGACYTE_HARVEST"
	MTShrineDefense   MissionType = "MT_SHRINE_DEFENSE"
	MTFaceoff         MissionType = "MT_FACEOFF"
	MTSkirmish        MissionType = "MT_SKIRMISH"
	MTVolatile        MissionType = "MT_VOLATILE"
	MTOrpheus         MissionType = "MT_ORPHEUS"
	MTAscension       MissionType = "MT_ASCENSION"
	MTCorruption      MissionType = "MT_CORRUPTION"
	MTEndlessCapture  MissionType = "MT_ENDLESS_CAPTURE"
	MTRelay           MissionType = "MT_RELAY"
)

var MissionTypeMap = map[MissionType]MissionTypeInfo{
	MTAssassination: {"刺杀", 1}, MTExtermination: {"歼灭", 2},
	MTSurvival: {"生存", 3}, MTRescue: {"救援", 4},
	MTSabotage: {"破坏", 5}, MTCapture: {"捕获", 6},
	MTDefault: {"默认", 7}, MTIntel: {"间谍", 8},
	MTDefense: {"防御", 9}, MTMobileDefense: {"移动防御", 10},
	MTPvp: {"PVP", 11}, MTSector: {"区域", 12},
	MTTerritory: {"拦截", 13}, MTHive: {"清巢", 14},
	MTRetrieval: {"劫持", 15}, MTExcavate: {"挖掘", 16},
	MTSalvage: {"资源回收", 17}, MTArena: {"竞技场", 18},
	MTPursuit: {"追击", 19}, MTAssault: {"强袭", 20},
	MTEvacuation: {"叛逃", 21}, MTLandscape: {"平原", 22},
	MTArtifact: {"神器", 23}, MTDisruption: {"中断", 24},
	MTVoidFlood: {"虚空洪流", 25}, MTVoidCascade: {"虚空覆涌", 26},
	MTVoidArmageddon: {"虚空决战", 27}, MTAlchemy: {"元素转换", 28},
	MTCambire: {"异化区", 29}, MTLegacyteHarvest: {"分离质收割", 30},
	MTShrineDefense: {"神坛防御", 31}, MTFaceoff: {"对抗", 32},
	MTSkirmish: {"前哨战", 33}, MTVolatile: {"爆发", 34},
	MTOrpheus: {"奧菲斯", 35}, MTAscension: {"扬升", 36},
	MTCorruption: {"虚空腐蚀", 37}, MTEndlessCapture: {"无限捕获", 38},
	MTRelay: {"中继站", 39},
}

// ModifierType 突击 modifier 类型
type ModifierType string

const (
	ModHazardRadiation   ModifierType = "SORTIE_MODIFIER_HAZARD_RADIATION"
	ModSlash             ModifierType = "SORTIE_MODIFIER_SLASH"
	ModBowOnly           ModifierType = "SORTIE_MODIFIER_BOW_ONLY"
	ModMagnetic          ModifierType = "SORTIE_MODIFIER_MAGNETIC"
	ModEximus            ModifierType = "SORTIE_MODIFIER_EXIMUS"
	ModExplosion         ModifierType = "SORTIE_MODIFIER_EXPLOSION"
	ModLowEnergy         ModifierType = "SORTIE_MODIFIER_LOW_ENERGY"
	ModShotgunOnly       ModifierType = "SORTIE_MODIFIER_SHOTGUN_ONLY"
	ModRifleOnly         ModifierType = "SORTIE_MODIFIER_RIFLE_ONLY"
	ModShields           ModifierType = "SORTIE_MODIFIER_SHIELDS"
	ModImpact            ModifierType = "SORTIE_MODIFIER_IMPACT"
	ModPuncture          ModifierType = "SORTIE_MODIFIER_PUNCTURE"
	ModCorrosive         ModifierType = "SORTIE_MODIFIER_CORROSIVE"
	ModViral             ModifierType = "SORTIE_MODIFIER_VIRAL"
	ModElectricity       ModifierType = "SORTIE_MODIFIER_ELECTRICITY"
	ModRadiation         ModifierType = "SORTIE_MODIFIER_RADIATION"
	ModGas               ModifierType = "SORTIE_MODIFIER_GAS"
	ModFire              ModifierType = "SORTIE_MODIFIER_FIRE"
	ModFreeze            ModifierType = "SORTIE_MODIFIER_FREEZE"
	ModToxin             ModifierType = "SORTIE_MODIFIER_TOXIN"
	ModPoison            ModifierType = "SORTIE_MODIFIER_POISON"
	ModHazardMagnetic    ModifierType = "SORTIE_MODIFIER_HAZARD_MAGNETIC"
	ModHazardFog         ModifierType = "SORTIE_MODIFIER_HAZARD_FOG"
	ModHazardFire        ModifierType = "SORTIE_MODIFIER_HAZARD_FIRE"
	ModHazardIce         ModifierType = "SORTIE_MODIFIER_HAZARD_ICE"
	ModHazardCold        ModifierType = "SORTIE_MODIFIER_HAZARD_COLD"
	ModArmor             ModifierType = "SORTIE_MODIFIER_ARMOR"
	ModSecondaryOnly     ModifierType = "SORTIE_MODIFIER_SECONDARY_ONLY"
	ModSniperOnly        ModifierType = "SORTIE_MODIFIER_SNIPER_ONLY"
	ModMeleeOnly         ModifierType = "SORTIE_MODIFIER_MELEE_ONLY"
)

var ModifierMap = map[ModifierType]string{
	ModHazardRadiation: "辐射灾害", ModSlash: "敌人物理强化(切割)",
	ModBowOnly: "限定弓", ModMagnetic: "磁力强化",
	ModEximus: "卓越者部队", ModExplosion: "爆炸",
	ModLowEnergy: "能量上限降低", ModShotgunOnly: "限定霰弹枪",
	ModRifleOnly: "限定步枪", ModShields: "护盾强化",
	ModImpact: "冲击强化", ModPuncture: "穿刺强化",
	ModCorrosive: "腐蚀强化", ModViral: "病毒强化",
	ModElectricity: "电击强化", ModRadiation: "辐射强化",
	ModGas: "气体强化", ModFire: "火焰强化",
	ModFreeze: "冰冻强化", ModToxin: "毒素强化",
	ModPoison: "剧毒", ModHazardMagnetic: "磁力灾害",
	ModHazardFog: "浓雾灾害", ModHazardFire: "火焰灾害",
	ModHazardIce: "冰冻灾害", ModHazardCold: "极寒灾害",
	ModArmor: "护甲强化", ModSecondaryOnly: "限定副武器",
	ModSniperOnly: "限定狙击枪", ModMeleeOnly: "限定近战",
}
