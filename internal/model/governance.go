// governance.go defines provider and provider-model governance data used by the model store.
// governance.go 定义模型存储使用的 provider 与 provider-model 治理数据结构。
package model

import (
	"context"
	"errors"
	"fmt"
	modelparams "moss/internal/model/parameters"
	"strings"
	"time"
)

const (
	// StoreDriverMemory keeps model governance state in-process for tests and lightweight development.
	// StoreDriverMemory 表示模型治理数据只保存在进程内，适合测试和轻量开发。
	StoreDriverMemory = "memory"

	// StoreDriverPostgres keeps model governance state in PostgreSQL.
	// StoreDriverPostgres 表示模型治理数据保存在 PostgreSQL 中。
	StoreDriverPostgres = "postgres"
)

const (
	// ProtocolOpenAICompatible uses the OpenAI-compatible chat completion contract.
	// ProtocolOpenAICompatible 表示采用 OpenAI 兼容聊天接口协议。
	ProtocolOpenAICompatible = "openai_compatible"

	// ProtocolArk uses Volcengine Ark's chat completion contract.
	// ProtocolArk 表示采用火山引擎 Ark 聊天协议。
	ProtocolArk = "ark"

	// ProtocolAnthropic reserves a provider protocol slot for future Anthropic-compatible support.
	// ProtocolAnthropic 为后续 Anthropic 协议支持预留一个供应商协议位。
	ProtocolAnthropic = "anthropic"
)

var (
	// ErrModelProviderNotFound reports one missing provider resource in governance CRUD flows.
	// ErrModelProviderNotFound 表示治理 CRUD 流程中引用的供应商资源不存在。
	ErrModelProviderNotFound = errors.New("model provider was not found")

	// ErrProviderModelNotFound reports one missing child-model resource in governance CRUD flows.
	// ErrProviderModelNotFound 表示治理 CRUD 流程中引用的子模型资源不存在。
	ErrProviderModelNotFound = errors.New("provider model was not found")
)

// ProviderDefinition is the transport-safe governance view of one configured model provider.
// ProviderDefinition 是一个已配置模型供应商的安全治理视图，不包含明文密钥。
type ProviderDefinition struct {
	ID                    string                `json:"id"`
	Name                  string                `json:"name"`
	BaseURL               string                `json:"base_url,omitempty"`
	Protocol              string                `json:"protocol"`
	RequestTimeoutSeconds int                   `json:"request_timeout_seconds,omitempty"`
	Enabled               bool                  `json:"enabled"`
	APIKeyConfigured      bool                  `json:"api_key_configured"`
	APIKeyMasked          string                `json:"api_key_masked,omitempty"`
	Headers               map[string]string     `json:"headers,omitempty"`
	Models                []ProviderModelRecord `json:"models,omitempty"`
	CreatedAt             time.Time             `json:"created_at,omitempty"`
	UpdatedAt             time.Time             `json:"updated_at,omitempty"`
}

// ProviderModelRecord is the transport-safe governance view of one model entry under a provider.
// ProviderModelRecord 是一个供应商下模型条目的安全治理视图。
type ProviderModelRecord struct {
	ID          string    `json:"id"`
	ProviderID  string    `json:"provider_id"`
	ModelID     string    `json:"model_id"`
	DisplayName string    `json:"display_name"`
	Enabled     bool      `json:"enabled"`
	IsDefault   bool      `json:"is_default"`
	IsFallback  bool      `json:"is_fallback"`
	CreatedAt   time.Time `json:"created_at,omitempty"`
	UpdatedAt   time.Time `json:"updated_at,omitempty"`
}

// ProviderUpsertInput is the write contract for creating or replacing a provider definition.
// ProviderUpsertInput 是创建或整体替换供应商定义时使用的写入契约。
type ProviderUpsertInput struct {
	Name                  string
	BaseURL               string
	Protocol              string
	RequestTimeoutSeconds int
	APIKey                string
	Headers               map[string]string
	Enabled               bool
	Models                []ProviderModelUpsertInput
}

// ProviderPatchInput captures governance toggles that do not replace the whole provider definition.
// ProviderPatchInput 描述不会整体替换供应商定义的治理开关。
type ProviderPatchInput struct {
	Enabled *bool
}

// ProviderModelUpsertInput is the write contract for creating or replacing one provider model entry.
// ProviderModelUpsertInput 是创建或整体替换供应商下模型条目时使用的写入契约。
type ProviderModelUpsertInput struct {
	ModelID     string
	DisplayName string
	Enabled     bool
	IsDefault   bool
	IsFallback  bool
}

