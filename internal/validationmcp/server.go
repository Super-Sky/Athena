// server.go implements Athena's deterministic validation MCP adapter.
// server.go 实现 Athena 的确定性 validation MCP 适配器。
package validationmcp

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"
)

const (
	serverID        = "athena-validation-mcp"
	serverName      = "Athena Validation MCP"
	serverTransport = "control-plane-http-adapter"
	statusReady     = "ready"
)

// Server exposes a small deterministic MCP-like surface for validation and governance checks.
// Server 暴露一个小型确定性 MCP 风格界面，用于验证和治理检查。
type Server struct {
	tools map[string]toolDefinition
}

type toolDefinition struct {
	schema  ToolSchema
	handler func(context.Context, map[string]any) (map[string]any, string, error)
}

// NewServer creates the built-in athena-validation-mcp adapter.
// NewServer 创建内置 athena-validation-mcp 适配器。
func NewServer() *Server {
	defs := []toolDefinition{
		newSecurityContextEchoTool(),
		newRiskSignalLookupTool(),
	}
	tools := make(map[string]toolDefinition, len(defs))
	for _, def := range defs {
		tools[def.schema.Name] = def
	}
	return &Server{tools: tools}
}

// Info returns server metadata together with the current sorted tool schemas.
// Info 返回 server 元数据以及当前已排序的 tool schemas。
func (s *Server) Info(ctx context.Context) (ServerInfo, error) {
	tools, err := s.ListTools(ctx)
	if err != nil {
		return ServerInfo{}, err
	}
	return ServerInfo{
		ServerID:  serverID,
		Name:      serverName,
		Transport: serverTransport,
		Status:    statusReady,
		Tools:     tools,
		Metadata: map[string]any{
			"schema_source":     "internal/validationmcp",
			"persistence_scope": "validation_only",
		},
	}, nil
}

// ListTools returns deterministic MCP-style tool schemas.
// ListTools 返回确定性的 MCP 风格 tool schemas。
func (s *Server) ListTools(_ context.Context) ([]ToolSchema, error) {
	if s == nil {
		return nil, fmt.Errorf("validation MCP server is nil")
	}
	items := make([]ToolSchema, 0, len(s.tools))
	for _, def := range s.tools {
		items = append(items, cloneToolSchema(def.schema))
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Name < items[j].Name
	})
	return items, nil
}

// Invoke executes one deterministic validation tool and returns only safe output and trace payloads.
// Invoke 执行一次确定性验证 tool，并且只返回安全输出和 trace payload。
func (s *Server) Invoke(ctx context.Context, req InvocationRequest) (InvocationResult, error) {
	if s == nil {
		return InvocationResult{}, fmt.Errorf("validation MCP server is nil")
	}
	toolName := strings.TrimSpace(req.ToolName)
	if toolName == "" {
		return InvocationResult{}, fmt.Errorf("tool_name is required")
	}
	def, ok := s.tools[toolName]
	if !ok {
		return InvocationResult{}, fmt.Errorf("validation MCP tool %q not found", toolName)
	}

	safeInput, inputRedacted := RedactSensitiveMap(req.Input)
	output, summary, err := def.handler(ctx, safeInput)
	if err != nil {
		return InvocationResult{}, err
	}
	safeOutput, outputRedacted := RedactSensitiveMap(output)
	safeMetadata, metadataRedacted := RedactSensitiveMap(req.Metadata)
	invocationID := "validation_mcp_" + uuid.NewString()

	return InvocationResult{
		InvocationID:  invocationID,
		ServerID:      serverID,
		ToolName:      def.schema.Name,
		Status:        "success",
		ResultSummary: summary,
		Output:        safeOutput,
		Trace: InvocationTrace{
			TraceID:   "trace_" + uuid.NewString(),
			TraceType: "validation_mcp_tool_invocation",
			Summary:   summary,
			SafeLabels: map[string]string{
				"server_id":  serverID,
				"tool_name":  def.schema.Name,
				"tool_scope": def.schema.ToolScope,
				"operation":  def.schema.Operation,
				"risk_level": def.schema.RiskLevel,
			},
			RedactedPayload: map[string]any{
				"input":    safeInput,
				"metadata": safeMetadata,
				"output":   safeOutput,
			},
		},
		AppliedRedaction: inputRedacted || outputRedacted || metadataRedacted,
		Metadata: map[string]any{
			"schema_ingested": true,
			"transport":       serverTransport,
		},
	}, nil
}

// Tool returns one schema by name for governance classification.
// Tool 按名称返回一条 schema，供治理判定使用。
func (s *Server) Tool(name string) (ToolSchema, bool) {
	if s == nil {
		return ToolSchema{}, false
	}
	def, ok := s.tools[strings.TrimSpace(name)]
	if !ok {
		return ToolSchema{}, false
	}
	return cloneToolSchema(def.schema), true
}

