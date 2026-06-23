// Package main 是企微对接网关的启动入口。
// 参照 LangBot 框架的启动流程和 DATRIX 设计文档的架构设计。
//
// 启动流程（参照 LangBot boot.py 的启动阶段序）：
// 1. LoadConfig  — 加载配置文件
// 2. SetupLogger — 初始化日志系统
// 3. InitDatabase — 初始化数据库连接和表结构
// 4. InitRedis    — 初始化 Redis 连接
// 5. BuildBridge  — 创建 DATRIX Bridge
// 6. BuildPipeline— 创建消息处理 Pipeline
// 7. StartBots    — 启动所有 IM 应用
// 8. StartServer  — 启动 HTTP 服务
// 9. WaitShutdown — 优雅退出
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/wecom-gateway/config"
	"github.com/wecom-gateway/internal/adapter"
	"github.com/wecom-gateway/internal/auth"
	"github.com/wecom-gateway/internal/botmgr"
	"github.com/wecom-gateway/internal/bridge"
	"github.com/wecom-gateway/internal/common"
	"github.com/wecom-gateway/internal/db"
	"github.com/wecom-gateway/internal/handler"
	"github.com/wecom-gateway/internal/model"
	"github.com/wecom-gateway/internal/processor"
	"github.com/wecom-gateway/internal/utils"
)

func main() {
	log.Println("[WECOM-GATEWAY] 正在启动企微对接网关...")

	// ========================================================================
	// Stage 1: 加载配置（参照 LangBot LoadConfigStage）
	// ========================================================================
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}
	populateCommonConfig(cfg)

	// ========================================================================
	// Stage 2: 初始化日志（参照 LangBot SetupLoggerStage）
	// ========================================================================
	if err := utils.InitLogger(&utils.LogConfig{
		File:       cfg.Log.File,
		Level:      cfg.Log.Level,
		MaxSize:    cfg.Log.MaxSize,
		MaxAge:     cfg.Log.MaxAge,
		MaxBackups: cfg.Log.Backups,
	}); err != nil {
		log.Fatalf("初始化日志失败: %v", err)
	}
	defer utils.Sync()

	utils.Sugar.Info("=== 企微对接网关启动 ===")
	utils.Sugar.Infof("配置: server=%d, mode=%s", cfg.Server.Port, cfg.Server.Mode)

	// ========================================================================
	// Stage 3: 初始化数据库（参照 LangBot MigrationStage）
	// ========================================================================
	dbInstance, err := db.InitDatabase(&db.DatabaseConfig{
		Host:            cfg.Database.Host,
		Port:            cfg.Database.Port,
		DriverName:      cfg.Database.DriverName,
		Database:        cfg.Database.Database,
		Username:        cfg.Database.Username,
		Password:        cfg.Database.Password,
		MaxIdleConns:    cfg.Database.MaxIdleConns,
		MaxOpenConns:    cfg.Database.MaxOpenConns,
		ConnMaxLifetime: cfg.Database.ConnMaxLifetime,
	}, db.LogLevelMap(cfg.Log.Level))
	if err != nil {
		utils.Sugar.Fatalf("初始化数据库失败: %v", err)
	}
	model.SetDB(dbInstance)
	utils.Sugar.Info("数据库连接成功")

	// 自动迁移表结构
	if err := model.InitAllTables(); err != nil {
		utils.Sugar.Warnf("自动迁移表结构失败: %v", err)
	}
	utils.Sugar.Info("数据表初始化完成")

	// ========================================================================
	// Stage 4: 初始化 Redis
	// ========================================================================
	if err := db.InitRedis(&db.RedisConfig{
		Host:     cfg.Redis.Host,
		Port:     cfg.Redis.Port,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
		Prefix:   cfg.Redis.Prefix,
	}); err != nil {
		utils.Sugar.Fatalf("初始化 Redis 失败: %v", err)
	}
	utils.Sugar.Info("Redis 连接成功")

	// ========================================================================
	// Stage 5: 创建 DATRIX Bridge
	// ========================================================================
	datrixBridge := bridge.NewDatrixBridge()
	utils.Sugar.Info("DATRIX Bridge 创建完成")

	// ========================================================================
	// Stage 6: 创建 BotManager + MessageProcessor + Pipeline
	// ========================================================================
	botMgr := botmgr.GetBotManager()
	msgProcessor := processor.NewMessageProcessor(botMgr, datrixBridge)

	// 设置 BotManager 的消息回调（委托给 MessageProcessor 的 Pipeline）
	botMgr.SetPipelineFunc(func(event *adapter.IMEvent) {
		msgProcessor.ProcessEvent(event)
	})

	utils.Sugar.Info("消息处理器 Pipeline 组装完成")

	// ========================================================================
	// Stage 7: 启动所有 IM 应用
	// ========================================================================
	if err := botMgr.StartAllBots(); err != nil {
		utils.Sugar.Fatalf("启动 Bot 失败: %v", err)
	}

	// ========================================================================
	// Stage 8: 创建 AuthService（扫码登录）
	// ========================================================================
	authService := auth.NewAuthService(botMgr, datrixBridge)

	// ========================================================================
	// Stage 9: 启动 HTTP 服务
	// ========================================================================
	ginMode := cfg.Server.Mode
	if ginMode == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	router := handler.NewRouter(botMgr, authService)

	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// 启动 HTTP 服务（非阻塞）
	go func() {
		utils.Sugar.Infof("HTTP 服务启动 [addr=%s]", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			utils.Sugar.Fatalf("HTTP 服务启动失败: %v", err)
		}
	}()

	// ========================================================================
	// Stage 10: 等待退出信号并优雅关闭
	// ========================================================================
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit

	utils.Sugar.Infof("收到退出信号: %v，开始优雅关闭...", sig)

	// 停止 HTTP 服务
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		utils.Sugar.Errorf("HTTP 服务关闭失败: %v", err)
	}
	utils.Sugar.Info("HTTP 服务已停止")

	// 停止所有 Bot
	botMgr.StopAllBots()
	utils.Sugar.Info("所有 Bot 已停止")

	utils.Sugar.Info("=== 企微对接网关已安全退出 ===")
}

