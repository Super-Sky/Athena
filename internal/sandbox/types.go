// types.go defines the minimal external sandbox reference boundary.
// types.go 定义最小 external sandbox reference 边界契约。
package sandbox

// ExternalSandboxRef identifies a controlled execution boundary outside the core runtime.
// ExternalSandboxRef 标识 core runtime 之外的一次受控执行边界。
type ExternalSandboxRef struct {
	RefID     string         `json:"ref_id"`
	Mode      string         `json:"mode"`
	Provider  string         `json:"provider"`
	Boundary  string         `json:"boundary"`
	Status    string         `json:"status"`
	Operation string         `json:"operation"`
	Resource  string         `json:"resource"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

// ExternalSandboxAuditSummary stores whitelist-safe audit details for the boundary.
// ExternalSandboxAuditSummary 保存边界执行的白名单安全审计摘要。
type ExternalSandboxAuditSummary struct {
	Summary              string            `json:"summary"`
	CredentialScope      string            `json:"credential_scope"`
	ContextScope         string            `json:"context_scope"`
	NestedExecution      string            `json:"nested_execution"`
	StateIntegrity       string            `json:"state_integrity"`
	AllowedOutputClasses []string          `json:"allowed_output_classes,omitempty"`
	SafeLabels           map[string]string `json:"safe_labels,omitempty"`
}

// ExternalSandboxStructuredResult is the safe result emitted by the boundary.
// ExternalSandboxStructuredResult 是边界产出的安全结构化结果。
type ExternalSandboxStructuredResult struct {
	Status        string         `json:"status"`
	ResultType    string         `json:"result_type"`
	Summary       string         `json:"summary"`
	Output        map[string]any `json:"output,omitempty"`
	RedactedInput map[string]any `json:"redacted_input,omitempty"`
}

// ExternalSandboxValidationResult combines the reference, audit, and structured result.
// ExternalSandboxValidationResult 组合 sandbox 引用、审计摘要和结构化结果。
type ExternalSandboxValidationResult struct {
	SandboxRef       ExternalSandboxRef              `json:"sandbox_ref"`
	StructuredResult ExternalSandboxStructuredResult `json:"structured_result"`
	AuditSummary     ExternalSandboxAuditSummary     `json:"audit_summary"`
	Projection       map[string]any                  `json:"projection,omitempty"`
}

// ValidationRequest carries only safe summaries into the external sandbox reference builder.
// ValidationRequest 只把安全摘要传入 external sandbox reference 构造器。
type ValidationRequest struct {
	RequestID          string
	RunID              string
	StepID             string
	Mode               string
	Provider           string
	Boundary           string
	Operation          string
	Resource           string
	ToolName           string
	GovernanceDecision string
	RiskLevel          string
	Signals            []string
	Metadata           map[string]any
}
