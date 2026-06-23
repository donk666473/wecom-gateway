// Package adapter 提供 IM 平台适配器的类型定义。
// 参照 DATRIX 设计文档 4.1 节适配器接口和架构设计篇第四章。
package adapter

// EventType 事件类型
type EventType string

const (
	// FriendMessage 单聊消息事件
	FriendMessage EventType = "FriendMessage"
	// GroupMessage 群聊消息事件
	GroupMessage EventType = "GroupMessage"
)

// ============================================================================
// 统一 IM 事件 — 屏蔽不同 IM 平台的消息差异
// ============================================================================

// IMEvent 统一 IM 事件结构体。
// 各平台适配器负责将平台特有消息格式转换为此统一格式。
type IMEvent struct {
	// Platform 平台类型（wecom/dingtalk/feishu）
	Platform string `json:"platform"`
	// MessageID 消息唯一标识（用于去重）
	MessageID string `json:"message_id"`
	// SenderID 发送者 ID（企微 userId）
	SenderID string `json:"sender_id"`
	// SenderNick 发送者昵称
	SenderNick string `json:"sender_nick"`
	// ConversationID 会话 ID（单聊时为 userId，群聊时为群 ID）
	ConversationID string `json:"conversation_id"`
	// ConversationType 会话类型 "1"=单聊 "2"=群聊
	ConversationType string `json:"conversation_type"`
	// Content 消息文本内容
	Content string `json:"content"`
	// MsgType 原始消息类型（text/image/voice/file/event）
	MsgType string `json:"msg_type"`
	// RawEvent 原始平台事件（用于调试和扩展）
	RawEvent interface{} `json:"raw_event"`
	// AppID 关联的应用 ID
	AppID string `json:"app_id"`
	// CorpID 企业 ID
	CorpID string `json:"corp_id"`
}

// ============================================================================
// 回复消息结构
// ============================================================================

// ReplyChunk 流式回复片段。
// 用于支持流式输出场景（如企微分段消息）。
type ReplyChunk struct {
	// Content 回复内容片段
	Content string `json:"content"`
	// IsFinal 是否为最后一个片段
	IsFinal bool `json:"is_final"`
	// IsError 是否为错误消息
	IsError bool `json:"is_error"`
	// Error 错误信息
	Error string `json:"error,omitempty"`
	// CardInstanceID 卡片实例 ID（钉钉流式卡片场景）
	CardInstanceID string `json:"card_instance_id,omitempty"`
}

// ============================================================================
// IM 用户信息
// ============================================================================

// IMUserInfo IM 平台用户信息（从 IM 平台 API 获取）
type IMUserInfo struct {
	// UserID IM 平台用户 ID
	UserID string `json:"user_id"`
	// UnionID IM 平台 UnionID（跨应用唯一标识）
	UnionID string `json:"union_id"`
	// Nickname 用户昵称
	Nickname string `json:"nickname"`
	// Mobile 手机号（脱敏）
	Mobile string `json:"mobile"`
}

// ============================================================================
// 消息处理回调函数类型
// ============================================================================

// MessageHandler 消息处理回调函数类型。
// 适配器收到消息后，调用注册的回调函数进行处理。
type MessageHandler func(event *IMEvent)
