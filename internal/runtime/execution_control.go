// execution_control.go defines provider-neutral goal criteria and hard execution budgets.
// execution_control.go 定义与 provider 无关的目标条件和强制执行预算。
package runtime

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	runtimetask "moss/internal/runtime/task"
)

const (
	defaultExecutionSafetyMaxRunSteps = 64
	maxExecutionDurationMS            = int64((24 * time.Hour) / time.Millisecond)
	maxExecutionModelCalls            = int64(100)
	maxExecutionToolCalls             = int64(1000)
	maxExecutionTokens                = int64(10_000_000)
)

// ErrExecutionBudgetExceeded marks a run stopped by one configured budget dimension.
// ErrExecutionBudgetExceeded 表示 run 因某个已配置预算维度耗尽而停止。
var ErrExecutionBudgetExceeded = errors.New("execution budget exceeded")

// ErrExecutionTokenUsageUnavailable prevents max_tokens from silently failing open.
// ErrExecutionTokenUsageUnavailable 防止 max_tokens 在 provider 不返回 usage 时静默放行。
var ErrExecutionTokenUsageUnavailable = errors.New("execution token usage unavailable")

// ExecutionBudget contains the generic optional limits enforced by Athena runtime.
// ExecutionBudget 包含由 Athena runtime 强制执行的通用可选限制。
type ExecutionBudget struct {
	MaxDurationMS int64      `json:"max_duration_ms,omitempty"`
	DeadlineAt    *time.Time `json:"deadline_at,omitempty"`
	MaxModelCalls int        `json:"max_model_calls,omitempty"`
	MaxToolCalls  int        `json:"max_tool_calls,omitempty"`
	MaxTokens     int64      `json:"max_tokens,omitempty"`
}

// ExecutionControlPlan is the goal-directed control contract attached to one execution spec.
// ExecutionControlPlan 是挂载到单次 execution spec 的目标驱动控制契约。
type ExecutionControlPlan struct {
	SuccessCriteria []string        `json:"success_criteria,omitempty"`
	Budget          ExecutionBudget `json:"budget,omitempty"`
}

// ExecutionBudgetExceededError identifies the exhausted dimension without exposing provider internals.
// ExecutionBudgetExceededError 标识耗尽的预算维度，但不暴露 provider 内部信息。
type ExecutionBudgetExceededError struct {
	Dimension string
	Limit     int64
	Observed  int64
}

// Error returns a stable operator-readable budget failure.
// Error 返回稳定且便于运维识别的预算失败信息。
func (e *ExecutionBudgetExceededError) Error() string {
	return fmt.Sprintf("%s: dimension=%s limit=%d observed=%d", ErrExecutionBudgetExceeded, strings.TrimSpace(e.Dimension), e.Limit, e.Observed)
}

// Unwrap allows errors.Is to classify all dimensions as budget exhaustion.
// Unwrap 允许 errors.Is 将所有维度统一识别为预算耗尽。
func (e *ExecutionBudgetExceededError) Unwrap() error {
	return ErrExecutionBudgetExceeded
}

// ParseExecutionBudget validates and normalizes the public Agent Run budget object.
// ParseExecutionBudget 校验并规范化公开 Agent Run budget 对象。
func ParseExecutionBudget(input map[string]any) (ExecutionBudget, error) {
	if len(input) == 0 {
		return ExecutionBudget{}, nil
	}
	allowed := map[string]struct{}{
		"max_duration_ms": {},
		"deadline_at":     {},
		"max_model_calls": {},
		"max_tool_calls":  {},
		"max_tokens":      {},
	}
	for key := range input {
		if _, ok := allowed[key]; !ok {
			return ExecutionBudget{}, fmt.Errorf("budget.%s is not supported", key)
		}
	}
	var budget ExecutionBudget
	var err error
	if budget.MaxDurationMS, err = positiveBudgetInt64(input, "max_duration_ms", maxExecutionDurationMS); err != nil {
		return ExecutionBudget{}, err
	}
	modelCalls, err := positiveBudgetInt64(input, "max_model_calls", maxExecutionModelCalls)
	if err != nil {
		return ExecutionBudget{}, err
	}
	budget.MaxModelCalls = int(modelCalls)
	toolCalls, err := positiveBudgetInt64(input, "max_tool_calls", maxExecutionToolCalls)
	if err != nil {
		return ExecutionBudget{}, err
	}
	budget.MaxToolCalls = int(toolCalls)
	if budget.MaxTokens, err = positiveBudgetInt64(input, "max_tokens", maxExecutionTokens); err != nil {
		return ExecutionBudget{}, err
	}
	if raw, exists := input["deadline_at"]; exists && raw != nil {
		value, ok := raw.(string)
		if !ok || strings.TrimSpace(value) == "" {
			return ExecutionBudget{}, fmt.Errorf("budget.deadline_at must be an RFC3339 timestamp")
		}
		deadline, parseErr := time.Parse(time.RFC3339Nano, strings.TrimSpace(value))
		if parseErr != nil {
			return ExecutionBudget{}, fmt.Errorf("budget.deadline_at must be an RFC3339 timestamp: %w", parseErr)
		}
		deadline = deadline.UTC()
		budget.DeadlineAt = &deadline
	}
	return budget, nil
}

