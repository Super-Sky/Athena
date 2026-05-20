// memory_store.go implements the in-memory model governance store used for local and test flows.
// memory_store.go 实现本地与测试流程使用的内存态模型治理存储。
package model

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type memoryProvider struct {
	definition      ProviderDefinition
	encryptedAPIKey string
}

// MemoryStore keeps provider and model governance state in-process for tests and lightweight runs.
// MemoryStore 会在进程内保存供应商与模型治理状态，适合测试和轻量运行。
type MemoryStore struct {
	mu            sync.RWMutex
	encryptionKey string
	providers     map[string]memoryProvider
}

// NewMemoryStore creates one in-process governance store.
// NewMemoryStore 会创建一个进程内治理 store。
func NewMemoryStore(encryptionKey string) *MemoryStore {
	return &MemoryStore{
		encryptionKey: encryptionKey,
		providers:     map[string]memoryProvider{},
	}
}

// AutoMigrate is a no-op for the in-memory implementation.
// AutoMigrate 对内存实现来说是一个空操作。
func (s *MemoryStore) AutoMigrate(context.Context) error {
	return nil
}

// ListProviders returns all configured providers and their child model entries.
// ListProviders 会返回全部已配置供应商及其子模型条目。
func (s *MemoryStore) ListProviders(context.Context) ([]ProviderDefinition, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]ProviderDefinition, 0, len(s.providers))
	for _, item := range s.providers {
		result = append(result, cloneProviderDefinition(item.definition))
	}
	slices.SortFunc(result, func(a, b ProviderDefinition) int {
		return strings.Compare(a.Name, b.Name)
	})
	return result, nil
}

// CreateProvider inserts one provider definition and keeps secrets encrypted at rest.
// CreateProvider 会插入一条供应商定义，并确保密钥以加密方式落地。
func (s *MemoryStore) CreateProvider(_ context.Context, input ProviderUpsertInput) (ProviderDefinition, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	definition, encrypted, err := s.normalizeCreateProvider(input)
	if err != nil {
		return ProviderDefinition{}, err
	}
	if err := s.ensureProviderNameUnique("", definition.Name); err != nil {
		return ProviderDefinition{}, err
	}
	if providerModelsContainDefault(definition.Models) {
		s.clearDefaultModel()
	}
	if providerModelsContainFallback(definition.Models) {
		s.clearFallbackModel()
	}
	s.providers[definition.ID] = memoryProvider{
		definition:      definition,
		encryptedAPIKey: encrypted,
	}
	return cloneProviderDefinition(definition), nil
}

// UpdateProvider replaces one provider definition while preserving its stable id and child models.
// UpdateProvider 会在保留稳定 id 和子模型列表的前提下整体替换供应商定义。
func (s *MemoryStore) UpdateProvider(_ context.Context, id string, input ProviderUpsertInput) (ProviderDefinition, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	record, ok := s.providers[strings.TrimSpace(id)]
	if !ok {
		return ProviderDefinition{}, &ResolveError{Code: "invalid_model_provider", Reason: "not_found", Message: "model provider was not found"}
	}
	definition, encrypted, err := s.normalizeUpdateProvider(record, input)
	if err != nil {
		return ProviderDefinition{}, err
	}
	if err := s.ensureProviderNameUnique(record.definition.ID, definition.Name); err != nil {
		return ProviderDefinition{}, err
	}
	record.definition = definition
	record.encryptedAPIKey = encrypted
	s.providers[record.definition.ID] = record
	return cloneProviderDefinition(record.definition), nil
}

// PatchProvider applies provider-level governance toggles such as enabled without replacing metadata.
// PatchProvider 会应用供应商级别的治理开关，例如启用状态，而不会整体替换元数据。
func (s *MemoryStore) PatchProvider(_ context.Context, id string, input ProviderPatchInput) (ProviderDefinition, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	record, ok := s.providers[strings.TrimSpace(id)]
	if !ok {
		return ProviderDefinition{}, &ResolveError{Code: "invalid_model_provider", Reason: "not_found", Message: "model provider was not found"}
	}
	if input.Enabled != nil {
		if !*input.Enabled && providerOwnsProtectedModel(record.definition.Models) {
			return ProviderDefinition{}, fmt.Errorf("provider owns the current default or fallback model and cannot be disabled")
		}
		record.definition.Enabled = *input.Enabled
	}
	record.definition.UpdatedAt = time.Now().UTC()
	s.providers[record.definition.ID] = record
	return cloneProviderDefinition(record.definition), nil
}

