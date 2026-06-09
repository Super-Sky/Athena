// runtime_contract_controls.go applies RuntimeContract fields onto execution constraints.
// runtime_contract_controls.go 负责把 RuntimeContract 字段映射到执行约束。
package runtime

import (
	"fmt"
	"strings"
)

const (
	defaultAgentCancelMode = "safe_point"
)

// AgentRuntimeControlSettings captures the normalized runtime control knobs used by the executor.
// AgentRuntimeControlSettings 表示执行器使用的规范化 runtime 控制参数。
type AgentRuntimeControlSettings struct {
	CancelEnabled           bool
	CancelMode              string
	CancelCheckpointEnabled bool
	ModelRetryMaxAttempts   int
	ModelFailoverEnabled    bool
	AfterAgentEnabled       bool
	AfterAgentSummaryMode   string
}

// ApplyResolvedRuntimeContract projects one resolved runtime contract and task type into execution constraints.
// ApplyResolvedRuntimeContract 把解析后的 runtime contract 与 task type 投影到执行约束中。
func ApplyResolvedRuntimeContract(spec *ExecutionSpec, contract *RuntimeContract, taskType *TaskTypeRegistration) {
	if spec == nil || contract == nil {
		return
	}
	if spec.Metadata.Constraints == nil {
		spec.Metadata.Constraints = map[string]any{}
	}
	spec.Metadata.Constraints["runtime_contract_id"] = strings.TrimSpace(contract.ID)
	spec.Metadata.Constraints["contract_id"] = strings.TrimSpace(contract.ID)
	spec.Metadata.Constraints["runtime_contract_name"] = strings.TrimSpace(contract.Name)
	spec.Metadata.Constraints["runtime_contract_version"] = strings.TrimSpace(contract.Version)
	spec.Metadata.Constraints["runtime_contract_status"] = strings.TrimSpace(contract.Status)
	spec.Metadata.Constraints["runtime_execution_profile"] = cloneRuntimeContractMap(contract.ExecutionProfile)
	spec.Metadata.Constraints["runtime_exit_policy"] = cloneRuntimeContractMap(contract.ExitPolicy)
	spec.Metadata.Constraints["runtime_projection_policy"] = cloneRuntimeContractMap(contract.ProjectionPolicy)
	if taskType != nil {
		spec.Metadata.Constraints["runtime_task_type_registration_id"] = strings.TrimSpace(taskType.ID)
		spec.Metadata.Constraints["runtime_task_type_status"] = strings.TrimSpace(taskType.Status)
	}
	controls := resolveAgentRuntimeControlsFromContract(*contract)
	spec.Metadata.Constraints["runtime_cancel_enabled"] = controls.CancelEnabled
	spec.Metadata.Constraints["runtime_cancel_mode"] = controls.CancelMode
	spec.Metadata.Constraints["runtime_cancel_checkpoint_enabled"] = controls.CancelCheckpointEnabled
	spec.Metadata.Constraints["runtime_model_retry_max_attempts"] = controls.ModelRetryMaxAttempts
	spec.Metadata.Constraints["runtime_model_failover_enabled"] = controls.ModelFailoverEnabled
	spec.Metadata.Constraints["runtime_after_agent_enabled"] = controls.AfterAgentEnabled
	spec.Metadata.Constraints["runtime_after_agent_summary_mode"] = controls.AfterAgentSummaryMode
}

// ResolveAgentRuntimeControlSettings reads runtime controls from execution constraints.
// ResolveAgentRuntimeControlSettings 从执行约束读取 runtime 控制参数。
func ResolveAgentRuntimeControlSettings(spec *ExecutionSpec) AgentRuntimeControlSettings {
	settings := AgentRuntimeControlSettings{
		CancelEnabled:           true,
		CancelMode:              defaultAgentCancelMode,
		CancelCheckpointEnabled: true,
		ModelRetryMaxAttempts:   0,
		ModelFailoverEnabled:    true,
		AfterAgentEnabled:       false,
		AfterAgentSummaryMode:   "terminal_projection",
	}
	if spec == nil || spec.Metadata.Constraints == nil {
		return settings
	}
	constraints := spec.Metadata.Constraints
	settings.CancelEnabled = anyBoolValue(constraints["runtime_cancel_enabled"], settings.CancelEnabled)
	settings.CancelMode = defaultString(anyStringValue(constraints["runtime_cancel_mode"]), settings.CancelMode)
	settings.CancelCheckpointEnabled = anyBoolValue(constraints["runtime_cancel_checkpoint_enabled"], settings.CancelCheckpointEnabled)
	settings.ModelRetryMaxAttempts = clampRetryAttempts(anyIntValue(constraints["runtime_model_retry_max_attempts"], settings.ModelRetryMaxAttempts))
	settings.ModelFailoverEnabled = anyBoolValue(constraints["runtime_model_failover_enabled"], settings.ModelFailoverEnabled)
	settings.AfterAgentEnabled = anyBoolValue(constraints["runtime_after_agent_enabled"], settings.AfterAgentEnabled)
	settings.AfterAgentSummaryMode = defaultString(anyStringValue(constraints["runtime_after_agent_summary_mode"]), settings.AfterAgentSummaryMode)
	return settings
}

