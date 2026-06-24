// Package bridge 提供 DATRIX 后端对接的具体实现。
// 参照钉钉对接项目中的 logic/datrix.go 和设计文档 6.3 节。
//
// 对接 DATRIX 两个服务：
// - Asset 服务（端口 30805）：用户查询、免密登录、Token 验证
// - Assistant 服务（端口 30806）：智能体管理、会话管理、WebSocket 流式对话
package bridge

import (
	"crypto/aes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/wecom-gateway/internal/common"
	"github.com/wecom-gateway/internal/utils"
)

// ============================================================================
// DatrixBridgeImpl — DatrixBridge 接口的具体实现
// ============================================================================

// DatrixBridgeImpl DATRIX 后端对接实现。
type DatrixBridgeImpl struct {
	httpClient *http.Client
}

// NewDatrixBridge 创建 DatrixBridge 实例。
func NewDatrixBridge() *DatrixBridgeImpl {
	return &DatrixBridgeImpl{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ============================================================================
// 用户管理
// ============================================================================

// SearchUser 查询 DATRIX 平台用户是否存在。
// 企微通过 userId 查询。
func (b *DatrixBridgeImpl) SearchUser(platform, unionID string) (bool, string, error) {
	var apiPath string
	switch platform {
	case common.PlatformWeCom:
		apiPath = fmt.Sprintf("/api/mx/api/v1/user/searchByWecomUserId?userId=%s", url.QueryEscape(unionID))
	case common.PlatformDingTalk:
		apiPath = fmt.Sprintf("/api/mx/api/v1/user/searchByDingtalkUnionId?unionId=%s", url.QueryEscape(unionID))
	default:
		return false, "", fmt.Errorf("不支持的平台: %s", platform)
	}

	resp, err := b.httpClient.Get(common.DatrixAssetURL + apiPath)
	if err != nil {
		return false, "", fmt.Errorf("查询DATRIX用户失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, "", fmt.Errorf("查询DATRIX用户失败: status=%d", resp.StatusCode)
	}

	var result struct {
		Code int `json:"code"`
		Data struct {
			UserName string `json:"userName"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, "", fmt.Errorf("解析DATRIX用户响应失败: %w", err)
	}

	if result.Code != 200 || result.Data.UserName == "" {
		return false, "", nil
	}
	return true, result.Data.UserName, nil
}

// ============================================================================
// Token 管理
// ============================================================================

// Login 免密登录获取 DATRIX Token。
func (b *DatrixBridgeImpl) Login(param *LoginParam) (string, string, error) {
	apiPath := "/api/mx/api/v1/user/freeLogin"
	body, err := json.Marshal(param)
	if err != nil {
		return "", "", fmt.Errorf("序列化免密登录参数失败: %w", err)
	}

	resp, err := b.httpClient.Post(
		common.DatrixAssetURL+apiPath,
		"application/json",
		strings.NewReader(string(body)),
	)
	if err != nil {
		return "", "", fmt.Errorf("免密登录请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("免密登录请求失败: status=%d", resp.StatusCode)
	}

	var tokenInfo common.TokenInfo
	if err := json.NewDecoder(resp.Body).Decode(&tokenInfo); err != nil {
		return "", "", fmt.Errorf("解析免密登录响应失败: %w", err)
	}
	if tokenInfo.Token == "" {
		return "", "", fmt.Errorf("免密登录返回空 Token")
	}

	return tokenInfo.Token, tokenInfo.UserID, nil
}

// CheckTokenValid 检查 Token 是否有效。
func (b *DatrixBridgeImpl) CheckTokenValid(token string) bool {
	apiPath := fmt.Sprintf("/api/mx/api/v1/user/checkToken?token=%s", url.QueryEscape(token))
	resp, err := b.httpClient.Get(common.DatrixAssetURL + apiPath)
	if err != nil {
		utils.Sugar.Warnf("[DatrixBridge] 检查Token失败: %v", err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		utils.Sugar.Warnf("[DatrixBridge] 检查Token失败: status=%d", resp.StatusCode)
		return false
	}

	var result struct {
		Code int `json:"code"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		utils.Sugar.Warnf("[DatrixBridge] 检查Token响应解析失败: %v", err)
		return false
	}
	return result.Code == 200
}

// GenerateFreePassword 生成免密登录密码。
// 使用配置的 AES Key 和 Signing Key 加密用户信息。
// 注意：此方法需要根据 DATRIX 平台的免密登录算法实际实现。
// 当前使用 AES-CBC 加密，实际算法以 DATRIX 平台规范为准。
func (b *DatrixBridgeImpl) GenerateFreePassword(userName string) string {
	data := fmt.Sprintf(`{"userName":"%s"}`, userName)

	if common.FreeLoginAESKey != "" {
		// 解码 AES Key
		key, err := base64.StdEncoding.DecodeString(common.FreeLoginAESKey)
		if err != nil {
			utils.Sugar.Warnf("[DatrixBridge] 解码 AES Key 失败: %v", err)
			return data
		}
		if len(key) < aes.BlockSize {
			utils.Sugar.Warnf("[DatrixBridge] AES Key 长度不足: %d", len(key))
			return data
		}
		// 使用 AES Key 前 16 字节作为 IV
		iv := key[:aes.BlockSize]
		encrypted, err := utils.AESEncrypt(key, iv, []byte(data))
		if err != nil {
			utils.Sugar.Warnf("[DatrixBridge] 免密密码加密失败: %v", err)
			return data
		}
		return encrypted
	}
	return data
}

// ============================================================================
// 智能体管理
// ============================================================================

// GetAssistantInfo 获取智能体详细信息。
func (b *DatrixBridgeImpl) GetAssistantInfo(token, assistantID string) (*AssistantInfo, error) {
	apiPath := fmt.Sprintf("/api/app/at/api/v1/assistant/%s?access-token=%s",
		assistantID, url.QueryEscape(token))

	resp, err := b.httpClient.Get(common.DatrixAssistantURL + apiPath)
	if err != nil {
		return nil, fmt.Errorf("获取智能体信息失败: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Code int `json:"code"`
		Data struct {
			ID         string   `json:"id"`
			Name       string   `json:"name"`
			Logo       string   `json:"logo"`
			KbOnly     bool     `json:"knowledge_base_only"`
			NeedUserKb bool     `json:"need_user_knowledge"`
			KbIDs      []string `json:"knowledge_base_ids"`
		} `json:"data"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	if result.Code != 200 {
		return nil, fmt.Errorf("获取智能体信息失败: code=%d", result.Code)
	}

	return &AssistantInfo{
		AssistantID:       result.Data.ID,
		AssistantName:     result.Data.Name,
		LogoURL:           result.Data.Logo,
		KnowledgeBaseIDs:  result.Data.KbIDs,
		KnowledgeBaseOnly: result.Data.KbOnly,
		NeedUserKnowledge: result.Data.NeedUserKb,
	}, nil
}

// ============================================================================
// 会话管理
// ============================================================================

// CreateAssistantSession 创建智能体对话会话。
func (b *DatrixBridgeImpl) CreateAssistantSession(token, assistantID, userID string) (string, error) {
	apiPath := fmt.Sprintf("/api/app/at/api/v1/assistant/session/%s?access-token=%s&assistantId=%s",
		userID, url.QueryEscape(token), assistantID)

	resp, err := b.httpClient.Get(common.DatrixAssistantURL + apiPath)
	if err != nil {
		return "", fmt.Errorf("创建会话失败: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Code int `json:"code"`
		Data struct {
			SessionID string `json:"sessionId"`
		} `json:"data"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	if result.Code != 200 {
		return "", fmt.Errorf("创建会话失败: code=%d", result.Code)
	}
	return result.Data.SessionID, nil
}

// GetHistory 获取对话历史上下文（最近 20 条）。
func (b *DatrixBridgeImpl) GetHistory(token, sessionID string) ([][]string, error) {
	apiPath := fmt.Sprintf("/api/app/at/api/v1/assistant/%s/history?access-token=%s",
		sessionID, url.QueryEscape(token))

	resp, err := b.httpClient.Get(common.DatrixAssistantURL + apiPath)
	if err != nil {
		return nil, fmt.Errorf("获取历史对话失败: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result struct {
		Code int        `json:"code"`
		Data [][]string `json:"data"`
	}
	json.Unmarshal(body, &result)

	// 如果解析失败，返回空历史（非关键路径）
	return result.Data, nil
}

// ============================================================================
// WebSocket 流式对话
// ============================================================================

// ChatWithAssistant 与 DATRIX 智能体进行 WebSocket 流式对话。
// 建立 WebSocket 连接，发送消息，异步读取流式响应。
func (b *DatrixBridgeImpl) ChatWithAssistant(token, sessionID, userID string, msg *ChatMessage) (<-chan ChatResponse, error) {
	baseURL, err := url.Parse(common.DatrixAssistantURL)
	if err != nil {
		return nil, fmt.Errorf("无效的 DATRIX Assistant URL: %w", err)
	}
	if baseURL.Scheme != "http" && baseURL.Scheme != "https" {
		return nil, fmt.Errorf("unsupported scheme: %s", baseURL.Scheme)
	}
	wsScheme := "ws"
	if baseURL.Scheme == "https" {
		wsScheme = "wss"
	}

	wsURL := url.URL{
		Scheme: wsScheme,
		Host:   baseURL.Host,
		Path:   fmt.Sprintf("/api/app/at/api/v1/assistant/%s/chat", sessionID),
	}
	query := wsURL.Query()
	query.Add("access-token", token)
	query.Add("user_id", userID)
	wsURL.RawQuery = query.Encode()

	// 建立 WebSocket 连接
	dialer := websocket.DefaultDialer
	dialer.HandshakeTimeout = 10 * time.Second

	conn, _, err := dialer.Dial(wsURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("WebSocket 连接失败: %w", err)
	}

	// 构建并发送 WebSocket 消息
	wsMsg := common.WebSocketMessage{
		SessionID:         sessionID,
		Question:          msg.Question,
		History:           msg.History,
		KnowledgeBases:    msg.KnowledgeBases,
		KnowledgeBaseOnly: msg.KnowledgeBaseOnly,
	}
	if err := conn.WriteJSON(wsMsg); err != nil {
		conn.Close()
		return nil, fmt.Errorf("发送 WebSocket 消息失败: %w", err)
	}

	// 异步读取响应
	respChan := make(chan ChatResponse, 100)
	go b.readWSResponses(conn, respChan)

	return respChan, nil
}

// readWSResponses 异步读取 WebSocket 响应流。
func (b *DatrixBridgeImpl) readWSResponses(conn *websocket.Conn, respChan chan ChatResponse) {
	defer close(respChan)
	defer conn.Close()

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				respChan <- ChatResponse{IsFinal: true}
				return
			}
			respChan <- ChatResponse{IsError: true, Error: err.Error()}
			return
		}

		var resp struct {
			Answer string `json:"answer"`
		}
		if err := json.Unmarshal(message, &resp); err != nil {
			utils.Sugar.Warnf("[DatrixBridge] 解析WS响应失败: %v", err)
			continue
		}

		respChan <- ChatResponse{
			Content: resp.Answer,
			IsFinal: false,
		}
	}
}
