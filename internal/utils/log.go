// Package utils 提供日志初始化功能。
// 使用 Zap + Lumberjack 实现高性能结构化日志和自动轮转。
// 参照钉钉对接项目中的 utils/log 模块。
package utils

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	// Logger 全局日志实例
	Logger *zap.Logger
	// Sugar 全局语法糖日志实例（方便使用）
	Sugar *zap.SugaredLogger
)

// LogConfig 日志配置
type LogConfig struct {
	File      string // 日志文件路径
	Level     string // 日志级别: debug/info/warn/error
	MaxSize   int    // 单文件最大大小(MB)
	MaxAge    int    // 最大保留天数
	MaxBackups int   // 最大备份数
}

// InitLogger 初始化全局日志实例。
// 同时输出到控制台和文件，文件自动轮转。
func InitLogger(cfg *LogConfig) error {
	// 解析日志级别
	level := zapcore.InfoLevel
	switch cfg.Level {
	case "debug":
		level = zapcore.DebugLevel
	case "warn":
		level = zapcore.WarnLevel
	case "error":
		level = zapcore.ErrorLevel
	}

	// 编码器配置（JSON 格式 + ISO8601 时间）
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.TimeKey = "time"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder

	// 文件输出（带轮转）
	fileWriter := zapcore.AddSync(&lumberjack.Logger{
		Filename:   cfg.File,
		MaxSize:    cfg.MaxSize,
		MaxBackups: cfg.MaxBackups,
		MaxAge:     cfg.MaxAge,
		Compress:   true,
	})

	// 控制台输出
	consoleWriter := zapcore.AddSync(os.Stdout)

	// 多输出
	core := zapcore.NewTee(
		zapcore.NewCore(zapcore.NewJSONEncoder(encoderConfig), fileWriter, level),
		zapcore.NewCore(zapcore.NewConsoleEncoder(encoderConfig), consoleWriter, level),
	)

	Logger = zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
	Sugar = Logger.Sugar()

	return nil
}

// Sync 刷新日志缓冲区（defer 调用）
func Sync() {
	if Logger != nil {
		_ = Logger.Sync()
	}
}
