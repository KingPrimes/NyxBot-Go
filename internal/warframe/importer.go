// 本地数据导入器，对应 Java NyxBot 的 WarframeDataSource
// 分阶段导入：Phase 0 下载导出文件 → Phase 1 状态翻译 → Phase 2 各业务表
// 落库策略对齐 Java：按主键 merge 的 upsert（非全量清空），CDN 数据按业务键先查后写
package warframe

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"nyxbot-go/internal/database"
	"nyxbot-go/internal/logging"
	modelwarframe "nyxbot-go/internal/model/warframe"
)

// exportSource 导出文件来源：文件前缀 → JSON 顶层 key → 默认 StateType 序数。
// 对齐 Java StateTranslationService.EXPORT_SOURCES（含 ExportSortieRewards → ExportOther 特例）。
type exportSource struct {
	prefix string // 导出文件名前缀
	key    string // JSON 顶层 key
	state  int    // 默认 StateType 序数
}

// exportSources 状态翻译使用的导出来源（对齐 Java EXPORT_SOURCES 全量 12 项）。
var exportSources = []exportSource{
	{"ExportCustoms", "ExportCustoms", 0},         // ALL
	{"ExportDrones", "ExportDrones", 0},           // ALL
	{"ExportFlavour", "ExportFlavour", 0},         // ALL
	{"ExportGear", "ExportGear", 1},               // GEAR
	{"ExportKeys", "ExportKeys", 2},               // KEYS
	{"ExportRelicArcane", "ExportRelicArcane", 0}, // ALL
	{"ExportResources", "ExportResources", 3},     // RESOURCES
	{"ExportSentinels", "ExportSentinels", 4},     // SENTINELS
	{"ExportSortieRewards", "ExportOther", 5},     // OTHER（key 为 ExportOther）
	{"ExportUpgrades", "ExportUpgrades", 6},       // MODS
	{"ExportWarframes", "ExportWarframes", 7},     // WARFRAMES
	{"ExportWeapons", "ExportWeapons", 8},         // WEAPONS
}

// DataImporter 管理 Warframe 本地表的启动导入与手动更新。
type DataImporter struct {
	exporter *ExportFilePath
	market   *MarketAPI
	db       *gorm.DB
}

// NewDataImporter 创建数据导入器；db 为 nil 时使用全局 database.DB。
func NewDataImporter(exporter *ExportFilePath, market *MarketAPI, db *gorm.DB) *DataImporter {
	if db == nil {
		db = database.DB
	}
	return &DataImporter{exporter: exporter, market: market, db: db}
}

// ImportAll 执行完整启动导入（对齐 Java WarframeDataSource.init 顺序）：
// 导出文件 → 状态翻译 → 其余表（市场 API 表 + 导出表 + CDN 表）。
// 任何一步失败不阻塞后续（对齐 Java 降级语义）。
func (importer *DataImporter) ImportAll(ctx context.Context) {
	// Phase 0：下载导出文件（失败且无本地缓存则仅告警，不终止启动）
	if _, err := importer.exporter.SeverExportFiles(ctx); err != nil {
		logging.WarnPack("warframe.import", "export files download failed: %v", err)
	}

	// Phase 1：状态翻译（失败仅告警，翻译功能不可用）
	if err := importer.ImportStateTranslation(ctx); err != nil {
		logging.WarnPack("warframe.import", "state translation import failed: %v", err)
	}

	// Phase 2：其余业务表（对齐 Java Phase 1c 市场表 + Phase 2 导出/CDN 表）
	steps := []struct {
		name string
		run  func(context.Context) error
	}{
		// 市场 API 表（Phase 1c 并行组）
		{"orders-items", importer.UpdateOrdersItems},
		{"riven-items", importer.UpdateRivenItems},
		{"lich-sister-weapons", importer.UpdateLichSister},
		{"ephemeras", importer.UpdateEphemeras},
		{"reward-pool", importer.ImportRewardPool},
		// 导出/CDN 表（Phase 2 并行组）
		{"alias", importer.ImportAlias},
		{"riven-tion", importer.ImportRivenTion},
		{"riven-tion-alias", importer.ImportRivenTionAlias},
		{"riven-analyse", importer.ImportRivenAnalyseTrend},
		{"nodes", importer.ImportNodes},
		{"weapons", importer.ImportWeapons},
		{"night-wave", importer.ImportNightWave},
		{"warframes", importer.ImportWarframes},
		{"relics", importer.ImportRelics},
	}
	for _, step := range steps {
		if err := step.run(ctx); err != nil {
			logging.WarnPack("warframe.import", "%s import failed: %v", step.name, err)
		}
	}
}

// flexibleNumber 兼容 JSON 数字与数字字符串（CDN 数据部分字段以字符串存储，如 "0"）。
type flexibleNumber float64

// UnmarshalJSON 接受数字或数字字符串。
func (number *flexibleNumber) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		*number = 0
		return nil
	}
	var asFloat float64
	if err := json.Unmarshal(data, &asFloat); err == nil {
		*number = flexibleNumber(asFloat)
		return nil
	}
	var asString string
	if err := json.Unmarshal(data, &asString); err == nil {
		value, err := strconv.ParseFloat(strings.TrimSpace(asString), 64)
		if err != nil {
			return err
		}
		*number = flexibleNumber(value)
		return nil
	}
	return fmt.Errorf("invalid number %q", string(data))
}

// Float64 返回数值。
func (number flexibleNumber) Float64() float64 {
	return float64(number)
}

// rarityNameToOrdinal CDN rarity 字符串枚举名 → RarityEnum ORDINAL（对齐 model 包映射）。
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

