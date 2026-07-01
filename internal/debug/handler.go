// Package debug 提供调试 HTTP 接口。
package debug

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/wecom-gateway/internal/botmgr"
	"github.com/wecom-gateway/internal/adapter"
)

// Handler 调试接口处理器。
type Handler struct {
	botMgr *botmgr.BotManager
}

// NewHandler 创建调试处理器。
func NewHandler() *Handler {
	return &Handler{botMgr: botmgr.GetBotManager()}
}

// RecentErrors 返回最近错误日志。
// GET /debug/errors?limit=50
func (h *Handler) RecentErrors(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	c.JSON(http.StatusOK, gin.H{
		"code":   0,
		"result": RecentErrors(limit),
	})
}

// Stats 返回平台/应用统计。
// GET /debug/stats
func (h *Handler) Stats(c *gin.Context) {
	platforms := adapter.SupportedPlatforms()
	platformStats := make(map[string]int, len(platforms))
	for _, p := range platforms {
		platformStats[p] = len(h.botMgr.GetAdaptersByPlatform(p))
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"result": map[string]interface{}{
			"apps":          h.botMgr.GetAppCount(),
			"platforms":     platformStats,
			"error_count":   Stats()["error_count"],
		},
	})
}
