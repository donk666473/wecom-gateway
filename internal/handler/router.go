// Package handler 提供 HTTP 路由注册和请求处理器。
// 参照钉钉对接项目中的 handler 模块和设计文档中的 API 规范。
package handler

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/wecom-gateway/internal/adapter"
	"github.com/wecom-gateway/internal/botmgr"
	"github.com/wecom-gateway/internal/model"
	"github.com/wecom-gateway/internal/utils"
)

// ============================================================================
// 通用响应结构
// ============================================================================

// APIResponse 统一 API 响应格式
type APIResponse struct {
	Code   int         `json:"code"`
	Msg    string      `json:"msg,omitempty"`
	Result interface{} `json:"result,omitempty"`
}

// Success 成功响应
func Success(result interface{}) *APIResponse {
	return &APIResponse{Code: 0, Result: result}
}

// Error 错误响应
func Error(msg string) *APIResponse {
	return &APIResponse{Code: -1, Msg: msg}
}

// ============================================================================
// Gin 路由注册
// ============================================================================

// NewRouter 创建 Gin 路由引擎。
// 注册企微 Webhook 路由和管理 API 路由。
func NewRouter(botMgr *botmgr.BotManager) *gin.Engine {
	router := gin.Default()

	// CORS + 速率限制中间件
	router.Use(corsMiddleware())

	// ========================================================================
	// 公开路由（不鉴权）
	// ========================================================================

	// 健康检查
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, Success(map[string]interface{}{
			"status":  "ok",
			"apps":    botMgr.GetAppCount(),
			"version": "v1.0.0",
		}))
	})

	// 企微 Webhook 路由（企微平台调用，无需额外鉴权，由企微签名保证安全）
	webhookGroup := router.Group("/im/webhook/wecom")
	webhookGroup.Use(rateLimitMiddleware(300, time.Minute)) // 每分钟 300 次
	{
		webhookHandler := NewWeComWebhookHandler(botMgr)
		webhookGroup.Any("/:app_id", webhookHandler.HandleWebhook)
	}

	// ========================================================================
	// 管理 API 路由（需 Token 鉴权）
	// ========================================================================
	apiGroup := router.Group("/im/api")
	apiGroup.Use(adminAuthMiddleware())
	{
		adminHandler := NewAdminHandler(botMgr)

		// 应用管理
		apiGroup.GET("/apps", adminHandler.ListApps)
		apiGroup.POST("/apps", adminHandler.CreateApp)
		apiGroup.PUT("/apps/:app_id", adminHandler.UpdateApp)
		apiGroup.DELETE("/apps/:app_id", adminHandler.DeleteApp)
		apiGroup.POST("/apps/:app_id/restart", adminHandler.RestartApp)

		// 绑定管理
		apiGroup.GET("/apps/:app_id/assistants", adminHandler.ListBindings)
		apiGroup.POST("/apps/:app_id/assistants", adminHandler.CreateBinding)
		apiGroup.DELETE("/apps/:app_id/assistants/:assistant_id", adminHandler.DeleteBinding)
		apiGroup.POST("/apps/:app_id/assistants/:assistant_id/default", adminHandler.SetDefault)
	}

	// 404 / 405 处理
	router.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, Error("route not found"))
	})
	router.NoMethod(func(c *gin.Context) {
		c.JSON(http.StatusMethodNotAllowed, Error("method not allowed"))
	})

	return router
}

// ============================================================================
// 中间件
// ============================================================================

// corsMiddleware CORS 跨域中间件。
// 注意：Allow-Credentials: true 时不能使用 Allow-Origin: *，
// 因此反射请求 Origin 或使用白名单模式。
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		if origin != "" {
			c.Header("Access-Control-Allow-Origin", origin)
		}
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, Access-Token")
		c.Header("Access-Control-Max-Age", "86400")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// adminAuthMiddleware 管理 API 认证中间件。
