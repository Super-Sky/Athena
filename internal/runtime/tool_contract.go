// tool_contract.go defines Athena's provider-neutral tool declarations and execution transcript.
// tool_contract.go 定义 Athena 与供应商无关的工具声明与执行转录。
package runtime

import (
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	// ToolTypeFunction names the function-style tool contract supported by the runtime.
	// ToolTypeFunction 表示 runtime 支持的函数式工具契约。
	ToolTypeFunction = "function"
)

// ToolFunctionDefinition describes one provider-neutral function tool.
// ToolFunctionDefinition 描述一份与供应商无关的函数工具定义。
type ToolFunctionDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

// ToolDefinition is the canonical runtime declaration converted from an app-facing tool schema.
// ToolDefinition 是从应用侧工具 schema 转换得到的 runtime 标准声明。
type ToolDefinition struct {
	Type     string                 `json:"type"`
	Function ToolFunctionDefinition `json:"function"`
	Metadata map[string]any         `json:"metadata,omitempty"`
}

// ToolCallStatus describes the current execution state of one canonical tool call.
// ToolCallStatus 描述一条标准工具调用当前的执行状态。
type ToolCallStatus string

const (
	// ToolCallStatusPending means the model declared the call but execution has not started.
	// ToolCallStatusPending 表示模型已声明调用，但执行尚未开始。
	ToolCallStatusPending ToolCallStatus = "pending"

	// ToolCallStatusRunning means the tool implementation is executing.
	// ToolCallStatusRunning 表示工具实现正在执行。
	ToolCallStatusRunning ToolCallStatus = "running"

	// ToolCallStatusCompleted means a correlated result was produced successfully.
	// ToolCallStatusCompleted 表示已成功产生并关联结果。
	ToolCallStatusCompleted ToolCallStatus = "completed"

	// ToolCallStatusFailed means execution or pre-dispatch processing failed.
	// ToolCallStatusFailed 表示执行或分发前处理失败。
	ToolCallStatusFailed ToolCallStatus = "failed"
)

// ToolResult correlates one tool output or error with its originating call.
// ToolResult 将一份工具输出或错误关联到其原始调用。
type ToolResult struct {
	ToolCallID string `json:"tool_call_id"`
	Name       string `json:"name,omitempty"`
	Content    string `json:"content,omitempty"`
	IsError    bool   `json:"is_error,omitempty"`
	Error      string `json:"error,omitempty"`
}

// ToolCall records one provider-neutral tool invocation and its correlated result.
// ToolCall 记录一条与供应商无关的工具调用及其关联结果。
type ToolCall struct {
	Index       int            `json:"index"`
	Round       int            `json:"round"`
	ID          string         `json:"id"`
	Type        string         `json:"type"`
	Name        string         `json:"name"`
	Arguments   string         `json:"arguments"`
	Status      ToolCallStatus `json:"status"`
	Result      *ToolResult    `json:"result,omitempty"`
	StartedAt   *time.Time     `json:"started_at,omitempty"`
	CompletedAt *time.Time     `json:"completed_at,omitempty"`
	DurationMS  int64          `json:"duration_ms,omitempty"`
}

// ToolCallTranscript collects one execution's calls while preserving declaration order.
// ToolCallTranscript 按声明顺序采集一次执行中的所有工具调用。
type ToolCallTranscript struct {
	mu        sync.Mutex
	calls     []ToolCall
	byID      map[string]int
	nextIdx   int
	nextRound int
}

// NewToolCallTranscript creates an empty per-execution tool transcript.
// NewToolCallTranscript 创建一份空的单次执行工具转录。
func NewToolCallTranscript() *ToolCallTranscript {
	return &ToolCallTranscript{byID: make(map[string]int)}
}

// Register records a model-emitted tool call and returns its stable unique ID.
// Register 记录模型发出的工具调用，并返回稳定且唯一的 ID。
func (t *ToolCallTranscript) Register(id, callType, name, arguments string) string {
	return t.RegisterAtRound(t.BeginRound(), id, callType, name, arguments)
}

