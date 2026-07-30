// execution_stop_reason.go defines stable, provider-neutral execution stop semantics.
// execution_stop_reason.go 定义稳定且与 provider 无关的执行停止语义。
package runtime

import (
	"context"
	"errors"
	"strings"
)

// ExecutionStopReason is the stable reason why one runtime execution stopped.
// ExecutionStopReason 表示一次 runtime 执行停止的稳定原因。
type ExecutionStopReason string

const (
	ExecutionStopSuccess              ExecutionStopReason = "success"
	ExecutionStopBudgetExhausted      ExecutionStopReason = "budget_exhausted"
	ExecutionStopDeadlineExceeded     ExecutionStopReason = "deadline_exceeded"
	ExecutionStopAwaitingInput        ExecutionStopReason = "awaiting_input"
	ExecutionStopAwaitingExternalData ExecutionStopReason = "awaiting_external_data"
	ExecutionStopGovernanceDenied     ExecutionStopReason = "governance_denied"
	ExecutionStopCancelled            ExecutionStopReason = "cancelled"
	ExecutionStopUnrecoverableError   ExecutionStopReason = "unrecoverable_error"
)

// NormalizeExecutionStopReason maps internal statuses and errors onto the public runtime taxonomy.
// NormalizeExecutionStopReason 把内部状态和错误映射到公开 runtime taxonomy。
func NormalizeExecutionStopReason(status string, err error, explicit ExecutionStopReason) ExecutionStopReason {
	if _, ok := ParseExecutionStopReason(string(explicit)); ok {
		return explicit
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return ExecutionStopDeadlineExceeded
	}
	if errors.Is(err, context.Canceled) {
		return ExecutionStopCancelled
	}
	if errors.Is(err, ErrExecutionBudgetExceeded) {
		return ExecutionStopBudgetExhausted
	}
	switch strings.ToLower(strings.TrimSpace(status)) {
	case TaskRunStatusWaiting, string(RequestStatusWaitingForInformation), string(RequestStatusPendingHuman):
		return ExecutionStopAwaitingInput
	case string(RequestStatusTimedOut):
		return ExecutionStopDeadlineExceeded
	case string(RequestStatusPolicyRejected):
		return ExecutionStopGovernanceDenied
	case TaskRunStatusCancelled:
		return ExecutionStopCancelled
	}
	if err != nil {
		return ExecutionStopUnrecoverableError
	}
	switch strings.ToLower(strings.TrimSpace(status)) {
	case RuntimeTerminalStatusCompleted:
		return ExecutionStopSuccess
	case RuntimeTerminalStatusFailed, string(RequestStatusInvalidModel), string(RequestStatusInvalidResumeToken), string(RequestStatusMissingInformationUnresolved):
		return ExecutionStopUnrecoverableError
	case "", TaskRunStatusCreated, TaskRunStatusRunning, TaskRunStatusResumed:
		return ""
	default:
		return ExecutionStopUnrecoverableError
	}
}

// ParseExecutionStopReason accepts only values in the public runtime taxonomy.
// ParseExecutionStopReason 仅接受公开 runtime taxonomy 中的值。
func ParseExecutionStopReason(value string) (ExecutionStopReason, bool) {
	reason := ExecutionStopReason(strings.TrimSpace(value))
	switch reason {
	case ExecutionStopSuccess,
		ExecutionStopBudgetExhausted,
		ExecutionStopDeadlineExceeded,
		ExecutionStopAwaitingInput,
		ExecutionStopAwaitingExternalData,
		ExecutionStopGovernanceDenied,
		ExecutionStopCancelled,
		ExecutionStopUnrecoverableError:
		return reason, true
	default:
		return "", false
	}
}
