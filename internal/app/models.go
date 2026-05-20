// models.go implements app-layer model governance use cases such as provider, model, and probe flows.
// models.go 实现 app 层模型治理用例，包括供应商、模型和探测相关流程。
package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	einomessage "github.com/cloudwego/eino/schema"
	"moss/internal/model"
	"moss/internal/observability"
)

// ModelTestResult describes one live availability probe against a configured provider model.
// ModelTestResult 描述一次针对已配置供应商模型的真实可用性探测结果。
type ModelTestResult struct {
	ProviderID    string `json:"provider_id"`
	ModelRecordID string `json:"model_record_id"`
	ProviderName  string `json:"provider_name"`
	ModelID       string `json:"model_id"`
	DisplayName   string `json:"display_name"`
	Available     bool   `json:"available"`
	DurationMS    int64  `json:"duration_ms"`
	Error         string `json:"error,omitempty"`
}

// ListModelProviders returns all configured providers with their child model rows.
// ListModelProviders 会返回全部已配置供应商及其子模型行。
func (s *Service) ListModelProviders(ctx context.Context) ([]model.ProviderDefinition, error) {
	if s.ModelStore == nil {
		return nil, nil
	}
	return s.ModelStore.ListProviders(ctx)
}

// CreateModelProvider stores one provider definition and keeps its secret encrypted at rest.
// CreateModelProvider 会保存一条供应商定义，并确保其密钥以加密形式落地。
func (s *Service) CreateModelProvider(ctx context.Context, input model.ProviderUpsertInput) (model.ProviderDefinition, error) {
	if s.ModelStore == nil {
		return model.ProviderDefinition{}, fmt.Errorf("model store is not configured")
	}
	item, err := s.ModelStore.CreateProvider(ctx, input)
	if err != nil {
		return model.ProviderDefinition{}, err
	}
	s.recordModelAudit(ctx, "model_provider_created", item.ID, "", map[string]any{
		"name":     item.Name,
		"protocol": item.Protocol,
		"headers":  redactHeaderValues(item.Headers),
	})
	return item, nil
}

// UpdateModelProvider replaces one provider definition by id while preserving its child models.
// UpdateModelProvider 会按 id 整体替换一条供应商定义，并保留其子模型。
func (s *Service) UpdateModelProvider(ctx context.Context, id string, input model.ProviderUpsertInput) (model.ProviderDefinition, error) {
	if s.ModelStore == nil {
		return model.ProviderDefinition{}, fmt.Errorf("model store is not configured")
	}
	item, err := s.ModelStore.UpdateProvider(ctx, strings.TrimSpace(id), input)
	if err != nil {
		return model.ProviderDefinition{}, err
	}
	s.recordModelAudit(ctx, "model_provider_updated", item.ID, "", map[string]any{
		"name":     item.Name,
		"protocol": item.Protocol,
		"headers":  redactHeaderValues(item.Headers),
	})
	return item, nil
}

// PatchModelProvider applies governance toggles such as enabled without replacing provider metadata.
// PatchModelProvider 会应用启用等治理开关，而不会整体替换供应商元数据。
func (s *Service) PatchModelProvider(ctx context.Context, id string, input model.ProviderPatchInput) (model.ProviderDefinition, error) {
	if s.ModelStore == nil {
		return model.ProviderDefinition{}, fmt.Errorf("model store is not configured")
	}
	item, err := s.ModelStore.PatchProvider(ctx, strings.TrimSpace(id), input)
	if err != nil {
		return model.ProviderDefinition{}, err
	}
	s.recordModelAudit(ctx, "model_provider_patched", item.ID, "", map[string]any{
		"enabled": item.Enabled,
	})
	return item, nil
}