// importMarketItems 导入市场物品（/v2/items），对齐 OrdersItemsService.initOrdersItemsData。
func (importer *DataImporter) importMarketItems() error {
	items, err := importer.market.FetchItems()
	if err != nil {
		return err
	}
	records := make([]modelwarframe.OrdersItem, 0, len(items))
	for _, item := range items {
		if item.ID == "" || item.Slug == "" {
			continue
		}
		records = append(records, modelwarframe.OrdersItem{
			ID:            item.ID,
			Slug:          item.Slug,
			GameRef:       item.GameRef,
			BulkTradable:  item.BulkTradable,
			MaxRank:       item.MaxRank,
			Ducats:        item.Ducats,
			Name:          item.Name,
			Icon:          item.Icon,
			Thumb:         item.Thumb,
			Vaulted:       item.Vaulted,
			MaxAmberStars: item.MaxAmberStars,
			MaxCyanStars:  item.MaxCyanStars,
			BaseEndo:      item.BaseEndo,
		})
	}
	if len(records) == 0 {
		return fmt.Errorf("market /v2/items returned no valid records")
	}
	return importer.db.Save(records).Error
}

// importRivenItems 导入紫卡武器（/v2/riven/weapons），对齐 RivenItemsService。
func (importer *DataImporter) importRivenItems() error {
	rawItems, err := importer.market.FetchRivenWeapons()
	if err != nil {
		return err
	}
	records := make([]modelwarframe.RivenItem, 0, len(rawItems))
	for _, item := range rawItems {
		if item.ID == "" || item.Slug == "" || item.GameRef == "" || item.Group == "" {
			continue
		}
		records = append(records, modelwarframe.RivenItem{
			ID:             item.ID,
			Slug:           item.Slug,
			GameRef:        item.GameRef,
			Group:          item.Group,
			RivenType:      item.RivenType,
			Disposition:    item.Disposition,
			ReqMasteryRank: item.ReqMasteryRank,
			Name:           item.Name,
			Icon:           item.Icon,
			Thumb:          item.Thumb,
		})
	}
	if len(records) == 0 {
		return fmt.Errorf("market /v2/riven/weapons returned no valid records")
	}
	return importer.db.Save(records).Error
}

// importLichSisterWeapons 导入赤毒/信条武器（双端点），对齐 LichSisterWeaponsService。
func (importer *DataImporter) importLichSisterWeapons() error {
	groups, err := importer.market.FetchLichSisterWeapons()
	if err != nil {
		return err
	}
	records := make([]modelwarframe.LichSisterWeapon, 0)
	for _, group := range groups {
		for _, item := range group {
			if item.ID == "" || item.Slug == "" || item.GameRef == "" {
				continue
			}
			records = append(records, modelwarframe.LichSisterWeapon{
				ID:             item.ID,
				Slug:           item.Slug,
				GameRef:        item.GameRef,
				ReqMasteryRank: item.ReqMasteryRank,
				Name:           item.Name,
				Icon:           item.Icon,
				Thumb:          item.Thumb,
			})
		}
	}
	if len(records) == 0 {
		return fmt.Errorf("market lich/sister weapons returned no valid records")
	}
	return importer.db.Save(records).Error
}

// importEphemeras 导入赤毒/信条幻纹（双端点），对齐 EphemerasService。
func (importer *DataImporter) importEphemeras() error {
	groups, err := importer.market.FetchLichSisterEphemeras()
	if err != nil {
		return err
	}
	records := make([]modelwarframe.Ephemera, 0)
	for _, group := range groups {
		for _, item := range group {
			if item.ID == "" {
				continue
			}
			records = append(records, modelwarframe.Ephemera{
				ID:        item.ID,
				Slug:      item.Slug,
				GameRef:   item.GameRef,
				Animation: item.Animation,
				Element:   item.Element,
				Name:      item.Name,
				Icon:      item.Icon,
				Thumb:     item.Thumb,
			})
		}
	}
	if len(records) == 0 {
		return fmt.Errorf("market lich/sister ephemeras returned no valid records")
	}
	return importer.db.Save(records).Error
}

// ImportStateTranslation 从导出文件导入状态翻译（Phase 1）。
// 对齐 Java StateTranslationService.initData：按 EXPORT_SOURCES 读取，
// type 默认用来源默认值，CDN 源（state_translation.json）才按 uniqueName 正则匹配覆盖。
func (importer *DataImporter) ImportStateTranslation(_ context.Context) error {
	records := make([]modelwarframe.StateTranslation, 0, 2048)

	for _, source := range exportSources {
		raw, err := importer.exporter.ReadExportFile(source.prefix)
		if err != nil || len(raw) == 0 {
			continue
		}
		var envelope map[string]json.RawMessage
		if err := json.Unmarshal(raw, &envelope); err != nil {
			continue
		}
		listRaw, ok := envelope[source.key]
		if !ok {
			continue
		}
		var entries []struct {
			UniqueName  string          `json:"uniqueName"`
			EnglishName string          `json:"englishName"`
			Description json.RawMessage `json:"description"`
		}
		if err := json.Unmarshal(listRaw, &entries); err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.UniqueName == "" {
				continue
			}
			description := extractFirstDescription(entry.Description)
			records = append(records, modelwarframe.StateTranslation{
				UniqueName:  entry.UniqueName,
				Name:        entry.EnglishName,
				Description: description,
				Type:        source.state,
			})
		}
	}

	// CDN state_translation.json：按 uniqueName 正则匹配 StateTypeEnum.KEY 覆盖 type（未匹配回退 RESOURCES）。
	if cdnRaw, cdnErr := cdnFetchFile("warframe/state_translation.json"); cdnErr == nil {
		var cdnRecords []struct {
			UniqueName  string          `json:"uniqueName"`
			Name        string          `json:"name"`
			Description json.RawMessage `json:"description"`
			Type        json.RawMessage `json:"type"`
			ParentName  string          `json:"parentName"`
		}
		if err := json.Unmarshal(cdnRaw, &cdnRecords); err == nil {
			for _, entry := range cdnRecords {
				if entry.UniqueName == "" {
					continue
				}
				records = append(records, modelwarframe.StateTranslation{
					UniqueName:  entry.UniqueName,
					Name:        entry.Name,
					Description: extractFirstDescription(entry.Description),
					Type:        stateTypeMatchOrdinal(entry.UniqueName, entry.Type, stateTypeNameToOrdinal()),
					ParentName:  entry.ParentName,
				})
			}
		}
	}

	if len(records) == 0 {
		return fmt.Errorf("no state translation records from export files")
	}
	return importer.db.Save(records).Error
}

