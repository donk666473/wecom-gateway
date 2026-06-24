// Package db 提供 PostgreSQL 数据库初始化，使用 GORM 作为 ORM。
// 参照钉钉对接项目中的 db/database 模块和 DATRIX 设计文档中的数据库设计。
package db

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	sqlite "github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// safeDBNameRegex 数据库名称安全校验正则
// 仅允许字母、数字、下划线，长度 1-63
var safeDBNameRegex = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]{0,62}$`)

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
	// 支持不同数据库驱动（postgres / sqlite）以便在本地快速运行测试
	var db *gorm.DB
	var err error

	switch cfg.DriverName {
	case "sqlite", "sqlite3":
		// 对于 sqlite，cfg.Database 应为 DSN（例如 file::memory:?cache=shared 或 ./wecom.db）
		db, err = gorm.Open(sqlite.Open(cfg.Database), &gorm.Config{Logger: logger.Default.LogMode(logLevel)})
		if err != nil {
			return nil, fmt.Errorf("打开 sqlite 数据库失败: %w", err)
		}
	default:
		dsn := fmt.Sprintf(
			"host=%s port=%d user=%s password=%s dbname=%s sslmode=disable TimeZone=Asia/Shanghai",
			cfg.Host, cfg.Port, cfg.Username, cfg.Password, cfg.Database,
		)

		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logLevel)})
		if err != nil {
			// 尝试创建数据库后重连
			if err := createDatabaseIfNotExist(cfg); err != nil {
				return nil, fmt.Errorf("创建数据库失败: %w", err)
			}
			db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logLevel)})
			if err != nil {
				return nil, fmt.Errorf("连接数据库失败: %w", err)
			}
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

// createDatabaseIfNotExist 尝试创建数据库（使用参数化查询和标识符校验防止 SQL 注入）。
func createDatabaseIfNotExist(cfg *DatabaseConfig) error {
	// 校验数据库名，防止 SQL 注入
	if !safeDBNameRegex.MatchString(cfg.Database) {
		return fmt.Errorf("数据库名包含非法字符: %s", cfg.Database)
	}

	// PostgreSQL 标识符安全引用（双引号 + 内部双引号转义）
	safeDBName := pqQuoteIdentifier(cfg.Database)

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

	// 参数化查询：检查数据库是否存在
	var exists bool
	if err := db.Raw("SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = ?)", cfg.Database).Scan(&exists).Error; err != nil {
		return err
	}
	if !exists {
		createSQL := fmt.Sprintf("CREATE DATABASE %s ENCODING 'UTF8'", safeDBName)
		return db.Exec(createSQL).Error
	}
	return nil
}

// pqQuoteIdentifier 安全引用 PostgreSQL 标识符（双引号 + 内部双引号转义）。
func pqQuoteIdentifier(name string) string {
	escaped := strings.ReplaceAll(name, "\"", "\"\"")
	return "\"" + escaped + "\""
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
