// Package processor 提供消息处理器的核心逻辑。
// 参照设计文档 5.1 节 MessageProcessor 和 LangBot 框架的 Pipeline 模式。
//
// 核心职责：
// 1. 组装 Pipeline 各阶段（去重→身份映射→Token→会话→对话→回复）
// 2. 作为 BotManager 与 Pipeline 之间的桥梁
// 3. 处理 Pipeline 中断时的错误响应（如发送未绑定提示）
package processor

import (
	"github.com/wecom-gateway/internal/adapter"
	"github.com/wecom-gateway/internal/botmgr"
	"github.com/wecom-gateway/internal/bridge"
	"github.com/wecom-gateway/internal/common"
	"github.com/wecom-gateway/internal/model"
	"github.com/wecom-gateway/internal/pipeline"
	"github.com/wecom-gateway/internal/pipeline/stages"
	"github.com/wecom-gateway/internal/utils"
)

// ============================================================================
// MessageProcessor — 消息处理核心
// ============================================================================

// MessageProcessor 消息处理器。
// 负责组装 Pipeline 并处理消息事件。
type MessageProcessor struct {
	pipeline   *pipeline.Pipeline
	botManager *botmgr.BotManager
}

// NewMessageProcessor 创建消息处理器。
// 组装完整的 Pipeline 阶段链：
// Dedup → Identity → Token → Session → Chat → Reply
func NewMessageProcessor(botMgr *botmgr.BotManager, datrixBridge bridge.DatrixBridge) *MessageProcessor {
	mp := &MessageProcessor{
		pipeline:   pipeline.NewPipeline("wecom_message"),
		botManager: botMgr,
	}

	// 组装 Pipeline 阶段链（参照 LangBot 责任链模式）
	mp.pipeline.AddStages(
		// 阶段 1：消息去重
		stages.NewDedupStage(),
		// 阶段 2：用户身份映射
		stages.NewIdentityStage(botMgr, datrixBridge),
		// 阶段 3：Token 管理
		stages.NewTokenStage(datrixBridge),
		// 阶段 4：会话管理（先解析智能体ID）
		pipeline.NewStage("resolve_assistant", mp.resolveAssistant),
		// 阶段 5：会话管理
		stages.NewSessionStage(datrixBridge),
		// 阶段 6：AI 对话
		stages.NewChatStage(datrixBridge),
		// 阶段 7：消息回复
		stages.NewReplyStage(botMgr),
	)

	return mp
}

// ProcessEvent 处理 IM 事件（Pipeline 入口）。
// 由 BotManager 在收到消息时调用。
func (p *MessageProcessor) ProcessEvent(event *adapter.IMEvent) {
	ctx := pipeline.NewContext(event)

	// 执行 Pipeline
	if err := p.pipeline.Execute(ctx); err != nil {
		// Pipeline 中断 — 根据不同错误类型发送友好提示
		p.handlePipelineError(ctx, err)
	}
}

// handlePipelineError 处理 Pipeline 中断错误。
// 根据错误类型向用户发送不同的友好提示。
func (p *MessageProcessor) handlePipelineError(ctx *pipeline.Context, err error) {
	adapterInstance := p.botManager.GetAdapter(ctx.Event.AppID)
	if adapterInstance == nil {
		utils.Sugar.Errorf("[MessageProcessor] 适配器不存在 [app_id=%s]", ctx.Event.AppID)
		return
	}

	var replyMsg string

	switch err {
	case common.ErrUserNotBound:
		replyMsg = "您尚未绑定 DATRIX 账号，请先在 DATRIX 平台个人中心绑定企微账号后再试。"
	case common.ErrSessionExpired:
		replyMsg = "会话已过期，请重新发送消息。"
	case common.ErrTokenInvalid:
		replyMsg = "登录状态已失效，请重新登录 DATRIX 平台后重试。"
	default:
		replyMsg = "抱歉，处理您的消息时出现了问题，请稍后重试。"
		utils.Sugar.Errorf("[MessageProcessor] 消息处理失败: %v", err)
	}

	_ = adapterInstance.ReplyMessage(ctx.Event, replyMsg)
}

// resolveAssistant 解析当前绑定智能体ID的 Pipeline 阶段。
// 从数据库查询应用的默认智能体。
func (p *MessageProcessor) resolveAssistant(ctx *pipeline.Context) *pipeline.StageResult {
	binding, err := model.GetDefaultAssistant(ctx.Event.AppID)
	if err != nil {
		utils.Sugar.Warnf("[MessageProcessor] 未找到默认智能体 [app_id=%s]", ctx.Event.AppID)
		ctx.AssistantID = "default"
	} else {
		ctx.AssistantID = binding.AssistantID
	}
	return pipeline.Continue()
}

// ============================================================================
// BotManager 接口适配（满足 stages 接口的依赖反转）
// ============================================================================

// GetAdapter 实现 AdapterManager 接口
func (p *MessageProcessor) GetAdapter(appID string) adapter.AbstractIMAdapter {
	return p.botManager.GetAdapter(appID)
}