// stateTypeNameToOrdinal 构建 StateType 枚举名 -> 序数映射（对齐 Java 枚举声明顺序）。
func stateTypeNameToOrdinal() map[string]int {
	result := make(map[string]int, len(stateTypeNames))
	for index, name := range stateTypeNames {
		result[name] = index
	}
	return result
}

// stateTypeNames StateTypeEnum 声明顺序（对齐 Java StateTypeEnum.java）。
var stateTypeNames = []string{
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

// stateTypeKeyPatterns StateType 正则（对齐 Java KEY 字段），用于 uniqueName 匹配。
var stateTypeKeyPatterns = []*regexp.Regexp{
	regexp.MustCompile(`^$`), // ALL
	regexp.MustCompile(`^$`), // GEAR
	regexp.MustCompile(`^$`), // KEYS
	regexp.MustCompile(`^$`), // RESOURCES
	regexp.MustCompile(`^$`), // SENTINELS
	regexp.MustCompile(`^$`), // OTHER
	regexp.MustCompile(`^$`), // MODS
	regexp.MustCompile(`^$`), // WARFRAMES
	regexp.MustCompile(`^$`), // WEAPONS
	regexp.MustCompile(`/Lotus/Types/Game/Projections/.*?(Bronze)$`),
	regexp.MustCompile(`/Lotus/Types/Game/Projections/.*?(Platinum)$`),
	regexp.MustCompile(`/Lotus/Types/Game/Projections/.*?(Gold)$`),
	regexp.MustCompile(`/Lotus/Types/Game/Projections/.*?(Silver)$`),
	regexp.MustCompile(`/Lotus/Upgrades/CosmeticEnhancers/.*`),
	regexp.MustCompile(`/Lotus/Upgrades/Skins/.*`),
	regexp.MustCompile(`/Lotus/Types/Ship/.*`),
	regexp.MustCompile(`/Lotus/Characters/Tenno/Accessory/Scarves/.*`),
	regexp.MustCompile(`/Lotus/Weapons/Tenno/Melee/.*Skin`),
	regexp.MustCompile(`/Lotus/Types/Game/KubrowPet/Patterns/.*`),
	regexp.MustCompile(`/Lotus/Types/Game/CatbrowPet/Patterns/.*`),
	regexp.MustCompile(`/Lotus/Types/Game/InfestedKavatPet/Patterns/.*`),
	regexp.MustCompile(`/Lotus/Types/Game/InfestedPredatorPet/Patterns/.*`),
	regexp.MustCompile(`/Lotus/Interface/Graphics/CustomUI/Backgrounds/.*`),
	regexp.MustCompile(`/Lotus/Interface/Graphics/CustomUI/Cursors/.*`),
	regexp.MustCompile(`/Lotus/Interface/Graphics/CustomUI/Sounds/.*`),
	regexp.MustCompile(`/Lotus/Interface/Graphics/CustomUI/.*Style`),
	regexp.MustCompile(`/Lotus/Types/Game/ActionFigureDioramas/.*`),
	regexp.MustCompile(`/Lotus/Types/Game/(.*)/?Colors/.*`),
	regexp.MustCompile(`/Lotus/Types/Game/NotePacks/.*`),
	regexp.MustCompile(`/Lotus/Types/Game/PoseSets/.*`),
	regexp.MustCompile(`/Lotus/Types/Game/QuartersWallpapers/.*`),
	regexp.MustCompile(`/Lotus/Types/Items/Arcade/.*`),
	regexp.MustCompile(`/Lotus/Types/Items/Emotes/.*`),
	regexp.MustCompile(`/Lotus/Types/Items/VideoWallBackdrops/.*`),
	regexp.MustCompile(`/Lotus/Types/Items/VideoWallSoundscapes/.*`),
	regexp.MustCompile(`/Lotus/Types/StoreItems/AvatarImages/.*`),
	regexp.MustCompile(`/Lotus/Types/StoreItems/SuitCustomizations/.*`),
	regexp.MustCompile(`/Lotus/Types/StoreItems/Packages/.*`),
	regexp.MustCompile(`/Lotus/Types/StoreItems/ShipScenes/.*`),
	regexp.MustCompile(`/Lotus/.*Blueprint`),
}

// stateTypeMatchOrdinal 确定 CDN 翻译条目的 type 序数：JSON 自带 type（枚举名）优先，
// 否则按 uniqueName 正则匹配 StateTypeEnum.KEY，仍未匹配回退 RESOURCES(3)。
func stateTypeMatchOrdinal(uniqueName string, rawType json.RawMessage, nameToOrdinal map[string]int) int {
	if len(rawType) > 0 && string(rawType) != "null" {
		var typeName string
		if err := json.Unmarshal(rawType, &typeName); err == nil && typeName != "" {
			if ordinal, ok := nameToOrdinal[typeName]; ok {
				return ordinal
			}
		}
	}
	for index, pattern := range stateTypeKeyPatterns {
		if pattern.MatchString(uniqueName) {
			return index
		}
	}
	return nameToOrdinal["RESOURCES"]
}

// extractFirstDescription description 字段可能为数组或字符串，取首个字符串（对齐 Java）。
func extractFirstDescription(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text
	}
	var list []string
	if err := json.Unmarshal(raw, &list); err == nil && len(list) > 0 {
		return list[0]
	}
	return ""
}

