// errors.go defines the stable app-layer error types exposed to transport and waiting flows.
// errors.go 定义对 transport 和 waiting 路径稳定暴露的 app 层错误类型。
package app

import "moss/internal/session"

// PendingWaitError means the session is still blocked by an active waiting gap.
// PendingWaitError 表示当前 session 仍被一个活跃 waiting gap 阻塞。
type PendingWaitError struct {
	SessionID string
	Pending   *session.PendingState
	Queued    *session.DeferredMessage
	Dropped   *session.DeferredMessage
}

// Error returns a stable error string for waiting-gap protocol failures.
// Error 返回等待缺口协议失败时使用的稳定错误字符串。
func (e *PendingWaitError) Error() string {
	return "session is still waiting for supplemental information"
}

// newPendingWaitError snapshots the active pending state before it is sent back to transport.
// newPendingWaitError 会复制当前 pending 状态，避免回传过程中意外修改原 session 数据。
func newPendingWaitError(sessionID string, pending *session.PendingState) *PendingWaitError {
	if pending == nil {
		return &PendingWaitError{SessionID: sessionID}
	}
	copied := *pending
	copied.MissingFields = append([]string(nil), pending.MissingFields...)
	return &PendingWaitError{
		SessionID: sessionID,
		Pending:   &copied,
	}
}

// InvalidResumeTokenError reports that a resume token can no longer reopen the gap.
// InvalidResumeTokenError 表示某个 resume token 已无法重新打开原等待缺口。
type InvalidResumeTokenError struct {
	SessionID   string
	ResumeToken string
	Reason      string
}

// Error returns the transport-facing error message for an unusable resume token.
// Error 返回 resume token 无法继续使用时的传输层错误消息。
func (e *InvalidResumeTokenError) Error() string {
	return "resume token is invalid for the current session state"
}

// InvalidSessionError reports that one caller-supplied session_id cannot be used.
// InvalidSessionError 用于描述调用方提供的 session_id 当前不可用。
type InvalidSessionError struct {
	SessionID string
	Reason    string
}

// Error returns a stable transport-facing error message for unusable sessions.
// Error 返回 session 不可用时对传输层暴露的稳定错误消息。
func (e *InvalidSessionError) Error() string {
	switch e.Reason {
	case "archived":
		return "requested session is archived"
	default:
		return "requested session is not available"
	}
}

// InvalidTaskRequestError reports that the incoming task contract is incomplete or unsupported.
// InvalidTaskRequestError 表示传入任务契约不完整或使用了不受支持的任务类型。
type InvalidTaskRequestError struct {
	TaskType      string
	MissingFields []string
	Reason        string
}

// Error returns the stable transport-facing error message for invalid task requests.
// Error 返回无效任务请求对传输层暴露的稳定错误消息。
func (e *InvalidTaskRequestError) Error() string {
	switch e.Reason {
	case "unsupported_task_type":
		return "task_type is not supported"
	case "missing_required_fields":
		return "task request is missing required fields"
	default:
		return "task request is invalid"
	}
}