// DeleteProvider removes one provider after all protected models have been reassigned.
// DeleteProvider 会在全部受保护模型已重分配后删除一个供应商。
func (s *MemoryStore) DeleteProvider(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	record, ok := s.providers[strings.TrimSpace(id)]
	if !ok {
		return ErrModelProviderNotFound
	}
	if providerOwnsProtectedModel(record.definition.Models) {
		return fmt.Errorf("provider owns the current default or fallback model and cannot be deleted")
	}
	delete(s.providers, record.definition.ID)
	return nil
}

// CreateProviderModel creates one child model entry under a provider.
// CreateProviderModel 会在某个供应商下创建一条模型子项。
func (s *MemoryStore) CreateProviderModel(_ context.Context, providerID string, input ProviderModelUpsertInput) (ProviderModelRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	record, ok := s.providers[strings.TrimSpace(providerID)]
	if !ok {
		return ProviderModelRecord{}, &ResolveError{Code: "invalid_model_provider", Reason: "not_found", Message: "model provider was not found"}
	}
	modelRecord, err := normalizeProviderModelRecord("", record.definition.ID, input, time.Now().UTC(), time.Now().UTC())
	if err != nil {
		return ProviderModelRecord{}, err
	}
	if err := s.ensureProviderModelUnique(record.definition, "", modelRecord.ModelID, modelRecord.DisplayName); err != nil {
		return ProviderModelRecord{}, err
	}
	if modelRecord.IsDefault {
		s.clearDefaultModel()
	}
	if modelRecord.IsFallback {
		s.clearFallbackModel()
	}
	record.definition.Models = append(record.definition.Models, modelRecord)
	record.definition.UpdatedAt = time.Now().UTC()
	s.providers[record.definition.ID] = record
	return cloneProviderModelRecord(modelRecord), nil
}

// UpdateProviderModel replaces one child model entry while preserving its stable record id.
// UpdateProviderModel 会整体替换一条模型子项，同时保留其稳定记录 id。
func (s *MemoryStore) UpdateProviderModel(_ context.Context, providerID string, modelRecordID string, input ProviderModelUpsertInput) (ProviderModelRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	record, ok := s.providers[strings.TrimSpace(providerID)]
	if !ok {
		return ProviderModelRecord{}, &ResolveError{Code: "invalid_model_provider", Reason: "not_found", Message: "model provider was not found"}
	}
	idx := providerModelIndex(record.definition.Models, modelRecordID)
	if idx < 0 {
		return ProviderModelRecord{}, &ResolveError{Code: "invalid_model", Reason: "not_found", Message: "provider model was not found"}
	}
	now := time.Now().UTC()
	modelRecord, err := normalizeProviderModelRecord(record.definition.Models[idx].ID, record.definition.ID, input, record.definition.Models[idx].CreatedAt, now)
	if err != nil {
		return ProviderModelRecord{}, err
	}
	if err := s.ensureProviderModelUnique(record.definition, modelRecord.ID, modelRecord.ModelID, modelRecord.DisplayName); err != nil {
		return ProviderModelRecord{}, err
	}
	if modelRecord.IsDefault {
		s.clearDefaultModel()
	}
	if modelRecord.IsFallback {
		s.clearFallbackModel()
	}
	record.definition.Models[idx] = modelRecord
	record.definition.UpdatedAt = now
	s.providers[record.definition.ID] = record
	return cloneProviderModelRecord(modelRecord), nil
}