// DeleteModelProvider removes one provider definition after default/fallback roles are reassigned.
// DeleteModelProvider 会在 default 与 fallback 角色被重分配后删除一条供应商定义。
func (s *Service) DeleteModelProvider(ctx context.Context, id string) error {
	if s.ModelStore == nil {
		return fmt.Errorf("model store is not configured")
	}
	trimmed := strings.TrimSpace(id)
	if err := s.ModelStore.DeleteProvider(ctx, trimmed); err != nil {
		return err
	}
	s.recordModelAudit(ctx, "model_provider_deleted", trimmed, "", nil)
	return nil
}

// CreateProviderModel adds one provider model row under the selected provider.
// CreateProviderModel 会在选中的供应商下新增一条模型子项。
func (s *Service) CreateProviderModel(ctx context.Context, providerID string, input model.ProviderModelUpsertInput) (model.ProviderModelRecord, error) {
	if s.ModelStore == nil {
		return model.ProviderModelRecord{}, fmt.Errorf("model store is not configured")
	}
	item, err := s.ModelStore.CreateProviderModel(ctx, strings.TrimSpace(providerID), input)
	if err != nil {
		return model.ProviderModelRecord{}, err
	}
	s.recordModelAudit(ctx, "provider_model_created", item.ProviderID, item.ID, map[string]any{
		"model_id":     item.ModelID,
		"display_name": item.DisplayName,
	})
	return item, nil
}

// UpdateProviderModel replaces one provider model row while preserving its stable row id.
// UpdateProviderModel 会整体替换一条供应商模型子项，同时保留其稳定行 id。
func (s *Service) UpdateProviderModel(ctx context.Context, providerID string, modelRecordID string, input model.ProviderModelUpsertInput) (model.ProviderModelRecord, error) {
	if s.ModelStore == nil {
		return model.ProviderModelRecord{}, fmt.Errorf("model store is not configured")
	}
	item, err := s.ModelStore.UpdateProviderModel(ctx, strings.TrimSpace(providerID), strings.TrimSpace(modelRecordID), input)
	if err != nil {
		return model.ProviderModelRecord{}, err
	}
	s.recordModelAudit(ctx, "provider_model_updated", item.ProviderID, item.ID, map[string]any{
		"model_id":     item.ModelID,
		"display_name": item.DisplayName,
	})
	return item, nil
}

// PatchProviderModel applies enabled/default/fallback toggles to one provider model row.
// PatchProviderModel 会对一条供应商模型子项应用 enabled、default、fallback 开关。
func (s *Service) PatchProviderModel(ctx context.Context, providerID string, modelRecordID string, input model.ProviderModelPatchInput) (model.ProviderModelRecord, error) {
	if s.ModelStore == nil {
		return model.ProviderModelRecord{}, fmt.Errorf("model store is not configured")
	}
	item, err := s.ModelStore.PatchProviderModel(ctx, strings.TrimSpace(providerID), strings.TrimSpace(modelRecordID), input)
	if err != nil {
		return model.ProviderModelRecord{}, err
	}
	s.recordModelAudit(ctx, "provider_model_patched", item.ProviderID, item.ID, map[string]any{
		"enabled":     item.Enabled,
		"is_default":  item.IsDefault,
		"is_fallback": item.IsFallback,
	})
	return item, nil
}

// DeleteProviderModel removes one provider model row after protected roles are reassigned.
// DeleteProviderModel 会在受保护角色被重分配后删除一条供应商模型子项。
func (s *Service) DeleteProviderModel(ctx context.Context, providerID string, modelRecordID string) error {
	if s.ModelStore == nil {
		return fmt.Errorf("model store is not configured")
	}
	trimmedProviderID := strings.TrimSpace(providerID)
	trimmedModelID := strings.TrimSpace(modelRecordID)
	if err := s.ModelStore.DeleteProviderModel(ctx, trimmedProviderID, trimmedModelID); err != nil {
		return err
	}
	s.recordModelAudit(ctx, "provider_model_deleted", trimmedProviderID, trimmedModelID, nil)
	return nil
}

