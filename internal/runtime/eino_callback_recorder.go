// eino_callback_recorder.go records safe Eino model/tool callbacks for runtime persistence.
// eino_callback_recorder.go 记录可安全落库的 Eino model/tool callback。
package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/cloudwego/eino/callbacks"
	einomodel "github.com/cloudwego/eino/components/model"
	einotool "github.com/cloudwego/eino/components/tool"
	ucb "github.com/cloudwego/eino/utils/callbacks"
)

const (
	runtimeCallbackComponentModel = "model"
	runtimeCallbackComponentTool  = "tool"
)

type runtimeCallbackStartKey struct{}

// RuntimeCallbackRecorderConfig carries stable model/provider labels from Athena runtime selection.
// RuntimeCallbackRecorderConfig 携带 Athena runtime 选型后的稳定 model/provider 标签。
type RuntimeCallbackRecorderConfig struct {
	ProviderName string
	ModelName    string
}

// RuntimeComponentCallbackEvent is a redacted model/tool callback event ready for later projection.
// RuntimeComponentCallbackEvent 表示可延迟投影的脱敏 model/tool callback 事件。
type RuntimeComponentCallbackEvent struct {
	Component        string
	Node             string
	Provider         string
	ResourceName     string
	Status           string
	DurationMS       int64
	InputCount       int
	InputRuneCount   int
	OutputRuneCount  int
	ToolCallCount    int
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	CachedTokens     int
	ReasoningTokens  int
	ErrorSummary     string
	Metadata         map[string]any
	ObservedAt       time.Time
}

// RuntimeCallbackRecorder collects safe callback events during graph-native turn execution.
// RuntimeCallbackRecorder 在 graph-native 单轮执行期间采集安全 callback 事件。
type RuntimeCallbackRecorder struct {
	mu     sync.Mutex
	config RuntimeCallbackRecorderConfig
	events []RuntimeComponentCallbackEvent
}

// NewRuntimeCallbackRecorder creates a recorder for one prepared runtime execution.
// NewRuntimeCallbackRecorder 为一次 prepared runtime execution 创建 callback recorder。
func NewRuntimeCallbackRecorder(config RuntimeCallbackRecorderConfig) *RuntimeCallbackRecorder {
	return &RuntimeCallbackRecorder{config: config}
}

// Handler returns an Eino callback handler for model/tool callback capture.
// Handler 返回用于采集 model/tool callback 的 Eino callback handler。
func (r *RuntimeCallbackRecorder) Handler() callbacks.Handler {
	if r == nil {
		return callbacks.NewHandlerBuilder().Build()
	}
	return ucb.NewHandlerHelper().
		ChatModel(&ucb.ModelCallbackHandler{
			OnStart: func(ctx context.Context, _ *callbacks.RunInfo, input *einomodel.CallbackInput) context.Context {
				return context.WithValue(ctx, runtimeCallbackStartKey{}, runtimeCallbackStart{
					startedAt: time.Now().UTC(),
					input:     summarizeModelCallbackInput(input),
				})
			},
			OnEnd: func(ctx context.Context, info *callbacks.RunInfo, output *einomodel.CallbackOutput) context.Context {
				start := runtimeCallbackStartFromContext(ctx)
				r.record(modelCallbackEvent(r.config, info, start, output, nil))
				return ctx
			},
			OnError: func(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
				start := runtimeCallbackStartFromContext(ctx)
				r.record(modelCallbackEvent(r.config, info, start, nil, err))
				return ctx
			},
		}).
		Tool(&ucb.ToolCallbackHandler{
			OnStart: func(ctx context.Context, _ *callbacks.RunInfo, input *einotool.CallbackInput) context.Context {
				return context.WithValue(ctx, runtimeCallbackStartKey{}, runtimeCallbackStart{
					startedAt: time.Now().UTC(),
					input:     summarizeToolCallbackInput(input),
				})
			},
			OnEnd: func(ctx context.Context, info *callbacks.RunInfo, output *einotool.CallbackOutput) context.Context {
				start := runtimeCallbackStartFromContext(ctx)
				r.record(toolCallbackEvent(info, start, output, nil))
				return ctx
			},
			OnError: func(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
				start := runtimeCallbackStartFromContext(ctx)
				r.record(toolCallbackEvent(info, start, nil, err))
				return ctx
			},
		}).
		Handler()
}

// Events returns a stable copy of the recorded safe callback events.
// Events 返回已记录安全 callback event 的稳定副本。
func (r *RuntimeCallbackRecorder) Events() []RuntimeComponentCallbackEvent {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]RuntimeComponentCallbackEvent(nil), r.events...)
}

func (r *RuntimeCallbackRecorder) record(event RuntimeComponentCallbackEvent) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
}

type runtimeCallbackStart struct {
	startedAt time.Time
	input     runtimeCallbackInputSummary
}

type runtimeCallbackInputSummary struct {
	inputCount     int
	inputRunes     int
	toolCallCount  int
	argumentKeys   []string
	argumentRunes  int
	redactionLabel string
}

func runtimeCallbackStartFromContext(ctx context.Context) runtimeCallbackStart {
	if ctx == nil {
		return runtimeCallbackStart{}
	}
	start, _ := ctx.Value(runtimeCallbackStartKey{}).(runtimeCallbackStart)
	return start
}

