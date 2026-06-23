package stages

import (
	"fmt"
	"time"

	"github.com/wecom-gateway/internal/common"
	"github.com/wecom-gateway/internal/model"
	"github.com/wecom-gateway/internal/pipeline"
	"github.com/wecom-gateway/internal/utils"
)

// ============================================================================
// SessionStage — 会话管理阶段
// 参照设计文档 3.5 节会话管理状态机和 4.4 节。
//
// 处理流程：
// 1. 查询本地会话表
// 2. 会话有效且智能体相同 → 复用
// 3. 会话过期或智能体变更 → 创建新会话
// ============================================================================

// SessionStage 会话管理阶段。
type SessionStage struct {
	bridgeSessionManager BridgeSessionManager
}

// BridgeSessionManager 会话管理接口（依赖反转）
type BridgeSessionManager interface {
	CreateAssistantSession(token, assistantID, userID string) (sessionID string, err error)
}

// NewSessionStage 创建会话管理阶段
func NewSessionStage(bridge BridgeSessionManager) *SessionStage {
	return &SessionStage{bridgeSessionManager: bridge}
}

func (s *SessionStage) Name() string { return "session" }

func (s *SessionStage) Process(ctx *pipeline.Context) *pipeline.StageResult {
	event := ctx.Event
	isGroup := event.ConversationType == common.ConversationTypeGroup
	assistantID := ctx.AssistantID

	// 1. 查询本地会话
	session, err := model.GetSessionInfo(event.AppID, event.SenderID, event.ConversationID, isGroup)
	if err == nil && session != nil {
		// 检查会话是否有效
		sessionValid := time.Since(session.LastChatAt) < common.SessionTimeout
		assistantSame := session.AssistantID == assistantID

		if sessionValid && assistantSame {
			// 复用现有会话
			session.LastChatAt = time.Now()
			if saveErr := model.SaveOrUpdateSession(session); saveErr != nil {
				utils.Sugar.Warnf("[%s] 更新会话时间失败: %v", s.Name(), saveErr)
			}
			ctx.SessionID = session.SessionID
			ctx.DatrixUserID = session.DatrixUserID

			utils.Sugar.Debugf("[%s] 复用现有会话 [session_id=%s]", s.Name(), session.SessionID)
			return pipeline.Continue()
		}

		utils.Sugar.Debugf("[%s] 会话需要重建 [valid=%v, same_assistant=%v]",
			s.Name(), sessionValid, assistantSame)
	}

	// 2. 创建新会话
	sessionID, err := s.bridgeSessionManager.CreateAssistantSession(ctx.DatrixToken, assistantID, ctx.DatrixUserID)
	if err != nil {
		utils.Sugar.Errorf("[%s] 创建DATRIX会话失败: %v", s.Name(), err)
		return pipeline.Interrupt(fmt.Errorf("创建会话失败: %w", err))
	}

	// 3. 保存到本地数据库
	if saveErr := model.SaveOrUpdateSession(&model.WeComSession{
		AppID:          event.AppID,
		StaffID:        event.SenderID,
		ConversationID: event.ConversationID,
		IsGroup:        isGroup,
		DatrixUserID:   ctx.DatrixUserID,
		Token:          ctx.DatrixToken,
		SessionID:      sessionID,
		AssistantID:    assistantID,
		LastChatAt:     time.Now(),
	}); saveErr != nil {
		utils.Sugar.Warnf("[%s] 保存会话记录失败: %v", s.Name(), saveErr)
	}

	ctx.SessionID = sessionID
	utils.Sugar.Infof("[%s] 创建新会话 [session_id=%s, assistant=%s]",
		s.Name(), sessionID, assistantID)

	return pipeline.Continue()
}
