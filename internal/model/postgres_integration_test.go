package model

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"
)

// TestPostgresStoreIntegrationRoundTrip verifies provider/model governance works against a real PostgreSQL database.
// TestPostgresStoreIntegrationRoundTrip 用于验证供应商模型治理在真实 PostgreSQL 数据库上的完整读写闭环。
func TestPostgresStoreIntegrationRoundTrip(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("ATHENA_PG_TEST_DSN"))
	if dsn == "" {
		t.Skip("skip postgres integration test: ATHENA_PG_TEST_DSN is not set")
	}

	db, err := NewPostgresDB(dsn)
	if err != nil {
		t.Fatalf("NewPostgresDB() error = %v", err)
	}
	store := NewPostgresStore(db, "postgres-integration-encryption-key")
	ctx := context.Background()
	if err := store.AutoMigrate(ctx); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	uniqueSuffix := time.Now().UTC().Format("20060102150405.000000000")
	provider, err := store.CreateProvider(ctx, ProviderUpsertInput{
		Name:                  "itest-provider-" + uniqueSuffix,
		BaseURL:               "https://example.com/v1",
		Protocol:              ProtocolOpenAICompatible,
		APIKey:                "sk-integration",
		RequestTimeoutSeconds: 40,
		Headers: map[string]string{
			"Accept-Encoding": "identity",
		},
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("CreateProvider() error = %v", err)
	}
	defer cleanupPostgresProvider(t, db, provider.ID)

	modelRecord, err := store.CreateProviderModel(ctx, provider.ID, ProviderModelUpsertInput{
		ModelID:     "gpt-4o-mini",
		DisplayName: "GPT-4o Mini",
		Enabled:     true,
		IsDefault:   true,
	})
	if err != nil {
		t.Fatalf("CreateProviderModel() error = %v", err)
	}

	selection, err := store.Resolve(ctx, "")
	if err != nil {
		t.Fatalf("Resolve(default) error = %v", err)
	}
	if selection.Primary.ModelRecordID != modelRecord.ID {
		t.Fatalf("primary model record id = %q, want %q", selection.Primary.ModelRecordID, modelRecord.ID)
	}
	if selection.Primary.APIKey != "sk-integration" {
		t.Fatalf("resolved api key = %q, want %q", selection.Primary.APIKey, "sk-integration")
	}
	if selection.Primary.Headers["Accept-Encoding"] != "identity" {
		t.Fatalf("resolved headers = %#v", selection.Primary.Headers)
	}

	updated, err := store.UpdateProvider(ctx, provider.ID, ProviderUpsertInput{
		Name:                  provider.Name,
		BaseURL:               "https://example.com/v2",
		Protocol:              provider.Protocol,
		RequestTimeoutSeconds: 50,
		Enabled:               true,
	})
	if err != nil {
		t.Fatalf("UpdateProvider() error = %v", err)
	}
	if !updated.APIKeyConfigured || updated.APIKeyMasked == "" {
		t.Fatalf("updated provider should keep masked api key, got %#v", updated)
	}

	explicit, err := store.Resolve(ctx, modelRecord.ID)
	if err != nil {
		t.Fatalf("Resolve(explicit) error = %v", err)
	}
	if explicit.Primary.BaseURL != "https://example.com/v2" {
		t.Fatalf("resolved base url = %q, want %q", explicit.Primary.BaseURL, "https://example.com/v2")
	}
}

