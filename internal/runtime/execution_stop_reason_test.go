package runtime

import (
	"context"
	"errors"
	"testing"
)

func TestNormalizeExecutionStopReason(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		status   string
		err      error
		explicit ExecutionStopReason
		want     ExecutionStopReason
	}{
		{name: "completed", status: TaskRunStatusCompleted, want: ExecutionStopSuccess},
		{name: "waiting", status: string(RequestStatusWaitingForInformation), want: ExecutionStopAwaitingInput},
		{name: "persisted waiting", status: TaskRunStatusWaiting, want: ExecutionStopAwaitingInput},
		{name: "policy", status: string(RequestStatusPolicyRejected), want: ExecutionStopGovernanceDenied},
		{name: "policy with detail error", status: string(RequestStatusPolicyRejected), err: errors.New("policy detail"), want: ExecutionStopGovernanceDenied},
		{name: "timeout with detail error", status: string(RequestStatusTimedOut), err: errors.New("timeout detail"), want: ExecutionStopDeadlineExceeded},
		{name: "deadline", err: context.DeadlineExceeded, want: ExecutionStopDeadlineExceeded},
		{name: "cancelled", err: context.Canceled, want: ExecutionStopCancelled},
		{name: "generic error", err: errors.New("provider failed"), want: ExecutionStopUnrecoverableError},
		{name: "explicit budget", status: TaskRunStatusFailed, explicit: ExecutionStopBudgetExhausted, want: ExecutionStopBudgetExhausted},
		{name: "explicit external wait", explicit: ExecutionStopAwaitingExternalData, want: ExecutionStopAwaitingExternalData},
		{name: "unknown explicit ignored", status: TaskRunStatusCompleted, explicit: "custom", want: ExecutionStopSuccess},
		{name: "active has no stop reason", status: TaskRunStatusRunning, want: ""},
		{name: "unknown terminal fails closed", status: "provider_specific_failure", want: ExecutionStopUnrecoverableError},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := NormalizeExecutionStopReason(test.status, test.err, test.explicit); got != test.want {
				t.Fatalf("NormalizeExecutionStopReason() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestParseExecutionStopReason(t *testing.T) {
	t.Parallel()

	if got, ok := ParseExecutionStopReason(" success "); !ok || got != ExecutionStopSuccess {
		t.Fatalf("ParseExecutionStopReason() = %q, %t, want success, true", got, ok)
	}
	if got, ok := ParseExecutionStopReason("minimal_persistence_complete"); ok || got != "" {
		t.Fatalf("ParseExecutionStopReason() = %q, %t, want empty, false", got, ok)
	}
}
