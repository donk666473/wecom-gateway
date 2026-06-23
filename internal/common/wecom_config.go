// Package common 提供企微平台特有配置的解析和类型定义。
package common

import "encoding/json"

// WeComExtraConfig 企微扩展配置，存储在 im_app 表的 extra_config JSON 字段中。
// 参照 DATRIX 设计文档 4.1.2 节企微应用配置规范。
type WeComExtraConfig struct {
	// Token 接收消息验证 Token（必填）
	Token string `json:"token"`
	// EncodingAESKey 消息加解密密钥（必填）
	EncodingAESKey string `json:"encoding_aes_key"`
	// AgentID 企微应用 AgentId（必填）
	AgentID int `json:"agent_id"`
	// ContactsSecret 通讯录同步 Secret（选填，用于获取用户信息）
	ContactsSecret string `json:"contacts_secret,omitempty"`
	// APIBaseURL 企微 API 基地址（默认 https://qyapi.weixin.qq.com/cgi-bin）
	APIBaseURL string `json:"api_base_url,omitempty"`
	// OAuth2CorpID 扫码登录使用的 CorpID（选填）
	OAuth2CorpID string `json:"oauth2_corp_id,omitempty"`
	// OAuth2AgentID 扫码登录使用的应用 AgentID（选填）
	OAuth2AgentID int `json:"oauth2_agent_id,omitempty"`
	// WebhookURL 回调 URL（用于企微 Webhook 模式）
	WebhookURL string `json:"webhook_url,omitempty"`
}

// ParseWeComConfig 解析企微扩展配置 JSON，并对必填字段进行默认值填充。
// extraJSON 来自 im_app 表的 extra_config 字段。
func ParseWeComConfig(extraJSON string) (*WeComExtraConfig, error) {
	cfg := &WeComExtraConfig{
		APIBaseURL: "https://qyapi.weixin.qq.com/cgi-bin",
	}
	if err := json.Unmarshal([]byte(extraJSON), cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Validate 校验企微配置必填项。
// 返回 nil 表示校验通过。
func (c *WeComExtraConfig) Validate() error {
	if c.Token == "" {
		return ErrMissingToken
	}
	if c.EncodingAESKey == "" {
		return ErrMissingAESKey
	}
	if c.AgentID <= 0 {
		return ErrMissingAgentID
	}
	return nil
}

// 配置校验错误
var (
	ErrMissingToken   = &ConfigError{"token is required"}
	ErrMissingAESKey  = &ConfigError{"encoding_aes_key is required"}
	ErrMissingAgentID = &ConfigError{"agent_id is required and must be positive"}
)
