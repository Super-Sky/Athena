package runtime

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestToolCallTranscriptCorrelatesParallelResultsInDeclarationOrder(t *testing.T) {
	transcript := NewToolCallTranscript()
	firstID := transcript.Register("call_first", "function", "first", `{"value":1}`)
	secondID := transcript.Register("call_second", "function", "second", `{"value":2}`)
	startedAt := time.Date(2026, time.July, 9, 1, 2, 3, 0, time.UTC)

	transcript.Start(secondID, "second", `{"value":2}`, startedAt)
	transcript.Finish(secondID, "second", `{"result":2}`, nil, startedAt.Add(20*time.Millisecond), 20*time.Millisecond)
	transcript.Start(firstID, "first", `{"value":1}`, startedAt.Add(time.Millisecond))
	transcript.Finish(firstID, "first", "", errors.New("first failed"), startedAt.Add(31*time.Millisecond), 30*time.Millisecond)

	calls := transcript.Snapshot()
	if len(calls) != 2 {
		t.Fatalf("calls len = %d, want 2", len(calls))
	}
	if calls[0].ID != firstID || calls[0].Index != 0 || calls[0].Status != ToolCallStatusFailed {
		t.Fatalf("first call = %#v, want failed declaration-order call", calls[0])
	}
	if calls[0].Result == nil || calls[0].Result.ToolCallID != firstID || !calls[0].Result.IsError {
		t.Fatalf("first result = %#v, want correlated error", calls[0].Result)
	}
	if calls[1].ID != secondID || calls[1].Index != 1 || calls[1].Status != ToolCallStatusCompleted {
		t.Fatalf("second call = %#v, want completed declaration-order call", calls[1])
	}
	if calls[1].Result == nil || calls[1].Result.ToolCallID != secondID || calls[1].Result.Content != `{"result":2}` {
		t.Fatalf("second result = %#v, want correlated content", calls[1].Result)
	}
}

func TestToolCallRedactedPayloadKeepsCorrelationWithoutRawData(t *testing.T) {
	call := ToolCall{
		Index:     2,
		Round:     1,
		ID:        "call_secret",
		Type:      ToolTypeFunction,
		Name:      "lookup",
		Arguments: `{"token":"secret-value","query":"athena"}`,
		Status:    ToolCallStatusCompleted,
		Result: &ToolResult{
			ToolCallID: "call_secret",
			Name:       "lookup",
			Content:    `{"secret":"result-value"}`,
		},
	}
	payload := toolCallRedactedPayload(call)
	rendered := fmt.Sprintf("%v", payload)
	if payload["tool_call_id"] != "call_secret" || payload["result_tool_call_id"] != "call_secret" {
		t.Fatalf("payload = %#v, want correlated IDs", payload)
	}
	if !containsRuntimeString(payload["argument_keys"].([]string), "token") {
		t.Fatalf("argument keys = %#v, want token key", payload["argument_keys"])
	}
	if fmt.Sprint(payload["argument_runes"]) == "0" || fmt.Sprint(payload["result_runes"]) == "0" {
		t.Fatalf("payload = %#v, want non-zero safe lengths", payload)
	}
	if strings.Contains(rendered, "secret-value") || strings.Contains(rendered, "result-value") {
		t.Fatalf("redacted payload leaked raw data: %s", rendered)
	}
}

func containsRuntimeString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func TestToolCallTranscriptSynthesizesUniqueStableIDs(t *testing.T) {
	transcript := NewToolCallTranscript()
	firstID := transcript.Register("", "", "lookup", `{}`)
	secondID := transcript.Register("duplicate", "function", "lookup", `{}`)
	thirdID := transcript.Register("duplicate", "function", "lookup", `{}`)

	if firstID == "" || firstID[:5] != "call_" {
		t.Fatalf("first id = %q, want synthesized call_ id", firstID)
	}
	if secondID != "duplicate" {
		t.Fatalf("second id = %q, want provider id", secondID)
	}
	if thirdID == secondID || thirdID[:5] != "call_" {
		t.Fatalf("third id = %q, want unique synthesized call_ id", thirdID)
	}
}

func BenchmarkToolCallTranscriptRegisterAndCorrelate(b *testing.B) {
	for index := 0; index < b.N; index++ {
		transcript := NewToolCallTranscript()
		id := transcript.Register("call_benchmark", ToolTypeFunction, "lookup", `{"query":"athena"}`)
		startedAt := time.Now()
		transcript.Start(id, "lookup", `{"query":"athena"}`, startedAt)
		transcript.Finish(id, "lookup", `{"answer":"ok"}`, nil, startedAt.Add(time.Millisecond), time.Millisecond)
		_ = transcript.Snapshot()
	}
}
