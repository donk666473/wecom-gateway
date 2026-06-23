// Package stages 提供 Pipeline 各阶段的具体实现。
// 每个阶段对应消息处理流程中的一个独立步骤，
// 参照 LangBot 的 resprule/bansess/cntfilter/ratelimit/preproc/process 阶段模式。
package stages

import (
	"fmt"

	"github.com/wecom-gateway/internal/common"
	"github.com/wecom-gateway/internal/db"
	"github.com/wecom-gateway/internal/pipeline"
	"github.com/wecom-gateway/internal/utils"
)

// ============================================================================
// DedupStage — 消息去重阶段
// 参照设计文档 6.2 节消息去重策略。
// 使用 Redis SETNX 实现幂等，TTL 5 分钟。
// ============================================================================

// DedupStage 消息去重阶段。
type DedupStage struct{}

// NewDedupStage 创建去重阶段
func NewDedupStage() *DedupStage {
	return &DedupStage{}
}

func (s *DedupStage) Name() string { return "dedup" }

func (s *DedupStage) Process(ctx *pipeline.Context) *pipeline.Result {
	if ctx.Event.MessageID == "" {
		return pipeline.Continue() // 无 MessageID 则跳过去重
	}

	cacheKey := fmt.Sprintf("%s:%s", common.RedisPrefixDedup, ctx.Event.MessageID)
	if !db.RedisSetNX(cacheKey, "1", common.DedupTTL) {
		utils.Sugar.Infof("[%s] 重复消息已丢弃 [msg_id=%s]", s.Name(), ctx.Event.MessageID)
		return pipeline.Interrupt(nil) // 重复消息，中断但不报错
	}

	return pipeline.Continue()
}
