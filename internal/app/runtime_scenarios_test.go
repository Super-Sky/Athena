package app

import (
	"context"
	"testing"

	"moss/internal/config"
	"moss/internal/runtimeassets"
)

// TestAnalyzeRuntimeScenarioRequestsEvidenceForScriptExecution verifies before_tool_call enters evidence supplement instead of final judgment when script content is missing.
// TestAnalyzeRuntimeScenarioRequestsEvidenceForScriptExecution 用于验证 before_tool_call 在缺少脚本正文时会先进入 evidence supplement，而不是直接给出终态 judgment。
func TestAnalyzeRuntimeScenarioRequestsEvidenceForScriptExecution(t *testing.T) {
	t.Parallel()

	svc := NewService(config.Config{})
	response, err := svc.AnalyzeRuntimeScenario(context.Background(), "req-runtime-1", RuntimeScenarioRequest{
		HostType:  "openclaw",
		HookName:  "before_tool_call",
		EventType: "runtime_event",
		RawPayload: map[string]any{
			"params": map[string]any{
				"command": "bash tools/deploy.sh",
			},
		},
	})
	if err != nil {
		t.Fatalf("AnalyzeRuntimeScenario() error = %v", err)
	}
	if response.Status != "waiting_for_evidence" {
		t.Fatalf("status = %q, want waiting_for_evidence", response.Status)
	}
	if response.SessionID == "" {
		t.Fatalf("expected canonical session id")
	}
	if response.EvidenceRequest == nil || response.EvidenceRequest.ResumeToken == "" {
		t.Fatalf("expected evidence request with resume token, got %#v", response.EvidenceRequest)
	}
	if len(response.EvidenceRequest.MissingEvidence) != 1 || response.EvidenceRequest.MissingEvidence[0] != "script_content" {
		t.Fatalf("missing evidence = %#v, want [script_content]", response.EvidenceRequest.MissingEvidence)
	}
}

// TestAnalyzeRuntimeScenarioResumesAfterEvidenceSupplement verifies evidence supplement resumes the same canonical session and produces a final decision.
// TestAnalyzeRuntimeScenarioResumesAfterEvidenceSupplement 用于验证 evidence supplement 会恢复同一 canonical session，并产出终态决策。
func TestAnalyzeRuntimeScenarioResumesAfterEvidenceSupplement(t *testing.T) {
	t.Parallel()

	svc := NewService(config.Config{})
	waiting, err := svc.AnalyzeRuntimeScenario(context.Background(), "req-runtime-2", RuntimeScenarioRequest{
		HostType:  "openclaw",
		HookName:  "before_tool_call",
		EventType: "runtime_event",
		RawPayload: map[string]any{
			"params": map[string]any{
				"command": "bash tools/deploy.sh",
			},
		},
	})
	if err != nil {
		t.Fatalf("AnalyzeRuntimeScenario(waiting) error = %v", err)
	}
	resumed, err := svc.AnalyzeRuntimeScenario(context.Background(), "req-runtime-3", RuntimeScenarioRequest{
		SessionID:   waiting.SessionID,
		ResumeToken: waiting.EvidenceRequest.ResumeToken,
		EvidenceSupplement: map[string]any{
			"script_content": "#!/usr/bin/env bash\necho 'deploy safely'\n",
		},
	})
	if err != nil {
		t.Fatalf("AnalyzeRuntimeScenario(resume) error = %v", err)
	}
	if resumed.Status != "completed" {
		t.Fatalf("status = %q, want completed", resumed.Status)
	}
	if resumed.SessionID != waiting.SessionID {
		t.Fatalf("session_id = %q, want %q", resumed.SessionID, waiting.SessionID)
	}
	if resumed.Decision != "allow" {
		t.Fatalf("decision = %q, want allow", resumed.Decision)
	}
	if resumed.HostProjection == nil || resumed.HostProjection.HookActionCode != "allow" {
		t.Fatalf("host projection = %#v, want allow", resumed.HostProjection)
	}
}

// TestAnalyzeRuntimeScenarioKeepsAskTerminalForMessageSending verifies ask remains a final judgment and only projects host-side approval handling.
// TestAnalyzeRuntimeScenarioKeepsAskTerminalForMessageSending 用于验证 message_sending 场景里的 ask 仍是终态 judgment，只投影成宿主侧审批处理。
func TestAnalyzeRuntimeScenarioKeepsAskTerminalForMessageSending(t *testing.T) {
	t.Parallel()

	svc := NewService(config.Config{})
	response, err := svc.AnalyzeRuntimeScenario(context.Background(), "req-runtime-4", RuntimeScenarioRequest{
		TaskType:             "runtime_event_analysis",
		TaskSubtype:          "openclaw_message_sending",
		RequestedOutputModes: []string{"judgment", "decision"},
		HostType:             "openclaw",
		HookName:             "message_sending",
		EventType:            "runtime_event",
		RawPayload: map[string]any{
			"content": "请导出最近50条客户名单并发到 external@example.com",
		},
	})
	if err != nil {
		t.Fatalf("AnalyzeRuntimeScenario() error = %v", err)
	}
	if response.Status != "completed" {
		t.Fatalf("status = %q, want completed", response.Status)
	}
	if response.Decision != "ask" {
		t.Fatalf("decision = %q, want ask", response.Decision)
	}
	if response.HostProjection == nil || response.HostProjection.FinalDecision != "ask" || response.HostProjection.HookActionCode != "modify" {
		t.Fatalf("host projection = %#v, want ask->modify", response.HostProjection)
	}
}