// ImportAlias 从 CDN alias.json 导入（按 cn 业务键智能更新，空数据抛错）。
func (importer *DataImporter) ImportAlias(_ context.Context) error {
	raw, err := cdnFetchFile("warframe/alias.json")
	if err != nil {
		return err
	}
	var records []struct {
		ID uint   `json:"id"`
		Cn string `json:"cn"`
		En string `json:"en"`
	}
	if err := json.Unmarshal(raw, &records); err != nil {
		return fmt.Errorf("parse alias.json: %w", err)
	}
	if len(records) == 0 {
		return fmt.Errorf("alias.json is empty")
	}
	// 按 cn 匹配现有记录复用 id（对齐 Java 智能更新）
	existing := make(map[string]uint)
	var all []modelwarframe.Alias
	importer.db.Find(&all)
	for _, record := range all {
		existing[record.Cn] = record.ID
	}
	upserts := make([]modelwarframe.Alias, 0, len(records))
	for _, record := range records {
		upserts = append(upserts, modelwarframe.Alias{
			ID: existing[record.Cn],
			En: record.En,
			Cn: record.Cn,
		})
	}
	return importer.db.Save(upserts).Error
}

// ImportRivenTion 从 CDN market_riven_tion.json 导入（按 urlName 业务键智能更新）。
// CDN 数据的 negative_only 等字段是字符串（如 "0"），用 flexibleNumber 兼容。
func (importer *DataImporter) ImportRivenTion(_ context.Context) error {
	raw, err := cdnFetchFile("warframe/market_riven_tion.json")
	if err != nil {
		return err
	}
	var records []struct {
		IDs                uint           `json:"ids"`
		Effect             string         `json:"effect"`
		Group              string         `json:"group"`
		NegativeOnly       flexibleNumber `json:"negative_only"`
		PositiveIsNegative flexibleNumber `json:"positive_is_negative"`
		Prefix             string         `json:"prefix"`
		SearchOnly         flexibleNumber `json:"search_only"`
		Suffix             string         `json:"suffix"`
		Units              string         `json:"units"`
		URLName            string         `json:"url_name"`
		ExclusiveTo        string         `json:"exclusive_to"`
	}
	if err := json.Unmarshal(raw, &records); err != nil {
		return fmt.Errorf("parse market_riven_tion.json: %w", err)
	}
	existing := make(map[string]uint)
	var all []modelwarframe.RivenTion
	importer.db.Find(&all)
	for _, record := range all {
		existing[record.URLName] = record.IDs
	}
	upserts := make([]modelwarframe.RivenTion, 0, len(records))
	for _, record := range records {
		upserts = append(upserts, modelwarframe.RivenTion{
			IDs:                existing[record.URLName],
			Effect:             record.Effect,
			Group:              record.Group,
			NegativeOnly:       record.NegativeOnly.Float64(),
			PositiveIsNegative: record.PositiveIsNegative.Float64(),
			Prefix:             record.Prefix,
			SearchOnly:         record.SearchOnly.Float64(),
			Suffix:             record.Suffix,
			Units:              record.Units,
			URLName:            record.URLName,
			ExclusiveTo:        record.ExclusiveTo,
		})
	}
	return importer.db.Save(upserts).Error
}

// ImportRivenTionAlias 从 CDN market_riven_tion_alias.json 导入（按 en|cn 组合键）。
func (importer *DataImporter) ImportRivenTionAlias(_ context.Context) error {
	raw, err := cdnFetchFile("warframe/market_riven_tion_alias.json")
	if err != nil {
		return err
	}
	var records []struct {
		ID uint   `json:"id"`
		En string `json:"en"`
		Cn string `json:"cn"`
	}
	if err := json.Unmarshal(raw, &records); err != nil {
		return fmt.Errorf("parse market_riven_tion_alias.json: %w", err)
	}
	existing := make(map[string]uint)
	var all []modelwarframe.RivenTionAlias
	importer.db.Find(&all)
	for _, record := range all {
		existing[record.En+"|"+record.Cn] = record.ID
	}
	upserts := make([]modelwarframe.RivenTionAlias, 0, len(records))
	for _, record := range records {
		upserts = append(upserts, modelwarframe.RivenTionAlias{
			ID: existing[record.En+"|"+record.Cn],
			En: record.En,
			Cn: record.Cn,
		})
	}
	return importer.db.Save(upserts).Error
}

// ImportRivenAnalyseTrend 从 ExportUpgrades 本地计算（对齐 RivenTrendGenerator）。
func (importer *DataImporter) ImportRivenAnalyseTrend(_ context.Context) error {
	raw, err := importer.exporter.ReadExportFile("ExportUpgrades")
	if err != nil || len(raw) == 0 {
		return fmt.Errorf("ExportUpgrades file unavailable")
	}
	return importer.computeRivenTrend(raw)
}

