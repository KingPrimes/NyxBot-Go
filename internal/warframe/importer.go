// 本地数据导入器，对应 Java NyxBot 的 WarframeDataSource
// 分阶段导入：Phase 0 下载导出文件 → Phase 1 状态翻译 → Phase 2 各业务表
// 落库策略对齐 Java：按主键 merge 的 upsert（非全量清空），CDN 数据按业务键先查后写
package warframe

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

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

// importBatchSize 分批写入条数。SQLite 单条 SQL 绑定变量有上限（32766），
// 按最大字段数（Weapons 24 字段）保守取值，500 条 × 24 = 12000 不会触限。
const importBatchSize = 500

// batchSave 分批执行 Save，规避 SQLite "too many SQL variables" 错误。
// fullAssociations 为 true 时携带关联保存（对齐 FullSaveAssociations 语义）。
func batchSave[T any](db *gorm.DB, records []T, fullAssociations bool) error {
	for start := 0; start < len(records); start += importBatchSize {
		end := start + importBatchSize
		if end > len(records) {
			end = len(records)
		}
		session := db
		if fullAssociations {
			session = db.Session(&gorm.Session{FullSaveAssociations: true})
		}
		if err := session.Save(records[start:end]).Error; err != nil {
			return err
		}
	}
	return nil
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

// productCategoryNameToOrdinal ProductCategory 枚举名 → ORDINAL（对齐 model 包映射，未知回退 LongGuns(1)）。
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
	return batchSave(importer.db, records, false)
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
	return batchSave(importer.db, records, false)
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
	return batchSave(importer.db, records, false)
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
	return batchSave(importer.db, records, false)
}

// ParseStateTranslationExport 解析单个官方导出文件为状态翻译记录
// （对齐 Java StateTranslationService.parseFromExport）：
// 中文导出文件（ExportXxx_zh.json）的名称字段是 name（Java 实体为 @JsonProperty("name")），
// englishName 仅作为英文导出的兼容兜底；name 为空的条目按 Java 语义跳过（filter(!getName().isEmpty())）；
// type 先按 StateTypeEnum.KEY 正则整串匹配 uniqueName，未命中回退 fallbackType（来源默认类型）。
func ParseStateTranslationExport(raw []byte, key string, fallbackType int) []modelwarframe.StateTranslation {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil
	}
	listRaw, ok := envelope[key]
	if !ok {
		return nil
	}
	var entries []struct {
		UniqueName  string          `json:"uniqueName"`
		Name        string          `json:"name"`
		EnglishName string          `json:"englishName"`
		Description json.RawMessage `json:"description"`
	}
	if err := json.Unmarshal(listRaw, &entries); err != nil {
		return nil
	}
	records := make([]modelwarframe.StateTranslation, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name
		if name == "" {
			name = entry.EnglishName
		}
		if entry.UniqueName == "" || name == "" {
			continue
		}
		records = append(records, modelwarframe.StateTranslation{
			UniqueName:  entry.UniqueName,
			Name:        name,
			Description: extractFirstDescription(entry.Description),
			Type:        exportTypeOrdinal(entry.UniqueName, fallbackType),
		})
	}
	return records
}

// ImportStateTranslation 从导出文件导入状态翻译（Phase 1）。
// 对齐 Java StateTranslationService.initData：按 EXPORT_SOURCES 读取导出文件，
// 再合并 CDN state_translation.json（CDN 条目按 uniqueName 正则匹配 StateTypeEnum.KEY，未匹配回退 RESOURCES）。
func (importer *DataImporter) ImportStateTranslation(_ context.Context) error {
	records := make([]modelwarframe.StateTranslation, 0, 2048)

	for _, source := range exportSources {
		raw, err := importer.exporter.ReadExportFile(source.prefix)
		if err != nil || len(raw) == 0 {
			continue
		}
		records = append(records, ParseStateTranslationExport(raw, source.key, source.state)...)
	}

	// CDN state_translation.json：按 uniqueName 正则匹配 StateTypeEnum.KEY 覆盖 type（未匹配回退 RESOURCES）。
	if cdnRaw, cdnErr := cdnFileFetcher("warframe/state_translation.json"); cdnErr == nil {
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
					Type:        stateTypeMatchOrdinal(entry.UniqueName, entry.Type),
					ParentName:  entry.ParentName,
				})
			}
		}
	}

	if len(records) == 0 {
		return fmt.Errorf("no state translation records from export files")
	}
	if err := batchSave(importer.db, records, false); err != nil {
		return err
	}
	// 清理历史空名行：旧实现读错了字段（englishName）导致大量 name 为空的记录入库，
	// 空名记录无翻译价值（Java 侧 @NotEmpty 且导入时已过滤），此处一并清掉。
	if err := importer.db.Exec("DELETE FROM state_translation WHERE name IS NULL OR TRIM(name) = ''").Error; err != nil {
		logging.WarnPack("warframe.import", "clean empty state translation failed: %v", err)
	}
	return nil
}

