// Package handler 提供 HTTP 路由注册和请求处理器。
// 参照钉钉对接项目中的 handler 模块和设计文档中的 API 规范。
package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/wecom-gateway/internal/adapter"
	"github.com/wecom-gateway/internal/auth"
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
// 注册企微 Webhook 路由、管理 API 路由和扫码登录路由。
func NewRouter(botMgr *botmgr.BotManager, authSvc *auth.AuthService) *gin.Engine {
	router := gin.Default()

	// CORS 中间件
	router.Use(corsMiddleware())

	// 健康检查
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, Success(map[string]interface{}{
			"status":  "ok",
			"apps":    botMgr.GetAppCount(),
			"version": "v1.0.0",
		}))
	})

	// ========================================================================
	// 企微 Webhook 路由
	// ========================================================================
	webhookGroup := router.Group("/im/webhook/wecom")
	{
		webhookHandler := NewWeComWebhookHandler(botMgr)
		// 企微回调 URL: GET 验证, POST 接收消息
		webhookGroup.Any("/:app_id", webhookHandler.HandleWebhook)
	}

	// ========================================================================
	// 管理 API 路由
	// ========================================================================
	apiGroup := router.Group("/im/api")
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

	// ========================================================================
	// 扫码登录路由
	// ========================================================================
	if authSvc != nil {
		authHandler := NewAuthHandler(authSvc)
		authGroup := router.Group("/im/auth")
		{
			authGroup.GET("/:platform/url", authHandler.GenerateAuthURL)
			authGroup.GET("/callback", authHandler.HandleCallback)
			authGroup.GET("/status", authHandler.GetAuthStatus)
		}
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

// corsMiddleware CORS 跨域中间件
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
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

// ListApps 获取所有应用列表
func (h *AdminHandler) ListApps(c *gin.Context) {
	apps, err := model.GetActiveApps()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Error("查询应用失败"))
		return
	}
	c.JSON(http.StatusOK, Success(apps))
}

// CreateApp 创建新应用
func (h *AdminHandler) CreateApp(c *gin.Context) {
	var app model.WeComApp
	if err := c.ShouldBindJSON(&app); err != nil {
		c.JSON(http.StatusBadRequest, Error("参数错误"))
		return
	}
	if err := app.Create(); err != nil {
		c.JSON(http.StatusInternalServerError, Error("创建应用失败"))
		return
	}
	// 创建后自动启动
	if app.Status == 1 {
		_ = h.botManager.StartApp(&app)
	}
	c.JSON(http.StatusOK, Success(app))
}

// UpdateApp 更新应用配置
func (h *AdminHandler) UpdateApp(c *gin.Context) {
	appID := c.Param("app_id")
	var app model.WeComApp
	if err := c.ShouldBindJSON(&app); err != nil {
		c.JSON(http.StatusBadRequest, Error("参数错误"))
		return
	}
	app.AppID = appID
	if err := app.Update(); err != nil {
		c.JSON(http.StatusInternalServerError, Error("更新应用失败"))
		return
	}
	// 更新后重启
	if app.Status == 1 {
		_ = h.botManager.RestartApp(&app)
	} else {
		_ = h.botManager.StopApp(appID)
	}
	c.JSON(http.StatusOK, Success(app))
}

// DeleteApp 删除应用
func (h *AdminHandler) DeleteApp(c *gin.Context) {
	appID := c.Param("app_id")
	app := &model.WeComApp{AppID: appID}
	if err := app.Delete(); err != nil {
		c.JSON(http.StatusInternalServerError, Error("删除应用失败"))
		return
	}
	_ = h.botManager.StopApp(appID)
	c.JSON(http.StatusOK, Success(nil))
}

// RestartApp 重启应用
func (h *AdminHandler) RestartApp(c *gin.Context) {
	appID := c.Param("app_id")
	existingApp, err := model.GetAppByID(appID)
	if err != nil {
		c.JSON(http.StatusNotFound, Error("应用不存在"))
		return
	}
	if err := h.botManager.RestartApp(existingApp); err != nil {
		c.JSON(http.StatusInternalServerError, Error("重启应用失败"))
		return
	}
	c.JSON(http.StatusOK, Success(existingApp))
}