// ImportNodes 从 ExportRegions 导入节点。
func (importer *DataImporter) ImportNodes(_ context.Context) error {
	raw, err := importer.exporter.ReadExportFile("ExportRegions")
	if err != nil || len(raw) == 0 {
		return fmt.Errorf("ExportRegions file unavailable")
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return err
	}
	var entries []struct {
		UniqueName    string `json:"uniqueName"`
		Name          string `json:"name"`
		SystemName    string `json:"systemName"`
		SystemIndex   int    `json:"systemIndex"`
		NodeType      int    `json:"nodeType"`
		MasteryReq    int    `json:"masteryReq"`
		MissionIndex  int    `json:"missionIndex"`
		FactionIndex  int    `json:"factionIndex"`
		MinEnemyLevel int    `json:"minEnemyLevel"`
		MaxEnemyLevel int    `json:"maxEnemyLevel"`
	}
	if err := json.Unmarshal(envelope["ExportRegions"], &entries); err != nil {
		return err
	}
	records := make([]modelwarframe.Nodes, 0, len(entries))
	for _, entry := range entries {
		if entry.UniqueName == "" {
			continue
		}
		records = append(records, modelwarframe.Nodes{
			UniqueName:    entry.UniqueName,
			Name:          entry.Name,
			SystemName:    entry.SystemName,
			SystemIndex:   entry.SystemIndex,
			NodeType:      entry.NodeType,
			MasteryReq:    entry.MasteryReq,
			MissionIndex:  entry.MissionIndex,
			FactionIndex:  entry.FactionIndex,
			MinEnemyLevel: entry.MinEnemyLevel,
			MaxEnemyLevel: entry.MaxEnemyLevel,
		})
	}
	return importer.db.Save(records).Error
}

// ImportWeapons 从 ExportWeapons 导入武器。
func (importer *DataImporter) ImportWeapons(_ context.Context) error {
	raw, err := importer.exporter.ReadExportFile("ExportWeapons")
	if err != nil || len(raw) == 0 {
		return fmt.Errorf("ExportWeapons file unavailable")
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return err
	}
	var entries []struct {
		UniqueName         string    `json:"uniqueName"`
		Name               string    `json:"name"`
		CodexSecret        bool      `json:"codexSecret"`
		DamagePerShot      []float64 `json:"damagePerShot"`
		TotalDamage        int       `json:"totalDamage"`
		Description        string    `json:"description"`
		CriticalChance     float64   `json:"criticalChance"`
		CriticalMultiplier float64   `json:"criticalMultiplier"`
		ProcChance         float64   `json:"procChance"`
		FireRate           float64   `json:"fireRate"`
		MasteryReq         int       `json:"masteryReq"`
		ProductCategory    int       `json:"productCategory"`
		Slot               int       `json:"slot"`
		Accuracy           float64   `json:"accuracy"`
		OmegaAttenuation   float64   `json:"omegaAttenuation"`
		MaxLevelCap        int       `json:"maxLevelCap"`
		Noise              string    `json:"noise"`
		Trigger            string    `json:"trigger"`
		MagazineSize       int       `json:"magazineSize"`
		ReloadTime         float64   `json:"reloadTime"`
		Sentinel           bool      `json:"sentinel"`
		Multishot          int       `json:"multishot"`
	}
	if err := json.Unmarshal(envelope["ExportWeapons"], &entries); err != nil {
		return err
	}
	records := make([]modelwarframe.Weapons, 0, len(entries))
	for _, entry := range entries {
		if entry.UniqueName == "" {
			continue
		}
		damageJSON, _ := json.Marshal(entry.DamagePerShot)
		records = append(records, modelwarframe.Weapons{
			UniqueName:         entry.UniqueName,
			Name:               entry.Name,
			CodexSecret:        entry.CodexSecret,
			DamagePerShot:      string(damageJSON),
			TotalDamage:        entry.TotalDamage,
			Description:        entry.Description,
			EnglishName:        extractEnglishName(entry.Description),
			CriticalChance:     entry.CriticalChance,
			CriticalMultiplier: entry.CriticalMultiplier,
			ProcChance:         entry.ProcChance,
			FireRate:           entry.FireRate,
			MasteryReq:         entry.MasteryReq,
			ProductCategory:    entry.ProductCategory,
			Slot:               entry.Slot,
			Accuracy:           entry.Accuracy,
			OmegaAttenuation:   entry.OmegaAttenuation,
			MaxLevelCap:        entry.MaxLevelCap,
			Noise:              entry.Noise,
			Trigger:            entry.Trigger,
			MagazineSize:       entry.MagazineSize,
			ReloadTime:         entry.ReloadTime,
			Sentinel:           entry.Sentinel,
			Multishot:          entry.Multishot,
		})
	}
	return importer.db.Save(records).Error
}

// ImportNightWave 从 ExportSortieRewards 的 ExportNightwave.challenges 导入电波。
func (importer *DataImporter) ImportNightWave(_ context.Context) error {
	raw, err := importer.exporter.ReadExportFile("ExportSortieRewards")
	if err != nil || len(raw) == 0 {
		return fmt.Errorf("ExportSortieRewards file unavailable")
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return err
	}
	var nightwave struct {
		Challenges []struct {
			UniqueName  string `json:"uniqueName"`
			Name        string `json:"name"`
			Description string `json:"description"`
			Standing    int    `json:"standing"`
			Required    int    `json:"required"`
		} `json:"challenges"`
	}
	if err := json.Unmarshal(envelope["ExportNightwave"], &nightwave); err != nil {
		return err
	}
	// description 入库保留原始文本（对齐 Java：|COUNT| 替换发生在序列化 getter，不入库）
	records := make([]modelwarframe.NightWave, 0, len(nightwave.Challenges))
	for _, entry := range nightwave.Challenges {
		if entry.UniqueName == "" {
			continue
		}
		records = append(records, modelwarframe.NightWave{
			UniqueName:  entry.UniqueName,
			Name:        entry.Name,
			Description: entry.Description,
			Standing:    entry.Standing,
			Required:    entry.Required,
		})
	}
	return importer.db.Save(records).Error
}

