// Package adapter 提供适配器注册表 — 平台适配器工厂的自动注册与创建。
// 参照改进文档 2.1 节建议，用注册表模式替代 BotManager 中的 switch-case。
//
// 使用方式：
// 1. 各平台适配器在 init() 中调用 RegisterAdapter() 自注册
// 2. BotManager 通过 CreateAdapter() 创建实例，无需修改 switch-case
// 3. 新增平台只需添加一个适配器文件，无需改动任何现有代码
package adapter

import (
	"fmt"
	"sync"

	"github.com/wecom-gateway/internal/model"
)

// AdapterFactory 适配器工厂函数类型。
// 接收应用配置，返回 AbstractIMAdapter 实例。
type AdapterFactory func(app *model.WeComApp) (AbstractIMAdapter, error)

var (
	adapterFactories   = make(map[string]AdapterFactory)
	adapterFactoriesMu sync.RWMutex
)

// RegisterAdapter 注册平台适配器工厂。
// platform: 平台类型字符串（wecom / dingtalk / feishu）
// factory: 适配器工厂函数
// 通常在各适配器文件的 init() 中调用。
func RegisterAdapter(platform string, factory AdapterFactory) {
	adapterFactoriesMu.Lock()
	defer adapterFactoriesMu.Unlock()
	adapterFactories[platform] = factory
}

// CreateAdapter 根据平台类型和配置创建适配器实例。
// 由 BotManager 调用，取代原有的 switch-case。
func CreateAdapter(app *model.WeComApp) (AbstractIMAdapter, error) {
	adapterFactoriesMu.RLock()
	factory, ok := adapterFactories[app.Platform]
	adapterFactoriesMu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("不支持的平台类型: %s", app.Platform)
	}
	return factory(app)
}

// SupportedPlatforms 返回所有已注册的平台类型列表。
func SupportedPlatforms() []string {
	adapterFactoriesMu.RLock()
	defer adapterFactoriesMu.RUnlock()
	platforms := make([]string, 0, len(adapterFactories))
	for p := range adapterFactories {
		platforms = append(platforms, p)
	}
	return platforms
}