// PatchProviderModel applies enable/default/fallback toggles on one child model entry.
// PatchProviderModel 会对一条模型子项应用启用、default、fallback 开关。
func (s *MemoryStore) PatchProviderModel(_ context.Context, providerID string, modelRecordID string, input ProviderModelPatchInput) (ProviderModelRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	record, ok := s.providers[strings.TrimSpace(providerID)]
	if !ok {
		return ProviderModelRecord{}, &ResolveError{Code: "invalid_model_provider", Reason: "not_found", Message: "model provider was not found"}
	}
	idx := providerModelIndex(record.definition.Models, modelRecordID)
	if idx < 0 {
		return ProviderModelRecord{}, &ResolveError{Code: "invalid_model", Reason: "not_found", Message: "provider model was not found"}
	}
	item := record.definition.Models[idx]
	if input.Enabled != nil {
		if !*input.Enabled && (item.IsDefault || item.IsFallback) {
			return ProviderModelRecord{}, fmt.Errorf("default or fallback model must be reassigned before disable")
		}
		item.Enabled = *input.Enabled
	}
	if input.IsDefault != nil {
		if *input.IsDefault {
			item.Enabled = true
			s.clearDefaultModel()
			item.IsDefault = true
		} else if item.IsDefault {
			return ProviderModelRecord{}, fmt.Errorf("default model must be reassigned before unset")
		}
	}
	if input.IsFallback != nil {
		if *input.IsFallback {
			item.Enabled = true
			s.clearFallbackModel()
			item.IsFallback = true
		} else if item.IsFallback {
			return ProviderModelRecord{}, fmt.Errorf("fallback model must be reassigned before unset")
		}
	}
	item.UpdatedAt = time.Now().UTC()
	record.definition.Models[idx] = item
	record.definition.UpdatedAt = item.UpdatedAt
	s.providers[record.definition.ID] = record
	return cloneProviderModelRecord(item), nil
}

// DeleteProviderModel removes one child model entry when it is no longer protected by default/fallback semantics.
// DeleteProviderModel 会在模型子项不再承担 default 或 fallback 角色时删除该条目。
func (s *MemoryStore) DeleteProviderModel(_ context.Context, providerID string, modelRecordID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	record, ok := s.providers[strings.TrimSpace(providerID)]
	if !ok {
		return ErrModelProviderNotFound
	}
	idx := providerModelIndex(record.definition.Models, modelRecordID)
	if idx < 0 {
		return ErrProviderModelNotFound
	}
	if record.definition.Models[idx].IsDefault || record.definition.Models[idx].IsFallback {
		return fmt.Errorf("default or fallback model must be reassigned before delete")
	}
	record.definition.Models = append(record.definition.Models[:idx], record.definition.Models[idx+1:]...)
	record.definition.UpdatedAt = time.Now().UTC()
	s.providers[record.definition.ID] = record
	return nil
}

// Resolve selects either the requested model or the current default model for one request.
// Resolve 会为一次请求选择显式模型或当前默认模型。
func (s *MemoryStore) Resolve(_ context.Context, requestedModelRecordID string) (*Selection, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if trimmed := strings.TrimSpace(requestedModelRecordID); trimmed != "" {
		provider, modelRecord, ok := s.findModelRecord(trimmed)
		if !ok {
			return nil, &ResolveError{Code: "invalid_model", Reason: "not_found", Message: "requested model was not found", Detail: map[string]any{"model_record_id": trimmed}}
		}
		if !provider.Enabled {
			return nil, &ResolveError{Code: "invalid_model", Reason: "provider_disabled", Message: "requested model provider is disabled", Detail: map[string]any{"model_record_id": trimmed, "provider_id": provider.ID}}
		}
		if !modelRecord.Enabled {
			return nil, &ResolveError{Code: "invalid_model", Reason: "disabled", Message: "requested model is disabled", Detail: map[string]any{"model_record_id": trimmed, "provider_id": provider.ID}}
		}
		primary, err := s.toChatConfig(provider, modelRecord)
		if err != nil {
			return nil, err
		}
		return &Selection{
			Primary:      primary,
			Explicit:     true,
			PrimaryMeta:  cloneProviderModelRecord(modelRecord),
			ProviderMeta: cloneProviderDefinition(provider),
		}, nil
	}

	var (
		defaultProvider *ProviderDefinition
		defaultModel    *ProviderModelRecord
		fallbackModel   *ProviderModelRecord
	)
	for _, item := range s.providers {
		if !item.definition.Enabled {
			continue
		}
		for _, modelRecord := range item.definition.Models {
			copyProvider := cloneProviderDefinition(item.definition)
			copyModel := cloneProviderModelRecord(modelRecord)
			if copyModel.Enabled && copyModel.IsDefault {
				defaultProvider = &copyProvider
				defaultModel = &copyModel
			}
			if copyModel.Enabled && copyModel.IsFallback {
				fallbackModel = &copyModel
			}
		}
	}
	if defaultProvider == nil || defaultModel == nil {
		return nil, &ResolveError{Code: "invalid_model", Reason: "default_missing", Message: "default model is not configured", Detail: map[string]any{"selection": "default"}}
	}
	primary, err := s.toChatConfig(*defaultProvider, *defaultModel)
	if err != nil {
		return nil, err
	}
	selection := &Selection{
		Primary:      primary,
		Explicit:     false,
		PrimaryMeta:  *defaultModel,
		ProviderMeta: *defaultProvider,
	}
	if fallbackModel != nil && fallbackModel.ID != defaultModel.ID {
		fallbackProvider, _, ok := s.findModelRecord(fallbackModel.ID)
		if ok && fallbackProvider.Enabled {
			fallback, err := s.toChatConfig(fallbackProvider, *fallbackModel)
			if err != nil {
				return nil, err
			}
			meta := cloneProviderModelRecord(*fallbackModel)
			selection.Fallback = &fallback
			selection.FallbackMeta = &meta
		}
	}
	return selection, nil
}

