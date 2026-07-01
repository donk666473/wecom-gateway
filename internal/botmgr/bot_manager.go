// Package botmgr 提供机器人生命周期管理器。
// 参照设计文档 4.3 节适配器生命周期和 8 节模块依赖关系。
// 以及钉钉对接项目中的 BotManager 单例模式。
//
// 核心职责：
// 1. 启动时加载所有已注册的 IM 应用，创建对应适配器
// 2. 支持运行时动态增减应用（无需重启服务）
// 3. 管理适配器的启动/停止生命周期
// 4. 提供适配器查询接口（供 MessageProcessor 和 HTTP Handler 调用）
package botmgr

import (
	"fmt"
	"sync"

	"github.com/wecom-gateway/internal/adapter"
	"github.com/wecom-gateway/internal/model"
	"github.com/wecom-gateway/internal/utils"
)

// ============================================================================
// BotManager — 机器人生命周期管理器（单例）
// ============================================================================

// BotManager 机器人生命周期管理器。
// 管理所有 IM 平台适配器的创建、启动、停止和查询。
// 支持按平台维度启停，实现平台间运行隔离。
type BotManager struct {
	// apps adapter 实例映射（key: app_id）
	apps map[string]adapter.AbstractIMAdapter
	// disabledPlatforms 已手动停用的平台集合（key: platform）
	disabledPlatforms map[string]bool
	// pipelineFunc 流水线处理函数（由外部注入）
	pipelineFunc func(event *adapter.IMEvent)
	mu           sync.RWMutex
}

var (
	instance *BotManager
	once     sync.Once
)

// GetBotManager 获取 BotManager 单例
func GetBotManager() *BotManager {
	once.Do(func() {
		instance = &BotManager{
			apps:              make(map[string]adapter.AbstractIMAdapter),
			disabledPlatforms: make(map[string]bool),
		}
	})
	return instance
}

// SetPipelineFunc 设置流水线处理函数
func (m *BotManager) SetPipelineFunc(fn func(event *adapter.IMEvent)) {
	m.pipelineFunc = fn
}

// ============================================================================
// 生命周期管理
// ============================================================================

// StartAllBots 启动所有已注册的 IM 应用。
// 从数据库加载所有 status=1 的应用，逐一创建适配器并启动。
//
// 参照 LangBot 框架的 PlatformManager.run() 和钉钉项目的 StartAllRobots()。
func (m *BotManager) StartAllBots() error {
	utils.Sugar.Info("[BotManager] 开始加载所有IM应用...")

	apps, err := model.GetActiveApps()
	if err != nil {
		return fmt.Errorf("查询活跃应用失败: %w", err)
	}

	utils.Sugar.Infof("[BotManager] 发现 %d 个活跃应用", len(apps))

	for _, app := range apps {
		if err := m.StartApp(&app); err != nil {
			utils.Sugar.Errorf("[BotManager] 启动应用失败 [app_id=%s, platform=%s]: %v",
				app.AppID, app.Platform, err)
			continue
		}
	}

	utils.Sugar.Infof("[BotManager] 启动完成，共运行 %d 个应用", len(m.apps))
	return nil
}

// StartApp 启动单个应用。
// 根据平台类型创建对应的适配器，注册消息回调，启动适配器。
func (m *BotManager) StartApp(app *model.WeComApp) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查是否已启动
	if _, exists := m.apps[app.AppID]; exists {
		utils.Sugar.Warnf("[BotManager] 应用已运行 [app_id=%s]", app.AppID)
		return nil
	}

	// 通过注册表创建适配器（支持多平台扩展，无需修改 BotManager）
	adapterInstance, err := adapter.CreateAdapter(app)
	if err != nil {
		return fmt.Errorf("创建适配器失败: %w", err)
	}

	// 注册消息处理回调（Pipeline）
	if m.pipelineFunc != nil {
		adapterInstance.OnMessage(adapter.FriendMessage, func(event *adapter.IMEvent) {
			m.pipelineFunc(event)
		})
		adapterInstance.OnMessage(adapter.GroupMessage, func(event *adapter.IMEvent) {
			m.pipelineFunc(event)
		})
	}

	// 启动适配器
	if err := adapterInstance.Start(); err != nil {
		return fmt.Errorf("启动适配器失败: %w", err)
	}

	// 注册到管理器
	m.apps[app.AppID] = adapterInstance
	utils.Sugar.Infof("[BotManager] 应用已启动 [app_id=%s, platform=%s, name=%s]",
		app.AppID, app.Platform, app.AppName)

	return nil
}

