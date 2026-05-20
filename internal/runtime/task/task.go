// task.go defines the generic runtime task model and chat normalization entry.
// task.go 定义通用 runtime task 模型及 chat 归一化入口。
package task

import (
	"fmt"
	"sort"
	"strings"
)

const (
	// InputKindChat marks one task normalized from the current chat-style request path.
	// InputKindChat 表示该任务由当前 chat 风格请求路径归一化而来。
	InputKindChat = "chat"

	// InputKindInspectionTask is a legacy-compatible task type that can become registered semantics later.
	// InputKindInspectionTask 是兼容旧调用的任务类型，后续可收敛为注册式语义。
	InputKindInspectionTask = "inspection_task"

	// InputKindIntegrationEvent is a legacy-compatible task type that can become registered semantics later.
	// InputKindIntegrationEvent 是兼容旧调用的任务类型，后续可收敛为注册式语义。
	InputKindIntegrationEvent = "integration_event"

	// InputKindScheduledJob is a legacy-compatible task type that can become registered semantics later.
	// InputKindScheduledJob 是兼容旧调用的任务类型，后续可收敛为注册式语义。
	InputKindScheduledJob = "scheduled_job"

	// InputKindWorkflowStepRequest is a legacy-compatible task type that can become registered semantics later.
	// InputKindWorkflowStepRequest 是兼容旧调用的任务类型，后续可收敛为注册式语义。
	InputKindWorkflowStepRequest = "workflow_step_request"

	// DefaultOutputModeText keeps the first phase aligned with the current text-first runtime output path.
	// DefaultOutputModeText 表示第一阶段继续沿用当前以文本为主的运行时输出路径。
	DefaultOutputModeText = "text"
)

// NormalizationInput captures the transport/app fields needed to build one internal runtime task.
// NormalizationInput 描述构建内部 runtime task 时需要的 transport/app 输入字段。
type NormalizationInput struct {
	TaskType              string
	TaskSubtype           string
	WorkspaceID           string
	MainSessionID         string
	AppInstanceID         string
	AppSessionID          string
	IntegrationInstanceID string
	WorkflowRunID         string
	StepID                string
	TriggerType           string
	AutomationTaskID      string
	UserLanguage          string
	Scene                 string
	Query                 string
	DesiredOutputMode     string
	GlobalContext         map[string]any
	AppContext            map[string]any
	InputPayload          map[string]any
	SupplementalFacts     map[string]string
}

// ValidationError reports that the normalized task input crosses a universal runtime boundary.
// ValidationError 表示归一化任务输入不满足通用 runtime 边界。
type ValidationError struct {
	TaskType      string
	Reason        string
	MissingFields []string
}

// Error returns the stable validation error message for task normalization failures.
// Error 返回任务归一化失败时使用的稳定校验错误消息。
func (e *ValidationError) Error() string {
	switch e.Reason {
	case "unsupported_task_type":
		return fmt.Sprintf("unsupported task_type %q", e.TaskType)
	case "missing_required_fields":
		return fmt.Sprintf("task_type %q is missing required fields: %s", e.TaskType, strings.Join(e.MissingFields, ", "))
	default:
		return "task normalization failed"
	}
}

// RuntimeTask is the minimal internal task model normalized from one incoming request.
// RuntimeTask 表示从一次传入请求归一化得到的最小内部 runtime task 模型。
type RuntimeTask struct {
	TaskID                string
	TaskType              string
	TaskSubtype           string
	InputKind             string
	Scene                 string
	WorkspaceID           string
	MainSessionID         string
	AppInstanceID         string
	AppSessionID          string
	IntegrationInstanceID string
	WorkflowRunID         string
	StepID                string
	TriggerType           string
	AutomationTaskID      string
	UserLanguage          string
	UserGoal              string
	KnownFacts            map[string]string
	MissingFacts          []string
	Constraints           map[string]string
	GlobalContext         map[string]any
	AppContext            map[string]any
	InputPayload          map[string]any
	OutputMode            string
}

// SecurityTask is a short-lived compatibility alias for callers that still import the old name.
// SecurityTask 是兼容旧调用方的短期别名，新代码应使用 RuntimeTask。
type SecurityTask = RuntimeTask

// NormalizeChatRequest builds one first-phase internal task from the current chat request path.
// NormalizeChatRequest 会从当前 chat 请求路径构建一个第一阶段内部任务对象。
func NormalizeChatRequest(requestID string, query string, supplementalFacts map[string]string) *RuntimeTask {
	task, err := NormalizeRequest(requestID, NormalizationInput{
		TaskType:          InputKindChat,
		Query:             query,
		SupplementalFacts: supplementalFacts,
	})
	if err != nil {
		return nil
	}
	return task
}