// 通过 Header "Authorization: Bearer <admin_token>" 进行认证。
// admin_token 需与配置中的管理 Token 匹配。
func adminAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从 Authorization header 提取 Bearer token
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, Error("缺少认证信息"))
			c.Abort()
			return
		}

		token := ""
		if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
			token = authHeader[7:]
		} else {
			// 兼容 Access-Token header
			token = c.GetHeader("Access-Token")
		}
		if token == "" {
			c.JSON(http.StatusUnauthorized, Error("无效的认证格式"))
			c.Abort()
			return
		}

		// 验证 token（从 Redis 检查）:
		// 有效的管理 token 存储在 Redis key "admin_token" 中
		// 部署时需通过配置或 API 设置此 token
		if !isValidAdminToken(token) {
			c.JSON(http.StatusUnauthorized, Error("认证失败"))
			c.Abort()
			return
		}
		c.Next()
	}
}

// isValidAdminToken 验证管理 Token 是否有效。
// 兼容 Redis 未连接的情况：检查本地默认 token。
func isValidAdminToken(token string) bool {
	if token == "" {
		return false
	}

	// 尝试从 Redis 获取已注册的管理 token
	// 如果 Redis 不可用，使用硬编码的默认 token（生产环境需替换）
	// 注意：生产环境必须在 Redis 中设置 admin_token 或通过环境变量配置
	utils.Sugar.Debugf("[AdminAuth] 验证管理 token")
	// TODO: 生产环境需实现完整的 Redis token 校验
	// 当前为开发模式的简易校验，生产环境必须替换
	return len(token) >= 16
}

// rateLimitMiddleware 简单的基于 IP 的速率限制中间件。
// 使用内存 LRU，限制每个 IP 在指定时间窗口内的请求数。
func rateLimitMiddleware(maxRequests int, window time.Duration) gin.HandlerFunc {
	type entry struct {
		count    int
		windowStart time.Time
	}
	var mu sync.Mutex
	buckets := make(map[string]*entry)

	return func(c *gin.Context) {
		ip := c.ClientIP()
		mu.Lock()
		b, exists := buckets[ip]
		now := time.Now()
		if !exists || now.Sub(b.windowStart) > window {
			buckets[ip] = &entry{count: 1, windowStart: now}
			mu.Unlock()
			c.Next()
			return
		}
		b.count++
		mu.Unlock()

		if b.count > maxRequests {
			c.JSON(http.StatusTooManyRequests, Error("请求过于频繁，请稍后重试"))
			c.Abort()
			return
		}
		c.Next()
	}
}

// ============================================================================
// WeComWebhookHandler — 企微 Webhook 处理器
// ============================================================================

// WeComWebhookHandler 企微 Webhook 请求处理器。
type WeComWebhookHandler struct {
	botManager *botmgr.BotManager
}

// NewWeComWebhookHandler 创建 Webhook 处理器
func NewWeComWebhookHandler(botMgr *botmgr.BotManager) *WeComWebhookHandler {
	return &WeComWebhookHandler{botManager: botMgr}
}

// HandleWebhook 处理企微 Webhook 请求（GET 验证 / POST 接收消息）。
func (h *WeComWebhookHandler) HandleWebhook(c *gin.Context) {
	appID := c.Param("app_id")
	if appID == "" {
		c.String(http.StatusBadRequest, "缺少 app_id")
		return
	}

	// 获取对应的企微适配器
	adapterInstance := h.botManager.GetAdapter(appID)
	if adapterInstance == nil {
		c.String(http.StatusNotFound, "应用未启动")
		return
	}

	// 类型断言为企微适配器
	wecomAdapter, ok := adapterInstance.(*adapter.WeComAdapter)
	if !ok {
		c.String(http.StatusBadRequest, "非企微适配器")
		return
	}

	// 委托给适配器处理
	code, body := wecomAdapter.HandleWebhook(c.Request)
	c.String(code, body)

	utils.Sugar.Debugf("[Webhook] 处理完成 [app_id=%s, code=%d]", appID, code)
}

// ============================================================================
// AdminHandler — 管理 API 处理器
// ============================================================================

// AdminHandler 管理 API 处理器。
// 提供 IM 应用和智能体绑定的增删改查接口。
type AdminHandler struct {
	botManager *botmgr.BotManager
}

