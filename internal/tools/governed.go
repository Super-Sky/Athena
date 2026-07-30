// governed.go wraps local invokable tools with a provider-neutral governance decision boundary.
// governed.go 为本地可调用工具增加与供应商无关的治理判定边界。
// It provides the Eino call-ID middleware and fail-closed local execution wrapper used by Core tools.
// 它提供 Eino 调用 ID 中间件和供 Core 工具使用的 fail-closed 本地执行包装器。
// Changes here affect governance enforcement and trace correlation for every wrapped local tool.
// 修改这里会影响所有已包装本地工具的治理执行和 trace 关联。
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	einoschema "github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
)

// GovernedToolRequest contains safe tool metadata used for a pre-execution decision.
// GovernedToolRequest 包含执行前判定使用的安全工具元数据。
type GovernedToolRequest struct {
	ToolCallID   string
	ToolName     string
	ToolScope    string
	Operation    string
	RiskLevel    string
	ArgumentKeys []string
}

// GovernedToolDecision is the minimum governance result needed by a local tool wrapper.
// GovernedToolDecision 是本地工具包装器消费的最小治理结果。
type GovernedToolDecision struct {
	DecisionID string
	Decision   string
	Reason     string
}

// GovernedToolEvaluator decides whether one local tool call may run.
// GovernedToolEvaluator 判定一条本地工具调用是否可以执行。
type GovernedToolEvaluator func(context.Context, GovernedToolRequest) (GovernedToolDecision, error)

// GovernedToolEvent records safe lifecycle fields for one governed local invocation.
// GovernedToolEvent 记录一条受治理本地调用的安全生命周期字段。
type GovernedToolEvent struct {
	ToolCallID       string
	ToolName         string
	ToolScope        string
	Status           string
	DurationMS       int64
	DecisionID       string
	GovernanceResult string
	ErrorCode        string
}

// GovernedToolObserver receives safe governance and execution observations.
// GovernedToolObserver 接收安全的治理与执行观测记录。
type GovernedToolObserver func(context.Context, GovernedToolEvent)

// GovernedToolOptions configures one local tool governance wrapper.
// GovernedToolOptions 配置一条本地工具治理包装器。
type GovernedToolOptions struct {
	Evaluate GovernedToolEvaluator
	Observe  GovernedToolObserver
}

// GovernedExecutionError provides a stable error code when governance prevents a local call.
// GovernedExecutionError 在治理阻止本地调用时提供稳定错误码。
type GovernedExecutionError struct {
	Code    string
	Message string
}

// Error returns the stable governed tool error text.
// Error 返回稳定的受治理工具错误文本。
func (e *GovernedExecutionError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("%s: %s", strings.TrimSpace(e.Code), strings.TrimSpace(e.Message))
}

type governedTool struct {
	definition Definition
	delegate   einotool.InvokableTool
	evaluate   GovernedToolEvaluator
	observe    GovernedToolObserver
}

type governedToolCallIDContextKey struct{}

// NewToolCallContextMiddleware exposes the Eino-generated tool call ID to a wrapped local tool.
// NewToolCallContextMiddleware 向被包装的本地工具暴露 Eino 生成的 tool call ID。
func NewToolCallContextMiddleware() compose.ToolMiddleware {
	return compose.ToolMiddleware{
		Invokable: func(next compose.InvokableToolEndpoint) compose.InvokableToolEndpoint {
			return func(ctx context.Context, input *compose.ToolInput) (*compose.ToolOutput, error) {
				if input == nil {
					return next(ctx, input)
				}
				ctx = context.WithValue(ctx, governedToolCallIDContextKey{}, strings.TrimSpace(input.CallID))
				return next(ctx, input)
			}
		},
	}
}

