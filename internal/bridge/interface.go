// Package bridge 提供 DATRIX 后端对接的抽象接口定义。
// 参照设计文档 6.1 节 DatrixBridge 接口和 LangBot 框架的 Provider 抽象模式。
//
// 设计原则：
// 1. Bridge 层封装所有与 DATRIX 后端的通信逻辑
// 2. 上层业务（MessageProcessor、AuthService）通过此接口调用
// 3. 接口抽象允许替换不同版本的 DATRIX 后端实现
package bridge

// DatrixBridge DATRIX 后端对接接口。
// 封装所有与 DATRIX Asset 和 Assistant 服务的交互。
type DatrixBridge interface {
	// ========================================================================
	// 用户管理
	// ========================================================================

	// SearchUser 查询 DATRIX 用户是否存在。
	// platform: IM 平台类型（wecom/dingtalk）
	// unionID: IM 平台用户唯一标识（企微为 userId）
	// 返回：是否存在、用户名、错误
	SearchUser(platform, unionID string) (isExist bool, userName string, err error)

	// ========================================================================
	// Token 管理
	// ========================================================================

	// Login 免密登录获取 DATRIX Token。
	// 使用 AES 加密的用户名密码进行免密登录。
	Login(param *LoginParam) (token string, userID string, err error)

	// CheckTokenValid 检查 Token 是否有效
	CheckTokenValid(token string) bool

	// GenerateFreePassword 生成免密登录密码。
	// 使用配置的 AES Key 和 Signing Key 加密用户信息。
	GenerateFreePassword(userName string) string

	// ========================================================================
	// 智能体管理
	// ========================================================================

	// GetAssistantInfo 获取智能体详细信息。
	GetAssistantInfo(token, assistantID string) (*AssistantInfo, error)

	// ========================================================================
	// 会话管理
	// ========================================================================

	// CreateAssistantSession 创建智能体对话会话。
	CreateAssistantSession(token, assistantID, userID string) (sessionID string, err error)

	// GetHistory 获取对话历史上下文。
	GetHistory(token, sessionID string) ([][]string, error)

	// ========================================================================
	// 对话
	// ========================================================================

	// ChatWithAssistant 与 DATRIX 智能体进行 WebSocket 流式对话。
	// 返回一个 channel，持续接收 AI 流式响应。
	ChatWithAssistant(token, sessionID, userID string, msg *ChatMessage) (<-chan ChatResponse, error)
}
