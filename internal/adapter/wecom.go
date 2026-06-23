// Package adapter 提供企微适配器的完整实现。
// 参照 DATRIX 设计文档 4.3 节企微适配器实现、
// 架构设计篇 4.2 节平台差异对比，以及钉钉对接项目中的适配器模式。
//
// 企微消息接收：Webhook 回调（HTTP POST）+ URL 验证（GET）
// 企微消息回复：调用 sendTextMsg 主动推送
// 企微加解密：AES-256-CBC + SHA1 签名验证
package adapter

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/wecom-gateway/internal/common"
	"github.com/wecom-gateway/internal/db"
	"github.com/wecom-gateway/internal/model"
	"github.com/wecom-gateway/internal/utils"
)

// ============================================================================
// WeComAdapter — 企微适配器实现 AbstractIMAdapter 接口
// ============================================================================

// WeComAdapter 企微平台适配器。
// 负责企微消息接收（Webhook）、消息回复、用户信息获取和 OAuth2 授权。
type WeComAdapter struct {
	// 基础凭证
	corpID  string // 企业 ID
	secret  string // 应用 Secret
	appID   string // 应用唯一标识（UUID）

	// Webhook 配置
	token          string // 回调 Token
	encodingAESKey string // 消息加解密密钥
	agentID        int    // 应用 AgentID
	apiBaseURL     string // 企微 API 基地址

	// 扩展配置
	contactsSecret string // 通讯录 Secret（选填）
	oauth2CorpID   string // OAuth2 扫码登录 CorpID
	oauth2AgentID  int    // OAuth2 扫码登录 AgentID

	// 消息处理
	handlers map[EventType][]MessageHandler // 注册的消息处理器
	mu       sync.RWMutex                    // 并发保护

	// AES 密钥（从 encodingAESKey 解码得到）
	aesKey []byte
}

// NewWeComAdapter 创建企微适配器实例。
// 从数据库应用记录中读取配置，解析 ExtraConfig JSON 并初始化。
func NewWeComAdapter(app *model.WeComApp) (*WeComAdapter, error) {
	cfg, err := common.ParseWeComConfig(app.ExtraConfig)
	if err != nil {
		return nil, fmt.Errorf("解析企微配置失败: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	apiBaseURL := cfg.APIBaseURL
	if apiBaseURL == "" {
		apiBaseURL = common.WeComAPIBaseURL
	}

	// 解码 AES Key（Base64 → []byte）
	aesKey, err := decodeAESKey(cfg.EncodingAESKey)
	if err != nil {
		return nil, fmt.Errorf("解码 EncodingAESKey 失败: %w", err)
	}

	return &WeComAdapter{
		corpID:         app.ClientID,
		secret:         app.ClientSecret,
		appID:          app.AppID,
		token:          cfg.Token,
		encodingAESKey: cfg.EncodingAESKey,
		agentID:        cfg.AgentID,
		apiBaseURL:     apiBaseURL,
		contactsSecret: cfg.ContactsSecret,
		oauth2CorpID:   cfg.OAuth2CorpID,
		oauth2AgentID:  cfg.OAuth2AgentID,
		handlers:       make(map[EventType][]MessageHandler),
		aesKey:         aesKey,
	}, nil
}

// ============================================================================
// AbstractIMAdapter 接口实现 — 生命周期
// ============================================================================

// Platform 返回平台类型
func (w *WeComAdapter) Platform() string {
	return common.PlatformWeCom
}

// AppID 返回应用 ID
func (w *WeComAdapter) AppID() string {
	return w.appID
}

// Start 启动适配器。
// 企微使用 Webhook 模式，不需要主动建立连接，此方法为空操作。
func (w *WeComAdapter) Start() error {
	utils.Sugar.Infof("[WeComAdapter] 企微适配器启动 [app_id=%s, agent_id=%d]", w.appID, w.agentID)
	return nil
}

// Stop 停止适配器。
func (w *WeComAdapter) Stop() error {
	utils.Sugar.Infof("[WeComAdapter] 企微适配器停止 [app_id=%s]", w.appID)
	return nil
}

// ============================================================================
// AbstractIMAdapter 接口实现 — 事件注册
// ============================================================================

// OnMessage 注册消息处理回调。
// 同一事件类型可注册多个处理器，收到消息时并发调用。
func (w *WeComAdapter) OnMessage(eventType EventType, handler MessageHandler) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.handlers[eventType] = append(w.handlers[eventType], handler)
}

// dispatchMessage 将统一事件分发给所有注册的处理器。
func (w *WeComAdapter) dispatchMessage(event *IMEvent) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	eventType := FriendMessage
	if event.ConversationType == common.ConversationTypeGroup {
		eventType = GroupMessage
	}

	for _, handler := range w.handlers[eventType] {
		go handler(event) // 并发处理
	}
}

