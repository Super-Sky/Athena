// interfaces.go defines the replaceable runtime extension points used by the service backbone.
// interfaces.go 定义 runtime 主骨架使用的可替换扩展接口。
package runtime

import (
	"context"

	"github.com/cloudwego/eino/adk"
	"moss/internal/session"
)

// ContextAssembler builds model-facing messages from session state and runtime input.
// ContextAssembler 负责根据 session 状态和 runtime 输入组装面向模型的消息列表。
type ContextAssembler interface {
	Assemble(context.Context, *session.Session, Input) ([]adk.Message, error)
}

// CapabilityResolver turns request state into one execution spec or control action.
// CapabilityResolver 负责把请求状态解析成执行规格或控制动作。
type CapabilityResolver interface {
	Resolve(context.Context, RuntimeState, Input) (*ExecutionSpec, *Action, error)
}

// TurnExecutor prepares one runnable execution object for a single turn.
// TurnExecutor 负责为单轮执行准备可运行对象。
type TurnExecutor interface {
	Prepare(context.Context, RuntimeState, *ExecutionSpec, []adk.Message) (*PreparedExecution, error)
}

// TurnProcessor handles control actions or turn results after execution.
// TurnProcessor 负责在执行后处理控制动作或单轮结果。
type TurnProcessor interface {
	ProcessAction(context.Context, RuntimeState, *Action) (*TurnResult, error)
}

// LoopController evaluates whether the runtime should wait, continue, or timeout after a turn.
// LoopController 负责在单轮后评估 runtime 应等待、继续还是超时收口。
type LoopController interface {
	Evaluate(context.Context, RuntimeState, *TurnResult) (*WaitState, TimeoutOutcome)
}
