// execution.go defines Athena-side execution governance contracts and explicit execution-intent resolution.
// execution.go 定义 Athena 侧执行治理 contract 以及显式执行意图解析逻辑。
package runtime

import (
	"regexp"
	"strings"
)

// ExecutionMode captures the kind of execution Athena may request from the platform.
// ExecutionMode 描述 Athena 可能请求平台执行的动作类型。
type ExecutionMode string

const (
	// ExecutionModeReadonlyAnalysis means the platform should only inspect or analyze without side effects.
	// ExecutionModeReadonlyAnalysis 表示平台只做检查或分析，不执行副作用。
	ExecutionModeReadonlyAnalysis ExecutionMode = "readonly_analysis"

	// ExecutionModeCommandExecution means the platform should run one direct command.
	// ExecutionModeCommandExecution 表示平台应执行一条直接命令。
	ExecutionModeCommandExecution ExecutionMode = "command_execution"

	// ExecutionModeScriptExecution means the platform should run one referenced script.
	// ExecutionModeScriptExecution 表示平台应执行一份脚本引用。
	ExecutionModeScriptExecution ExecutionMode = "script_execution"
)

// ExecutionRiskLevel captures the governance risk level of one execution request.
// ExecutionRiskLevel 描述一次执行请求的治理风险等级。
type ExecutionRiskLevel string

const (
	// ExecutionRiskLevelLow means the execution request is low risk.
	// ExecutionRiskLevelLow 表示执行请求风险较低。
	ExecutionRiskLevelLow ExecutionRiskLevel = "low"

	// ExecutionRiskLevelMedium means the execution request should usually be confirmed first.
	// ExecutionRiskLevelMedium 表示执行请求通常需要先确认。
	ExecutionRiskLevelMedium ExecutionRiskLevel = "medium"

	// ExecutionRiskLevelHigh means the execution request should be denied by default.
	// ExecutionRiskLevelHigh 表示执行请求默认应被拒绝。
	ExecutionRiskLevelHigh ExecutionRiskLevel = "high"
)

// NetworkPolicy captures the network access level requested for one execution.
// NetworkPolicy 描述一次执行请求的网络访问等级。
type NetworkPolicy string

const (
	// NetworkPolicyDisabled blocks network access completely.
	// NetworkPolicyDisabled 表示完全禁止网络访问。
	NetworkPolicyDisabled NetworkPolicy = "disabled"

	// NetworkPolicyRestricted allows controlled or allowlisted network access.
	// NetworkPolicyRestricted 表示只允许受控或白名单网络访问。
	NetworkPolicyRestricted NetworkPolicy = "restricted"

	// NetworkPolicyEnabled allows normal outbound network access.
	// NetworkPolicyEnabled 表示允许普通外网访问。
	NetworkPolicyEnabled NetworkPolicy = "enabled"
)

// FilesystemPolicy captures the filesystem access level requested for one execution.
// FilesystemPolicy 描述一次执行请求的文件系统访问等级。
type FilesystemPolicy string

const (
	// FilesystemPolicyReadOnly means the execution should not mutate mounted files.
	// FilesystemPolicyReadOnly 表示执行过程不应改写挂载文件。
	FilesystemPolicyReadOnly FilesystemPolicy = "read_only"

	// FilesystemPolicyWorkspaceWrite means the execution may write inside the task workspace only.
	// FilesystemPolicyWorkspaceWrite 表示执行只允许写入任务工作区。
	FilesystemPolicyWorkspaceWrite FilesystemPolicy = "workspace_write"

	// FilesystemPolicyIsolatedWrite means the execution may write only into one isolated scratch area.
	// FilesystemPolicyIsolatedWrite 表示执行只允许写入隔离 scratch 区域。
	FilesystemPolicyIsolatedWrite FilesystemPolicy = "isolated_write"
)

// ExecutionArtifact captures one named artifact reference produced by platform execution.
// ExecutionArtifact 描述平台执行产出的一条具名产物引用。
type ExecutionArtifact struct {
	Name string `json:"name,omitempty"`
	Path string `json:"path,omitempty"`
	Kind string `json:"kind,omitempty"`
}

