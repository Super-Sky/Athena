package memory

import (
	"strings"
	"testing"

	"moss/internal/session"
)

func TestPrepareHistoryWithPreservedContextPrependsContinuitySummary(t *testing.T) {
	preserved := &session.PreservedContext{
		Goal:           "show user profile",
		LastUserIntent: "continue profile investigation",
		MissingFields:  []string{"user_id"},
		Facts: map[string]string{
			"case_id": "case-1",
		},
		DegradeReason: "timeout_expired",
	}

	history := []session.Message{
		{Role: "user", Content: strings.Repeat("older ", 40)},
		{Role: "assistant", Content: strings.Repeat("assistant ", 30)},
		{Role: "user", Content: "recent-1"},
		{Role: "assistant", Content: "recent-2"},
		{Role: "user", Content: "recent-3"},
		{Role: "assistant", Content: "recent-4"},
		{Role: "user", Content: "recent-5"},
	}

	result := PrepareHistoryWithPreservedContext(history, preserved, ContextPolicy{
		EnableSummaryCompression: true,
		CompressionThreshold:     10,
	})
	if len(result) == 0 {
		t.Fatalf("expected preserved context message")
	}
	if !strings.Contains(result[0].Content, "Preserved continuity context") {
		t.Fatalf("unexpected preserved context message = %q", result[0].Content)
	}
	if !strings.Contains(result[0].Content, "Goal: show user profile") {
		t.Fatalf("expected preserved goal in summary = %q", result[0].Content)
	}
	if !strings.Contains(result[0].Content, "case_id=case-1") {
		t.Fatalf("expected preserved facts in summary = %q", result[0].Content)
	}
	if !strings.Contains(result[0].Content, "Missing fields: user_id") {
		t.Fatalf("expected missing fields in summary = %q", result[0].Content)
	}
}