// stateTypeKeyPatterns StateType 正则（从 modelwarframe.StateTypes 编译，下标即枚举 ORDINAL）。
// Java 用 String.matches() 整串匹配语义，故统一锚定为 ^(?:...)$；空 KEY 编译为 ^(?:)$，永不出现在真实数据上。
var stateTypeKeyPatterns = compileStateTypePatterns()

// compileStateTypePatterns 把 StateTypes 表的 KEY 源串编译为整串匹配正则（对齐 Java String.matches）。
func compileStateTypePatterns() []*regexp.Regexp {
	patterns := make([]*regexp.Regexp, 0, len(modelwarframe.StateTypes))
	for _, definition := range modelwarframe.StateTypes {
		patterns = append(patterns, regexp.MustCompile("^(?:"+definition.Key+")$"))
	}
	return patterns
}

// exportTypeOrdinal 确定导出文件条目的 StateType 序数（对齐 Java StateTranslationService.parseFromExport）：
// 先按 StateTypeEnum.KEY 正则整串匹配 uniqueName，未命中回退来源默认类型 fallback
// （fallback 即 exportSources 表中对应来源的 type）。
func exportTypeOrdinal(uniqueName string, fallback int) int {
	for index, pattern := range stateTypeKeyPatterns {
		if pattern.MatchString(uniqueName) {
			return index
		}
	}
	return fallback
}

// stateTypeMatchOrdinal 确定 CDN 翻译条目的 type 序数：JSON 自带 type（枚举名）优先，
// 否则按 uniqueName 正则匹配 StateTypeEnum.KEY，仍未匹配回退 RESOURCES。
func stateTypeMatchOrdinal(uniqueName string, rawType json.RawMessage) int {
	if len(rawType) > 0 && string(rawType) != "null" {
		var typeName string
		if err := json.Unmarshal(rawType, &typeName); err == nil && typeName != "" {
			if ordinal, ok := modelwarframe.StateTypeOrdinal(typeName); ok {
				return ordinal
			}
		}
	}
	return exportTypeOrdinal(uniqueName, modelwarframe.StateTypeNameToOrdinal("RESOURCES"))
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
	raw, err := cdnFileFetcher("warframe/alias.json")
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
	return batchSave(importer.db, upserts, false)
}

// ImportRivenTion 从 CDN market_riven_tion.json 导入（按 urlName 业务键智能更新）。
// CDN 数据的 negative_only 等字段是字符串（如 "0"），用 flexibleNumber 兼容。
func (importer *DataImporter) ImportRivenTion(_ context.Context) error {
	raw, err := cdnFileFetcher("warframe/market_riven_tion.json")
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
	return batchSave(importer.db, upserts, false)
}

// ImportRivenTionAlias 从 CDN market_riven_tion_alias.json 导入（按 en|cn 组合键）。
func (importer *DataImporter) ImportRivenTionAlias(_ context.Context) error {
	raw, err := cdnFileFetcher("warframe/market_riven_tion_alias.json")
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
	return batchSave(importer.db, upserts, false)
}

// ImportRivenAnalyseTrend 从 ExportUpgrades 本地计算（对齐 RivenTrendGenerator）。
func (importer *DataImporter) ImportRivenAnalyseTrend(_ context.Context) error {
	raw, err := importer.exporter.ReadExportFile("ExportUpgrades")
	if err != nil || len(raw) == 0 {
		return fmt.Errorf("ExportUpgrades file unavailable")
	}
	return importer.computeRivenTrend(raw)
}

