// 数据库初始化层，对应 Java NyxBot 的 JPA/H2 配置
// 使用 GORM + SQLite 替代 JPA/H2
// 启动时自动创建表结构（AutoMigrate），注册全量 warframe 数据库模型
package database

import (
	"os"
	"path/filepath"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"nyxbot-go/internal/logging"
	"nyxbot-go/internal/model/bot"
	"nyxbot-go/internal/model/plugin"
	"nyxbot-go/internal/model/system"
	"nyxbot-go/internal/model/warframe"
)

// DB 全局数据库连接实例，包内各处直接引用。
var DB *gorm.DB

// Init 初始化 SQLite 数据库连接，执行自动迁移和默认管理员创建。
func Init(dbPath string, startupLog bool) {
	if dbPath == "" {
		dbPath = "data/nyxbot.db"
	}

	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		logging.ErrorPack("database", "failed to create data directory: %v", err)
		panic(err)
	}

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		logging.ErrorPack("database", "failed to connect database: %v", err)
		panic(err)
	}

	DB = db

	if err := autoMigrate(); err != nil {
		logging.ErrorPack("database", "failed to auto migrate: %v", err)
		panic(err)
	}
	if err := ensureDefaultAdmin(); err != nil {
		logging.ErrorPack("database", "failed to initialize default admin user: %v", err)
		panic(err)
	}

	if startupLog {
		logging.InfoPack("database", "database initialized")
	}
}

// autoMigrate 自动创建/更新所有注册的数据表结构。
func autoMigrate() error {
	return DB.AutoMigrate(
		&warframe.Alias{},
		&warframe.Ephemera{},
		&warframe.LichSisterWeapon{},
		&warframe.OrdersItem{},
		&warframe.RivenTion{},
		&warframe.RivenTionAlias{},
		&warframe.RivenItem{},
		&warframe.RivenAnalyseTrend{},
		&warframe.StateTranslation{},
		&warframe.NotificationHistory{},
		&warframe.MissionSubscribe{},
		&warframe.MissionSubscribeUser{},
		&warframe.MissionSubscribeUserCheckType{},
		&warframe.Weapons{},
		&warframe.Warframes{},
		&warframe.WarframesAbility{},
		&warframe.Upgrades{},
		&warframe.Sentinels{},
		&warframe.NightWave{},
		&warframe.Nodes{},
		&warframe.Relics{},
		&warframe.RelicRewards{},
		&warframe.ModSet{},
		&warframe.Customs{},
		&warframe.RewardPool{},
		&warframe.Reward{},
		// system
		&system.SysUser{},
		&system.PersistentLogin{},
		&system.LogInfo{},
		&system.Hint{},
		// bot
		&bot.BotAdmin{},
		&bot.GroupWhite{},
		&bot.ProveWhite{},
		&bot.GroupBlack{},
		&bot.ProveBlack{},
		// plugin
		&plugin.PluginInfo{},
	)
}
