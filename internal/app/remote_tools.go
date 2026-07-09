// remote_tools.go connects persisted remote registrations to the live runtime tool catalog.
// remote_tools.go 将持久化远程注册接入实时 runtime 工具目录。
package app

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"moss/internal/controlplane"
	"moss/internal/tools"
)

func remoteToolOptions(service *Service) tools.RemoteToolOptions {
	if service == nil {
		return tools.RemoteToolOptions{}
	}
	return tools.RemoteToolOptions{
		AllowedOrigins:   append([]string(nil), service.Config.RemoteTools.AllowedOrigins...),
		MaxResponseBytes: service.Config.RemoteTools.MaxResponseBytes,
		Evaluate: func(ctx context.Context, request tools.RemoteGovernanceRequest) (tools.RemoteGovernanceDecision, error) {
			if service.ControlPlane == nil {
				return tools.RemoteGovernanceDecision{Decision: "allow"}, nil
			}
			decision, err := service.ControlPlane.EvaluateToolGovernance(ctx, controlplane.ToolGovernanceDecisionRequest{
				ToolName:  request.ToolName,
				ToolScope: request.ToolScope,
				Operation: request.Operation,
				RiskLevel: request.RiskLevel,
				Metadata: map[string]any{
					"app_id":        request.AppID,
					"tool_call_id":  request.ToolCallID,
					"argument_keys": append([]string(nil), request.ArgumentKeys...),
				},
			})
			if err != nil {
				return tools.RemoteGovernanceDecision{}, err
			}
			return tools.RemoteGovernanceDecision{
				DecisionID:   decision.DecisionID,
				Decision:     decision.Decision,
				Reason:       decision.Reason,
				RedactFields: append([]string(nil), decision.RedactFields...),
				SandboxRef:   decision.SandboxRef,
			}, nil
		},
		Observe: func(ctx context.Context, event tools.RemoteInvocationEvent) {
			if service.Observability == nil {
				return
			}
			attrs := map[string]string{
				"tool_call_id":      event.ToolCallID,
				"registration_id":   event.RegistrationID,
				"app_id":            event.AppID,
				"tool_name":         event.ToolName,
				"endpoint_origin":   event.EndpointOrigin,
				"attempt":           strconv.Itoa(event.Attempt),
				"status":            event.Status,
				"decision_id":       event.DecisionID,
				"governance_result": event.GovernanceResult,
				"error_code":        event.ErrorCode,
			}
			service.Observability.Trace(ctx, "remote_tool_invocation", attrs)
			service.Observability.Observe("remote_tool_duration_ms", float64(event.DurationMS), map[string]string{
				"tool_name": event.ToolName,
				"status":    event.Status,
			})
		},
	}
}

// ListRemoteTools returns persisted app-owned HTTP tool registrations.
// ListRemoteTools 返回已持久化的应用自有 HTTP 工具注册。
func (s *Service) ListRemoteTools(ctx context.Context) ([]tools.RemoteRegistration, error) {
	if s == nil || s.ControlPlane == nil {
		return nil, nil
	}
	return s.ControlPlane.ListRemoteTools(ctx)
}

// UpsertRemoteTool validates, persists, and atomically publishes one remote registration.
// UpsertRemoteTool 校验、持久化并原子发布一条远程工具注册。
func (s *Service) UpsertRemoteTool(ctx context.Context, name string, input tools.RemoteRegistration) (tools.RemoteRegistration, error) {
	if s == nil || s.ControlPlane == nil || s.ToolCatalog == nil {
		return tools.RemoteRegistration{}, fmt.Errorf("remote tool registry is not configured")
	}
	s.remoteToolMu.Lock()
	defer s.remoteToolMu.Unlock()
	name = strings.TrimSpace(name)
	input.Name = defaultRemoteToolName(input.Name, name)
	_, managed := s.remoteTools[name]
	if _, exists := s.ToolCatalog.Get(name); exists && !managed {
		return tools.RemoteRegistration{}, fmt.Errorf("remote tool %q conflicts with a built-in tool", name)
	}
	definition, err := tools.NewRemoteDefinition(input, remoteToolOptions(s))
	if err != nil {
		return tools.RemoteRegistration{}, err
	}
	item, err := s.ControlPlane.PutRemoteTool(ctx, name, input)
	if err != nil {
		return tools.RemoteRegistration{}, err
	}
	if !item.Enabled {
		s.ToolCatalog.Delete(name)
		delete(s.remoteTools, name)
		return item, nil
	}
	if err := s.ToolCatalog.Register(definition); err != nil {
		return tools.RemoteRegistration{}, err
	}
	s.remoteTools[name] = struct{}{}
	return item, nil
}

// DeleteRemoteTool removes one registration from persistence and the live catalog.
// DeleteRemoteTool 从持久化与实时目录中删除一条注册。
func (s *Service) DeleteRemoteTool(ctx context.Context, name string) error {
	if s == nil || s.ControlPlane == nil || s.ToolCatalog == nil {
		return fmt.Errorf("remote tool registry is not configured")
	}
	s.remoteToolMu.Lock()
	defer s.remoteToolMu.Unlock()
	if err := s.ControlPlane.DeleteRemoteTool(ctx, name); err != nil {
		return err
	}
	s.ToolCatalog.Delete(name)
	delete(s.remoteTools, strings.TrimSpace(name))
	return nil
}

func (s *Service) reloadRemoteToolCatalog(ctx context.Context) error {
	if s == nil || s.ControlPlane == nil || s.ToolCatalog == nil {
		return fmt.Errorf("remote tool registry is not configured")
	}
	s.remoteToolMu.Lock()
	defer s.remoteToolMu.Unlock()

	registrations, err := s.ControlPlane.ListRemoteTools(ctx)
	if err != nil {
		return err
	}
	next := make(map[string]tools.Definition)
	for _, registration := range registrations {
		if !registration.Enabled {
			continue
		}
		definition, err := tools.NewRemoteDefinition(registration, remoteToolOptions(s))
		if err != nil {
			return fmt.Errorf("restore remote tool %q failed: %w", registration.Name, err)
		}
		if _, exists := s.ToolCatalog.Get(definition.Name); exists {
			if _, managed := s.remoteTools[definition.Name]; !managed {
				return fmt.Errorf("restore remote tool %q conflicts with an existing tool", definition.Name)
			}
		}
		next[definition.Name] = definition
	}
	for name := range s.remoteTools {
		s.ToolCatalog.Delete(name)
	}
	for _, definition := range next {
		if err := s.ToolCatalog.Register(definition); err != nil {
			return err
		}
	}
	s.remoteTools = make(map[string]struct{}, len(next))
	for name := range next {
		s.remoteTools[name] = struct{}{}
	}
	return nil
}

func defaultRemoteToolName(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return strings.TrimSpace(fallback)
}