// NormalizeRequest builds one first-phase internal task from the unified task input contract.
// NormalizeRequest 会从统一任务输入契约构建一个第一阶段内部任务对象。
func NormalizeRequest(requestID string, input NormalizationInput) (*RuntimeTask, error) {
	taskType := normalizeTaskType(input.TaskType)
	if taskType == "" {
		taskType = InputKindChat
	}

	missingFields := validateRequiredFields(taskType, input)
	if len(missingFields) > 0 {
		return nil, &ValidationError{
			TaskType:      taskType,
			Reason:        "missing_required_fields",
			MissingFields: append([]string(nil), missingFields...),
		}
	}

	knownFacts := cloneStringMap(input.SupplementalFacts)

	goal := deriveTaskGoal(taskType, input)
	outputMode := strings.TrimSpace(input.DesiredOutputMode)
	if outputMode == "" {
		outputMode = DefaultOutputModeText
	}

	taskSubtype := strings.TrimSpace(input.TaskSubtype)
	if taskSubtype == "" {
		taskSubtype = deriveTaskSubtype(taskType, input)
	}

	scene := strings.TrimSpace(input.Scene)
	if scene == "" {
		scene = deriveScene(taskType, input)
	}

	constraints := map[string]string{
		"entry_path": taskType,
	}
	if requestID != "" {
		constraints["request_id"] = requestID
	}
	if strings.TrimSpace(input.MainSessionID) != "" {
		constraints["main_session_id"] = strings.TrimSpace(input.MainSessionID)
	}

	return &RuntimeTask{
		TaskID:                taskIDFromRequest(requestID, taskType, input, goal),
		TaskType:              taskType,
		TaskSubtype:           taskSubtype,
		InputKind:             taskType,
		Scene:                 scene,
		WorkspaceID:           strings.TrimSpace(input.WorkspaceID),
		MainSessionID:         strings.TrimSpace(input.MainSessionID),
		AppInstanceID:         strings.TrimSpace(input.AppInstanceID),
		AppSessionID:          strings.TrimSpace(input.AppSessionID),
		IntegrationInstanceID: strings.TrimSpace(input.IntegrationInstanceID),
		WorkflowRunID:         strings.TrimSpace(input.WorkflowRunID),
		StepID:                strings.TrimSpace(input.StepID),
		TriggerType:           strings.TrimSpace(input.TriggerType),
		AutomationTaskID:      strings.TrimSpace(input.AutomationTaskID),
		UserLanguage:          strings.TrimSpace(input.UserLanguage),
		UserGoal:              goal,
		KnownFacts:            knownFacts,
		MissingFacts:          nil,
		Constraints:           constraints,
		GlobalContext:         cloneAnyMap(input.GlobalContext),
		AppContext:            cloneAnyMap(input.AppContext),
		InputPayload:          cloneAnyMap(input.InputPayload),
		OutputMode:            outputMode,
	}, nil
}

func validateRequiredFields(_ string, _ NormalizationInput) []string {
	// Phase 0 keeps core normalization at universal runtime boundaries.
	// Phase 0 只保留通用 runtime 边界，不在 core 中校验场景字段。
	return nil
}

func taskIDFromRequest(requestID string, taskType string, input NormalizationInput, goal string) string {
	if strings.TrimSpace(requestID) != "" {
		return strings.TrimSpace(requestID)
	}
	if strings.TrimSpace(input.WorkflowRunID) != "" && strings.TrimSpace(input.StepID) != "" {
		return fmt.Sprintf("%s:%s", strings.TrimSpace(input.WorkflowRunID), strings.TrimSpace(input.StepID))
	}
	if strings.TrimSpace(input.AutomationTaskID) != "" {
		return fmt.Sprintf("%s:%s", taskType, strings.TrimSpace(input.AutomationTaskID))
	}
	if strings.TrimSpace(input.IntegrationInstanceID) != "" {
		return fmt.Sprintf("%s:%s", taskType, strings.TrimSpace(input.IntegrationInstanceID))
	}
	goal = strings.TrimSpace(goal)
	if goal == "" {
		return "task-" + taskType
	}
	return fmt.Sprintf("task-%s-%x", taskType, goal)
}

func normalizeTaskType(value string) string {
	return strings.TrimSpace(value)
}

func deriveTaskGoal(taskType string, input NormalizationInput) string {
	if goal := strings.TrimSpace(input.Query); goal != "" {
		return goal
	}
	switch taskType {
	case InputKindInspectionTask:
		return "run the requested inspection task"
	case InputKindIntegrationEvent:
		return "analyze the incoming integration event"
	case InputKindScheduledJob:
		return "execute the scheduled automation task"
	case InputKindWorkflowStepRequest:
		return "continue the workflow step request"
	default:
		return "continue the current runtime task"
	}
}

func deriveTaskSubtype(taskType string, input NormalizationInput) string {
	switch taskType {
	case InputKindChat:
		if strings.TrimSpace(input.AppInstanceID) != "" {
			return "app_dialogue"
		}
		return "default"
	case InputKindInspectionTask:
		return defaultString(strings.TrimSpace(input.TriggerType), "inspection")
	case InputKindIntegrationEvent:
		return defaultString(strings.TrimSpace(input.TriggerType), "integration_event")
	case InputKindScheduledJob:
		return "scheduled_automation"
	case InputKindWorkflowStepRequest:
		return "workflow_step"
	default:
		return taskType
	}
}

func deriveScene(taskType string, input NormalizationInput) string {
	if strings.TrimSpace(input.Scene) != "" {
		return strings.TrimSpace(input.Scene)
	}
	switch taskType {
	case InputKindChat:
		if strings.TrimSpace(input.AppInstanceID) != "" {
			return "application_dialogue"
		}
		return "default"
	case InputKindInspectionTask:
		return "inspection"
	case InputKindIntegrationEvent:
		return "alerts"
	case InputKindScheduledJob:
		return "workflow"
	case InputKindWorkflowStepRequest:
		return "workflow"
	default:
		return taskType
	}
}

func cloneStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	keys := make([]string, 0, len(input))
	for key := range input {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make(map[string]string, len(input))
	for _, key := range keys {
		result[key] = input[key]
	}
	return result
}

func cloneAnyMap(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	keys := make([]string, 0, len(input))
	for key := range input {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make(map[string]any, len(input))
	for _, key := range keys {
		result[key] = input[key]
	}
	return result
}

func defaultString(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}