// NewGovernedDefinition wraps an invokable definition without changing its runtime metadata.
// NewGovernedDefinition 在不改变 runtime 元数据的前提下包装一条可调用定义。
func NewGovernedDefinition(definition Definition, options GovernedToolOptions) (Definition, error) {
	if strings.TrimSpace(definition.Name) == "" {
		return Definition{}, fmt.Errorf("governed tool name is required")
	}
	delegate, ok := definition.BaseTool.(einotool.InvokableTool)
	if !ok || delegate == nil {
		return Definition{}, fmt.Errorf("tool %q must be invokable before governance wrapping", definition.Name)
	}
	definition.BaseTool = &governedTool{definition: definition, delegate: delegate, evaluate: options.Evaluate, observe: options.Observe}
	return definition, nil
}

func (t *governedTool) Info(ctx context.Context) (*einoschema.ToolInfo, error) {
	if t == nil || t.delegate == nil {
		return nil, fmt.Errorf("governed tool is not configured")
	}
	return t.delegate.Info(ctx)
}

func (t *governedTool) InvokableRun(ctx context.Context, arguments string, opts ...einotool.Option) (string, error) {
	if t == nil || t.delegate == nil {
		return "", &GovernedExecutionError{Code: "tool_unconfigured", Message: "governed tool is not configured"}
	}
	startedAt := time.Now().UTC()
	callID, _ := ctx.Value(governedToolCallIDContextKey{}).(string)
	if strings.TrimSpace(callID) == "" {
		callID = "builtin_call_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	}
	request := GovernedToolRequest{
		ToolCallID:   callID,
		ToolName:     t.definition.Name,
		ToolScope:    t.definition.ToolScope,
		Operation:    "read",
		RiskLevel:    "low",
		ArgumentKeys: governedArgumentKeys(arguments),
	}
	decision := GovernedToolDecision{Decision: "allow"}
	if t.evaluate != nil {
		resolved, err := t.evaluate(ctx, request)
		if err != nil {
			governedObserve(t.observe, ctx, request, decision, "error", time.Since(startedAt), "governance_evaluation_failed")
			return "", &GovernedExecutionError{Code: "governance_evaluation_failed", Message: err.Error()}
		}
		decision = resolved
	}
	decision.Decision = strings.ToLower(strings.TrimSpace(decision.Decision))
	switch decision.Decision {
	case "", "allow":
		decision.Decision = "allow"
	case "deny":
		governedObserve(t.observe, ctx, request, decision, "denied", time.Since(startedAt), "governance_denied")
		return "", &GovernedExecutionError{Code: "governance_denied", Message: defaultGovernedMessage(decision.Reason, "local tool invocation denied")}
	default:
		governedObserve(t.observe, ctx, request, decision, "denied", time.Since(startedAt), "governance_decision_unsupported")
		return "", &GovernedExecutionError{Code: "governance_decision_unsupported", Message: "local deterministic tools only accept allow or deny decisions"}
	}

	output, err := t.delegate.InvokableRun(ctx, arguments, opts...)
	if err != nil {
		governedObserve(t.observe, ctx, request, decision, "error", time.Since(startedAt), "tool_execution_failed")
		return "", err
	}
	governedObserve(t.observe, ctx, request, decision, "ok", time.Since(startedAt), "")
	return output, nil
}

func governedObserve(observer GovernedToolObserver, ctx context.Context, request GovernedToolRequest, decision GovernedToolDecision, status string, duration time.Duration, errorCode string) {
	if observer == nil {
		return
	}
	observer(ctx, GovernedToolEvent{
		ToolCallID:       request.ToolCallID,
		ToolName:         request.ToolName,
		ToolScope:        request.ToolScope,
		Status:           status,
		DurationMS:       duration.Milliseconds(),
		DecisionID:       decision.DecisionID,
		GovernanceResult: decision.Decision,
		ErrorCode:        errorCode,
	})
}

func governedArgumentKeys(arguments string) []string {
	var payload map[string]any
	if err := json.Unmarshal([]byte(arguments), &payload); err != nil || payload == nil {
		return nil
	}
	keys := make([]string, 0, len(payload))
	for key := range payload {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func defaultGovernedMessage(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}
