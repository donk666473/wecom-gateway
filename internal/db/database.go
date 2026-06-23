// Package db 提供 PostgreSQL 数据库初始化，使用 GORM 作为 ORM。
// 参照钉钉对接项目中的 db/database 模块和 DATRIX 设计文档中的数据库设计。
package db

import (
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DatabaseConfig 数据库连接配置
type DatabaseConfig struct {
	Host            string
	Port            int
	DriverName      string
	Database        string
	Username        string
	Password        string
	MaxIdleConns    int
	MaxOpenConns    int
	ConnMaxLifetime int
}

// InitDatabase 初始化数据库连接并返回 GORM 实例。
// 支持自动创建数据库和连接池配置。
func InitDatabase(cfg *DatabaseConfig, logLevel logger.LogLevel) (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=disable TimeZone=Asia/Shanghai",
		cfg.Host, cfg.Port, cfg.Username, cfg.Password, cfg.Database,
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	})
	if err != nil {
		// 尝试创建数据库后重连
		if err := createDatabaseIfNotExist(cfg); err != nil {
			return nil, fmt.Errorf("创建数据库失败: %w", err)
		}
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
			Logger: logger.Default.LogMode(logLevel),
		})
		if err != nil {
			return nil, fmt.Errorf("连接数据库失败: %w", err)
		}
	}

	// 配置连接池
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("获取 sql.DB 失败: %w", err)
	}

	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetime) * time.Second)

	return db, nil
}

// createDatabaseIfNotExist 尝试创建数据库
func createDatabaseIfNotExist(cfg *DatabaseConfig) error {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=postgres sslmode=disable TimeZone=Asia/Shanghai",
		cfg.Host, cfg.Port, cfg.Username, cfg.Password,
	)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return err
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()

	var exists bool
	checkSQL := fmt.Sprintf("SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname='%s')", cfg.Database)
	if err := db.Raw(checkSQL).Scan(&exists).Error; err != nil {
		return err
	}
	if !exists {
		createSQL := fmt.Sprintf("CREATE DATABASE %s", cfg.Database)
		return db.Exec(createSQL).Error
	}
	return nil
}

// LogLevelMap 将字符串日志级别映射为 GORM 日志级别
func LogLevelMap(level string) logger.LogLevel {
	switch level {
	case "debug":
		return logger.Info
	case "warn":
		return logger.Warn
	case "error":
		return logger.Error
	default:
		return logger.Silent
	}
}
