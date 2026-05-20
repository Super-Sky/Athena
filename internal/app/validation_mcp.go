// validation_mcp.go wires the validation MCP adapter through app-layer governance.
// validation_mcp.go 负责把 validation MCP 适配器通过 app 层治理链路接出。
package app

import (
	"context"
	"fmt"
	"strings"

	"moss/internal/controlplane"
	"moss/internal/validationmcp"
)

// ValidationMCPInvocationResult combines schema ingestion, governance decision, and safe tool result.
// ValidationMCPInvocationResult 组合 schema 摄取、治理判定和安全 tool 结果。
type ValidationMCPInvocationResult struct {
	Server             validationmcp.ServerInfo            `json:"server"`
	Tool               validationmcp.ToolSchema            `json:"tool"`
	Request            validationmcp.InvocationRequest     `json:"request"`
	GovernanceDecision controlplane.ToolGovernanceDecision `json:"governance_decision"`
	Result             validationmcp.InvocationResult      `json:"result"`
}

// ValidationMCPServerInfo returns the validation MCP server descriptor.
// ValidationMCPServerInfo 返回 validation MCP server 描述。
func (s *Service) ValidationMCPServerInfo(ctx context.Context) (validationmcp.ServerInfo, error) {
	server := s.validationMCPServer()
	return server.Info(ctx)
}

// ListValidationMCPTools returns schemas ingested by the validation MCP adapter.
// ListValidationMCPTools 返回 validation MCP 适配器摄取到的 schemas。
func (s *Service) ListValidationMCPTools(ctx context.Context) ([]validationmcp.ToolSchema, error) {
	server := s.validationMCPServer()
	return server.ListTools(ctx)
}

// InvokeValidationMCPTool classifies and invokes one deterministic validation MCP tool.
// InvokeValidationMCPTool 会治理判定并调用一个确定性 validation MCP tool。
func (s *Service) InvokeValidationMCPTool(ctx context.Context, req validationmcp.InvocationRequest) (ValidationMCPInvocationResult, error) {
	server := s.validationMCPServer()
	tool, ok := server.Tool(req.ToolName)
	if !ok {
		return ValidationMCPInvocationResult{}, fmt.Errorf("validation MCP tool %q not found", strings.TrimSpace(req.ToolName))
	}

	decision, err := s.EvaluateToolGovernance(ctx, controlplane.ToolGovernanceDecisionRequest{
		ToolName:  tool.Name,
		ToolScope: tool.ToolScope,
		Operation: tool.Operation,
		RiskLevel: tool.RiskLevel,
		Metadata:  req.Metadata,
	})
	if err != nil {
		return ValidationMCPInvocationResult{}, err
	}
	if strings.EqualFold(decision.Decision, "deny") {
		return ValidationMCPInvocationResult{}, fmt.Errorf("validation MCP tool %q denied by governance: %s", tool.Name, decision.Reason)
	}

	result, err := server.Invoke(ctx, req)
	if err != nil {
		return ValidationMCPInvocationResult{}, err
	}
	info, err := server.Info(ctx)
	if err != nil {
		return ValidationMCPInvocationResult{}, err
	}
	safeInput, _ := validationmcp.RedactSensitiveMap(req.Input)
	safeMetadata, _ := validationmcp.RedactSensitiveMap(req.Metadata)
	return ValidationMCPInvocationResult{
		Server: info,
		Tool:   tool,
		Request: validationmcp.InvocationRequest{
			ToolName: tool.Name,
			Input:    safeInput,
			Metadata: safeMetadata,
		},
		GovernanceDecision: decision,
		Result:             result,
	}, nil
}

func (s *Service) validationMCPServer() *validationmcp.Server {
	if s == nil || s.ValidationMCP == nil {
		return validationmcp.NewServer()
	}
	return s.ValidationMCP
}
