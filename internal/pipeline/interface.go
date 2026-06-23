// Package pipeline 提供消息处理流水线的接口定义。
// 参照 LangBot 框架的 Pipeline 责任链模式（pkg/pipeline/）。
//
// 设计思想：
// 1. 责任链模式：消息依次经过各个处理阶段
// 2. 每个阶段处理特定职责（去重、身份映射、Token管理、会话管理、对话、回复）
// 3. 阶段可返回 CONTINUE（继续下一阶段）或 INTERRUPT（中断流水线）
// 4. 支持运行时动态组合阶段顺序
package pipeline

import (
	"github.com/wecom-gateway/internal/adapter"
)

// ============================================================================
// 流水线上下文 — 在阶段间传递的处理上下文
// ============================================================================

// Context 流水线处理上下文。
// 携带原始事件和各个阶段产生的中间结果，在阶段间传递。
type Context struct {
	// Event 原始 IM 事件
	Event *adapter.IMEvent

	// 身份映射结果
	DatrixUserName string // DATRIX 用户名
	IMUserInfo     *adapter.IMUserInfo // IM 平台用户信息

	// Token 管理结果
	DatrixToken string // DATRIX Token
	DatrixUserID string // DATRIX 用户 ID

	// 智能体信息
	AssistantID   string                 // 当前绑定的智能体 ID
	AssistantInfo interface{}            // 智能体详细信息

	// 会话管理结果
	SessionID string // DATRIX 会话 ID

	// 对话结果
	FullResponse string // 完整 AI 回复
	Error        error  // 处理过程中产生的错误

	// 元数据
	Metadata map[string]interface{} // 扩展字段
}

// NewContext 创建流水线上下文
func NewContext(event *adapter.IMEvent) *Context {
	return &Context{
		Event:    event,
		Metadata: make(map[string]interface{}),
	}
}

// ============================================================================
// 阶段结果
// ============================================================================

// ResultType 阶段处理结果类型
type ResultType int

const (
	// ResultContinue 继续执行下一个阶段
	ResultContinue ResultType = iota
	// ResultInterrupt 中断流水线（如用户未绑定、错误等）
	ResultInterrupt
)

// StageResult 阶段处理结果
type StageResult struct {
	// Type 结果类型
	Type ResultType
	// Error 错误信息（当 Type 为 Interrupt 时）
	Error error
}

// Continue 返回"继续"结果
func Continue() *StageResult {
	return &StageResult{Type: ResultContinue}
}

// Interrupt 返回"中断"结果
func Interrupt(err error) *StageResult {
	return &StageResult{Type: ResultInterrupt, Error: err}
}

// ============================================================================
// 阶段接口
// ============================================================================

// Stage 流水线阶段接口。
// 每个阶段实现此接口，处理流水线上下文并返回处理结果。
//
// 参照 LangBot 的 PipelineStage 接口（pkg/pipeline/stage.py）。
type Stage interface {
	// Name 返回阶段名称（用于日志和调试）
	Name() string

	// Process 处理流水线上下文。
	// ctx: 流水线上下文（携带事件和中间结果）
	// 返回阶段处理结果：Continue → 继续下一阶段，Interrupt → 中断流水线
	Process(ctx *Context) *StageResult
}

// StageFunc 函数式阶段（简化实现）
type StageFunc struct {
	name    string
	process func(ctx *Context) *StageResult
}

// NewStage 创建函数式阶段
func NewStage(name string, fn func(ctx *Context) *StageResult) *StageFunc {
	return &StageFunc{name: name, process: fn}
}

// Name 返回阶段名称
func (s *StageFunc) Name() string { return s.name }

// Process 处理上下文
func (s *StageFunc) Process(ctx *Context) *StageResult { return s.process(ctx) }