// ImportWarframes 从 ExportWarframes 导入战甲（含技能级联）。
func (importer *DataImporter) ImportWarframes(_ context.Context) error {
	raw, err := importer.exporter.ReadExportFile("ExportWarframes")
	if err != nil || len(raw) == 0 {
		return fmt.Errorf("ExportWarframes file unavailable")
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return err
	}
	var entries []struct {
		UniqueName      string `json:"uniqueName"`
		Name            string `json:"name"`
		ParentName      string `json:"parentName"`
		Description     string `json:"description"`
		Health          int    `json:"health"`
		Shield          int    `json:"shield"`
		Armor           int    `json:"armor"`
		Stamina         int    `json:"stamina"`
		Power           int    `json:"power"`
		CodexSecret     bool   `json:"codexSecret"`
		MasteryReq      int    `json:"masteryReq"`
		SprintSpeed     int    `json:"sprintSpeed"`
		ProductCategory string `json:"productCategory"`
		Abilities       []struct {
			AbilityUniqueName string `json:"abilityUniqueName"`
			AbilityName       string `json:"abilityName"`
			Description       string `json:"description"`
		} `json:"abilities"`
	}
	if err := json.Unmarshal(envelope["ExportWarframes"], &entries); err != nil {
		return err
	}
	records := make([]modelwarframe.Warframes, 0, len(entries))
	for _, entry := range entries {
		if entry.UniqueName == "" {
			continue
		}
		abilities := make([]modelwarframe.WarframesAbility, 0, len(entry.Abilities))
		for _, ability := range entry.Abilities {
			if ability.AbilityUniqueName == "" {
				continue
			}
			abilities = append(abilities, modelwarframe.WarframesAbility{
				AbilityUniqueName:  ability.AbilityUniqueName,
				AbilityName:        ability.AbilityName,
				Description:        ability.Description,
				WarframeUniqueName: entry.UniqueName,
			})
		}
		records = append(records, modelwarframe.Warframes{
			UniqueName:      entry.UniqueName,
			Name:            entry.Name,
			ParentName:      entry.ParentName,
			Description:     entry.Description,
			Health:          entry.Health,
			Shield:          entry.Shield,
			Armor:           entry.Armor,
			Stamina:         entry.Stamina,
			Power:           entry.Power,
			CodexSecret:     entry.CodexSecret,
			MasteryReq:      entry.MasteryReq,
			SprintSpeed:     entry.SprintSpeed,
			ProductCategory: entry.ProductCategory,
			Abilities:       abilities,
		})
	}
	return importer.db.Session(&gorm.Session{FullSaveAssociations: true}).Save(records).Error
}

// ImportRelics 从 ExportRelicArcane 导入遗物（按 name 去重 + 翻译奖励名）。
func (importer *DataImporter) ImportRelics(_ context.Context) error {
	raw, err := importer.exporter.ReadExportFile("ExportRelicArcane")
	if err != nil || len(raw) == 0 {
		return fmt.Errorf("ExportRelicArcane file unavailable")
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return err
	}
	var entries []struct {
		UniqueName  string `json:"uniqueName"`
		Name        string `json:"name"`
		CodexSecret bool   `json:"codexSecret"`
		Description string `json:"description"`
		Rewards     []struct {
			ID         string `json:"id"`
			RewardName string `json:"rewardName"`
			Rarity     int    `json:"rarity"`
			Tier       int    `json:"tier"`
			ItemCount  int    `json:"itemCount"`
		} `json:"relicRewards"`
	}
	if err := json.Unmarshal(envelope["ExportRelicArcane"], &entries); err != nil {
		return err
	}

	// 按 name 去重（保留首个）并过滤 codexSecret
	seen := make(map[string]bool, len(entries))
	records := make([]modelwarframe.Relics, 0, len(entries))
	for _, entry := range entries {
		if entry.CodexSecret || entry.UniqueName == "" || seen[entry.Name] {
			continue
		}
		seen[entry.Name] = true
		rewards := make([]modelwarframe.RelicRewards, 0, len(entry.Rewards))
		for _, reward := range entry.Rewards {
			id := reward.ID
			if id == "" {
				id = reward.RewardName
			}
			rewards = append(rewards, modelwarframe.RelicRewards{
				ID:         id,
				RelicsID:   entry.UniqueName,
				RewardName: reward.RewardName,
				Rarity:     reward.Rarity,
				Tier:       reward.Tier,
				ItemCount:  reward.ItemCount,
			})
		}
		records = append(records, modelwarframe.Relics{
			UniqueName:   entry.UniqueName,
			Name:         entry.Name,
			CodexSecret:  entry.CodexSecret,
			Description:  entry.Description,
			RelicRewards: rewards,
		})
	}
	return importer.db.Session(&gorm.Session{FullSaveAssociations: true}).Save(records).Error
}

// ImportRewardPool 从 CDN reward_pool.json 导入奖励池（|COUNT| 运行时替换）。
// CDN 数据的 rarity 是字符串枚举名（如 "COMMON"），经 rarityNameToOrdinal 转 ORDINAL。
func (importer *DataImporter) ImportRewardPool(_ context.Context) error {
	raw, err := cdnFetchFile("warframe/reward_pool.json")
	if err != nil {
		return err
	}
	var records []struct {
		UniqueName string `json:"uniqueName"`
		Rewards    []struct {
			ID        string `json:"id"`
			Item      string `json:"item"`
			Rarity    string `json:"rarity"`
			ItemCount int    `json:"itemCount"`
		} `json:"rewards"`
	}
	if err := json.Unmarshal(raw, &records); err != nil {
		return fmt.Errorf("parse reward_pool.json: %w", err)
	}
	pools := make([]modelwarframe.RewardPool, 0, len(records))
	for _, record := range records {
		if record.UniqueName == "" {
			continue
		}
		rewards := make([]modelwarframe.Reward, 0, len(record.Rewards))
		for _, reward := range record.Rewards {
			id := reward.ID
			if id == "" {
				id = fmt.Sprintf("%s-%s", record.UniqueName, reward.Item)
			}
			item := strings.ReplaceAll(reward.Item, "|COUNT|", fmt.Sprintf("%d", reward.ItemCount))
			rewards = append(rewards, modelwarframe.Reward{
				ID:        id,
				PoolID:    record.UniqueName,
				Item:      item,
				Rarity:    rarityNameToOrdinal(reward.Rarity),
				ItemCount: reward.ItemCount,
			})
		}
		pools = append(pools, modelwarframe.RewardPool{
			UniqueName: record.UniqueName,
			Rewards:    rewards,
		})
	}
	return importer.db.Session(&gorm.Session{FullSaveAssociations: true}).Save(pools).Error
}

