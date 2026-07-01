// Package wecom 提供企业微信适配器相关的常量定义。
package wecom

var (
	// APIBaseURL 企微 API 基地址（默认）
	APIBaseURL = "https://qyapi.weixin.qq.com/cgi-bin"
	// RedisPrefixToken Redis 中企微 access_token 缓存前缀
	RedisPrefixToken = "access_token"
)

// ============================================================================
// 企微消息类型常量
// ============================================================================
const (
	MsgTypeText  = "text"
	MsgTypeImage = "image"
	MsgTypeVoice = "voice"
	MsgTypeFile  = "file"
	MsgTypeEvent = "event"
)

// ============================================================================
// 企微事件类型常量
// ============================================================================
const (
	EventSubscribe   = "subscribe"
	EventUnsubscribe = "unsubscribe"
	EventClick       = "click"
)