// TestPostgresStoreResolveReturnsExpectedErrors verifies postgres-store resolution keeps invalid_model reasons/details stable.
// TestPostgresStoreResolveReturnsExpectedErrors 用于验证 postgres 版解析会稳定返回约定的 invalid_model reason/detail。
func TestPostgresStoreResolveReturnsExpectedErrors(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("ATHENA_PG_TEST_DSN"))
	if dsn == "" {
		t.Skip("skip postgres integration test: ATHENA_PG_TEST_DSN is not set")
	}

	db, err := NewPostgresDB(dsn)
	if err != nil {
		t.Fatalf("NewPostgresDB() error = %v", err)
	}
	store := NewPostgresStore(db, "postgres-integration-encryption-key")
	ctx := context.Background()
	if err := store.AutoMigrate(ctx); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	t.Run("explicit not found", func(t *testing.T) {
		_, err := store.Resolve(ctx, "missing-model")
		assertResolveError(t, err, "not_found", map[string]any{"model_record_id": "missing-model"})
	})

	t.Run("explicit disabled", func(t *testing.T) {
		uniqueSuffix := time.Now().UTC().Format("20060102150405.000000000")
		provider, err := store.CreateProvider(ctx, ProviderUpsertInput{
			Name:                  "itest-disabled-model-provider-" + uniqueSuffix,
			BaseURL:               "https://example.com/v1",
			Protocol:              ProtocolOpenAICompatible,
			APIKey:                "sk-disabled-model",
			RequestTimeoutSeconds: 30,
			Enabled:               true,
		})
		if err != nil {
			t.Fatalf("CreateProvider() error = %v", err)
		}
		defer cleanupPostgresProvider(t, db, provider.ID)

		modelRecord, err := store.CreateProviderModel(ctx, provider.ID, ProviderModelUpsertInput{
			ModelID:     "disabled-model-" + uniqueSuffix,
			DisplayName: "Disabled Model " + uniqueSuffix,
			Enabled:     false,
		})
		if err != nil {
			t.Fatalf("CreateProviderModel() error = %v", err)
		}
		_, err = store.Resolve(ctx, modelRecord.ID)
		assertResolveError(t, err, "disabled", map[string]any{"model_record_id": modelRecord.ID, "provider_id": provider.ID})
	})

	t.Run("explicit provider disabled", func(t *testing.T) {
		uniqueSuffix := time.Now().UTC().Format("20060102150405.000000000")
		provider, err := store.CreateProvider(ctx, ProviderUpsertInput{
			Name:                  "itest-provider-disabled-" + uniqueSuffix,
			BaseURL:               "https://example.com/v1",
			Protocol:              ProtocolOpenAICompatible,
			APIKey:                "sk-provider-disabled",
			RequestTimeoutSeconds: 30,
			Enabled:               true,
		})
		if err != nil {
			t.Fatalf("CreateProvider() error = %v", err)
		}
		defer cleanupPostgresProvider(t, db, provider.ID)

		modelRecord, err := store.CreateProviderModel(ctx, provider.ID, ProviderModelUpsertInput{
			ModelID:     "provider-disabled-model-" + uniqueSuffix,
			DisplayName: "Provider Disabled Model " + uniqueSuffix,
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

	t.Run("default provider disabled", func(t *testing.T) {
		uniqueSuffix := time.Now().UTC().Format("20060102150405.000000000")
		provider, err := store.CreateProvider(ctx, ProviderUpsertInput{
			Name:                  "itest-default-provider-" + uniqueSuffix,
			BaseURL:               "https://example.com/v1",
			Protocol:              ProtocolOpenAICompatible,
			APIKey:                "sk-default-provider-disabled",
			RequestTimeoutSeconds: 30,
			Enabled:               true,
		})
		if err != nil {
			t.Fatalf("CreateProvider() error = %v", err)
		}
		defer cleanupPostgresProvider(t, db, provider.ID)

		_, err = store.CreateProviderModel(ctx, provider.ID, ProviderModelUpsertInput{
			ModelID:     "default-disabled-provider-" + uniqueSuffix,
			DisplayName: "Default Disabled Provider " + uniqueSuffix,
			Enabled:     true,
			IsDefault:   true,
		})
		if err != nil {
			t.Fatalf("CreateProviderModel() error = %v", err)
		}
		if err := db.Model(&PostgresProviderModel{}).Where("id = ?", provider.ID).Update("enabled", false).Error; err != nil {
			t.Fatalf("disable default provider row failed: %v", err)
		}
		_, err = store.Resolve(ctx, "")
		assertResolveError(t, err, "default_provider_disabled", nil)
	})
}

func cleanupPostgresProvider(t *testing.T, db *gorm.DB, providerID string) {
	t.Helper()
	if err := db.Exec(`DELETE FROM model_provider_models WHERE provider_id = ?`, providerID).Error; err != nil {
		t.Fatalf("cleanup model_provider_models failed: %v", err)
	}
	if err := db.Exec(`DELETE FROM model_providers WHERE id = ?`, providerID).Error; err != nil {
		t.Fatalf("cleanup model_providers failed: %v", err)
	}
}
