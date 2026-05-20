// http.go is the primary HTTP transport hub for Athena and registers most route handlers and request contracts.
// http.go 负责 Athena 的主要 HTTP transport 枢纽，并注册大部分路由处理器和请求契约。
//
// It exists because the current repository keeps transport mapping, request validation, and protocol-level
// response shaping in one place before handing business orchestration to the app layer.
// 这个文件存在的原因是当前仓库把 transport 映射、请求校验和协议级响应整形集中在一处，
// 然后再把业务编排下放给 app 层。
//
// Main entry points:
// 主要入口：
// - `NewHTTPServer`
// - session resource handlers
// - chat stream/respond handlers
// - model / skill governance handlers
//
// Change carefully when:
// 修改时重点注意：
// - API field changes usually require synchronized updates to OpenAPI, tests, and docs/api.md
// - this file is large, so new handlers should be split out when a stable sub-domain emerges
// - waiting/SSE behavior changes can affect both transport semantics and client compatibility
package server

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	hertzapp "github.com/cloudwego/hertz/pkg/app"
	hserver "github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/hertz-contrib/sse"
	appcore "moss/internal/app"
	"moss/internal/config"
	"moss/internal/contextassets"
	"moss/internal/customization"
	platformautomation "moss/internal/extensions/platform/automation"
	platformcontext "moss/internal/extensions/platform/context"
	"moss/internal/model"
	"moss/internal/observability"
	"moss/internal/runtime"
	intentpkg "moss/internal/runtime/intent"
	"moss/internal/session"
	"moss/internal/skills"
)

// ChatStreamRequest is the JSON contract for the single streaming chat entrypoint.
// ChatStreamRequest 是单一流式聊天入口使用的 JSON 请求契约。
type ChatStreamRequest struct {
	TaskType               string                     `json:"task_type,omitempty"`
	TaskSubtype            string                     `json:"task_subtype,omitempty"`
	Scene                  string                     `json:"scene,omitempty"`
	Query                  string                     `json:"query"`
	SessionID              string                     `json:"session_id,omitempty"`
	MainSessionID          string                     `json:"main_session_id,omitempty"`
	WorkspaceID            string                     `json:"workspace_id,omitempty"`
	AppInstanceID          string                     `json:"app_instance_id,omitempty"`
	AppSessionID           string                     `json:"app_session_id,omitempty"`
	IntegrationInstanceID  string                     `json:"integration_instance_id,omitempty"`
	WorkflowRunID          string                     `json:"workflow_run_id,omitempty"`
	StepID                 string                     `json:"step_id,omitempty"`
	TriggerType            string                     `json:"trigger_type,omitempty"`
	AutomationTaskID       string                     `json:"automation_task_id,omitempty"`
	UserLanguage           string                     `json:"user_language,omitempty"`
	DesiredOutputMode      string                     `json:"desired_output_mode,omitempty"`
	GlobalContext          map[string]any             `json:"global_context,omitempty"`
	AppContext             map[string]any             `json:"app_context,omitempty"`
	InputPayload           map[string]any             `json:"input_payload,omitempty"`
	ModelID                string                     `json:"model_id,omitempty"`
	ModelRecordID          string                     `json:"model_record_id,omitempty"`
	EnabledSkills          []string                   `json:"enabled_skills,omitempty"`
	EnabledTools           []string                   `json:"enabled_tools,omitempty"`
	PromptTemplate         string                     `json:"prompt_template,omitempty"`
	ContextAssetOverrides  []contextassets.Asset      `json:"context_asset_overrides,omitempty"`
	DisabledAssetTypes     []string                   `json:"disabled_asset_types,omitempty"`
	AssetPriorityOverrides map[string]int             `json:"asset_priority_overrides,omitempty"`
	Supplement             *runtime.SupplementPayload `json:"supplement,omitempty"`
	SupplementOutcome      runtime.SupplementOutcome  `json:"supplement_outcome,omitempty"`
	TimeoutAfterSeconds    int                        `json:"timeout_after_seconds,omitempty"`
	ResumeToken            string                     `json:"resume_token,omitempty"`
	DisableFastPath        bool                       `json:"disable_fast_path,omitempty"`
}

// ChatRespondRequest is the JSON contract for the structured-result chat entrypoint.
// ChatRespondRequest 是结构化结果聊天入口使用的 JSON 请求契约。
type ChatRespondRequest struct {
	TaskType               string                     `json:"task_type,omitempty"`
	TaskSubtype            string                     `json:"task_subtype,omitempty"`
	Scene                  string                     `json:"scene,omitempty"`
	Query                  string                     `json:"query"`
	SessionID              string                     `json:"session_id,omitempty"`
	MainSessionID          string                     `json:"main_session_id,omitempty"`
	WorkspaceID            string                     `json:"workspace_id,omitempty"`
	AppInstanceID          string                     `json:"app_instance_id,omitempty"`
	AppSessionID           string                     `json:"app_session_id,omitempty"`
	IntegrationInstanceID  string                     `json:"integration_instance_id,omitempty"`
	WorkflowRunID          string                     `json:"workflow_run_id,omitempty"`
	StepID                 string                     `json:"step_id,omitempty"`
	TriggerType            string                     `json:"trigger_type,omitempty"`
	AutomationTaskID       string                     `json:"automation_task_id,omitempty"`
	UserLanguage           string                     `json:"user_language,omitempty"`
	DesiredOutputMode      string                     `json:"desired_output_mode,omitempty"`
	GlobalContext          map[string]any             `json:"global_context,omitempty"`
	AppContext             map[string]any             `json:"app_context,omitempty"`
	InputPayload           map[string]any             `json:"input_payload,omitempty"`
	ModelID                string                     `json:"model_id,omitempty"`
	ModelRecordID          string                     `json:"model_record_id,omitempty"`
	EnabledSkills          []string                   `json:"enabled_skills,omitempty"`
	EnabledTools           []string                   `json:"enabled_tools,omitempty"`
	PromptTemplate         string                     `json:"prompt_template,omitempty"`
	ContextAssetOverrides  []contextassets.Asset      `json:"context_asset_overrides,omitempty"`
	DisabledAssetTypes     []string                   `json:"disabled_asset_types,omitempty"`
	AssetPriorityOverrides map[string]int             `json:"asset_priority_overrides,omitempty"`
	Supplement             *runtime.SupplementPayload `json:"supplement,omitempty"`
	SupplementOutcome      runtime.SupplementOutcome  `json:"supplement_outcome,omitempty"`
	TimeoutAfterSeconds    int                        `json:"timeout_after_seconds,omitempty"`
	ResumeToken            string                     `json:"resume_token,omitempty"`
	DisableFastPath        bool                       `json:"disable_fast_path,omitempty"`
	StrictSchemaValidation bool                       `json:"strict_schema_validation,omitempty"`
	SchemaRetryCount       int                        `json:"schema_retry_count,omitempty"`
	SchemaRepairMode       string                     `json:"schema_repair_mode,omitempty"`
	SchemaFailureAction    string                     `json:"schema_failure_action,omitempty"`
}

// HeaderKVPair keeps one explicit key/value header pair for flexible transport parsing.
// HeaderKVPair 用于描述一个显式的请求头键值对，方便兼容列表式输入。
type HeaderKVPair struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// ProviderHeadersInput accepts either one JSON object or one key/value list for provider headers.
// ProviderHeadersInput 支持把供应商请求头写成 JSON 对象或 key/value 列表两种形式。
type ProviderHeadersInput struct {
	Values map[string]string
}

// UnmarshalJSON keeps provider headers machine-readable without forcing a single client payload shape.
// UnmarshalJSON 会在不强制客户端固定格式的前提下解析机器可读的供应商请求头。
func (i *ProviderHeadersInput) UnmarshalJSON(payload []byte) error {
	trimmed := bytes.TrimSpace(payload)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		i.Values = nil
		return nil
	}

	var object map[string]string
	if err := json.Unmarshal(trimmed, &object); err == nil {
		i.Values = cloneHeaders(object)
		return nil
	}

	var items []HeaderKVPair
	if err := json.Unmarshal(trimmed, &items); err != nil {
		return fmt.Errorf("headers must be an object or an array of key/value pairs")
	}
	result := make(map[string]string, len(items))
	for _, item := range items {
		key := strings.TrimSpace(item.Key)
		if key == "" {
			return fmt.Errorf("header key is required")
		}
		result[key] = item.Value
	}
	i.Values = result
	return nil
}

// Clone returns one isolated header map for downstream transport or store writes.
// Clone 会返回一份隔离的请求头 map，供下游 transport 或 store 写入使用。
func (i ProviderHeadersInput) Clone() map[string]string {
	return cloneHeaders(i.Values)
}

// UpdateSkillPackageStateRequest controls the enabled state of one uploaded skill package.
// UpdateSkillPackageStateRequest 用于控制一份 uploaded skill package 的启用状态。
type UpdateSkillPackageStateRequest struct {
	Enabled bool `json:"enabled"`
}

type SkillPackageFilesRequest struct {
	Name    string            `json:"name"`
	Enabled *bool             `json:"enabled,omitempty"`
	Files   map[string]string `json:"files"`
}

// RollbackSkillPackageRequest selects one historical revision to restore.
// RollbackSkillPackageRequest 用于选择一个需要恢复的历史版本。
type RollbackSkillPackageRequest struct {
	Revision int `json:"revision"`
}

// CreateSessionRequest captures the minimal v1 session creation payload.
// CreateSessionRequest 描述 v1 session 创建接口允许的最小请求体。
type CreateSessionRequest struct {
	Title string `json:"title,omitempty"`
}

// PatchSessionRequest captures the narrow v1 editable session metadata.
// PatchSessionRequest 描述 v1 允许编辑的最小 session 元数据。
type PatchSessionRequest struct {
	Title string `json:"title"`
}

// CreateModelProviderRequest captures provider creation input, including optional child models.
// CreateModelProviderRequest 描述供应商创建输入，并允许一次性内嵌子模型。
type CreateModelProviderRequest struct {
	Name                  string                       `json:"name"`
	BaseURL               string                       `json:"base_url,omitempty"`
	Protocol              string                       `json:"protocol"`
	APIKey                string                       `json:"api_key,omitempty"`
	RequestTimeoutSeconds int                          `json:"request_timeout_seconds,omitempty"`
	Headers               ProviderHeadersInput         `json:"headers,omitempty"`
	Enabled               *bool                        `json:"enabled,omitempty"`
	Models                []UpsertProviderModelRequest `json:"models,omitempty"`
}

// UpsertModelProviderRequest captures provider-level CRUD input without exposing stored secrets in responses.
// UpsertModelProviderRequest 描述供应商级别 CRUD 输入，并避免在响应中暴露已存储密钥。
type UpsertModelProviderRequest struct {
	Name                  string               `json:"name"`
	BaseURL               string               `json:"base_url,omitempty"`
	Protocol              string               `json:"protocol"`
	APIKey                string               `json:"api_key,omitempty"`
	RequestTimeoutSeconds int                  `json:"request_timeout_seconds,omitempty"`
	Headers               ProviderHeadersInput `json:"headers,omitempty"`
	Enabled               *bool                `json:"enabled,omitempty"`
}

// PatchModelProviderRequest captures provider-level toggle fields.
// PatchModelProviderRequest 描述供应商级别的局部开关字段。
type PatchModelProviderRequest struct {
	Enabled *bool `json:"enabled,omitempty"`
}

// UpsertProviderModelRequest captures one provider-model CRUD input.
// UpsertProviderModelRequest 描述单条供应商模型的 CRUD 输入。
type UpsertProviderModelRequest struct {
	ModelID     string `json:"model_id"`
	DisplayName string `json:"display_name"`
	Enabled     *bool  `json:"enabled,omitempty"`
	IsDefault   bool   `json:"is_default"`
	IsFallback  bool   `json:"is_fallback"`
}

// PatchProviderModelRequest captures provider-model toggle fields.
// PatchProviderModelRequest 描述供应商模型的局部开关字段。
type PatchProviderModelRequest struct {
	Enabled    *bool `json:"enabled,omitempty"`
	IsDefault  *bool `json:"is_default,omitempty"`
	IsFallback *bool `json:"is_fallback,omitempty"`
}

// StreamEvent is the normalized SSE payload emitted by the server layer.
// StreamEvent 是 server 层统一发出的 SSE 事件结构。
type StreamEvent struct {
	Type             string                            `json:"type"`
	RequestID        string                            `json:"request_id,omitempty"`
	SessionID        string                            `json:"session_id,omitempty"`
	AgentName        string                            `json:"agent_name,omitempty"`
	RunPath          string                            `json:"run_path,omitempty"`
	Content          string                            `json:"content,omitempty"`
	ToolCalls        []schema.ToolCall                 `json:"tool_calls,omitempty"`
	ToolCallID       string                            `json:"tool_call_id,omitempty"`
	ToolName         string                            `json:"tool_name,omitempty"`
	ToolArguments    string                            `json:"tool_arguments,omitempty"`
	DurationMS       int64                             `json:"duration_ms,omitempty"`
	Status           string                            `json:"status,omitempty"`
	StartedAt        string                            `json:"started_at,omitempty"`
	ActionType       string                            `json:"action_type,omitempty"`
	Error            string                            `json:"error,omitempty"`
	Action           *runtime.Action                   `json:"action,omitempty"`
	Notification     *runtime.Notification             `json:"notification,omitempty"`
	ErrorDetail      *runtime.ProtocolError            `json:"error_detail,omitempty"`
	WaitState        *runtime.WaitState                `json:"wait_state,omitempty"`
	StructuredOutput *runtime.StructuredOutputContract `json:"structured_output,omitempty"`
	ProgressStep     *ProgressStep                     `json:"progress_step,omitempty"`
	Detail           map[string]any                    `json:"detail,omitempty"`
}

// ProgressStep defines one stable, user-visible progress step emitted over SSE.
// ProgressStep 定义一条通过 SSE 发出的、对用户可见的稳定过程步骤。
type ProgressStep struct {
	StepID        string `json:"step_id"`
	Category      string `json:"category,omitempty"`
	Label         string `json:"label,omitempty"`
	Summary       string `json:"summary,omitempty"`
	DisplayToUser bool   `json:"display_to_user"`
}

// StepFlowSummary describes the user-visible step-flow contract once one request reaches a terminal SSE `done` event.
// StepFlowSummary 描述一次请求在到达终态 SSE `done` 事件后，对用户可见步骤流的稳定总结契约。
type StepFlowSummary struct {
	VisibleEventTypes  []string `json:"visible_event_types,omitempty"`
	InternalEventTypes []string `json:"internal_event_types,omitempty"`
	TerminalEvent      string   `json:"terminal_event,omitempty"`
	FinalStatus        string   `json:"final_status,omitempty"`
	AutoFold           bool     `json:"auto_fold"`
	CurrentStage       string   `json:"current_stage,omitempty"`
	CompletedStages    []string `json:"completed_stages,omitempty"`
	Summary            string   `json:"summary,omitempty"`
}

