// Package wecom 提供企业微信适配器实现，并在 init 中向注册表自注册。
package wecom

import (
	"github.com/wecom-gateway/internal/adapter"
	"github.com/wecom-gateway/internal/model"
)

func init() {
	adapter.RegisterAdapter("wecom", func(app *model.WeComApp) (adapter.AbstractIMAdapter, error) {
		return NewWeComAdapter(app)
	})
}
