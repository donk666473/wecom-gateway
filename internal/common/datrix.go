// Package common 提供 DATRIX 平台相关的数据结构定义和错误类型。
// 参照 DATRIX 设计文档及钉钉对接项目中的 datrix 模块。
package common

import "fmt"

// ConfigError 配置错误类型
type ConfigError struct {
	Message string
}

func (e *ConfigError) Error() string {
	return fmt.Sprintf("config error: %s", e.Message)
}

// ============================================================================
// WebSocket 消息结构 — 发送给 DATRIX Assistant 的 WebSocket 消息
// ============================================================================

// WebSocketMessage 发送给 DATRIX 智能体的 WebSocket 消息体
type WebSocketMessage struct {
	SessionID          string     `json:"session_id"`
	Question           string     `json:"question"`
	History            [][]string `json:"history"`
	KnowledgeBases     []string   `json:"knowledge_bases"`
	KnowledgeBaseOnly  bool       `json:"knowledge_base_only"`
	Flag               int        `json:"flag"`
	FileIDs            []string   `json:"file_ids"`
	KnowledgeBaseFiles []string   `json:"knowledge_base_files"`
	FileQAActive       bool       `json:"file_qa_active"`
}

// ============================================================================
// 登录相关结构
// ============================================================================

// LoginParam DATRIX 免密登录入参
type LoginParam struct {
	Password string `json:"password"`
	LoginID  string `json:"loginId"`
	From     int    `json:"from"`
	Type     int    `json:"type"`
	WebID    string `json:"webId"`
}

// TokenInfo DATRIX 返回的 Token 信息
type TokenInfo struct {
	Server   string `json:"server"`
	Token    string `json:"token"`
	User     string `json:"user"`
	UserID   string `json:"userid"`
	Password string `json:"password"`
	ParentID string `json:"parentId"`
	Time     int64  `json:"time"`
}

// RedisTokenInfo Redis 中缓存的 Token 信息
type RedisTokenInfo struct {
	UserID string `json:"user_id"`
	Token  string `json:"token"`
}

// ============================================================================
// 智能体相关结构
// ============================================================================

// AssistantInfo 智能体信息
type AssistantInfo struct {
	AssistantID       string   `json:"id"`
	AssistantName     string   `json:"name"`
	LogoURL           string   `json:"logo"`
	KnowledgeBaseIDs  []string `json:"knowledge_base_ids"`
	KnowledgeBaseOnly bool     `json:"knowledge_base_only"`
	NeedUserKnowledge bool     `json:"need_user_knowledge"`
}

// ============================================================================
// 操作日志
// ============================================================================

// AddLogResponse DATRIX 操作日志接口响应
type AddLogResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Result  interface{} `json:"result"`
}

// ============================================================================
// 通用错误
// ============================================================================

// ErrUserNotBound 用户未绑定错误
var ErrUserNotBound = fmt.Errorf("用户未绑定DATRIX账号")

// ErrSessionExpired 会话已过期
var ErrSessionExpired = fmt.Errorf("会话已过期")

// ErrTokenInvalid Token 无效
var ErrTokenInvalid = fmt.Errorf("Token已失效")
