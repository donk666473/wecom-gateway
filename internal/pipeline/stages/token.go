package stages

import (
	"fmt"

	"github.com/wecom-gateway/internal/bridge"
	"github.com/wecom-gateway/internal/common"
	"github.com/wecom-gateway/internal/db"
	"github.com/wecom-gateway/internal/model"
	"github.com/wecom-gateway/internal/pipeline"
	"github.com/wecom-gateway/internal/utils"
)

// ============================================================================
// TokenStage — Token 管理阶段
// 参照设计文档 3.4 节 Token 管理状态机和 4.3 节。
//
// 处理流程：
// 1. 从 Redis 缓存获取 Token → 验证有效性
// 2. Token 无效/不存在 → 免密登录获取新 Token
// 3. 缓存新 Token 到 Redis
// ============================================================================

// TokenStage Token 管理阶段。
type TokenStage struct {
	bridgeTokenManager BridgeTokenManager
}

// BridgeTokenManager Token 管理接口（依赖反转）
type BridgeTokenManager interface {
	Login(param *bridge.LoginParam) (token string, userID string, err error)
	CheckTokenValid(token string) bool
	GenerateFreePassword(userName string) string
}

// NewTokenStage 创建 Token 管理阶段
func NewTokenStage(bridge BridgeTokenManager) *TokenStage {
	return &TokenStage{bridgeTokenManager: bridge}
}

func (s *TokenStage) Name() string { return "token" }

func (s *TokenStage) Process(ctx *pipeline.Context) *pipeline.StageResult {
	event := ctx.Event
	userName := ctx.DatrixUserName

	cacheKey := fmt.Sprintf("%s:%s:%s", common.RedisPrefixTokenUser, event.AppID, event.SenderID)

	// 1. 尝试从 Redis 获取缓存的 Token
	if cachedToken, err := db.RedisGet(cacheKey); err == nil && cachedToken != "" {
		if s.bridgeTokenManager.CheckTokenValid(cachedToken) {
			ctx.DatrixToken = cachedToken

			// 尝试从 sessions 表获取已关联的 userId
			session, _ := model.GetSessionInfo(
				event.AppID, event.SenderID, event.ConversationID,
				event.ConversationType == common.ConversationTypeGroup,
			)
			if session != nil {
				ctx.DatrixUserID = session.DatrixUserID
			}
			utils.Sugar.Debugf("[%s] Token缓存命中 [user=%s]", s.Name(), userName)
			return pipeline.Continue()
		}
		utils.Sugar.Debugf("[%s] Token缓存已过期，重新获取", s.Name())
	}

	// 2. 免密登录获取新 Token
	pwd := s.bridgeTokenManager.GenerateFreePassword(userName)
	token, userID, err := s.bridgeTokenManager.Login(&bridge.LoginParam{
		Password: pwd,
		LoginID:  userName,
		From:     0,
		Type:     1,
	})
	if err != nil {
		utils.Sugar.Errorf("[%s] 免密登录失败: %v", s.Name(), err)
		return pipeline.Interrupt(fmt.Errorf("免密登录失败: %w", err))
	}

	// 3. 缓存 Token
	_ = db.RedisSet(cacheKey, token, common.TokenTTL)

	ctx.DatrixToken = token
	ctx.DatrixUserID = userID
	utils.Sugar.Infof("[%s] Token获取成功 [user=%s, user_id=%s]", s.Name(), userName, userID)

	return pipeline.Continue()
}
