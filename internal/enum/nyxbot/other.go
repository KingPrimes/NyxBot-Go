// NyxBot 其他枚举（AsyncBeanName、内部工具枚举）
package nyxbot

type AsyncBeanName string

const (
	AsyncMyAsync  AsyncBeanName = "MYASYNC"
	AsyncService  AsyncBeanName = "SERVICE"
	AsyncInitData AsyncBeanName = "InitData"
)

// Str 返回异步执行器的 Bean 名称字符串。
func (a AsyncBeanName) Str() string {
	mp := map[AsyncBeanName]string{
		AsyncMyAsync: "myAsync", AsyncService: "scheduledExecutorService",
		AsyncInitData: "initDataExecutor",
	}
	return mp[a]
}

// WeaponCategory 武器分类（RivenTrendGenerator 内部枚举）
type WeaponCategory string

const (
	WeaponCatRifle    WeaponCategory = "RIFLE"
	WeaponCatShotgun  WeaponCategory = "SHOTGUN"
	WeaponCatPistol   WeaponCategory = "PISTOL"
	WeaponCatKitgun   WeaponCategory = "KITGUN"
	WeaponCatMelee    WeaponCategory = "MELEE"
	WeaponCatZaw      WeaponCategory = "ZAW"
	WeaponCatArchwing WeaponCategory = "ARCHWING"
)

// Polarity 极性（MarketRivenUtils 内部枚举）
type Polarity string

const (
	PolarityAny     Polarity = "ANY"
	PolarityMadurai Polarity = "MADURAI"
	PolarityVazarin Polarity = "VAZARIN"
	PolarityNaramon Polarity = "NARAMON"
)

// SearchType 玄骸/姐妹搜索类型
type SearchType string

const (
	SearchTypeLich   SearchType = "LICH"
	SearchTypeSister SearchType = "SISTER"
)

// Type 返回搜索类型的小写 API 参数。
func (s SearchType) Type() string {
	mp := map[SearchType]string{
		SearchTypeLich: "lich", SearchTypeSister: "sister",
	}
	return mp[s]
}

// SearchElement 玄骸/姐妹搜索元素（MarketLichSisterUtils）
type SearchElement string

const (
	SearchElemCold        SearchElement = "COLD"
	SearchElemRadiation   SearchElement = "RADIATION"
	SearchElemHeat        SearchElement = "HEAT"
	SearchElemMagnetic    SearchElement = "MAGNETIC"
	SearchElemToxin       SearchElement = "TOXIN"
	SearchElemElectricity SearchElement = "ELECTRICITY"
	SearchElemImpact      SearchElement = "IMPACT"
	SearchElemAny         SearchElement = "ANY"
)

// DucatsType 杜卡德币统计类型
type DucatsType string

const (
	DucatsSilver DucatsType = "SILVER"
	DucatsGod    DucatsType = "GOD"
)

// ProductCategory 武器产品分类（Weapons 嵌套枚举）
type ProductCategory string

const (
	ProductPistols         ProductCategory = "Pistols"
	ProductLongGuns        ProductCategory = "LongGuns"
	ProductMelee           ProductCategory = "Melee"
	ProductSpaceGuns       ProductCategory = "SpaceGuns"
	ProductSpaceMelee      ProductCategory = "SpaceMelee"
	ProductSpecialItems    ProductCategory = "SpecialItems"
	ProductCrewShipWeapons ProductCategory = "CrewShipWeapons"
	ProductSentinelWeapons ProductCategory = "SentinelWeapons"
	ProductShotguns        ProductCategory = "Shotguns"
)

// Name 返回产品分类的中文名称。
func (p ProductCategory) Name() string {
	mp := map[ProductCategory]string{
		ProductPistols: "次要武器", ProductLongGuns: "主要武器",
		ProductMelee: "近战武器", ProductSpaceGuns: "空战武器",
		ProductSpaceMelee: "空战近战", ProductSpecialItems: "特殊物品",
		ProductCrewShipWeapons: "舰员武器", ProductSentinelWeapons: "守护武器",
		ProductShotguns: "霰弹枪",
	}
	return mp[p]
}

// DataRefreshStatus 数据刷新状态
type DataRefreshStatus string

const (
	RefreshStarted   DataRefreshStatus = "STARTED"
	RefreshCompleted DataRefreshStatus = "COMPLETED"
	RefreshFailed    DataRefreshStatus = "FAILED"
)
