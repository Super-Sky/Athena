// postgres_store.go implements the PostgreSQL-backed model governance store.
// postgres_store.go 实现基于 PostgreSQL 的模型治理存储。
package model

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"moss/internal/postgresutil"
)

// PostgresProviderModel keeps one provider definition persisted in PostgreSQL.
// PostgresProviderModel 用于在 PostgreSQL 中持久化一条供应商定义。
type PostgresProviderModel struct {
	ID                    string    `gorm:"column:id;type:text;primaryKey"`
	Name                  string    `gorm:"column:name;type:text;not null;uniqueIndex:uk_model_provider_name"`
	BaseURL               string    `gorm:"column:base_url;type:text;not null;default:''"`
	Protocol              string    `gorm:"column:protocol;type:text;not null"`
	RequestTimeoutSeconds int       `gorm:"column:request_timeout_seconds;type:integer;not null;default:60"`
	Enabled               bool      `gorm:"column:enabled;type:boolean;not null;default:true"`
	EncryptedAPIKey       string    `gorm:"column:encrypted_api_key;type:text;not null;default:''"`
	APIKeyMasked          string    `gorm:"column:api_key_masked;type:text;not null;default:''"`
	HeadersJSON           []byte    `gorm:"column:headers_json;type:jsonb;not null"`
	CreatedAt             time.Time `gorm:"column:created_at;type:timestamptz;not null;autoCreateTime"`
	UpdatedAt             time.Time `gorm:"column:updated_at;type:timestamptz;not null;autoUpdateTime;index:idx_model_providers_updated_at"`
}

// TableName keeps provider definitions in the dedicated governance table.
// TableName 用于把供应商定义固定到专门的治理表。
func (PostgresProviderModel) TableName() string {
	return "model_providers"
}

// PostgresProviderModelEntry keeps one child model row under a provider in PostgreSQL.
// PostgresProviderModelEntry 用于在 PostgreSQL 中保存供应商下的一条模型子项。
type PostgresProviderModelEntry struct {
	ID          string    `gorm:"column:id;type:text;primaryKey"`
	ProviderID  string    `gorm:"column:provider_id;type:text;not null;index:idx_provider_models_provider_id"`
	ModelID     string    `gorm:"column:model_id;type:text;not null"`
	DisplayName string    `gorm:"column:display_name;type:text;not null"`
	Enabled     bool      `gorm:"column:enabled;type:boolean;not null;default:true"`
	IsDefault   bool      `gorm:"column:is_default;type:boolean;not null;default:false"`
	IsFallback  bool      `gorm:"column:is_fallback;type:boolean;not null;default:false"`
	CreatedAt   time.Time `gorm:"column:created_at;type:timestamptz;not null;autoCreateTime"`
	UpdatedAt   time.Time `gorm:"column:updated_at;type:timestamptz;not null;autoUpdateTime;index:idx_provider_models_updated_at"`
}

// TableName keeps child model entries in their own governance table.
// TableName 用于把模型子项固定到独立的治理表。
func (PostgresProviderModelEntry) TableName() string {
	return "model_provider_models"
}

// PostgresStore keeps provider and model governance state in PostgreSQL.
// PostgresStore 会把供应商和模型治理状态保存在 PostgreSQL 中。
type PostgresStore struct {
	db            *gorm.DB
	encryptionKey string
}

const defaultWriteRetryAttempts = 3

// NewPostgresDB opens a PostgreSQL-backed GORM connection for model governance.
// NewPostgresDB 会创建一个面向模型治理的 PostgreSQL GORM 连接。
func NewPostgresDB(dsn string, opts ...func(*gorm.Config)) (*gorm.DB, error) {
	cfg := &gorm.Config{}
	for _, opt := range opts {
		if opt != nil {
			opt(cfg)
		}
	}
	return gorm.Open(postgres.Open(dsn), cfg)
}

// NewPostgresStore creates one PostgreSQL-backed governance store.
// NewPostgresStore 会创建一个 PostgreSQL 版治理 store。
func NewPostgresStore(db *gorm.DB, encryptionKey string) *PostgresStore {
	return &PostgresStore{db: db, encryptionKey: encryptionKey}
}