// SandboxViolation captures one platform-reported sandbox violation.
// SandboxViolation 描述一条由平台回传的沙盒违规记录。
type SandboxViolation struct {
	Code   string `json:"code,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// ExecutionIntent is the canonical Athena -> platform execution-governance contract.
// ExecutionIntent 表示标准 Athena -> 平台 执行治理 contract。
type ExecutionIntent struct {
	IntentID             string             `json:"intent_id,omitempty"`
	RequestID            string             `json:"request_id,omitempty"`
	SessionID            string             `json:"session_id,omitempty"`
	TaskType             string             `json:"task_type,omitempty"`
	Scene                string             `json:"scene,omitempty"`
	WorkflowRunID        string             `json:"workflow_run_id,omitempty"`
	StepID               string             `json:"step_id,omitempty"`
	ExecutionMode        ExecutionMode      `json:"execution_mode,omitempty"`
	Command              string             `json:"command,omitempty"`
	ScriptRef            string             `json:"script_ref,omitempty"`
	Arguments            []string           `json:"arguments,omitempty"`
	TimeoutSeconds       int                `json:"timeout_seconds,omitempty"`
	CPULimit             string             `json:"cpu_limit,omitempty"`
	MemoryLimitMB        int                `json:"memory_limit_mb,omitempty"`
	NetworkPolicy        NetworkPolicy      `json:"network_policy,omitempty"`
	FilesystemPolicy     FilesystemPolicy   `json:"filesystem_policy,omitempty"`
	EnvWhitelist         []string           `json:"env_whitelist,omitempty"`
	RiskLevel            ExecutionRiskLevel `json:"risk_level,omitempty"`
	RequiresConfirmation bool               `json:"requires_confirmation,omitempty"`
	DenyReason           string             `json:"deny_reason,omitempty"`
	Allowed              bool               `json:"allowed,omitempty"`
	Explanation          string             `json:"explanation,omitempty"`
}

// ExecutionResult is the canonical platform -> Athena execution result contract.
// ExecutionResult 表示标准平台 -> Athena 执行结果 contract。
type ExecutionResult struct {
	ExecutionID      string              `json:"execution_id,omitempty"`
	IntentID         string              `json:"intent_id,omitempty"`
	Status           string              `json:"status,omitempty"`
	ExitCode         int                 `json:"exit_code,omitempty"`
	Stdout           string              `json:"stdout,omitempty"`
	Stderr           string              `json:"stderr,omitempty"`
	Artifacts        []ExecutionArtifact `json:"artifacts,omitempty"`
	StartedAt        string              `json:"started_at,omitempty"`
	EndedAt          string              `json:"ended_at,omitempty"`
	TimedOut         bool                `json:"timed_out,omitempty"`
	SandboxViolation *SandboxViolation   `json:"sandbox_violation,omitempty"`
	ResourceUsage    map[string]any      `json:"resource_usage,omitempty"`
}

// ExecutionGovernanceInput captures the minimum input Athena needs to decide whether an execution intent should exist.
// ExecutionGovernanceInput 描述 Athena 决定是否生成执行意图所需的最小输入。
type ExecutionGovernanceInput struct {
	RequestID         string
	SessionID         string
	TaskType          string
	Scene             string
	WorkflowRunID     string
	StepID            string
	DesiredOutputMode string
	InputPayload      map[string]any
}

var (
	denyCommandPattern     = regexp.MustCompile(`(?i)(?:\brm\s+-rf\b|\bmkfs\b|\bshutdown\b|\breboot\b|curl\s*\|\s*(?:bash|sh)|ignore\s+previous\s+instructions|169\.254\.169\.254)`)
	sensitivePathPattern   = regexp.MustCompile(`(?i)(?:\.aws/credentials|\.ssh/id_rsa|authorized_keys|/etc/shadow|/proc/self/environ|kubeconfig|config\.json)`)
	networkCommandPattern  = regexp.MustCompile(`(?i)\b(?:curl|wget|nc|netcat|scp|rsync|ssh)\b`)
	writeCommandPattern    = regexp.MustCompile(`(?i)(?:\btee\b|\bmv\b|\bcp\b|\bchmod\b|\bchown\b|>>|>\s*[^\s])`)
	scriptExecutionPattern = regexp.MustCompile(`(?i)\b(?:bash|sh|python|python3|node)\b`)
)

// ResolveExecutionIntent returns one canonical execution intent only for explicit execution-governance scenarios.
// ResolveExecutionIntent 仅在显式执行治理场景下返回标准执行意图。
func ResolveExecutionIntent(input ExecutionGovernanceInput) *ExecutionIntent {
	request := extractExecutionRequest(input.InputPayload)
	if request == nil && !isExplicitExecutionMode(input.DesiredOutputMode) {
		return nil
	}
	if request == nil {
		request = map[string]any{}
	}

	command := stringValue(request["command"])
	scriptRef := stringValue(request["script_ref"])
	if command == "" && scriptRef == "" {
		return nil
	}

	mode := resolveExecutionMode(command, scriptRef)
	riskLevel, allowed, requiresConfirmation, denyReason, explanation := classifyExecutionRequest(command, scriptRef)
	timeoutSeconds := intValue(request["timeout_seconds"], 30)
	cpuLimit := defaultStringValue(stringValue(request["cpu_limit"]), "1")
	memoryLimitMB := intValue(request["memory_limit_mb"], 256)
	networkPolicy := resolveNetworkPolicy(command, request)
	filesystemPolicy := resolveFilesystemPolicy(command, scriptRef, request)

	intent := &ExecutionIntent{
		IntentID:             defaultStringValue(stringValue(request["intent_id"]), buildExecutionIntentID(input)),
		RequestID:            strings.TrimSpace(input.RequestID),
		SessionID:            strings.TrimSpace(input.SessionID),
		TaskType:             strings.TrimSpace(input.TaskType),
		Scene:                strings.TrimSpace(input.Scene),
		WorkflowRunID:        strings.TrimSpace(input.WorkflowRunID),
		StepID:               strings.TrimSpace(input.StepID),
		ExecutionMode:        mode,
		Command:              command,
		ScriptRef:            scriptRef,
		Arguments:            stringListValue(request["arguments"]),
		TimeoutSeconds:       timeoutSeconds,
		CPULimit:             cpuLimit,
		MemoryLimitMB:        memoryLimitMB,
		NetworkPolicy:        networkPolicy,
		FilesystemPolicy:     filesystemPolicy,
		EnvWhitelist:         stringListValue(request["env_whitelist"]),
		RiskLevel:            riskLevel,
		RequiresConfirmation: requiresConfirmation,
		DenyReason:           denyReason,
		Allowed:              allowed,
		Explanation:          explanation,
	}
	return intent
}

// ParseExecutionResult reads one platform-returned execution_result from input payload and normalizes its minimal shape.
// ParseExecutionResult 会从输入载荷中读取平台回传的 execution_result，并归一化其最小结构。
func ParseExecutionResult(inputPayload map[string]any) *ExecutionResult {
	if len(inputPayload) == 0 {
		return nil
	}
	raw, ok := inputPayload["execution_result"].(map[string]any)
	if !ok || len(raw) == 0 {
		return nil
	}
	result := &ExecutionResult{
		ExecutionID:   stringValue(raw["execution_id"]),
		IntentID:      stringValue(raw["intent_id"]),
		Status:        stringValue(raw["status"]),
		ExitCode:      intValue(raw["exit_code"], 0),
		Stdout:        stringValue(raw["stdout"]),
		Stderr:        stringValue(raw["stderr"]),
		StartedAt:     stringValue(raw["started_at"]),
		EndedAt:       stringValue(raw["ended_at"]),
		TimedOut:      boolValue(raw["timed_out"]),
		ResourceUsage: mapValue(raw["resource_usage"]),
	}
	if artifacts := artifactListValue(raw["artifacts"]); len(artifacts) > 0 {
		result.Artifacts = artifacts
	}
	if violationMap := mapValue(raw["sandbox_violation"]); len(violationMap) > 0 {
		result.SandboxViolation = &SandboxViolation{
			Code:   stringValue(violationMap["code"]),
			Reason: stringValue(violationMap["reason"]),
		}
	}
	if result.ExecutionID == "" && result.Status == "" && result.Stdout == "" && result.Stderr == "" && len(result.Artifacts) == 0 && result.SandboxViolation == nil {
		return nil
	}
	return result
}

func buildExecutionIntentID(input ExecutionGovernanceInput) string {
	if strings.TrimSpace(input.RequestID) != "" {
		return "intent-" + strings.TrimSpace(input.RequestID)
	}
	parts := compactStrings([]string{input.TaskType, input.WorkflowRunID, input.StepID})
	if len(parts) == 0 {
		return "intent-runtime"
	}
	return "intent-" + strings.Join(parts, "-")
}

func isExplicitExecutionMode(mode string) bool {
	switch strings.TrimSpace(mode) {
	case "execution_governance", "execution_intent", "execution_result":
		return true
	default:
		return false
	}
}

func extractExecutionRequest(inputPayload map[string]any) map[string]any {
	if len(inputPayload) == 0 {
		return nil
	}
	if nested, ok := inputPayload["execution_request"].(map[string]any); ok && len(nested) > 0 {
		return nested
	}
	return nil
}

func resolveExecutionMode(command, scriptRef string) ExecutionMode {
	switch {
	case strings.TrimSpace(scriptRef) != "":
		return ExecutionModeScriptExecution
	case strings.TrimSpace(command) != "":
		if scriptExecutionPattern.MatchString(command) {
			return ExecutionModeScriptExecution
		}
		return ExecutionModeCommandExecution
	default:
		return ExecutionModeReadonlyAnalysis
	}
}

func classifyExecutionRequest(command, scriptRef string) (ExecutionRiskLevel, bool, bool, string, string) {
	joined := strings.TrimSpace(command + " " + scriptRef)
	switch {
	case denyCommandPattern.MatchString(joined) || sensitivePathPattern.MatchString(joined):
		return ExecutionRiskLevelHigh, false, false, "high_risk_command_or_path", "the requested command or script matches a denied high-risk execution pattern"
	case networkCommandPattern.MatchString(joined):
		return ExecutionRiskLevelMedium, true, true, "", "the requested execution needs outbound network access and should be confirmed first"
	case writeCommandPattern.MatchString(joined) || strings.TrimSpace(scriptRef) != "" || scriptExecutionPattern.MatchString(command):
		return ExecutionRiskLevelMedium, true, true, "", "the requested execution can mutate files or run a referenced script and should be confirmed first"
	default:
		return ExecutionRiskLevelLow, true, false, "", "the requested execution is limited to low-risk inspection or bounded command execution"
	}
}

func resolveNetworkPolicy(command string, request map[string]any) NetworkPolicy {
	if policy := strings.TrimSpace(stringValue(request["network_policy"])); policy != "" {
		return NetworkPolicy(policy)
	}
	if networkCommandPattern.MatchString(command) {
		return NetworkPolicyRestricted
	}
	return NetworkPolicyDisabled
}

func resolveFilesystemPolicy(command, scriptRef string, request map[string]any) FilesystemPolicy {
	if policy := strings.TrimSpace(stringValue(request["filesystem_policy"])); policy != "" {
		return FilesystemPolicy(policy)
	}
	if strings.TrimSpace(scriptRef) != "" {
		return FilesystemPolicyWorkspaceWrite
	}
	if writeCommandPattern.MatchString(command) {
		return FilesystemPolicyWorkspaceWrite
	}
	return FilesystemPolicyReadOnly
}

func stringListValue(value any) []string {
	switch items := value.(type) {
	case []string:
		return compactStrings(items)
	case []any:
		result := make([]string, 0, len(items))
		for _, item := range items {
			if text := strings.TrimSpace(stringValue(item)); text != "" {
				result = append(result, text)
			}
		}
		return compactStrings(result)
	default:
		return nil
	}
}

func artifactListValue(value any) []ExecutionArtifact {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	result := make([]ExecutionArtifact, 0, len(items))
	for _, item := range items {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		result = append(result, ExecutionArtifact{
			Name: stringValue(entry["name"]),
			Path: stringValue(entry["path"]),
			Kind: stringValue(entry["kind"]),
		})
	}
	return result
}

func mapValue(value any) map[string]any {
	mapped, ok := value.(map[string]any)
	if !ok || len(mapped) == 0 {
		return nil
	}
	result := make(map[string]any, len(mapped))
	for key, item := range mapped {
		result[key] = item
	}
	return result
}

func intValue(value any, fallback int) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return fallback
	}
}

func boolValue(value any) bool {
	typed, ok := value.(bool)
	return ok && typed
}

func defaultStringValue(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}
