// app.go defines the main app-layer service that orchestrates transport input into runtime execution.
// app.go 定义 app 层主服务，把传输层输入编排进 runtime 执行主链。
package app

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	einomessage "github.com/cloudwego/eino/schema"
	"moss/internal/config"
	"moss/internal/contextassets"
	"moss/internal/controlplane"
	"moss/internal/customization"
	"moss/internal/memory"
	"moss/internal/model"
	"moss/internal/observability"
	"moss/internal/policy"
	"moss/internal/runtime"
	runtimescene "moss/internal/runtime/scene"
	"moss/internal/session"
	"moss/internal/skills"
	"moss/internal/systemtruth"
	"moss/internal/tools"
	"moss/internal/validationmcp"
	"moss/internal/workflow"
)

// Service is the app-layer orchestrator that bridges transport, session rules, fast path hooks, and runtime execution.
// Service 是 app 层的总编排器，负责衔接 transport、session 规则、fast path 挂点与 runtime 执行。
type Service struct {
	Config        config.Config
	Policy        policy.CapabilityPolicy
	SessionStore  session.Store
	ModelStore    model.Store
	ModelProvider model.Provider
	SkillStore    skills.Store
	PackageStore  skills.PackageStore
	SkillLoader   skills.Loader
	ControlPlane  *controlplane.Manager
	Observability *observability.Manager
	Runtime       *runtime.Service
	RuntimeStore  runtime.RuntimePersistenceStore
	ValidationMCP *validationmcp.Server
	FastPath      FastPathEvaluator
	requestSlots  chan struct{}
}

// ChatRequest is the app-layer request contract before runtime normalization.
// ChatRequest 是进入 runtime 之前的 app 层请求契约。
type ChatRequest struct {
	Query                 string
	TaskType              string
	TaskSubtype           string
	Scene                 string
	SessionID             string
	MainSessionID         string
	WorkspaceID           string
	AppInstanceID         string
	AppSessionID          string
	IntegrationInstanceID string
	WorkflowRunID         string
	StepID                string
	TriggerType           string
	AutomationTaskID      string
	UserLanguage          string
	DesiredOutputMode     string
	GlobalContext         map[string]any
	AppContext            map[string]any
	InputPayload          map[string]any
	ModelID               string
	Customization         customization.UserCustomization
	Supplement            *runtime.SupplementPayload
	TimeoutAfter          time.Duration
	DisableFastPath       bool
}

// ChatSession carries the prepared execution plus app-layer session bookkeeping.
// ChatSession 保存预处理结果以及 app 层需要的 session 记账信息。
type ChatSession struct {
	RequestID string
	SessionID string
	Session   *session.Session
	Prepared  *runtime.PreparedExecution
	FastPath  *FastPathResult
	GapClosed *runtime.GapClosedAction
	Dequeued  *session.DeferredMessage
	release   func()
	save      func(context.Context, string) error
}

// NewService wires the default scaffold service with runtime, model, tools, and observability.
// NewService 负责装配当前脚手架的默认 service、runtime、模型、tools 和观测能力。
func NewService(cfg config.Config) *Service {
	return NewServiceWithObservability(cfg, nil)
}

// NewServiceWithObservability wires the scaffold service with an injected observability manager.
// NewServiceWithObservability 允许在装配脚手架 service 时显式注入 observability manager。
func NewServiceWithObservability(cfg config.Config, obs *observability.Manager) *Service {
	return NewServiceWithDependencies(
		cfg,
		obs,
		session.NewMemoryStoreWithOptions(
			cfg.Runtime.DeferredQueueLimit,
			time.Duration(cfg.Runtime.ClosedTokenTTLSecs)*time.Second,
		),
		model.NewMemoryStore(cfg.Security.EncryptionKey),
	)
}

// NewServiceWithDependencies wires the scaffold service with explicit runtime dependencies.
// NewServiceWithDependencies 允许在装配脚手架 service 时显式注入运行时依赖。
func NewServiceWithDependencies(cfg config.Config, obs *observability.Manager, sessionStore session.Store, modelStore model.Store) *Service {
	return NewServiceWithRuntimeStore(cfg, obs, sessionStore, modelStore, nil)
}