// AutoMigrate creates or updates the provider and provider-model tables plus uniqueness constraints.
// AutoMigrate 会创建或更新供应商表、模型子项表以及相关唯一约束。
func (s *PostgresStore) AutoMigrate(ctx context.Context) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("postgres model store is not configured")
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.AutoMigrate(&PostgresProviderModel{}, &PostgresProviderModelEntry{}); err != nil {
			return err
		}
		for _, statement := range []string{
			`CREATE UNIQUE INDEX IF NOT EXISTS uk_provider_models_provider_model_id ON model_provider_models (provider_id, model_id)`,
			`CREATE UNIQUE INDEX IF NOT EXISTS uk_provider_models_provider_display_name ON model_provider_models (provider_id, display_name)`,
			`CREATE UNIQUE INDEX IF NOT EXISTS uk_provider_models_one_default ON model_provider_models ((is_default)) WHERE is_default = true`,
			`CREATE UNIQUE INDEX IF NOT EXISTS uk_provider_models_one_fallback ON model_provider_models ((is_fallback)) WHERE is_fallback = true`,
		} {
			if err := tx.Exec(statement).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// ListProviders returns every provider row plus its child model rows.
// ListProviders 会返回全部供应商行以及对应的模型子项行。
func (s *PostgresStore) ListProviders(ctx context.Context) ([]ProviderDefinition, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("postgres model store is not configured")
	}
	var providerRows []PostgresProviderModel
	if err := s.db.WithContext(ctx).Order("name asc").Find(&providerRows).Error; err != nil {
		return nil, err
	}
	providerIDs := make([]string, 0, len(providerRows))
	for _, row := range providerRows {
		providerIDs = append(providerIDs, row.ID)
	}
	modelRowsByProvider, err := s.loadProviderModels(ctx, providerIDs)
	if err != nil {
		return nil, err
	}
	result := make([]ProviderDefinition, 0, len(providerRows))
	for _, row := range providerRows {
		result = append(result, providerRowToDefinition(row, modelRowsByProvider[row.ID]))
	}
	return result, nil
}

// CreateProvider inserts one provider definition with an encrypted API key.
// CreateProvider 会插入一条带加密 API key 的供应商定义。
func (s *PostgresStore) CreateProvider(ctx context.Context, input ProviderUpsertInput) (ProviderDefinition, error) {
	if s == nil || s.db == nil {
		return ProviderDefinition{}, fmt.Errorf("postgres model store is not configured")
	}
	var output ProviderDefinition
	err := postgresutil.WithRetry(ctx, defaultWriteRetryAttempts, func() error {
		return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			definition, encrypted, err := s.normalizeCreateProvider(input)
			if err != nil {
				return err
			}
			row, err := providerDefinitionToRow(definition, encrypted)
			if err != nil {
				return err
			}
			if err := createPostgresProviderRow(tx, row); err != nil {
				return err
			}
			modelRows := make([]PostgresProviderModelEntry, 0, len(definition.Models))
			if providerModelsContainDefault(definition.Models) {
				if err := tx.Model(&PostgresProviderModelEntry{}).Where("is_default = ?", true).Update("is_default", false).Error; err != nil {
					return err
				}
			}
			if providerModelsContainFallback(definition.Models) {
				if err := tx.Model(&PostgresProviderModelEntry{}).Where("is_fallback = ?", true).Update("is_fallback", false).Error; err != nil {
					return err
				}
			}
			for _, item := range definition.Models {
				modelRow := providerModelRecordToRow(item)
				if err := createPostgresProviderModelRow(tx, modelRow); err != nil {
					return err
				}
				modelRows = append(modelRows, modelRow)
			}
			output = providerRowToDefinition(row, modelRows)
			return nil
		})
	})
	return output, err
}

// UpdateProvider replaces one provider definition while preserving its child models.
// UpdateProvider 会整体替换一条供应商定义，并保留其子模型列表。
func (s *PostgresStore) UpdateProvider(ctx context.Context, id string, input ProviderUpsertInput) (ProviderDefinition, error) {
	if s == nil || s.db == nil {
		return ProviderDefinition{}, fmt.Errorf("postgres model store is not configured")
	}
	var output ProviderDefinition
	err := postgresutil.WithRetry(ctx, defaultWriteRetryAttempts, func() error {
		return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			existing, err := s.loadProviderRow(tx, id)
			if err != nil {
				return err
			}
			definition, encrypted, err := s.normalizeUpdateProvider(existing, input)
			if err != nil {
				return err
			}
			row, err := providerDefinitionToRow(definition, encrypted)
			if err != nil {
				return err
			}
			if err := tx.Model(&PostgresProviderModel{}).Where("id = ?", row.ID).Updates(map[string]any{
				"name":                    row.Name,
				"base_url":                row.BaseURL,
				"protocol":                row.Protocol,
				"request_timeout_seconds": row.RequestTimeoutSeconds,
				"enabled":                 row.Enabled,
				"encrypted_api_key":       row.EncryptedAPIKey,
				"api_key_masked":          row.APIKeyMasked,
				"headers_json":            row.HeadersJSON,
				"updated_at":              row.UpdatedAt,
			}).Error; err != nil {
				return err
			}
			modelRows, err := s.loadProviderModelRows(tx, row.ID)
			if err != nil {
				return err
			}
			output = providerRowToDefinition(row, modelRows)
			return nil
		})
	})
	return output, err
}

