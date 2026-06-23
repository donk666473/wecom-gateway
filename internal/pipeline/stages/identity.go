package stages

import (
	"fmt"

	"github.com/wecom-gateway/internal/adapter"
	"github.com/wecom-gateway/internal/common"
	"github.com/wecom-gateway/internal/model"
	"github.com/wecom-gateway/internal/pipeline"
	"github.com/wecom-gateway/internal/utils"
)

// ============================================================================
// IdentityStage — 用户身份映射阶段
// 参照设计文档 3.3 节用户身份映射决策树和 4.2 节。
//
// 处理流程：
// 1. 查询本地 im_user_info 表
// 2. 未绑定时：从 IM 平台获取用户信息 → 查询 DATRIX 用户
// 3. 自动绑定或返回未绑定错误
// ============================================================================

// IdentityStage 用户身份映射阶段。
// 需要注入 AdapterManager 和 DatrixBridge 依赖。
type IdentityStage struct {
	adapterManager AdapterManager
	bridgeUserChecker BridgeUserChecker
}

// AdapterManager 适配器管理器接口（依赖反转）
type AdapterManager interface {
	GetAdapter(appID string) adapter.AbstractIMAdapter
}

// BridgeUserChecker DATRIX 用户查询接口（依赖反转）
type BridgeUserChecker interface {
	SearchUser(platform, unionID string) (isExist bool, userName string, err error)
}

// NewIdentityStage 创建身份映射阶段
func NewIdentityStage(am AdapterManager, checker BridgeUserChecker) *IdentityStage {
	return &IdentityStage{
		adapterManager:   am,
		bridgeUserChecker: checker,
	}
}

func (s *IdentityStage) Name() string { return "identity" }

func (s *IdentityStage) Process(ctx *pipeline.Context) *pipeline.Result {
	event := ctx.Event

	// 1. 先从本地数据库查询已绑定的用户
	userInfo, err := model.GetIMUserInfo(event.Platform, event.SenderID)
	if err == nil && userInfo != nil && userInfo.DatrixUserName != "" {
		ctx.DatrixUserName = userInfo.DatrixUserName
		ctx.Metadata["user_already_bound"] = true
		utils.Sugar.Debugf("[%s] 用户已绑定 [user=%s, staff_id=%s]",
			s.Name(), userInfo.DatrixUserName, event.SenderID)
		return pipeline.Continue()
	}

	// 2. 未绑定 — 从 IM 平台获取用户信息
	adapterInstance := s.adapterManager.GetAdapter(event.AppID)
	if adapterInstance == nil {
		return pipeline.Interrupt(fmt.Errorf("适配器不存在: app_id=%s", event.AppID))
	}

	imUser, err := adapterInstance.GetUserInfo(event.SenderID)
	if err != nil {
		utils.Sugar.Warnf("[%s] 获取IM用户信息失败: %v", s.Name(), err)
		ctx.IMUserInfo = &adapter.IMUserInfo{UserID: event.SenderID, UnionID: event.SenderID}
	} else {
		ctx.IMUserInfo = imUser
	}

	// 企微使用 userId 作为唯一标识
	unionID := event.SenderID
	if ctx.IMUserInfo != nil && ctx.IMUserInfo.UnionID != "" {
		unionID = ctx.IMUserInfo.UnionID
	}

	// 3. 查询 DATRIX 平台用户
	isExist, userName, err := s.bridgeUserChecker.SearchUser(event.Platform, unionID)
	if err != nil {
		utils.Sugar.Errorf("[%s] 查询DATRIX用户失败: %v", s.Name(), err)
		return pipeline.Interrupt(fmt.Errorf("查询DATRIX用户失败: %w", err))
	}

	if !isExist {
		utils.Sugar.Warnf("[%s] 用户未绑定DATRIX [platform=%s, user=%s]",
			s.Name(), event.Platform, event.SenderID)
		ctx.Error = common.ErrUserNotBound
		return pipeline.Interrupt(common.ErrUserNotBound)
	}

	// 4. 保存绑定关系到本地数据库
	mobile := ""
	if ctx.IMUserInfo != nil {
		mobile = ctx.IMUserInfo.Mobile
	}
	if err := model.SaveOrUpdateIMUserInfo(&model.WeComUserInfo{
		Platform:       event.Platform,
		StaffID:        event.SenderID,
		UnionID:        unionID,
		Mobile:         mobile,
		DatrixUserName: userName,
	}); err != nil {
		utils.Sugar.Warnf("[%s] 保存用户绑定关系失败: %v", s.Name(), err)
	}

	ctx.DatrixUserName = userName
	utils.Sugar.Infof("[%s] 用户身份映射成功 [platform=%s, staff=%s → datrix=%s]",
		s.Name(), event.Platform, event.SenderID, userName)

	return pipeline.Continue()
}