// ParseNodesExport 解析 ExportRegions 官方导出文件为节点记录
// （对齐 Java NodeService.initFromExportFile：取顶层 ExportRegions 数组直接映射）。
// uniqueName 为空的条目丢弃（空主键入库只会留下永远匹配不到的垃圾行）。
func ParseNodesExport(raw []byte) ([]modelwarframe.Nodes, error) {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, err
	}
	var entries []modelwarframe.Nodes
	if err := json.Unmarshal(envelope["ExportRegions"], &entries); err != nil {
		return nil, err
	}
	return dropNodesWithoutKey(entries), nil
}

// ParseNodesCDN 解析 CDN warframe/nodes.json（对齐 Java NodeService.initFromCdn）：
// 文件是裸 JSON 数组，字段与官方导出同构（社区维护的补充节点：九重天、活动节点等）。
func ParseNodesCDN(raw []byte) ([]modelwarframe.Nodes, error) {
	var entries []modelwarframe.Nodes
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, fmt.Errorf("parse CDN nodes.json: %w", err)
	}
	return dropNodesWithoutKey(entries), nil
}

// dropNodesWithoutKey 过滤 uniqueName 为空的节点条目。
func dropNodesWithoutKey(entries []modelwarframe.Nodes) []modelwarframe.Nodes {
	records := make([]modelwarframe.Nodes, 0, len(entries))
	for _, entry := range entries {
		if entry.UniqueName == "" {
			continue
		}
		records = append(records, entry)
	}
	return records
}

