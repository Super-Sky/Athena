// middleware.go implements tool lifecycle middleware used by the runtime execution path.
// middleware.go 实现 runtime 执行路径使用的 tool 生命周期中间件。
package tools

import (
	"context"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"moss/internal/observability"
)

// NewToolTraceMiddleware creates middleware that emits start and finish events for each tool call.
// NewToolTraceMiddleware 创建一份会为每次 tool 调用发出开始和结束事件的中间件。
func NewToolTraceMiddleware() adk.AgentMiddleware {
	return adk.AgentMiddleware{
		WrapToolCall: compose.ToolMiddleware{
			Invokable: toolTraceInvokableMiddleware,
		},
	}
}

func toolTraceInvokableMiddleware(next compose.InvokableToolEndpoint) compose.InvokableToolEndpoint {
	return func(ctx context.Context, input *compose.ToolInput) (*compose.ToolOutput, error) {
		startedAt := time.Now()
		observability.LogAction(observability.LogLevelDebug, observability.ActionLog{
			Module: "tools",
			Action: "tool_call",
			Step:   "start",
			Status: "running",
			Detail: map[string]any{
				"tool_call_id": input.CallID,
				"tool_name":    input.Name,
				"tool_args":    input.Arguments,
			},
			At: startedAt,
		})
		emitToolLifecycleEvent(ctx, "tool_call_started", input, startedAt, 0, "running", "")

		output, err := next(ctx, input)
		duration := time.Since(startedAt)
		if err != nil {
			observability.LogAction(observability.LogLevelError, observability.ActionLog{
				Module:     "tools",
				Action:     "tool_call",
				Step:       "finish",
				Status:     "error",
				Reason:     "tool_call_failed",
				ErrorCode:  "tool_call_error",
				DurationMS: duration.Milliseconds(),
				Detail: map[string]any{
					"tool_call_id": input.CallID,
					"tool_name":    input.Name,
					"tool_args":    input.Arguments,
					"error":        err.Error(),
				},
				At: time.Now(),
			})
			emitToolLifecycleEvent(ctx, "tool_call_finished", input, startedAt, duration.Milliseconds(), "error", err.Error())
			return nil, err
		}

		observability.LogAction(observability.LogLevelInfo, observability.ActionLog{
			Module:     "tools",
			Action:     "tool_call",
			Step:       "finish",
			Status:     "ok",
			Reason:     "tool_call_completed",
			DurationMS: duration.Milliseconds(),
			Detail: map[string]any{
				"tool_call_id": input.CallID,
				"tool_name":    input.Name,
			},
			At: time.Now(),
		})
		emitToolLifecycleEvent(ctx, "tool_call_finished", input, startedAt, duration.Milliseconds(), "ok", "")
		return output, nil
	}
}

func emitToolLifecycleEvent(ctx context.Context, eventType string, input *compose.ToolInput, startedAt time.Time, durationMS int64, status, errMsg string) {
	msg := schema.AssistantMessage("", nil)
	msg.Extra = map[string]any{
		"event_type":     eventType,
		"tool_call_id":   input.CallID,
		"tool_name":      input.Name,
		"tool_arguments": input.Arguments,
		"started_at":     startedAt.Format(time.RFC3339Nano),
		"duration_ms":    durationMS,
		"status":         status,
	}
	if errMsg != "" {
		msg.Extra["error"] = errMsg
	}

	_ = adk.SendEvent(ctx, &adk.AgentEvent{
		Output: &adk.AgentOutput{
			MessageOutput: &adk.MessageVariant{
				Message: msg,
			},
		},
	})
}
