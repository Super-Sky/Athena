package server

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	appcore "moss/internal/app"
	"moss/internal/automation"
	"moss/internal/config"
	"moss/internal/contextassets"
	"moss/internal/controlplane"
	platformautomation "moss/internal/extensions/platform/automation"
	platformtools "moss/internal/extensions/platform/tools"
	"moss/internal/model"
	modelparams "moss/internal/model/parameters"
	"moss/internal/runtime"
	"moss/internal/session"
	"moss/internal/workflow"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	hertzapp "github.com/cloudwego/hertz/pkg/app"
)

func TestParseStructuredChatResultRepairsFencedJSON(t *testing.T) {
	result, repaired, err := parseStructuredChatResult("```json\n{\"answer\":\"ok\"}\n```", "basic")
	if err != nil {
		t.Fatalf("parseStructuredChatResult() error = %v", err)
	}
	if !repaired {
		t.Fatalf("expected repair flag")
	}
	if result.MainAnswer != "ok" || result.Answer != "ok" {
		t.Fatalf("answer fields = %#v, want ok", result)
	}
}

func TestCollectRespondOutputReturnsInterruptedError(t *testing.T) {
	runner := adk.NewRunner(context.Background(), adk.RunnerConfig{
		Agent: interruptedRespondAgent{},
	})
	_, _, err := collectRespondOutput(context.Background(), runner, []adk.Message{schema.UserMessage("pause")})
	if err == nil || !strings.Contains(err.Error(), "agent execution interrupted") {
		t.Fatalf("collectRespondOutput() error = %v, want interrupted error", err)
	}
}

func TestParseStructuredChatResultRepairsExplanatoryText(t *testing.T) {
	result, repaired, err := parseStructuredChatResult("Here is the result:\n{\"answer\":\"ok\",\"reason\":\"clear\"}", "basic")
	if err != nil {
		t.Fatalf("parseStructuredChatResult() error = %v", err)
	}
	if !repaired {
		t.Fatalf("expected repair flag")
	}
	if result.MainAnswer != "ok" || result.Answer != "ok" || result.Reason != "clear" {
		t.Fatalf("unexpected result = %#v", result)
	}
}

func TestParseStructuredChatResultRejectsInvalidJSON(t *testing.T) {
	if _, _, err := parseStructuredChatResult("{answer:", "basic"); err == nil {
		t.Fatalf("expected invalid JSON error")
	}
}

func TestParseStructuredChatResultRejectsMissingRequiredField(t *testing.T) {
	if _, _, err := parseStructuredChatResult(`{"reason":"missing answer"}`, "basic"); err == nil || !strings.Contains(err.Error(), "main_answer is required") {
		t.Fatalf("expected missing main_answer validation error, got %v", err)
	}
}

func TestParseStructuredChatResultUnwrapsNestedJSONStringAnswer(t *testing.T) {
	raw := `{"main_answer":"{\"main_answer\":\"clean answer\",\"structured_result\":{\"context_usage\":{\"identity\":true}},\"next_questions\":[\"q1\"]}","answer":"{\"main_answer\":\"clean answer\",\"structured_result\":{\"context_usage\":{\"identity\":true}},\"next_questions\":[\"q1\"]}"}`
	result, repaired, err := parseStructuredChatResult(raw, "basic")
	if err != nil {
		t.Fatalf("parseStructuredChatResult() error = %v", err)
	}
	if !repaired {
		t.Fatalf("expected repaired flag because nested object extraction path was used")
	}
	if result.MainAnswer != "clean answer" {
		t.Fatalf("main_answer = %q, want clean answer", result.MainAnswer)
	}
	usage, ok := result.StructuredResult["context_usage"].(map[string]any)
	if !ok || usage["identity"] != true {
		t.Fatalf("structured_result = %#v, want nested context_usage", result.StructuredResult)
	}
	if len(result.NextQuestions) != 1 || result.NextQuestions[0] != "q1" {
		t.Fatalf("next_questions = %#v, want [q1]", result.NextQuestions)
	}
	if result.Answer != "clean answer" {
		t.Fatalf("answer = %q, want clean answer", result.Answer)
	}
}

type interruptedRespondAgent struct{}

func (interruptedRespondAgent) Name(context.Context) string { return "interrupted_respond_agent" }

func (interruptedRespondAgent) Description(context.Context) string { return "interrupted test agent" }

func (interruptedRespondAgent) Run(context.Context, *adk.AgentInput, ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent] {
	iter, generator := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	go func() {
		defer generator.Close()
		generator.Send(&adk.AgentEvent{
			Action: &adk.AgentAction{
				Interrupted: &adk.InterruptInfo{},
			},
		})
	}()
	return iter
}

func TestResolveStructuredResultUsesPartialFallbackWhenConfigured(t *testing.T) {
	prepared := &runtime.PreparedExecution{
		StructuredOutput: &runtime.StructuredOutputContract{
			ContractID: "structured-output.v1",
		},
	}
	result, report, failureErr, detail := resolveStructuredResult(context.Background(), prepared, "结论: 需要人工复核", ChatRespondRequest{
		StrictSchemaValidation: true,
		SchemaFailureAction:    "partial",
		SchemaRepairMode:       "basic",
	}, "req-partial", "sess-partial", false, "", nil)
	if failureErr == nil {
		t.Fatalf("expected partial failure marker")
	}
	if result == nil || strings.TrimSpace(result.MainAnswer) == "" {
		t.Fatalf("expected partial result, got %#v", result)
	}
	if report.Valid {
		t.Fatalf("expected invalid schema report for partial fallback")
	}
	if !report.RegexFallbackUsed {
		t.Fatalf("expected regex fallback to be used")
	}
	if detail["partial"] != true {
		t.Fatalf("expected partial detail, got %#v", detail)
	}
}

func TestResolveStructuredResultSkipsFormattingRetryAfterToolSideEffects(t *testing.T) {
	prepared := &runtime.PreparedExecution{
		StructuredOutput: &runtime.StructuredOutputContract{
			ContractID: "structured-output.v1",
		},
	}
	result, report, failureErr, detail := resolveStructuredResult(context.Background(), prepared, "not valid json", ChatRespondRequest{
		StrictSchemaValidation: true,
		SchemaRetryCount:       3,
		SchemaRepairMode:       "off",
		SchemaFailureAction:    "error",
	}, "req-tool", "sess-tool", true, "", nil)
	if report.RetriesUsed != 0 {
		t.Fatalf("retries_used = %d, want 0", report.RetriesUsed)
	}
	if report.FailureStage != "tool_side_effect_guard" {
		t.Fatalf("failure_stage = %q, want tool_side_effect_guard", report.FailureStage)
	}
	if result != nil {
		t.Fatalf("expected no successful result under strict error mode, got %#v", result)
	}
	if failureErr == nil {
		t.Fatalf("expected terminal schema failure")
	}
	if detail["tool_side_effects"] != true {
		t.Fatalf("expected tool_side_effects detail, got %#v", detail)
	}
	if detail["fallback_source"] != "regex" {
		t.Fatalf("fallback_source = %#v, want regex attempt to remain observable", detail["fallback_source"])
	}
}