// PatchProvider applies provider-level governance toggles such as enabled.
// PatchProvider 会应用供应商级别的治理开关，例如启用状态。
func (s *PostgresStore) PatchProvider(ctx context.Context, id string, input ProviderPatchInput) (ProviderDefinition, error) {
	if s == nil || s.db == nil {
		return ProviderDefinition{}, fmt.Errorf("postgres model store is not configured")
	}
	var output ProviderDefinition
	err := postgresutil.WithRetry(ctx, defaultWriteRetryAttempts, func() error {
		return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			providerRow, err := s.loadProviderRow(tx, id)
			if err != nil {
				return err
			}
			modelRows, err := s.loadProviderModelRows(tx, providerRow.ID)
			if err != nil {
				return err
			}
			if input.Enabled != nil {
				if !*input.Enabled && rowsOwnProtectedModel(modelRows) {
					return fmt.Errorf("provider owns the current default or fallback model and cannot be disabled")
				}
				providerRow.Enabled = *input.Enabled
			}
			providerRow.UpdatedAt = time.Now().UTC()
			if err := tx.Model(&PostgresProviderModel{}).Where("id = ?", providerRow.ID).Updates(map[string]any{
				"enabled":    providerRow.Enabled,
				"updated_at": providerRow.UpdatedAt,
			}).Error; err != nil {
				return err
			}
			output = providerRowToDefinition(providerRow, modelRows)
			return nil
		})
	})
	return output, err
}

// DeleteProvider removes one provider when it no longer owns default/fallback model entries.
// DeleteProvider 会在供应商不再拥有 default 或 fallback 模型条目时删除该供应商。
func (s *PostgresStore) DeleteProvider(ctx context.Context, id string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("postgres model store is not configured")
	}
	return postgresutil.WithRetry(ctx, defaultWriteRetryAttempts, func() error {
		return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			providerRow, err := s.loadProviderRow(tx, id)
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return ErrModelProviderNotFound
				}
				return err
			}
			modelRows, err := s.loadProviderModelRows(tx, providerRow.ID)
			if err != nil {
				return err
			}
			if rowsOwnProtectedModel(modelRows) {
				return fmt.Errorf("provider owns the current default or fallback model and cannot be deleted")
			}
			if err := tx.Where("provider_id = ?", providerRow.ID).Delete(&PostgresProviderModelEntry{}).Error; err != nil {
				return err
			}
			return tx.Delete(&PostgresProviderModel{}, "id = ?", providerRow.ID).Error
		})
	})
}

// CreateProviderModel creates one child model entry under a provider.
// CreateProviderModel 会在某个供应商下创建一条模型子项。
func (s *PostgresStore) CreateProviderModel(ctx context.Context, providerID string, input ProviderModelUpsertInput) (ProviderModelRecord, error) {
	if s == nil || s.db == nil {
		return ProviderModelRecord{}, fmt.Errorf("postgres model store is not configured")
	}
	var output ProviderModelRecord
	err := postgresutil.WithRetry(ctx, defaultWriteRetryAttempts, func() error {
		return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if _, err := s.loadProviderRow(tx, providerID); err != nil {
				return err
			}
			record, err := normalizeProviderModelRecord("", strings.TrimSpace(providerID), input, time.Now().UTC(), time.Now().UTC())
			if err != nil {
				return err
			}
			if record.IsDefault {
				if err := tx.Model(&PostgresProviderModelEntry{}).Where("is_default = ?", true).Update("is_default", false).Error; err != nil {
					return err
				}
			}
			if record.IsFallback {
				if err := tx.Model(&PostgresProviderModelEntry{}).Where("is_fallback = ?", true).Update("is_fallback", false).Error; err != nil {
					return err
				}
			}
			row := providerModelRecordToRow(record)
			if err := createPostgresProviderModelRow(tx, row); err != nil {
				return err
			}
			output = providerModelRowToDefinition(row)
			return nil
		})
	})
	return output, err
}