// TestProviderModel runs a live completion probe to confirm that one configured provider/model pair is usable.
// TestProviderModel 会执行一次真实 completion 探测，以确认某个供应商/模型组合是否可用。
func (s *Service) TestProviderModel(ctx context.Context, modelRecordID string) (ModelTestResult, error) {
	if s.ModelStore == nil {
		return ModelTestResult{}, fmt.Errorf("model store is not configured")
	}
	startedAt := time.Now()
	selection, err := s.ModelStore.Resolve(ctx, strings.TrimSpace(modelRecordID))
	if err != nil {
		return ModelTestResult{}, err
	}
	provider := s.ModelProvider
	if provider == nil {
		provider = model.NewProvider()
	}
	chatConfig, err := resolveAppModelConfig(selection.Primary, providerProbePolicyContext())
	if err != nil {
		return ModelTestResult{}, err
	}
	chatModel, err := provider.NewChatModel(ctx, chatConfig)
	if err != nil {
		return ModelTestResult{
			ProviderID:    selection.Primary.ProviderID,
			ModelRecordID: selection.Primary.ModelRecordID,
			ProviderName:  selection.Primary.ProviderName,
			ModelID:       selection.Primary.ProviderModelID,
			DisplayName:   selection.Primary.ModelDisplayName,
			Available:     false,
			DurationMS:    time.Since(startedAt).Milliseconds(),
			Error:         err.Error(),
		}, nil
	}
	_, err = chatModel.Generate(ctx, []*einomessage.Message{
		einomessage.SystemMessage("You are a concise health checker."),
		einomessage.UserMessage("Reply with exactly: pong"),
	})
	result := ModelTestResult{
		ProviderID:    selection.Primary.ProviderID,
		ModelRecordID: selection.Primary.ModelRecordID,
		ProviderName:  selection.Primary.ProviderName,
		ModelID:       selection.Primary.ProviderModelID,
		DisplayName:   selection.Primary.ModelDisplayName,
		Available:     err == nil,
		DurationMS:    time.Since(startedAt).Milliseconds(),
	}
	if err != nil {
		result.Error = err.Error()
	}
	if s.Observability != nil {
		level := observability.LogLevelInfo
		status := "ok"
		reason := "probe_succeeded"
		errorCode := ""
		detail := map[string]any{
			"provider_id":     result.ProviderID,
			"model_record_id": result.ModelRecordID,
			"provider_name":   result.ProviderName,
			"provider_model":  result.ModelID,
			"duration_ms":     result.DurationMS,
		}
		if err != nil {
			level = observability.LogLevelError
			status = "error"
			reason = "probe_failed"
			errorCode = "provider_model_probe_failed"
			detail["error"] = err.Error()
		}
		s.Observability.LogAction(ctx, level, observability.ActionLog{
			Module:     "app",
			Action:     "provider_model_probe",
			Step:       "completed",
			Status:     status,
			RequestID:  "",
			SessionID:  "",
			Reason:     reason,
			ErrorCode:  errorCode,
			DurationMS: result.DurationMS,
			Detail:     detail,
		})
	}
	s.recordModelAudit(ctx, "provider_model_tested", result.ProviderID, result.ModelRecordID, map[string]any{
		"available":   result.Available,
		"duration_ms": result.DurationMS,
	})
	return result, nil
}

func (s *Service) recordModelAudit(ctx context.Context, action string, providerID string, modelRecordID string, detail map[string]any) {
	if s.Observability == nil {
		return
	}
	labels := map[string]string{}
	if providerID != "" {
		labels["provider_id"] = providerID
	}
	if modelRecordID != "" {
		labels["model_record_id"] = modelRecordID
	}
	s.Observability.Inc("model_governance_total", labels)
	s.Observability.RecordAudit(ctx, observability.AuditRecord{
		Action: action,
		Detail: map[string]any{
			"provider_id":     providerID,
			"model_record_id": modelRecordID,
			"detail":          detail,
		},
	})
}

func redactHeaderValues(headers map[string]string) map[string]string {
	if len(headers) == 0 {
		return nil
	}
	result := make(map[string]string, len(headers))
	for key := range headers {
		result[key] = "***redacted***"
	}
	return result
}
