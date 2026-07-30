package runtime

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cloudwego/eino/adk"
	einotool "github.com/cloudwego/eino/components/tool"
	einoschema "github.com/cloudwego/eino/schema"
	runtimetask "moss/internal/runtime/task"
)

func TestParseExecutionBudgetAndEffectiveDeadline(t *testing.T) {
	now := time.Date(2026, 7, 23, 3, 0, 0, 0, time.UTC)
	budget, err := ParseExecutionBudget(map[string]any{
		"max_duration_ms": 5000.0,
		"deadline_at":     now.Add(3 * time.Second).Format(time.RFC3339Nano),
		"max_model_calls": "4",
		"max_tool_calls":  8,
		"max_tokens":      int64(4096),
	})
	if err != nil {
		t.Fatalf("ParseExecutionBudget() error=%v", err)
	}
	deadline, ok := budget.EffectiveDeadline(now)
	if !ok || !deadline.Equal(now.Add(3*time.Second)) {
		t.Fatalf("deadline=%s ok=%v", deadline, ok)
	}
	if budget.MaxModelCalls != 4 || budget.MaxToolCalls != 8 || budget.MaxTokens != 4096 {
		t.Fatalf("budget=%#v", budget)
	}
}

func TestParseExecutionBudgetRejectsInvalidLimits(t *testing.T) {
	tests := []map[string]any{
		{"max_duration_ms": 0},
		{"max_model_calls": 1.5},
		{"max_tool_calls": -1},
		{"max_tokens": "many"},
		{"deadline_at": "tomorrow"},
		{"max_model_call": 3},
	}
	for _, input := range tests {
		if _, err := ParseExecutionBudget(input); err == nil {
			t.Fatalf("ParseExecutionBudget(%#v) expected error", input)
		}
	}
}

