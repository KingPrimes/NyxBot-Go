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
	"nyxbot-go/internal/ocr"
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
	database.Init(cfg.Database.Path, cfg.Log.Startup, cfg.Log.SQLLevel, cfg.Log.SQLSlowMS)

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

	// 紫卡 OCR：后台校验/下载模型并初始化引擎（不阻塞主流程，就绪后经 ocr.Ready 获取）
	if cfg.Ocr.Enabled {
		go ocr.Prepare(serverContext, ocr.Config{
			ModelDir:       cfg.Ocr.ModelDir,
			UseRGB:         cfg.Ocr.UseRGB,
			AutoDownload:   cfg.Ocr.AutoDownload,
			DownloadSource: cfg.Ocr.DownloadSource,
		})
	}

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
		if hint := listenFailureHint(httpServer.Addr, err); hint != "" {
			logging.ErrorPack("main", "%s", hint)
		}
	}
}

// Windows 套接字错误码：Go 在 Windows 上不会把它们映射成 syscall.EACCES/EADDRINUSE，
// 只能按原始码判断（实测 errors.Is(err, syscall.EACCES) 对 WSAEACCES 为 false）。
const (
	wsaEACCES     = 10013 // WSAEACCES：绑定被系统拒绝（例如端口落在保留段内）
	wsaEADDRINUSE = 10048 // WSAEADDRINUSE：端口已被其它进程占用
)

// listenFailureHint 针对监听失败的常见原因返回一行排障提示（端口被系统保留 / 端口被占用），
// 同时覆盖 Windows（WSAEACCES/WSAEADDRINUSE）与类 Unix（EACCES/EADDRINUSE）取值；无匹配返回空串。
func listenFailureHint(addr string, err error) string {
	var errno syscall.Errno
	if !errors.As(err, &errno) {
		return ""
	}
	switch {
	case int(errno) == wsaEACCES:
		return fmt.Sprintf("hint: %s bind denied (WSAEACCES/10013); the port is likely inside a Windows reserved range "+
			"(Hyper-V/WSL/Docker winnat). Check `netsh interface ipv4 show excludedportrange protocol=tcp`, "+
			"then change server.port in config.yaml, or run `net stop winnat` before starting the server", addr)
	case int(errno) == wsaEADDRINUSE:
		return fmt.Sprintf("hint: %s address already in use (WSAEADDRINUSE/10048); find the owner with "+
			"`netstat -ano | findstr :%s`, or change server.port in config.yaml", addr, portOfAddr(addr))
	case errors.Is(err, syscall.EACCES):
		return fmt.Sprintf("hint: %s bind denied (EACCES); on Unix this is usually a privileged port (<1024) "+
			"or a port reserved by policy", addr)
	case errors.Is(err, syscall.EADDRINUSE):
		return fmt.Sprintf("hint: %s address already in use (EADDRINUSE); find the owner with `lsof -i :%s`, "+
			"or change server.port in config.yaml", addr, portOfAddr(addr))
	}
	return ""
}

// portOfAddr 从监听地址中取端口部分（"0.0.0.0:18080" -> "18080"），解析不出时原样返回。
func portOfAddr(addr string) string {
	if index := strings.LastIndex(addr, ":"); index >= 0 && index < len(addr)-1 {
		return addr[index+1:]
	}
	return addr
}