// UpdateProviderModel replaces one child model entry while preserving its stable record id.
// UpdateProviderModel 会整体替换一条模型子项，同时保留其稳定记录 id。
func (s *PostgresStore) UpdateProviderModel(ctx context.Context, providerID string, modelRecordID string, input ProviderModelUpsertInput) (ProviderModelRecord, error) {
	if s == nil || s.db == nil {
		return ProviderModelRecord{}, fmt.Errorf("postgres model store is not configured")
	}
	var output ProviderModelRecord
	err := postgresutil.WithRetry(ctx, defaultWriteRetryAttempts, func() error {
		return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if _, err := s.loadProviderRow(tx, providerID); err != nil {
				return err
			}
			existing, err := s.loadProviderModelRow(tx, providerID, modelRecordID)
			if err != nil {
				return err
			}
			record, err := normalizeProviderModelRecord(existing.ID, existing.ProviderID, input, existing.CreatedAt, time.Now().UTC())
			if err != nil {
				return err
			}
			if record.IsDefault {
				if err := tx.Model(&PostgresProviderModelEntry{}).Where("is_default = ? AND id <> ?", true, record.ID).Update("is_default", false).Error; err != nil {
					return err
				}
			}
			if record.IsFallback {
				if err := tx.Model(&PostgresProviderModelEntry{}).Where("is_fallback = ? AND id <> ?", true, record.ID).Update("is_fallback", false).Error; err != nil {
					return err
				}
			}
			row := providerModelRecordToRow(record)
			if err := tx.Model(&PostgresProviderModelEntry{}).Where("id = ?", row.ID).Updates(map[string]any{
				"model_id":     row.ModelID,
				"display_name": row.DisplayName,
				"enabled":      row.Enabled,
				"is_default":   row.IsDefault,
				"is_fallback":  row.IsFallback,
				"updated_at":   row.UpdatedAt,
			}).Error; err != nil {
				return err
			}
			output = providerModelRowToDefinition(row)
			return nil
		})
	})
	return output, err
}

// PatchProviderModel applies enable/default/fallback toggles on one child model entry.
// PatchProviderModel 会对一条模型子项应用启用、default、fallback 开关。
func (s *PostgresStore) PatchProviderModel(ctx context.Context, providerID string, modelRecordID string, input ProviderModelPatchInput) (ProviderModelRecord, error) {
	if s == nil || s.db == nil {
		return ProviderModelRecord{}, fmt.Errorf("postgres model store is not configured")
	}
	var output ProviderModelRecord
	err := postgresutil.WithRetry(ctx, defaultWriteRetryAttempts, func() error {
		return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			row, err := s.loadProviderModelRow(tx, providerID, modelRecordID)
			if err != nil {
				return err
			}
			if input.Enabled != nil {
				if !*input.Enabled && (row.IsDefault || row.IsFallback) {
					return fmt.Errorf("default or fallback model must be reassigned before disable")
				}
				row.Enabled = *input.Enabled
			}
			if input.IsDefault != nil {
				if *input.IsDefault {
					row.Enabled = true
					if err := tx.Model(&PostgresProviderModelEntry{}).Where("is_default = ? AND id <> ?", true, row.ID).Update("is_default", false).Error; err != nil {
						return err
					}
					row.IsDefault = true
				} else if row.IsDefault {
					return fmt.Errorf("default model must be reassigned before unset")
				}
			}
			if input.IsFallback != nil {
				if *input.IsFallback {
					row.Enabled = true
					if err := tx.Model(&PostgresProviderModelEntry{}).Where("is_fallback = ? AND id <> ?", true, row.ID).Update("is_fallback", false).Error; err != nil {
						return err
					}
					row.IsFallback = true
				} else if row.IsFallback {
					return fmt.Errorf("fallback model must be reassigned before unset")
				}
			}
			row.UpdatedAt = time.Now().UTC()
			if err := tx.Model(&PostgresProviderModelEntry{}).Where("id = ?", row.ID).Updates(map[string]any{
				"enabled":     row.Enabled,
				"is_default":  row.IsDefault,
				"is_fallback": row.IsFallback,
				"updated_at":  row.UpdatedAt,
			}).Error; err != nil {
				return err
			}
			output = providerModelRowToDefinition(row)
			return nil
		})
	})
	return output, err
}

