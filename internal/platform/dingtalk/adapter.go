// Package dingtalk 提供钉钉平台适配器的占位实现。
// 当前为结构占位：可创建实例并注册，但实际对话功能待后续实现。
package dingtalk

import (
	"encoding/json"
	"fmt"

	"github.com/wecom-gateway/internal/adapter"
	"github.com/wecom-gateway/internal/model"
	"github.com/wecom-gateway/internal/utils"
)

func init() {
	adapter.RegisterAdapter("dingtalk", func(app *model.WeComApp) (adapter.AbstractIMAdapter, error) {
		return NewDingTalkAdapter(app)
	})
}

// ExtraConfig 钉钉扩展配置（占位）。
type ExtraConfig struct {
	AppKey    string `json:"app_key"`
	AppSecret string `json:"app_secret"`
	// Webhook 模式：机器人 outgoing webhook 加签密钥
	WebhookToken  string `json:"webhook_token,omitempty"`
	WebhookSecret string `json:"webhook_secret,omitempty"`
}

// ParseExtraConfig 解析钉钉配置 JSON。
func ParseExtraConfig(extraJSON string) (*ExtraConfig, error) {
	cfg := &ExtraConfig{}
	if extraJSON == "" {
		return cfg, nil
	}
	return cfg, json.Unmarshal([]byte(extraJSON), cfg)
}

// Validate 校验钉钉配置。
func (c *ExtraConfig) Validate() error {
	if c.AppKey == "" {
		return fmt.Errorf("app_key is required")
	}
	if c.AppSecret == "" {
		return fmt.Errorf("app_secret is required")
	}
	return nil
}

// DingTalkAdapter 钉钉平台适配器（占位实现）。
type DingTalkAdapter struct {
	appID   string
	appKey  string
	appSecret string
}

// NewDingTalkAdapter 创建钉钉适配器实例。
func NewDingTalkAdapter(app *model.WeComApp) (*DingTalkAdapter, error) {
	cfg, err := ParseExtraConfig(app.ExtraConfig)
	if err != nil {
		return nil, fmt.Errorf("解析钉钉配置失败: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &DingTalkAdapter{
		appID:     app.AppID,
		appKey:    cfg.AppKey,
		appSecret: cfg.AppSecret,
	}, nil
}

// Platform 返回平台类型。
func (d *DingTalkAdapter) Platform() string { return "dingtalk" }

// AppID 返回应用 ID。
func (d *DingTalkAdapter) AppID() string { return d.appID }

// Start 启动钉钉适配器（占位）。
func (d *DingTalkAdapter) Start() error {
	utils.Sugar.Infof("[DingTalkAdapter] 钉钉适配器已启动 [app_id=%s]（功能占位）", d.appID)
	return nil
}

// Stop 停止钉钉适配器（占位）。
func (d *DingTalkAdapter) Stop() error {
	utils.Sugar.Infof("[DingTalkAdapter] 钉钉适配器已停止 [app_id=%s]", d.appID)
	return nil
}

// OnMessage 注册消息处理回调。
func (d *DingTalkAdapter) OnMessage(eventType adapter.EventType, handler adapter.MessageHandler) {
	utils.Sugar.Debugf("[DingTalkAdapter] OnMessage 注册未实现 [event_type=%s]", eventType)
}

// ReplyMessage 回复消息（占位）。
func (d *DingTalkAdapter) ReplyMessage(event *adapter.IMEvent, content string) error {
	return fmt.Errorf("钉钉 ReplyMessage 尚未实现")
}

// ReplyMessageChunk 回复流式片段（占位）。
func (d *DingTalkAdapter) ReplyMessageChunk(event *adapter.IMEvent, chunk *adapter.ReplyChunk) error {
	return fmt.Errorf("钉钉 ReplyMessageChunk 尚未实现")
}

// GetUserInfo 获取用户信息（占位）。
func (d *DingTalkAdapter) GetUserInfo(userID string) (*adapter.IMUserInfo, error) {
	return nil, fmt.Errorf("钉钉 GetUserInfo 尚未实现")
}

// GetAccessToken 获取 AccessToken（占位）。
func (d *DingTalkAdapter) GetAccessToken() (string, error) {
	return "", fmt.Errorf("钉钉 GetAccessToken 尚未实现")
}