func newSecurityContextEchoTool() toolDefinition {
	return toolDefinition{
		schema: ToolSchema{
			Name:        "security_context_echo",
			Description: "Echo a safe security context summary for validation.",
			ToolScope:   "validation_mcp",
			Operation:   "invoke",
			RiskLevel:   "low",
			InputSchema: objectSchema(map[string]any{
				"subject":        stringSchema("Subject or caller label."),
				"classification": stringSchema("Data classification label."),
				"request_id":     stringSchema("Validation request identifier."),
				"credentials":    objectSchema(map[string]any{}, nil),
			}, []string{"subject"}),
			OutputSchema: objectSchema(map[string]any{
				"subject":        stringSchema("Safe subject label."),
				"classification": stringSchema("Safe classification label."),
				"request_id":     stringSchema("Safe request identifier."),
				"echoed":         map[string]any{"type": "boolean"},
			}, []string{"subject", "echoed"}),
			Metadata: map[string]any{"current_phase": "phase_4_validation_mcp"},
		},
		handler: func(_ context.Context, input map[string]any) (map[string]any, string, error) {
			subject := defaultString(valueAsString(input["subject"]), "anonymous")
			classification := defaultString(valueAsString(input["classification"]), "internal")
			requestID := valueAsString(input["request_id"])
			output := map[string]any{
				"subject":        subject,
				"classification": classification,
				"request_id":     requestID,
				"echoed":         true,
			}
			return output, "security context echoed with whitelist-safe fields", nil
		},
	}
}

func newRiskSignalLookupTool() toolDefinition {
	return toolDefinition{
		schema: ToolSchema{
			Name:        "risk_signal_lookup",
			Description: "Return deterministic validation risk signals for one risk key.",
			ToolScope:   "validation_mcp",
			Operation:   "invoke",
			RiskLevel:   "medium",
			InputSchema: objectSchema(map[string]any{
				"risk_key":      stringSchema("Risk lookup key."),
				"severity_hint": stringSchema("Optional severity hint."),
				"credentials":   objectSchema(map[string]any{}, nil),
			}, []string{"risk_key"}),
			OutputSchema: objectSchema(map[string]any{
				"risk_key":   stringSchema("Risk lookup key."),
				"risk_level": stringSchema("Deterministic risk level."),
				"risk_score": map[string]any{"type": "integer"},
				"signals":    map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			}, []string{"risk_key", "risk_level", "risk_score", "signals"}),
			Metadata: map[string]any{"current_phase": "phase_4_validation_mcp"},
		},
		handler: func(_ context.Context, input map[string]any) (map[string]any, string, error) {
			riskKey := strings.TrimSpace(valueAsString(input["risk_key"]))
			if riskKey == "" {
				return nil, "", fmt.Errorf("risk_key is required")
			}
			level, score, signals := deterministicRiskSignals(riskKey, valueAsString(input["severity_hint"]))
			output := map[string]any{
				"risk_key":   riskKey,
				"risk_level": level,
				"risk_score": score,
				"signals":    signals,
			}
			return output, fmt.Sprintf("risk signal %s classified as %s", riskKey, level), nil
		},
	}
}

func deterministicRiskSignals(riskKey string, severityHint string) (string, int, []string) {
	combined := strings.ToLower(riskKey + " " + severityHint)
	switch {
	case strings.Contains(combined, "credential") || strings.Contains(combined, "secret") || strings.Contains(combined, "token"):
		return "high", 90, []string{"credential_like", "redaction_required"}
	case strings.Contains(combined, "write") || strings.Contains(combined, "sandbox"):
		return "medium", 60, []string{"boundary_required"}
	default:
		return "low", 20, []string{"read_only"}
	}
}

func objectSchema(properties map[string]any, required []string) map[string]any {
	result := map[string]any{
		"type":                 "object",
		"properties":           properties,
		"additionalProperties": true,
	}
	if len(required) > 0 {
		result["required"] = append([]string(nil), required...)
	}
	return result
}

func stringSchema(description string) map[string]any {
	return map[string]any{
		"type":        "string",
		"description": description,
	}
}

func cloneToolSchema(input ToolSchema) ToolSchema {
	return ToolSchema{
		Name:         input.Name,
		Description:  input.Description,
		ToolScope:    input.ToolScope,
		Operation:    input.Operation,
		RiskLevel:    input.RiskLevel,
		InputSchema:  cloneAnyMap(input.InputSchema),
		OutputSchema: cloneAnyMap(input.OutputSchema),
		Metadata:     cloneAnyMap(input.Metadata),
	}
}

func defaultString(value string, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return fallback
}