// DeleteProviderModel removes one child model entry after protected roles are reassigned.
// DeleteProviderModel 会在受保护角色完成重分配后删除一条模型子项。
func (s *PostgresStore) DeleteProviderModel(ctx context.Context, providerID string, modelRecordID string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("postgres model store is not configured")
	}
	return postgresutil.WithRetry(ctx, defaultWriteRetryAttempts, func() error {
		return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			row, err := s.loadProviderModelRow(tx, providerID, modelRecordID)
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return ErrProviderModelNotFound
				}
				return err
			}
			if row.IsDefault || row.IsFallback {
				return fmt.Errorf("default or fallback model must be reassigned before delete")
			}
			return tx.Delete(&PostgresProviderModelEntry{}, "id = ?", row.ID).Error
		})
	})
}

// Resolve selects either the explicitly requested model or the globally configured default model.
// Resolve 会选择显式请求的模型，或者全局配置的默认模型。
func (s *PostgresStore) Resolve(ctx context.Context, requestedModelRecordID string) (*Selection, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("postgres model store is not configured")
	}
	if trimmed := strings.TrimSpace(requestedModelRecordID); trimmed != "" {
		row, provider, err := s.loadResolvedRow(ctx, trimmed)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, &ResolveError{Code: "invalid_model", Reason: "not_found", Message: "requested model was not found", Detail: map[string]any{"model_record_id": trimmed}}
			}
			return nil, err
		}
		if !provider.Enabled {
			return nil, &ResolveError{Code: "invalid_model", Reason: "provider_disabled", Message: "requested model provider is disabled", Detail: map[string]any{"model_record_id": trimmed, "provider_id": provider.ID}}
		}
		if !row.Enabled {
			return nil, &ResolveError{Code: "invalid_model", Reason: "disabled", Message: "requested model is disabled", Detail: map[string]any{"model_record_id": trimmed, "provider_id": provider.ID}}
		}
		primary, err := s.toChatConfig(provider, row)
		if err != nil {
			return nil, err
		}
		return &Selection{
			Primary:      primary,
			Explicit:     true,
			PrimaryMeta:  providerModelRowToDefinition(row),
			ProviderMeta: providerRowToDefinition(provider, nil),
		}, nil
	}

	var defaultRow PostgresProviderModelEntry
	if err := s.db.WithContext(ctx).First(&defaultRow, "is_default = ? AND enabled = ?", true, true).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &ResolveError{Code: "invalid_model", Reason: "default_missing", Message: "default model is not configured", Detail: map[string]any{"selection": "default"}}
		}
		return nil, err
	}
	var defaultProvider PostgresProviderModel
	if err := s.db.WithContext(ctx).First(&defaultProvider, "id = ?", defaultRow.ProviderID).Error; err != nil {
		return nil, err
	}
	if !defaultProvider.Enabled {
		return nil, &ResolveError{Code: "invalid_model", Reason: "default_provider_disabled", Message: "default model provider is disabled"}
	}
	primary, err := s.toChatConfig(defaultProvider, defaultRow)
	if err != nil {
		return nil, err
	}
	selection := &Selection{
		Primary:      primary,
		Explicit:     false,
		PrimaryMeta:  providerModelRowToDefinition(defaultRow),
		ProviderMeta: providerRowToDefinition(defaultProvider, nil),
	}

	var fallbackRow PostgresProviderModelEntry
	if err := s.db.WithContext(ctx).First(&fallbackRow, "is_fallback = ? AND enabled = ? AND id <> ?", true, true, defaultRow.ID).Error; err == nil {
		var fallbackProvider PostgresProviderModel
		if err := s.db.WithContext(ctx).First(&fallbackProvider, "id = ?", fallbackRow.ProviderID).Error; err == nil && fallbackProvider.Enabled {
			fallback, err := s.toChatConfig(fallbackProvider, fallbackRow)
			if err != nil {
				return nil, err
			}
			meta := providerModelRowToDefinition(fallbackRow)
			selection.Fallback = &fallback
			selection.FallbackMeta = &meta
		}
	}
	return selection, nil
}