// ImportNodes 导入星图节点：官方导出 ExportRegions + CDN warframe/nodes.json
// （对齐 Java NodeService.initData：initFromExportFile → initFromCdn）。
// CDN 只补官方导出没有的节点（九重天/活动节点等），不覆盖同名节点：
// Java 用 saveAll 按主键整行覆盖，会把导出更准的字段冲掉——实测唯一重叠的 SolNode229，
// 导出是 factionIndex=2 / 星系序号 16 / 等级 15-30，CDN 是 1 / 0 / 0-0，
// 而 factionIndex 会用于世界状态入侵/警报的派系渲染（nodeFactionByKey），属实现缺陷，不照搬。
// CDN 拉取/解析失败同样只告警、保留已导入的官方导出数据（Java 的 @Transactional 会连带回滚导出节点）。
func (importer *DataImporter) ImportNodes(_ context.Context) error {
	raw, err := importer.exporter.ReadExportFile("ExportRegions")
	if err != nil || len(raw) == 0 {
		return fmt.Errorf("ExportRegions file unavailable")
	}
	exportRecords, err := ParseNodesExport(raw)
	if err != nil {
		return err
	}
	if err := batchSave(importer.db, exportRecords, false); err != nil {
		return err
	}

	cdnRaw, cdnErr := cdnFileFetcher("warframe/nodes.json")
	if cdnErr != nil {
		logging.WarnPack("warframe.import",
			"CDN nodes.json unavailable, kept %d nodes from ExportRegions: %v", len(exportRecords), cdnErr)
		return nil
	}
	cdnRecords, err := ParseNodesCDN(cdnRaw)
	if err != nil {
		logging.WarnPack("warframe.import",
			"CDN nodes.json invalid, kept %d nodes from ExportRegions: %v", len(exportRecords), err)
		return nil
	}
	exportKeys := make(map[string]struct{}, len(exportRecords))
	for _, record := range exportRecords {
		exportKeys[record.UniqueName] = struct{}{}
	}
	additions := make([]modelwarframe.Nodes, 0, len(cdnRecords))
	for _, record := range cdnRecords {
		if _, exists := exportKeys[record.UniqueName]; exists {
			continue
		}
		additions = append(additions, record)
	}
	if err := batchSave(importer.db, additions, false); err != nil {
		return err
	}
	logging.InfoPack("warframe.import",
		"nodes imported: %d from ExportRegions, %d from CDN nodes.json (%d already in export)",
		len(exportRecords), len(additions), len(cdnRecords)-len(additions))
	return nil
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
		TotalDamage        float64   `json:"totalDamage"` // 真实数据含浮点（如 45.999996），入库截断对齐 Java
		Description        string    `json:"description"`
		CriticalChance     float64   `json:"criticalChance"`
		CriticalMultiplier float64   `json:"criticalMultiplier"`
		ProcChance         float64   `json:"procChance"`
		FireRate           float64   `json:"fireRate"`
		MasteryReq         int       `json:"masteryReq"`
		ProductCategory    string    `json:"productCategory"` // 枚举名（如 "Pistols"），入库转 ORDINAL
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
			TotalDamage:        int(entry.TotalDamage),
			Description:        entry.Description,
			EnglishName:        extractEnglishName(entry.Description),
			CriticalChance:     entry.CriticalChance,
			CriticalMultiplier: entry.CriticalMultiplier,
			ProcChance:         entry.ProcChance,
			FireRate:           entry.FireRate,
			MasteryReq:         entry.MasteryReq,
			ProductCategory:    productCategoryNameToOrdinal(entry.ProductCategory),
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
	return batchSave(importer.db, records, false)
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
	return batchSave(importer.db, records, false)
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
		UniqueName      string  `json:"uniqueName"`
		Name            string  `json:"name"`
		ParentName      string  `json:"parentName"`
		Description     string  `json:"description"`
		Health          int     `json:"health"`
		Shield          int     `json:"shield"`
		Armor           int     `json:"armor"`
		Stamina         int     `json:"stamina"`
		Power           int     `json:"power"`
		CodexSecret     bool    `json:"codexSecret"`
		MasteryReq      int     `json:"masteryReq"`
		SprintSpeed     float64 `json:"sprintSpeed"` // 真实数据含浮点（如 1.1），入库截断对齐 Java
		ProductCategory string  `json:"productCategory"`
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
			SprintSpeed:     int(entry.SprintSpeed),
			ProductCategory: entry.ProductCategory,
			Abilities:       abilities,
		})
	}
	// 技能子表完全由导出文件派生：先校正主键结构（DDL 独立于事务），再在同一事务里清空 + 整体写入
	// （对齐 Java @OneToMany(orphanRemoval = true)，避免旧行残留与旧单列主键结构阻止写入；
	// 写入失败时回滚删除，避免技能数据被清空后无法恢复）。
	if err := importer.ensureAbilitiesSchema(); err != nil {
		return err
	}
	return importer.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).
			Delete(&modelwarframe.WarframesAbility{}).Error; err != nil {
			return fmt.Errorf("clear warframe abilities: %w", err)
		}
		return batchSave(tx, records, true)
	})
}

// ensureAbilitiesSchema 校验 abilities 表的主键结构：
// 旧库按 Java 实体的单列主键（ability_unique_name）建立，无法保存基础版/Prime 共用的技能行，
// 检测到旧结构时重建子表（子表完全由导出文件派生，重建无数据损失）。
func (importer *DataImporter) ensureAbilitiesSchema() error {
	var primaryKeys int64
	if err := importer.db.Raw("SELECT COUNT(*) FROM pragma_table_info('abilities') WHERE pk > 0").
		Scan(&primaryKeys).Error; err != nil {
		return fmt.Errorf("inspect abilities primary key: %w", err)
	}
	// primaryKeys == 0 覆盖两种情况：表尚未创建（pragma_table_info 对不存在的表返回 0 行而非报错）
	// 与旧的单列主键结构——两者都按新结构重建，DROP 对不存在的表是空操作。
	if primaryKeys >= 2 {
		return nil
	}
	if err := importer.db.Migrator().DropTable(&modelwarframe.WarframesAbility{}); err != nil {
		return fmt.Errorf("rebuild abilities table: %w", err)
	}
	return importer.db.AutoMigrate(&modelwarframe.WarframesAbility{})
}

