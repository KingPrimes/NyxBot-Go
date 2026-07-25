// WorldState 远程 API 数据模型，参考 draw-image-plugin 的 WorldState 系列 POJO
// 非数据库模型，用于反序列化 Warframe 官方 WorldState API 的 JSON 响应
// 包含：地球/Cetus/金星/火卫二昼夜循环、突击、裂缝、入侵、活动、Baro、每日优惠、仲裁等
package warframe

// WorldStateBase 所有具有 ID + 开始/结束时间的模型公用的嵌入基类
type WorldStateBase struct {
	ID         string `json:"id"`         // 唯一标识符
	Activation string `json:"activation"` // 开始时间
	Expiry     string `json:"expiry"`     // 结束时间
}

type WorldState struct {
	NoManifest bool `json:"NoManifest"` // 是否无清单模式
}

type CycleInfo struct {
	WorldStateBase
	IsDay    bool   `json:"isDay"`    // 是否为白天
	TimeLeft string `json:"timeLeft"` // 剩余时间
	State    string `json:"state"`    // 状态描述
}

type CetusCycle struct {
	WorldStateBase
	IsDay       bool   `json:"isDay"`       // 是否为白天
	TimeLeft    string `json:"timeLeft"`    // 剩余时间
	IsCetus     bool   `json:"isCetus"`     // 是否为 Cetus 平原
	State       string `json:"state"`       // 状态（白天/夜晚）
	ShortString string `json:"shortString"` // 简短状态文字
}

type VallisCycle struct {
	WorldStateBase
	IsWarm      bool   `json:"isWarm"`      // 是否为温暖状态
	TimeLeft    string `json:"timeLeft"`    // 剩余时间
	IsVallis    bool   `json:"isVallis"`    // 是否为金星平原
	State       string `json:"state"`       // 状态（温暖/寒冷）
	ShortString string `json:"shortString"` // 简短状态文字
}

type CambionCycle struct {
	WorldStateBase
	Active      string `json:"active"`      // 当前活跃状态
	TimeLeft    string `json:"timeLeft"`    // 剩余时间
	IsCambion   bool   `json:"isCambion"`   // 是否为火卫二平原
	State       string `json:"state"`       // 状态
	ShortString string `json:"shortString"` // 简短状态文字
}

type Sortie struct {
	WorldStateBase
	Faction  string          `json:"faction"`  // 敌对派系
	Boss     string          `json:"boss"`     // 头目名称
	Variants []SortieVariant `json:"variants"` // 阶段列表
	Reward   []SortieReward  `json:"reward"`   // 奖励列表
}

type SortieVariant struct {
	Node         string `json:"node"`                // 节点名称
	MissionType  string `json:"missionType"`         // 任务类型
	Modifier     string `json:"modifier"`            // modifier 类型
	ModifierDesc string `json:"modifierDescription"` // modifier 描述
}

type SortieReward struct {
	ItemName string `json:"itemName"` // 奖励物品名称
}

type Fissure struct {
	WorldStateBase
	Tier        string `json:"tier"`        // 纪元等级（Lith/Meso/Neo/Axi）
	Node        string `json:"node"`        // 节点名称
	Mission     string `json:"mission"`     // 任务名称
	MissionType string `json:"missionType"` // 任务类型
	Eta         string `json:"eta"`         // 剩余时间文字
	IsStorm     bool   `json:"isStorm"`     // 是否为风暴
	IsHard      bool   `json:"isHard"`      // 是否为钢铁之路
}

type Invasion struct {
	WorldStateBase
	Node           string          `json:"node"`           // 节点名称
	Faction        string          `json:"faction"`        // 星球派系
	Attacker       string          `json:"attacker"`       // 攻击方
	Defender       string          `json:"defender"`       // 防守方
	AttackerReward *InvasionReward `json:"attackerReward"` // 攻击方奖励
	DefenderReward *InvasionReward `json:"defenderReward"` // 防守方奖励
	Count          int             `json:"count"`          // 当前进度
	RequiredCount  int             `json:"requiredCount"`  // 所需进度
	Completion     float64         `json:"completion"`     // 完成百分比
	VS             string          `json:"vs"`             // 对抗文字（攻 vs 守）
	Eta            string          `json:"eta"`            // 剩余时间
}

type InvasionReward struct {
	ItemName  string `json:"itemName"`  // 奖励物品
	Count     int    `json:"count"`     // 数量
	Thumb     string `json:"thumb"`     // 缩略图
	Credits   int    `json:"credits"`   // 星币数量
	IsCredits bool   `json:"isCredits"` // 是否为星币奖励
}

type Event struct {
	WorldStateBase
	Description   string       `json:"description"`    // 描述
	Tooltip       string       `json:"tooltip"`        // 提示文字
	Node          string       `json:"node"`           // 节点
	MaximumScore  int          `json:"maximumScore"`   // 最高分数
	CurrentScore  int          `json:"currentScore"`   // 当前分数
	Health        int          `json:"health"`         // 生命值
	Reward        *EventReward `json:"rewards"`        // 最终奖励
	InterimGoal   int          `json:"interimGoals"`   // 阶段性目标
	IsPersonal    bool         `json:"isPersonal"`     // 是否为个人活动
	InterimReward *EventReward `json:"interimRewards"` // 阶段性奖励
}