// stepFlowStepSnapshot stores one emitted user-visible step together with its terminal progress status.
// stepFlowStepSnapshot 保存一条已发出的用户可见步骤及其对应的进度状态，供终态步骤流摘要复用。
type stepFlowStepSnapshot struct {
	StepID string
	Status string
}

const (
	progressStepType        = "progress_step"
	progressStatusStarted   = "started"
	progressStatusRunning   = "running"
	progressStatusCompleted = "completed"
	progressStatusFailed    = "failed"
	progressStatusWaiting   = "waiting"
)

const (
	progressCategoryContext  = "context"
	progressCategoryAnalysis = "analysis"
	progressCategoryPlanning = "planning"
	progressCategoryTool     = "tool"
	progressCategoryResponse = "response"
	progressCategoryWaiting  = "waiting"
	progressCategoryError    = "error"
)

// HTTPServer owns the Hertz engine used by the transport layer.
// HTTPServer 持有 transport 层使用的 Hertz 引擎。
type HTTPServer struct {
	engine *hserver.Hertz
}

// NewHTTPServer registers the health check and the single POST SSE endpoint.
// NewHTTPServer 负责注册健康检查和唯一的 POST SSE 聊天入口。
func NewHTTPServer(cfg config.Config, application *appcore.Service) *HTTPServer {
	h := hserver.Default(hserver.WithHostPorts(":" + strconv.Itoa(cfg.Server.HTTPPort)))
	h.GET("/swagger", handleSwaggerUI(cfg))
	h.GET("/swagger/assets/swagger-ui.css", handleSwaggerCSS())
	h.GET("/swagger/assets/swagger-ui-bundle.js", handleSwaggerBundleJS())
	h.GET("/swagger/openapi.json", handleOpenAPISpec(cfg))
	h.OPTIONS("/swagger/openapi.json", func(ctx context.Context, c *hertzapp.RequestContext) {
		handleControlPlaneOptions(ctx, c, cfg)
	})
	h.GET("/healthz", func(_ context.Context, c *hertzapp.RequestContext) {
		c.JSON(consts.StatusOK, map[string]string{
			"status": "ok",
		})
	})
	h.POST("/api/sessions", func(ctx context.Context, c *hertzapp.RequestContext) {
		handleCreateSession(ctx, c, application)
	})
	h.GET("/api/sessions", func(ctx context.Context, c *hertzapp.RequestContext) {
		handleListSessions(ctx, c, application)
	})
	h.GET("/api/sessions/:id", func(ctx context.Context, c *hertzapp.RequestContext) {
		handleGetSession(ctx, c, application)
	})
	h.PATCH("/api/sessions/:id", func(ctx context.Context, c *hertzapp.RequestContext) {
		handlePatchSession(ctx, c, application)
	})
	h.POST("/api/sessions/:id/archive", func(ctx context.Context, c *hertzapp.RequestContext) {
		handleArchiveSession(ctx, c, application)
	})
	h.POST("/api/chat/stream", func(ctx context.Context, c *hertzapp.RequestContext) {
		handleChat(ctx, c, cfg, application)
	})
	h.POST("/api/chat/respond", func(ctx context.Context, c *hertzapp.RequestContext) {
		handleChatRespond(ctx, c, cfg, application)
	})
	h.OPTIONS("/api/control-plane/*path", func(ctx context.Context, c *hertzapp.RequestContext) {
		handleControlPlaneOptions(ctx, c, cfg)
	})
	h.GET("/api/control-plane/auth/status", func(ctx context.Context, c *hertzapp.RequestContext) {
		handleControlPlaneAuthStatus(ctx, c, cfg, application)
	})
	h.POST("/api/control-plane/login", func(ctx context.Context, c *hertzapp.RequestContext) {
		handleControlPlaneLogin(ctx, c, cfg, application)
	})
	h.POST("/api/control-plane/logout", func(ctx context.Context, c *hertzapp.RequestContext) {
		handleControlPlaneLogout(ctx, c, cfg, application)
	})
	h.GET("/api/control-plane/bootstrap", func(ctx context.Context, c *hertzapp.RequestContext) {
		withControlPlaneAuth(cfg, application, handleControlPlaneBootstrap)(ctx, c, cfg, application)
	})
	h.GET("/api/control-plane/scenes", func(ctx context.Context, c *hertzapp.RequestContext) {
		withControlPlaneAuth(cfg, application, handleListControlPlaneScenes)(ctx, c, cfg, application)
	})
	h.PUT("/api/control-plane/scenes/:id", func(ctx context.Context, c *hertzapp.RequestContext) {
		withControlPlaneAuth(cfg, application, handlePutControlPlaneScene)(ctx, c, cfg, application)
	})
	h.GET("/api/control-plane/skills", func(ctx context.Context, c *hertzapp.RequestContext) {
		withControlPlaneAuth(cfg, application, handleListControlPlaneSkills)(ctx, c, cfg, application)
	})
	h.PUT("/api/control-plane/skills/:name", func(ctx context.Context, c *hertzapp.RequestContext) {
		withControlPlaneAuth(cfg, application, handlePutControlPlaneSkill)(ctx, c, cfg, application)
	})
	h.GET("/api/control-plane/tools", func(ctx context.Context, c *hertzapp.RequestContext) {
		withControlPlaneAuth(cfg, application, handleListControlPlaneTools)(ctx, c, cfg, application)
	})
	h.PUT("/api/control-plane/tools/:name", func(ctx context.Context, c *hertzapp.RequestContext) {
		withControlPlaneAuth(cfg, application, handlePutControlPlaneTool)(ctx, c, cfg, application)
	})
	h.GET("/api/control-plane/runtime-config", func(ctx context.Context, c *hertzapp.RequestContext) {
		withControlPlaneAuth(cfg, application, handleGetControlPlaneRuntime)(ctx, c, cfg, application)
	})
	h.PUT("/api/control-plane/runtime-config", func(ctx context.Context, c *hertzapp.RequestContext) {
		withControlPlaneAuth(cfg, application, handlePutControlPlaneRuntime)(ctx, c, cfg, application)
	})
	h.POST("/api/control-plane/runtime/validation-runs", func(ctx context.Context, c *hertzapp.RequestContext) {
		withControlPlaneAuth(cfg, application, handleCreateControlPlaneRuntimeValidationRun)(ctx, c, cfg, application)
	})
	h.GET("/api/control-plane/runtime/runs", func(ctx context.Context, c *hertzapp.RequestContext) {
		withControlPlaneAuth(cfg, application, handleListControlPlaneRuntimeRuns)(ctx, c, cfg, application)
	})
	h.GET("/api/control-plane/runtime/runs/:runID", func(ctx context.Context, c *hertzapp.RequestContext) {
		withControlPlaneAuth(cfg, application, handleGetControlPlaneRuntimeRun)(ctx, c, cfg, application)
	})
	h.GET("/api/control-plane/runtime/runs/:runID/steps", func(ctx context.Context, c *hertzapp.RequestContext) {
		withControlPlaneAuth(cfg, application, handleListControlPlaneRuntimeSteps)(ctx, c, cfg, application)
	})
	h.GET("/api/control-plane/runtime/runs/:runID/lifecycle", func(ctx context.Context, c *hertzapp.RequestContext) {
		withControlPlaneAuth(cfg, application, handleListControlPlaneRuntimeLifecycleEvents)(ctx, c, cfg, application)
	})
	h.GET("/api/control-plane/runtime/runs/:runID/traces", func(ctx context.Context, c *hertzapp.RequestContext) {
		withControlPlaneAuth(cfg, application, handleListControlPlaneRuntimeTraces)(ctx, c, cfg, application)
	})
	h.GET("/api/control-plane/runtime/runs/:runID/usage", func(ctx context.Context, c *hertzapp.RequestContext) {
		withControlPlaneAuth(cfg, application, handleListControlPlaneRuntimeUsage)(ctx, c, cfg, application)
	})
	h.GET("/api/control-plane/runtime/runs/:runID/projections", func(ctx context.Context, c *hertzapp.RequestContext) {
		withControlPlaneAuth(cfg, application, handleListControlPlaneRuntimeProjectionCandidates)(ctx, c, cfg, application)
	})
	h.GET("/api/control-plane/runtime/contracts/foundation", func(ctx context.Context, c *hertzapp.RequestContext) {
		withControlPlaneAuth(cfg, application, handleGetControlPlaneRuntimeContractFoundation)(ctx, c, cfg, application)
	})
	h.PUT("/api/control-plane/runtime/contracts/:contractID", func(ctx context.Context, c *hertzapp.RequestContext) {
		withControlPlaneAuth(cfg, application, handlePutControlPlaneRuntimeContract)(ctx, c, cfg, application)
	})
	h.PUT("/api/control-plane/runtime/task-types/:typeKey", func(ctx context.Context, c *hertzapp.RequestContext) {
		withControlPlaneAuth(cfg, application, handlePutControlPlaneRuntimeTaskType)(ctx, c, cfg, application)
	})
	h.PUT("/api/control-plane/runtime/hook-bindings/:bindingID", func(ctx context.Context, c *hertzapp.RequestContext) {
		withControlPlaneAuth(cfg, application, handlePutControlPlaneRuntimeHookBinding)(ctx, c, cfg, application)
	})
	h.GET("/api/control-plane/governance", func(ctx context.Context, c *hertzapp.RequestContext) {
		withControlPlaneAuth(cfg, application, handleGetControlPlaneGovernance)(ctx, c, cfg, application)
	})
	h.PUT("/api/control-plane/governance", func(ctx context.Context, c *hertzapp.RequestContext) {
		withControlPlaneAuth(cfg, application, handlePutControlPlaneGovernance)(ctx, c, cfg, application)
	})
	h.GET("/api/control-plane/tool-governance/policy", func(ctx context.Context, c *hertzapp.RequestContext) {
		withControlPlaneAuth(cfg, application, handleGetToolGovernancePolicy)(ctx, c, cfg, application)
	})
	h.GET("/api/control-plane/tool-governance/decisions", func(ctx context.Context, c *hertzapp.RequestContext) {
		withControlPlaneAuth(cfg, application, handleListToolGovernanceDecisions)(ctx, c, cfg, application)
	})
	h.POST("/api/control-plane/tool-governance/evaluate", func(ctx context.Context, c *hertzapp.RequestContext) {
		withControlPlaneAuth(cfg, application, handleEvaluateToolGovernance)(ctx, c, cfg, application)
	})
	h.GET("/api/control-plane/validation-mcp/server", func(ctx context.Context, c *hertzapp.RequestContext) {
		withControlPlaneAuth(cfg, application, handleGetValidationMCPServer)(ctx, c, cfg, application)
	})
	h.GET("/api/control-plane/validation-mcp/tools", func(ctx context.Context, c *hertzapp.RequestContext) {
		withControlPlaneAuth(cfg, application, handleListValidationMCPTools)(ctx, c, cfg, application)
	})
	h.POST("/api/control-plane/validation-mcp/invocations", func(ctx context.Context, c *hertzapp.RequestContext) {
		withControlPlaneAuth(cfg, application, handleInvokeValidationMCPTool)(ctx, c, cfg, application)
	})
	h.GET("/api/control-plane/config-versions", func(ctx context.Context, c *hertzapp.RequestContext) {
		withControlPlaneAuth(cfg, application, handleListControlPlaneConfigVersions)(ctx, c, cfg, application)
	})
	h.GET("/api/control-plane/config-versions/:versionID", func(ctx context.Context, c *hertzapp.RequestContext) {
		withControlPlaneAuth(cfg, application, handleGetControlPlaneConfigVersion)(ctx, c, cfg, application)
	})
	h.POST("/api/control-plane/config-versions/:versionID/rollback", func(ctx context.Context, c *hertzapp.RequestContext) {
		withControlPlaneAuth(cfg, application, handleRollbackControlPlaneConfigVersion)(ctx, c, cfg, application)
	})
	h.GET("/api/system-resources", func(ctx context.Context, c *hertzapp.RequestContext) {
		withControlPlaneAuth(cfg, application, handleListSystemResources)(ctx, c, cfg, application)
	})
	h.POST("/api/system-resources/sync", func(ctx context.Context, c *hertzapp.RequestContext) {
		withControlPlaneAuth(cfg, application, handleSyncSystemResources)(ctx, c, cfg, application)
	})
	h.POST("/api/system-resources", func(ctx context.Context, c *hertzapp.RequestContext) {
		withControlPlaneAuth(cfg, application, handleCreateSystemResource)(ctx, c, cfg, application)
	})
	h.POST("/api/system-resources/build-package", func(ctx context.Context, c *hertzapp.RequestContext) {
		withControlPlaneAuth(cfg, application, handleBuildSystemAssetsPackage)(ctx, c, cfg, application)
	})
	h.GET("/api/system-resources/export", func(ctx context.Context, c *hertzapp.RequestContext) {
		withControlPlaneAuth(cfg, application, handleExportSystemResources)(ctx, c, cfg, application)
	})
	h.GET("/api/system-resources/:id", func(ctx context.Context, c *hertzapp.RequestContext) {
		withControlPlaneAuth(cfg, application, handleGetSystemResource)(ctx, c, cfg, application)
	})
	h.GET("/api/system-resources/:id/versions", func(ctx context.Context, c *hertzapp.RequestContext) {
		withControlPlaneAuth(cfg, application, handleListSystemResourceVersions)(ctx, c, cfg, application)
	})
	h.GET("/api/system-resources/:id/versions/:versionID", func(ctx context.Context, c *hertzapp.RequestContext) {
		withControlPlaneAuth(cfg, application, handleGetSystemResourceVersion)(ctx, c, cfg, application)
	})
	h.POST("/api/system-resources/:id/versions/:versionID/rollback", func(ctx context.Context, c *hertzapp.RequestContext) {
		withControlPlaneAuth(cfg, application, handleRollbackSystemResourceVersion)(ctx, c, cfg, application)
	})
	h.GET("/api/system-resources/:id/audit", func(ctx context.Context, c *hertzapp.RequestContext) {
		withControlPlaneAuth(cfg, application, handleListSystemResourceAudit)(ctx, c, cfg, application)
	})
	h.DELETE("/api/system-resources/:id", func(ctx context.Context, c *hertzapp.RequestContext) {
		withControlPlaneAuth(cfg, application, handleDeleteSystemResource)(ctx, c, cfg, application)
	})
	h.PATCH("/api/system-resources/:id/metadata", func(ctx context.Context, c *hertzapp.RequestContext) {
		withControlPlaneAuth(cfg, application, handlePatchSystemResourceMetadata)(ctx, c, cfg, application)
	})
	h.GET("/api/system-resources/:id/source", func(ctx context.Context, c *hertzapp.RequestContext) {
		withControlPlaneAuth(cfg, application, handleGetSystemResourceSource)(ctx, c, cfg, application)
	})
	h.PUT("/api/system-resources/:id/source", func(ctx context.Context, c *hertzapp.RequestContext) {
		withControlPlaneAuth(cfg, application, handlePutSystemResourceSource)(ctx, c, cfg, application)
	})
	h.POST("/api/system-resources/:id/parse", func(ctx context.Context, c *hertzapp.RequestContext) {
		withControlPlaneAuth(cfg, application, handleParseSystemResource)(ctx, c, cfg, application)
	})
	h.POST("/api/system-resources/:id/compile", func(ctx context.Context, c *hertzapp.RequestContext) {
		withControlPlaneAuth(cfg, application, handleCompileSystemResource)(ctx, c, cfg, application)
	})
	h.POST("/api/system-resources/:id/activate", func(ctx context.Context, c *hertzapp.RequestContext) {
		withControlPlaneAuth(cfg, application, handleActivateSystemResource)(ctx, c, cfg, application)
	})
	h.GET("/api/system-resources/:id/pipeline", func(ctx context.Context, c *hertzapp.RequestContext) {
		withControlPlaneAuth(cfg, application, handleGetSystemResourcePipeline)(ctx, c, cfg, application)
	})
	h.GET("/api/system-resources/:id/parse-result", func(ctx context.Context, c *hertzapp.RequestContext) {
		withControlPlaneAuth(cfg, application, handleGetSystemResourceParseResult)(ctx, c, cfg, application)
	})
	h.GET("/api/system-resources/:id/compile-result", func(ctx context.Context, c *hertzapp.RequestContext) {
		withControlPlaneAuth(cfg, application, handleGetSystemResourceCompileResult)(ctx, c, cfg, application)
	})
	h.GET("/api/system-resources/:id/debug-payload", func(ctx context.Context, c *hertzapp.RequestContext) {
		withControlPlaneAuth(cfg, application, handleGetSystemResourceDebugPayload)(ctx, c, cfg, application)
	})
	h.GET("/api/system-resources/:id/download", func(ctx context.Context, c *hertzapp.RequestContext) {
		withControlPlaneAuth(cfg, application, handleDownloadSystemResource)(ctx, c, cfg, application)
	})
	h.GET("/api/runtime/skills", func(ctx context.Context, c *hertzapp.RequestContext) {
		handleListRuntimeSkills(ctx, c, application)
	})
	h.POST("/api/runtime/respond", func(ctx context.Context, c *hertzapp.RequestContext) {
		handleChatRespond(ctx, c, cfg, application)
	})
	h.POST("/api/runtime/scenario/respond", func(ctx context.Context, c *hertzapp.RequestContext) {
		handleRuntimeScenarioRespond(ctx, c, application)
	})
	h.GET("/api/skills", func(ctx context.Context, c *hertzapp.RequestContext) {
		handleListSkills(ctx, c, application)
	})
	h.GET("/api/skills/packages", func(ctx context.Context, c *hertzapp.RequestContext) {
		handleListSkillPackages(ctx, c, application)
	})
	h.GET("/api/skills/packages/:id/revisions", func(ctx context.Context, c *hertzapp.RequestContext) {
		handleListSkillPackageRevisions(ctx, c, application)
	})
	h.GET("/api/skills/packages/:id", func(ctx context.Context, c *hertzapp.RequestContext) {
		handleGetSkillPackage(ctx, c, application)
	})
	h.POST("/api/skills/packages", func(ctx context.Context, c *hertzapp.RequestContext) {
		handleUploadSkillPackage(ctx, c, application)
	})
	h.PUT("/api/skills/packages/:id", func(ctx context.Context, c *hertzapp.RequestContext) {
		handleReplaceSkillPackage(ctx, c, application)
	})
	h.POST("/api/skills/packages/:id/rollback", func(ctx context.Context, c *hertzapp.RequestContext) {
		handleRollbackSkillPackage(ctx, c, application)
	})
	h.PATCH("/api/skills/packages/:id", func(ctx context.Context, c *hertzapp.RequestContext) {
		handleUpdateSkillPackageState(ctx, c, application)
	})
	h.DELETE("/api/skills/packages/:id", func(ctx context.Context, c *hertzapp.RequestContext) {
		handleDeleteSkillPackage(ctx, c, application)
	})
	h.GET("/api/models/providers", func(ctx context.Context, c *hertzapp.RequestContext) {
		handleListModelProviders(ctx, c, application)
	})
	h.POST("/api/models/providers", func(ctx context.Context, c *hertzapp.RequestContext) {
		handleCreateModelProvider(ctx, c, application)
	})
	h.PUT("/api/models/providers/:id", func(ctx context.Context, c *hertzapp.RequestContext) {
		handleUpdateModelProvider(ctx, c, application)
	})
	h.PATCH("/api/models/providers/:id", func(ctx context.Context, c *hertzapp.RequestContext) {
		handlePatchModelProvider(ctx, c, application)
	})
	h.DELETE("/api/models/providers/:id", func(ctx context.Context, c *hertzapp.RequestContext) {
		handleDeleteModelProvider(ctx, c, application)
	})
	h.POST("/api/models/providers/:id/models", func(ctx context.Context, c *hertzapp.RequestContext) {
		handleCreateProviderModel(ctx, c, application)
	})
	h.PUT("/api/models/providers/:id/models/:record_id", func(ctx context.Context, c *hertzapp.RequestContext) {
		handleUpdateProviderModel(ctx, c, application)
	})
	h.PATCH("/api/models/providers/:id/models/:record_id", func(ctx context.Context, c *hertzapp.RequestContext) {
		handlePatchProviderModel(ctx, c, application)
	})
	h.DELETE("/api/models/providers/:id/models/:record_id", func(ctx context.Context, c *hertzapp.RequestContext) {
		handleDeleteProviderModel(ctx, c, application)
	})
	h.POST("/api/models/providers/:id/models/:record_id/test", func(ctx context.Context, c *hertzapp.RequestContext) {
		handleTestProviderModel(ctx, c, application)
	})
	return &HTTPServer{engine: h}
}