func (s *PostgresStore) normalizeCreateProvider(input ProviderUpsertInput) (ProviderDefinition, string, error) {
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

func (s *PostgresStore) normalizeUpdateProvider(existing PostgresProviderModel, input ProviderUpsertInput) (ProviderDefinition, string, error) {
	apiKey := strings.TrimSpace(input.APIKey)
	encrypted := existing.EncryptedAPIKey
	var err error
	if apiKey != "" {
		encrypted, err = EncryptSecret(apiKey, s.encryptionKey)
		if err != nil {
			return ProviderDefinition{}, "", err
		}
	}
	definition, err := normalizeProviderDefinition(existing.ID, input, existing.CreatedAt, time.Now().UTC())
	if err != nil {
		return ProviderDefinition{}, "", err
	}
	if apiKey == "" {
		definition.APIKeyConfigured = strings.TrimSpace(existing.EncryptedAPIKey) != ""
		definition.APIKeyMasked = existing.APIKeyMasked
	}
	return definition, encrypted, nil
}

func (s *PostgresStore) loadProviderRow(db *gorm.DB, id string) (PostgresProviderModel, error) {
	var row PostgresProviderModel
	err := db.First(&row, "id = ?", strings.TrimSpace(id)).Error
	return row, err
}

func (s *PostgresStore) loadProviderModelRow(db *gorm.DB, providerID string, modelRecordID string) (PostgresProviderModelEntry, error) {
	var row PostgresProviderModelEntry
	err := db.First(&row, "provider_id = ? AND id = ?", strings.TrimSpace(providerID), strings.TrimSpace(modelRecordID)).Error
	return row, err
}

func (s *PostgresStore) loadProviderModelRows(db *gorm.DB, providerID string) ([]PostgresProviderModelEntry, error) {
	var rows []PostgresProviderModelEntry
	err := db.Order("display_name asc").Find(&rows, "provider_id = ?", strings.TrimSpace(providerID)).Error
	return rows, err
}

func (s *PostgresStore) loadProviderModels(ctx context.Context, providerIDs []string) (map[string][]PostgresProviderModelEntry, error) {
	result := map[string][]PostgresProviderModelEntry{}
	if len(providerIDs) == 0 {
		return result, nil
	}
	var rows []PostgresProviderModelEntry
	if err := s.db.WithContext(ctx).Order("display_name asc").Find(&rows, "provider_id IN ?", providerIDs).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.ProviderID] = append(result[row.ProviderID], row)
	}
	return result, nil
}

func (s *PostgresStore) loadResolvedRow(ctx context.Context, modelRecordID string) (PostgresProviderModelEntry, PostgresProviderModel, error) {
	var modelRow PostgresProviderModelEntry
	if err := s.db.WithContext(ctx).First(&modelRow, "id = ?", strings.TrimSpace(modelRecordID)).Error; err != nil {
		return PostgresProviderModelEntry{}, PostgresProviderModel{}, err
	}
	var providerRow PostgresProviderModel
	if err := s.db.WithContext(ctx).First(&providerRow, "id = ?", modelRow.ProviderID).Error; err != nil {
		return PostgresProviderModelEntry{}, PostgresProviderModel{}, err
	}
	return modelRow, providerRow, nil
}

func (s *PostgresStore) toChatConfig(providerRow PostgresProviderModel, modelRow PostgresProviderModelEntry) (ChatConfig, error) {
	apiKey, err := DecryptSecret(providerRow.EncryptedAPIKey, s.encryptionKey)
	if err != nil {
		return ChatConfig{}, err
	}
	headers, err := decodeHeaders(providerRow.HeadersJSON)
	if err != nil {
		return ChatConfig{}, err
	}
	return ChatConfig{
		ProviderID:       providerRow.ID,
		ProviderName:     providerRow.Name,
		ProviderProtocol: providerRow.Protocol,
		BaseURL:          providerRow.BaseURL,
		RequestTimeout:   time.Duration(providerRow.RequestTimeoutSeconds) * time.Second,
		APIKey:           apiKey,
		Headers:          headers,
		ModelRecordID:    modelRow.ID,
		ProviderModelID:  modelRow.ModelID,
		ModelDisplayName: modelRow.DisplayName,
	}, nil
}