// ============================================================================
// AbstractIMAdapter 接口实现 — Webhook 处理（企微特有）
// ============================================================================

// HandleWebhook 处理企微 Webhook 回调请求（由 HTTP Handler 调用）。
// GET 请求：URL 验证（echostr）
// POST 请求：接收加密消息
func (w *WeComAdapter) HandleWebhook(r *http.Request) (code int, body string) {
	msgSignature := r.URL.Query().Get("msg_signature")
	timestamp := r.URL.Query().Get("timestamp")
	nonce := r.URL.Query().Get("nonce")

	if r.Method == "GET" {
		// URL 验证
		echostr := r.URL.Query().Get("echostr")
		return w.handleURLVerify(msgSignature, timestamp, nonce, echostr)
	}

	// POST — 接收消息
	return w.handleMessageReceive(r, msgSignature, timestamp, nonce)
}

// handleURLVerify 处理企微 URL 验证（GET 请求）。
// 验证签名有效性并解密 echostr。
func (w *WeComAdapter) handleURLVerify(msgSignature, timestamp, nonce, echostr string) (int, string) {
	// 1. 验证签名
	if !w.verifySignature(msgSignature, timestamp, nonce, echostr) {
		utils.Sugar.Warnf("[WeComAdapter] URL验证签名失败 [app_id=%s]", w.appID)
		return http.StatusBadRequest, "signature verification failed"
	}

	// 2. 解密 echostr
	plaintext, err := w.decryptMsg(echostr, w.corpID)
	if err != nil {
		utils.Sugar.Errorf("[WeComAdapter] URL验证解密echostr失败: %v", err)
		return http.StatusBadRequest, "decrypt echostr failed"
	}

	utils.Sugar.Infof("[WeComAdapter] URL验证成功 [app_id=%s]", w.appID)
	return http.StatusOK, string(plaintext)
}

// handleMessageReceive 处理企微消息接收（POST 请求）。
// 解密 XML 消息体，转换为统一 IMEvent 并分发给处理器。
func (w *WeComAdapter) handleMessageReceive(r *http.Request, msgSignature, timestamp, nonce string) (int, string) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return http.StatusBadRequest, "read body failed"
	}

	// 解析加密 XML
	var xmlMsg wecomEncryptedXML
	if err := xml.Unmarshal(body, &xmlMsg); err != nil {
		utils.Sugar.Errorf("[WeComAdapter] 解析XML失败: %v", err)
		return http.StatusBadRequest, "parse xml failed"
	}

	// 验证签名
	if !w.verifySignature(msgSignature, timestamp, nonce, xmlMsg.Encrypt) {
		utils.Sugar.Warnf("[WeComAdapter] 消息签名验证失败 [app_id=%s]", w.appID)
		return http.StatusBadRequest, "signature verification failed"
	}

	// 解密消息
	plaintext, err := w.decryptMsg(xmlMsg.Encrypt, w.corpID)
	if err != nil {
		utils.Sugar.Errorf("[WeComAdapter] 消息解密失败: %v", err)
		return http.StatusBadRequest, "decrypt message failed"
	}

	// 解析解密后的 XML
	var decryptedMsg wecomDecryptedXML
	if err := xml.Unmarshal(plaintext, &decryptedMsg); err != nil {
		utils.Sugar.Errorf("[WeComAdapter] 解析解密后XML失败: %v", err)
		return http.StatusBadRequest, "parse decrypted xml failed"
	}

	// 转换为统一 IMEvent
	event := w.convertToIMEvent(&decryptedMsg)
	utils.Sugar.Infof("[WeComAdapter] 收到消息 [app_id=%s, msg_id=%s, sender=%s, type=%s]",
		w.appID, event.MessageID, event.SenderID, event.MsgType)

	// 分发给处理器
	w.dispatchMessage(event)

	return http.StatusOK, "success"
}

// ============================================================================
// AbstractIMAdapter 接口实现 — 消息回复
// ============================================================================

