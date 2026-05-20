// task.go normalizes inbound chat requests into the first-phase internal task model.
// task.go 负责把进入的 chat 请求归一化为第一阶段内部任务模型。
package app

import runtimetask "moss/internal/runtime/task"

// buildRuntimeTaskFromRequest normalizes the current app request into the first-phase internal task model.
// buildRuntimeTaskFromRequest 会把当前 app 请求归一化为第一阶段内部任务模型。
func buildRuntimeTaskFromRequest(requestID string, req ChatRequest) (*runtimetask.RuntimeTask, error) {
	var knownFacts map[string]string
	if req.Supplement != nil && len(req.Supplement.Data) > 0 {
		knownFacts = make(map[string]string, len(req.Supplement.Data))
		for key, value := range req.Supplement.Data {
			knownFacts[key] = value
		}
	}
	task, err := runtimetask.NormalizeRequest(requestID, runtimetask.NormalizationInput{
		TaskType:              req.TaskType,
		TaskSubtype:           req.TaskSubtype,
		WorkspaceID:           req.WorkspaceID,
		MainSessionID:         req.MainSessionID,
		AppInstanceID:         req.AppInstanceID,
		AppSessionID:          req.AppSessionID,
		IntegrationInstanceID: req.IntegrationInstanceID,
		WorkflowRunID:         req.WorkflowRunID,
		StepID:                req.StepID,
		TriggerType:           req.TriggerType,
		AutomationTaskID:      req.AutomationTaskID,
		UserLanguage:          req.UserLanguage,
		Scene:                 req.Scene,
		Query:                 req.Query,
		DesiredOutputMode:     req.DesiredOutputMode,
		GlobalContext:         req.GlobalContext,
		AppContext:            req.AppContext,
		InputPayload:          req.InputPayload,
		SupplementalFacts:     knownFacts,
	})
	if err != nil {
		taskErr := &InvalidTaskRequestError{
			TaskType: req.TaskType,
			Reason:   "invalid_task_request",
		}
		if validationErr, ok := err.(*runtimetask.ValidationError); ok {
			taskErr.TaskType = validationErr.TaskType
			taskErr.Reason = validationErr.Reason
			taskErr.MissingFields = append([]string(nil), validationErr.MissingFields...)
		}
		return nil, taskErr
	}
	return task, nil
}