func providerDefinitionToRow(input ProviderDefinition, encryptedAPIKey string) (PostgresProviderModel, error) {
	headersJSON, err := json.Marshal(cloneHeaders(input.Headers))
	if err != nil {
		return PostgresProviderModel{}, err
	}
	return PostgresProviderModel{
		ID:                    input.ID,
		Name:                  input.Name,
		BaseURL:               input.BaseURL,
		Protocol:              input.Protocol,
		RequestTimeoutSeconds: input.RequestTimeoutSeconds,
		Enabled:               input.Enabled,
		EncryptedAPIKey:       encryptedAPIKey,
		APIKeyMasked:          input.APIKeyMasked,
		HeadersJSON:           headersJSON,
		CreatedAt:             input.CreatedAt,
		UpdatedAt:             input.UpdatedAt,
	}, nil
}

func providerRowToDefinition(row PostgresProviderModel, modelRows []PostgresProviderModelEntry) ProviderDefinition {
	headers, _ := decodeHeaders(row.HeadersJSON)
	definition := ProviderDefinition{
		ID:                    row.ID,
		Name:                  row.Name,
		BaseURL:               row.BaseURL,
		Protocol:              row.Protocol,
		RequestTimeoutSeconds: row.RequestTimeoutSeconds,
		Enabled:               row.Enabled,
		APIKeyConfigured:      strings.TrimSpace(row.EncryptedAPIKey) != "",
		APIKeyMasked:          row.APIKeyMasked,
		Headers:               headers,
		CreatedAt:             row.CreatedAt,
		UpdatedAt:             row.UpdatedAt,
	}
	if len(modelRows) > 0 {
		definition.Models = make([]ProviderModelRecord, 0, len(modelRows))
		for _, item := range modelRows {
			definition.Models = append(definition.Models, providerModelRowToDefinition(item))
		}
	}
	return definition
}

func providerModelRecordToRow(input ProviderModelRecord) PostgresProviderModelEntry {
	return PostgresProviderModelEntry{
		ID:          input.ID,
		ProviderID:  input.ProviderID,
		ModelID:     input.ModelID,
		DisplayName: input.DisplayName,
		Enabled:     input.Enabled,
		IsDefault:   input.IsDefault,
		IsFallback:  input.IsFallback,
		CreatedAt:   input.CreatedAt,
		UpdatedAt:   input.UpdatedAt,
	}
}

func createPostgresProviderRow(tx *gorm.DB, row PostgresProviderModel) error {
	return tx.Model(&PostgresProviderModel{}).Create(map[string]any{
		"id":                      row.ID,
		"name":                    row.Name,
		"base_url":                row.BaseURL,
		"protocol":                row.Protocol,
		"request_timeout_seconds": row.RequestTimeoutSeconds,
		"enabled":                 row.Enabled,
		"encrypted_api_key":       row.EncryptedAPIKey,
		"api_key_masked":          row.APIKeyMasked,
		"headers_json":            row.HeadersJSON,
		"created_at":              row.CreatedAt,
		"updated_at":              row.UpdatedAt,
	}).Error
}

func createPostgresProviderModelRow(tx *gorm.DB, row PostgresProviderModelEntry) error {
	return tx.Model(&PostgresProviderModelEntry{}).Create(map[string]any{
		"id":           row.ID,
		"provider_id":  row.ProviderID,
		"model_id":     row.ModelID,
		"display_name": row.DisplayName,
		"enabled":      row.Enabled,
		"is_default":   row.IsDefault,
		"is_fallback":  row.IsFallback,
		"created_at":   row.CreatedAt,
		"updated_at":   row.UpdatedAt,
	}).Error
}

func providerModelRowToDefinition(row PostgresProviderModelEntry) ProviderModelRecord {
	return ProviderModelRecord{
		ID:          row.ID,
		ProviderID:  row.ProviderID,
		ModelID:     row.ModelID,
		DisplayName: row.DisplayName,
		Enabled:     row.Enabled,
		IsDefault:   row.IsDefault,
		IsFallback:  row.IsFallback,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}
}

func rowsOwnProtectedModel(rows []PostgresProviderModelEntry) bool {
	for _, item := range rows {
		if item.IsDefault || item.IsFallback {
			return true
		}
	}
	return false
}

func decodeHeaders(payload []byte) (map[string]string, error) {
	if len(payload) == 0 {
		return nil, nil
	}
	var result map[string]string
	if err := json.Unmarshal(payload, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func cloneHeaders(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	result := make(map[string]string, len(input))
	for key, value := range input {
		result[key] = value
	}
	return result
}
