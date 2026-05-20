// types.go defines the lightweight validation MCP contract used by the control-plane validation surface.
// types.go 定义控制面验证界面使用的轻量 validation MCP 契约。
package validationmcp

// ServerInfo captures the deterministic validation MCP server identity and advertised tools.
// ServerInfo 描述确定性 validation MCP server 的身份和对外声明的工具。
type ServerInfo struct {
	ServerID  string         `json:"server_id"`
	Name      string         `json:"name"`
	Transport string         `json:"transport"`
	Status    string         `json:"status"`
	Tools     []ToolSchema   `json:"tools,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

// ToolSchema captures one MCP-style tool schema ingested by Athena validation.
// ToolSchema 描述 Athena validation 摄取的一条 MCP 风格 tool schema。
type ToolSchema struct {
	Name         string         `json:"name"`
	Description  string         `json:"description,omitempty"`
	ToolScope    string         `json:"tool_scope,omitempty"`
	Operation    string         `json:"operation,omitempty"`
	RiskLevel    string         `json:"risk_level,omitempty"`
	InputSchema  map[string]any `json:"input_schema,omitempty"`
	OutputSchema map[string]any `json:"output_schema,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

// InvocationRequest carries one deterministic validation MCP tool call.
// InvocationRequest 描述一次确定性 validation MCP tool 调用。
type InvocationRequest struct {
	ToolName string         `json:"tool_name"`
	Input    map[string]any `json:"input,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// InvocationTrace captures a whitelist-safe trace summary for one MCP tool call.
// InvocationTrace 描述一次 MCP tool 调用的白名单安全 trace 摘要。
type InvocationTrace struct {
	TraceID         string            `json:"trace_id"`
	TraceType       string            `json:"trace_type"`
	Summary         string            `json:"summary"`
	SafeLabels      map[string]string `json:"safe_labels,omitempty"`
	RedactedPayload map[string]any    `json:"redacted_payload,omitempty"`
}

// InvocationResult captures the safe deterministic result of one validation MCP call.
// InvocationResult 描述一次 validation MCP 调用的安全确定性结果。
type InvocationResult struct {
	InvocationID     string          `json:"invocation_id"`
	ServerID         string          `json:"server_id"`
	ToolName         string          `json:"tool_name"`
	Status           string          `json:"status"`
	ResultSummary    string          `json:"result_summary"`
	Output           map[string]any  `json:"output,omitempty"`
	Trace            InvocationTrace `json:"trace"`
	AppliedRedaction bool            `json:"applied_redaction"`
	Metadata         map[string]any  `json:"metadata,omitempty"`
}