// Spin starts the underlying Hertz engine.
// Spin 启动底层 Hertz 服务。
func (h *HTTPServer) Spin() {
	h.engine.Spin()
}

// handleChat converts one HTTP request into either an immediate SSE outcome or a streamed runner execution.
// handleChat 会把一次 HTTP 请求转换成即时 SSE 结果或正式的流式 runner 执行。
func handleChat(ctx context.Context, c *hertzapp.RequestContext, cfg config.Config, application *appcore.Service) {
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(cfg.Runtime.RequestTimeoutSeconds)*time.Second)
	defer cancel()

	requestID := newRequestID()
	ctx = withRequestID(timeoutCtx, requestID)
	c.Header("X-Request-ID", requestID)

	req, err := parseChatStreamRequest(c)
	if err != nil {
		c.JSON(consts.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
		return
	}

	query := strings.TrimSpace(req.Query)
	if query == "" && req.Supplement == nil && strings.TrimSpace(req.TaskType) == "" {
		c.JSON(consts.StatusBadRequest, map[string]string{
			"error": "query is required unless supplement is provided",
		})
		return
	}

	custom := customization.UserCustomization{
		PromptTemplate:         strings.TrimSpace(req.PromptTemplate),
		EnabledSkills:          req.EnabledSkills,
		EnabledTools:           req.EnabledTools,
		ContextAssetOverrides:  req.ContextAssetOverrides,
		DisabledAssetTypes:     req.DisabledAssetTypes,
		AssetPriorityOverrides: req.AssetPriorityOverrides,
	}
	preparedPlatform := preparePlatformContext(ctx, c, cfg, req.GlobalContext, platformcontext.UsageInput{
		Query:             query,
		TaskType:          strings.TrimSpace(req.TaskType),
		Scene:             strings.TrimSpace(req.Scene),
		DesiredOutputMode: strings.TrimSpace(req.DesiredOutputMode),
	})
	preparedAssets := prepareContextAssets(ctx, application, preparedPlatform.GlobalContext, custom, contextassets.UsageInput{
		Query:             query,
		TaskType:          strings.TrimSpace(req.TaskType),
		Scene:             strings.TrimSpace(req.Scene),
		DesiredOutputMode: strings.TrimSpace(req.DesiredOutputMode),
	})
	req.GlobalContext = preparedAssets.GlobalContext

	chatSession, err := application.OpenChatSession(ctx, requestID, appcore.ChatRequest{
		TaskType:              strings.TrimSpace(req.TaskType),
		TaskSubtype:           strings.TrimSpace(req.TaskSubtype),
		Scene:                 strings.TrimSpace(req.Scene),
		Query:                 query,
		SessionID:             strings.TrimSpace(req.SessionID),
		MainSessionID:         strings.TrimSpace(req.MainSessionID),
		WorkspaceID:           strings.TrimSpace(req.WorkspaceID),
		AppInstanceID:         strings.TrimSpace(req.AppInstanceID),
		AppSessionID:          strings.TrimSpace(req.AppSessionID),
		IntegrationInstanceID: strings.TrimSpace(req.IntegrationInstanceID),
		WorkflowRunID:         strings.TrimSpace(req.WorkflowRunID),
		StepID:                strings.TrimSpace(req.StepID),
		TriggerType:           strings.TrimSpace(req.TriggerType),
		AutomationTaskID:      strings.TrimSpace(req.AutomationTaskID),
		UserLanguage:          strings.TrimSpace(req.UserLanguage),
		DesiredOutputMode:     strings.TrimSpace(req.DesiredOutputMode),
		GlobalContext:         cloneAnyMap(req.GlobalContext),
		AppContext:            cloneAnyMap(req.AppContext),
		InputPayload:          cloneAnyMap(req.InputPayload),
		ModelID:               strings.TrimSpace(req.ModelID),
		Customization:         custom,
		Supplement:            req.Supplement,
		TimeoutAfter:          time.Duration(req.TimeoutAfterSeconds) * time.Second,
		DisableFastPath:       req.DisableFastPath,
	})
	if err != nil {
		var pendingErr *appcore.PendingWaitError
		if errors.As(err, &pendingErr) {
			sessionID := strings.TrimSpace(req.SessionID)
			if pendingErr.SessionID != "" {
				sessionID = pendingErr.SessionID
			}
			c.Header("X-Session-ID", sessionID)
			stream := sse.NewStream(c)
			if err := sendSSEEvent(stream, StreamEvent{
				Type:      "request_started",
				RequestID: requestID,
				SessionID: sessionID,
				Status:    "running",
			}); err != nil {
				application.Observability.LogAction(ctx, observability.LogLevelError, observability.ActionLog{
					Module:    "server",
					Action:    "chat_stream",
					Step:      "write_request_started",
					Status:    "error",
					RequestID: requestID,
					SessionID: sessionID,
					Reason:    "sse_write_failed",
					ErrorCode: "sse_request_started_write_failed",
					Detail:    map[string]any{"error": err.Error()},
				})
				return
			}
			if err := writePendingWaitAndFinish(stream, requestID, sessionID, pendingErr.Pending, pendingErr.Queued, pendingErr.Dropped); err != nil {
				application.Observability.LogAction(ctx, observability.LogLevelError, observability.ActionLog{
					Module:    "server",
					Action:    "chat_stream",
					Step:      "write_pending_wait",
					Status:    "error",
					RequestID: requestID,
					SessionID: sessionID,
					Reason:    "sse_write_failed",
					ErrorCode: "sse_pending_wait_write_failed",
					Detail:    map[string]any{"error": err.Error()},
				})
			}
			return
		}
		var invalidTokenErr *appcore.InvalidResumeTokenError
		if errors.As(err, &invalidTokenErr) {
			sessionID := strings.TrimSpace(req.SessionID)
			if invalidTokenErr.SessionID != "" {
				sessionID = invalidTokenErr.SessionID
			}
			c.Header("X-Session-ID", sessionID)
			stream := sse.NewStream(c)
			if err := sendSSEEvent(stream, StreamEvent{
				Type:      "request_started",
				RequestID: requestID,
				SessionID: sessionID,
				Status:    "running",
			}); err != nil {
				application.Observability.LogAction(ctx, observability.LogLevelError, observability.ActionLog{
					Module:    "server",
					Action:    "chat_stream",
					Step:      "write_request_started",
					Status:    "error",
					RequestID: requestID,
					SessionID: sessionID,
					Reason:    "sse_write_failed",
					ErrorCode: "sse_request_started_write_failed",
					Detail:    map[string]any{"error": err.Error()},
				})
				return
			}
			if err := writeInvalidResumeTokenAndFinish(stream, requestID, sessionID, invalidTokenErr); err != nil {
				application.Observability.LogAction(ctx, observability.LogLevelError, observability.ActionLog{
					Module:    "server",
					Action:    "chat_stream",
					Step:      "write_invalid_resume_token",
					Status:    "error",
					RequestID: requestID,
					SessionID: sessionID,
					Reason:    "sse_write_failed",
					ErrorCode: "sse_invalid_resume_write_failed",
					Detail:    map[string]any{"error": err.Error()},
				})
			}
			return
		}
		var invalidTaskErr *appcore.InvalidTaskRequestError
		if errors.As(err, &invalidTaskErr) {
			c.JSON(consts.StatusBadRequest, map[string]any{
				"error":          invalidTaskErr.Error(),
				"task_type":      invalidTaskErr.TaskType,
				"reason":         invalidTaskErr.Reason,
				"missing_fields": invalidTaskErr.MissingFields,
			})
			return
		}
		status := consts.StatusInternalServerError
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			status = consts.StatusTooManyRequests
		}
		c.JSON(status, map[string]string{
			"error": err.Error(),
		})
		return
	}
	defer chatSession.Release()

	stream := sse.NewStream(c)
	c.Header("X-Session-ID", chatSession.SessionID)
	var assistantOutput strings.Builder
	defer func() {
		_ = c.Flush()
	}()

	if err := sendSSEEvent(stream, StreamEvent{
		Type:      "request_started",
		RequestID: requestID,
		SessionID: chatSession.SessionID,
		Status:    "running",
		Detail:    requestStartedDetail(chatSession.FastPath, req.DisableFastPath, chatSession.Dequeued),
	}); err != nil {
		application.Observability.LogAction(ctx, observability.LogLevelError, observability.ActionLog{
			Module:    "server",
			Action:    "chat_stream",
			Step:      "write_request_started",
			Status:    "error",
			RequestID: requestID,
			SessionID: chatSession.SessionID,
			Reason:    "sse_write_failed",
			ErrorCode: "sse_request_started_write_failed",
			Detail:    map[string]any{"error": err.Error()},
		})
		return
	}

	if err := writeGapClosedNotification(stream, requestID, chatSession.SessionID, chatSession.GapClosed); err != nil {
		application.Observability.LogAction(ctx, observability.LogLevelError, observability.ActionLog{
			Module:    "server",
			Action:    "chat_stream",
			Step:      "write_gap_closed",
			Status:    "error",
			RequestID: requestID,
			SessionID: chatSession.SessionID,
			Reason:    "sse_write_failed",
			ErrorCode: "sse_gap_closed_write_failed",
			Detail:    map[string]any{"error": err.Error()},
		})
		return
	}

	tracker := newProgressStepTracker(requestID, chatSession.SessionID, stream)
	if err := tracker.emitContextReady(); err != nil {
		application.Observability.LogAction(ctx, observability.LogLevelError, observability.ActionLog{
			Module:    "server",
			Action:    "chat_stream",
			Step:      "write_progress_context",
			Status:    "error",
			RequestID: requestID,
			SessionID: chatSession.SessionID,
			Reason:    "sse_write_failed",
			ErrorCode: "sse_progress_context_write_failed",
			Detail:    map[string]any{"error": err.Error()},
		})
		return
	}
	if err := tracker.startAnalysis(); err != nil {
		application.Observability.LogAction(ctx, observability.LogLevelError, observability.ActionLog{
			Module:    "server",
			Action:    "chat_stream",
			Step:      "write_progress_analysis",
			Status:    "error",
			RequestID: requestID,
			SessionID: chatSession.SessionID,
			Reason:    "sse_write_failed",
			ErrorCode: "sse_progress_analysis_write_failed",
			Detail:    map[string]any{"error": err.Error()},
		})
		return
	}

	if chatSession.Prepared.Initial != nil && chatSession.Prepared.Initial.Action != nil {
		if err := writeInitialActionAndFinish(stream, requestID, chatSession.SessionID, chatSession.Prepared, tracker); err != nil {
			application.Observability.LogAction(ctx, observability.LogLevelError, observability.ActionLog{
				Module:    "server",
				Action:    "chat_stream",
				Step:      "write_initial_action",
				Status:    "error",
				RequestID: requestID,
				SessionID: chatSession.SessionID,
				Reason:    "sse_write_failed",
				ErrorCode: "sse_initial_action_write_failed",
				Detail:    map[string]any{"error": err.Error()},
			})
			return
		}
		if err := chatSession.Complete(ctx, ""); err != nil {
			application.Observability.LogAction(ctx, observability.LogLevelWarn, observability.ActionLog{
				Module:    "server",
				Action:    "chat_stream",
				Step:      "complete_initial_action_session",
				Status:    "error",
				RequestID: requestID,
				SessionID: chatSession.SessionID,
				Reason:    "session_save_failed",
				ErrorCode: "session_complete_failed",
				Detail:    map[string]any{"error": err.Error()},
			})
		}
		return
	}
	if chatSession.Prepared.Initial != nil && chatSession.Prepared.Initial.Error != "" {
		if err := writeInitialErrorAndFinish(stream, requestID, chatSession.SessionID, chatSession.Prepared, tracker); err != nil {
			application.Observability.LogAction(ctx, observability.LogLevelError, observability.ActionLog{
				Module:    "server",
				Action:    "chat_stream",
				Step:      "write_initial_error",
				Status:    "error",
				RequestID: requestID,
				SessionID: chatSession.SessionID,
				Reason:    "sse_write_failed",
				ErrorCode: "sse_initial_error_write_failed",
				Detail:    map[string]any{"error": err.Error()},
			})
			return
		}
		if err := chatSession.Complete(ctx, ""); err != nil {
			application.Observability.LogAction(ctx, observability.LogLevelWarn, observability.ActionLog{
				Module:    "server",
				Action:    "chat_stream",
				Step:      "complete_initial_error_session",
				Status:    "error",
				RequestID: requestID,
				SessionID: chatSession.SessionID,
				Reason:    "session_save_failed",
				ErrorCode: "session_complete_failed",
				Detail:    map[string]any{"error": err.Error()},
			})
		}
		return
	}

	iter := chatSession.Prepared.Runner.Run(ctx, chatSession.Prepared.Messages)
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		if err := processAgentEvent(ctx, stream, event, &assistantOutput, chatSession.SessionID, tracker); err != nil {
			application.Observability.LogAction(ctx, observability.LogLevelError, observability.ActionLog{
				Module:    "server",
				Action:    "chat_stream",
				Step:      "process_agent_event",
				Status:    "error",
				RequestID: requestID,
				SessionID: chatSession.SessionID,
				Reason:    "stream_event_processing_failed",
				ErrorCode: "stream_event_processing_failed",
				Detail:    map[string]any{"error": err.Error()},
			})
			break
		}
	}

	if assistantOutput.Len() == 0 {
		if err := tracker.startResponse(); err != nil {
			application.Observability.LogAction(ctx, observability.LogLevelError, observability.ActionLog{
				Module:    "server",
				Action:    "chat_stream",
				Step:      "write_progress_response",
				Status:    "error",
				RequestID: requestID,
				SessionID: chatSession.SessionID,
				Reason:    "sse_write_failed",
				ErrorCode: "sse_progress_response_write_failed",
				Detail:    map[string]any{"error": err.Error()},
			})
			return
		}
	}

	if err := chatSession.Complete(ctx, assistantOutput.String()); err != nil {
		application.Observability.LogAction(ctx, observability.LogLevelWarn, observability.ActionLog{
			Module:    "server",
			Action:    "chat_stream",
			Step:      "complete_session",
			Status:    "error",
			RequestID: requestID,
			SessionID: chatSession.SessionID,
			Reason:    "session_save_failed",
			ErrorCode: "session_complete_failed",
			Detail:    map[string]any{"error": err.Error()},
		})
	}

	if err := emitStructuredCompletionEvents(ctx, stream, requestID, chatSession.SessionID, chatSession.Prepared, req, assistantOutput.String(), cfg.Runtime.SharedRootDir, chatSession.Session); err != nil {
		application.Observability.LogAction(ctx, observability.LogLevelError, observability.ActionLog{
			Module:    "server",
			Action:    "chat_stream",
			Step:      "write_structured_completion",
			Status:    "error",
			RequestID: requestID,
			SessionID: chatSession.SessionID,
			Reason:    "sse_write_failed",
			ErrorCode: "sse_structured_completion_failed",
			Detail:    map[string]any{"error": err.Error()},
		})
	}

	if err := tracker.completeResponse(""); err != nil {
		application.Observability.LogAction(ctx, observability.LogLevelError, observability.ActionLog{
			Module:    "server",
			Action:    "chat_stream",
			Step:      "write_progress_response_completed",
			Status:    "error",
			RequestID: requestID,
			SessionID: chatSession.SessionID,
			Reason:    "sse_write_failed",
			ErrorCode: "sse_progress_response_completed_write_failed",
			Detail:    map[string]any{"error": err.Error()},
		})
		return
	}

	doneDetail := requestStartedDetail(chatSession.FastPath, req.DisableFastPath, chatSession.Dequeued)
	doneDetail = withStepFlowDetail(doneDetail, tracker.stepFlowSummary("completed", "Athena finished this request. The user-visible step flow can be collapsed and the final result can remain expanded."))
	if err := writeCompletedAndFinish(stream, requestID, chatSession.SessionID, chatSession.Prepared, doneDetail); err != nil {
		application.Observability.LogAction(ctx, observability.LogLevelError, observability.ActionLog{
			Module:    "server",
			Action:    "chat_stream",
			Step:      "write_done",
			Status:    "error",
			RequestID: requestID,
			SessionID: chatSession.SessionID,
			Reason:    "sse_write_failed",
			ErrorCode: "sse_done_write_failed",
			Detail:    map[string]any{"error": err.Error()},
		})
	}
}