// ParseRelicExport 解析 ExportRelicArcane 导出文件（对齐 Java RelicsImportUtil.readRelicsFromFile +
// 按 name 去重 + filterSecretRelics）。奖励行主键取
// "{遗物uniqueName}|{奖励原始uniqueName}|{稀有度}|{等级}|{数量}"：
// 导出文件不含奖励 id，且同一奖励物品会被多个遗物共用，用奖励物品名当主键会造成跨遗物互相覆盖
// （Java 侧是 @GeneratedValue(UUID)，每个遗物各自成行）。奖励名此处保留原始 uniqueName，
// 由 TranslateRelicRewardNames 翻译。
func ParseRelicExport(raw []byte) ([]modelwarframe.Relics, error) {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, err
	}
	var entries []struct {
		UniqueName  string `json:"uniqueName"`
		Name        string `json:"name"`
		CodexSecret bool   `json:"codexSecret"`
		Description string `json:"description"`
		Rewards     []struct {
			RewardName string `json:"rewardName"`
			Rarity     string `json:"rarity"` // 枚举名（如 "COMMON"），入库转 ORDINAL
			Tier       int    `json:"tier"`
			ItemCount  int    `json:"itemCount"`
		} `json:"relicRewards"`
	}
	if err := json.Unmarshal(envelope["ExportRelicArcane"], &entries); err != nil {
		return nil, err
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
			if reward.RewardName == "" {
				continue
			}
			rewards = append(rewards, modelwarframe.RelicRewards{
				// 确定性主键：遗物 + 奖励 uniqueName + 稀有度/等级/数量（理由见 ParseRelicExport 注释）
				ID: fmt.Sprintf("%s|%s|%s|%d|%d",
					entry.UniqueName, reward.RewardName, reward.Rarity, reward.Tier, reward.ItemCount),
				RelicsID:   entry.UniqueName,
				RewardName: reward.RewardName,
				Rarity:     rarityNameToOrdinal(reward.Rarity),
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
	return records, nil
}

// TranslateRelicRewardNames 把遗物奖励名翻译为中文（对齐 Java RelicsImportUtil.loadTranslationMap +
// translateReward）：关键词取奖励 uniqueName 的末三段（StringUtils.getLastThreeSegments）并去重，
// 一次性批量查询 state_translation（对齐 Java 的 OR LIKE 规格查询，避免逐条 4000+ 次单查），
// 未命中保留原始路径并计入返回值（返回值由 ImportRelics 收集成未翻译清单导出）。
func TranslateRelicRewardNames(relics []modelwarframe.Relics) []string {
	keywords := make([]string, 0, 1024)
	seen := make(map[string]bool, 1024)
	for index := range relics {
		for reward := range relics[index].RelicRewards {
			key := getLastThreeSegments(relics[index].RelicRewards[reward].RewardName)
			if key == "" || seen[key] {
				continue
			}
			seen[key] = true
			keywords = append(keywords, key)
		}
	}
	translations := LoadStateTranslationMap(keywords)

	untranslated := make([]string, 0)
	for index := range relics {
		for reward := range relics[index].RelicRewards {
			raw := relics[index].RelicRewards[reward].RewardName
			name := translations[getLastThreeSegments(raw)]
			if name == "" {
				untranslated = append(untranslated, raw)
				continue
			}
			relics[index].RelicRewards[reward].RewardName = name
		}
	}
	return untranslated
}

// ImportRelics 从 ExportRelicArcane 导入遗物
// （对齐 Java RelicsImportUtil.importRelicsData：解析 → 过滤机密 → 按 name 去重 → 翻译奖励名 → 落库）。
// 落库前清空 relic_rewards：子表完全由该导出文件派生，对齐 Java @OneToMany(orphanRemoval = true) 语义，
// 同时清掉旧主键方案（直接用奖励物品名当 id）留下的脏数据。
// 清空与写入放在同一事务：写入失败时回滚删除，避免奖励数据被清空后无法恢复。
func (importer *DataImporter) ImportRelics(_ context.Context) error {
	raw, err := importer.exporter.ReadExportFile("ExportRelicArcane")
	if err != nil || len(raw) == 0 {
		return fmt.Errorf("ExportRelicArcane file unavailable")
	}
	records, err := ParseRelicExport(raw)
	if err != nil {
		return err
	}
	if len(records) == 0 {
		return fmt.Errorf("no relic records in ExportRelicArcane")
	}
	untranslated := TranslateRelicRewardNames(records)
	if err := importer.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).
			Delete(&modelwarframe.RelicRewards{}).Error; err != nil {
			return fmt.Errorf("clear relic rewards: %w", err)
		}
		return batchSave(tx, records, true)
	}); err != nil {
		return err
	}
	rewardCount := 0
	for _, record := range records {
		rewardCount += len(record.RelicRewards)
	}
	// 未翻译奖励清单（对齐 Java RelicsImportUtil.exportUntranslatedItems→
	// ./data/UntranslatedRelicsRewardsName.json）：按 uniqueName 合并进现有清单，已在文件中的不重复添加，
	// 人工填写的译名不会被覆盖；运行期世界状态的未命中项也落在同一份文件里。
	// 放在落库成功之后：导入失败时不该留下"待翻译"报告；写文件失败只告警，不影响已落库的数据
	//（Java 侧此处抛 IOException 会连带回滚事务，属实现缺陷，不照搬）。
	untranslatedItems := CollectUntranslatedItems(untranslated)
	added, mergeErr := MergeUntranslatedItems(untranslatedItems)
	if mergeErr != nil {
		logging.WarnPack("warframe.import", "merge untranslated relic rewards into %s failed: %v",
			UntranslatedListPath(), mergeErr)
	}
	logging.InfoPack("warframe.import",
		"relics imported: %d relics, %d rewards, %d reward names untranslated (%d newly recorded in %s)",
		len(records), rewardCount, len(untranslatedItems), added, UntranslatedListPath())
	return nil
}