func TestResolveExecutionControlPlanFromTask(t *testing.T) {
	plan, err := ResolveExecutionControlPlan(&runtimetask.RuntimeTask{InputPayload: map[string]any{
		"agent_run": map[string]any{
			"success_criteria": []any{"has risks", "has next action", "has risks"},
			"budget":           map[string]any{"max_model_calls": 3},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.SuccessCriteria) != 2 || plan.Budget.MaxModelCalls != 3 {
		t.Fatalf("plan=%#v", plan)
	}
}

func TestExecutionBudgetExceededMapsToStableStopReason(t *testing.T) {
	err := &ExecutionBudgetExceededError{Dimension: "model_calls", Limit: 2, Observed: 3}
	if !errors.Is(err, ErrExecutionBudgetExceeded) {
		t.Fatalf("errors.Is(%v) = false", err)
	}
	if got := NormalizeExecutionStopReason("failed", err, ""); got != ExecutionStopBudgetExhausted {
		t.Fatalf("NormalizeExecutionStopReason()=%q", got)
	}
}

func TestRuntimeGraphNativeModelCallBudgetStopsBeforeInvocation(t *testing.T) {
	state := &runtimeGraphNativeState{ModelCalls: 1}
	handler := runtimeGraphNativeModelPreHandlerWithBudget(ExecutionBudget{MaxModelCalls: 1})
	_, err := handler(context.Background(), []*einoschema.Message{einoschema.UserMessage("continue")}, state)
	assertExecutionBudgetDimension(t, err, "model_calls")
}

func TestEinoGraphNativeAgentEnforcesModelCallBudget(t *testing.T) {
	model := &graphNativeRecordingModel{generate: func(call int, _ []*einoschema.Message) (*einoschema.Message, error) {
		return einoschema.AssistantMessage("", []einoschema.ToolCall{{
			ID: "call_lookup",
			Function: einoschema.FunctionCall{
				Name:      "lookup",
				Arguments: `{"query":"athena"}`,
			},
		}}), nil
	}}
	lookup := &graphNativeLookupTool{}
	agent, err := NewEinoGraphNativeAgent(context.Background(), EinoGraphNativeAgentConfig{
		Model:          model,
		Tools:          []einotool.BaseTool{lookup},
		ToolTranscript: NewToolCallTranscript(),
		Budget:         ExecutionBudget{MaxModelCalls: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	runner := adk.NewRunner(context.Background(), adk.RunnerConfig{Agent: agent, EnableStreaming: true})
	iter := runner.Run(context.Background(), []adk.Message{einoschema.UserMessage("lookup athena")})
	var runErr error
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		if event != nil && event.Err != nil {
			runErr = event.Err
		}
	}
	assertExecutionBudgetDimension(t, runErr, "model_calls")
	if model.generateCalls != 1 || lookup.calls != 1 {
		t.Fatalf("model calls=%d tool calls=%d, want 1 and 1", model.generateCalls, lookup.calls)
	}
}

func TestRuntimeGraphNativeToolCallBudgetStopsBatchBeforeInvocation(t *testing.T) {
	state := &runtimeGraphNativeState{}
	handler := runtimeGraphNativeToolsPreHandlerWithBudget(ExecutionBudget{MaxToolCalls: 1})
	message := einoschema.AssistantMessage("", []einoschema.ToolCall{{ID: "one"}, {ID: "two"}})
	_, err := handler(context.Background(), message, state)
	assertExecutionBudgetDimension(t, err, "tool_calls")
	if state.ToolCalls != 0 {
		t.Fatalf("tool calls=%d, want no partial reservation", state.ToolCalls)
	}
}

func TestRuntimeGraphNativeTokenBudgetStopsAfterReportedUsage(t *testing.T) {
	state := &runtimeGraphNativeState{}
	handler := runtimeGraphNativeModelPostHandlerWithTranscriptAndBudget(NewToolCallTranscript(), ExecutionBudget{MaxTokens: 9})
	message := einoschema.AssistantMessage("done", nil)
	message.ResponseMeta = &einoschema.ResponseMeta{Usage: &einoschema.TokenUsage{TotalTokens: 10}}
	_, err := handler(context.Background(), message, state)
	assertExecutionBudgetDimension(t, err, "tokens")
	if state.TotalTokens != 10 {
		t.Fatalf("total tokens=%d, want 10", state.TotalTokens)
	}
}

func TestRuntimeGraphNativeTokenBudgetFailsClosedWithoutUsage(t *testing.T) {
	state := &runtimeGraphNativeState{}
	handler := runtimeGraphNativeModelPostHandlerWithTranscriptAndBudget(NewToolCallTranscript(), ExecutionBudget{MaxTokens: 9})
	_, err := handler(context.Background(), einoschema.AssistantMessage("done", nil), state)
	if !errors.Is(err, ErrExecutionTokenUsageUnavailable) {
		t.Fatalf("error=%v, want ErrExecutionTokenUsageUnavailable", err)
	}
}

func TestEinoGraphNativeBudgetCountersSurviveCheckpointResume(t *testing.T) {
	ctx := context.Background()
	approvalTool := &graphNativeApprovalTool{}
	model := &graphNativeRecordingModel{generate: func(call int, _ []*einoschema.Message) (*einoschema.Message, error) {
		return einoschema.AssistantMessage("", []einoschema.ToolCall{{
			ID: "call_approval",
			Function: einoschema.FunctionCall{
				Name:      "approval",
				Arguments: `{"action":"continue"}`,
			},
		}}), nil
	}}
	agent, err := NewEinoGraphNativeAgent(ctx, EinoGraphNativeAgentConfig{
		Model:           model,
		Tools:           []einotool.BaseTool{approvalTool},
		CheckpointStore: NewMemoryRuntimeGraphCheckpointStore(),
		CheckpointID:    "budget-checkpoint-test",
		Budget:          ExecutionBudget{MaxModelCalls: 1, MaxToolCalls: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	runner := adk.NewRunner(ctx, adk.RunnerConfig{Agent: agent})
	runGraphNativeAgentUntilInterrupt(t, runner, []*einoschema.Message{einoschema.UserMessage("needs approval")})
	iter := agent.Resume(ctx, &adk.ResumeInfo{})
	var runErr error
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		if event != nil && event.Err != nil {
			runErr = event.Err
		}
	}
	assertExecutionBudgetDimension(t, runErr, "model_calls")
	if model.generateCalls != 1 || !approvalTool.resumed {
		t.Fatalf("model calls=%d approval resumed=%v, want restored counter before second model call", model.generateCalls, approvalTool.resumed)
	}
}

func assertExecutionBudgetDimension(tb testing.TB, err error, dimension string) {
	tb.Helper()
	var budgetErr *ExecutionBudgetExceededError
	if !errors.As(err, &budgetErr) || budgetErr.Dimension != dimension {
		tb.Fatalf("error=%v, want budget dimension %q", err, dimension)
	}
}

func BenchmarkParseExecutionBudget(b *testing.B) {
	input := map[string]any{
		"max_duration_ms": 30_000,
		"max_model_calls": 8,
		"max_tool_calls":  24,
		"max_tokens":      16_384,
		"deadline_at":     "2026-07-23T05:00:00Z",
	}
	for b.Loop() {
		if _, err := ParseExecutionBudget(input); err != nil {
			b.Fatal(err)
		}
	}
}