// NewServiceWithRuntimeStore wires the service with an optional runtime persistence store for graph projections.
// NewServiceWithRuntimeStore 会为 service 注入可选 runtime 持久化 store，用于 graph 投影。
func NewServiceWithRuntimeStore(cfg config.Config, obs *observability.Manager, sessionStore session.Store, modelStore model.Store, runtimeStore runtime.RuntimePersistenceStore) *Service {
	if sessionStore == nil {
		sessionStore = session.NewMemoryStoreWithOptions(
			cfg.Runtime.DeferredQueueLimit,
			time.Duration(cfg.Runtime.ClosedTokenTTLSecs)*time.Second,
		)
	}
	if modelStore == nil {
		modelStore = model.NewMemoryStore(cfg.Security.EncryptionKey)
	}

	toolDefs, err := tools.DemoDefinitions()
	if err != nil {
		panic(err)
	}

	if obs == nil {
		obs = observability.NewDefaultManagerWithLevel(observability.LogLevel(cfg.Observability.LogLevel))
	}
	provider := model.NewProvider()
	p := policy.AllowAll()
	skillStore := skills.NewMemoryStore()
	packageStore := skills.NewMemoryPackageStore(cfg.Runtime.SkillPackageRevisionLimit)
	truthDir := strings.TrimSpace(cfg.System.TruthDir)
	if truthDir == "" {
		truthDir = systemtruth.DefaultTruthDir()
	}
	runtimescene.SetSourcesRoot(filepath.Join(truthDir, "sources"))
	workflow.SetSourcesRoot(filepath.Join(truthDir, "sources"))
	skillLoader := skills.NewLoader([]skills.Source{
		skills.NewBuiltinSourceWithTruthDir(truthDir),
		skills.NewPackageSource(packageStore, skills.NewPackageAdapter()),
	}, skillStore)
	registry, err := skillLoader.Load(context.Background())
	if err != nil {
		panic(err)
	}
	activeStateDir := strings.TrimSpace(cfg.System.ActiveStateDir)
	if activeStateDir == "" {
		activeStateDir = filepath.Join("output", "system-state")
	}
	controlPlane := controlplane.NewManagerWithTruthAndStateDirs(controlplane.NewFileStore(cfg.ControlPlane.StorePath), truthDir, activeStateDir)
	if err := controlPlane.SyncSystemSources(context.Background()); err != nil {
		panic(fmt.Errorf("sync system truth failed: %w", err))
	}
	if strings.TrimSpace(cfg.ControlPlane.AuthToken) != "" {
		if dsn := strings.TrimSpace(cfg.PostgresDSN()); dsn != "" {
			authStore, err := controlplane.NewPostgresAuthStateStore(dsn)
			if err != nil {
				panic(fmt.Errorf("configure control-plane auth store failed: %w", err))
			}
			controlPlane.SetAuthStateStore(authStore)
		}
	}
	controlPlane.SetAuthConfig(
		cfg.ControlPlane.AuthToken,
		time.Duration(cfg.ControlPlane.SessionTTLSecs)*time.Second,
		cfg.ControlPlane.MaxFailedAttempts,
	)
	effectiveDefs, err := controlPlane.ApplySkillOverrides(context.Background(), registry.List())
	if err != nil {
		panic(err)
	}
	registry = skills.NewRegistryFromDefinitions(effectiveDefs)
	adapter := skills.NewAdapter()
	// Eino Graph is the default runtime execution surface; the wrapped executor preserves current turn behavior.
	// Eino Graph 是默认 runtime 执行承载面；被包装的 executor 继续保持当前单轮行为。
	checkpointStore, _ := runtimeStore.(runtime.RuntimeGraphCheckpointByteStore)
	turnExecutor := runtime.NewEinoGraphTurnExecutor(
		runtime.NewEinoTurnExecutorWithCheckpointStore(cfg, provider, toolDefs, obs, checkpointStore),
		runtime.EinoGraphTurnExecutorOptions{Store: runtimeStore},
	)

	rt := runtime.NewService(
		cfg,
		p,
		memory.DefaultContextPolicy(),
		registry,
		adapter,
		toolDefs,
		turnExecutor,
		obs,
	)
	if resolver, ok := rt.CapabilityResolver.(runtime.DefaultCapabilityResolver); ok {
		resolver.RegistryProvider = func(context.Context) *skills.Registry {
			registry, err := effectiveSkillRegistry(controlPlane, skillLoader)
			if err != nil {
				return &resolver.Registry
			}
			return registry
		}
		resolver.SceneCatalogProvider = func(ctx context.Context) []runtimescene.Definition {
			catalog, err := effectiveSceneCatalog(ctx, controlPlane)
			if err != nil {
				return runtimescene.BuiltinCatalog()
			}
			return catalog
		}
		rt.CapabilityResolver = resolver
	}

	service := &Service{
		Config:        cfg,
		Policy:        p,
		SessionStore:  sessionStore,
		ModelStore:    modelStore,
		ModelProvider: provider,
		SkillStore:    skillStore,
		PackageStore:  packageStore,
		SkillLoader:   skillLoader,
		ControlPlane:  controlPlane,
		Observability: obs,
		Runtime:       rt,
		RuntimeStore:  runtimeStore,
		ValidationMCP: validationmcp.NewServer(),
		FastPath:      NoopFastPathEvaluator{},
		requestSlots:  make(chan struct{}, cfg.Runtime.MaxConcurrentRequests),
	}
	if err := service.syncRuntimeContractFoundation(context.Background()); err != nil {
		panic(fmt.Errorf("sync runtime contract foundation failed: %w", err))
	}

	return service
}

// resolveContextAssets hydrates ref-first context assets against the active truth dir compile snapshot.
// resolveContextAssets 负责基于 active truth dir 的编译快照水合 ref-first context assets。
func (s *Service) resolveContextAssets(ctx context.Context, assets []contextassets.Asset) ([]contextassets.ResolvedAsset, error) {
	if s == nil || s.ControlPlane == nil || len(assets) == 0 {
		return nil, nil
	}
	result := make([]contextassets.ResolvedAsset, 0, len(assets))
	for _, asset := range assets {
		resolved := contextassets.ResolvedAsset{
			Asset: asset,
		}
		if strings.EqualFold(strings.TrimSpace(asset.Mode), "ref") && asset.Ref != nil && strings.EqualFold(strings.TrimSpace(asset.Ref.RefType), "compiled_asset") {
			target := strings.TrimSpace(asset.Ref.Target)
			if target == "" {
				target = strings.TrimSpace(asset.AssetID)
			}
			compiled, err := s.ControlPlane.GetSystemResourceCompileResult(ctx, target)
			if err != nil {
				continue
			}
			resolved.Summary = strings.TrimSpace(compiled.Summary)
			resolved.GuidanceText = strings.TrimSpace(compiled.GuidanceText)
			if text, _ := compiled.Payload["content_text"].(string); strings.TrimSpace(text) != "" {
				resolved.SourceContent = strings.TrimSpace(text)
			}
			resolved.CompiledVersion = strings.TrimSpace(compiled.CompiledVersion)
			resolved.CompiledChecksum = strings.TrimSpace(compiled.CompiledChecksum)
			resolved.TruthDirVersion = strings.TrimSpace(compiled.TruthDirVersion)
			resolved.Payload = cloneAppAnyMap(compiled.Payload)
			resolved.LoadedFromRef = true
		} else if strings.EqualFold(strings.TrimSpace(asset.Mode), "inline") {
			resolved.Payload = cloneAppAnyMap(asset.Content)
			resolved.Summary = strings.TrimSpace(stringValue(asset.Content["summary"]))
		}
		result = append(result, resolved)
	}
	return result, nil
}

