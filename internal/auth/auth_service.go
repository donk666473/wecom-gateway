// Package auth 提供扫码登录服务。
// 参照设计文档 9 节扫码登录架构扩展、3.6 节扫码登录时序和 4.9 节。
//
// 核心流程：
// 1. GenerateAuthURL：生成 OAuth2 授权链接 + state，存入 Redis
// 2. HandleCallback：处理 OAuth2 回调，code 换用户信息，身份映射，免密登录
// 3. GetAuthStatus：前端轮询扫码结果状态
// 4. GetAuthToken：前端获取临时 Token（读取后删除）
package auth

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/wecom-gateway/internal/adapter"
	"github.com/wecom-gateway/internal/common"
	"github.com/wecom-gateway/internal/db"
	"github.com/wecom-gateway/internal/model"
	"github.com/wecom-gateway/internal/utils"
)

// ============================================================================
// 类型定义
// ============================================================================

// StateData OAuth2 state 存储数据
type StateData struct {
	Platform string `json:"platform"`
	AppID    string `json:"app_id"`
	CorpID   string `json:"corp_id"`
}

// AuthStatus 扫码状态
type AuthStatus string

const (
	// StatusPending 等待扫码
	StatusPending AuthStatus = "pending"
	// StatusSuccess 扫码成功
	StatusSuccess AuthStatus = "success"
	// StatusExpired 已过期
	StatusExpired AuthStatus = "expired"
	// StatusError 扫码失败
	StatusError AuthStatus = "error"
)

// AuthResult 扫码结果
type AuthResult struct {
	Status AuthStatus `json:"status"`
	Token  string     `json:"token,omitempty"`
	Error  string     `json:"error,omitempty"`
}

// AuthURLResponse 授权链接响应
type AuthURLResponse struct {
	AuthURL string `json:"auth_url"`
	State   string `json:"state"`
}

// ============================================================================
// AuthService — 扫码登录服务
// ============================================================================

// AuthService 扫码登录服务。
// 依赖：AdapterManager（生成OAuth2链接）、DatrixBridge（身份映射+免密登录）
type AuthService struct {
	adapterManager AdapterManager
	bridgeAuth     BridgeAuth
}

// AdapterManager 适配器管理器接口
type AdapterManager interface {
	GetAdapter(appID string) adapter.AbstractIMAdapter
	GetWeComAdapters() []adapter.AbstractIMAdapter
}

// BridgeAuth DATRIX 认证接口
type BridgeAuth interface {
	SearchUser(platform, unionID string) (isExist bool, userName string, err error)
	Login(param *LoginParam) (token string, userID string, err error)
	GenerateFreePassword(userName string) string
}

// LoginParam 简化的登录参数
type LoginParam struct {
	Password string
	LoginID  string
	From     int
	Type     int
}

// NewAuthService 创建 AuthService
func NewAuthService(am AdapterManager, bridge BridgeAuth) *AuthService {
	return &AuthService{
		adapterManager: am,
		bridgeAuth:     bridge,
	}
}

// ============================================================================
// 公开方法
// ============================================================================

// GenerateAuthURL 生成 OAuth2 授权链接。
// 1. 生成 UUID 作为 state
// 2. 存储 state → {platform, app_id} 到 Redis（TTL 5 分钟）
// 3. 调用适配器生成平台特定的 OAuth2 URL
// 4. 初始化扫码状态为 pending
func (s *AuthService) GenerateAuthURL(platform, appID string) (*AuthURLResponse, error) {
	adapterInstance := s.adapterManager.GetAdapter(appID)
	if adapterInstance == nil {
		return nil, fmt.Errorf("适配器不存在: %s", appID)
	}

	// 生成 state
	state := utils.GenerateUUIDWithDash()

	// 存储 state 映射
	stateData := StateData{
		Platform: platform,
		AppID:    appID,
	}
	stateJSON, _ := json.Marshal(stateData)
	stateKey := fmt.Sprintf("%s:%s", common.RedisPrefixAuthState, state)
	_ = db.RedisSet(stateKey, string(stateJSON), time.Duration(common.AuthStateTTL)*time.Second)

	// 初始化扫码状态
	resultKey := fmt.Sprintf("%s:%s", common.RedisPrefixAuthResult, state)
	result := AuthResult{Status: StatusPending}
	resultJSON, _ := json.Marshal(result)
	_ = db.RedisSet(resultKey, string(resultJSON), time.Duration(common.AuthStateTTL)*time.Second)

	// 生成授权链接
	authURL, err := adapterInstance.GetOAuth2URL(state)
	if err != nil {
		return nil, fmt.Errorf("生成授权链接失败: %w", err)
	}

	return &AuthURLResponse{
		AuthURL: authURL,
		State:   state,
	}, nil
}