// populateCommonConfig 将配置注入到 common 包的全局变量。
// 此函数作为配置中心，将 config.Config 分发到各模块。
func populateCommonConfig(cfg *config.Config) {
	// 日志
	common.LogFile = cfg.Log.File
	common.LogLevel = cfg.Log.Level
	common.LogMaxSize = cfg.Log.MaxSize
	common.LogBackups = cfg.Log.Backups
	common.LogMaxAge = cfg.Log.MaxAge

	// Redis
	common.RedisKeyPrefix = cfg.Redis.Prefix
	common.RedisHost = cfg.Redis.Host
	common.RedisPassword = cfg.Redis.Password
	common.RedisDB = cfg.Redis.DB

	// DATRIX
	common.DatrixAssetURL = cfg.Datrix.AssetURL
	common.DatrixAssistantURL = cfg.Datrix.AssistantURL
	common.FreeLoginAESKey = cfg.Datrix.FreeLogin.AESKey
	common.FreeLoginSigningKey = cfg.Datrix.FreeLogin.SigningKey

	// 会话配置
	common.SessionTimeout, _ = time.ParseDuration(cfg.Datrix.Session.Timeout)
	common.TokenTTL, _ = time.ParseDuration(cfg.Datrix.Session.TokenTTL)
	common.DedupTTL, _ = time.ParseDuration(cfg.Datrix.Session.DedupTTL)

	// 消息配置
	common.MessageBatchSize = cfg.Message.BatchSize
	common.WecomChunkSize = cfg.Message.ChunkSize

	// 企微配置
	common.WeComAPIBaseURL = cfg.WeCom.APIBaseURL

	// 扫码登录
	common.AuthCallbackURL = cfg.Auth.CallbackURL
	common.AuthStateTTL = time.Duration(cfg.Auth.StateTTL) * time.Second
	common.AuthTokenTTL = time.Duration(cfg.Auth.TokenTTL) * time.Second
}
