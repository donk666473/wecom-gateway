package stages

import (
	"fmt"

	"github.com/wecom-gateway/internal/adapter"
	"github.com/wecom-gateway/internal/pipeline"
	"github.com/wecom-gateway/internal/utils"
)

// ============================================================================
// ReplyStage — 消息回复阶段
// 参照设计文档 5.2 节回复数据流和 3.2 节企微回复时序。
//
// 企微回复策略：分段文本消息发送（每段 2048 字节）
// ============================================================================

// ReplyStage 消息回复阶段。
type ReplyStage struct {
	adapterManager AdapterManager
}

// NewReplyStage 创建回复阶段
func NewReplyStage(am AdapterManager) *ReplyStage {
	return &ReplyStage{adapterManager: am}
}

func (s *ReplyStage) Name() string { return "reply" }

func (s *ReplyStage) Process(ctx *pipeline.Context) *pipeline.Result {
	event := ctx.Event

	// 获取适配器
	adapterInstance := s.adapterManager.GetAdapter(event.AppID)
	if adapterInstance == nil {
		return pipeline.Interrupt(fmt.Errorf("适配器不存在: app_id=%s", event.AppID))
	}

	// 发送回复消息
	if ctx.FullResponse == "" {
		// 没有回复内容，发送默认消息
		ctx.FullResponse = "抱歉，我暂时无法回答您的问题，请稍后重试。"
	}

	if err := adapterInstance.ReplyMessage(event, ctx.FullResponse); err != nil {
		utils.Sugar.Errorf("[%s] 发送回复失败: %v", s.Name(), err)
		return pipeline.Interrupt(fmt.Errorf("发送回复失败: %w", err))
	}

	utils.Sugar.Infof("[%s] 回复发送成功 [msg_id=%s, len=%d]",
		s.Name(), event.MessageID, len(ctx.FullResponse))

	return pipeline.Continue()
}