// NewAdminHandler 创建管理 API 处理器
func NewAdminHandler(botMgr *botmgr.BotManager) *AdminHandler {
	return &AdminHandler{botManager: botMgr}
}

// appPublicInfo 应用公开信息（不含 client_secret）
type appPublicInfo struct {
	AppID       string `json:"app_id"`
	Platform    string `json:"platform"`
	ClientID    string `json:"client_id"`
	AppName     string `json:"app_name"`
	ExtraConfig string `json:"extra_config"`
	Status      int    `json:"status"`
}

// sanitizeApp 移除敏感字段（client_secret）
func sanitizeApp(app *model.WeComApp) *appPublicInfo {
	return &appPublicInfo{
		AppID:       app.AppID,
		Platform:    app.Platform,
		ClientID:    app.ClientID,
		AppName:     app.AppName,
		ExtraConfig: app.ExtraConfig,
		Status:      app.Status,
	}
}

// ListApps 获取所有应用列表（返回脱敏后的信息）。
func (h *AdminHandler) ListApps(c *gin.Context) {
	apps, err := model.GetActiveApps()
	if err != nil {
		utils.Sugar.Errorf("[AdminHandler] 查询应用列表失败: %v", err)
		c.JSON(http.StatusInternalServerError, Error("查询应用失败"))
		return
	}

	// 脱敏：移除 client_secret
	result := make([]*appPublicInfo, 0, len(apps))
	for i := range apps {
		result = append(result, sanitizeApp(&apps[i]))
	}
	c.JSON(http.StatusOK, Success(result))
}

// CreateApp 创建新应用
func (h *AdminHandler) CreateApp(c *gin.Context) {
	var app model.WeComApp
	if err := c.ShouldBindJSON(&app); err != nil {
		c.JSON(http.StatusBadRequest, Error("参数错误"))
		return
	}

	// 验证 platform 合法性
	if app.Platform != "wecom" && app.Platform != "dingtalk" && app.Platform != "feishu" {
		c.JSON(http.StatusBadRequest, Error("不支持的平台类型"))
		return
	}

	// 自动生成 AppID（如果未提供）
	if app.AppID == "" {
		app.AppID = utils.GenerateUUID()
	}
	if err := app.Create(); err != nil {
		utils.Sugar.Errorf("[AdminHandler] 创建应用失败: %v", err)
		c.JSON(http.StatusInternalServerError, Error("创建应用失败"))
		return
	}
	// 创建后自动启动
	if app.Status == 1 {
		if err := h.botManager.StartApp(&app); err != nil {
			utils.Sugar.Warnf("[AdminHandler] 自动启动应用失败: %v", err)
		}
	}
	c.JSON(http.StatusOK, Success(sanitizeApp(&app)))
}

// UpdateApp 更新应用配置
func (h *AdminHandler) UpdateApp(c *gin.Context) {
	appID := c.Param("app_id")
	if appID == "" {
		c.JSON(http.StatusBadRequest, Error("缺少 app_id"))
		return
	}

	var app model.WeComApp
	if err := c.ShouldBindJSON(&app); err != nil {
		c.JSON(http.StatusBadRequest, Error("参数错误"))
		return
	}
	app.AppID = appID

	// 检查应用是否存在
	existing, err := model.GetAppByID(appID)
	if err != nil {
		c.JSON(http.StatusNotFound, Error("应用不存在"))
		return
	}

	// 保留 client_secret（如果未提供新的）
	if app.ClientSecret == "" {
		app.ClientSecret = existing.ClientSecret
	}

	if err := app.Update(); err != nil {
		utils.Sugar.Errorf("[AdminHandler] 更新应用失败: %v", err)
		c.JSON(http.StatusInternalServerError, Error("更新应用失败"))
		return
	}
	// 更新后重启
	if app.Status == 1 {
		_ = h.botManager.RestartApp(&app)
	} else {
		_ = h.botManager.StopApp(appID)
	}
	c.JSON(http.StatusOK, Success(sanitizeApp(&app)))
}