// ListBindings 获取应用智能体绑定列表
func (h *AdminHandler) ListBindings(c *gin.Context) {
	appID := c.Param("app_id")
	bindings, err := model.GetAppAssistants(appID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Error("查询绑定失败"))
		return
	}
	c.JSON(http.StatusOK, Success(bindings))
}

// CreateBinding 创建智能体绑定
func (h *AdminHandler) CreateBinding(c *gin.Context) {
	appID := c.Param("app_id")
	var binding model.AppAssistant
	if err := c.ShouldBindJSON(&binding); err != nil {
		c.JSON(http.StatusBadRequest, Error("参数错误"))
		return
	}
	binding.AppID = appID
	if err := binding.Create(); err != nil {
		c.JSON(http.StatusInternalServerError, Error("创建绑定失败"))
		return
	}
	c.JSON(http.StatusOK, Success(binding))
}

// DeleteBinding 删除智能体绑定
func (h *AdminHandler) DeleteBinding(c *gin.Context) {
	appID := c.Param("app_id")
	assistantID := c.Param("assistant_id")
	binding := &model.AppAssistant{AppID: appID, AssistantID: assistantID}
	if err := binding.Delete(); err != nil {
		c.JSON(http.StatusInternalServerError, Error("删除绑定失败"))
		return
	}
	c.JSON(http.StatusOK, Success(nil))
}

// SetDefault 设置默认智能体
func (h *AdminHandler) SetDefault(c *gin.Context) {
	appID := c.Param("app_id")
	assistantID := c.Param("assistant_id")
	if err := model.SetDefault(appID, assistantID); err != nil {
		c.JSON(http.StatusInternalServerError, Error("设置默认失败"))
		return
	}
	c.JSON(http.StatusOK, Success(nil))
}

// ============================================================================
// AuthHandler — 扫码登录处理器
// ============================================================================

// AuthHandler 扫码登录 API 处理器。
type AuthHandler struct {
	authService *auth.AuthService
}

// NewAuthHandler 创建扫码登录处理器
func NewAuthHandler(authSvc *auth.AuthService) *AuthHandler {
	return &AuthHandler{authService: authSvc}
}

// GenerateAuthURL 生成授权链接
// GET /im/auth/{platform}/url?app_id=xxx
func (h *AuthHandler) GenerateAuthURL(c *gin.Context) {
	platform := c.Param("platform")
	appID := c.Query("app_id")
	if appID == "" {
		c.JSON(http.StatusBadRequest, Error("缺少 app_id 参数"))
		return
	}

	result, err := h.authService.GenerateAuthURL(platform, appID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Error(err.Error()))
		return
	}
	c.JSON(http.StatusOK, Success(result))
}

// HandleCallback 处理 OAuth2 回调
// GET /im/auth/callback?code=xxx&state=yyy
func (h *AuthHandler) HandleCallback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")
	if code == "" || state == "" {
		c.JSON(http.StatusBadRequest, Error("缺少 code 或 state 参数"))
		return
	}

	if err := h.authService.HandleCallback(code, state); err != nil {
		c.JSON(http.StatusInternalServerError, Error(err.Error()))
		return
	}

	// 回调成功后返回 HTML 提示（企微要求）
	c.Data(http.StatusOK, "text/html; charset=utf-8",
		[]byte("<html><body><h3>授权成功，请返回页面</h3></body></html>"))
}

// GetAuthStatus 获取扫码状态（前端轮询）
// GET /im/auth/status?state=xxx
func (h *AuthHandler) GetAuthStatus(c *gin.Context) {
	state := c.Query("state")
	if state == "" {
		c.JSON(http.StatusBadRequest, Error("缺少 state 参数"))
		return
	}

	result, err := h.authService.GetAuthStatus(state)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Error(err.Error()))
		return
	}
	c.JSON(http.StatusOK, Success(result))
}