// HandleCallback 处理 OAuth2 回调。
// 完整流程：校验 state → code 换用户信息 → 查询 DATRIX 用户 → 免密登录 → 绑定 → 暂存 Token
func (s *AuthService) HandleCallback(code, state string) error {
	// 1. 校验 state
	stateKey := fmt.Sprintf("%s:%s", common.RedisPrefixAuthState, state)
	stateJSON, err := db.RedisGet(stateKey)
	if err != nil {
		return fmt.Errorf("state 无效或已过期")
	}

	var stateData StateData
	if err := json.Unmarshal([]byte(stateJSON), &stateData); err != nil {
		return fmt.Errorf("解析 state 失败: %w", err)
	}

	// 2. 获取适配器
	adapterInstance := s.adapterManager.GetAdapter(stateData.AppID)
	if adapterInstance == nil {
		return fmt.Errorf("适配器不存在: %s", stateData.AppID)
	}

	// 3. 通过 code 换取用户信息
	imUser, err := adapterInstance.GetUserByCode(code)
	if err != nil {
		s.setAuthError(state, fmt.Sprintf("获取用户信息失败: %v", err))
		return err
	}

	// 企微使用 userId 作为唯一标识
	unionID := imUser.UserID
	if imUser.UnionID != "" {
		unionID = imUser.UnionID
	}

	// 4. 查询 DATRIX 用户
	isExist, userName, err := s.bridgeAuth.SearchUser(stateData.Platform, unionID)
	if err != nil {
		s.setAuthError(state, fmt.Sprintf("查询DATRIX用户失败: %v", err))
		return err
	}
	if !isExist {
		s.setAuthError(state, "该IM账号未绑定DATRIX平台用户")
		return fmt.Errorf("用户未绑定")
	}

	// 5. 免密登录
	pwd := s.bridgeAuth.GenerateFreePassword(userName)
	token, userID, err := s.bridgeAuth.Login(&LoginParam{
		Password: pwd,
		From:     0,
		Type:     1,
	})
	if err != nil {
		s.setAuthError(state, fmt.Sprintf("免密登录失败: %v", err))
		return err
	}

	// 6. 保存绑定关系
	_ = model.SaveOrUpdateIMUserInfo(&model.WeComUserInfo{
		Platform:       stateData.Platform,
		StaffID:        imUser.UserID,
		UnionID:        unionID,
		Mobile:         imUser.Mobile,
		DatrixUserID:   userID,
		DatrixUserName: userName,
	})

	// 7. 暂存 Token（一次性读取，TTL 30 秒）
	tokenKey := fmt.Sprintf("%s:%s", common.RedisPrefixAuthToken, state)
	tokenData := map[string]string{
		"token":   token,
		"user_id": userID,
		"name":    userName,
	}
	tokenJSON, _ := json.Marshal(tokenData)
	_ = db.RedisSet(tokenKey, string(tokenJSON), time.Duration(common.AuthTokenTTL)*time.Second)

	// 8. 更新扫码状态为成功
	s.setAuthResult(state, AuthResult{Status: StatusSuccess, Token: token})

	utils.Sugar.Infof("[AuthService] 扫码登录成功 [platform=%s, user=%s]", stateData.Platform, userName)
	return nil
}

// GetAuthStatus 获取扫码状态（前端轮询）。
func (s *AuthService) GetAuthStatus(state string) (*AuthResult, error) {
	resultKey := fmt.Sprintf("%s:%s", common.RedisPrefixAuthResult, state)
	resultJSON, err := db.RedisGet(resultKey)
	if err != nil {
		return &AuthResult{Status: StatusExpired}, nil
	}

	var result AuthResult
	if err := json.Unmarshal([]byte(resultJSON), &result); err != nil {
		return &AuthResult{Status: StatusExpired}, nil
	}

	// 如果扫码成功，读取 Token 后立即删除
	if result.Status == StatusSuccess && result.Token == "" {
		tokenKey := fmt.Sprintf("%s:%s", common.RedisPrefixAuthToken, state)
		tokenJSON, err := db.RedisGetDel(tokenKey)
		if err == nil {
			var tokenData map[string]string
			if json.Unmarshal([]byte(tokenJSON), &tokenData) == nil {
				result.Token = tokenData["token"]
			}
		}
	}

	return &result, nil
}

// ============================================================================
// 内部方法
// ============================================================================

// setAuthResult 更新扫码结果状态
func (s *AuthService) setAuthResult(state string, result AuthResult) {
	resultKey := fmt.Sprintf("%s:%s", common.RedisPrefixAuthResult, state)
	resultJSON, _ := json.Marshal(result)
	_ = db.RedisSet(resultKey, string(resultJSON), time.Duration(common.AuthStateTTL)*time.Second)
}

// setAuthError 设置扫码错误状态
func (s *AuthService) setAuthError(state, errMsg string) {
	s.setAuthResult(state, AuthResult{Status: StatusError, Error: errMsg})
	utils.Sugar.Warnf("[AuthService] 扫码登录失败 [state=%s]: %s", state, errMsg)
}