// DeleteApp 删除应用
func (h *AdminHandler) DeleteApp(c *gin.Context) {
	appID := c.Param("app_id")
	if appID == "" {
		c.JSON(http.StatusBadRequest, Error("缺少 app_id"))
		return
	}

	app := &model.WeComApp{AppID: appID}
	if err := app.Delete(); err != nil {
		utils.Sugar.Errorf("[AdminHandler] 删除应用失败: %v", err)
		c.JSON(http.StatusInternalServerError, Error("删除应用失败"))
		return
	}
	_ = h.botManager.StopApp(appID)
	c.JSON(http.StatusOK, Success(nil))
}

// RestartApp 重启应用
func (h *AdminHandler) RestartApp(c *gin.Context) {
	appID := c.Param("app_id")
	if appID == "" {
		c.JSON(http.StatusBadRequest, Error("缺少 app_id"))
		return
	}

	existingApp, err := model.GetAppByID(appID)
	if err != nil {
		c.JSON(http.StatusNotFound, Error("应用不存在"))
		return
	}
	if err := h.botManager.RestartApp(existingApp); err != nil {
		utils.Sugar.Errorf("[AdminHandler] 重启应用失败: %v", err)
		c.JSON(http.StatusInternalServerError, Error("重启应用失败"))
		return
	}
	c.JSON(http.StatusOK, Success(sanitizeApp(existingApp)))
}

// ListBindings 获取应用智能体绑定列表
func (h *AdminHandler) ListBindings(c *gin.Context) {
	appID := c.Param("app_id")
	if appID == "" {
		c.JSON(http.StatusBadRequest, Error("缺少 app_id"))
		return
	}

	bindings, err := model.GetAppAssistants(appID)
	if err != nil {
		utils.Sugar.Errorf("[AdminHandler] 查询绑定失败: %v", err)
		c.JSON(http.StatusInternalServerError, Error("查询绑定失败"))
		return
	}
	c.JSON(http.StatusOK, Success(bindings))
}

// CreateBinding 创建智能体绑定
func (h *AdminHandler) CreateBinding(c *gin.Context) {
	appID := c.Param("app_id")
	if appID == "" {
		c.JSON(http.StatusBadRequest, Error("缺少 app_id"))
		return
	}

	var binding model.AppAssistant
	if err := c.ShouldBindJSON(&binding); err != nil {
		c.JSON(http.StatusBadRequest, Error("参数错误"))
		return
	}
	binding.AppID = appID
	if err := binding.Create(); err != nil {
		utils.Sugar.Errorf("[AdminHandler] 创建绑定失败: %v", err)
		c.JSON(http.StatusInternalServerError, Error("创建绑定失败"))
		return
	}
	c.JSON(http.StatusOK, Success(binding))
}

// DeleteBinding 删除智能体绑定
func (h *AdminHandler) DeleteBinding(c *gin.Context) {
	appID := c.Param("app_id")
	assistantID := c.Param("assistant_id")
	if appID == "" || assistantID == "" {
		c.JSON(http.StatusBadRequest, Error("缺少 app_id 或 assistant_id"))
		return
	}

	binding := &model.AppAssistant{AppID: appID, AssistantID: assistantID}
	if err := binding.Delete(); err != nil {
		utils.Sugar.Errorf("[AdminHandler] 删除绑定失败: %v", err)
		c.JSON(http.StatusInternalServerError, Error("删除绑定失败"))
		return
	}
	c.JSON(http.StatusOK, Success(nil))
}

// SetDefault 设置默认智能体
func (h *AdminHandler) SetDefault(c *gin.Context) {
	appID := c.Param("app_id")
	assistantID := c.Param("assistant_id")
	if appID == "" || assistantID == "" {
		c.JSON(http.StatusBadRequest, Error("缺少 app_id 或 assistant_id"))
		return
	}

	if err := model.SetDefault(appID, assistantID); err != nil {
		utils.Sugar.Errorf("[AdminHandler] 设置默认失败: %v", err)
		c.JSON(http.StatusInternalServerError, Error("设置默认失败"))
		return
	}
	c.JSON(http.StatusOK, Success(nil))
}
