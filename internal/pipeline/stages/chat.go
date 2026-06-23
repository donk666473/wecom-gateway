package stages

import (
	"fmt"

	"github.com/wecom-gateway/internal/bridge"
	"github.com/wecom-gateway/internal/pipeline"
	"github.com/wecom-gateway/internal/utils"
)

// ============================================================================
// ChatStage — AI 对话阶段
// 参照设计文档 5.1 节消息处理核心和 3.1/3.2 节时序图。
//
// 处理流程：
// 1. 获取智能体信息
// 2. 获取历史对话上下文
// 3. 构建对话消息
// 4. 通过 WebSocket 与 DATRIX 智能体流式对话
// 5. 收集完整回复
// ============================================================================

// ChatStage AI 对话阶段。
type ChatStage struct {
	bridgeChatManager BridgeChatManager
}

// BridgeChatManager 对话管理接口（依赖反转）
type BridgeChatManager interface {
	GetAssistantInfo(token, assistantID string) (*bridge.AssistantInfo, error)
	GetHistory(token, sessionID string) ([][]string, error)
	ChatWithAssistant(token, sessionID, userID string, msg *bridge.ChatMessage) (<-chan bridge.ChatResponse, error)
}

// NewChatStage 创建对话阶段
func NewChatStage(bridge BridgeChatManager) *ChatStage {
	return &ChatStage{bridgeChatManager: bridge}
}

func (s *ChatStage) Name() string { return "chat" }

func (s *ChatStage) Process(ctx *pipeline.Context) *pipeline.StageResult {
	event := ctx.Event

	// 1. 获取智能体信息
	assistantInfo, err := s.bridgeChatManager.GetAssistantInfo(ctx.DatrixToken, ctx.AssistantID)
	if err != nil {
		utils.Sugar.Errorf("[%s] 获取智能体信息失败: %v", s.Name(), err)
		return pipeline.Interrupt(fmt.Errorf("获取智能体信息失败: %w", err))
	}
	ctx.AssistantInfo = assistantInfo

	// 2. 获取历史上下文
	history, err := s.bridgeChatManager.GetHistory(ctx.DatrixToken, ctx.SessionID)
	if err != nil {
		utils.Sugar.Warnf("[%s] 获取对话历史失败（非致命）: %v", s.Name(), err)
		history = nil
	}

	// 3. 构建对话消息
	chatMsg := &bridge.ChatMessage{
		Question:          event.Content,
		History:           history,
		KnowledgeBases:    assistantInfo.KnowledgeBaseIDs,
		KnowledgeBaseOnly: assistantInfo.KnowledgeBaseOnly,
	}

	// 4. 通过 WebSocket 与智能体对话
	respChan, err := s.bridgeChatManager.ChatWithAssistant(
		ctx.DatrixToken, ctx.SessionID, ctx.DatrixUserID, chatMsg,
	)
	if err != nil {
		utils.Sugar.Errorf("[%s] 建立对话连接失败: %v", s.Name(), err)
		return pipeline.Interrupt(fmt.Errorf("对话连接失败: %w", err))
	}

	// 5. 收集流式响应
	var fullResponse string
	for resp := range respChan {
		if resp.IsError {
			utils.Sugar.Errorf("[%s] 对话出错: %s", s.Name(), resp.Error)
			return pipeline.Interrupt(fmt.Errorf("对话出错: %s", resp.Error))
		}
		if resp.IsFinal {
			break
		}
		fullResponse = resp.Content
	}

	ctx.FullResponse = fullResponse
	utils.Sugar.Infof("[%s] 对话完成 [session=%s, len=%d]",
		s.Name(), ctx.SessionID, len(fullResponse))

	return pipeline.Continue()
}