func TestResolveStructuredResultRejectsRegexFallbackForStrictErrorMode(t *testing.T) {
	prepared := &runtime.PreparedExecution{
		StructuredOutput: &runtime.StructuredOutputContract{
			ContractID: "structured-output.v1",
		},
	}
	result, report, failureErr, detail := resolveStructuredResult(context.Background(), prepared, "结论: 需要人工复核", ChatRespondRequest{
		StrictSchemaValidation: true,
		SchemaFailureAction:    "error",
		SchemaRepairMode:       "basic",
	}, "req-strict", "sess-strict", false, "", nil)
	if result != nil {
		t.Fatalf("expected no successful result, got %#v", result)
	}
	if failureErr == nil {
		t.Fatalf("expected schema failure under strict error mode")
	}
	if !report.RegexFallbackUsed {
		t.Fatalf("expected regex fallback attempt to be recorded")
	}
	if report.Valid {
		t.Fatalf("expected invalid schema report")
	}
	if detail["fallback_source"] != "regex" {
		t.Fatalf("fallback_source = %#v, want regex", detail["fallback_source"])
	}
}

func TestResolveStructuredResultKeepsRegexObservabilityWhenRepairModeOff(t *testing.T) {
	prepared := &runtime.PreparedExecution{
		StructuredOutput: &runtime.StructuredOutputContract{
			ContractID: "structured-output.v1",
		},
	}
	result, report, failureErr, detail := resolveStructuredResult(context.Background(), prepared, "结论: 需要人工复核", ChatRespondRequest{
		StrictSchemaValidation: true,
		SchemaFailureAction:    "error",
		SchemaRepairMode:       "off",
	}, "req-off", "sess-off", false, "", nil)
	if result != nil {
		t.Fatalf("expected no successful result, got %#v", result)
	}
	if failureErr == nil {
		t.Fatalf("expected schema failure under strict error mode")
	}
	if report.RepairAttempted {
		t.Fatalf("expected repair_attempted=false when repair mode is off")
	}
	if !report.RegexFallbackUsed {
		t.Fatalf("expected regex fallback observability to remain true")
	}
	if detail["fallback_source"] != "regex" {
		t.Fatalf("fallback_source = %#v, want regex", detail["fallback_source"])
	}
}

func TestValidateStructuredChatResultRejectsTooManyNextQuestions(t *testing.T) {
	if err := validateStructuredChatResult(structuredChatResult{
		MainAnswer:    "ok",
		NextQuestions: []string{"q1", "q2", "q3", "q4"},
	}); err == nil || !strings.Contains(err.Error(), "at most 3") {
		t.Fatalf("expected next_questions validation error, got %v", err)
	}
}

func TestResolveStructuredResultAddsInspectionArtifacts(t *testing.T) {
	prepared := &runtime.PreparedExecution{
		StructuredOutput: &runtime.StructuredOutputContract{ContractID: "structured-output.v1"},
	}
	result, report, failureErr, _ := resolveStructuredResult(context.Background(), prepared, `{"main_answer":"inspection summary"}`, ChatRespondRequest{
		TaskType: "inspection_task",
	}, "req-inspection", "sess-inspection", false, "", nil)
	if failureErr != nil || !report.Valid {
		t.Fatalf("expected valid result, got err=%v report=%#v", failureErr, report)
	}
	if result.StructuredResult["inspection_report"] == nil {
		t.Fatalf("expected inspection_report in structured_result, got %#v", result.StructuredResult)
	}
	if result.RightPanelView == nil {
		t.Fatalf("expected right_panel_view")
	}
	if len(result.ContentCards) == 0 {
		t.Fatalf("expected content cards")
	}
	if len(result.NextQuestions) == 0 {
		t.Fatalf("expected next questions")
	}
	if result.ScoreDelta == nil {
		t.Fatalf("expected score delta for inspection task")
	}
	if result.DeliveryProfile == nil || len(result.DeliveryProfile.StableFields) == 0 {
		t.Fatalf("expected delivery profile metadata")
	}
}

func TestResolveStructuredResultAddsWorkflowPlanOnDemand(t *testing.T) {
	prepared := &runtime.PreparedExecution{
		StructuredOutput: &runtime.StructuredOutputContract{ContractID: "structured-output.v1"},
	}
	result, report, failureErr, _ := resolveStructuredResult(context.Background(), prepared, `{"main_answer":"workflow summary"}`, ChatRespondRequest{
		TaskType:          "workflow_step_request",
		WorkflowRunID:     "run-1",
		StepID:            "step-analyze",
		DesiredOutputMode: "workflow_plan",
	}, "req-workflow", "sess-workflow", false, "", nil)
	if failureErr != nil || !report.Valid {
		t.Fatalf("expected valid result, got err=%v report=%#v", failureErr, report)
	}
	if result.StructuredResult["workflow_plan"] == nil {
		t.Fatalf("expected workflow_plan in structured_result, got %#v", result.StructuredResult)
	}
	plan, ok := result.StructuredResult["workflow_plan"].(*workflow.Plan)
	if !ok || plan == nil {
		t.Fatalf("expected workflow_plan contract, got %#v", result.StructuredResult["workflow_plan"])
	}
	if plan.Goal == "" || plan.RiskLevel == "" || !plan.RequiresConfirmation {
		t.Fatalf("workflow_plan missing stabilized fields = %#v", plan)
	}
	if result.StructuredResult["workflow_step_result"] == nil {
		t.Fatalf("expected workflow_step_result in structured_result, got %#v", result.StructuredResult)
	}
}