// ResolveExecutionControlPlan extracts the app-neutral Agent Run controls from one normalized task.
// ResolveExecutionControlPlan 从规范化 task 中提取与应用无关的 Agent Run 控制契约。
func ResolveExecutionControlPlan(task *runtimetask.RuntimeTask) (ExecutionControlPlan, error) {
	if task == nil {
		return ExecutionControlPlan{}, nil
	}
	contract := anyMapValue(task.InputPayload["agent_run"])
	budget, err := ParseExecutionBudget(anyMapValue(contract["budget"]))
	if err != nil {
		return ExecutionControlPlan{}, err
	}
	return ExecutionControlPlan{
		SuccessCriteria: compactExecutionCriteria(contract["success_criteria"]),
		Budget:          budget,
	}, nil
}

// EffectiveDeadline returns the earlier deadline derived from duration and absolute limits.
// EffectiveDeadline 返回由持续时间与绝对时间限制共同决定的较早 deadline。
func (b ExecutionBudget) EffectiveDeadline(now time.Time) (time.Time, bool) {
	var deadline time.Time
	if b.MaxDurationMS > 0 {
		deadline = now.Add(time.Duration(b.MaxDurationMS) * time.Millisecond)
	}
	if b.DeadlineAt != nil && (deadline.IsZero() || b.DeadlineAt.Before(deadline)) {
		deadline = b.DeadlineAt.UTC()
	}
	return deadline, !deadline.IsZero()
}

// MaxRunSteps converts explicit call budgets into an Eino graph safety ceiling.
// MaxRunSteps 将显式调用预算转换为 Eino graph 的安全上限。
func (b ExecutionBudget) MaxRunSteps() int {
	steps := 0
	if b.MaxModelCalls > 0 {
		steps = b.MaxModelCalls*2 + 2
	}
	if b.MaxToolCalls > 0 {
		toolSteps := b.MaxToolCalls*2 + 3
		if steps == 0 || toolSteps < steps {
			steps = toolSteps
		}
	}
	if steps == 0 {
		return defaultExecutionSafetyMaxRunSteps
	}
	return min(steps, 2048)
}

func positiveBudgetInt64(input map[string]any, key string, maximum int64) (int64, error) {
	raw, exists := input[key]
	if !exists || raw == nil {
		return 0, nil
	}
	value, ok := exactInt64(raw)
	if !ok || value <= 0 || value > maximum {
		return 0, fmt.Errorf("budget.%s must be an integer between 1 and %d", key, maximum)
	}
	return value, nil
}

func exactInt64(value any) (int64, bool) {
	switch typed := value.(type) {
	case int:
		return int64(typed), true
	case int64:
		return typed, true
	case float64:
		if math.Trunc(typed) != typed || typed > math.MaxInt64 || typed < math.MinInt64 {
			return 0, false
		}
		return int64(typed), true
	case json.Number:
		parsed, err := typed.Int64()
		return parsed, err == nil
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func compactExecutionCriteria(value any) []string {
	var raw []string
	switch typed := value.(type) {
	case []string:
		raw = typed
	case []any:
		for _, item := range typed {
			if text, ok := item.(string); ok {
				raw = append(raw, text)
			}
		}
	}
	seen := map[string]struct{}{}
	result := make([]string, 0, len(raw))
	for _, item := range raw {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		key := strings.ToLower(item)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, item)
	}
	return result
}