// UpdateOrdersItems 更新市场物品数据（对应 POST /data/warframe/market/update）。
func (importer *DataImporter) UpdateOrdersItems(_ context.Context) error {
	return importer.importMarketItems()
}

// UpdateRivenItems 更新紫卡武器数据（对应 POST /data/warframe/market/riven/update）。
func (importer *DataImporter) UpdateRivenItems(_ context.Context) error {
	return importer.importRivenItems()
}

// UpdateLichSister 更新赤毒/信条武器（对应 POST /data/warframe/lich-sister/update）。
func (importer *DataImporter) UpdateLichSister(_ context.Context) error {
	return importer.importLichSisterWeapons()
}

// UpdateEphemeras 更新幻纹（对应 POST /data/warframe/ephemeras/update）。
func (importer *DataImporter) UpdateEphemeras(_ context.Context) error {
	return importer.importEphemeras()
}

// extractEnglishName 从描述提取 "（英文：xxx）" 并转 camelCase（对齐 Java contEnglishName）。
func extractEnglishName(description string) string {
	if !strings.Contains(description, "英文") {
		return ""
	}
	start := strings.Index(description, "（英文：")
	if start < 0 {
		start = strings.Index(description, "(英文:")
		if start < 0 {
			return ""
		}
	}
	start += len("（英文：")
	end := strings.Index(description[start:], "）")
	if end < 0 {
		end = strings.Index(description[start:], ")")
		if end < 0 {
			return ""
		}
	}
	return toCamelCase(description[start : start+end])
}

// toCamelCase 将 "MK1-Braton" 等名称转 camelCase（对齐 Java 规则，MK1- 特判保留大写）。
func toCamelCase(name string) string {
	lower := strings.ToLower(name)
	parts := strings.FieldsFunc(lower, func(r rune) bool { return r == '-' || r == '_' || r == ' ' })
	if len(parts) == 0 {
		return ""
	}
	result := parts[0]
	for _, part := range parts[1:] {
		if part == "" {
			continue
		}
		result += strings.ToUpper(part[:1]) + part[1:]
	}
	return result
}

// computeRivenTrend 从 ExportUpgrades 计算紫卡倾向（对齐 RivenTrendGenerator）。
func (importer *DataImporter) computeRivenTrend(raw []byte) error {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return err
	}
	var entries []struct {
		UniqueName     string `json:"uniqueName"`
		Rarity         string `json:"rarity"`
		UpgradeEntries []struct {
			Tag           string `json:"tag"`
			UpgradeValues []struct {
				Value float64 `json:"value"`
			} `json:"upgradeValues"`
		} `json:"upgradeEntries"`
	}
	if err := json.Unmarshal(envelope["ExportUpgrades"], &entries); err != nil {
		return err
	}

	type trendAccumulator struct {
		rifle, shotgun, pistol, melle, archwing float64
		prefix, suffix                          string
	}
	accumulators := make(map[string]*trendAccumulator)
	for _, entry := range entries {
		if entry.Rarity != "COMMON" || !strings.Contains(entry.UniqueName, "/Mods/Randomized/") {
			continue
		}
		category := detectWeaponCategory(entry.UniqueName)
		if category == "" {
			continue
		}
		for _, upgrade := range entry.UpgradeEntries {
			trendName := tagToTrendName(upgrade.Tag)
			if trendName == "" || len(upgrade.UpgradeValues) == 0 {
				continue
			}
			coefficient := upgrade.UpgradeValues[0].Value
			if coefficient < 0 {
				coefficient = -coefficient
			}
			acc := accumulators[trendName]
			if acc == nil {
				acc = &trendAccumulator{prefix: "-", suffix: "-"}
				accumulators[trendName] = acc
			}
			value := coefficient * 9000
			if nonPercentStat(trendName) {
				value = coefficient * 90
			}
			switch category {
			case "RIFLE":
				if value > acc.rifle {
					acc.rifle = value
				}
			case "SHOTGUN":
				if value > acc.shotgun {
					acc.shotgun = value
				}
			case "PISTOL", "KITGUN":
				if value > acc.pistol {
					acc.pistol = value
				}
			case "MELEE", "ZAW":
				if value > acc.melle {
					acc.melle = value
				}
			case "ARCHWING":
				if value > acc.archwing {
					acc.archwing = value
				}
			}
		}
	}

	records := make([]modelwarframe.RivenAnalyseTrend, 0, len(accumulators))
	for name, acc := range accumulators {
		records = append(records, modelwarframe.RivenAnalyseTrend{
			Name:     name,
			Prefix:   acc.prefix,
			Suffix:   acc.suffix,
			Rifle:    acc.rifle,
			Shotgun:  acc.shotgun,
			Pistol:   acc.pistol,
			Melle:    acc.melle,
			Archwing: acc.archwing,
		})
	}
	return importer.db.Save(records).Error
}