func normalizeProviderModelRecords(providerID string, inputs []ProviderModelUpsertInput, createdAt time.Time, updatedAt time.Time) ([]ProviderModelRecord, error) {
	if len(inputs) == 0 {
		return nil, nil
	}

	seenModelIDs := make(map[string]struct{}, len(inputs))
	seenDisplayNames := make(map[string]struct{}, len(inputs))
	defaultCount := 0
	fallbackCount := 0
	records := make([]ProviderModelRecord, 0, len(inputs))

	for _, input := range inputs {
		record, err := normalizeProviderModelRecord("", providerID, input, createdAt, updatedAt)
		if err != nil {
			return nil, err
		}
		modelKey := strings.ToLower(record.ModelID)
		if _, exists := seenModelIDs[modelKey]; exists {
			return nil, fmt.Errorf("provider model_id %q is duplicated in request", record.ModelID)
		}
		seenModelIDs[modelKey] = struct{}{}

		displayKey := strings.ToLower(record.DisplayName)
		if _, exists := seenDisplayNames[displayKey]; exists {
			return nil, fmt.Errorf("provider model display_name %q is duplicated in request", record.DisplayName)
		}
		seenDisplayNames[displayKey] = struct{}{}

		if record.IsDefault {
			defaultCount++
		}
		if record.IsFallback {
			fallbackCount++
		}
		records = append(records, record)
	}

	if defaultCount > 1 {
		return nil, fmt.Errorf("only one default model may be declared in a single provider request")
	}
	if fallbackCount > 1 {
		return nil, fmt.Errorf("only one fallback model may be declared in a single provider request")
	}
	return records, nil
}

// ProviderModelPatchInput captures model entry governance toggles without replacing identifiers.
// ProviderModelPatchInput 描述不会替换模型标识的条目治理开关。
type ProviderModelPatchInput struct {
	Enabled    *bool
	IsDefault  *bool
	IsFallback *bool
}

// ChatConfig contains the decrypted runtime provider and model configuration used by executors.
// ChatConfig 包含执行器真正使用的已解密供应商和模型运行时配置。
type ChatConfig struct {
	ProviderID         string
	ProviderName       string
	ProviderProtocol   string
	BaseURL            string
	RequestTimeout     time.Duration
	APIKey             string
	Headers            map[string]string
	ModelRecordID      string
	ProviderModelID    string
	ModelDisplayName   string
	ResolvedParameters *modelparams.ResolvedModelParameters
}

// Selection describes the resolved primary model plus the optional technical fallback candidate.
// Selection 描述本次请求解析出的主模型以及可选的技术失败 fallback 候选模型。
type Selection struct {
	Primary      ChatConfig
	Fallback     *ChatConfig
	Explicit     bool
	PrimaryMeta  ProviderModelRecord
	FallbackMeta *ProviderModelRecord
	ProviderMeta ProviderDefinition
}

// ResolveError reports that one request-level model selection cannot continue.
// ResolveError 表示某次请求级模型选择无法继续。
type ResolveError struct {
	Code    string
	Reason  string
	Message string
	Detail  map[string]any
}

// Error returns one human-readable summary for a model resolution failure.
// Error 返回一次模型解析失败的人类可读摘要。
func (e *ResolveError) Error() string {
	if e == nil {
		return "model resolution failed"
	}
	if e.Message != "" {
		return e.Message
	}
	return "model resolution failed"
}

// Store owns durable provider/model governance data plus request-level resolution.
// Store 持有供应商与模型的持久化治理数据，并负责请求级模型解析。
type Store interface {
	ListProviders(ctx context.Context) ([]ProviderDefinition, error)
	CreateProvider(ctx context.Context, input ProviderUpsertInput) (ProviderDefinition, error)
	UpdateProvider(ctx context.Context, id string, input ProviderUpsertInput) (ProviderDefinition, error)
	PatchProvider(ctx context.Context, id string, input ProviderPatchInput) (ProviderDefinition, error)
	DeleteProvider(ctx context.Context, id string) error
	CreateProviderModel(ctx context.Context, providerID string, input ProviderModelUpsertInput) (ProviderModelRecord, error)
	UpdateProviderModel(ctx context.Context, providerID string, modelRecordID string, input ProviderModelUpsertInput) (ProviderModelRecord, error)
	PatchProviderModel(ctx context.Context, providerID string, modelRecordID string, input ProviderModelPatchInput) (ProviderModelRecord, error)
	DeleteProviderModel(ctx context.Context, providerID string, modelRecordID string) error
	Resolve(ctx context.Context, requestedModelRecordID string) (*Selection, error)
	AutoMigrate(ctx context.Context) error
}
