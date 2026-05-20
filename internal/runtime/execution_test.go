package runtime

import "testing"

// TestResolveExecutionIntentSkipsOrdinaryChat verifies ordinary chat does not emit execution governance by default.
// TestResolveExecutionIntentSkipsOrdinaryChat 用于验证普通 chat 默认不会产出执行治理 contract。
func TestResolveExecutionIntentSkipsOrdinaryChat(t *testing.T) {
	intent := ResolveExecutionIntent(ExecutionGovernanceInput{
		RequestID:         "req-chat",
		SessionID:         "sess-chat",
		TaskType:          "chat",
		Scene:             "default",
		DesiredOutputMode: "text",
		InputPayload:      nil,
	})
	if intent != nil {
		t.Fatalf("expected nil intent, got %#v", intent)
	}
}

// TestResolveExecutionIntentBuildsGovernedIntent verifies explicit execution-governance payloads become canonical intents.
// TestResolveExecutionIntentBuildsGovernedIntent 用于验证显式执行治理载荷会生成标准执行意图。
func TestResolveExecutionIntentBuildsGovernedIntent(t *testing.T) {
	intent := ResolveExecutionIntent(ExecutionGovernanceInput{
		RequestID:         "req-exec",
		SessionID:         "sess-exec",
		TaskType:          "workflow_step_request",
		Scene:             "workflow",
		WorkflowRunID:     "run-1",
		StepID:            "step-1",
		DesiredOutputMode: "execution_governance",
		InputPayload: map[string]any{
			"execution_request": map[string]any{
				"command":         "python3 scripts/analyze.py",
				"timeout_seconds": 45,
			},
		},
	})
	if intent == nil {
		t.Fatalf("expected execution intent")
	}
	if intent.RequestID != "req-exec" || intent.SessionID != "sess-exec" {
		t.Fatalf("unexpected correlation ids = %#v", intent)
	}
	if intent.TaskType != "workflow_step_request" || intent.Scene != "workflow" {
		t.Fatalf("unexpected task correlation = %#v", intent)
	}
	if intent.ExecutionMode != ExecutionModeScriptExecution {
		t.Fatalf("ExecutionMode = %q, want script_execution", intent.ExecutionMode)
	}
	if intent.TimeoutSeconds != 45 {
		t.Fatalf("TimeoutSeconds = %d, want 45", intent.TimeoutSeconds)
	}
}

// TestResolveExecutionIntentDeniesHighRiskCommand verifies obviously dangerous commands are denied instead of confirmed.
// TestResolveExecutionIntentDeniesHighRiskCommand 用于验证明显危险的命令会被拒绝而不是进入确认。
func TestResolveExecutionIntentDeniesHighRiskCommand(t *testing.T) {
	intent := ResolveExecutionIntent(ExecutionGovernanceInput{
		RequestID:         "req-deny",
		SessionID:         "sess-deny",
		TaskType:          "workflow_step_request",
		Scene:             "workflow",
		DesiredOutputMode: "execution_governance",
		InputPayload: map[string]any{
			"execution_request": map[string]any{
				"command": "rm -rf /tmp/workspace",
			},
		},
	})
	if intent == nil {
		t.Fatalf("expected execution intent")
	}
	if intent.Allowed {
		t.Fatalf("expected denied intent, got %#v", intent)
	}
	if intent.DenyReason == "" || intent.RiskLevel != ExecutionRiskLevelHigh {
		t.Fatalf("unexpected denied intent = %#v", intent)
	}
}

// TestParseExecutionResultBuildsStructuredResult verifies execution_result payloads become canonical runtime results.
// TestParseExecutionResultBuildsStructuredResult 用于验证 execution_result 载荷会转成标准运行时结果。
func TestParseExecutionResultBuildsStructuredResult(t *testing.T) {
	result := ParseExecutionResult(map[string]any{
		"execution_result": map[string]any{
			"execution_id": "exec-1",
			"intent_id":    "intent-1",
			"status":       "completed",
			"exit_code":    0,
			"stdout":       "ok",
			"artifacts": []any{
				map[string]any{
					"name": "report",
					"path": "/tmp/report.json",
					"kind": "json",
				},
			},
		},
	})
	if result == nil {
		t.Fatalf("expected execution result")
	}
	if result.ExecutionID != "exec-1" || result.Status != "completed" || len(result.Artifacts) != 1 {
		t.Fatalf("unexpected execution result = %#v", result)
	}
}
