// Package bridge 提供 DATRIX 后端对接的数据类型定义。
// 参照 DATRIX 设计文档 4.7 节 DATRIX 后端对接和 6.2 节数据类型。
package bridge

// ============================================================================
// 对话相关
// ============================================================================

// ChatMessage DATRIX 对话请求
type ChatMessage struct {
	Question          string     `json:"question"`
	History           [][]string `json:"history"`
	KnowledgeBases    []string   `json:"knowledge_bases"`
	KnowledgeBaseOnly bool       `json:"knowledge_base_only"`
}

// ChatResponse DATRIX 对话响应（WebSocket 流式）
type ChatResponse struct {
	Content string `json:"content"`
	IsFinal bool   `json:"is_final"`
	IsError bool   `json:"is_error"`
	Error   string `json:"error,omitempty"`
}

// ============================================================================
// 智能体相关
// ============================================================================

// AssistantInfo 智能体信息
type AssistantInfo struct {
	AssistantID       string   `json:"assistant_id"`
	AssistantName     string   `json:"assistant_name"`
	LogoURL           string   `json:"logo_url"`
	KnowledgeBaseIDs  []string `json:"knowledge_base_ids"`
	KnowledgeBaseOnly bool     `json:"knowledge_base_only"`
	NeedUserKnowledge bool     `json:"need_user_knowledge"`
}

// ============================================================================
// 登录相关
// ============================================================================

// LoginParam 免密登录参数
type LoginParam struct {
	Password string `json:"password"`
	LoginID  string `json:"loginId"`
	From     int    `json:"from"`
	Type     int    `json:"type"`
}

// ============================================================================
// 用户查询相关
// ============================================================================

// SearchUserResult 用户查询结果
type SearchUserResult struct {
	IsExist  bool   `json:"is_exist"`
	UserName string `json:"user_name"`
}