// ReplyMessage 回复消息（企微：按字节分段发送文本消息）。
func (w *WeComAdapter) ReplyMessage(event *IMEvent, content string) error {
	accessToken, err := w.GetAccessToken()
	if err != nil {
		return fmt.Errorf("获取 access_token 失败: %w", err)
	}

	// 按字节数分段（企微单条消息限制 2048 字节）
	parts := splitByBytes(content, common.WecomChunkSize)
	for i, part := range parts {
		if err := w.sendTextMsg(accessToken, event.SenderID, part); err != nil {
			return fmt.Errorf("发送第%d段消息失败: %w", i+1, err)
		}
		if i < len(parts)-1 {
			time.Sleep(200 * time.Millisecond) // 避免频率限制
		}
	}
	return nil
}

// ReplyMessageChunk 流式回复片段。
// 企微不支持流式卡片更新，此方法仅记录日志，实际回复由 ReplyMessage 处理。
func (w *WeComAdapter) ReplyMessageChunk(event *IMEvent, chunk *ReplyChunk) error {
	utils.Sugar.Debugf("[WeComAdapter] 流式片段（企微暂不支持） [is_final=%v]", chunk.IsFinal)
	return nil
}

// ============================================================================
// AbstractIMAdapter 接口实现 — 用户信息
// ============================================================================