type EventReward struct {
	Credits int               `json:"credits"` // 星币
	Items   []EventRewardItem `json:"items"`   // 物品列表
}

type EventRewardItem struct {
	ItemName string `json:"itemName"` // 物品名称
	Thumb    string `json:"thumb"`    // 缩略图
	Count    int    `json:"count"`    // 数量
}

type VoidTrader struct {
	WorldStateBase
	Character string           `json:"character"` // 商人名称
	Location  string           `json:"location"`  // 所在中继站
	Inventory []VoidTraderItem `json:"inventory"` // 商品列表
	IsBaro    bool             `json:"isBaro"`    // 是否 Baro
}

type VoidTraderItem struct {
	ItemName   string `json:"itemName"`   // 物品名称
	Thumb      string `json:"thumb"`      // 缩略图
	Ducats     int    `json:"ducats"`     // 杜卡德金币价格
	Credits    int    `json:"credits"`    // 星币价格
	PrimePrice int    `json:"primePrice"` // Prime 价格
}

type DailyDeal struct {
	WorldStateBase
	ItemName     string `json:"itemName"`     // 物品名称
	Thumb        string `json:"thumb"`        // 缩略图
	OriginalCost int    `json:"originalCost"` // 原价
	SaleCost     int    `json:"saleCost"`     // 优惠价
	Total        int    `json:"total"`        // 总库存
	Sold         int    `json:"sold"`         // 已售出
	Remaining    int    `json:"remaining"`    // 剩余库存
}

type ConstructionProgress struct {
	WorldStateBase
	Fomorian  *Progress `json:"fomorian"`  // 弗摩尔战舰进度
	Razorback *Progress `json:"razorback"` // 机械脊蛛进度
}

type Progress struct {
	Health    float64  `json:"health"`    // 当前生命
	MaxHealth float64  `json:"maxHealth"` // 最大生命
	Attackers int      `json:"attackers"` // 攻击者数量
	Locations []string `json:"locations"` // 位置列表
	IsActive  bool     `json:"isActive"`  // 是否活跃
}

type Arbitration struct {
	WorldStateBase
	Node     string `json:"node"`     // 节点
	Enemy    string `json:"enemy"`    // 敌人派系
	EnemyLv  int    `json:"enemyLv"`  // 敌人等级
	Type     string `json:"type"`     // 任务类型
	IsArchon bool   `json:"isArchon"` // 是否执刑官猎
}

type ArchonHunt struct {
	ID       string              `json:"id"`       // 执刑官猎 ID
	Boss     string              `json:"boss"`     // BOSS 名称
	Missions []ArchonHuntMission `json:"missions"` // 任务阶段列表
}

type ArchonHuntMission struct {
	Node        string `json:"node"`        // 节点
	MissionType string `json:"missionType"` // 任务类型
	Modifier    string `json:"modifier"`    // modifier
}

type Simaris struct {
	WorldStateBase
	Target   string `json:"target"`   // 目标名称
	Standing int    `json:"standing"` // 声望值
}

type ConclaveChallenge struct {
	WorldStateBase
	Mode          string `json:"mode"`          // 模式
	Category      string `json:"category"`      // 分类
	Standing      int    `json:"standing"`      // 声望
	Description   string `json:"description"`   // 描述
	Eta           string `json:"eta"`           // 剩余时间
	RootChallenge string `json:"rootChallenge"` // 根挑战 ID
}

type LightRating struct {
	ID    string `json:"id"`    // 评分 ID
	Value string `json:"value"` // 评分值
}

type ActiveMission struct {
	Node           string   `json:"node"`           // 节点名称
	Enemy          string   `json:"enemy"`          // 敌人派系
	EnemyLv        int      `json:"enemyLv"`        // 敌人等级
	MinEnemyLevel  int      `json:"minEnemyLevel"`  // 最低敌人等级
	MaxEnemyLevel  int      `json:"maxEnemyLevel"`  // 最高敌人等级
	Type           string   `json:"type"`           // 任务类型
	ExclusiveEnemy string   `json:"exclusiveEnemy"` // 专属敌人
	Factions       []string `json:"factions"`       // 可用派系
	Job            any      `json:"job"`            // 额外工作数据
}

type Act struct {
	ID     string `json:"id"`     // Act ID
	Expiry string `json:"expiry"` // 过期时间
	Type   string `json:"type"`   // 类型
}

type NightWaveChallenge struct {
	WorldStateBase
	Daily       bool   `json:"daily"`       // 是否为每日
	Elite       bool   `json:"elite"`       // 是否为精英
	Standing    int    `json:"standing"`    // 声望
	Description string `json:"description"` // 描述
	Title       string `json:"title"`       // 标题
	IsCompleted bool   `json:"isCompleted"` // 是否已完成
	IsExpired   bool   `json:"isExpired"`   // 是否已过期
}