// ImportRewardPool 从 CDN reward_pool.json 导入奖励池（|COUNT| 运行时替换）。
// CDN 数据的 rarity 是字符串枚举名（如 "COMMON"），经 rarityNameToOrdinal 转 ORDINAL。
func (importer *DataImporter) ImportRewardPool(_ context.Context) error {
	raw, err := cdnFileFetcher("warframe/reward_pool.json")
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
	return batchSave(importer.db, pools, true)
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

// UpdateTranslation 组合更新全部翻译数据（对齐 Java UpdateWarframeTar）：
// 先导状态翻译，再顺序节点/武器/奖励池/电波翻译。
func (importer *DataImporter) UpdateTranslation(ctx context.Context) error {
	if err := importer.ImportStateTranslation(ctx); err != nil {
		return err
	}
	steps := []func(context.Context) error{
		importer.ImportNodes,
		importer.ImportWeapons,
		importer.ImportRewardPool,
		importer.ImportNightWave,
	}
	for _, step := range steps {
		if err := step(ctx); err != nil {
			return err
		}
	}
	return nil
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
	return batchSave(importer.db, records, false)
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

// cdnFileFetcher 当前的 CDN 数据文件拉取实现（默认多 CDN 回退；黑盒测试可经
// SetCDNFileFetcher 换成本地 httptest 服务，避免测试打真实 CDN）。
var cdnFileFetcher = cdnFetchFile

// SetCDNFileFetcher 替换进程级 CDN 数据文件拉取实现，返回原实现（供黑盒测试注入本地服务器）。
// 传 nil 时回退默认实现。
func SetCDNFileFetcher(fetch func(path string) ([]byte, error)) func(path string) ([]byte, error) {
	previous := cdnFileFetcher
	if fetch == nil {
		cdnFileFetcher = cdnFetchFile
		return previous
	}
	cdnFileFetcher = fetch
	return previous
}

// cdnFetchFile 从 KingPrimes/DataSource 多 CDN 拉取文件（对齐 CdnTagResolver + ApiDataSourceUtils）。
// 拉取标签（失败回退 latest）→ 依次尝试 CDN（每个带网络重试），首个 2xx 且 JSON 解析成功者胜。
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

// httpGetBytes 发起带重试的 GET 请求并读取响应体（网络错误与 429/5xx 重试 3 次）。
func httpGetBytes(url string) ([]byte, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	return doRequestWithRetry(context.Background(), client, http.MethodGet, url, nil)
}

// init 校验 package 级一致性：正则表必须与 StateTypes 表逐项对应（下标即 ORDINAL）。
func init() {
	if len(stateTypeKeyPatterns) != len(modelwarframe.StateTypes) {
		panic("state type patterns count mismatch")
	}
}