// GetUserInfo 从企微通讯录获取用户信息。
func (w *WeComAdapter) GetUserInfo(userID string) (*IMUserInfo, error) {
	accessToken, err := w.GetAccessToken()
	if err != nil {
		return nil, err
	}

	reqURL := fmt.Sprintf("%s/user/get?access_token=%s&userid=%s",
		w.apiBaseURL, accessToken, url.QueryEscape(userID))

	resp, err := http.Get(reqURL)
	if err != nil {
		return nil, fmt.Errorf("请求企微用户信息失败: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Errcode int    `json:"errcode"`
		Errmsg  string `json:"errmsg"`
		UserID  string `json:"userid"`
		Name    string `json:"name"`
		Mobile  string `json:"mobile"`
		OpenID  string `json:"open_userid"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析用户信息响应失败: %w", err)
	}
	if result.Errcode != 0 {
		return nil, fmt.Errorf("获取企微用户信息失败: errcode=%d errmsg=%s", result.Errcode, result.Errmsg)
	}

	return &IMUserInfo{
		UserID:   result.UserID,
		UnionID:  result.UserID, // 企微用 userId 作为唯一标识
		Nickname: result.Name,
		Mobile:   result.Mobile,
	}, nil
}

// ============================================================================
// AbstractIMAdapter 接口实现 — Token 管理
// ============================================================================

// GetAccessToken 获取企微 Access Token（带 Redis 缓存）。
// 缓存策略：提前 60 秒过期，确保 Token 始终有效。
func (w *WeComAdapter) GetAccessToken() (string, error) {
	cacheKey := fmt.Sprintf("%s:%s:%s", common.RedisPrefixToken, w.Platform(), w.corpID)

	// 1. 尝试从 Redis 获取
	if token, err := db.RedisGet(cacheKey); err == nil && token != "" {
		return token, nil
	}

	// 2. 调用企微 API 获取
	reqURL := fmt.Sprintf("%s/gettoken?corpid=%s&corpsecret=%s", w.apiBaseURL, w.corpID, w.secret)
	resp, err := http.Get(reqURL)
	if err != nil {
		return "", fmt.Errorf("请求企微 access_token 失败: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		Errcode     int    `json:"errcode"`
		Errmsg      string `json:"errmsg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("解析 access_token 响应失败: %w", err)
	}
	if result.Errcode != 0 {
		return "", fmt.Errorf("获取企微 access_token 失败: errcode=%d errmsg=%s", result.Errcode, result.Errmsg)
	}

	// 3. 缓存到 Redis（提前 60 秒过期）
	ttl := time.Duration(result.ExpiresIn-60) * time.Second
	if ttl <= 0 {
		ttl = 7200 * time.Second
	}
	_ = db.RedisSet(cacheKey, result.AccessToken, ttl)

	return result.AccessToken, nil
}

// ============================================================================
// AbstractIMAdapter 接口实现 — OAuth2 扫码登录
// ============================================================================

// GetOAuth2URL 生成企微授权登录链接。
// 参照设计文档 4.3 节企微 OAuth2 实现。
func (w *WeComAdapter) GetOAuth2URL(state string) (string, error) {
	if w.oauth2CorpID == "" {
		return "", fmt.Errorf("oauth2_corp_id 未配置")
	}
	// 企微 OAuth2 授权链接：https://developer.work.weixin.qq.com/document/path/91022
	authURL := fmt.Sprintf(
		"https://open.weixin.qq.com/connect/oauth2/authorize?appid=%s&redirect_uri=%s&response_type=code&scope=snsapi_privateinfo&state=%s&agentid=%d#wechat_redirect",
		w.oauth2CorpID,
		url.QueryEscape(common.AuthCallbackURL),
		state,
		w.oauth2AgentID,
	)
	return authURL, nil
}

// GetUserByCode 通过 OAuth2 code 换取企微用户信息。
func (w *WeComAdapter) GetUserByCode(code string) (*IMUserInfo, error) {
	// 1. 获取 access_token
	accessToken, err := w.GetAccessToken()
	if err != nil {
		return nil, fmt.Errorf("获取 access_token 失败: %w", err)
	}

	// 2. 通过 code 换取 userid
	userID, err := w.getUserIDByCode(accessToken, code)
	if err != nil {
		return nil, fmt.Errorf("换取 userid 失败: %w", err)
	}

	// 3. 通过 userid 获取用户详情
	return w.GetUserInfo(userID)
}

// getUserIDByCode 通过 OAuth2 code 获取企微 userid
func (w *WeComAdapter) getUserIDByCode(accessToken, code string) (string, error) {
	reqURL := fmt.Sprintf("%s/auth/getuserinfo?access_token=%s&code=%s",
		w.apiBaseURL, accessToken, code)
	resp, err := http.Get(reqURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		Errcode  int    `json:"errcode"`
		Errmsg   string `json:"errmsg"`
		UserID   string `json:"userid"`
		UserTicket string `json:"user_ticket"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if result.Errcode != 0 {
		return "", fmt.Errorf("企微 getuserinfo 错误: errcode=%d errmsg=%s", result.Errcode, result.Errmsg)
	}
	return result.UserID, nil
}

// ============================================================================
// 内部方法 — 消息发送
// ============================================================================

// sendTextMsg 发送企微文本消息（主动推送）。
func (w *WeComAdapter) sendTextMsg(accessToken, userID, content string) error {
	reqURL := fmt.Sprintf("%s/message/send?access_token=%s", w.apiBaseURL, accessToken)
	payload := map[string]interface{}{
		"touser":  userID,
		"msgtype": "text",
		"agentid": w.agentID,
		"text":    map[string]string{"content": content},
	}
	body, _ := json.Marshal(payload)

	resp, err := http.Post(reqURL, "application/json", strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result struct {
		Errcode int    `json:"errcode"`
		Errmsg  string `json:"errmsg"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	if result.Errcode != 0 {
		return fmt.Errorf("发送企微消息失败: errcode=%d errmsg=%s", result.Errcode, result.Errmsg)
	}
	return nil
}

// sendMarkdownMsg 发送企微 Markdown 消息。
func (w *WeComAdapter) sendMarkdownMsg(accessToken, userID, content string) error {
	reqURL := fmt.Sprintf("%s/message/send?access_token=%s", w.apiBaseURL, accessToken)
	payload := map[string]interface{}{
		"touser":  userID,
		"msgtype": "markdown",
		"agentid": w.agentID,
		"markdown": map[string]string{"content": content},
	}
	body, _ := json.Marshal(payload)

	resp, err := http.Post(reqURL, "application/json", strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result struct {
		Errcode int    `json:"errcode"`
		Errmsg  string `json:"errmsg"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	if result.Errcode != 0 {
		return fmt.Errorf("发送企微 Markdown 消息失败: errcode=%d errmsg=%s", result.Errcode, result.Errmsg)
	}
	return nil
}

// ============================================================================
// 内部方法 — 消息转换
// ============================================================================

// convertToIMEvent 将企微解密后的消息转换为统一 IMEvent 格式。
func (w *WeComAdapter) convertToIMEvent(msg *wecomDecryptedXML) *IMEvent {
	conversationType := common.ConversationTypeSingle

	return &IMEvent{
		Platform:         common.PlatformWeCom,
		MessageID:        msg.MsgID,
		SenderID:         msg.FromUserName,
		SenderNick:       "",
		ConversationID:   msg.FromUserName, // 单聊使用 userId 作为 conversationId
		ConversationType: conversationType,
		Content:          msg.Content,
		MsgType:          msg.MsgType,
		RawEvent:         msg,
		AppID:            w.appID,
		CorpID:           w.corpID,
	}
}
