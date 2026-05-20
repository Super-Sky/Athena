package model

import (
	"context"
	"testing"
)

// TestMemoryStoreResolveDefaultAndFallback verifies one request can resolve both default and technical fallback models.
// TestMemoryStoreResolveDefaultAndFallback 用于验证一次请求可以同时解析出默认模型和技术失败 fallback 模型。
func TestMemoryStoreResolveDefaultAndFallback(t *testing.T) {
	store := NewMemoryStore("memory-store-test-key")
	ctx := context.Background()

	primaryProvider, err := store.CreateProvider(ctx, ProviderUpsertInput{
		Name:                  "openai-primary",
		BaseURL:               "https://example.com/v1",
		Protocol:              ProtocolOpenAICompatible,
		APIKey:                "sk-primary",
		RequestTimeoutSeconds: 45,
		Headers: map[string]string{
			"Accept-Encoding": "identity",
		},
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("CreateProvider(primary) error = %v", err)
	}
	primaryModel, err := store.CreateProviderModel(ctx, primaryProvider.ID, ProviderModelUpsertInput{
		ModelID:     "gpt-4o-mini",
		DisplayName: "GPT-4o Mini",
		Enabled:     true,
		IsDefault:   true,
	})
	if err != nil {
		t.Fatalf("CreateProviderModel(primary) error = %v", err)
	}

	fallbackProvider, err := store.CreateProvider(ctx, ProviderUpsertInput{
		Name:                  "ark-fallback",
		BaseURL:               "https://ark.example.com/api",
		Protocol:              ProtocolArk,
		APIKey:                "sk-fallback",
		RequestTimeoutSeconds: 30,
		Enabled:               true,
	})
	if err != nil {
		t.Fatalf("CreateProvider(fallback) error = %v", err)
	}
	fallbackModel, err := store.CreateProviderModel(ctx, fallbackProvider.ID, ProviderModelUpsertInput{
		ModelID:     "doubao-lite",
		DisplayName: "Doubao Lite",
		Enabled:     true,
		IsFallback:  true,
	})
	if err != nil {
		t.Fatalf("CreateProviderModel(fallback) error = %v", err)
	}

	selection, err := store.Resolve(ctx, "")
	if err != nil {
		t.Fatalf("Resolve(default) error = %v", err)
	}
	if selection.Primary.ModelRecordID != primaryModel.ID {
		t.Fatalf("primary model record id = %q, want %q", selection.Primary.ModelRecordID, primaryModel.ID)
	}
	if selection.Primary.Headers["Accept-Encoding"] != "identity" {
		t.Fatalf("primary headers = %#v", selection.Primary.Headers)
	}
	if selection.Fallback == nil || selection.Fallback.ModelRecordID != fallbackModel.ID {
		t.Fatalf("fallback = %#v, want model %q", selection.Fallback, fallbackModel.ID)
	}

	explicit, err := store.Resolve(ctx, primaryModel.ID)
	if err != nil {
		t.Fatalf("Resolve(explicit) error = %v", err)
	}
	if !explicit.Explicit {
		t.Fatalf("expected explicit selection")
	}
	if explicit.Fallback != nil {
		t.Fatalf("explicit selection should not include fallback, got %#v", explicit.Fallback)
	}
}

// TestMemoryStoreResolveReturnsExpectedErrors verifies memory-store resolution keeps invalid_model reasons/details stable.
// TestMemoryStoreResolveReturnsExpectedErrors 用于验证内存版解析会稳定返回约定的 invalid_model reason/detail。
func TestMemoryStoreResolveReturnsExpectedErrors(t *testing.T) {
	ctx := context.Background()

	t.Run("explicit not found", func(t *testing.T) {
		store := NewMemoryStore("memory-store-test-key")
		_, err := store.Resolve(ctx, "missing-model")
		assertResolveError(t, err, "not_found", map[string]any{"model_record_id": "missing-model"})
	})

	t.Run("explicit disabled", func(t *testing.T) {
		store := NewMemoryStore("memory-store-test-key")
		provider, err := store.CreateProvider(ctx, ProviderUpsertInput{
			Name:                  "disabled-model-provider",
			BaseURL:               "https://example.com/v1",
			Protocol:              ProtocolOpenAICompatible,
			APIKey:                "sk-disabled-model",
			RequestTimeoutSeconds: 30,
			Enabled:               true,
		})
		if err != nil {
			t.Fatalf("CreateProvider() error = %v", err)
		}
		modelRecord, err := store.CreateProviderModel(ctx, provider.ID, ProviderModelUpsertInput{
			ModelID:     "disabled-model",
			DisplayName: "Disabled Model",
			Enabled:     false,
		})
		if err != nil {
			t.Fatalf("CreateProviderModel() error = %v", err)
		}
		_, err = store.Resolve(ctx, modelRecord.ID)
		assertResolveError(t, err, "disabled", map[string]any{"model_record_id": modelRecord.ID, "provider_id": provider.ID})
	})

	t.Run("explicit provider disabled", func(t *testing.T) {
		store := NewMemoryStore("memory-store-test-key")
		provider, err := store.CreateProvider(ctx, ProviderUpsertInput{
			Name:                  "provider-to-disable",
			BaseURL:               "https://example.com/v1",
			Protocol:              ProtocolOpenAICompatible,
			APIKey:                "sk-disabled-provider",
			RequestTimeoutSeconds: 30,
			Enabled:               true,
		})
		if err != nil {
			t.Fatalf("CreateProvider() error = %v", err)
		}
		modelRecord, err := store.CreateProviderModel(ctx, provider.ID, ProviderModelUpsertInput{
			ModelID:     "provider-disabled-model",
			DisplayName: "Provider Disabled Model",
			Enabled:     true,
		})
		if err != nil {
			t.Fatalf("CreateProviderModel() error = %v", err)
		}
		disabled := false
		if _, err := store.PatchProvider(ctx, provider.ID, ProviderPatchInput{Enabled: &disabled}); err != nil {
			t.Fatalf("PatchProvider() error = %v", err)
		}
		_, err = store.Resolve(ctx, modelRecord.ID)
		assertResolveError(t, err, "provider_disabled", map[string]any{"model_record_id": modelRecord.ID, "provider_id": provider.ID})
	})

	t.Run("default missing", func(t *testing.T) {
		store := NewMemoryStore("memory-store-test-key")
		_, err := store.Resolve(ctx, "")
		assertResolveError(t, err, "default_missing", map[string]any{"selection": "default"})
	})
}

// TestMemoryStoreUpdateProviderPreservesSecret verifies empty API key updates keep the stored encrypted secret unchanged.
// TestMemoryStoreUpdateProviderPreservesSecret 用于验证更新供应商时若未重传 API key，会保留已存储的加密密钥。
func TestMemoryStoreUpdateProviderPreservesSecret(t *testing.T) {
	store := NewMemoryStore("memory-store-test-key")
	ctx := context.Background()

	provider, err := store.CreateProvider(ctx, ProviderUpsertInput{
		Name:                  "updatable-provider",
		BaseURL:               "https://example.com/v1",
		Protocol:              ProtocolOpenAICompatible,
		APIKey:                "sk-original",
		RequestTimeoutSeconds: 30,
		Enabled:               true,
	})
	if err != nil {
		t.Fatalf("CreateProvider() error = %v", err)
	}
	modelRecord, err := store.CreateProviderModel(ctx, provider.ID, ProviderModelUpsertInput{
		ModelID:     "gpt-4o",
		DisplayName: "GPT-4o",
		Enabled:     true,
		IsDefault:   true,
	})
	if err != nil {
		t.Fatalf("CreateProviderModel() error = %v", err)
	}

	updated, err := store.UpdateProvider(ctx, provider.ID, ProviderUpsertInput{
		Name:                  "updatable-provider",
		BaseURL:               "https://example.com/v2",
		Protocol:              ProtocolOpenAICompatible,
		RequestTimeoutSeconds: 60,
		Headers: map[string]string{
			"Accept-Encoding": "identity",
		},
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("UpdateProvider() error = %v", err)
	}
	if !updated.APIKeyConfigured || updated.APIKeyMasked == "" {
		t.Fatalf("updated provider should keep masked api key, got %#v", updated)
	}

	selection, err := store.Resolve(ctx, modelRecord.ID)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if selection.Primary.APIKey != "sk-original" {
		t.Fatalf("resolved api key = %q, want %q", selection.Primary.APIKey, "sk-original")
	}
	if selection.Primary.BaseURL != "https://example.com/v2" {
		t.Fatalf("resolved base url = %q, want %q", selection.Primary.BaseURL, "https://example.com/v2")
	}
}

// TestMemoryStoreDeleteMissingResources verifies delete paths report explicit not-found errors.
// TestMemoryStoreDeleteMissingResources 用于验证删除路径会对缺失资源返回明确的 not-found 错误。
func TestMemoryStoreDeleteMissingResources(t *testing.T) {
	store := NewMemoryStore("memory-store-test-key")
	ctx := context.Background()

	if err := store.DeleteProvider(ctx, "missing-provider"); err == nil || err != ErrModelProviderNotFound {
		t.Fatalf("DeleteProvider(missing) error = %v, want %v", err, ErrModelProviderNotFound)
	}

	provider, err := store.CreateProvider(ctx, ProviderUpsertInput{
		Name:     "delete-missing-provider-model",
		Protocol: ProtocolOpenAICompatible,
		APIKey:   "sk-demo",
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("CreateProvider() error = %v", err)
	}

	if err := store.DeleteProviderModel(ctx, "missing-provider", "missing-model"); err == nil || err != ErrModelProviderNotFound {
		t.Fatalf("DeleteProviderModel(missing provider) error = %v, want %v", err, ErrModelProviderNotFound)
	}
	if err := store.DeleteProviderModel(ctx, provider.ID, "missing-model"); err == nil || err != ErrProviderModelNotFound {
		t.Fatalf("DeleteProviderModel(missing model) error = %v, want %v", err, ErrProviderModelNotFound)
	}
}

func assertResolveError(t *testing.T, err error, wantReason string, wantDetail map[string]any) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected resolve error")
	}
	resolvedErr, ok := err.(*ResolveError)
	if !ok {
		t.Fatalf("expected *ResolveError, got %T", err)
	}
	if resolvedErr.Code != "invalid_model" {
		t.Fatalf("resolve error code = %q, want invalid_model", resolvedErr.Code)
	}
	if resolvedErr.Reason != wantReason {
		t.Fatalf("resolve error reason = %q, want %q", resolvedErr.Reason, wantReason)
	}
	for key, wantValue := range wantDetail {
		if got := resolvedErr.Detail[key]; got != wantValue {
			t.Fatalf("resolve error detail[%q] = %#v, want %#v", key, got, wantValue)
		}
	}
}

// BenchmarkMemoryStoreResolveDefault keeps a baseline for the in-memory provider-model resolution path.
// BenchmarkMemoryStoreResolveDefault 为内存版供应商模型解析路径建立性能基线。
func BenchmarkMemoryStoreResolveDefault(b *testing.B) {
	store := NewMemoryStore("memory-store-bench-key")
	ctx := context.Background()

	provider, err := store.CreateProvider(ctx, ProviderUpsertInput{
		Name:                  "bench-provider",
		BaseURL:               "https://example.com/v1",
		Protocol:              ProtocolOpenAICompatible,
		APIKey:                "sk-bench",
		RequestTimeoutSeconds: 30,
		Enabled:               true,
	})
	if err != nil {
		b.Fatalf("CreateProvider() error = %v", err)
	}
	if _, err := store.CreateProviderModel(ctx, provider.ID, ProviderModelUpsertInput{
		ModelID:     "gpt-4o-mini",
		DisplayName: "GPT-4o Mini",
		Enabled:     true,
		IsDefault:   true,
	}); err != nil {
		b.Fatalf("CreateProviderModel() error = %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		selection, err := store.Resolve(ctx, "")
		if err != nil {
			b.Fatalf("Resolve() error = %v", err)
		}
		if selection.Primary.ProviderModelID == "" {
			b.Fatalf("expected resolved provider model id")
		}
	}
}