func TestAttachContextAssetArtifactsIncludesUsageAndEffectiveViews(t *testing.T) {
	result := &structuredChatResult{
		StructuredResult: map[string]any{},
	}
	trace := contextassets.UsageTrace{
		UsedContextAssets: []string{"persona.default", "policy_rule.core.safety_constitution"},
		ResidentAssets:    []string{"persona.default"},
		OnDemandAssets:    []string{"memory.weekly"},
		SuppressedAssets:  []string{"skill.experimental"},
		AssetConflictsResolved: []map[string]any{
			{"asset_type": "persona", "winner": "persona.default"},
		},
		RequestedAssetDetails: []string{"memory.weekly"},
		LoadedAssetDetails:    []string{"memory.weekly"},
		CandidateAssetTargets: []map[string]any{
			{"asset_id": "memory.weekly", "target_type": "memory_view"},
		},
		CandidateAssetDiffs: []map[string]any{
			{"asset_id": "memory.weekly", "summary": "new preference discovered"},
		},
		CandidateAssetUpdates: []map[string]any{
			{"asset_id": "memory.weekly", "mode": "candidate_update"},
		},
		AssetUsageTrace: []map[string]any{
			{"asset_id": "persona.default", "reason": "resident persona guidance applied"},
		},
	}
	views := contextassets.EffectiveViews{
		EffectivePersona: map[string]any{
			"summary": "直接、严格、证据优先",
		},
		EffectivePolicyRules: []map[string]any{
			{"rule_id": "safety_constitution", "title": "Safety Constitution"},
		},
		EffectiveSkills: []map[string]any{
			{"skill_id": "user_overview", "name": "User Overview"},
		},
	}

	attachContextAssetArtifacts(result, trace, views)

	if got := result.StructuredResult["used_context_assets"]; got == nil {
		t.Fatalf("used_context_assets missing: %#v", result.StructuredResult)
	}
	if got := result.StructuredResult["asset_usage_trace"]; got == nil {
		t.Fatalf("asset_usage_trace missing: %#v", result.StructuredResult)
	}
	if got := result.StructuredResult["candidate_asset_updates"]; got == nil {
		t.Fatalf("candidate_asset_updates missing: %#v", result.StructuredResult)
	}
	persona, ok := result.StructuredResult["effective_persona"].(map[string]any)
	if !ok || persona["summary"] != "直接、严格、证据优先" {
		t.Fatalf("effective_persona = %#v, want injected effective persona", result.StructuredResult["effective_persona"])
	}
	rules, ok := result.StructuredResult["effective_policy_rules"].([]map[string]any)
	if !ok || len(rules) != 1 || rules[0]["rule_id"] != "safety_constitution" {
		t.Fatalf("effective_policy_rules = %#v, want policy rule", result.StructuredResult["effective_policy_rules"])
	}
	skills, ok := result.StructuredResult["effective_skills"].([]map[string]any)
	if !ok || len(skills) != 1 || skills[0]["skill_id"] != "user_overview" {
		t.Fatalf("effective_skills = %#v, want skill", result.StructuredResult["effective_skills"])
	}
}

func TestResolveStructuredResultAddsAlertForIntegrationEvent(t *testing.T) {
	prepared := &runtime.PreparedExecution{
		StructuredOutput: &runtime.StructuredOutputContract{ContractID: "structured-output.v1"},
	}
	result, report, failureErr, _ := resolveStructuredResult(context.Background(), prepared, `{"main_answer":"alert summary"}`, ChatRespondRequest{
		TaskType:    "integration_event",
		TriggerType: "completed_event",
	}, "req-alert", "sess-alert", false, "", nil)
	if failureErr != nil || !report.Valid {
		t.Fatalf("expected valid result, got err=%v report=%#v", failureErr, report)
	}
	if result.StructuredResult["alert"] == nil {
		t.Fatalf("expected alert in structured_result, got %#v", result.StructuredResult)
	}
	if result.ScoreDelta == nil {
		t.Fatalf("expected score delta for integration event")
	}
}

func TestResolveStructuredResultRetrySuccessStillEnriches(t *testing.T) {
	prepared := &runtime.PreparedExecution{
		StructuredOutput: &runtime.StructuredOutputContract{ContractID: "structured-output.v1"},
	}
	originalFormattingRetry := formattingRetryFunc
	formattingRetryFunc = func(context.Context, *runtime.PreparedExecution, string, string) (string, error) {
		return `{"main_answer":"workflow retry summary"}`, nil
	}
	t.Cleanup(func() {
		formattingRetryFunc = originalFormattingRetry
	})

	result, report, failureErr, detail := resolveStructuredResult(context.Background(), prepared, "not valid json", ChatRespondRequest{
		StrictSchemaValidation: true,
		SchemaRetryCount:       1,
		SchemaRepairMode:       "off",
		SchemaFailureAction:    "error",
		TaskType:               "workflow_step_request",
		WorkflowRunID:          "run-1",
		StepID:                 "step-analyze",
		DesiredOutputMode:      "workflow_plan",
	}, "req-retry", "sess-retry", false, "", nil)
	if failureErr != nil {
		t.Fatalf("expected retry success, got err=%v", failureErr)
	}
	if detail != nil {
		t.Fatalf("expected no failure detail after retry success, got %#v", detail)
	}
	if !report.Valid || report.RetriesUsed != 1 {
		t.Fatalf("unexpected report = %#v", report)
	}
	if result.StructuredResult["scene_match"] == nil {
		t.Fatalf("expected scene_match after retry success, got %#v", result.StructuredResult)
	}
	if result.StructuredResult["workflow_plan"] == nil {
		t.Fatalf("expected workflow_plan after retry success, got %#v", result.StructuredResult)
	}
	if result.ResultSummary == nil {
		t.Fatalf("expected result_summary after retry success")
	}
	if len(result.ContentCards) == 0 {
		t.Fatalf("expected content_cards after retry success")
	}
	if result.RightPanelView == nil {
		t.Fatalf("expected right_panel_view after retry success")
	}
	if len(result.NextQuestions) == 0 {
		t.Fatalf("expected next_questions after retry success")
	}
}

func TestResolveStructuredResultAddsKnowledgeCandidatesOnDemand(t *testing.T) {
	prepared := &runtime.PreparedExecution{
		StructuredOutput: &runtime.StructuredOutputContract{ContractID: "structured-output.v1"},
	}
	result, report, failureErr, _ := resolveStructuredResult(context.Background(), prepared, `{"main_answer":"candidate summary"}`, ChatRespondRequest{
		DesiredOutputMode: "knowledge_candidate",
	}, "req-candidate", "sess-candidate", false, "", nil)
	if failureErr != nil || !report.Valid {
		t.Fatalf("expected valid result, got err=%v report=%#v", failureErr, report)
	}
	if result.StructuredResult["knowledge_candidates"] == nil {
		t.Fatalf("expected knowledge_candidates in structured_result, got %#v", result.StructuredResult)
	}
}

func TestResolveStructuredResultKeepsDefaultChatLightweight(t *testing.T) {
	prepared := &runtime.PreparedExecution{
		StructuredOutput: &runtime.StructuredOutputContract{ContractID: "structured-output.v1"},
	}
	result, report, failureErr, _ := resolveStructuredResult(context.Background(), prepared, `{"main_answer":"chat summary"}`, ChatRespondRequest{
		TaskType: "chat",
	}, "req-chat", "sess-chat", false, "", nil)
	if failureErr != nil || !report.Valid {
		t.Fatalf("expected valid result, got err=%v report=%#v", failureErr, report)
	}
	if result.ResultSummary != nil {
		t.Fatalf("expected default chat to keep result_summary optional, got %#v", result.ResultSummary)
	}
	if result.RightPanelView != nil || len(result.ContentCards) != 0 || result.ScoreDelta != nil {
		t.Fatalf("expected default chat richer fields to stay empty by default, got %#v", result)
	}
	if len(result.FollowUpSuggestions) == 0 || len(result.NextQuestions) == 0 {
		t.Fatalf("expected default chat follow-up suggestions to be populated")
	}
}