func (s *MemoryStore) normalizeCreateProvider(input ProviderUpsertInput) (ProviderDefinition, string, error) {
	now := time.Now().UTC()
	encrypted, err := EncryptSecret(strings.TrimSpace(input.APIKey), s.encryptionKey)
	if err != nil {
		return ProviderDefinition{}, "", err
	}
	definition, err := normalizeProviderDefinition(uuid.NewString(), input, now, now)
	if err != nil {
		return ProviderDefinition{}, "", err
	}
	models, err := normalizeProviderModelRecords(definition.ID, input.Models, now, now)
	if err != nil {
		return ProviderDefinition{}, "", err
	}
	definition.Models = models
	return definition, encrypted, nil
}

func (s *MemoryStore) normalizeUpdateProvider(existing memoryProvider, input ProviderUpsertInput) (ProviderDefinition, string, error) {
	apiKey := strings.TrimSpace(input.APIKey)
	encrypted := existing.encryptedAPIKey
	var err error
	if apiKey != "" {
		encrypted, err = EncryptSecret(apiKey, s.encryptionKey)
		if err != nil {
			return ProviderDefinition{}, "", err
		}
	}
	definition, err := normalizeProviderDefinition(existing.definition.ID, input, existing.definition.CreatedAt, time.Now().UTC())
	if err != nil {
		return ProviderDefinition{}, "", err
	}
	if apiKey == "" {
		definition.APIKeyConfigured = existing.definition.APIKeyConfigured
		definition.APIKeyMasked = existing.definition.APIKeyMasked
	}
	definition.Models = cloneProviderModels(existing.definition.Models)
	return definition, encrypted, nil
}

func (s *MemoryStore) ensureProviderNameUnique(currentID string, name string) error {
	for _, item := range s.providers {
		if item.definition.ID != currentID && strings.EqualFold(item.definition.Name, name) {
			return fmt.Errorf("model provider name %q already exists", name)
		}
	}
	return nil
}

func (s *MemoryStore) ensureProviderModelUnique(provider ProviderDefinition, currentRecordID string, modelID string, displayName string) error {
	for _, item := range provider.Models {
		if item.ID == currentRecordID {
			continue
		}
		if strings.EqualFold(item.ModelID, modelID) {
			return fmt.Errorf("provider model_id %q already exists under provider %q", modelID, provider.Name)
		}
		if strings.EqualFold(item.DisplayName, displayName) {
			return fmt.Errorf("provider model display_name %q already exists under provider %q", displayName, provider.Name)
		}
	}
	return nil
}

func (s *MemoryStore) clearDefaultModel() {
	for providerID, provider := range s.providers {
		for idx := range provider.definition.Models {
			provider.definition.Models[idx].IsDefault = false
		}
		s.providers[providerID] = provider
	}
}

func (s *MemoryStore) clearFallbackModel() {
	for providerID, provider := range s.providers {
		for idx := range provider.definition.Models {
			provider.definition.Models[idx].IsFallback = false
		}
		s.providers[providerID] = provider
	}
}

func (s *MemoryStore) findModelRecord(modelRecordID string) (ProviderDefinition, ProviderModelRecord, bool) {
	for _, provider := range s.providers {
		for _, modelRecord := range provider.definition.Models {
			if modelRecord.ID == modelRecordID {
				return cloneProviderDefinition(provider.definition), cloneProviderModelRecord(modelRecord), true
			}
		}
	}
	return ProviderDefinition{}, ProviderModelRecord{}, false
}