func resolveAgentRuntimeControlsFromContract(contract RuntimeContract) AgentRuntimeControlSettings {
	settings := AgentRuntimeControlSettings{
		CancelEnabled:           true,
		CancelMode:              defaultAgentCancelMode,
		CancelCheckpointEnabled: true,
		ModelRetryMaxAttempts:   0,
		ModelFailoverEnabled:    true,
		AfterAgentEnabled:       false,
		AfterAgentSummaryMode:   "terminal_projection",
	}
	profile := cloneRuntimeContractMap(contract.ExecutionProfile)
	exitPolicy := cloneRuntimeContractMap(contract.ExitPolicy)
	controls := anyMapValue(profile["agent_runtime_controls"])

	cancelPolicy := anyMapValue(controls["cancel"])
	if len(cancelPolicy) == 0 {
		cancelPolicy = anyMapValue(profile["cancel_policy"])
	}
	settings.CancelEnabled = anyBoolValue(cancelPolicy["enabled"], settings.CancelEnabled)
	settings.CancelMode = defaultString(anyStringValue(cancelPolicy["mode"]), settings.CancelMode)
	settings.CancelCheckpointEnabled = anyBoolValue(cancelPolicy["checkpoint_enabled"], settings.CancelCheckpointEnabled)
	settings.CancelCheckpointEnabled = anyBoolValue(cancelPolicy["checkpoint_on_cancel"], settings.CancelCheckpointEnabled)

	retryPolicy := anyMapValue(controls["model_retry"])
	if len(retryPolicy) == 0 {
		retryPolicy = anyMapValue(profile["model_retry"])
	}
	retryFromExecution := anyIntValue(retryPolicy["max_attempts"], settings.ModelRetryMaxAttempts)
	retryFromExitPolicy := anyIntValue(anyMapValue(exitPolicy["model_retry"])["max_attempts"], retryFromExecution)
	settings.ModelRetryMaxAttempts = clampRetryAttempts(retryFromExitPolicy)

	failoverPolicy := anyMapValue(controls["model_failover"])
	if len(failoverPolicy) == 0 {
		failoverPolicy = anyMapValue(profile["model_failover"])
	}
	settings.ModelFailoverEnabled = anyBoolValue(failoverPolicy["enabled"], settings.ModelFailoverEnabled)

	afterAgent := anyMapValue(controls["after_agent"])
	if len(afterAgent) == 0 {
		afterAgent = anyMapValue(profile["after_agent"])
	}
	settings.AfterAgentEnabled = anyBoolValue(afterAgent["enabled"], settings.AfterAgentEnabled)
	settings.AfterAgentSummaryMode = defaultString(anyStringValue(afterAgent["summary_mode"]), settings.AfterAgentSummaryMode)
	return settings
}

func cloneRuntimeContractMap(input map[string]any) map[string]any {
	if len(input) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(input))
	for key, value := range input {
		trimmed := strings.TrimSpace(key)
		if trimmed == "" {
			continue
		}
		out[trimmed] = value
	}
	return out
}

func anyMapValue(value any) map[string]any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneRuntimeContractMap(typed)
	case map[string]string:
		out := make(map[string]any, len(typed))
		for key, child := range typed {
			if strings.TrimSpace(key) == "" {
				continue
			}
			out[key] = child
		}
		return out
	default:
		return map[string]any{}
	}
}

func anyBoolValue(value any, fallback bool) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "true", "1", "yes", "on":
			return true
		case "false", "0", "no", "off":
			return false
		default:
			return fallback
		}
	default:
		return fallback
	}
}

func anyStringValue(value any) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func anyIntValue(value any, fallback int) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case float32:
		return int(typed)
	case string:
		var parsed int
		if _, err := fmt.Sscanf(strings.TrimSpace(typed), "%d", &parsed); err == nil {
			return parsed
		}
	}
	return fallback
}

func clampRetryAttempts(value int) int {
	if value < 0 {
		return 0
	}
	if value > 5 {
		return 5
	}
	return value
}