func TestResolveStructuredResultAddsAppDialogueSummary(t *testing.T) {
	prepared := &runtime.PreparedExecution{
		StructuredOutput: &runtime.StructuredOutputContract{ContractID: "structured-output.v1"},
	}
	result, report, failureErr, _ := resolveStructuredResult(context.Background(), prepared, `{"main_answer":"app summary"}`, ChatRespondRequest{
		TaskType:      "chat",
		AppInstanceID: "app-1",
	}, "req-app", "sess-app", false, "", nil)
	if failureErr != nil || !report.Valid {
		t.Fatalf("expected valid result, got err=%v report=%#v", failureErr, report)
	}
	if result.ResultSummary == nil {
		t.Fatalf("expected app dialogue to emit result_summary")
	}
}

func TestResolveStructuredResultAddsExecutionGovernanceArtifactsOnlyForExplicitScenarios(t *testing.T) {
	prepared := &runtime.PreparedExecution{
		StructuredOutput: &runtime.StructuredOutputContract{ContractID: "structured-output.v1"},
	}
	result, report, failureErr, _ := resolveStructuredResult(context.Background(), prepared, `{"main_answer":"execution summary"}`, ChatRespondRequest{
		TaskType:          "workflow_step_request",
		Scene:             "workflow",
		WorkflowRunID:     "run-1",
		StepID:            "step-analyze",
		DesiredOutputMode: "execution_governance",
		InputPayload: map[string]any{
			"execution_request": map[string]any{
				"command":           "python3 scripts/analyze.py",
				"arguments":         []any{"--dry-run"},
				"timeout_seconds":   45,
				"cpu_limit":         "2",
				"memory_limit_mb":   512,
				"env_whitelist":     []any{"PATH", "PYTHONPATH"},
				"filesystem_policy": "workspace_write",
			},
		},
	}, "req-exec", "sess-exec", false, "", nil)
	if failureErr != nil || !report.Valid {
		t.Fatalf("expected valid result, got err=%v report=%#v", failureErr, report)
	}
	intent, ok := result.StructuredResult["execution_intent"].(*runtime.ExecutionIntent)
	if !ok || intent == nil {
		t.Fatalf("expected execution_intent contract, got %#v", result.StructuredResult["execution_intent"])
	}
	if intent.RequestID != "req-exec" || intent.SessionID != "sess-exec" {
		t.Fatalf("unexpected correlation ids: %#v", intent)
	}
	if intent.ExecutionMode != runtime.ExecutionModeScriptExecution {
		t.Fatalf("execution mode = %q, want script_execution", intent.ExecutionMode)
	}
	if !intent.RequiresConfirmation {
		t.Fatalf("expected confirmation for script execution")
	}
	if result.DeliveryProfile == nil || !containsString(result.DeliveryProfile.StableFields, "execution_intent") {
		t.Fatalf("expected delivery profile to advertise execution_intent, got %#v", result.DeliveryProfile)
	}
}

func TestResolveStructuredResultConsumesExecutionResultInsideStructuredResultOnly(t *testing.T) {
	prepared := &runtime.PreparedExecution{
		StructuredOutput: &runtime.StructuredOutputContract{ContractID: "structured-output.v1"},
	}
	result, report, failureErr, _ := resolveStructuredResult(context.Background(), prepared, `{"main_answer":"workflow after execution"}`, ChatRespondRequest{
		TaskType:      "workflow_step_request",
		Scene:         "workflow",
		WorkflowRunID: "run-1",
		StepID:        "step-analyze",
		InputPayload: map[string]any{
			"execution_result": map[string]any{
				"execution_id": "exec-1",
				"intent_id":    "intent-1",
				"status":       "failed",
				"exit_code":    1,
				"stderr":       "permission denied",
			},
		},
	}, "req-exec-result", "sess-exec-result", false, "", nil)
	if failureErr != nil || !report.Valid {
		t.Fatalf("expected valid result, got err=%v report=%#v", failureErr, report)
	}
	executionResult, ok := result.StructuredResult["execution_result"].(*runtime.ExecutionResult)
	if !ok || executionResult == nil {
		t.Fatalf("expected execution_result contract, got %#v", result.StructuredResult["execution_result"])
	}
	if executionResult.ExecutionID != "exec-1" || executionResult.Status != "failed" {
		t.Fatalf("unexpected execution result = %#v", executionResult)
	}
	stepResult, ok := result.StructuredResult["workflow_step_result"].(workflow.StepResult)
	if !ok {
		t.Fatalf("expected workflow_step_result value, got %#v", result.StructuredResult["workflow_step_result"])
	}
	if stepResult.Decision != "review_execution_failure" {
		t.Fatalf("decision = %q, want review_execution_failure", stepResult.Decision)
	}
}