func (s *MemoryStore) toChatConfig(provider ProviderDefinition, modelRecord ProviderModelRecord) (ChatConfig, error) {
	record, ok := s.providers[provider.ID]
	if !ok {
		return ChatConfig{}, fmt.Errorf("model provider %q is not available", provider.ID)
	}
	apiKey, err := DecryptSecret(record.encryptedAPIKey, s.encryptionKey)
	if err != nil {
		return ChatConfig{}, err
	}
	return ChatConfig{
		ProviderID:       provider.ID,
		ProviderName:     provider.Name,
		ProviderProtocol: provider.Protocol,
		BaseURL:          provider.BaseURL,
		RequestTimeout:   time.Duration(provider.RequestTimeoutSeconds) * time.Second,
		APIKey:           apiKey,
		Headers:          cloneMetadata(provider.Headers),
		ModelRecordID:    modelRecord.ID,
		ProviderModelID:  modelRecord.ModelID,
		ModelDisplayName: modelRecord.DisplayName,
	}, nil
}

func providerOwnsProtectedModel(models []ProviderModelRecord) bool {
	for _, item := range models {
		if item.IsDefault || item.IsFallback {
			return true
		}
	}
	return false
}

func providerModelIndex(items []ProviderModelRecord, modelRecordID string) int {
	for idx, item := range items {
		if item.ID == strings.TrimSpace(modelRecordID) {
			return idx
		}
	}
	return -1
}

func normalizeProviderDefinition(id string, input ProviderUpsertInput, createdAt time.Time, updatedAt time.Time) (ProviderDefinition, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return ProviderDefinition{}, fmt.Errorf("provider name is required")
	}
	protocol := strings.ToLower(strings.TrimSpace(input.Protocol))
	if protocol == "" {
		protocol = ProtocolOpenAICompatible
	}
	switch protocol {
	case ProtocolOpenAICompatible, ProtocolArk, ProtocolAnthropic:
	default:
		return ProviderDefinition{}, fmt.Errorf("unsupported provider protocol %q", input.Protocol)
	}
	timeoutSeconds := input.RequestTimeoutSeconds
	if timeoutSeconds <= 0 {
		timeoutSeconds = 60
	}
	return ProviderDefinition{
		ID:                    id,
		Name:                  name,
		BaseURL:               strings.TrimSpace(input.BaseURL),
		Protocol:              protocol,
		RequestTimeoutSeconds: timeoutSeconds,
		Enabled:               input.Enabled,
		APIKeyConfigured:      strings.TrimSpace(input.APIKey) != "",
		APIKeyMasked:          MaskSecret(input.APIKey),
		Headers:               cloneMetadata(input.Headers),
		CreatedAt:             createdAt,
		UpdatedAt:             updatedAt,
	}, nil
}

func normalizeProviderModelRecord(id string, providerID string, input ProviderModelUpsertInput, createdAt time.Time, updatedAt time.Time) (ProviderModelRecord, error) {
	if strings.TrimSpace(input.ModelID) == "" {
		return ProviderModelRecord{}, fmt.Errorf("model_id is required")
	}
	if strings.TrimSpace(input.DisplayName) == "" {
		return ProviderModelRecord{}, fmt.Errorf("display_name is required")
	}
	if strings.TrimSpace(id) == "" {
		id = uuid.NewString()
	}
	return ProviderModelRecord{
		ID:          id,
		ProviderID:  providerID,
		ModelID:     strings.TrimSpace(input.ModelID),
		DisplayName: strings.TrimSpace(input.DisplayName),
		Enabled:     input.Enabled,
		IsDefault:   input.IsDefault,
		IsFallback:  input.IsFallback,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}, nil
}

func cloneProviderDefinition(input ProviderDefinition) ProviderDefinition {
	input.Headers = cloneMetadata(input.Headers)
	input.Models = cloneProviderModels(input.Models)
	return input
}

func cloneProviderModels(input []ProviderModelRecord) []ProviderModelRecord {
	if len(input) == 0 {
		return nil
	}
	result := make([]ProviderModelRecord, len(input))
	copy(result, input)
	return result
}

func cloneProviderModelRecord(input ProviderModelRecord) ProviderModelRecord {
	return input
}

func providerModelsContainDefault(models []ProviderModelRecord) bool {
	for _, model := range models {
		if model.IsDefault {
			return true
		}
	}
	return false
}

func providerModelsContainFallback(models []ProviderModelRecord) bool {
	for _, model := range models {
		if model.IsFallback {
			return true
		}
	}
	return false
}

func cloneMetadata(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	result := make(map[string]string, len(input))
	for key, value := range input {
		result[key] = value
	}
	return result
}