func parseChatStreamRequest(c *hertzapp.RequestContext) (ChatStreamRequest, error) {
	var req ChatStreamRequest
	body := bytesTrimSpace(c.Request.Body())
	if len(body) == 0 {
		return ChatStreamRequest{}, fmt.Errorf("request body is required")
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return ChatStreamRequest{}, fmt.Errorf("invalid json body: %w", err)
	}
	if req.Supplement == nil && req.SupplementOutcome != "" {
		req.Supplement = &runtime.SupplementPayload{}
	}
	if req.Supplement != nil {
		if req.Supplement.Outcome == "" && req.SupplementOutcome != "" {
			req.Supplement.Outcome = req.SupplementOutcome
		}
		if req.Supplement.Resume == nil && strings.TrimSpace(req.ResumeToken) != "" {
			req.Supplement.Resume = &runtime.ResumeContext{}
		}
		if req.Supplement.Resume != nil && req.Supplement.Resume.ResumeToken == "" {
			req.Supplement.Resume.ResumeToken = strings.TrimSpace(req.ResumeToken)
		}
		if len(req.Supplement.Data) > 0 && req.Supplement.Outcome == "" {
			req.Supplement.Outcome = runtime.SupplementOutcomeProvided
		}
		if len(req.Supplement.Data) == 0 && req.Supplement.Outcome == "" && req.Supplement.Resume == nil {
			req.Supplement = nil
		}
	}
	if strings.TrimSpace(req.ModelRecordID) != "" {
		return ChatStreamRequest{}, fmt.Errorf("model_record_id is no longer supported; use model_id")
	}
	return req, nil
}

func parseChatRespondRequest(c *hertzapp.RequestContext) (ChatRespondRequest, error) {
	var req ChatRespondRequest
	if len(c.Request.Body()) == 0 {
		return ChatRespondRequest{}, fmt.Errorf("request body is required")
	}
	if err := json.Unmarshal(c.Request.Body(), &req); err != nil {
		return ChatRespondRequest{}, fmt.Errorf("invalid json body: %w", err)
	}

	if req.Supplement != nil {
		if req.Supplement.Outcome == "" && len(req.Supplement.Data) > 0 {
			req.Supplement.Outcome = runtime.SupplementOutcomeProvided
		}
		if req.ResumeToken != "" && (req.Supplement.Resume == nil || strings.TrimSpace(req.Supplement.Resume.ResumeToken) == "") {
			if req.Supplement.Resume == nil {
				req.Supplement.Resume = &runtime.ResumeContext{}
			}
			req.Supplement.Resume.ResumeToken = req.ResumeToken
		}
	}
	if req.Supplement == nil && req.SupplementOutcome != "" {
		req.Supplement = &runtime.SupplementPayload{Outcome: req.SupplementOutcome}
	} else if req.Supplement != nil && req.Supplement.Outcome == "" && req.SupplementOutcome != "" {
		req.Supplement.Outcome = req.SupplementOutcome
	}
	if req.Supplement != nil && req.Supplement.Resume != nil && strings.TrimSpace(req.Supplement.Resume.ResumeToken) == "" && req.ResumeToken != "" {
		req.Supplement.Resume.ResumeToken = req.ResumeToken
	}

	if req.ModelRecordID != "" {
		return ChatRespondRequest{}, fmt.Errorf("model_record_id is no longer supported; use model_id")
	}

	return req, nil
}

func cloneAnyMap(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	result := make(map[string]any, len(input))
	for key, value := range input {
		result[key] = value
	}
	return result
}

func bytesTrimSpace(value []byte) []byte {
	return []byte(strings.TrimSpace(string(value)))
}

func handleCreateSession(ctx context.Context, c *hertzapp.RequestContext, application *appcore.Service) {
	if application == nil {
		c.JSON(consts.StatusInternalServerError, map[string]string{"error": "application is not configured"})
		return
	}
	var req CreateSessionRequest
	body := bytesTrimSpace(c.Request.Body())
	if len(body) > 0 {
		if err := json.Unmarshal(body, &req); err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "invalid session payload"})
			return
		}
	}
	item, err := application.CreateSession(ctx, req.Title)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	c.JSON(consts.StatusCreated, item)
}

func handleListSessions(ctx context.Context, c *hertzapp.RequestContext, application *appcore.Service) {
	if application == nil {
		c.JSON(consts.StatusInternalServerError, map[string]string{"error": "application is not configured"})
		return
	}
	query := appcore.SessionListQuery{
		Status: strings.TrimSpace(c.Query("status")),
		Limit:  parseOptionalInt(c.Query("limit")),
		Offset: parseOptionalInt(c.Query("offset")),
	}
	if archivedRaw := strings.TrimSpace(c.Query("archived")); archivedRaw != "" {
		value := strings.EqualFold(archivedRaw, "true")
		query.Archived = &value
	}
	items, err := application.ListSessions(ctx, query)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	c.JSON(consts.StatusOK, map[string]any{"items": items})
}

func handleGetSession(ctx context.Context, c *hertzapp.RequestContext, application *appcore.Service) {
	if application == nil {
		c.JSON(consts.StatusInternalServerError, map[string]string{"error": "application is not configured"})
		return
	}
	item, ok, err := application.GetSession(ctx, strings.TrimSpace(c.Param("id")))
	if err != nil {
		c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if !ok {
		c.JSON(consts.StatusNotFound, map[string]string{"error": "session was not found"})
		return
	}
	c.JSON(consts.StatusOK, item)
}

func handlePatchSession(ctx context.Context, c *hertzapp.RequestContext, application *appcore.Service) {
	if application == nil {
		c.JSON(consts.StatusInternalServerError, map[string]string{"error": "application is not configured"})
		return
	}
	var req PatchSessionRequest
	body := bytesTrimSpace(c.Request.Body())
	if len(body) == 0 {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": "request body is required"})
		return
	}
	if err := json.Unmarshal(body, &req); err != nil {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": "invalid session patch payload"})
		return
	}
	item, err := application.UpdateSessionTitle(ctx, strings.TrimSpace(c.Param("id")), req.Title)
	if err != nil {
		var invalidSessionErr *appcore.InvalidSessionError
		if errors.As(err, &invalidSessionErr) {
			c.JSON(consts.StatusNotFound, map[string]string{"error": invalidSessionErr.Error()})
			return
		}
		c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	c.JSON(consts.StatusOK, item)
}

func handleArchiveSession(ctx context.Context, c *hertzapp.RequestContext, application *appcore.Service) {
	if application == nil {
		c.JSON(consts.StatusInternalServerError, map[string]string{"error": "application is not configured"})
		return
	}
	item, err := application.ArchiveSession(ctx, strings.TrimSpace(c.Param("id")))
	if err != nil {
		var invalidSessionErr *appcore.InvalidSessionError
		if errors.As(err, &invalidSessionErr) {
			c.JSON(consts.StatusNotFound, map[string]string{"error": invalidSessionErr.Error()})
			return
		}
		c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	c.JSON(consts.StatusOK, item)
}

func parseOptionalInt(raw string) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0
	}
	return value
}

// handleListSkills returns the currently visible skill set from the unified loader chain.
// handleListSkills 会从统一 loader 链返回当前可见的 skill 集合。
func handleListSkills(ctx context.Context, c *hertzapp.RequestContext, application *appcore.Service) {
	if application == nil {
		c.JSON(consts.StatusInternalServerError, map[string]string{"error": "application is not configured"})
		return
	}
	items, err := application.ListVisibleSkills(ctx)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	c.JSON(consts.StatusOK, map[string]any{"items": items})
}

// handleListSkillPackages returns metadata for all uploaded official skill bundles.
// handleListSkillPackages 会返回全部上传官方 skill 包的元信息。
func handleListSkillPackages(ctx context.Context, c *hertzapp.RequestContext, application *appcore.Service) {
	if application == nil {
		c.JSON(consts.StatusInternalServerError, map[string]string{"error": "application is not configured"})
		return
	}
	items, err := application.ListSkillPackages(ctx)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	c.JSON(consts.StatusOK, map[string]any{"items": items})
}

func handleGetSkillPackage(ctx context.Context, c *hertzapp.RequestContext, application *appcore.Service) {
	if application == nil {
		c.JSON(consts.StatusInternalServerError, map[string]string{"error": "application is not configured"})
		return
	}
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": "skill package id is required"})
		return
	}
	detail, ok, err := application.GetSkillPackage(ctx, id)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if !ok {
		c.JSON(consts.StatusNotFound, map[string]string{"error": "skill package not found"})
		return
	}
	c.JSON(consts.StatusOK, detail)
}

