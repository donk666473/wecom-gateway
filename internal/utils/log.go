// Package utils 提供日志初始化功能。
// 使用 Zap + Lumberjack 实现高性能结构化日志和自动轮转。
// 参照钉钉对接项目中的 utils/log 模块。
//
// 特性：
// 1. 同时输出到控制台和文件
// 2. 文件自动轮转
// 3. error 及以上级别日志单独写入 error 日志文件
// 4. 支持注册错误日志钩子（用于调试服务收集错误）
package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	File       string // 主日志文件路径
	ErrorFile  string // 错误日志文件路径（为空时按主日志派生）
	Level      string // 日志级别: debug/info/warn/error
	MaxSize    int    // 单文件最大大小(MB)
	MaxAge     int    // 最大保留天数
	MaxBackups int    // 最大备份数
}

// ErrorHookFunc 错误日志钩子函数签名。
// 当 logger 写入 error/fatal/panic 级别日志时触发。
type ErrorHookFunc func(level zapcore.Level, entry zapcore.Entry, fields []zapcore.Field)

var errorHook ErrorHookFunc

// RegisterErrorHook 注册错误日志钩子。
// 通常由调试服务在初始化时调用。
func RegisterErrorHook(fn ErrorHookFunc) {
	errorHook = fn
}

// InitLogger 初始化全局日志实例。
// 同时输出到控制台和文件，文件自动轮转；error 级别单独写入 error 文件。
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

	mainLogFile := cfg.File
	if mainLogFile == "" {
		mainLogFile = "/var/log/wecom-gateway.log"
	}

	// 错误日志文件：默认与主日志同目录，后缀 .error.log
	errorLogFile := cfg.ErrorFile
	if errorLogFile == "" {
		dir := filepath.Dir(mainLogFile)
		base := filepath.Base(mainLogFile)
		ext := filepath.Ext(base)
		name := strings.TrimSuffix(base, ext)
		if ext == "" {
			errorLogFile = filepath.Join(dir, fmt.Sprintf("%s.error", name))
		} else {
			errorLogFile = filepath.Join(dir, fmt.Sprintf("%s.error%s", name, ext))
		}
	}

	// 主日志文件输出（带轮转）
	mainFileWriter := zapcore.AddSync(&lumberjack.Logger{
		Filename:   mainLogFile,
		MaxSize:    cfg.MaxSize,
		MaxBackups: cfg.MaxBackups,
		MaxAge:     cfg.MaxAge,
		Compress:   true,
	})

	// 错误日志文件输出（带轮转）
	errorFileWriter := zapcore.AddSync(&lumberjack.Logger{
		Filename:   errorLogFile,
		MaxSize:    cfg.MaxSize,
		MaxBackups: cfg.MaxBackups,
		MaxAge:     cfg.MaxAge,
		Compress:   true,
	})

	// 控制台输出
	consoleWriter := zapcore.AddSync(os.Stdout)

	encoder := zapcore.NewJSONEncoder(encoderConfig)

	// 主日志 core（all levels）
	mainCore := zapcore.NewCore(encoder, mainFileWriter, level)
	// 错误日志 core（error+）
	errorCore := zapcore.NewCore(encoder, errorFileWriter, zapcore.ErrorLevel)
	// 控制台 core
	consoleCore := zapcore.NewCore(zapcore.NewConsoleEncoder(encoderConfig), consoleWriter, level)

	// 包装主日志 core，增加错误日志钩子调用
	hookedMainCore := &errorHookCore{Core: mainCore}

	// 多输出聚合
	core := zapcore.NewTee(hookedMainCore, errorCore, consoleCore)

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

// errorHookCore 包装 zapcore.Core，在 error 及以上级别日志写入时触发钩子。
type errorHookCore struct {
	zapcore.Core
}

func (c *errorHookCore) Check(ent zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(ent.Level) {
		return ce.AddCore(ent, c)
	}
	return ce
}

func (c *errorHookCore) Write(ent zapcore.Entry, fields []zapcore.Field) error {
	if ent.Level >= zapcore.ErrorLevel && errorHook != nil {
		func() {
			defer func() {
				if r := recover(); r != nil {
					// 钩子异常不能影响日志写入
				}
			}()
			errorHook(ent.Level, ent, fields)
		}()
	}
	return c.Core.Write(ent, fields)
}
