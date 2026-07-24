// draw-image-plugin 市场相关枚举
package drawplugin

// MarketActivityType 玩家活跃状态
type MarketActivityType string

const (
	MarketActUnknown   MarketActivityType = "UNKNOWN"
	MarketActIdle      MarketActivityType = "IDLE"
	MarketActOnMission MarketActivityType = "ON_MISSION"
	MarketActInDojo    MarketActivityType = "IN_DOJO"
	MarketActInOrbiter MarketActivityType = "IN_ORBITER"
	MarketActInRelay   MarketActivityType = "IN_RELAY"
)

var MarketActivityMap = map[MarketActivityType]string{
	MarketActUnknown: "未知", MarketActIdle: "空闲",
	MarketActOnMission: "任务中", MarketActInDojo: "在道场",
	MarketActInOrbiter: "在轨道器", MarketActInRelay: "在中继站",
}

// MarketStatus 用户在线状态
type MarketStatus string

const (
	MarketStatusInvisible MarketStatus = "INVISIBLE"
	MarketStatusOffline   MarketStatus = "OFFLINE"
	MarketStatusOnline    MarketStatus = "ONLINE"
	MarketStatusIngame    MarketStatus = "INGAME"
)

var MarketStatusMap = map[MarketStatus]string{
	MarketStatusInvisible: "不可见", MarketStatusOffline: "离线",
	MarketStatusOnline: "在线", MarketStatusIngame: "游戏中",
}

// TransactionType 交易类型
type TransactionType string

const (
	TransSell TransactionType = "SELL"
	TransBuy  TransactionType = "BUY"
	TransAll  TransactionType = "ALL"
	TransNone TransactionType = "NONE"
)

func (t TransactionType) Name() string {
	mp := map[TransactionType]string{
		TransSell: "sell", TransBuy: "buy",
		TransAll: "all", TransNone: "none",
	}
	return mp[t]
}
