// Package common 定义全局常量和变量，作为整个应用的配置中心。
// 参照 LangBot 框架的全局配置模式和 DATRIX 设计文档中的常量定义。
package common

import "time"

// HTTP 请求超时时间（分钟）
var HTTPRequestTimeoutMin = 10

// ============================================================================
// 日志配置 — 由 config 包在启动时注入
// ============================================================================
var (
	LogFile    = "/var/log/wecom-gateway.log"
	LogLevel   = "info"
	LogMaxSize = 100
	LogBackups = 3
	LogMaxAge  = 30
)

// ============================================================================
// Redis 配置 — 由 config 包在启动时注入
// ============================================================================
var (
	RedisKeyPrefix string
	RedisHost      string
	RedisPassword  string
	RedisDB        int
)

// ============================================================================
// DATRIX 后端服务配置 — 由 config 包在启动时注入
// ============================================================================
var (
	DatrixAssetURL     string
	DatrixAssistantURL string

	// 免密登录密钥
	FreeLoginAESKey     string
	FreeLoginSigningKey string

	// 会话配置
	SessionTimeout time.Duration
	TokenTTL       time.Duration
	DedupTTL       time.Duration

	// 消息配置
	MessageBatchSize int
	WecomChunkSize   int

	// 扫码登录配置
	AuthCallbackURL string
	AuthStateTTL    time.Duration
	AuthTokenTTL    time.Duration
)

// ============================================================================
// 数据库配置 — 由 config 包在启动时注入
// ============================================================================
var (
	DBHost            string
	DBPort            int
	DBDriverName      string
	DBDataBase        string
	DBUsername        string
	DBPassword        string
	DBMaxIdleConns    int
	DBMaxOpenConns    int
	DBConnMaxLifetime int
)

// ============================================================================
// 企微平台配置
// ============================================================================
var (
	// WeComAPIBaseURL 企微 API 基地址
	WeComAPIBaseURL = "https://qyapi.weixin.qq.com/cgi-bin"
)

// ============================================================================
// 平台类型常量
// ============================================================================
const (
	PlatformWeCom    = "wecom"
	PlatformDingTalk = "dingtalk"
	PlatformFeishu   = "feishu"
)

// ============================================================================
// 会话类型常量
// ============================================================================
const (
	ConversationTypeSingle = "1" // 单聊
	ConversationTypeGroup  = "2" // 群聊
)

// ============================================================================
// Redis Key 前缀定义
// 注意：实际 Redis Key 会由 db/redis.go 的 redisPrefixHook 自动添加全局前缀。
// 以下常量仅定义业务 Key 后缀，最终 Key 格式为：{prefix}:{常量值}
// ============================================================================
const (
	RedisPrefixToken      = "access_token"   // 企微 access_token 缓存
	RedisPrefixDedup      = "dedup"          // 消息去重
	RedisPrefixTokenUser  = "token"          // 用户 Token 缓存
	RedisPrefixAuthState  = "auth:state"     // 扫码登录 state
	RedisPrefixAuthToken  = "auth:token"     // 扫码登录临时 Token
	RedisPrefixAuthResult = "auth:result"    // 扫码登录结果
)

// ============================================================================
// 企微消息类型常量
// ============================================================================
const (
	WeComMsgTypeText  = "text"
	WeComMsgTypeImage = "image"
	WeComMsgTypeVoice = "voice"
	WeComMsgTypeFile  = "file"
	WeComMsgTypeEvent = "event"
)

// ============================================================================
// 企微事件类型常量
// ============================================================================
const (
	WeComEventSubscribe   = "subscribe"
	WeComEventUnsubscribe = "unsubscribe"
	WeComEventClick       = "click"
)
