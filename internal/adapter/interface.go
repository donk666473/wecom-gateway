// Package adapter 提供 IM 平台适配器的抽象接口定义。
// 参照 LangBot 框架的 AbstractMessagePlatformAdapter 模式
// 和 DATRIX 设计文档 4.1 节 AbstractIMAdapter 接口定义。
//
// 设计原则：
// 1. 适配器模式：每种 IM 平台实现独立 Adapter，统一继承此接口
// 2. 上层业务无需关心平台差异
// 3. 支持运行时动态加载和卸载适配器
package adapter

// AbstractIMAdapter IM 平台适配器抽象接口。
// 所有 IM 平台（企微、钉钉、飞书）适配器必须实现此接口。
type AbstractIMAdapter interface {
	// ========================================================================
	// 生命周期管理
	// ========================================================================

	// Start 启动适配器。
	// 企微：无操作（Webhook 由 HTTP Router 处理）
	// 钉钉：建立 Stream WebSocket 长连接
	Start() error

	// Stop 停止适配器。
	// 企微：无操作
	// 钉钉：关闭 Stream 连接
	Stop() error

	// ========================================================================
	// 事件注册
	// ========================================================================

	// OnMessage 注册消息处理回调。
	// eventType: 事件类型（单聊/群聊）
	// handler: 消息处理回调函数
	OnMessage(eventType EventType, handler MessageHandler)

	// ========================================================================
	// 消息回复
	// ========================================================================

	// ReplyMessage 回复完整消息（非流式）。
	// 企微：分段发送文本消息
	// 钉钉：发送交互卡片
	ReplyMessage(event *IMEvent, content string) error

	// ReplyMessageChunk 回复流式消息片段。
	// 企微：不支持流式卡片，此方法返回 nil（由 ReplyMessage 处理最终回复）
	// 钉钉：更新流式交互卡片
	ReplyMessageChunk(event *IMEvent, chunk *ReplyChunk) error

	// ========================================================================
	// 用户信息获取
	// ========================================================================

	// GetUserInfo 从 IM 平台获取用户信息。
	// userID: IM 平台用户 ID
	GetUserInfo(userID string) (*IMUserInfo, error)

	// ========================================================================
	// 平台标识
	// ========================================================================

	// Platform 返回平台类型字符串（wecom/dingtalk/feishu）
	Platform() string

	// AppID 返回关联的应用 ID
	AppID() string

	// ========================================================================
	// 扫码登录扩展（OAuth2）
	// ========================================================================

	// GetOAuth2URL 生成 IM 平台 OAuth2 授权链接。
	// state: 防 CSRF 的 state 参数
	GetOAuth2URL(state string) (string, error)

	// GetUserByCode 通过 OAuth2 code 换取 IM 用户信息。
	// code: OAuth2 回调返回的授权码
	GetUserByCode(code string) (*IMUserInfo, error)

	// ========================================================================
	// Token 管理
	// ========================================================================

	// GetAccessToken 获取 IM 平台的 Access Token（带缓存）。
	GetAccessToken() (string, error)
}
