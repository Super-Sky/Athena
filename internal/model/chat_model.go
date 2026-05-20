// chat_model.go adapts provider-backed chat models into the runtime execution layer.
// chat_model.go 负责把供应商驱动的 chat model 适配到 runtime 执行层。
package model

import (
	"context"
	"fmt"
	modelparams "moss/internal/model/parameters"
	"net/http"
	"strings"

	"github.com/cloudwego/eino-ext/components/model/ark"
	"github.com/cloudwego/eino-ext/components/model/claude"
	"github.com/cloudwego/eino-ext/components/model/openai"
	einomodel "github.com/cloudwego/eino/components/model"
	arkModel "github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
)

// NewChatModelWithContext creates one protocol-specific chat model from a resolved runtime provider config.
// NewChatModelWithContext 会根据解析后的运行时供应商配置创建一个协议对应的 chat model。
func NewChatModelWithContext(ctx context.Context, cfg ChatConfig) (einomodel.ToolCallingChatModel, error) {
	protocol := strings.ToLower(strings.TrimSpace(cfg.ProviderProtocol))
	resolved := cfg.ResolvedParameters

	switch protocol {
	case ProtocolArk:
		arkConfig := &ark.ChatModelConfig{
			APIKey:       cfg.APIKey,
			Model:        cfg.ProviderModelID,
			BaseURL:      cfg.BaseURL,
			CustomHeader: cloneHeaders(cfg.Headers),
			Thinking: &arkModel.Thinking{
				Type: arkModel.ThinkingTypeDisabled,
			},
		}
		if cfg.RequestTimeout > 0 {
			timeout := cfg.RequestTimeout
			arkConfig.Timeout = &timeout
		}
		applyResolvedParametersToArkConfig(arkConfig, resolved)
		cm, err := ark.NewChatModel(ctx, &ark.ChatModelConfig{
			APIKey:          arkConfig.APIKey,
			Timeout:         arkConfig.Timeout,
			BaseURL:         arkConfig.BaseURL,
			Model:           arkConfig.Model,
			MaxTokens:       arkConfig.MaxTokens,
			Temperature:     arkConfig.Temperature,
			TopP:            arkConfig.TopP,
			Stop:            arkConfig.Stop,
			ResponseFormat:  arkConfig.ResponseFormat,
			ReasoningEffort: arkConfig.ReasoningEffort,
			CustomHeader:    arkConfig.CustomHeader,
			Thinking:        arkConfig.Thinking,
		})
		if err != nil {
			return nil, fmt.Errorf("ark.NewChatModel failed: %w", err)
		}
		return cm, nil
	case ProtocolOpenAICompatible:
		baseURL := cfg.BaseURL
		extraFields := map[string]any{}
		if strings.Contains(strings.ToLower(baseURL), "api.minimaxi.com") {
			extraFields["reasoning_split"] = true
		}

		httpClient := &http.Client{
			Timeout: cfg.RequestTimeout,
			Transport: roundTripperWithHeaders{
				base:    http.DefaultTransport,
				headers: cloneHeaders(cfg.Headers),
			},
		}
		openAIConfig := &openai.ChatModelConfig{
			APIKey:      cfg.APIKey,
			Model:       cfg.ProviderModelID,
			BaseURL:     baseURL,
			ExtraFields: extraFields,
			ByAzure:     false,
			Timeout:     cfg.RequestTimeout,
			HTTPClient:  httpClient,
		}
		applyResolvedParametersToOpenAIConfig(openAIConfig, resolved)
		cm, err := openai.NewChatModel(ctx, openAIConfig)
		if err != nil {
			return nil, fmt.Errorf("openai.NewChatModel failed: %w", err)
		}
		return cm, nil
	case ProtocolAnthropic:
		httpClient := &http.Client{
			Timeout: cfg.RequestTimeout,
		}
		var baseURL *string
		if trimmed := strings.TrimSpace(cfg.BaseURL); trimmed != "" {
			baseURL = &trimmed
		}
		claudeConfig := &claude.Config{
			APIKey:                 cfg.APIKey,
			Model:                  cfg.ProviderModelID,
			BaseURL:                baseURL,
			MaxTokens:              256,
			HTTPClient:             httpClient,
			AdditionalHeaderFields: cloneHeaders(cfg.Headers),
		}
		applyResolvedParametersToClaudeConfig(claudeConfig, resolved)
		cm, err := claude.NewChatModel(ctx, claudeConfig)
		if err != nil {
			return nil, fmt.Errorf("claude.NewChatModel failed: %w", err)
		}
		return cm, nil
	default:
		return nil, fmt.Errorf("provider protocol %q is not supported by the current runtime adapter", cfg.ProviderProtocol)
	}
}

