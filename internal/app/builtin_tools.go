// builtin_tools.go connects Athena Core deterministic tools to the persisted governance decision log.
// builtin_tools.go 将 Athena Core 确定性工具接入持久化治理判定日志。
// It is the app-layer assembly point for live catalog publication and safe observability emission.
// 它是 live catalog 发布和安全可观测事件发射的 app 层装配入口。
// Changes here affect service startup and the tool governance trace visible to operators.
// 修改这里会影响服务启动和运维人员可见的工具治理 trace。
package app

import (
	"context"
	"fmt"

	"moss/internal/controlplane"
	"moss/internal/observability"
	"moss/internal/tools"
)

func governBuiltinToolDefinitions(definitions map[string]tools.Definition, catalog *tools.Catalog, manager *controlplane.Manager, observer *observability.Manager) error {
	if catalog == nil {
		return fmt.Errorf("tool catalog is not configured")
	}
	for _, name := range tools.BuiltinToolNames() {
		definition, ok := definitions[name]
		if !ok {
			return fmt.Errorf("built-in tool %q is not registered", name)
		}
		wrapped, err := tools.NewGovernedDefinition(definition, tools.GovernedToolOptions{
			Evaluate: func(ctx context.Context, request tools.GovernedToolRequest) (tools.GovernedToolDecision, error) {
				if manager == nil {
					return tools.GovernedToolDecision{Decision: "allow"}, nil
				}
				decision, err := manager.EvaluateToolGovernance(ctx, controlplane.ToolGovernanceDecisionRequest{
					ToolName:  request.ToolName,
					ToolScope: request.ToolScope,
					Operation: request.Operation,
					RiskLevel: request.RiskLevel,
					Metadata: map[string]any{
						"tool_call_id":  request.ToolCallID,
						"argument_keys": append([]string(nil), request.ArgumentKeys...),
						"tool_kind":     "builtin_deterministic",
					},
				})
				if err != nil {
					return tools.GovernedToolDecision{}, err
				}
				return tools.GovernedToolDecision{DecisionID: decision.DecisionID, Decision: decision.Decision, Reason: decision.Reason}, nil
			},
			Observe: func(ctx context.Context, event tools.GovernedToolEvent) {
				if observer == nil {
					return
				}
				attrs := map[string]string{
					"tool_call_id":      event.ToolCallID,
					"tool_name":         event.ToolName,
					"tool_scope":        event.ToolScope,
					"status":            event.Status,
					"decision_id":       event.DecisionID,
					"governance_result": event.GovernanceResult,
					"error_code":        event.ErrorCode,
				}
				observer.Trace(ctx, "builtin_tool_governance", attrs)
				observer.Observe("builtin_tool_duration_ms", float64(event.DurationMS), map[string]string{
					"tool_name": event.ToolName,
					"status":    event.Status,
				})
			},
		})
		if err != nil {
			return fmt.Errorf("govern built-in tool %q: %w", name, err)
		}
		definitions[name] = wrapped
		if err := catalog.Register(wrapped); err != nil {
			return fmt.Errorf("publish governed built-in tool %q: %w", name, err)
		}
	}
	return nil
}
