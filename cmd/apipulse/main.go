package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/xhy/api-pulse/config"
	"github.com/xhy/api-pulse/internal/apifox"
	"github.com/xhy/api-pulse/internal/dingtalk"
	"github.com/xhy/api-pulse/internal/server"
	"github.com/xhy/api-pulse/internal/service"
	"github.com/xhy/api-pulse/internal/storage"
	"github.com/xhy/api-pulse/pkg/utils"
)

func main() {
	// 命令行参数
	configPath := flag.String("config", "config/config.yaml", "配置文件路径")
	flag.Parse()

	// 初始化日志
	logger := utils.SetupLogger()
	logger.Info("API Pulse 服务启动中...")

	// 加载配置
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		logger.WithError(err).Fatal("加载配置失败")
	}

	// 初始化API存储 - 纯内存实现
	apiStore := storage.NewApiStore(logger)

	// 初始化Apifox客户端
	apifoxClient := apifox.NewClient(&cfg.Apifox, logger)

	// 初始化差异比较服务
	diffService := apifox.NewDiffService(logger)

	// 初始化钉钉通知服务 - 不再使用 secret
	notifyService := dingtalk.NewNotifyService(cfg.Dingtalk.WebhookURL, logger)

	// 初始化API服务
	apiService := service.NewApiService(logger, apifoxClient, apiStore, diffService)

	// 初始化API列表
	logger.Info("正在初始化 API 列表...")
	successCount, failureCount, failedApis, err := apiService.InitializeApiList()
	if err != nil {
		logger.WithError(err).Error("初始化 API 列表失败")
	} else {
		if len(failedApis) > 0 {
			logger.WithFields(map[string]interface{}{
				"failed_apis": failedApis,
			}).Warn("部分 API 初始化失败")
		}

		logger.WithFields(map[string]interface{}{
			"success_count": successCount,
			"failure_count": failureCount,
		}).Info("API 列表初始化完成")
	}

	// 设置同步间隔为30分钟
	apiService.SetSyncInterval(30 * time.Minute)

	// 启动定时同步任务
	apiService.StartSync()
	logger.Info("API定时同步任务已启动")

	// 初始化API处理器
	apiHandler := server.NewApiNotifyHandler(apifoxClient, diffService, notifyService, apiStore, logger, apiService)

	// 初始化HTTP服务器
	srv := server.NewServer(cfg.Server.Port, apiHandler, logger)

	// 处理优雅关闭
	done := make(chan bool, 1)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		logger.Info("服务器正在关闭...")

		// 停止API同步任务
		apiService.StopSync()

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			logger.WithError(err).Fatal("强制关闭服务器")
		}

		done <- true
	}()

	// 启动服务器
	if err := srv.Start(); err != nil {
		logger.WithError(err).Fatal("启动服务器失败")
	}

	<-done
	logger.Info("服务器已关闭")
}