// StopApp 停止单个应用。
func (m *BotManager) StopApp(appID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	adapterInstance, exists := m.apps[appID]
	if !exists {
		return fmt.Errorf("应用未运行: %s", appID)
	}

	if err := adapterInstance.Stop(); err != nil {
		utils.Sugar.Warnf("[BotManager] 停止应用失败 [app_id=%s]: %v", appID, err)
	}

	delete(m.apps, appID)
	utils.Sugar.Infof("[BotManager] 应用已停止 [app_id=%s]", appID)
	return nil
}

// RestartApp 重启单个应用（先停后启，原子操作）。
func (m *BotManager) RestartApp(app *model.WeComApp) error {
	m.mu.Lock()
	_, exists := m.apps[app.AppID]
	m.mu.Unlock()

	if exists {
		if err := m.StopApp(app.AppID); err != nil {
			utils.Sugar.Warnf("[BotManager] 停止旧应用失败 [app_id=%s]: %v", app.AppID, err)
		}
	}
	// 再启动
	return m.StartApp(app)
}

// StopAllBots 停止所有应用
func (m *BotManager) StopAllBots() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for appID, a := range m.apps {
		if err := a.Stop(); err != nil {
			utils.Sugar.Warnf("[BotManager] 停止应用失败 [app_id=%s]: %v", appID, err)
		}
	}
	m.apps = make(map[string]adapter.AbstractIMAdapter)
	m.disabledPlatforms = make(map[string]bool)
	utils.Sugar.Info("[BotManager] 所有应用已停止")
}

// ============================================================================
// 平台级生命周期管理（实现平台间隔离）
// ============================================================================

// StartPlatform 启动指定平台所有 status=1 的应用，并解除平台停用状态。
func (m *BotManager) StartPlatform(platform string) error {
	m.mu.Lock()
	delete(m.disabledPlatforms, platform)
	m.mu.Unlock()

	apps, err := model.GetAppsByPlatform(platform)
	if err != nil {
		return fmt.Errorf("查询平台活跃应用失败: %w", err)
	}

	utils.Sugar.Infof("[BotManager] 启动平台 %s，发现 %d 个活跃应用", platform, len(apps))
	for _, app := range apps {
		if err := m.StartApp(&app); err != nil {
			utils.Sugar.Errorf("[BotManager] 启动平台应用失败 [app_id=%s, platform=%s]: %v",
				app.AppID, platform, err)
		}
	}
	return nil
}

// StopPlatform 停止指定平台的所有运行中应用，并标记平台为停用。
// 其他平台不受影响。
func (m *BotManager) StopPlatform(platform string) {
	m.mu.Lock()
	m.disabledPlatforms[platform] = true
	m.mu.Unlock()

	apps := m.GetAdaptersByPlatform(platform)
	for appID, a := range apps {
		if err := a.Stop(); err != nil {
			utils.Sugar.Warnf("[BotManager] 停止平台应用失败 [app_id=%s, platform=%s]: %v", appID, platform, err)
		}
		if err := m.StopApp(appID); err != nil {
			utils.Sugar.Warnf("[BotManager] 从管理器移除应用失败 [app_id=%s]: %v", appID, err)
		}
	}
	utils.Sugar.Infof("[BotManager] 平台 %s 已停止，共 %d 个应用", platform, len(apps))
}

// IsPlatformRunning 判断指定平台是否处于运行状态（未被手动停用）。
func (m *BotManager) IsPlatformRunning(platform string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return !m.disabledPlatforms[platform]
}

// ============================================================================
// 查询接口
// ============================================================================

// GetAdapter 获取指定应用的适配器实例。
func (m *BotManager) GetAdapter(appID string) adapter.AbstractIMAdapter {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.apps[appID]
}

// GetAppCount 获取当前运行的应用数量
func (m *BotManager) GetAppCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.apps)
}

// GetAdaptersByPlatform 返回指定平台的所有运行中适配器。
func (m *BotManager) GetAdaptersByPlatform(platform string) map[string]adapter.AbstractIMAdapter {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]adapter.AbstractIMAdapter)
	for appID, a := range m.apps {
		if a.Platform() == platform {
			result[appID] = a
		}
	}
	return result
}

// GetAllAdapters 获取所有适配器
func (m *BotManager) GetAllAdapters() map[string]adapter.AbstractIMAdapter {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]adapter.AbstractIMAdapter)
	for k, v := range m.apps {
		result[k] = v
	}
	return result
}
