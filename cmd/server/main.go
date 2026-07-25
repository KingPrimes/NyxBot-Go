// Package main 服务入口，解析命令行参数后依次初始化日志、配置、数据库并启动 HTTP 服务。
package main

import (
	"fmt"
	"os"
	"strings"

	"nyxbot-go/internal/config"
	"nyxbot-go/internal/database"
	"nyxbot-go/internal/logging"
	"nyxbot-go/internal/server"
	"nyxbot-go/internal/version"
)

// main 程序入口，支持 --version/--help 命令行参数。
func main() {
	if len(os.Args) > 1 {
		switch strings.TrimSpace(os.Args[1]) {
		case "--version", "-v", "version":
			fmt.Printf("%s %s (%s)\n", version.ProductName, version.Version, version.Commit)
			return
		case "--help", "-h", "help":
			fmt.Printf("%s - %s\n\n", version.ProductName, version.Description)
			fmt.Println("Usage:")
			fmt.Printf("  %s [flags]\n\n", version.ProductName)
			fmt.Println("Flags:")
			fmt.Println("  --version, -v    Print version information")
			fmt.Println("  --help, -h       Print help")
			return
		}
	}

	logging.Init(logging.Config{})
	rt := config.NewRuntime(config.Load(), "config.yaml")
	cfg := rt.Config()
	logging.Init(logging.Config{
		Dir:            cfg.Log.Dir,
		MaxFileSize:    int64(cfg.Log.MaxFileSizeMB) * 1024 * 1024,
		MaxAgeDays:     cfg.Log.MaxAgeDays,
		HistorySize:    cfg.Log.HistorySize,
		ConsoleEnabled: cfg.Log.Console,
	})
	database.Init(cfg.Database.Path, cfg.Log.Startup)

	r := server.NewRouter(rt)
	if err := r.Run(cfg.Addr()); err != nil {
		logging.ErrorPack("main", "server stopped: %v", err)
	}
}