// detectWeaponCategory 识别武器类别（对齐 RivenTrendGenerator 的 uniqueName 规则）。
func detectWeaponCategory(uniqueName string) string {
	switch {
	case strings.Contains(uniqueName, "Archgun"):
		return "ARCHWING"
	case strings.Contains(uniqueName, "ModularMelee"):
		return "ZAW"
	case strings.Contains(uniqueName, "PlayerMelee"):
		return "MELEE"
	case strings.Contains(uniqueName, "ModularPistol"):
		return "KITGUN"
	case strings.Contains(uniqueName, "LotusPistol"):
		return "PISTOL"
	case strings.Contains(uniqueName, "Rifle"):
		return "RIFLE"
	case strings.Contains(uniqueName, "Shotgun"):
		return "SHOTGUN"
	default:
		return ""
	}
}

// tagToTrendName 41 条 tag -> 中文词条名映射（对齐 RivenTrendGenerator 的 TAG_TO_TREND）。
func tagToTrendName(tag string) string {
	names := map[string]string{
		"WeaponDamageAmountMod":                  "伤害",
		"WeaponMeleeDamageAmountMod":             "近战伤害",
		"WeaponCritChanceMod":                    "暴击几率",
		"WeaponCritDamageMod":                    "暴击伤害",
		"WeaponFireRateMod":                      "射速",
		"WeaponMeleeAttackSpeedMod":              "攻击速度",
		"WeaponFireIterationsMod":                "多重射击",
		"WeaponProcTimeMod":                      "触发时间",
		"WeaponStunChanceMod":                    "触发几率",
		"WeaponDamageColdMod":                    "冰元素伤害",
		"WeaponDamageElectricityMod":             "电元素伤害",
		"WeaponDamageHeatMod":                    "火元素伤害",
		"WeaponDamageToxinMod":                   "毒元素伤害",
		"WeaponDamageImpactMod":                  "冲击伤害",
		"WeaponDamagePunctureMod":                "穿刺伤害",
		"WeaponDamageSlashMod":                   "切割伤害",
		"WeaponAmmoMaxMod":                       "弹药最大值",
		"WeaponClipMaxMod":                       "弹匣容量",
		"WeaponRecoilReductionMod":               "后坐力",
		"WeaponReloadSpeedMod":                   "装填速度",
		"WeaponProjectileSpeedMod":               "投射物飞行速度",
		"WeaponPunctureDepthMod":                 "穿透",
		"WeaponZoomFovMod":                       "变焦",
		"WeaponFactionDamageGrinerMod":           "对Grineer伤害",
		"WeaponFactionDamageCorpusMod":           "对Corpus伤害",
		"WeaponFactionDamageInfestationMod":      "对Infested伤害",
		"WeaponMeleeFactionDamageGrinerMod":      "对Grineer伤害",
		"WeaponMeleeFactionDamageCorpusMod":      "对Corpus伤害",
		"WeaponMeleeFactionDamageInfestationMod": "对Infested伤害",
		"WeaponMeleeRangeIncMod":                 "攻击范围",
		"WeaponMeleeFinisherDamageMod":           "处决伤害",
		"WeaponMeleeComboEfficiencyMod":          "重击效率",
		"WeaponMeleeComboInitialBonusMod":        "初始连击",
		"WeaponMeleeComboPointsOnHitMod":         "额外连击数几率",
		"WeaponMeleeComboBonusOnHitMod":          "几率不获得连击数",
		"ComboDurationMod":                       "连击持续时间",
		"SlideAttackCritChanceMod":               "滑行攻击暴击几率",
	}
	return names[tag]
}

// nonPercentStat 判断非百分比类词条（对齐 NON_PCT_STATS）。
func nonPercentStat(name string) bool {
	switch name {
	case "穿透", "连击持续时间", "攻击范围", "初始连击":
		return true
	default:
		return false
	}
}

// cdnFetchFile 从 KingPrimes/DataSource 多 CDN 拉取文件（对齐 CdnTagResolver + ApiDataSourceUtils）。
// 拉取标签（失败回退 latest）→ 依次尝试 CDN，首个 2xx 且 JSON 解析成功者胜。
func cdnFetchFile(path string) ([]byte, error) {
	tag, err := fetchLatestDataSourceTag()
	if err != nil {
		tag = "latest"
	}
	cdns := []string{
		"https://testingcf.jsdelivr.net/gh/KingPrimes/DataSource@" + tag + "/",
		"https://jsd.onmicrosoft.cn/gh/KingPrimes/DataSource@" + tag + "/",
		"https://cdn.jsdelivr.net/gh/KingPrimes/DataSource@" + tag + "/",
		"https://kingprimes.top/",
	}
	var lastErr error
	for _, base := range cdns {
		raw, err := httpGetBytes(base + path)
		if err != nil {
			lastErr = err
			continue
		}
		if !json.Valid(raw) {
			lastErr = fmt.Errorf("invalid JSON from %s", base)
			continue
		}
		return raw, nil
	}
	return nil, fmt.Errorf("all CDN sources failed for %s: %w", path, lastErr)
}

// fetchLatestDataSourceTag 拉取 DataSource 仓库最新 tag（失败回退 latest）。
func fetchLatestDataSourceTag() (string, error) {
	raw, err := httpGetBytes("https://api.github.com/repos/KingPrimes/DataSource/tags")
	if err != nil {
		return "", err
	}
	var tags []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(raw, &tags); err != nil || len(tags) == 0 {
		return "", fmt.Errorf("parse tags failed")
	}
	return tags[0].Name, nil
}

// httpGetBytes 发起简单 GET 请求并读取响应体。
func httpGetBytes(url string) ([]byte, error) {
	client := &http.Client{}
	response, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d from %s", response.StatusCode, url)
	}
	body := make([]byte, 0)
	buffer := make([]byte, 4096)
	for {
		n, readErr := response.Body.Read(buffer)
		body = append(body, buffer[:n]...)
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return nil, readErr
		}
	}
	return body, nil
}

// init 校验 package 级一致性。
func init() {
	if len(stateTypeKeyPatterns) != len(stateTypeNames) {
		panic("state type patterns count mismatch")
	}
}