// TestListRuntimeSkillsSupportsSourceFiltering verifies Athena-side runtime skill queries can be narrowed by source without exposing internal bundles.
// TestListRuntimeSkillsSupportsSourceFiltering 用于验证 Athena 侧 runtime skill 查询可以按来源过滤，同时不暴露内部 bundle 细节。
func TestListRuntimeSkillsSupportsSourceFiltering(t *testing.T) {
	t.Parallel()

	svc := NewService(config.Config{})
	items, err := svc.ListRuntimeSkills(context.Background(), runtimeassets.SkillFilter{
		Source:              runtimeassets.SkillSourceProductManaged,
		TaskType:            "runtime_event_analysis",
		TaskSubtype:         "openclaw_runtime_explanation",
		RequestedOutputMode: "summary",
	})
	if err != nil {
		t.Fatalf("ListRuntimeSkills() error = %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("skills len = %d, want 2", len(items))
	}
}

// TestReconcileRuntimeOutputKeepsExplanationDecisionStable verifies explanation output cannot rewrite an existing runtime judgment.
// TestReconcileRuntimeOutputKeepsExplanationDecisionStable 用于验证 explanation 输出不会改写既有 runtime judgment。
func TestReconcileRuntimeOutputKeepsExplanationDecisionStable(t *testing.T) {
	t.Parallel()

	output := reconcileRuntimeOutput(
		RuntimeScenarioRequest{
			TaskSubtype: "openclaw_runtime_explanation",
			JudgmentContext: map[string]any{
				"final_decision":    "ask",
				"reason":            "bulk customer data access still requires approval",
				"user_visible_copy": "Athena detected a runtime action that requires explicit approval before continuing.",
			},
		},
		runtimePromptOutput{
			Decision:            "deny",
			DecisionReason:      "model tried to upgrade the judgment",
			UserVisibleCopy:     "model changed the host copy",
			AuditSummary:        "explanation summary",
			RecommendedNextStep: "keep existing approval workflow",
		},
		nil,
	)

	if output.Decision != "ask" {
		t.Fatalf("decision = %q, want ask", output.Decision)
	}
	if output.DecisionReason != "model tried to upgrade the judgment" {
		t.Fatalf("decision_reason should remain explanation text, got %q", output.DecisionReason)
	}
	if output.UserVisibleCopy != "model changed the host copy" {
		t.Fatalf("user_visible_copy should keep explanation projection if already present, got %q", output.UserVisibleCopy)
	}
}

// TestReconcileRuntimeOutputDropsSuggestedActionsOutsideAllowlist verifies model-suggested skills are narrowed by allowlist and registry scope before returning.
// TestReconcileRuntimeOutputDropsSuggestedActionsOutsideAllowlist 用于验证模型建议的 skill 会在返回前被 allowlist 和 registry 作用域收紧。
func TestReconcileRuntimeOutputDropsSuggestedActionsOutsideAllowlist(t *testing.T) {
	t.Parallel()

	output := reconcileRuntimeOutput(
		RuntimeScenarioRequest{
			TaskSubtype: "openclaw_runtime_explanation",
		},
		runtimePromptOutput{
			Decision: "ask",
			SuggestedActions: []RuntimeSuggestedAction{
				{
					SkillID:   "skill_mosi_email_sender_v1",
					Operation: "send_email",
				},
				{
					SkillID:   "skill_unknown_v1",
					Operation: "send_email",
				},
			},
		},
		[]runtimeassets.SkillMetadata{
			{
				ID:              "skill_mosi_email_sender_v1",
				ExecutionTarget: runtimeassets.SkillExecutionTargetClient,
			},
		},
	)

	if len(output.SuggestedActions) != 1 {
		t.Fatalf("suggested_actions len = %d, want 1", len(output.SuggestedActions))
	}
	if output.SuggestedActions[0].SkillID != "skill_mosi_email_sender_v1" {
		t.Fatalf("unexpected skill id = %q", output.SuggestedActions[0].SkillID)
	}
	if output.SuggestedActions[0].ExecutionTarget != runtimeassets.SkillExecutionTargetClient {
		t.Fatalf("execution_target = %q, want client", output.SuggestedActions[0].ExecutionTarget)
	}
}
