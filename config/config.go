// Package config 提供配置文件加载、命令行参数绑定和环境变量管理。
// 参照 LangBot 框架的配置驱动设计思想，支持 viper + pflag + 环境变量三重绑定。
package config

import (
	"bytes"
	"fmt"
	"os"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// Config 全局配置结构体，映射 config.yaml 中的所有配置段。
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Log      LogConfig      `mapstructure:"log"`
	Database DatabaseConfig `mapstructure:"database"`
	Redis    RedisConfig    `mapstructure:"redis"`
	Datrix   DatrixConfig   `mapstructure:"datrix"`
	WeCom    WeComConfig    `mapstructure:"wecom"`
	Message  MessageConfig  `mapstructure:"message"`
}

// ServerConfig HTTP 服务器配置
type ServerConfig struct {
	Port       int    `mapstructure:"port"`
	Mode       string `mapstructure:"mode"`        // debug / release / test
	AdminToken string `mapstructure:"admin_token"` // 管理 API 认证 Token
}

// LogConfig 日志配置
type LogConfig struct {
	File      string `mapstructure:"file"`
	Level     string `mapstructure:"level"`
	MaxSize   int    `mapstructure:"max_size"`   // MB
	Backups   int    `mapstructure:"backups"`     // 保留备份数
	MaxAge    int    `mapstructure:"max_age"`     // 保留天数
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Host            string `mapstructure:"host"`
	Port            int    `mapstructure:"port"`
	DriverName      string `mapstructure:"driver_name"`
	Database        string `mapstructure:"database"`
	Username        string `mapstructure:"username"`
	Password        string `mapstructure:"password"`
	MaxIdleConns    int    `mapstructure:"max_idle_conns"`
	MaxOpenConns    int    `mapstructure:"max_open_conns"`
	ConnMaxLifetime int    `mapstructure:"conn_max_lifetime"`
}

// RedisConfig Redis 配置
type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
	Prefix   string `mapstructure:"prefix"`
}

// DatrixConfig DATRIX 后端服务配置
type DatrixConfig struct {
	AssetURL     string           `mapstructure:"asset_url"`
	AssistantURL string           `mapstructure:"assistant_url"`
	FreeLogin    FreeLoginConfig  `mapstructure:"free_login"`
	Session      SessionConfig    `mapstructure:"session"`
}

// FreeLoginConfig 免密登录配置
type FreeLoginConfig struct {
	AESKey     string `mapstructure:"aes_key"`
	SigningKey string `mapstructure:"signing_key"`
}

// SessionConfig 会话相关配置
type SessionConfig struct {
	Timeout    string `mapstructure:"timeout"`     // 会话超时时间，如 "2h"
	TokenTTL   string `mapstructure:"token_ttl"`   // Token 缓存时间
	DedupTTL   string `mapstructure:"dedup_ttl"`   // 消息去重缓存时间
}

// WeComConfig 企微平台全局配置
type WeComConfig struct {
	APIBaseURL string `mapstructure:"api_base_url"` // 企微 API 基地址
}

// MessageConfig 消息处理配置
type MessageConfig struct {
	BatchSize    int `mapstructure:"batch_size"`     // 流式更新批次大小
	ChunkSize    int `mapstructure:"chunk_size"`     // 企微分段消息最大字节数
	PollInterval int `mapstructure:"poll_interval"`  // 前端轮询间隔（毫秒）
}

