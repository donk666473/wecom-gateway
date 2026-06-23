// Package pipeline 提供流水线管理器和运行时执行引擎。
// 参照 LangBot 框架的 PipelineManager + RuntimePipeline（pkg/pipeline/pipelinemgr.py）。
//
// 核心职责：
// 1. 管理流水线阶段的有序注册
// 2. 按顺序执行阶段链（责任链）
// 3. 支持阶段中断和错误处理
package pipeline

import (
	"github.com/wecom-gateway/internal/utils"
)

// ============================================================================
// Pipeline — 流水线运行时
// ============================================================================

// Pipeline 流水线运行时。
// 维护一个有序的阶段列表，按顺序执行各阶段。
type Pipeline struct {
	name   string  // 流水线名称（如 "default", "wecom_message"）
	stages []Stage // 有序的阶段列表
}

// NewPipeline 创建流水线实例
func NewPipeline(name string) *Pipeline {
	return &Pipeline{
		name:   name,
		stages: make([]Stage, 0),
	}
}

// AddStage 向流水线尾部添加一个阶段
func (p *Pipeline) AddStage(stage Stage) *Pipeline {
	p.stages = append(p.stages, stage)
	utils.Sugar.Debugf("[Pipeline:%s] 注册阶段: %s", p.name, stage.Name())
	return p
}

// AddStages 批量添加阶段
func (p *Pipeline) AddStages(stages ...Stage) *Pipeline {
	for _, stage := range stages {
		p.AddStage(stage)
	}
	return p
}

// InsertStage 在指定位置插入阶段
func (p *Pipeline) InsertStage(index int, stage Stage) *Pipeline {
	if index < 0 || index > len(p.stages) {
		p.stages = append(p.stages, stage)
		return p
	}
	p.stages = append(p.stages[:index], append([]Stage{stage}, p.stages[index:]...)...)
	utils.Sugar.Debugf("[Pipeline:%s] 插入阶段 %s 到位置 %d", p.name, stage.Name(), index)
	return p
}

// RemoveStage 移除指定名称的阶段
func (p *Pipeline) RemoveStage(name string) *Pipeline {
	for i, stage := range p.stages {
		if stage.Name() == name {
			p.stages = append(p.stages[:i], p.stages[i+1:]...)
			utils.Sugar.Debugf("[Pipeline:%s] 移除阶段: %s", p.name, name)
			break
		}
	}
	return p
}

// Execute 执行流水线（责任链模式）。
// 按顺序执行各阶段，遇 Interrupt 则中断并返回错误。
//
// 参照 LangBot 的 _execute_from_stage 方法。
func (p *Pipeline) Execute(ctx *Context) error {
	utils.Sugar.Debugf("[Pipeline:%s] 开始处理消息 [msg_id=%s, platform=%s]",
		p.name, ctx.Event.MessageID, ctx.Event.Platform)

	for _, stage := range p.stages {
		result := stage.Process(ctx)

		switch result.Type {
		case ResultContinue:
			// 继续下一个阶段
			utils.Sugar.Debugf("[Pipeline:%s] 阶段 %s 完成，继续", p.name, stage.Name())
		case ResultInterrupt:
			// 中断流水线
			if result.Error != nil {
				utils.Sugar.Warnf("[Pipeline:%s] 阶段 %s 中断: %v", p.name, stage.Name(), result.Error)
			} else {
				utils.Sugar.Debugf("[Pipeline:%s] 阶段 %s 中断（无错误）", p.name, stage.Name())
			}
			return result.Error
		}
	}

	utils.Sugar.Debugf("[Pipeline:%s] 消息处理完成 [msg_id=%s]", p.name, ctx.Event.MessageID)
	return nil
}

// Name 返回流水线名称
func (p *Pipeline) Name() string { return p.name }

// StageCount 返回阶段数量
func (p *Pipeline) StageCount() int { return len(p.stages) }
