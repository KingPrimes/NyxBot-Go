// Package main 服务入口，解析命令行参数后依次初始化日志、配置、数据库并启动 HTTP 服务。
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	botdirectory "nyxbot-go/internal/bot"
	"nyxbot-go/internal/config"
	"nyxbot-go/internal/database"
	"nyxbot-go/internal/logging"
	"nyxbot-go/internal/onebot"
	"nyxbot-go/internal/server"
	"nyxbot-go/internal/version"
	"nyxbot-go/internal/warframe"
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
		Level:          cfg.Log.Level,
	})
	database.Init(cfg.Database.Path, cfg.Log.Startup)

	serverContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Warframe 数据层（阶段 8）：导出文件下载器 + 市场 API + 导入器 + 更新器 + HTTP 接口
	// 重试参数从 config.yaml 的 warframe 块读取（网络错误自动重试，指数退避）
	warframe.SetRetryConfig(
		cfg.Warframe.HTTPRetryAttempts,
		cfg.Warframe.HTTPRetryBaseWaitSeconds,
		cfg.Warframe.HTTPRetryTimeoutSeconds,
	)
	exportClient := &http.Client{Timeout: 30 * time.Second}
	exporter := warframe.NewExportFilePath(exportClient, "zh")
	marketAPI := warframe.NewMarketAPI(exportClient)
	importer := warframe.NewDataImporter(exporter, marketAPI, database.DB)
	updater := warframe.NewDataUpdater(importer)
	dataHandler := warframe.NewDataHandler(importer, updater)

	// 启动异步导入（失败不阻塞主流程）与 WorldState 动态轮询
	warframe.SetUserAgentVersion(version.Version)
	go importer.ImportAll(serverContext)
	go warframe.RunWorldStatePolling(serverContext, exportClient)

	r := server.NewRouter(rt, dataHandler, updater)
	botRuntime, err := onebot.NewRuntimeFromConfig(rt, botdirectory.DefaultDirectory)
	if err != nil {
		logging.ErrorPack("main", "OneBot disabled: %v", err)
	} else {
		if err := botRuntime.RegisterRoutes(r); err != nil {
			logging.ErrorPack("main", "OneBot route registration failed: %v", err)
		} else {
			botRuntime.SetDataExecutor(importer)
			if err := botRuntime.Start(serverContext); err != nil {
				logging.ErrorPack("main", "OneBot startup failed: %v", err)
			}
		}
		defer func() {
			if err := botRuntime.Close(); err != nil {
				logging.WarnPack("main", "close OneBot runtime failed: %v", err)
			}
		}()
	}

	httpServer := &http.Server{
		Addr:              cfg.Addr(),
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		<-serverContext.Done()
		shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownContext); err != nil {
			logging.WarnPack("main", "graceful HTTP shutdown failed: %v", err)
		}
	}()
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logging.ErrorPack("main", "server stopped: %v", err)
	}
}