type roundTripperWithHeaders struct {
	base    http.RoundTripper
	headers map[string]string
}

// RoundTrip injects provider headers into outbound HTTP requests before delegating transport.
// RoundTrip 会在委托底层 transport 前把供应商请求头注入到出站 HTTP 请求中。
func (r roundTripperWithHeaders) RoundTrip(req *http.Request) (*http.Response, error) {
	cloned := req.Clone(req.Context())
	for key, value := range r.headers {
		cloned.Header.Set(key, value)
	}
	return r.base.RoundTrip(cloned)
}

func applyResolvedParametersToOpenAIConfig(cfg *openai.ChatModelConfig, resolved *modelparams.ResolvedModelParameters) {
	if cfg == nil || resolved == nil {
		return
	}
	cfg.Temperature = float32PtrFromFloat64(resolved.Temperature)
	cfg.TopP = float32PtrFromFloat64(resolved.TopP)
	cfg.MaxCompletionTokens = intPtrFromInt(resolved.MaxOutputTokens)
	cfg.Stop = append([]string(nil), resolved.Stop...)
	cfg.Seed = intPtrFromInt64(resolved.Seed)
	cfg.ReasoningEffort = openai.ReasoningEffortLevel(resolved.ReasoningEffort)
}

func applyResolvedParametersToClaudeConfig(cfg *claude.Config, resolved *modelparams.ResolvedModelParameters) {
	if cfg == nil || resolved == nil {
		return
	}
	if resolved.MaxOutputTokens > 0 {
		cfg.MaxTokens = resolved.MaxOutputTokens
	}
	cfg.Temperature = float32PtrFromFloat64(resolved.Temperature)
	cfg.TopP = float32PtrFromFloat64(resolved.TopP)
	cfg.StopSequences = append([]string(nil), resolved.Stop...)
	if resolved.ToolChoice.Kind == modelparams.ToolChoiceNone {
		disable := true
		cfg.DisableParallelToolUse = &disable
	}
}

func applyResolvedParametersToArkConfig(cfg *ark.ChatModelConfig, resolved *modelparams.ResolvedModelParameters) {
	if cfg == nil || resolved == nil {
		return
	}
	cfg.MaxTokens = intPtrFromInt(resolved.MaxOutputTokens)
	cfg.Temperature = float32PtrFromFloat64(resolved.Temperature)
	cfg.TopP = float32PtrFromFloat64(resolved.TopP)
	cfg.Stop = append([]string(nil), resolved.Stop...)
	switch resolved.ResponseFormat {
	case modelparams.ResponseFormatJSONSchema:
		cfg.ResponseFormat = &ark.ResponseFormat{
			Type: arkModel.ResponseFormatJSONSchema,
		}
	}
	cfg.ReasoningEffort = arkReasoningEffortPtr(resolved.ReasoningEffort)
}

func float32PtrFromFloat64(value float64) *float32 {
	typed := float32(value)
	return &typed
}

func intPtrFromInt(value int) *int {
	if value <= 0 {
		return nil
	}
	typed := value
	return &typed
}

func intPtrFromInt64(value int64) *int {
	if value <= 0 {
		return nil
	}
	typed := int(value)
	return &typed
}

func arkReasoningEffortPtr(value modelparams.ReasoningEffort) *arkModel.ReasoningEffort {
	var mapped arkModel.ReasoningEffort
	switch value {
	case modelparams.ReasoningEffortHigh:
		mapped = arkModel.ReasoningEffortHigh
	case modelparams.ReasoningEffortLow:
		mapped = arkModel.ReasoningEffortMinimal
	default:
		mapped = arkModel.ReasoningEffortMedium
	}
	return &mapped
}