func modelCallbackEvent(config RuntimeCallbackRecorderConfig, info *callbacks.RunInfo, start runtimeCallbackStart, output *einomodel.CallbackOutput, err error) RuntimeComponentCallbackEvent {
	observedAt := time.Now().UTC()
	event := RuntimeComponentCallbackEvent{
		Component:      runtimeCallbackComponentModel,
		Node:           callbackNodeName(info),
		Provider:       defaultString(config.ProviderName, "eino"),
		ResourceName:   defaultString(config.ModelName, callbackNodeName(info)),
		Status:         callbackStatus(err),
		DurationMS:     callbackDurationMS(start.startedAt, observedAt),
		InputCount:     start.input.inputCount,
		InputRuneCount: start.input.inputRunes,
		Metadata: map[string]any{
			"redaction_policy": "whitelist_summary",
			"safe_label":       "model_callback",
		},
		ObservedAt: observedAt,
	}
	if output != nil && output.Message != nil {
		event.OutputRuneCount = utf8.RuneCountInString(output.Message.Content)
		event.ToolCallCount = len(output.Message.ToolCalls)
		if output.Message.ResponseMeta != nil && output.Message.ResponseMeta.Usage != nil {
			event.PromptTokens = output.Message.ResponseMeta.Usage.PromptTokens
			event.CompletionTokens = output.Message.ResponseMeta.Usage.CompletionTokens
			event.TotalTokens = output.Message.ResponseMeta.Usage.TotalTokens
			event.CachedTokens = output.Message.ResponseMeta.Usage.PromptTokenDetails.CachedTokens
			event.ReasoningTokens = output.Message.ResponseMeta.Usage.CompletionTokensDetails.ReasoningTokens
		}
	}
	if output != nil && output.TokenUsage != nil {
		event.PromptTokens = output.TokenUsage.PromptTokens
		event.CompletionTokens = output.TokenUsage.CompletionTokens
		event.TotalTokens = output.TokenUsage.TotalTokens
		event.CachedTokens = output.TokenUsage.PromptTokenDetails.CachedTokens
		event.ReasoningTokens = output.TokenUsage.CompletionTokensDetails.ReasoningTokens
	}
	if err != nil {
		event.ErrorSummary = safeTerminalErrorSummary(err)
	}
	return event
}

func toolCallbackEvent(info *callbacks.RunInfo, start runtimeCallbackStart, output *einotool.CallbackOutput, err error) RuntimeComponentCallbackEvent {
	observedAt := time.Now().UTC()
	event := RuntimeComponentCallbackEvent{
		Component:      runtimeCallbackComponentTool,
		Node:           callbackNodeName(info),
		Provider:       "eino",
		ResourceName:   callbackNodeName(info),
		Status:         callbackStatus(err),
		DurationMS:     callbackDurationMS(start.startedAt, observedAt),
		InputRuneCount: start.input.argumentRunes,
		Metadata: map[string]any{
			"redaction_policy": "whitelist_summary",
			"safe_label":       "tool_callback",
			"argument_keys":    start.input.argumentKeys,
		},
		ObservedAt: observedAt,
	}
	if output != nil {
		event.OutputRuneCount = utf8.RuneCountInString(output.Response)
		if output.ToolOutput != nil {
			event.Metadata["structured_tool_output"] = true
		}
	}
	if err != nil {
		event.ErrorSummary = safeTerminalErrorSummary(err)
	}
	return event
}

func summarizeModelCallbackInput(input *einomodel.CallbackInput) runtimeCallbackInputSummary {
	if input == nil {
		return runtimeCallbackInputSummary{redactionLabel: "model_input_absent"}
	}
	summary := runtimeCallbackInputSummary{
		inputCount:     len(input.Messages),
		redactionLabel: "message_content_omitted",
	}
	for _, message := range input.Messages {
		if message == nil {
			continue
		}
		summary.inputRunes += utf8.RuneCountInString(message.Content)
		summary.toolCallCount += len(message.ToolCalls)
	}
	return summary
}

func summarizeToolCallbackInput(input *einotool.CallbackInput) runtimeCallbackInputSummary {
	if input == nil {
		return runtimeCallbackInputSummary{redactionLabel: "tool_input_absent"}
	}
	return runtimeCallbackInputSummary{
		argumentKeys:   safeJSONArgumentKeys(input.ArgumentsInJSON),
		argumentRunes:  utf8.RuneCountInString(input.ArgumentsInJSON),
		redactionLabel: "tool_arguments_omitted",
	}
}

func safeJSONArgumentKeys(arguments string) []string {
	var parsed map[string]any
	if err := json.Unmarshal([]byte(arguments), &parsed); err != nil {
		return nil
	}
	keys := make([]string, 0, len(parsed))
	for key := range parsed {
		key = strings.TrimSpace(key)
		if key != "" {
			keys = append(keys, key)
		}
	}
	return keys
}

func callbackStatus(err error) string {
	if err != nil {
		return "error"
	}
	return "success"
}

func callbackDurationMS(startedAt time.Time, observedAt time.Time) int64 {
	if startedAt.IsZero() {
		return 0
	}
	duration := observedAt.Sub(startedAt).Milliseconds()
	if duration < 0 {
		return 0
	}
	return duration
}

func callbackNodeName(info *callbacks.RunInfo) string {
	if info == nil {
		return "unknown"
	}
	return defaultString(strings.TrimSpace(info.Name), fmt.Sprint(info.Component))
}