func cloneAppAnyMap(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	result := make(map[string]any, len(input))
	for key, value := range input {
		result[key] = value
	}
	return result
}

// StartupModelGreeting probes the configured model once during startup and logs a simple greeting.
// StartupModelGreeting 会在启动阶段探测一次模型，并记录一条简单的问候输出。
func (s *Service) StartupModelGreeting(ctx context.Context) {
	probeCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	selection, err := s.ModelStore.Resolve(probeCtx, "")
	if err != nil {
		if resolvedErr, ok := err.(*model.ResolveError); ok && resolvedErr.Reason == "default_missing" {
			observability.LogAction(observability.LogLevelWarn, observability.ActionLog{
				Module: "app",
				Action: "startup_model_greeting",
				Step:   "resolve_default_model",
				Status: "skipped",
				Reason: "default_model_missing",
			})
			return
		}
		observability.LogAction(observability.LogLevelError, observability.ActionLog{
			Module:    "app",
			Action:    "startup_model_greeting",
			Step:      "resolve_default_model",
			Status:    "error",
			Reason:    "resolve_failed",
			ErrorCode: "model_resolve_failed",
			Detail: map[string]any{
				"error": err.Error(),
			},
		})
		return
	}

	provider := s.ModelProvider
	if provider == nil {
		provider = model.NewProvider()
	}
	chatConfig, err := resolveAppModelConfig(selection.Primary, startupGreetingPolicyContext())
	if err != nil {
		observability.LogAction(observability.LogLevelError, observability.ActionLog{
			Module:    "app",
			Action:    "startup_model_greeting",
			Step:      "resolve_model_policy",
			Status:    "error",
			Reason:    "policy_resolution_failed",
			ErrorCode: "model_policy_resolution_failed",
			Detail: map[string]any{
				"error": err.Error(),
			},
		})
		return
	}
	chatModel, err := provider.NewChatModel(probeCtx, chatConfig)
	if err != nil {
		observability.LogAction(observability.LogLevelError, observability.ActionLog{
			Module:    "app",
			Action:    "startup_model_greeting",
			Step:      "build_chat_model",
			Status:    "error",
			Reason:    "provider_build_failed",
			ErrorCode: "model_provider_build_failed",
			Detail: map[string]any{
				"error": err.Error(),
			},
		})
		return
	}

	reply, err := chatModel.Generate(probeCtx, []*einomessage.Message{
		einomessage.SystemMessage("You are a concise assistant."),
		einomessage.UserMessage("Return exactly one short random inspirational quote in Chinese. Plain text only, no title, no explanation, no quotation marks."),
	})
	if err != nil {
		observability.LogAction(observability.LogLevelError, observability.ActionLog{
			Module:    "app",
			Action:    "startup_model_greeting",
			Step:      "generate_probe",
			Status:    "error",
			Reason:    "probe_generation_failed",
			ErrorCode: "model_probe_failed",
			Detail: map[string]any{
				"error": err.Error(),
			},
		})
		return
	}

	observability.LogAction(observability.LogLevelInfo, observability.ActionLog{
		Module: "app",
		Action: "startup_model_greeting",
		Step:   "completed",
		Status: "ok",
		Detail: map[string]any{
			"provider_name":  selection.Primary.ProviderName,
			"provider_model": selection.Primary.ProviderModelID,
			"quote":          strings.TrimSpace(reply.Content),
		},
	})
}