// handleListSkillPackageRevisions returns all stored revisions for one uploaded package.
// handleListSkillPackageRevisions 会返回一份 uploaded package 的全部历史版本。
func handleListSkillPackageRevisions(ctx context.Context, c *hertzapp.RequestContext, application *appcore.Service) {
	if application == nil {
		c.JSON(consts.StatusInternalServerError, map[string]string{"error": "application is not configured"})
		return
	}
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": "skill package id is required"})
		return
	}
	items, err := application.ListSkillPackageRevisions(ctx, id)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	c.JSON(consts.StatusOK, map[string]any{"items": items})
}

// handleUploadSkillPackage accepts one zip bundle and stores it as an uploaded official skill package.
// handleUploadSkillPackage 接收一个 zip 包，并把它保存为 uploaded official skill package。
func handleUploadSkillPackage(ctx context.Context, c *hertzapp.RequestContext, application *appcore.Service) {
	if application == nil {
		c.JSON(consts.StatusInternalServerError, map[string]string{"error": "application is not configured"})
		return
	}
	pkg, err := parseSkillPackageRequest(c)
	if err != nil {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	metadata, err := application.PutSkillPackage(ctx, pkg)
	if err != nil {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	c.JSON(consts.StatusCreated, metadata)
}

// handleReplaceSkillPackage replaces one uploaded official skill package by id.
// handleReplaceSkillPackage 会按 id 替换一份上传官方 skill 包。
func handleReplaceSkillPackage(ctx context.Context, c *hertzapp.RequestContext, application *appcore.Service) {
	if application == nil {
		c.JSON(consts.StatusInternalServerError, map[string]string{"error": "application is not configured"})
		return
	}
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": "skill package id is required"})
		return
	}
	pkg, err := parseSkillPackageRequest(c)
	if err != nil {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	metadata, err := application.ReplaceSkillPackage(ctx, id, pkg)
	if err != nil {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	c.JSON(consts.StatusOK, metadata)
}

// handleRollbackSkillPackage restores one historical revision as the latest active package.
// handleRollbackSkillPackage 会把一个历史版本恢复为最新生效 package。
func handleRollbackSkillPackage(ctx context.Context, c *hertzapp.RequestContext, application *appcore.Service) {
	if application == nil {
		c.JSON(consts.StatusInternalServerError, map[string]string{"error": "application is not configured"})
		return
	}
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": "skill package id is required"})
		return
	}
	var req RollbackSkillPackageRequest
	body := bytesTrimSpace(c.Request.Body())
	if len(body) == 0 {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": "request body is required"})
		return
	}
	if err := json.Unmarshal(body, &req); err != nil {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": fmt.Sprintf("invalid json body: %v", err)})
		return
	}
	if req.Revision <= 0 {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": "revision must be greater than 0"})
		return
	}
	result, err := application.RollbackSkillPackage(ctx, id, req.Revision)
	if err != nil {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	c.JSON(consts.StatusOK, result)
}

// handleUpdateSkillPackageState toggles the enabled state of one uploaded skill package.
// handleUpdateSkillPackageState 会切换一份 uploaded skill package 的启用状态。
func handleUpdateSkillPackageState(ctx context.Context, c *hertzapp.RequestContext, application *appcore.Service) {
	if application == nil {
		c.JSON(consts.StatusInternalServerError, map[string]string{"error": "application is not configured"})
		return
	}
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": "skill package id is required"})
		return
	}
	var req UpdateSkillPackageStateRequest
	body := bytesTrimSpace(c.Request.Body())
	if len(body) == 0 {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": "request body is required"})
		return
	}
	if err := json.Unmarshal(body, &req); err != nil {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": fmt.Sprintf("invalid json body: %v", err)})
		return
	}
	metadata, err := application.SetSkillPackageEnabled(ctx, id, req.Enabled)
	if err != nil {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	c.JSON(consts.StatusOK, metadata)
}

// handleDeleteSkillPackage deletes one uploaded official skill package by id.
// handleDeleteSkillPackage 会按 id 删除一份上传官方 skill 包。
func handleDeleteSkillPackage(ctx context.Context, c *hertzapp.RequestContext, application *appcore.Service) {
	if application == nil {
		c.JSON(consts.StatusInternalServerError, map[string]string{"error": "application is not configured"})
		return
	}
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": "skill package id is required"})
		return
	}
	if err := application.DeleteSkillPackage(ctx, id); err != nil {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	c.JSON(consts.StatusOK, map[string]string{
		"id":     id,
		"status": "deleted",
	})
}

// handleListModelProviders returns the current provider governance view with nested model entries.
// handleListModelProviders 会返回当前供应商治理视图以及其嵌套的模型条目。
func handleListModelProviders(ctx context.Context, c *hertzapp.RequestContext, application *appcore.Service) {
	items, err := application.ListModelProviders(ctx)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	c.JSON(consts.StatusOK, map[string]any{"items": items})
}

// handleCreateModelProvider stores one provider definition from JSON input.
// handleCreateModelProvider 会根据 JSON 输入保存一条供应商定义。
func handleCreateModelProvider(ctx context.Context, c *hertzapp.RequestContext, application *appcore.Service) {
	var req CreateModelProviderRequest
	if err := json.Unmarshal(c.Request.Body(), &req); err != nil {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": "invalid provider payload"})
		return
	}
	item, err := application.CreateModelProvider(ctx, req.toProviderUpsertInput())
	if err != nil {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	c.JSON(consts.StatusCreated, item)
}

// handleUpdateModelProvider replaces one provider definition by id.
// handleUpdateModelProvider 会按 id 整体替换一条供应商定义。
func handleUpdateModelProvider(ctx context.Context, c *hertzapp.RequestContext, application *appcore.Service) {
	var req UpsertModelProviderRequest
	if err := json.Unmarshal(c.Request.Body(), &req); err != nil {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": "invalid provider payload"})
		return
	}
	item, err := application.UpdateModelProvider(ctx, string(c.Param("id")), req.toProviderUpsertInput())
	if err != nil {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	c.JSON(consts.StatusOK, item)
}

func (req CreateModelProviderRequest) toProviderUpsertInput() model.ProviderUpsertInput {
	return model.ProviderUpsertInput{
		Name:                  strings.TrimSpace(req.Name),
		BaseURL:               strings.TrimSpace(req.BaseURL),
		Protocol:              strings.TrimSpace(req.Protocol),
		APIKey:                strings.TrimSpace(req.APIKey),
		RequestTimeoutSeconds: req.RequestTimeoutSeconds,
		Headers:               req.Headers.Clone(),
		Enabled:               valueOrDefault(req.Enabled, true),
		Models:                toProviderModelUpsertInputs(req.Models),
	}
}

func (req UpsertModelProviderRequest) toProviderUpsertInput() model.ProviderUpsertInput {
	return model.ProviderUpsertInput{
		Name:                  strings.TrimSpace(req.Name),
		BaseURL:               strings.TrimSpace(req.BaseURL),
		Protocol:              strings.TrimSpace(req.Protocol),
		APIKey:                strings.TrimSpace(req.APIKey),
		RequestTimeoutSeconds: req.RequestTimeoutSeconds,
		Headers:               req.Headers.Clone(),
		Enabled:               valueOrDefault(req.Enabled, true),
	}
}

func toProviderModelUpsertInputs(items []UpsertProviderModelRequest) []model.ProviderModelUpsertInput {
	if len(items) == 0 {
		return nil
	}
	result := make([]model.ProviderModelUpsertInput, 0, len(items))
	for _, item := range items {
		result = append(result, model.ProviderModelUpsertInput{
			ModelID:     strings.TrimSpace(item.ModelID),
			DisplayName: strings.TrimSpace(item.DisplayName),
			Enabled:     valueOrDefault(item.Enabled, true),
			IsDefault:   item.IsDefault,
			IsFallback:  item.IsFallback,
		})
	}
	return result
}

// handlePatchModelProvider applies provider-level toggles such as enabled.
// handlePatchModelProvider 会应用供应商级别的局部开关，例如 enabled。
func handlePatchModelProvider(ctx context.Context, c *hertzapp.RequestContext, application *appcore.Service) {
	var req PatchModelProviderRequest
	if err := json.Unmarshal(c.Request.Body(), &req); err != nil {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": "invalid provider patch payload"})
		return
	}
	item, err := application.PatchModelProvider(ctx, string(c.Param("id")), model.ProviderPatchInput{
		Enabled: req.Enabled,
	})
	if err != nil {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	c.JSON(consts.StatusOK, item)
}

// handleDeleteModelProvider removes one provider definition after protected models are reassigned.
// handleDeleteModelProvider 会在受保护模型完成重分配后删除一条供应商定义。
func handleDeleteModelProvider(ctx context.Context, c *hertzapp.RequestContext, application *appcore.Service) {
	if err := application.DeleteModelProvider(ctx, string(c.Param("id"))); err != nil {
		c.JSON(modelGovernanceStatus(err), map[string]string{"error": err.Error()})
		return
	}
	c.JSON(consts.StatusOK, map[string]string{"status": "deleted"})
}

// handleCreateProviderModel stores one model entry under the selected provider.
// handleCreateProviderModel 会在选中的供应商下保存一条模型子项。
func handleCreateProviderModel(ctx context.Context, c *hertzapp.RequestContext, application *appcore.Service) {
	var req UpsertProviderModelRequest
	if err := json.Unmarshal(c.Request.Body(), &req); err != nil {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": "invalid provider model payload"})
		return
	}
	item, err := application.CreateProviderModel(ctx, string(c.Param("id")), model.ProviderModelUpsertInput{
		ModelID:     strings.TrimSpace(req.ModelID),
		DisplayName: strings.TrimSpace(req.DisplayName),
		Enabled:     valueOrDefault(req.Enabled, true),
		IsDefault:   req.IsDefault,
		IsFallback:  req.IsFallback,
	})
	if err != nil {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	c.JSON(consts.StatusCreated, item)
}

// handleUpdateProviderModel replaces one provider-model row by id.
// handleUpdateProviderModel 会按 id 整体替换一条供应商模型行。
func handleUpdateProviderModel(ctx context.Context, c *hertzapp.RequestContext, application *appcore.Service) {
	var req UpsertProviderModelRequest
	if err := json.Unmarshal(c.Request.Body(), &req); err != nil {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": "invalid provider model payload"})
		return
	}
	item, err := application.UpdateProviderModel(ctx, string(c.Param("id")), string(c.Param("record_id")), model.ProviderModelUpsertInput{
		ModelID:     strings.TrimSpace(req.ModelID),
		DisplayName: strings.TrimSpace(req.DisplayName),
		Enabled:     valueOrDefault(req.Enabled, true),
		IsDefault:   req.IsDefault,
		IsFallback:  req.IsFallback,
	})
	if err != nil {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	c.JSON(consts.StatusOK, item)
}

// handlePatchProviderModel applies enabled/default/fallback toggles to one provider-model row.
// handlePatchProviderModel 会对一条供应商模型行应用 enabled、default、fallback 开关。
func handlePatchProviderModel(ctx context.Context, c *hertzapp.RequestContext, application *appcore.Service) {
	var req PatchProviderModelRequest
	if err := json.Unmarshal(c.Request.Body(), &req); err != nil {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": "invalid provider model patch payload"})
		return
	}
	item, err := application.PatchProviderModel(ctx, string(c.Param("id")), string(c.Param("record_id")), model.ProviderModelPatchInput{
		Enabled:    req.Enabled,
		IsDefault:  req.IsDefault,
		IsFallback: req.IsFallback,
	})
	if err != nil {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	c.JSON(consts.StatusOK, item)
}

// handleDeleteProviderModel removes one provider-model row after protected roles are reassigned.
// handleDeleteProviderModel 会在受保护角色被重分配后删除一条供应商模型行。
func handleDeleteProviderModel(ctx context.Context, c *hertzapp.RequestContext, application *appcore.Service) {
	if err := application.DeleteProviderModel(ctx, string(c.Param("id")), string(c.Param("record_id"))); err != nil {
		c.JSON(modelGovernanceStatus(err), map[string]string{"error": err.Error()})
		return
	}
	c.JSON(consts.StatusOK, map[string]string{"status": "deleted"})
}

func modelGovernanceStatus(err error) int {
	if errors.Is(err, model.ErrModelProviderNotFound) || errors.Is(err, model.ErrProviderModelNotFound) {
		return consts.StatusNotFound
	}
	return consts.StatusBadRequest
}

// handleTestProviderModel probes one configured provider/model pair with a live request.
// handleTestProviderModel 会通过真实请求探测一组已配置的供应商/模型是否可用。
func handleTestProviderModel(ctx context.Context, c *hertzapp.RequestContext, application *appcore.Service) {
	item, err := application.TestProviderModel(ctx, string(c.Param("record_id")))
	if err != nil {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	c.JSON(consts.StatusOK, item)
}

func parseSkillPackageRequest(c *hertzapp.RequestContext) (skills.Package, error) {
	contentType := strings.ToLower(string(c.Request.Header.ContentType()))
	if strings.Contains(contentType, "application/json") {
		var req SkillPackageFilesRequest
		if err := json.Unmarshal(c.Request.Body(), &req); err != nil {
			return skills.Package{}, fmt.Errorf("invalid skill package files payload: %w", err)
		}
		return skillPackageFromFilesRequest(req)
	}
	file, err := c.FormFile("bundle")
	if err != nil {
		return skills.Package{}, fmt.Errorf("multipart bundle file is required")
	}
	opened, err := file.Open()
	if err != nil {
		return skills.Package{}, fmt.Errorf("open uploaded bundle failed: %w", err)
	}
	defer opened.Close()
	payload, err := io.ReadAll(opened)
	if err != nil {
		return skills.Package{}, fmt.Errorf("read uploaded bundle failed: %w", err)
	}
	return parseSkillZipBundle(file.Filename, payload)
}

func skillPackageFromFilesRequest(req SkillPackageFilesRequest) (skills.Package, error) {
	if len(req.Files) == 0 {
		return skills.Package{}, fmt.Errorf("files are required")
	}
	files := make(map[string][]byte, len(req.Files))
	for path, content := range req.Files {
		path = filepath.ToSlash(strings.TrimSpace(path))
		if path == "" {
			return skills.Package{}, fmt.Errorf("file path is required")
		}
		files[path] = []byte(content)
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	return skills.Package{
		Name:    strings.TrimSpace(req.Name),
		Files:   files,
		Enabled: enabled,
	}, nil
}

// parseSkillZipBundle normalizes one uploaded zip file into the runtime package bundle form.
// parseSkillZipBundle 会把上传的 zip 文件规范化成运行时使用的 package bundle 形态。
func parseSkillZipBundle(fileName string, payload []byte) (skills.Package, error) {
	reader, err := zip.NewReader(bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		return skills.Package{}, fmt.Errorf("uploaded skill bundle must be a valid zip archive: %w", err)
	}

	files := make(map[string][]byte)
	rootPrefix := detectSingleRootPrefix(reader.File)
	for _, item := range reader.File {
		if item.FileInfo().IsDir() {
			continue
		}
		name := strings.TrimSpace(strings.TrimPrefix(item.Name, rootPrefix))
		name = strings.TrimPrefix(name, "/")
		if name == "" {
			continue
		}
		opened, err := item.Open()
		if err != nil {
			return skills.Package{}, fmt.Errorf("open zip entry %q failed: %w", item.Name, err)
		}
		body, readErr := io.ReadAll(opened)
		closeErr := opened.Close()
		if readErr != nil {
			return skills.Package{}, fmt.Errorf("read zip entry %q failed: %w", item.Name, readErr)
		}
		if closeErr != nil {
			return skills.Package{}, fmt.Errorf("close zip entry %q failed: %w", item.Name, closeErr)
		}
		files[name] = body
	}
	if _, ok := files["SKILL.md"]; !ok {
		return skills.Package{}, fmt.Errorf("uploaded skill bundle must contain SKILL.md")
	}

	trimmed := strings.TrimSpace(fileName)
	trimmed = strings.TrimSuffix(trimmed, ".zip")
	return skills.Package{
		Name:  trimmed,
		Files: files,
	}, nil
}

func detectSingleRootPrefix(items []*zip.File) string {
	root := ""
	for _, item := range items {
		name := strings.TrimSpace(item.Name)
		name = strings.TrimPrefix(name, "/")
		if name == "" {
			continue
		}
		first, _, _ := strings.Cut(name, "/")
		if first == "" || !strings.Contains(name, "/") {
			return ""
		}
		if root == "" {
			root = first
			continue
		}
		if root != first {
			return ""
		}
	}
	if root == "" {
		return ""
	}
	return root + "/"
}

func valueOrDefault(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func processAgentEvent(ctx context.Context, stream *sse.Stream, event *adk.AgentEvent, assistantOutput *strings.Builder, sessionID string, tracker *progressStepTracker) error {
	if event.Err != nil {
		if err := tracker.failActiveStep(event.Err.Error()); err != nil {
			return err
		}
		return sendSSEEvent(stream, StreamEvent{
			Type:      "error",
			RequestID: requestIDFromContext(ctx),
			SessionID: sessionID,
			AgentName: event.AgentName,
			RunPath:   formatRunPath(event.RunPath),
			Error:     event.Err.Error(),
		})
	}

	if event.Output != nil && event.Output.MessageOutput != nil {
		if err := handleMessageOutput(ctx, stream, event, assistantOutput, sessionID, tracker); err != nil {
			return err
		}
	}

	if event.Action != nil {
		if err := handleAction(ctx, stream, event, sessionID); err != nil {
			return err
		}
	}

	return nil
}

func handleMessageOutput(ctx context.Context, stream *sse.Stream, event *adk.AgentEvent, assistantOutput *strings.Builder, sessionID string, tracker *progressStepTracker) error {
	msgOutput := event.Output.MessageOutput

	if msg := msgOutput.Message; msg != nil {
		return handleRegularMessage(ctx, stream, event, msg, assistantOutput, sessionID, tracker)
	}

	if msgStream := msgOutput.MessageStream; msgStream != nil {
		return handleStreamingMessage(ctx, stream, event, msgStream, assistantOutput, sessionID, tracker)
	}

	return nil
}

func handleRegularMessage(ctx context.Context, stream *sse.Stream, event *adk.AgentEvent, msg *schema.Message, assistantOutput *strings.Builder, sessionID string, tracker *progressStepTracker) error {
	if lifecycle, ok := buildToolLifecycleEvent(ctx, event, msg, sessionID); ok {
		if lifecycle.Type == "tool_call_started" {
			if err := tracker.emitToolStarted(lifecycle.ToolCallID, lifecycle.ToolName); err != nil {
				return err
			}
		}
		if lifecycle.Type == "tool_call_finished" {
			if err := tracker.emitToolFinished(lifecycle.ToolCallID, lifecycle.ToolName, lifecycle.Status, lifecycle.Error); err != nil {
				return err
			}
		}
		return sendSSEEvent(stream, lifecycle)
	}

	eventType := "message"
	if msg.Role == schema.Tool {
		eventType = "tool_result"
	}

	payload := StreamEvent{
		Type:       eventType,
		RequestID:  requestIDFromContext(ctx),
		SessionID:  sessionID,
		AgentName:  event.AgentName,
		RunPath:    formatRunPath(event.RunPath),
		Content:    msg.Content,
		ToolCallID: msg.ToolCallID,
		ToolName:   msg.ToolName,
	}
	if len(msg.ToolCalls) > 0 {
		payload.ToolCalls = msg.ToolCalls
	}
	if eventType == "message" && msg.Content != "" {
		if err := tracker.startResponse(); err != nil {
			return err
		}
		assistantOutput.WriteString(msg.Content)
	}

	return sendSSEEvent(stream, payload)
}

func buildToolLifecycleEvent(ctx context.Context, event *adk.AgentEvent, msg *schema.Message, sessionID string) (StreamEvent, bool) {
	if msg.Extra == nil {
		return StreamEvent{}, false
	}

	eventType, _ := msg.Extra["event_type"].(string)
	if eventType != "tool_call_started" && eventType != "tool_call_finished" {
		return StreamEvent{}, false
	}

	payload := StreamEvent{
		Type:      eventType,
		RequestID: requestIDFromContext(ctx),
		SessionID: sessionID,
		AgentName: event.AgentName,
		RunPath:   formatRunPath(event.RunPath),
	}

	if v, ok := msg.Extra["tool_call_id"].(string); ok {
		payload.ToolCallID = v
	}
	if v, ok := msg.Extra["tool_name"].(string); ok {
		payload.ToolName = v
	}
	if v, ok := msg.Extra["tool_arguments"].(string); ok {
		payload.ToolArguments = v
	}
	if v, ok := msg.Extra["started_at"].(string); ok {
		payload.StartedAt = v
	}
	if v, ok := msg.Extra["status"].(string); ok {
		payload.Status = v
	}
	if v, ok := msg.Extra["error"].(string); ok {
		payload.Error = v
	}
	switch v := msg.Extra["duration_ms"].(type) {
	case int64:
		payload.DurationMS = v
	case int:
		payload.DurationMS = int64(v)
	case float64:
		payload.DurationMS = int64(v)
	}

	return payload, true
}

func handleStreamingMessage(ctx context.Context, stream *sse.Stream, event *adk.AgentEvent, msgStream *schema.StreamReader[*schema.Message], assistantOutput *strings.Builder, sessionID string, tracker *progressStepTracker) error {
	toolCallsMap := make(map[int][]*schema.Message)

	for {
		chunk, err := msgStream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			if failErr := tracker.failActiveStep(fmt.Sprintf("stream error: %v", err)); failErr != nil {
				return failErr
			}
			return sendSSEEvent(stream, StreamEvent{
				Type:      "error",
				RequestID: requestIDFromContext(ctx),
				SessionID: sessionID,
				AgentName: event.AgentName,
				RunPath:   formatRunPath(event.RunPath),
				Error:     fmt.Sprintf("stream error: %v", err),
			})
		}

		if chunk.Content != "" {
			eventType := "stream_chunk"
			if chunk.Role == schema.Tool {
				eventType = "tool_result_chunk"
			}
			if eventType == "stream_chunk" {
				if err := tracker.startResponse(); err != nil {
					return err
				}
			}

			if err := sendSSEEvent(stream, StreamEvent{
				Type:       eventType,
				RequestID:  requestIDFromContext(ctx),
				SessionID:  sessionID,
				AgentName:  event.AgentName,
				RunPath:    formatRunPath(event.RunPath),
				Content:    chunk.Content,
				ToolCallID: chunk.ToolCallID,
				ToolName:   chunk.ToolName,
			}); err != nil {
				return err
			}
			if eventType == "stream_chunk" {
				assistantOutput.WriteString(chunk.Content)
			}
		}

		if len(chunk.ToolCalls) == 0 {
			continue
		}

		for _, tc := range chunk.ToolCalls {
			if tc.Index == nil {
				continue
			}
			toolCallsMap[*tc.Index] = append(toolCallsMap[*tc.Index], &schema.Message{
				Role: chunk.Role,
				ToolCalls: []schema.ToolCall{{
					ID:    tc.ID,
					Type:  tc.Type,
					Index: tc.Index,
					Function: schema.FunctionCall{
						Name:      tc.Function.Name,
						Arguments: tc.Function.Arguments,
					},
				}},
			})
		}
	}

	indices := make([]int, 0, len(toolCallsMap))
	for index := range toolCallsMap {
		indices = append(indices, index)
	}
	sort.Ints(indices)

	for _, index := range indices {
		msgs := toolCallsMap[index]
		merged, err := schema.ConcatMessages(msgs)
		if err != nil {
			return err
		}
		if err := sendSSEEvent(stream, StreamEvent{
			Type:      "tool_calls",
			RequestID: requestIDFromContext(ctx),
			SessionID: sessionID,
			AgentName: event.AgentName,
			RunPath:   formatRunPath(event.RunPath),
			ToolCalls: merged.ToolCalls,
		}); err != nil {
			return err
		}
	}

	return nil
}

func handleAction(ctx context.Context, stream *sse.Stream, event *adk.AgentEvent, sessionID string) error {
	action := event.Action
	if action.TransferToAgent != nil {
		return sendSSEEvent(stream, StreamEvent{
			Type:       "action",
			RequestID:  requestIDFromContext(ctx),
			SessionID:  sessionID,
			AgentName:  event.AgentName,
			RunPath:    formatRunPath(event.RunPath),
			ActionType: "transfer",
			Content:    fmt.Sprintf("transferring to %s", action.TransferToAgent.DestAgentName),
		})
	}
	if action.Interrupted != nil {
		return sendSSEEvent(stream, StreamEvent{
			Type:       "action",
			RequestID:  requestIDFromContext(ctx),
			SessionID:  sessionID,
			AgentName:  event.AgentName,
			RunPath:    formatRunPath(event.RunPath),
			ActionType: "interrupted",
			Content:    "agent execution interrupted",
		})
	}
	if action.Exit {
		return sendSSEEvent(stream, StreamEvent{
			Type:       "action",
			RequestID:  requestIDFromContext(ctx),
			SessionID:  sessionID,
			AgentName:  event.AgentName,
			RunPath:    formatRunPath(event.RunPath),
			ActionType: "exit",
			Content:    "agent execution completed",
		})
	}
	return nil
}

func sendSSEEvent(stream *sse.Stream, payload StreamEvent) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return stream.Publish(&sse.Event{Data: data})
}

// progressStepTracker keeps the user-visible step flow consistent across one SSE request.
// progressStepTracker 用于在一次 SSE 请求内维持对用户可见的步骤流一致性。
type progressStepTracker struct {
	requestID      string
	sessionID      string
	stream         *sse.Stream
	analysisActive bool
	responseActive bool
	history        []stepFlowStepSnapshot
}

// newProgressStepTracker creates the request-scoped progress tracker used by chat stream delivery.
// newProgressStepTracker 创建 chat stream 交付路径使用的请求级步骤跟踪器。
func newProgressStepTracker(requestID, sessionID string, stream *sse.Stream) *progressStepTracker {
	return &progressStepTracker{
		requestID: requestID,
		sessionID: sessionID,
		stream:    stream,
	}
}

func (t *progressStepTracker) emitContextReady() error {
	if t == nil {
		return nil
	}
	return t.emit(progressStatusCompleted, ProgressStep{
		StepID:        string(runtime.StageContextAssembly),
		Category:      progressCategoryContext,
		Label:         "Reading current context",
		Summary:       "Athena prepared the current session and runtime context.",
		DisplayToUser: true,
	}, nil)
}

func (t *progressStepTracker) startAnalysis() error {
	if t == nil || t.analysisActive {
		return nil
	}
	t.analysisActive = true
	return t.emit(progressStatusRunning, ProgressStep{
		StepID:        string(runtime.StageCapabilityResolution),
		Category:      progressCategoryAnalysis,
		Label:         "Planning the next action",
		Summary:       "Athena is deciding how to answer this request.",
		DisplayToUser: true,
	}, nil)
}

func (t *progressStepTracker) completeAnalysis(summary string) error {
	if t == nil || !t.analysisActive {
		return nil
	}
	t.analysisActive = false
	if strings.TrimSpace(summary) == "" {
		summary = "Athena finished planning the next action."
	}
	return t.emit(progressStatusCompleted, ProgressStep{
		StepID:        string(runtime.StageCapabilityResolution),
		Category:      progressCategoryAnalysis,
		Label:         "Planning the next action",
		Summary:       summary,
		DisplayToUser: true,
	}, nil)
}

func (t *progressStepTracker) waitAnalysis(summary string) error {
	if t == nil {
		return nil
	}
	t.analysisActive = false
	if strings.TrimSpace(summary) == "" {
		summary = "Athena needs additional information before it can continue."
	}
	return t.emit(progressStatusWaiting, ProgressStep{
		StepID:        string(runtime.StageCapabilityResolution),
		Category:      progressCategoryWaiting,
		Label:         "Waiting for more information",
		Summary:       summary,
		DisplayToUser: true,
	}, nil)
}

func (t *progressStepTracker) failAnalysis(summary string) error {
	if t == nil {
		return nil
	}
	t.analysisActive = false
	if strings.TrimSpace(summary) == "" {
		summary = "Athena could not finish the current request."
	}
	return t.emit(progressStatusFailed, ProgressStep{
		StepID:        string(runtime.StageCapabilityResolution),
		Category:      progressCategoryError,
		Label:         "Request failed",
		Summary:       summary,
		DisplayToUser: true,
	}, nil)
}

func (t *progressStepTracker) startResponse() error {
	if t == nil || t.responseActive {
		return nil
	}
	if err := t.completeAnalysis("Athena finished planning and is preparing the final result."); err != nil {
		return err
	}
	t.responseActive = true
	return t.emit(progressStatusRunning, ProgressStep{
		StepID:        string(runtime.StageTurnProcessing),
		Category:      progressCategoryResponse,
		Label:         "Organizing the final result",
		Summary:       "Athena is drafting the final answer and packaging the result.",
		DisplayToUser: true,
	}, nil)
}

func (t *progressStepTracker) completeResponse(summary string) error {
	if t == nil || !t.responseActive {
		return nil
	}
	t.responseActive = false
	if strings.TrimSpace(summary) == "" {
		summary = "Athena finished the final answer and result packaging."
	}
	return t.emit(progressStatusCompleted, ProgressStep{
		StepID:        string(runtime.StageTurnProcessing),
		Category:      progressCategoryResponse,
		Label:         "Organizing the final result",
		Summary:       summary,
		DisplayToUser: true,
	}, nil)
}

func (t *progressStepTracker) failActiveStep(summary string) error {
	if t == nil {
		return nil
	}
	if t.responseActive {
		t.responseActive = false
		if strings.TrimSpace(summary) == "" {
			summary = "Athena could not finish the final result."
		}
		return t.emit(progressStatusFailed, ProgressStep{
			StepID:        string(runtime.StageTurnProcessing),
			Category:      progressCategoryError,
			Label:         "Request failed",
			Summary:       summary,
			DisplayToUser: true,
		}, nil)
	}
	if t.analysisActive {
		return t.failAnalysis(summary)
	}
	return t.emit(progressStatusFailed, ProgressStep{
		StepID:        "request_failed",
		Category:      progressCategoryError,
		Label:         "Request failed",
		Summary:       defaultProgressFailureSummary(summary),
		DisplayToUser: true,
	}, nil)
}

func (t *progressStepTracker) emitToolStarted(toolCallID, toolName string) error {
	if t == nil {
		return nil
	}
	if err := t.completeAnalysis("Athena selected one supporting capability and is starting it."); err != nil {
		return err
	}
	step := buildToolProgressStep(toolCallID, toolName)
	return t.emit(progressStatusRunning, step, nil)
}

func (t *progressStepTracker) emitToolFinished(toolCallID, toolName, status, errMsg string) error {
	if t == nil {
		return nil
	}
	step := buildToolProgressStep(toolCallID, toolName)
	if strings.EqualFold(strings.TrimSpace(status), "error") {
		step.Category = progressCategoryError
		if strings.TrimSpace(errMsg) != "" {
			step.Summary = "Athena could not finish one supporting capability."
		} else {
			step.Summary = "Athena encountered an error while running one supporting capability."
		}
		return t.emit(progressStatusFailed, step, nil)
	}
	step.Summary = "Athena finished one supporting capability."
	return t.emit(progressStatusCompleted, step, nil)
}

func (t *progressStepTracker) emit(status string, step ProgressStep, detail map[string]any) error {
	if t == nil {
		return nil
	}
	t.record(status, step)
	return sendProgressStepEvent(t.stream, t.requestID, t.sessionID, status, step, detail)
}

func (t *progressStepTracker) record(status string, step ProgressStep) {
	if t == nil || strings.TrimSpace(step.StepID) == "" {
		return
	}
	t.history = append(t.history, stepFlowStepSnapshot{
		StepID: strings.TrimSpace(step.StepID),
		Status: strings.TrimSpace(status),
	})
}

func (t *progressStepTracker) stepFlowSummary(finalStatus, summary string) StepFlowSummary {
	if t == nil {
		return buildStepFlowSummary(finalStatus, summary, nil)
	}
	return buildStepFlowSummary(finalStatus, summary, t.history)
}

func sendProgressStepEvent(stream *sse.Stream, requestID, sessionID, status string, step ProgressStep, detail map[string]any) error {
	if stream == nil || strings.TrimSpace(step.StepID) == "" {
		return nil
	}
	if strings.TrimSpace(status) == "" {
		status = progressStatusRunning
	}
	if !step.DisplayToUser {
		step.DisplayToUser = true
	}
	return sendSSEEvent(stream, StreamEvent{
		Type:         progressStepType,
		RequestID:    requestID,
		SessionID:    sessionID,
		Status:       status,
		ProgressStep: &step,
		Detail:       detail,
	})
}

func buildToolProgressStep(toolCallID, toolName string) ProgressStep {
	label, summary := classifyToolProgress(toolName)
	stepID := strings.TrimSpace(toolCallID)
	if stepID == "" {
		stepID = strings.TrimSpace(toolName)
	}
	if stepID == "" {
		stepID = "tool"
	}
	return ProgressStep{
		StepID:        "tool:" + stepID,
		Category:      progressCategoryTool,
		Label:         label,
		Summary:       summary,
		DisplayToUser: true,
	}
}

func classifyToolProgress(toolName string) (string, string) {
	normalized := strings.ToLower(strings.TrimSpace(toolName))
	switch {
	case containsAny(normalized, "lookup", "search", "find", "query", "list", "read", "fetch", "get", "inspect"):
		return "Querying supporting data", "Athena is gathering the data it needs for the next step."
	case containsAny(normalized, "write", "save", "create", "update", "patch", "delete"):
		return "Preparing a supporting change", "Athena is using one supporting capability to prepare the requested change."
	default:
		return "Running a supporting capability", "Athena is using one internal capability to continue the current task."
	}
}

func containsAny(input string, patterns ...string) bool {
	for _, pattern := range patterns {
		if strings.Contains(input, pattern) {
			return true
		}
	}
	return false
}

func defaultProgressFailureSummary(summary string) string {
	if strings.TrimSpace(summary) != "" {
		return summary
	}
	return "Athena could not finish the current request."
}

func buildStepFlowSummary(finalStatus, summary string, history []stepFlowStepSnapshot) StepFlowSummary {
	completed := make([]string, 0, len(history))
	seenCompleted := make(map[string]struct{}, len(history))
	currentStage := ""
	for _, item := range history {
		stepID := strings.TrimSpace(item.StepID)
		if stepID == "" {
			continue
		}
		currentStage = stepID
		if strings.TrimSpace(item.Status) != progressStatusCompleted {
			continue
		}
		if _, exists := seenCompleted[stepID]; exists {
			continue
		}
		seenCompleted[stepID] = struct{}{}
		completed = append(completed, stepID)
	}
	return StepFlowSummary{
		VisibleEventTypes: []string{
			progressStepType,
			"done",
		},
		InternalEventTypes: []string{
			"tool_calls",
			"tool_call_started",
			"tool_call_finished",
			"tool_result",
			"stream_chunk",
		},
		TerminalEvent:   "done",
		FinalStatus:     strings.TrimSpace(finalStatus),
		AutoFold:        shouldAutoFoldStepFlow(finalStatus),
		CurrentStage:    currentStage,
		CompletedStages: completed,
		Summary:         strings.TrimSpace(summary),
	}
}

func shouldAutoFoldStepFlow(status string) bool {
	return strings.TrimSpace(status) == "completed"
}

func withStepFlowDetail(detail map[string]any, summary StepFlowSummary) map[string]any {
	if len(summary.VisibleEventTypes) == 0 && strings.TrimSpace(summary.TerminalEvent) == "" {
		return detail
	}
	if detail == nil {
		detail = make(map[string]any, 1)
	}
	detail["step_flow"] = summary
	return detail
}

func buildWaitingProgressStep(stepID, label, summary string) ProgressStep {
	if strings.TrimSpace(stepID) == "" {
		stepID = string(runtime.ActionTypeInformationRequest)
	}
	if strings.TrimSpace(label) == "" {
		label = "Waiting for more information"
	}
	if strings.TrimSpace(summary) == "" {
		summary = "Athena needs additional information before it can continue."
	}
	return ProgressStep{
		StepID:        stepID,
		Category:      progressCategoryWaiting,
		Label:         label,
		Summary:       summary,
		DisplayToUser: true,
	}
}

func buildErrorProgressStep(stepID, summary string) ProgressStep {
	if strings.TrimSpace(stepID) == "" {
		stepID = "request_failed"
	}
	return ProgressStep{
		StepID:        stepID,
		Category:      progressCategoryError,
		Label:         "Request failed",
		Summary:       defaultProgressFailureSummary(summary),
		DisplayToUser: true,
	}
}

func emitStructuredCompletionEvents(ctx context.Context, stream *sse.Stream, requestID, sessionID string, prepared *runtime.PreparedExecution, req ChatStreamRequest, rawOutput string, sharedRootDir string, currentSession *session.Session) error {
	converted := ChatRespondRequest{
		TaskType:              req.TaskType,
		TaskSubtype:           req.TaskSubtype,
		Scene:                 req.Scene,
		Query:                 req.Query,
		SessionID:             req.SessionID,
		MainSessionID:         req.MainSessionID,
		WorkspaceID:           req.WorkspaceID,
		AppInstanceID:         req.AppInstanceID,
		AppSessionID:          req.AppSessionID,
		IntegrationInstanceID: req.IntegrationInstanceID,
		WorkflowRunID:         req.WorkflowRunID,
		StepID:                req.StepID,
		TriggerType:           req.TriggerType,
		AutomationTaskID:      req.AutomationTaskID,
		UserLanguage:          req.UserLanguage,
		DesiredOutputMode:     req.DesiredOutputMode,
		GlobalContext:         cloneAnyMap(req.GlobalContext),
		AppContext:            cloneAnyMap(req.AppContext),
		InputPayload:          cloneAnyMap(req.InputPayload),
	}
	result, _, _, _ := resolveStructuredResult(ctx, prepared, rawOutput, converted, requestID, sessionID, false, sharedRootDir, currentSession)
	if result == nil {
		return nil
	}

	if err := emitInteractionProgressEvents(stream, requestID, sessionID, result); err != nil {
		return err
	}

	if plan, ok := result.StructuredResult["workflow_plan"]; ok {
		if err := sendSSEEvent(stream, StreamEvent{
			Type:      "workflow_plan",
			RequestID: requestID,
			SessionID: sessionID,
			Status:    "completed",
			Detail:    map[string]any{"workflow_plan": plan},
		}); err != nil {
			return err
		}
	}

	if effectiveTaskType(converted) == "inspection_task" {
		if err := sendSSEEvent(stream, StreamEvent{
			Type:      "inspection_progress",
			RequestID: requestID,
			SessionID: sessionID,
			Status:    "completed",
			Detail: map[string]any{
				"phase":    "completed",
				"progress": 100,
				"summary":  result.MainAnswer,
			},
		}); err != nil {
			return err
		}
	}

	for _, card := range result.ContentCards {
		if err := sendSSEEvent(stream, StreamEvent{
			Type:      "card_created",
			RequestID: requestID,
			SessionID: sessionID,
			Status:    "completed",
			Detail: map[string]any{
				"card":      card,
				"card_type": card.CardType,
				"title":     card.Title,
				"summary":   card.Summary,
				"source":    card.Source,
			},
		}); err != nil {
			return err
		}
	}

	if result.RightPanelView != nil {
		if err := sendSSEEvent(stream, StreamEvent{
			Type:      "right_panel_view",
			RequestID: requestID,
			SessionID: sessionID,
			Status:    "completed",
			Detail:    map[string]any{"right_panel_view": result.RightPanelView},
		}); err != nil {
			return err
		}
	}

	if len(result.NextQuestions) > 0 {
		if err := sendSSEEvent(stream, StreamEvent{
			Type:      "next_questions",
			RequestID: requestID,
			SessionID: sessionID,
			Status:    "completed",
			Detail:    map[string]any{"next_questions": result.NextQuestions},
		}); err != nil {
			return err
		}
	}

	return sendSSEEvent(stream, StreamEvent{
		Type:      "completed",
		RequestID: requestID,
		SessionID: sessionID,
		Status:    "completed",
		Detail: map[string]any{
			"main_answer":       result.MainAnswer,
			"result_summary":    result.ResultSummary,
			"structured_result": result.StructuredResult,
		},
	})
}

func emitInteractionProgressEvents(stream *sse.Stream, requestID, sessionID string, result *structuredChatResult) error {
	if result == nil || result.StructuredResult == nil {
		return nil
	}
	mode, _ := result.StructuredResult["interaction_mode"].(*runtime.InteractionModeResult)
	progress, _ := result.StructuredResult["interaction_progress"].(*runtime.InteractionProgress)
	if mode == nil {
		return nil
	}
	switch mode.Mode {
	case "automation_draft":
		steps := platformautomation.BuildPlanningProgressSteps(intentMode(mode.Mode), progressSummary(progress, "Athena finished building a draft plan that the user can review before creation."))
		for _, step := range steps {
			if err := sendProgressStepEvent(stream, requestID, sessionID, progressStatusCompleted, progressDescriptorToStep(step), nil); err != nil {
				return err
			}
		}
	case "choice_required":
		steps := platformautomation.BuildPlanningProgressSteps(intentMode(mode.Mode), progressSummary(progress, "Athena needs the user to choose whether to continue normal discussion or generate an automation draft."))
		for _, step := range steps {
			if err := sendProgressStepEvent(stream, requestID, sessionID, progressStatusCompleted, progressDescriptorToStep(step), nil); err != nil {
				return err
			}
		}
	}
	return nil
}

func progressSummary(progress *runtime.InteractionProgress, fallback string) string {
	if progress == nil || strings.TrimSpace(progress.Summary) == "" {
		return fallback
	}
	return strings.TrimSpace(progress.Summary)
}

func intentMode(mode string) intentpkg.InteractionMode {
	return intentpkg.InteractionMode(strings.TrimSpace(mode))
}

func progressDescriptorToStep(step platformautomation.PlanningProgressStep) ProgressStep {
	return ProgressStep{
		StepID:        strings.TrimSpace(step.StepID),
		Category:      progressCategoryPlanning,
		Label:         strings.TrimSpace(step.Label),
		Summary:       strings.TrimSpace(step.Summary),
		DisplayToUser: true,
	}
}

func writeCompletedAndFinish(stream *sse.Stream, requestID, sessionID string, prepared *runtime.PreparedExecution, detail map[string]any) error {
	return sendSSEEvent(stream, StreamEvent{
		Type:             "done",
		RequestID:        requestID,
		SessionID:        sessionID,
		Status:           "completed",
		StructuredOutput: structuredOutputOrNil(prepared),
		Detail:           detail,
	})
}

func writeInitialActionAndFinish(stream *sse.Stream, requestID, sessionID string, prepared *runtime.PreparedExecution, tracker *progressStepTracker) error {
	if prepared == nil || prepared.Initial == nil || prepared.Initial.Action == nil {
		return nil
	}

	stepHistory := make([]stepFlowStepSnapshot, 0, 1)
	switch prepared.Initial.Action.Type {
	case runtime.ActionTypeInformationRequest:
		if tracker != nil {
			if err := tracker.waitAnalysis(strings.TrimSpace(prepared.Initial.Action.Message)); err != nil {
				return err
			}
		} else {
			waitingStep := buildWaitingProgressStep(string(runtime.StageCapabilityResolution), "Waiting for more information", strings.TrimSpace(prepared.Initial.Action.Message))
			if err := sendProgressStepEvent(stream, requestID, sessionID, progressStatusWaiting, waitingStep, nil); err != nil {
				return err
			}
			stepHistory = append(stepHistory, stepFlowStepSnapshot{StepID: waitingStep.StepID, Status: progressStatusWaiting})
		}
	case runtime.ActionTypePendingHuman:
		if tracker != nil {
			tracker.analysisActive = false
			waitingStep := buildWaitingProgressStep(string(runtime.ActionTypePendingHuman), "Waiting for human handoff", "Athena paused this request so a human or external system can continue it.")
			if err := sendProgressStepEvent(stream, requestID, sessionID, progressStatusWaiting, waitingStep, nil); err != nil {
				return err
			}
			tracker.record(progressStatusWaiting, waitingStep)
		} else {
			waitingStep := buildWaitingProgressStep(string(runtime.ActionTypePendingHuman), "Waiting for human handoff", "Athena paused this request so a human or external system can continue it.")
			if err := sendProgressStepEvent(stream, requestID, sessionID, progressStatusWaiting, waitingStep, nil); err != nil {
				return err
			}
			stepHistory = append(stepHistory, stepFlowStepSnapshot{StepID: waitingStep.StepID, Status: progressStatusWaiting})
		}
	}

	if err := sendSSEEvent(stream, StreamEvent{
		Type:             "action",
		RequestID:        requestID,
		SessionID:        sessionID,
		ActionType:       string(prepared.Initial.Action.Type),
		Action:           prepared.Initial.Action,
		WaitState:        prepared.Initial.WaitState,
		StructuredOutput: structuredOutputOrNil(prepared),
		Status:           string(prepared.InitialStatus),
		Detail: map[string]any{
			"primary_skill": prepared.Spec.Skill.PrimarySkill,
		},
	}); err != nil {
		return err
	}

	return sendSSEEvent(stream, StreamEvent{
		Type:             "done",
		RequestID:        requestID,
		SessionID:        sessionID,
		Status:           string(prepared.InitialStatus),
		StructuredOutput: structuredOutputOrNil(prepared),
		WaitState:        prepared.TimeoutWait,
		Detail: withStepFlowDetail(nil, func() StepFlowSummary {
			if tracker != nil {
				return tracker.stepFlowSummary(string(prepared.InitialStatus), "Athena paused this request and is waiting for the next explicit user or system action.")
			}
			return buildStepFlowSummary(string(prepared.InitialStatus), "Athena paused this request and is waiting for the next explicit user or system action.", stepHistory)
		}()),
	})
}

func requestStartedDetail(result *appcore.FastPathResult, disabled bool, dequeued *session.DeferredMessage) map[string]any {
	detail := map[string]any{}
	if result == nil || !result.Matched {
		if disabled {
			detail["fast_path"] = map[string]any{
				"disabled": true,
			}
		}
	} else {
		detail["fast_path"] = map[string]any{
			"matched": true,
			"name":    result.Name,
			"reason":  result.Reason,
		}
	}

	if dequeued != nil {
		detail["deferred_queue"] = map[string]any{
			"auto_consumed": true,
			"received_at":   dequeued.ReceivedAt,
		}
	}
	if len(detail) == 0 {
		return nil
	}
	return detail
}

func writePendingWaitAndFinish(stream *sse.Stream, requestID, sessionID string, pending *session.PendingState, queued *session.DeferredMessage, dropped *session.DeferredMessage) error {
	if pending == nil {
		return nil
	}

	waitState := &runtime.WaitState{
		Stage:        runtime.RuntimeStage(pending.Stage),
		StartedAt:    pending.TimeoutAt.Add(-pending.TimeoutAfter),
		TimeoutAt:    pending.TimeoutAt,
		TimeoutAfter: pending.TimeoutAfter,
		ResumeToken:  pending.ResumeToken,
	}
	action := &runtime.Action{
		Type:    runtime.ActionTypeInformationRequest,
		Code:    string(runtime.ActionTypeInformationRequest),
		Message: "the current analysis is waiting for supplemental information",
		Target:  runtime.SupplementTargetClient,
		Schema: &runtime.ActionSchema{
			Input: map[string]runtime.ActionSchemaField{
				"supplement.data": {
					Type: "object",
				},
				"supplement_outcome": {
					Type:     "string",
					Required: true,
					Enum: []string{
						string(runtime.SupplementOutcomeProvided),
						string(runtime.SupplementOutcomeUnableToProvide),
						string(runtime.SupplementOutcomeTimeoutExpired),
						string(runtime.SupplementOutcomeAbandonAndContinue),
						string(runtime.SupplementOutcomePendingHuman),
					},
				},
				"resume_token": {
					Type:     "string",
					Required: true,
				},
			},
		},
		InformationRequest: &runtime.InformationRequestAction{
			AllowDegrade:    true,
			SuggestedAction: "provide the requested supplemental data together with the matching resume_token, or explicitly send supplement_outcome if the data cannot be provided",
			Target:          runtime.SupplementTargetClient,
			WaitPolicy: runtime.WaitTimeoutPolicy{
				TimeoutAfter: pending.TimeoutAfter,
			},
		},
		Payload: map[string]any{
			"missing_fields": pending.MissingFields,
		},
		TimeoutPolicy: &runtime.WaitTimeoutPolicy{
			TimeoutAfter: pending.TimeoutAfter,
		},
		ExpectedResult: &runtime.ActionExpectedResult{
			ResumeTokenRequired: true,
			AllowedOutcomes: []runtime.SupplementOutcome{
				runtime.SupplementOutcomeProvided,
				runtime.SupplementOutcomeUnableToProvide,
				runtime.SupplementOutcomeTimeoutExpired,
				runtime.SupplementOutcomeAbandonAndContinue,
				runtime.SupplementOutcomePendingHuman,
			},
		},
	}
	for _, field := range pending.MissingFields {
		action.InformationRequest.Missing = append(action.InformationRequest.Missing, runtime.MissingInformationItem{
			Field:    field,
			Reason:   "the current analysis stage is paused until this missing information is resolved",
			Impact:   "the pending gap cannot be closed and the analysis chain cannot continue without an explicit resume outcome",
			Required: true,
		})
	}

	detail := map[string]any{
		"accepted":                 false,
		"blocked_by_pending_wait":  true,
		"resume_required":          true,
		"pending_action_type":      pending.ActionType,
		"pending_status":           pending.Status,
		"received_but_not_applied": true,
		"queued_for_follow_up":     queued != nil,
	}
	if dropped != nil {
		detail["queue_overflow"] = map[string]any{
			"dropped_oldest": true,
			"query":          dropped.Query,
			"received_at":    dropped.ReceivedAt,
		}
	}

	waitingStep := buildWaitingProgressStep(string(runtime.StageCapabilityResolution), "Waiting for more information", action.Message)
	if err := sendProgressStepEvent(stream, requestID, sessionID, progressStatusWaiting, waitingStep, nil); err != nil {
		return err
	}

	if err := sendSSEEvent(stream, StreamEvent{
		Type:       "action",
		RequestID:  requestID,
		SessionID:  sessionID,
		ActionType: string(action.Type),
		Action:     action,
		WaitState:  waitState,
		Status:     string(runtime.RequestStatusWaitingForInformation),
		Detail:     detail,
	}); err != nil {
		return err
	}

	return sendSSEEvent(stream, StreamEvent{
		Type:      "done",
		RequestID: requestID,
		SessionID: sessionID,
		Status:    string(runtime.RequestStatusWaitingForInformation),
		WaitState: waitState,
		Detail: withStepFlowDetail(detail, buildStepFlowSummary(string(runtime.RequestStatusWaitingForInformation), "Athena is waiting for more information before it can continue this request.", []stepFlowStepSnapshot{{
			StepID: waitingStep.StepID,
			Status: progressStatusWaiting,
		}})),
	})
}

func writeGapClosedNotification(stream *sse.Stream, requestID, sessionID string, gapClosed *runtime.GapClosedAction) error {
	if gapClosed == nil {
		return nil
	}
	return sendSSEEvent(stream, StreamEvent{
		Type:      "notification",
		RequestID: requestID,
		SessionID: sessionID,
		Status:    "running",
		Notification: &runtime.Notification{
			Code:        "gap_closed",
			Message:     "the pending gap has been closed",
			SessionID:   sessionID,
			ResumeToken: gapClosed.ResumeToken,
			NextStep:    gapClosed.NextStep,
			ClientHint:  "inspect next_step and do not reuse the closed resume_token",
			Detail: map[string]any{
				"close_reason":  gapClosed.CloseReason,
				"token_invalid": gapClosed.TokenInvalid,
			},
		},
	})
}

func writeInvalidResumeTokenAndFinish(stream *sse.Stream, requestID, sessionID string, invalid *appcore.InvalidResumeTokenError) error {
	if invalid == nil {
		return nil
	}
	errorDetail := &runtime.ProtocolError{
		Code:         string(runtime.RequestStatusInvalidResumeToken),
		Reason:       invalid.Reason,
		Retryable:    false,
		ClientAction: invalidResumeTokenClientAction(invalid.Reason),
		Detail: map[string]any{
			"resume_token": invalid.ResumeToken,
			"session_id":   sessionID,
		},
	}
	errorStep := buildErrorProgressStep(string(runtime.RequestStatusInvalidResumeToken), "Athena could not resume the previous waiting step because the resume token is no longer valid.")
	if err := sendProgressStepEvent(stream, requestID, sessionID, progressStatusFailed, errorStep, nil); err != nil {
		return err
	}
	if err := sendSSEEvent(stream, StreamEvent{
		Type:        "error",
		RequestID:   requestID,
		SessionID:   sessionID,
		Status:      string(runtime.RequestStatusInvalidResumeToken),
		Error:       invalid.Error(),
		ErrorDetail: errorDetail,
		Detail:      errorDetail.Detail,
	}); err != nil {
		return err
	}
	return sendSSEEvent(stream, StreamEvent{
		Type:      "done",
		RequestID: requestID,
		SessionID: sessionID,
		Status:    string(runtime.RequestStatusInvalidResumeToken),
		Detail: withStepFlowDetail(map[string]any{
			"resume_token":  invalid.ResumeToken,
			"reason":        invalid.Reason,
			"client_action": invalidResumeTokenClientAction(invalid.Reason),
		}, buildStepFlowSummary(string(runtime.RequestStatusInvalidResumeToken), "Athena failed to resume the previous waiting step because the supplied resume token is no longer valid.", []stepFlowStepSnapshot{{
			StepID: errorStep.StepID,
			Status: progressStatusFailed,
		}})),
	})
}

func invalidResumeTokenClientAction(reason string) string {
	switch reason {
	case "closed":
		return "start_new_request"
	case "expired":
		return "request_new_gap"
	case "session_mismatch":
		return "use_matching_session_and_resume_token"
	default:
		return "request_new_gap"
	}
}

func writeInitialErrorAndFinish(stream *sse.Stream, requestID, sessionID string, prepared *runtime.PreparedExecution, tracker *progressStepTracker) error {
	if prepared == nil || prepared.Initial == nil || prepared.Initial.Error == "" {
		return nil
	}
	errorDetail := prepared.InitialError
	var detail map[string]any
	errorStepID := "initial_error"
	if errorDetail != nil {
		detail = map[string]any{
			"code":          errorDetail.Code,
			"reason":        errorDetail.Reason,
			"retryable":     errorDetail.Retryable,
			"client_action": errorDetail.ClientAction,
		}
		if len(errorDetail.Detail) > 0 {
			detail["detail"] = errorDetail.Detail
		}
	}
	if tracker != nil {
		if err := tracker.failAnalysis(prepared.Initial.Error); err != nil {
			return err
		}
		errorStepID = string(runtime.StageCapabilityResolution)
	} else {
		errorStep := buildErrorProgressStep(errorStepID, prepared.Initial.Error)
		if err := sendProgressStepEvent(stream, requestID, sessionID, progressStatusFailed, errorStep, nil); err != nil {
			return err
		}
	}
	if err := sendSSEEvent(stream, StreamEvent{
		Type:             "error",
		RequestID:        requestID,
		SessionID:        sessionID,
		Status:           string(prepared.InitialStatus),
		Error:            prepared.Initial.Error,
		ErrorDetail:      errorDetail,
		StructuredOutput: structuredOutputOrNil(prepared),
		Detail:           detail,
	}); err != nil {
		return err
	}
	return sendSSEEvent(stream, StreamEvent{
		Type:             "done",
		RequestID:        requestID,
		SessionID:        sessionID,
		Status:           string(prepared.InitialStatus),
		StructuredOutput: structuredOutputOrNil(prepared),
		Detail: withStepFlowDetail(detail, func() StepFlowSummary {
			if tracker != nil {
				return tracker.stepFlowSummary(string(prepared.InitialStatus), "Athena could not finish the current request.")
			}
			return buildStepFlowSummary(string(prepared.InitialStatus), "Athena could not finish the current request.", []stepFlowStepSnapshot{{
				StepID: errorStepID,
				Status: progressStatusFailed,
			}})
		}()),
	})
}

func formatRunPath(runPath []adk.RunStep) string {
	return fmt.Sprintf("%v", runPath)
}

func structuredOutputOrNil(prepared *runtime.PreparedExecution) *runtime.StructuredOutputContract {
	if prepared == nil {
		return nil
	}
	return prepared.StructuredOutput
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