func TestResolveFormattingRetryModelConfigUsesRetryFormattingPolicy(t *testing.T) {
	prepared := &runtime.PreparedExecution{
		Spec: &runtime.ExecutionSpec{
			Tools: runtime.ToolSpec{
				AllowedTools: []string{"knowledge_search"},
			},
			Model: runtime.ModelSpec{
				ExecutedConfig: &model.ChatConfig{
					ProviderID:       "provider-primary",
					ProviderName:     "Primary Provider",
					ProviderProtocol: "openai_compatible",
					ModelRecordID:    "model-primary",
					ProviderModelID:  "gpt-primary",
					ModelDisplayName: "Primary Model",
				},
			},
			Metadata: runtime.ExecutionMetadata{
				Constraints: map[string]any{
					"task_type":             "workflow_step_request",
					"scene":                 "workflow",
					"desired_output_mode":   "workflow_plan",
					"model_policy_override": map[string]any{"tool_policy": "none"},
				},
			},
		},
	}
	cfg, err := resolveFormattingRetryModelConfig(prepared)
	if err != nil {
		t.Fatalf("resolveFormattingRetryModelConfig() error = %v", err)
	}
	if cfg.ResolvedParameters == nil {
		t.Fatalf("expected resolved parameters on retry config")
	}
	if cfg.ResolvedParameters.PolicyName != "retry_formatting" {
		t.Fatalf("policy_name = %q, want retry_formatting", cfg.ResolvedParameters.PolicyName)
	}
	if cfg.ResolvedParameters.Temperature != 0.0 || cfg.ResolvedParameters.TopP != 1.0 {
		t.Fatalf("unexpected retry parameters = %#v", cfg.ResolvedParameters)
	}
	if cfg.ResolvedParameters.ToolChoice.Kind != modelparams.ToolChoiceNone {
		t.Fatalf("tool_choice = %#v, want none", cfg.ResolvedParameters.ToolChoice)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestResolveStructuredResultWritesArtifactForExplicitArtifactMode(t *testing.T) {
	sharedRoot := t.TempDir()
	prepared := &runtime.PreparedExecution{
		StructuredOutput: &runtime.StructuredOutputContract{ContractID: "structured-output.v1"},
	}
	result, report, failureErr, _ := resolveStructuredResult(context.Background(), prepared, `{"main_answer":"# 风险报告\n主要结论"}`, ChatRespondRequest{
		TaskType:          "chat",
		WorkspaceID:       "ws-1",
		DesiredOutputMode: runtime.DesiredOutputModeArtifactWrite,
		InputPayload: map[string]any{
			"artifact_request": map[string]any{
				"kind":     "markdown",
				"title":    "风险报告",
				"filename": "risk-report.md",
			},
		},
	}, "req-artifact", "sess-artifact", false, sharedRoot, nil)
	if failureErr != nil || !report.Valid {
		t.Fatalf("expected valid result, got err=%v report=%#v", failureErr, report)
	}
	artifactResult, ok := result.StructuredResult["artifact_write"].(*runtime.ArtifactWriteResult)
	if !ok || artifactResult == nil {
		t.Fatalf("expected artifact_write result, got %#v", result.StructuredResult["artifact_write"])
	}
	if artifactResult.RelativePath != "workspaces/ws-1/reports/risk-report.md" {
		t.Fatalf("relative_path = %q, want workspaces/ws-1/reports/risk-report.md", artifactResult.RelativePath)
	}
	if !containsString(result.DeliveryProfile.StableFields, "artifact_write") {
		t.Fatalf("expected delivery profile to advertise artifact_write, got %#v", result.DeliveryProfile)
	}
}

func TestResolveStructuredResultAddsReadOnlyResourceParseAndRuntimeStateContracts(t *testing.T) {
	prepared := &runtime.PreparedExecution{
		StructuredOutput: &runtime.StructuredOutputContract{ContractID: "structured-output.v1"},
	}
	currentSession := &session.Session{
		ID:       "sess-state",
		Messages: []session.Message{{Role: "user", Content: "show state"}},
	}
	result, report, failureErr, _ := resolveStructuredResult(context.Background(), prepared, `{"main_answer":"state summary"}`, ChatRespondRequest{
		TaskType:          "workflow_step_request",
		WorkflowRunID:     "run-1",
		StepID:            "step-1",
		DesiredOutputMode: runtime.DesiredOutputModeQueryRuntimeState,
		InputPayload: map[string]any{
			"read_only_resource_request": map[string]any{
				"resource_id":    "doc-1",
				"resource_kind":  "injected_document",
				"resource_scope": "task",
				"projection":     "summary",
			},
			"read_only_resources": map[string]any{
				"doc-1": map[string]any{
					"summary": "injected summary",
				},
			},
			"structured_data_request": map[string]any{
				"format":  "json",
				"content": `{"name":"athena"}`,
			},
			"runtime_state_request": map[string]any{
				"include": []any{"session_snapshot", "structured_result", "last_turn_summary"},
			},
		},
	}, "req-state", "sess-state", false, "", currentSession)
	if failureErr != nil || !report.Valid {
		t.Fatalf("expected valid result, got err=%v report=%#v", failureErr, report)
	}
	if result.StructuredResult["read_only_resource_read"] == nil {
		t.Fatalf("expected read_only_resource_read result, got %#v", result.StructuredResult)
	}
	if result.StructuredResult["structured_data_parse"] == nil {
		t.Fatalf("expected structured_data_parse result, got %#v", result.StructuredResult)
	}
	stateResult, ok := result.StructuredResult["runtime_state"].(*runtime.RuntimeStateQueryResult)
	if !ok || stateResult == nil {
		t.Fatalf("expected runtime_state query result, got %#v", result.StructuredResult["runtime_state"])
	}
	if stateResult.SessionSnapshot == nil || stateResult.SessionSnapshot.SessionID != "sess-state" {
		t.Fatalf("unexpected session snapshot = %#v", stateResult.SessionSnapshot)
	}
	if stateResult.LastTurnSummary == "" {
		t.Fatalf("expected last_turn_summary in runtime state")
	}
}

func TestResolveStructuredResultAddsLocalTransformAndFactQualityContracts(t *testing.T) {
	prepared := &runtime.PreparedExecution{
		StructuredOutput: &runtime.StructuredOutputContract{ContractID: "structured-output.v1"},
	}
	result, report, failureErr, _ := resolveStructuredResult(context.Background(), prepared, `{"main_answer":"state summary"}`, ChatRespondRequest{
		Query:             "当前风险情况怎么样",
		DesiredOutputMode: runtime.DesiredOutputModeFactQualityGate,
		InputPayload: map[string]any{
			"local_data_transform_request": map[string]any{
				"operation":   "merge_objects",
				"source_keys": []any{"risk", "orders"},
			},
			"local_data_sources": map[string]any{
				"risk": map[string]any{
					"score": "medium",
				},
				"orders": map[string]any{
					"count": 2,
				},
			},
			"fact_quality_request": map[string]any{
				"question_scope":       "current_state",
				"session_context_used": true,
				"missing_data":         []any{"risk_flags", "review_status"},
			},
		},
	}, "req-fact", "sess-fact", false, "", nil)
	if failureErr != nil || !report.Valid {
		t.Fatalf("expected valid result, got err=%v report=%#v", failureErr, report)
	}
	if result.StructuredResult["local_data_transform"] == nil {
		t.Fatalf("expected local_data_transform result, got %#v", result.StructuredResult)
	}
	factQuality, ok := result.StructuredResult["fact_quality"].(*runtime.FactQualityGateResult)
	if !ok || factQuality == nil {
		t.Fatalf("expected fact_quality result, got %#v", result.StructuredResult["fact_quality"])
	}
	if factQuality.AnswerMode != runtime.FactAnswerModeClarification {
		t.Fatalf("answer_mode = %q, want clarification", factQuality.AnswerMode)
	}
	if !containsString(result.DeliveryProfile.StableFields, "local_data_transform") {
		t.Fatalf("expected delivery profile to advertise local_data_transform, got %#v", result.DeliveryProfile)
	}
	if !containsString(result.DeliveryProfile.StableFields, "fact_quality") {
		t.Fatalf("expected delivery profile to advertise fact_quality, got %#v", result.DeliveryProfile)
	}
}

func TestResolveStructuredResultBuildsAutomationPlanDraft(t *testing.T) {
	prepared := &runtime.PreparedExecution{
		StructuredOutput: &runtime.StructuredOutputContract{ContractID: "structured-output.v1"},
	}
	result, report, failureErr, _ := resolveStructuredResult(context.Background(), prepared, `{"main_answer":"自动化计划说明"}`, ChatRespondRequest{
		TaskType:          "scheduled_job",
		MainSessionID:     "main-1",
		DesiredOutputMode: "automation_plan_draft",
		Query:             "每日分析我的个人习惯",
		Scene:             "main",
		GlobalContext: map[string]any{
			"platform_context_catalog": map[string]any{
				"memory":    map[string]any{"available": true},
				"knowledge": map[string]any{"available": true},
				"skills":    map[string]any{"available": true},
				"persona":   map[string]any{"available": true},
			},
			"memory_summary":    map[string]any{"summary": "长期关注习惯变化与持续复盘"},
			"knowledge_summary": map[string]any{"summary": "已有习惯分析相关知识摘要"},
			"skills_summary":    map[string]any{"summary": "支持分析与自动化规划能力"},
			"persona_summary":   map[string]any{"summary": "偏好简洁、结构化表达"},
		},
	}, "req-auto-plan", "sess-auto-plan", false, "", nil)
	if failureErr != nil || !report.Valid {
		t.Fatalf("expected valid result, got err=%v report=%#v", failureErr, report)
	}
	draft, ok := result.StructuredResult["automation_plan_draft"].(*automation.PlanDraft)
	if !ok || draft == nil {
		t.Fatalf("expected automation_plan_draft, got %#v", result.StructuredResult["automation_plan_draft"])
	}
	if draft.PlanType == "" || draft.Goal == "" || draft.UserVisibleExplanation == "" {
		t.Fatalf("draft missing stable fields = %#v", draft)
	}
	if result.StructuredResult["user_visible_explanation"] == "" {
		t.Fatalf("expected user_visible_explanation")
	}
	usedContexts, ok := result.StructuredResult["used_contexts"].([]string)
	if !ok || len(usedContexts) == 0 {
		t.Fatalf("used_contexts = %#v", result.StructuredResult["used_contexts"])
	}
	contextUsage, ok := result.StructuredResult["context_usage"].(map[string]bool)
	if !ok || !contextUsage["memory"] || !contextUsage["knowledge"] {
		t.Fatalf("context_usage = %#v", result.StructuredResult["context_usage"])
	}
}

func TestResolveStructuredResultUsesExplicitAutomationEntryMode(t *testing.T) {
	prepared := &runtime.PreparedExecution{
		StructuredOutput: &runtime.StructuredOutputContract{ContractID: "structured-output.v1"},
	}
	result, report, failureErr, _ := resolveStructuredResult(context.Background(), prepared, `{"main_answer":"自动化计划说明"}`, ChatRespondRequest{
		TaskType: "chat",
		Query:    "请为我设计一个每日执行的计划",
		InputPayload: map[string]any{
			"interaction_context": map[string]any{
				"entry_mode": "automation_create",
			},
		},
	}, "req-auto-entry", "sess-auto-entry", false, "", nil)
	if failureErr != nil || !report.Valid {
		t.Fatalf("expected valid result, got err=%v report=%#v", failureErr, report)
	}
	mode, ok := result.StructuredResult["interaction_mode"].(*runtime.InteractionModeResult)
	if !ok || mode == nil || mode.Mode != "automation_draft" {
		t.Fatalf("interaction_mode = %#v, want automation_draft", result.StructuredResult["interaction_mode"])
	}
	intentResolution, ok := result.StructuredResult["intent_resolution"].(*runtime.IntentResolutionResult)
	if !ok || intentResolution == nil || intentResolution.SelectedRoute != "automation_draft" {
		t.Fatalf("intent_resolution = %#v, want automation_draft route", result.StructuredResult["intent_resolution"])
	}
	if result.StructuredResult["automation_plan_draft"] == nil {
		t.Fatalf("expected automation_plan_draft")
	}
	createPayload, ok := result.StructuredResult["automation_create_payload"].(*platformautomation.CreatePayload)
	if !ok || createPayload == nil || createPayload.Trigger.Cadence == "" {
		t.Fatalf("automation_create_payload = %#v, want structured payload", result.StructuredResult["automation_create_payload"])
	}
	if result.StructuredResult["platform_tool_hints"] == nil {
		t.Fatalf("expected platform_tool_hints")
	}
	if result.StructuredResult["platform_tool_descriptors"] == nil {
		t.Fatalf("expected platform_tool_descriptors")
	}
	if result.StructuredResult["workflow_plan"] == nil {
		t.Fatalf("expected workflow_plan")
	}
	if result.RightPanelView == nil {
		t.Fatalf("expected right_panel_view")
	}
	progress, ok := result.StructuredResult["interaction_progress"].(*runtime.InteractionProgress)
	if !ok || progress == nil || progress.CurrentStage != "building_draft" {
		t.Fatalf("interaction_progress = %#v, want building_draft", result.StructuredResult["interaction_progress"])
	}
}

func TestResolveStructuredResultReturnsChoiceRequiredForAmbiguousAutomationIntent(t *testing.T) {
	prepared := &runtime.PreparedExecution{
		StructuredOutput: &runtime.StructuredOutputContract{ContractID: "structured-output.v1"},
	}
	result, report, failureErr, _ := resolveStructuredResult(context.Background(), prepared, `{"main_answer":"我们可以继续聊清楚需求。 "}`, ChatRespondRequest{
		TaskType: "chat",
		Query:    "我想让墨思每天帮我做点事情",
	}, "req-choice", "sess-choice", false, "", nil)
	if failureErr != nil || !report.Valid {
		t.Fatalf("expected valid result, got err=%v report=%#v", failureErr, report)
	}
	mode, ok := result.StructuredResult["interaction_mode"].(*runtime.InteractionModeResult)
	if !ok || mode == nil || mode.Mode != "choice_required" {
		t.Fatalf("interaction_mode = %#v, want choice_required", result.StructuredResult["interaction_mode"])
	}
	intentResolution, ok := result.StructuredResult["intent_resolution"].(*runtime.IntentResolutionResult)
	if !ok || intentResolution == nil || !intentResolution.RequiresClarification {
		t.Fatalf("intent_resolution = %#v, want requires_clarification", result.StructuredResult["intent_resolution"])
	}
	options, ok := result.StructuredResult["interaction_options"].([]runtime.InteractionOption)
	if !ok || len(options) != 2 {
		t.Fatalf("interaction_options = %#v, want 2 options", result.StructuredResult["interaction_options"])
	}
	if options[0].ID != "continue_chat" {
		t.Fatalf("first interaction option id = %q, want continue_chat", options[0].ID)
	}
}

func TestResolveStructuredResultBuildsStableRightPanelView(t *testing.T) {
	prepared := &runtime.PreparedExecution{
		StructuredOutput: &runtime.StructuredOutputContract{ContractID: "structured-output.v1"},
	}
	result, report, failureErr, _ := resolveStructuredResult(context.Background(), prepared, `{"main_answer":"workflow summary"}`, ChatRespondRequest{
		TaskType:          "workflow_step_request",
		WorkflowRunID:     "run-1",
		StepID:            "step-analyze",
		DesiredOutputMode: "right_panel_view",
	}, "req-panel", "sess-panel", false, "", nil)
	if failureErr != nil || !report.Valid {
		t.Fatalf("expected valid result, got err=%v report=%#v", failureErr, report)
	}
	if result.RightPanelView == nil {
		t.Fatalf("expected right_panel_view")
	}
	if result.RightPanelView.View == "" || result.RightPanelView.Title == "" {
		t.Fatalf("right_panel_view missing stable fields = %#v", result.RightPanelView)
	}
	if result.RightPanelView.Content == "" && len(result.RightPanelView.Sections) == 0 {
		t.Fatalf("right_panel_view missing content/sections = %#v", result.RightPanelView)
	}
}

func TestResolveStructuredResultAddsKnowledgeRetrievalPipeline(t *testing.T) {
	prepared := &runtime.PreparedExecution{
		StructuredOutput: &runtime.StructuredOutputContract{ContractID: "structured-output.v1"},
	}
	result, report, failureErr, _ := resolveStructuredResult(context.Background(), prepared, `{"main_answer":"candidate summary"}`, ChatRespondRequest{
		TaskType:          "inspection_task",
		DesiredOutputMode: "knowledge_candidate",
		Query:             "show me the latest inspection knowledge",
	}, "req-knowledge", "sess-knowledge", false, "", nil)
	if failureErr != nil || !report.Valid {
		t.Fatalf("expected valid result, got err=%v report=%#v", failureErr, report)
	}
	if result.StructuredResult["knowledge_candidates"] == nil {
		t.Fatalf("expected knowledge_candidates")
	}
	if result.StructuredResult["knowledge_retrieval"] == nil {
		t.Fatalf("expected knowledge_retrieval")
	}
}

func TestResolveStructuredResultReturnsChoiceRequiredForExplicitRouteChoicePrompt(t *testing.T) {
	prepared := &runtime.PreparedExecution{
		StructuredOutput: &runtime.StructuredOutputContract{ContractID: "structured-output.v1"},
	}
	result, report, failureErr, _ := resolveStructuredResult(context.Background(), prepared, `{"main_answer":"请先确认路径。 "}`, ChatRespondRequest{
		TaskType: "chat",
		Query:    "我想把习惯分析这件事做成可周期执行的任务，但还没决定是先继续讨论还是直接生成自动化草案。",
	}, "req-choice-explicit", "sess-choice-explicit", false, "", nil)
	if failureErr != nil || !report.Valid {
		t.Fatalf("expected valid result, got err=%v report=%#v", failureErr, report)
	}
	mode, ok := result.StructuredResult["interaction_mode"].(*runtime.InteractionModeResult)
	if !ok || mode == nil || mode.Mode != "choice_required" {
		t.Fatalf("interaction_mode = %#v, want choice_required", result.StructuredResult["interaction_mode"])
	}
	progress, ok := result.StructuredResult["interaction_progress"].(*runtime.InteractionProgress)
	if !ok || progress == nil || progress.CurrentStage != "detecting_interaction_mode" {
		t.Fatalf("interaction_progress = %#v, want detecting_interaction_mode", result.StructuredResult["interaction_progress"])
	}
}

func TestResolveStructuredResultReturnsChoiceRequiredForPendingDraftRoutePrompt(t *testing.T) {
	prepared := &runtime.PreparedExecution{
		StructuredOutput: &runtime.StructuredOutputContract{ContractID: "structured-output.v1"},
	}
	result, report, failureErr, _ := resolveStructuredResult(context.Background(), prepared, `{"main_answer":"请先确认路径。 "}`, ChatRespondRequest{
		TaskType: "chat",
		Query:    "我想让系统以后定期帮我分析个人习惯，但这次先帮我判断接下来应该继续聊细节，还是直接进入待确认方案。",
	}, "req-choice-pending-plan", "sess-choice-pending-plan", false, "", nil)
	if failureErr != nil || !report.Valid {
		t.Fatalf("expected valid result, got err=%v report=%#v", failureErr, report)
	}
	mode, ok := result.StructuredResult["interaction_mode"].(*runtime.InteractionModeResult)
	if !ok || mode == nil || mode.Mode != "choice_required" {
		t.Fatalf("interaction_mode = %#v, want choice_required", result.StructuredResult["interaction_mode"])
	}
	options, ok := result.StructuredResult["interaction_options"].([]runtime.InteractionOption)
	if !ok || len(options) != 2 || options[0].ID != "continue_chat" {
		t.Fatalf("interaction_options = %#v", result.StructuredResult["interaction_options"])
	}
}

func TestResolveStructuredResultRequestsKnowledgeDetailWhenSummaryTooThin(t *testing.T) {
	prepared := &runtime.PreparedExecution{
		StructuredOutput: &runtime.StructuredOutputContract{ContractID: "structured-output.v1"},
	}
	result, report, failureErr, _ := resolveStructuredResult(context.Background(), prepared, `{"main_answer":"近期供应链安全关注点摘要"}`, ChatRespondRequest{
		TaskType: "chat",
		Scene:    "security_review",
		Query:    "结合我的知识详情，给我一个近期供应链安全关注点总结。",
		GlobalContext: map[string]any{
			"platform_context_catalog": map[string]any{
				"knowledge": map[string]any{"available": true},
			},
			"knowledge_summary": map[string]any{"summary": "供应链"},
		},
	}, "req-context-detail", "sess-context-detail", false, "", nil)
	if failureErr != nil || !report.Valid {
		t.Fatalf("expected valid result, got err=%v report=%#v", failureErr, report)
	}
	details, ok := result.StructuredResult["context_details_requested"].([]string)
	if !ok || len(details) != 1 || details[0] != "knowledge" {
		t.Fatalf("context_details_requested = %#v", result.StructuredResult["context_details_requested"])
	}
	hints, ok := result.StructuredResult["platform_tool_hints"].([]platformtools.ToolInvocationHint)
	if !ok || len(hints) == 0 || hints[0].ToolName != "get_platform_context_detail" {
		t.Fatalf("platform_tool_hints = %#v", result.StructuredResult["platform_tool_hints"])
	}
}

func TestResolveStructuredResultMarksLoadedContextDetailsWhenPreloaded(t *testing.T) {
	prepared := &runtime.PreparedExecution{
		StructuredOutput: &runtime.StructuredOutputContract{ContractID: "structured-output.v1"},
	}
	result, report, failureErr, _ := resolveStructuredResult(context.Background(), prepared, `{"main_answer":"近期供应链安全关注点摘要"}`, ChatRespondRequest{
		TaskType: "chat",
		Scene:    "security_review",
		Query:    "结合我的知识详情，给我一个近期供应链安全关注点总结。",
		GlobalContext: map[string]any{
			"platform_context_catalog": map[string]any{
				"knowledge": map[string]any{"available": true},
			},
			"knowledge_summary": map[string]any{"summary": "供应链"},
			"knowledge_detail":  map[string]any{"summary": "近期重点关注供应链安全和依赖风险"},
		},
	}, "req-context-loaded", "sess-context-loaded", false, "", nil)
	if failureErr != nil || !report.Valid {
		t.Fatalf("expected valid result, got err=%v report=%#v", failureErr, report)
	}
	loaded, ok := result.StructuredResult["context_details_loaded"].([]string)
	if !ok || len(loaded) != 1 || loaded[0] != "knowledge" {
		t.Fatalf("context_details_loaded = %#v", result.StructuredResult["context_details_loaded"])
	}
	if _, ok := result.StructuredResult["platform_tool_hints"]; ok {
		t.Fatalf("platform_tool_hints should be omitted when detail is already loaded: %#v", result.StructuredResult["platform_tool_hints"])
	}
}

func TestResolveStructuredResultUsesPlatformContextToImproveCreatePayload(t *testing.T) {
	prepared := &runtime.PreparedExecution{
		StructuredOutput: &runtime.StructuredOutputContract{ContractID: "structured-output.v1"},
	}
	result, report, failureErr, _ := resolveStructuredResult(context.Background(), prepared, `{"main_answer":"自动化计划说明"}`, ChatRespondRequest{
		TaskType:          "scheduled_job",
		MainSessionID:     "main-1",
		DesiredOutputMode: "automation_plan_draft",
		Query:             "请为我设计一个周期性分析任务。",
		Scene:             "main",
		GlobalContext: map[string]any{
			"platform_context_catalog": map[string]any{
				"knowledge": map[string]any{"available": true},
				"skills":    map[string]any{"available": true},
			},
			"knowledge_summary": map[string]any{"summary": "近期重点关注供应链安全和依赖风险"},
			"skills_summary":    map[string]any{"summary": "支持分析与自动化能力"},
		},
	}, "req-auto-context", "sess-auto-context", false, "", nil)
	if failureErr != nil || !report.Valid {
		t.Fatalf("expected valid result, got err=%v report=%#v", failureErr, report)
	}
	payload, ok := result.StructuredResult["automation_create_payload"].(*platformautomation.CreatePayload)
	if !ok || payload == nil {
		t.Fatalf("automation_create_payload = %#v", result.StructuredResult["automation_create_payload"])
	}
	if payload.Title != "供应链安全分析" {
		t.Fatalf("payload.Title = %q, want 供应链安全分析", payload.Title)
	}
	if payload.Goal != "定期分析近期供应链安全关注点并生成摘要" {
		t.Fatalf("payload.Goal = %q", payload.Goal)
	}
}

func TestBuildAutomationDraftFallbackEnvelopeReturnsCompletedFallback(t *testing.T) {
	prepared := &runtime.PreparedExecution{
		StructuredOutput: &runtime.StructuredOutputContract{ContractID: "structured-output.v1"},
	}
	envelope := buildAutomationDraftFallbackEnvelope(prepared, ChatRespondRequest{
		TaskType:          "workflow_step_request",
		DesiredOutputMode: "automation_plan_draft",
		Query:             "请为我设计一个每日习惯分析的自动化计划草案，先不要执行，等我确认后再创建。",
	}, "req-fallback", "sess-fallback", "", nil, "collect_respond_output_failed: timeout")
	if envelope.Status != "completed_with_fallback" {
		t.Fatalf("status = %q, want completed_with_fallback", envelope.Status)
	}
	if envelope.Result == nil || envelope.Result.StructuredResult["automation_plan_draft"] == nil {
		t.Fatalf("expected fallback automation_plan_draft, got %#v", envelope.Result)
	}
	if envelope.Result.StructuredResult["fallback_reason"] == nil {
		t.Fatalf("expected fallback_reason")
	}
}

func TestHandleChatRespondReturnsFallbackWhenOpenSessionFailsForAutomationDraft(t *testing.T) {
	cfg := config.Config{
		Runtime: config.RuntimeConfig{
			RequestTimeoutSeconds: 30,
		},
	}
	ctx := hertzapp.NewContext(0)
	ctx.Request.Header.SetMethod("POST")
	ctx.Request.SetRequestURI("/api/chat/respond")
	body := map[string]any{
		"task_type":           "chat",
		"query":               "请为我设计一个每日习惯分析的自动化计划草案，先不要执行，等我确认后再创建。",
		"desired_output_mode": "automation_plan_draft",
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	ctx.Request.SetBodyRaw(encoded)

	app := &appcore.Service{}
	handleChatRespond(context.Background(), ctx, cfg, app)
	if got := ctx.Response.StatusCode(); got != 200 {
		t.Fatalf("status = %d, want 200", got)
	}
	var envelope chatRespondEnvelope
	if err := json.Unmarshal(ctx.Response.Body(), &envelope); err != nil {
		t.Fatalf("Unmarshal() error = %v; body=%s", err, string(ctx.Response.Body()))
	}
	if envelope.Status != "completed_with_fallback" {
		t.Fatalf("status = %q, want completed_with_fallback; body=%s", envelope.Status, string(ctx.Response.Body()))
	}
	if envelope.Result == nil || envelope.Result.StructuredResult["automation_plan_draft"] == nil {
		t.Fatalf("expected fallback automation_plan_draft, got %#v", envelope.Result)
	}
}

func TestWriteRecoveredRespondFailureReturnsAutomationFallback(t *testing.T) {
	ctx := hertzapp.NewContext(0)
	originalResolveStructuredResult := resolveStructuredResultFunc
	resolveStructuredResultFunc = func(context.Context, *appcore.Service, controlplane.RuntimeTuning, *runtime.PreparedExecution, string, ChatRespondRequest, string, string, bool, string, *session.Session) (*structuredChatResult, schemaValidationReport, error, map[string]any) {
		return &structuredChatResult{
			MainAnswer:       "fallback answer",
			Answer:           "fallback answer",
			StructuredResult: map[string]any{"automation_plan_draft": map[string]any{"goal": "draft"}},
		}, schemaValidationReport{Valid: true}, nil, nil
	}
	t.Cleanup(func() {
		resolveStructuredResultFunc = originalResolveStructuredResult
	})

	writeRecoveredRespondFailure(ctx, nil, controlplane.DefaultRuntimeTuning(), nil, ChatRespondRequest{
		TaskType:          "chat",
		Query:             "请为我设计一个每日习惯分析的自动化计划草案，先不要执行，等我确认后再创建。",
		DesiredOutputMode: "automation_plan_draft",
	}, "req-panic", "sess-panic", "", nil, "forced resolve panic")
	if got := ctx.Response.StatusCode(); got != 200 {
		t.Fatalf("status = %d, want 200", got)
	}
	var envelope chatRespondEnvelope
	if err := json.Unmarshal(ctx.Response.Body(), &envelope); err != nil {
		t.Fatalf("Unmarshal() error = %v; body=%s", err, string(ctx.Response.Body()))
	}
	if envelope.Status != "completed_with_fallback" {
		t.Fatalf("status = %q, want completed_with_fallback; body=%s", envelope.Status, string(ctx.Response.Body()))
	}
	if envelope.Result == nil || envelope.Result.StructuredResult["fallback_reason"] == nil {
		t.Fatalf("expected fallback result with fallback_reason, got %#v", envelope.Result)
	}
}