// AcquireRequestSlot enforces the configured request concurrency limit before work begins.
// AcquireRequestSlot 会在请求开始前套用配置中的并发槽位限制。
func (s *Service) AcquireRequestSlot(ctx context.Context) error {
	select {
	case s.requestSlots <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// ReleaseRequestSlot returns one concurrency slot back to the app-level limiter.
// ReleaseRequestSlot 会把一个并发槽位归还给 app 层限流器。
func (s *Service) ReleaseRequestSlot() {
	select {
	case <-s.requestSlots:
	default:
	}
}

// ResolveOrCreateSession reuses an existing session or creates one only when the caller omitted session_id.
// ResolveOrCreateSession 会复用已有 session；仅在调用方未提供 session_id 时才自动创建新会话。
func (s *Service) ResolveOrCreateSession(ctx context.Context, sessionID string) (*session.Session, error) {
	now := time.Now().UTC()
	trimmedID := strings.TrimSpace(sessionID)
	if trimmedID != "" {
		if existing, ok := s.SessionStore.Get(ctx, trimmedID); ok {
			if existing.Archived {
				return nil, &InvalidSessionError{SessionID: trimmedID, Reason: "archived"}
			}
			return existing, nil
		}
		return nil, &InvalidSessionError{SessionID: trimmedID, Reason: "not_found"}
	}
	return &session.Session{
		ID:           session.NewID(),
		LastActiveAt: now,
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

// OpenChatSession applies app-level rules before runtime execution, including waiting gaps, fast path hooks, and deferred follow-up messages.
// OpenChatSession 在进入 runtime 前先处理 app 层规则，包括 waiting gap、fast path 挂点和排队消息。
func (s *Service) OpenChatSession(ctx context.Context, requestID string, req ChatRequest) (*ChatSession, error) {
	task, err := buildRuntimeTaskFromRequest(requestID, req)
	if err != nil {
		return nil, err
	}
	if err := s.AcquireRequestSlot(ctx); err != nil {
		return nil, err
	}

	userSessionID := strings.TrimSpace(req.SessionID)
	if userSessionID == "" {
		userSessionID = strings.TrimSpace(req.MainSessionID)
	}

	userSession, err := s.ResolveOrCreateSession(ctx, userSessionID)
	if err != nil {
		s.ReleaseRequestSlot()
		return &ChatSession{
			RequestID: strings.TrimSpace(requestID),
			SessionID: strings.TrimSpace(req.SessionID),
			Prepared:  invalidSessionPrepared(req.SessionID, err),
			release:   s.ReleaseRequestSlot,
			save:      nil,
		}, nil
	}
	queuedMessage, gapClosed, err := enrichSupplementFromPending(
		userSession,
		&req,
		s.Config.Runtime.DeferredQueueLimit,
		time.Duration(s.Config.Runtime.ClosedTokenTTLSecs)*time.Second,
	)
	if err != nil {
		s.ReleaseRequestSlot()
		if pendingErr, ok := err.(*PendingWaitError); ok && (pendingErr.Queued != nil || pendingErr.Dropped != nil) {
			_ = s.SessionStore.Put(ctx, userSession)
			s.Observability.Emit(ctx, observability.Event{
				Name:      "app.waiting_request_blocked",
				RequestID: requestID,
				SessionID: userSession.ID,
				Level:     string(observability.LogLevelWarn),
				Detail: map[string]any{
					"queued":         pendingErr.Queued != nil,
					"queue_overflow": pendingErr.Dropped != nil,
					"pending_stage":  userSession.Pending.Stage,
				},
			})
			s.Observability.Inc("app_waiting_blocked_total", map[string]string{
				"session_id": userSession.ID,
			})
			if pendingErr.Queued != nil {
				s.Observability.RecordAudit(ctx, observability.AuditRecord{
					Action:    "deferred_message_queued",
					RequestID: requestID,
					SessionID: userSession.ID,
					Detail: map[string]any{
						"query":       pendingErr.Queued.Query,
						"received_at": pendingErr.Queued.ReceivedAt,
					},
				})
			}
			if pendingErr.Dropped != nil {
				s.Observability.Inc("app_deferred_queue_overflow_total", map[string]string{
					"session_id": userSession.ID,
				})
				s.Observability.RecordAudit(ctx, observability.AuditRecord{
					Action:    "deferred_queue_overflow",
					RequestID: requestID,
					SessionID: userSession.ID,
					Level:     string(observability.LogLevelWarn),
					Detail: map[string]any{
						"dropped_query":       pendingErr.Dropped.Query,
						"dropped_received_at": pendingErr.Dropped.ReceivedAt,
					},
				})
			}
		}
		var invalidTokenErr *InvalidResumeTokenError
		if errors.As(err, &invalidTokenErr) {
			s.Observability.Inc("app_invalid_resume_token_total", map[string]string{
				"reason": invalidTokenErr.Reason,
			})
			s.Observability.RecordAudit(ctx, observability.AuditRecord{
				Action:    "invalid_resume_token",
				RequestID: requestID,
				SessionID: invalidTokenErr.SessionID,
				Level:     string(observability.LogLevelWarn),
				Detail: map[string]any{
					"resume_token": invalidTokenErr.ResumeToken,
					"reason":       invalidTokenErr.Reason,
				},
			})
		}
		return nil, err
	}
	if queuedMessage != nil && strings.TrimSpace(req.Query) == "" {
		// When a gap closes, the oldest deferred message becomes the next normal request.
		// 当 gap 被关闭时，最早进入队列的消息会成为下一条正常请求。
		req.Query = queuedMessage.Query
		req.ModelID = queuedMessage.ModelID
		req.Customization.PromptTemplate = queuedMessage.PromptTemplate
		req.Customization.EnabledSkills = append([]string(nil), queuedMessage.EnabledSkills...)
		req.Customization.EnabledTools = append([]string(nil), queuedMessage.EnabledTools...)
		req.Customization.ContextAssetOverrides = append([]contextassets.Asset(nil), queuedMessage.ContextAssetOverrides...)
		req.Customization.DisabledAssetTypes = append([]string(nil), queuedMessage.DisabledAssetTypes...)
		req.Customization.AssetPriorityOverrides = cloneCustomizationIntMap(queuedMessage.AssetPriorityOverrides)
		req.DisableFastPath = queuedMessage.DisableFastPath
		restoreDeferredContextAssets(&req, queuedMessage)
		req.Supplement = nil
	}
	if gapClosed != nil {
		s.Observability.Emit(ctx, observability.Event{
			Name:      "app.gap_closed",
			RequestID: requestID,
			SessionID: userSession.ID,
			Detail: map[string]any{
				"resume_token": gapClosed.ResumeToken,
				"close_reason": gapClosed.CloseReason,
				"next_step":    gapClosed.NextStep,
			},
		})
		s.Observability.Inc("app_gap_closed_total", map[string]string{
			"reason": gapClosed.CloseReason,
			"step":   gapClosed.NextStep,
		})
		s.Observability.RecordAudit(ctx, observability.AuditRecord{
			Action:    "gap_closed",
			RequestID: requestID,
			SessionID: userSession.ID,
			Detail: map[string]any{
				"resume_token":  gapClosed.ResumeToken,
				"close_reason":  gapClosed.CloseReason,
				"next_step":     gapClosed.NextStep,
				"token_invalid": gapClosed.TokenInvalid,
			},
		})
	}
	if queuedMessage != nil {
		s.Observability.Emit(ctx, observability.Event{
			Name:      "app.deferred_message_auto_consumed",
			RequestID: requestID,
			SessionID: userSession.ID,
			Detail: map[string]any{
				"received_at": queuedMessage.ReceivedAt,
			},
		})
		s.Observability.Inc("app_deferred_auto_consume_total", map[string]string{
			"session_id": userSession.ID,
		})
		s.Observability.RecordAudit(ctx, observability.AuditRecord{
			Action:    "deferred_message_auto_consumed",
			RequestID: requestID,
			SessionID: userSession.ID,
			Detail: map[string]any{
				"query":       queuedMessage.Query,
				"received_at": queuedMessage.ReceivedAt,
			},
		})
	}

	var (
		prepared *runtime.PreparedExecution
		fastPath *FastPathResult
	)
	if !req.DisableFastPath && s.FastPath != nil {
		fastPath, err = s.FastPath.Evaluate(ctx, userSession, req)
		if err != nil {
			s.ReleaseRequestSlot()
			return nil, err
		}
	}
	if prepared == nil && fastPath != nil && fastPath.Matched {
		prepared = fastPath.Prepared
	} else if prepared == nil {
		if s.ModelStore == nil {
			prepared = invalidModelPrepared("not_configured", nil, "model governance is not configured")
		}
	}
	if prepared == nil {
		modelSelection, modelResolveErr := s.ModelStore.Resolve(ctx, req.ModelID)
		if modelResolveErr != nil {
			reason := "unknown"
			var detail map[string]any
			message := modelResolveErr.Error()
			if resolvedErr, ok := modelResolveErr.(*model.ResolveError); ok {
				reason = resolvedErr.Reason
				detail = resolvedErr.Detail
			}
			prepared = invalidModelPrepared(reason, detail, message)
		} else {
			prepared, err = s.Runtime.Prepare(ctx, userSession, runtime.Input{
				RequestID:       requestID,
				SessionID:       userSession.ID,
				Query:           req.Query,
				ModelSelection:  modelSelection,
				Task:            task,
				Orchestration:   resolveOrchestrationState(req),
				Customization:   req.Customization,
				Supplement:      req.Supplement,
				TimeoutOverride: req.TimeoutAfter,
				Pending:         userSession.Pending,
			})
			if err != nil {
				s.ReleaseRequestSlot()
				return nil, err
			}
		}
	}

	if s.Observability != nil {
		s.Observability.Emit(ctx, observability.Event{
			Name:      "app.chat_session.opened",
			RequestID: requestID,
			SessionID: userSession.ID,
			Detail: map[string]any{
				"has_initial_action": prepared.Initial != nil,
				"query":              req.Query,
				"fast_path_matched":  fastPath != nil && fastPath.Matched,
			},
		})
	}

	return &ChatSession{
		RequestID: requestID,
		SessionID: userSession.ID,
		Session:   userSession,
		Prepared:  prepared,
		FastPath:  fastPath,
		GapClosed: gapClosed,
		Dequeued:  queuedMessage,
		release:   s.ReleaseRequestSlot,
		save: func(ctx context.Context, assistantOutput string) error {
			saved, err := s.SessionStore.Update(ctx, userSession.ID, func(current *session.Session) error {
				now := time.Now().UTC()
				if strings.TrimSpace(req.Query) != "" {
					current.Messages = append(current.Messages, session.Message{
						Role:    "user",
						Content: req.Query,
					})
				}
				if strings.TrimSpace(assistantOutput) != "" {
					current.Messages = append(current.Messages, session.Message{
						Role:    "assistant",
						Content: assistantOutput,
					})
				}
				assets, bindings, compiledRefs := snapshotContextAssets(req.GlobalContext)
				current.ContextAssets = assets
				current.ContextAssetBindings = bindings
				current.CompiledAssetRefs = compiledRefs
				if prepared.InitialStatus == runtime.RequestStatusWaitingForInformation && prepared.Initial != nil && prepared.Initial.Action != nil && prepared.TimeoutWait != nil {
					pending := &session.PendingState{
						Stage:        string(prepared.TimeoutWait.Stage),
						Status:       string(prepared.InitialStatus),
						ActionType:   string(prepared.Initial.Action.Type),
						ResumeToken:  prepared.TimeoutWait.ResumeToken,
						ModelID:      req.ModelID,
						TimeoutAt:    prepared.TimeoutWait.TimeoutAt,
						TimeoutAfter: prepared.TimeoutWait.TimeoutAfter,
					}
					if prepared.Spec != nil && prepared.Spec.Metadata.PreservedContext != nil {
						pending.Preserved = cloneSessionPreservedContext(prepared.Spec.Metadata.PreservedContext)
						pending.Preserved.WaitStage = pending.Stage
						pending.Preserved.ResumeToken = pending.ResumeToken
						pending.Preserved.TimeoutAt = pending.TimeoutAt
						pending.Preserved.TimeoutAfter = pending.TimeoutAfter
					}
					if prepared.Initial.Action.InformationRequest != nil {
						for _, item := range prepared.Initial.Action.InformationRequest.Missing {
							pending.MissingFields = append(pending.MissingFields, item.Field)
						}
					}
					current.Pending = pending
				} else {
					current.Pending = nil
				}
				current.LastActiveAt = now
				return nil
			})
			if err != nil {
				return err
			}
			userSession = saved
			return nil
		},
	}, nil
}

func resolveOrchestrationState(req ChatRequest) runtime.OrchestrationState {
	if req.Supplement != nil && req.Supplement.Resume != nil && strings.TrimSpace(req.Supplement.Resume.ResumeToken) != "" {
		return runtime.OrchestrationStateResumed
	}
	return runtime.OrchestrationStateNormalized
}

// invalidModelPrepared converts model-selection failures into one transport-safe initial error response.
// invalidModelPrepared 会把模型选择失败转换成一次对外安全的初始错误响应。
func invalidModelPrepared(reason string, detail map[string]any, message string) *runtime.PreparedExecution {
	if strings.TrimSpace(message) == "" {
		message = invalidModelMessage(reason)
	}
	return &runtime.PreparedExecution{
		Initial: &runtime.TurnResult{
			Kind:  runtime.TurnResultError,
			Error: message,
		},
		InitialError: &runtime.ProtocolError{
			Code:         "invalid_model",
			Reason:       reason,
			Retryable:    false,
			ClientAction: "select_available_model",
			Detail:       invalidModelDetail(detail),
		},
		InitialStatus: runtime.RequestStatusInvalidModel,
	}
}

// invalidSessionPrepared converts session-resolution failures into one transport-safe initial error response.
// invalidSessionPrepared 会把 session 解析失败转换成一次对外安全的初始错误响应。
func invalidSessionPrepared(sessionID string, err error) *runtime.PreparedExecution {
	reason := "not_found"
	message := "requested session is not available"
	clientAction := "start_new_session"
	detail := map[string]any{
		"session_id": strings.TrimSpace(sessionID),
	}
	if invalidErr, ok := err.(*InvalidSessionError); ok {
		reason = invalidErr.Reason
		message = invalidErr.Error()
		if invalidErr.Reason == "archived" {
			detail["archived"] = true
			clientAction = "create_new_session"
		}
	}
	return &runtime.PreparedExecution{
		Initial: &runtime.TurnResult{
			Kind:  runtime.TurnResultError,
			Error: message,
		},
		InitialError: &runtime.ProtocolError{
			Code:         "invalid_session",
			Reason:       reason,
			Retryable:    false,
			ClientAction: clientAction,
			Detail:       detail,
		},
		InitialStatus: runtime.RequestStatus("invalid_session"),
	}
}

func invalidModelMessage(reason string) string {
	switch strings.TrimSpace(reason) {
	case "not_found":
		return "requested model was not found"
	case "disabled":
		return "requested model is disabled"
	case "provider_disabled":
		return "requested model provider is disabled"
	case "default_missing":
		return "default model is not configured"
	case "default_provider_disabled":
		return "default model provider is disabled"
	case "not_configured":
		return "model governance is not configured"
	default:
		return "requested model is not available"
	}
}

func invalidModelDetail(detail map[string]any) map[string]any {
	if len(detail) == 0 {
		return nil
	}
	cloned := make(map[string]any, len(detail))
	for key, value := range detail {
		if key == "model_record_id" {
			cloned["model_id"] = value
			continue
		}
		cloned[key] = value
	}
	return cloned
}

// enrichSupplementFromPending enforces the waiting-gap protocol before runtime sees the request.
// enrichSupplementFromPending 在请求进入 runtime 之前，先落实 waiting gap 协议。
func enrichSupplementFromPending(userSession *session.Session, req *ChatRequest, deferredQueueLimit int, closedTokenTTL time.Duration) (*session.DeferredMessage, *runtime.GapClosedAction, error) {
	if userSession == nil || userSession.Pending == nil {
		if err := validateResumeTokenWithoutPending(userSession, req, deferredQueueLimit, closedTokenTTL); err != nil {
			return nil, nil, err
		}
		if userSession != nil {
			userSession.Normalize(time.Now(), deferredQueueLimit, closedTokenTTL)
		}
		return nil, nil, nil
	}

	userSession.Normalize(time.Now(), deferredQueueLimit, closedTokenTTL)

	pending := userSession.Pending
	if req.Supplement == nil && !pending.TimeoutAt.IsZero() && time.Now().After(pending.TimeoutAt) {
		req.Supplement = &runtime.SupplementPayload{
			Outcome: runtime.SupplementOutcomeTimeoutExpired,
		}
	}
	if req.Supplement == nil {
		// A normal message during waiting is queued for later instead of polluting the current recovery turn.
		// waiting 期间到达的普通消息不会污染当前恢复链，而是先进入延后消费队列。
		queued, dropped := queueDeferredMessage(userSession, *req, deferredQueueLimit, closedTokenTTL)
		pendingErr := newPendingWaitError(userSession.ID, pending)
		pendingErr.Queued = queued
		pendingErr.Dropped = dropped
		return nil, nil, pendingErr
	}
	if req.Supplement.Resume == nil {
		req.Supplement.Resume = &runtime.ResumeContext{}
	}
	if req.Supplement.Resume.Stage == "" {
		req.Supplement.Resume.Stage = runtime.RuntimeStage(pending.Stage)
	}
	if req.Supplement.Resume.ResumeToken != "" && pending.ResumeToken != "" && req.Supplement.Resume.ResumeToken != pending.ResumeToken {
		return nil, nil, &InvalidResumeTokenError{
			SessionID:   userSession.ID,
			ResumeToken: req.Supplement.Resume.ResumeToken,
			Reason:      "session_mismatch",
		}
	}
	if req.Supplement.Resume.ResumeToken == "" {
		req.Supplement.Resume.ResumeToken = pending.ResumeToken
	}
	req.ModelID = pending.ModelID
	if strings.TrimSpace(req.Query) == "" && pending.Preserved != nil && strings.TrimSpace(pending.Preserved.Goal) != "" {
		req.Query = pending.Preserved.Goal
	}
	if len(req.Supplement.Data) == 0 && req.Supplement.Outcome == "" {
		queued, dropped := queueDeferredMessage(userSession, *req, deferredQueueLimit, closedTokenTTL)
		pendingErr := newPendingWaitError(userSession.ID, pending)
		pendingErr.Queued = queued
		pendingErr.Dropped = dropped
		return nil, nil, pendingErr
	}
	if closesPendingGap(req.Supplement.Outcome) {
		// Closing a gap invalidates its token first, then optionally hands one deferred message back to the mainline.
		// 关闭 gap 时先让 token 失效，再按需把一条排队消息交回主线。
		userSession.Pending = nil
		recordClosedResumeToken(userSession, pending.ResumeToken, string(req.Supplement.Outcome), deferredQueueLimit, closedTokenTTL)
		gapClosed := &runtime.GapClosedAction{
			ResumeToken:  pending.ResumeToken,
			CloseReason:  string(req.Supplement.Outcome),
			TokenInvalid: true,
		}
		dequeued := dequeueDeferredMessage(userSession)
		switch {
		case dequeued != nil:
			gapClosed.NextStep = "consume_deferred"
		case req.Supplement.Outcome == runtime.SupplementOutcomePendingHuman:
			gapClosed.NextStep = "pending_human"
		case req.Supplement.Outcome == runtime.SupplementOutcomeAbandonAndContinue:
			gapClosed.NextStep = "continue_mainline"
		case req.Supplement.Outcome == runtime.SupplementOutcomeUnableToProvide:
			gapClosed.NextStep = "policy_decide"
		case req.Supplement.Outcome == runtime.SupplementOutcomeTimeoutExpired:
			gapClosed.NextStep = "timeout_policy"
		}
		return dequeued, gapClosed, nil
	}
	return nil, nil, nil
}

func cloneSessionPreservedContext(ctx *session.PreservedContext) *session.PreservedContext {
	if ctx == nil {
		return nil
	}
	cloned := *ctx
	cloned.MissingFields = append([]string(nil), ctx.MissingFields...)
	if len(ctx.Facts) > 0 {
		cloned.Facts = make(map[string]string, len(ctx.Facts))
		for key, value := range ctx.Facts {
			cloned.Facts[key] = value
		}
	}
	return &cloned
}

// queueDeferredMessage stores a normal follow-up input without letting it pollute the active recovery turn.
// queueDeferredMessage 会把普通后续输入暂存到 deferred queue，避免污染当前恢复链。
func queueDeferredMessage(userSession *session.Session, req ChatRequest, deferredQueueLimit int, closedTokenTTL time.Duration) (*session.DeferredMessage, *session.DeferredMessage) {
	if userSession == nil || strings.TrimSpace(req.Query) == "" {
		return nil, nil
	}
	deferred := session.DeferredMessage{
		Query:                  strings.TrimSpace(req.Query),
		ModelID:                req.ModelID,
		PromptTemplate:         req.Customization.PromptTemplate,
		EnabledSkills:          append([]string(nil), req.Customization.EnabledSkills...),
		EnabledTools:           append([]string(nil), req.Customization.EnabledTools...),
		ContextAssetOverrides:  append([]contextassets.Asset(nil), req.Customization.ContextAssetOverrides...),
		DisabledAssetTypes:     append([]string(nil), req.Customization.DisabledAssetTypes...),
		AssetPriorityOverrides: cloneCustomizationIntMap(req.Customization.AssetPriorityOverrides),
		ContextAssets:          nil,
		ContextBindings:        nil,
		CompiledRefs:           nil,
		DisableFastPath:        req.DisableFastPath,
		ReceivedAt:             time.Now(),
	}
	deferred.ContextAssets, deferred.ContextBindings, deferred.CompiledRefs = snapshotContextAssets(req.GlobalContext)
	var dropped *session.DeferredMessage
	if deferredQueueLimit > 0 && len(userSession.DeferredQueue) >= deferredQueueLimit {
		droppedItem := userSession.DeferredQueue[0]
		dropped = &droppedItem
	}
	userSession.DeferredQueue = append(userSession.DeferredQueue, deferred)
	userSession.Normalize(time.Now(), deferredQueueLimit, closedTokenTTL)
	return &deferred, dropped
}

// dequeueDeferredMessage pops the oldest deferred input so it can become the next normal request.
// dequeueDeferredMessage 会弹出最早的排队消息，使其成为下一条正常请求。
func dequeueDeferredMessage(userSession *session.Session) *session.DeferredMessage {
	if userSession == nil || len(userSession.DeferredQueue) == 0 {
		return nil
	}
	item := userSession.DeferredQueue[0]
	userSession.DeferredQueue = append([]session.DeferredMessage(nil), userSession.DeferredQueue[1:]...)
	return &item
}

func snapshotContextAssets(globalContext map[string]any) ([]contextassets.Asset, []contextassets.ResolvedAsset, []contextassets.Ref) {
	bundle := contextassets.BuildBundle(globalContext)
	if bundle == nil {
		return nil, nil, nil
	}
	assets := make([]contextassets.Asset, 0, len(bundle.Assets))
	bindings := make([]contextassets.ResolvedAsset, 0, len(bundle.ResolvedAssets))
	compiledRefs := make([]contextassets.Ref, 0, len(bundle.Assets))
	for _, asset := range bundle.Assets {
		assets = append(assets, asset)
		if asset.Ref != nil && strings.EqualFold(strings.TrimSpace(asset.Ref.RefType), "compiled_asset") {
			compiledRefs = append(compiledRefs, *asset.Ref)
		}
	}
	bindings = append(bindings, bundle.ResolvedAssets...)
	return assets, bindings, compiledRefs
}

func restoreDeferredContextAssets(req *ChatRequest, deferred *session.DeferredMessage) {
	if req == nil || deferred == nil {
		return
	}
	if req.GlobalContext == nil {
		req.GlobalContext = map[string]any{}
	}
	if len(deferred.ContextAssets) > 0 && req.GlobalContext["context_assets"] == nil {
		req.GlobalContext["context_assets"] = toContextAssetMaps(deferred.ContextAssets)
	}
	if len(deferred.ContextBindings) > 0 && req.GlobalContext["context_assets_resolved"] == nil {
		req.GlobalContext["context_assets_resolved"] = toAnySlice(contextassets.ResolvedMaps(deferred.ContextBindings))
	}
}

func cloneCustomizationIntMap(input map[string]int) map[string]int {
	if len(input) == 0 {
		return nil
	}
	result := make(map[string]int, len(input))
	for key, value := range input {
		result[key] = value
	}
	return result
}

func toContextAssetMaps(items []contextassets.Asset) []any {
	result := make([]any, 0, len(items))
	for _, item := range items {
		entry := map[string]any{
			"asset_id":           item.AssetID,
			"asset_type":         item.AssetType,
			"asset_name":         item.AssetName,
			"scope":              item.Scope,
			"source_kind":        item.SourceKind,
			"mode":               item.Mode,
			"priority":           item.Priority,
			"read_only":          item.ReadOnly,
			"candidate_writable": item.CandidateWritable,
			"auth_scope":         item.AuthScope,
			"content":            cloneAppAnyMap(item.Content),
			"resolution": map[string]any{
				"prefer_compiled":       item.Resolution.PreferCompiled,
				"allow_detail_fetch":    item.Resolution.AllowDetailFetch,
				"allow_inline_fallback": item.Resolution.AllowInlineFallback,
				"resident_hint":         item.Resolution.ResidentHint,
				"background_only":       item.Resolution.BackgroundOnly,
			},
			"metadata": map[string]any{
				"source_label": item.Metadata.SourceLabel,
				"tags":         append([]string(nil), item.Metadata.Tags...),
			},
		}
		if item.Ref != nil {
			entry["ref"] = map[string]any{
				"ref_type":          item.Ref.RefType,
				"target":            item.Ref.Target,
				"version":           item.Ref.Version,
				"checksum":          item.Ref.Checksum,
				"truth_dir_version": item.Ref.TruthDirVersion,
				"detail_endpoint":   item.Ref.DetailEndpoint,
			}
		}
		result = append(result, entry)
	}
	return result
}

func toAnySlice(items []map[string]any) []any {
	if len(items) == 0 {
		return nil
	}
	result := make([]any, 0, len(items))
	for _, item := range items {
		result = append(result, item)
	}
	return result
}

// recordClosedResumeToken keeps a tombstone so closed gaps can be distinguished from unknown tokens later.
// recordClosedResumeToken 会保留已关闭 token 的 tombstone，方便后续区分“已关闭”和“未知 token”。
func recordClosedResumeToken(userSession *session.Session, token, reason string, deferredQueueLimit int, closedTokenTTL time.Duration) {
	if userSession == nil || strings.TrimSpace(token) == "" {
		return
	}
	userSession.ClosedTokens = append(userSession.ClosedTokens, session.ClosedResumeToken{
		Token:    token,
		Reason:   reason,
		ClosedAt: time.Now(),
	})
	userSession.Normalize(time.Now(), deferredQueueLimit, closedTokenTTL)
}

// validateResumeTokenWithoutPending rejects stale tokens when the session no longer has an active gap.
// validateResumeTokenWithoutPending 会在 session 已无活跃 gap 时拦截过期或失效 token。
func validateResumeTokenWithoutPending(userSession *session.Session, req *ChatRequest, deferredQueueLimit int, closedTokenTTL time.Duration) error {
	if userSession == nil || req == nil || req.Supplement == nil || req.Supplement.Resume == nil {
		return nil
	}
	now := time.Now()
	resumeToken := strings.TrimSpace(req.Supplement.Resume.ResumeToken)
	if resumeToken == "" {
		return nil
	}
	for _, closed := range userSession.ClosedTokens {
		if closed.Token != resumeToken {
			continue
		}
		if closedTokenTTL > 0 && !closed.ClosedAt.IsZero() && now.Sub(closed.ClosedAt) >= closedTokenTTL {
			userSession.Normalize(now, deferredQueueLimit, closedTokenTTL)
			return &InvalidResumeTokenError{
				SessionID:   userSession.ID,
				ResumeToken: resumeToken,
				Reason:      "expired",
			}
		}
		userSession.Normalize(now, deferredQueueLimit, closedTokenTTL)
		return &InvalidResumeTokenError{
			SessionID:   userSession.ID,
			ResumeToken: resumeToken,
			Reason:      "closed",
		}
	}
	userSession.Normalize(now, deferredQueueLimit, closedTokenTTL)
	for _, closed := range userSession.ClosedTokens {
		if closed.Token == resumeToken {
			return &InvalidResumeTokenError{
				SessionID:   userSession.ID,
				ResumeToken: resumeToken,
				Reason:      "closed",
			}
		}
	}
	return &InvalidResumeTokenError{
		SessionID:   userSession.ID,
		ResumeToken: resumeToken,
		Reason:      "not_found",
	}
}

// closesPendingGap reports whether the supplied outcome should permanently close the active gap.
// closesPendingGap 用于判断某个 outcome 是否会永久关闭当前活跃 gap。
func closesPendingGap(outcome runtime.SupplementOutcome) bool {
	switch outcome {
	case runtime.SupplementOutcomeUnableToProvide, runtime.SupplementOutcomeTimeoutExpired, runtime.SupplementOutcomeAbandonAndContinue, runtime.SupplementOutcomePendingHuman:
		return true
	default:
		return false
	}
}

// Release frees the app-level request slot owned by the session.
// Release 会释放当前 ChatSession 占用的 app 层并发槽位。
func (s *ChatSession) Release() {
	if s.release != nil {
		s.release()
	}
}

// Complete persists the final session state after the current request has finished.
// Complete 会在当前请求结束后持久化最终的 session 状态。
func (s *ChatSession) Complete(ctx context.Context, assistantOutput string) error {
	if s.save == nil {
		return nil
	}
	return s.save(ctx, assistantOutput)
}