// BeginRound reserves the next assistant tool-call round.
// BeginRound 预留下一轮 assistant 工具调用轮次。
func (t *ToolCallTranscript) BeginRound() int {
	if t == nil {
		return 0
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	round := t.nextRound
	t.nextRound++
	return round
}

// RegisterAtRound records one call in an already reserved assistant round.
// RegisterAtRound 在已预留的 assistant 轮次中记录一条调用。
func (t *ToolCallTranscript) RegisterAtRound(round int, id, callType, name, arguments string) string {
	if t == nil {
		return stableToolCallID(id)
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ensureIndex()

	id = strings.TrimSpace(id)
	if id == "" {
		id = stableToolCallID("")
	}
	if _, exists := t.byID[id]; exists {
		id = stableToolCallID("")
	}
	callType = strings.TrimSpace(callType)
	if callType == "" {
		callType = ToolTypeFunction
	}
	call := ToolCall{
		Index:     t.nextIdx,
		Round:     round,
		ID:        id,
		Type:      callType,
		Name:      strings.TrimSpace(name),
		Arguments: arguments,
		Status:    ToolCallStatusPending,
	}
	t.nextIdx++
	t.byID[id] = len(t.calls)
	t.calls = append(t.calls, call)
	return id
}

// Start marks one registered call as running.
// Start 将一条已注册调用标记为执行中。
func (t *ToolCallTranscript) Start(id, name, arguments string, at time.Time) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	index := t.ensureCall(id, name, arguments)
	startedAt := at.UTC()
	t.calls[index].Status = ToolCallStatusRunning
	t.calls[index].StartedAt = &startedAt
}

// Finish correlates a successful or failed result to its originating call.
// Finish 将成功或失败结果关联到其原始调用。
func (t *ToolCallTranscript) Finish(id, name, content string, callErr error, at time.Time, duration time.Duration) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	index := t.ensureCall(id, name, "")
	completedAt := at.UTC()
	call := &t.calls[index]
	call.CompletedAt = &completedAt
	call.DurationMS = duration.Milliseconds()
	call.Result = &ToolResult{
		ToolCallID: call.ID,
		Name:       defaultString(strings.TrimSpace(name), call.Name),
		Content:    content,
	}
	if callErr != nil {
		call.Status = ToolCallStatusFailed
		call.Result.IsError = true
		call.Result.Error = callErr.Error()
		return
	}
	call.Status = ToolCallStatusCompleted
}

// ObserveResult correlates an actual tool-result message and preserves existing timing data.
// ObserveResult 关联真实工具结果消息，并保留已有计时数据。
func (t *ToolCallTranscript) ObserveResult(id, name, content string, at time.Time) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	index := t.ensureCall(id, name, "")
	call := &t.calls[index]
	if call.StartedAt == nil {
		startedAt := at.UTC()
		call.StartedAt = &startedAt
	}
	if call.CompletedAt == nil {
		completedAt := at.UTC()
		call.CompletedAt = &completedAt
	}
	if call.Status != ToolCallStatusFailed {
		call.Status = ToolCallStatusCompleted
	}
	if call.Result == nil {
		call.Result = &ToolResult{ToolCallID: call.ID}
	}
	call.Result.ToolCallID = call.ID
	call.Result.Name = defaultString(strings.TrimSpace(name), call.Name)
	call.Result.Content = content
}

// FailPending marks calls that could not reach tool middleware as failed.
// FailPending 将未能进入工具中间件的调用标记为失败。
func (t *ToolCallTranscript) FailPending(callErr error, at time.Time) {
	if t == nil || callErr == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	completedAt := at.UTC()
	for index := range t.calls {
		call := &t.calls[index]
		if call.Status == ToolCallStatusCompleted || call.Status == ToolCallStatusFailed {
			continue
		}
		call.Status = ToolCallStatusFailed
		call.CompletedAt = &completedAt
		call.Result = &ToolResult{
			ToolCallID: call.ID,
			Name:       call.Name,
			IsError:    true,
			Error:      callErr.Error(),
		}
	}
}

// Snapshot returns an immutable copy ordered by the model's call declarations.
// Snapshot 返回一份按模型调用声明排序的不可变副本。
func (t *ToolCallTranscript) Snapshot() []ToolCall {
	if t == nil {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]ToolCall, len(t.calls))
	for index, call := range t.calls {
		out[index] = call
		if call.Result != nil {
			result := *call.Result
			out[index].Result = &result
		}
	}
	return out
}

func (t *ToolCallTranscript) ensureIndex() {
	if t.byID == nil {
		t.byID = make(map[string]int)
	}
}

func (t *ToolCallTranscript) ensureCall(id, name, arguments string) int {
	t.ensureIndex()
	id = strings.TrimSpace(id)
	if index, ok := t.byID[id]; ok {
		return index
	}
	if id == "" {
		id = stableToolCallID("")
	}
	call := ToolCall{
		Index:     t.nextIdx,
		Round:     t.nextRound,
		ID:        id,
		Type:      ToolTypeFunction,
		Name:      strings.TrimSpace(name),
		Arguments: arguments,
		Status:    ToolCallStatusPending,
	}
	t.nextIdx++
	t.nextRound++
	t.byID[id] = len(t.calls)
	t.calls = append(t.calls, call)
	return len(t.calls) - 1
}

func stableToolCallID(id string) string {
	if id = strings.TrimSpace(id); id != "" {
		return id
	}
	return "call_" + strings.ReplaceAll(uuid.NewString(), "-", "")
}