// LoadConfig 加载配置文件，支持命令行参数和环境变量覆盖。
// 配置加载优先级：命令行参数 > 环境变量 > 配置文件 > 默认值
func LoadConfig() (*Config, error) {
	v := viper.New()

	// 绑定命令行参数
	pflag.String("config", "config/config.yaml", "配置文件路径")
	pflag.Int("server.port", 8080, "HTTP 服务端口")
	pflag.String("server.mode", "release", "运行模式: debug/release/test")
	pflag.String("log.file", "/var/log/wecom-gateway.log", "日志文件路径")
	pflag.String("log.level", "info", "日志级别")
	pflag.String("db.host", "127.0.0.1", "数据库地址")
	pflag.Int("db.port", 3306, "数据库端口")
	pflag.String("db.database", "asset", "数据库名")
	pflag.String("db.username", "root", "数据库用户名")
	pflag.String("db.password", "", "数据库密码")
	pflag.String("redis.host", "127.0.0.1", "Redis 地址")
	pflag.Int("redis.port", 6379, "Redis 端口")
	pflag.String("redis.password", "", "Redis 密码")
	pflag.String("datrix.asset_url", "http://127.0.0.1:30805", "DATRIX Asset 服务地址")
	pflag.String("datrix.assistant_url", "http://127.0.0.1:30806", "DATRIX Assistant 服务地址")
	pflag.String("wecom.api_base_url", "https://qyapi.weixin.qq.com/cgi-bin", "企微 API 基地址")
	pflag.Int("message.chunk_size", 2048, "企微分段消息最大字节数")
	pflag.Parse()

	// 将 pflag 绑定到 viper
	_ = v.BindPFlags(pflag.CommandLine)

	// 绑定环境变量（支持 WECOM_ 前缀）
	v.SetEnvPrefix("WECOM")
	v.AutomaticEnv()

	// 设置默认值
	setDefaults(v)

	// 读取配置文件
	configPath := v.GetString("config")
	if configPath != "" {
		v.SetConfigFile(configPath)
		v.SetConfigType("yaml")
		if err := v.ReadInConfig(); err != nil {
			if !os.IsNotExist(err) {
				return nil, fmt.Errorf("读取配置文件失败: %w", err)
			}
			// 配置文件不存在时使用默认配置
			fmt.Printf("[WARN] 配置文件 %s 不存在，使用默认配置\n", configPath)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}

	return &cfg, nil
}

// setDefaults 设置所有配置项的默认值
func setDefaults(v *viper.Viper) {
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.mode", "release")
	v.SetDefault("server.admin_token", "")
	v.SetDefault("log.file", "/var/log/wecom-gateway.log")
	v.SetDefault("log.level", "info")
	v.SetDefault("log.max_size", 100)
	v.SetDefault("log.backups", 3)
	v.SetDefault("log.max_age", 30)
	v.SetDefault("database.host", "127.0.0.1")
	v.SetDefault("database.port", 3306)
	v.SetDefault("database.driver_name", "postgres")
	v.SetDefault("database.database", "asset")
	v.SetDefault("database.username", "root")
	v.SetDefault("database.max_idle_conns", 10)
	v.SetDefault("database.max_open_conns", 100)
	v.SetDefault("database.conn_max_lifetime", 3600)
	v.SetDefault("redis.host", "127.0.0.1")
	v.SetDefault("redis.port", 6379)
	v.SetDefault("redis.db", 0)
	v.SetDefault("redis.prefix", "wecom")
	v.SetDefault("datrix.asset_url", "http://127.0.0.1:30805")
	v.SetDefault("datrix.assistant_url", "http://127.0.0.1:30806")
	v.SetDefault("datrix.free_login.aes_key", "")
	v.SetDefault("datrix.free_login.signing_key", "")
	v.SetDefault("datrix.session.timeout", "2h")
	v.SetDefault("datrix.session.token_ttl", "24h")
	v.SetDefault("datrix.session.dedup_ttl", "5m")
	v.SetDefault("wecom.api_base_url", "https://qyapi.weixin.qq.com/cgi-bin")
	v.SetDefault("message.batch_size", 50)
	v.SetDefault("message.chunk_size", 2048)
	v.SetDefault("message.poll_interval", 1500)
}

// LoadConfigFromBytes 从 YAML 字节数组加载配置（用于测试）
func LoadConfigFromBytes(data []byte) (*Config, error) {
	v := viper.New()
	setDefaults(v)
	v.SetConfigType("yaml")
	if err := v.ReadConfig(bytes.NewReader(data)); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}
	return &cfg, nil
}
