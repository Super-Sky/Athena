package app

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"moss/internal/config"
	"moss/internal/contextassets"
	"moss/internal/customization"
	"moss/internal/model"
	"moss/internal/observability"
	"moss/internal/runtime"
	"moss/internal/session"
)

type matchingFastPathEvaluator struct {
	called bool
}

func (m *matchingFastPathEvaluator) Evaluate(_ context.Context, _ *session.Session, _ ChatRequest) (*FastPathResult, error) {
	m.called = true
	return &FastPathResult{
		Matched: true,
		Name:    "test_fast_path",
		Reason:  "matched by test evaluator",
		Prepared: &runtime.PreparedExecution{
			Initial: &runtime.TurnResult{
				Kind:    runtime.TurnResultFinal,
				Content: "fast path result",
			},
		},
	}, nil
}

func TestInvalidModelPreparedMapsModelRecordIDToModelID(t *testing.T) {
	prepared := invalidModelPrepared("not_found", map[string]any{
		"model_record_id": "model-explicit",
		"provider_id":     "provider-1",
	}, "")

	if prepared.Initial == nil {
		t.Fatalf("expected initial turn result")
	}
	if prepared.Initial.Kind != runtime.TurnResultError {
		t.Fatalf("initial kind = %q, want %q", prepared.Initial.Kind, runtime.TurnResultError)
	}
	if prepared.Initial.Error != "requested model was not found" {
		t.Fatalf("initial error = %q, want requested model was not found", prepared.Initial.Error)
	}
	if prepared.InitialError == nil {
		t.Fatalf("expected initial error")
	}
	if prepared.InitialError.Detail["model_id"] != "model-explicit" {
		t.Fatalf("model_id detail = %#v, want model-explicit", prepared.InitialError.Detail)
	}
	if _, exists := prepared.InitialError.Detail["model_record_id"]; exists {
		t.Fatalf("unexpected model_record_id detail = %#v", prepared.InitialError.Detail)
	}
	if prepared.InitialError.Detail["provider_id"] != "provider-1" {
		t.Fatalf("provider_id detail = %#v, want provider-1", prepared.InitialError.Detail)
	}
}

func TestEnrichSupplementFromPendingPreservesPendingModelIDOnResume(t *testing.T) {
	userSession := &session.Session{
		ID: "sess-resume-explicit",
		Pending: &session.PendingState{
			Stage:       string(runtime.StageCapabilityResolution),
			Status:      string(runtime.RequestStatusWaitingForInformation),
			ResumeToken: "resume-explicit",
			ModelID:     "model-pending",
			TimeoutAt:   time.Now().Add(time.Minute),
		},
	}

	req := ChatRequest{
		ModelID: "model-new",
		Supplement: &runtime.SupplementPayload{
			Outcome: runtime.SupplementOutcomeProvided,
			Data:    map[string]string{"user_id": "u1001"},
			Resume:  &runtime.ResumeContext{ResumeToken: "resume-explicit"},
		},
	}
	if _, _, err := enrichSupplementFromPending(userSession, &req, session.DefaultDeferredQueueLimit, session.DefaultClosedResumeTokenTTL); err != nil {
		t.Fatalf("enrichSupplementFromPending() error = %v", err)
	}
	if req.ModelID != "model-pending" {
		t.Fatalf("req.ModelID = %q, want model-pending", req.ModelID)
	}
}

func TestEnrichSupplementFromPendingClearsNewExplicitModelWhenPendingWasDefault(t *testing.T) {
	userSession := &session.Session{
		ID: "sess-resume-default",
		Pending: &session.PendingState{
			Stage:       string(runtime.StageCapabilityResolution),
			Status:      string(runtime.RequestStatusWaitingForInformation),
			ResumeToken: "resume-default",
			TimeoutAt:   time.Now().Add(time.Minute),
		},
	}

	req := ChatRequest{
		ModelID: "model-new",
		Supplement: &runtime.SupplementPayload{
			Outcome: runtime.SupplementOutcomeProvided,
			Data:    map[string]string{"user_id": "u1001"},
			Resume:  &runtime.ResumeContext{ResumeToken: "resume-default"},
		},
	}
	if _, _, err := enrichSupplementFromPending(userSession, &req, session.DefaultDeferredQueueLimit, session.DefaultClosedResumeTokenTTL); err != nil {
		t.Fatalf("enrichSupplementFromPending() error = %v", err)
	}
	if req.ModelID != "" {
		t.Fatalf("req.ModelID = %q, want empty for default re-resolve", req.ModelID)
	}
}

func TestEnrichSupplementFromPendingSetsResumeAndTimeoutOutcome(t *testing.T) {
	userSession := &session.Session{
		ID: "sess-1",
		Pending: &session.PendingState{
			Stage:       string(runtime.StageCapabilityResolution),
			Status:      string(runtime.RequestStatusWaitingForInformation),
			ResumeToken: "resume-1",
			ModelID:     "model-pending",
			TimeoutAt:   time.Now().Add(-time.Minute),
		},
	}

	req := ChatRequest{}
	if _, _, err := enrichSupplementFromPending(userSession, &req, session.DefaultDeferredQueueLimit, session.DefaultClosedResumeTokenTTL); err != nil {
		t.Fatalf("enrichSupplementFromPending() error = %v", err)
	}

	if req.Supplement == nil {
		t.Fatalf("expected supplement to be created")
	}
	if req.Supplement.Outcome != runtime.SupplementOutcomeTimeoutExpired {
		t.Fatalf("supplement outcome = %q, want %q", req.Supplement.Outcome, runtime.SupplementOutcomeTimeoutExpired)
	}
	if req.Supplement.Resume == nil || req.Supplement.Resume.ResumeToken != "resume-1" {
		t.Fatalf("unexpected resume context = %#v", req.Supplement.Resume)
	}
	if req.ModelID != "model-pending" {
		t.Fatalf("req.ModelID = %q, want model-pending", req.ModelID)
	}
	if req.Supplement.Resume.Stage != runtime.StageCapabilityResolution {
		t.Fatalf("resume stage = %q, want %q", req.Supplement.Resume.Stage, runtime.StageCapabilityResolution)
	}
}

func TestEnrichSupplementFromPendingRestoresPreservedGoalWhenResumeQueryEmpty(t *testing.T) {
	userSession := &session.Session{
		ID: "sess-preserved-goal",
		Pending: &session.PendingState{
			Stage:       string(runtime.StageCapabilityResolution),
			Status:      string(runtime.RequestStatusWaitingForInformation),
			ResumeToken: "resume-goal",
			TimeoutAt:   time.Now().Add(time.Minute),
			Preserved: &session.PreservedContext{
				Goal: "show user profile",
			},
		},
	}

	req := ChatRequest{
		Supplement: &runtime.SupplementPayload{
			Outcome: runtime.SupplementOutcomeProvided,
			Data:    map[string]string{"user_id": "u1001"},
			Resume:  &runtime.ResumeContext{ResumeToken: "resume-goal"},
		},
	}
	if _, _, err := enrichSupplementFromPending(userSession, &req, session.DefaultDeferredQueueLimit, session.DefaultClosedResumeTokenTTL); err != nil {
		t.Fatalf("enrichSupplementFromPending() error = %v", err)
	}
	if req.Query != "show user profile" {
		t.Fatalf("req.Query = %q, want preserved goal", req.Query)
	}
}

func TestEnrichSupplementFromPendingRejectsMismatchedResumeToken(t *testing.T) {
	userSession := &session.Session{
		ID: "sess-2",
		Pending: &session.PendingState{
			Stage:       string(runtime.StageCapabilityResolution),
			Status:      string(runtime.RequestStatusWaitingForInformation),
			ResumeToken: "resume-2",
			TimeoutAt:   time.Now().Add(time.Minute),
		},
	}

	req := ChatRequest{
		Supplement: &runtime.SupplementPayload{
			Outcome: runtime.SupplementOutcomeProvided,
			Resume: &runtime.ResumeContext{
				ResumeToken: "bad-token",
			},
		},
	}
	if _, _, err := enrichSupplementFromPending(userSession, &req, session.DefaultDeferredQueueLimit, session.DefaultClosedResumeTokenTTL); err == nil {
		t.Fatalf("expected resume token mismatch error")
	}
}

func TestEnrichSupplementFromPendingReturnsPendingWaitErrorWithoutSupplement(t *testing.T) {
	userSession := &session.Session{
		ID: "sess-3",
		Pending: &session.PendingState{
			Stage:         string(runtime.StageCapabilityResolution),
			Status:        string(runtime.RequestStatusWaitingForInformation),
			ActionType:    string(runtime.ActionTypeInformationRequest),
			ResumeToken:   "resume-3",
			TimeoutAt:     time.Now().Add(time.Minute),
			TimeoutAfter:  time.Minute,
			MissingFields: []string{"user_id"},
		},
	}

	req := ChatRequest{Query: "继续分析", ModelID: "model-queued"}
	_, _, err := enrichSupplementFromPending(userSession, &req, session.DefaultDeferredQueueLimit, session.DefaultClosedResumeTokenTTL)
	if err == nil {
		t.Fatalf("expected pending wait error")
	}

	var pendingErr *PendingWaitError
	if !errors.As(err, &pendingErr) {
		t.Fatalf("expected PendingWaitError, got %T", err)
	}
	if pendingErr.Pending == nil || pendingErr.Pending.ResumeToken != "resume-3" {
		t.Fatalf("unexpected pending wait error = %#v", pendingErr)
	}
	if pendingErr.Queued == nil || pendingErr.Queued.Query != "继续分析" {
		t.Fatalf("expected deferred queue item, got %#v", pendingErr.Queued)
	}
	if pendingErr.Queued.ModelID != "model-queued" {
		t.Fatalf("queued model id = %q, want model-queued", pendingErr.Queued.ModelID)
	}
}

func TestEnrichSupplementFromPendingClosesGapAndDequeuesOneMessage(t *testing.T) {
	userSession := &session.Session{
		ID: "sess-4",
		Pending: &session.PendingState{
			Stage:       string(runtime.StageCapabilityResolution),
			Status:      string(runtime.RequestStatusWaitingForInformation),
			ResumeToken: "resume-4",
			TimeoutAt:   time.Now().Add(time.Minute),
		},
		DeferredQueue: []session.DeferredMessage{{
			Query:      "下一条问题",
			ModelID:    "model-deferred",
			ReceivedAt: time.Now(),
		}},
	}

	req := ChatRequest{
		Supplement: &runtime.SupplementPayload{
			Outcome: runtime.SupplementOutcomeAbandonAndContinue,
			Resume:  &runtime.ResumeContext{ResumeToken: "resume-4"},
		},
	}
	dequeued, gapClosed, err := enrichSupplementFromPending(userSession, &req, session.DefaultDeferredQueueLimit, session.DefaultClosedResumeTokenTTL)
	if err != nil {
		t.Fatalf("enrichSupplementFromPending() error = %v", err)
	}
	if userSession.Pending != nil {
		t.Fatalf("expected pending gap to be cleared")
	}
	if dequeued == nil || dequeued.Query != "下一条问题" {
		t.Fatalf("unexpected dequeued message = %#v", dequeued)
	}
	if dequeued.ModelID != "model-deferred" {
		t.Fatalf("dequeued model id = %q, want model-deferred", dequeued.ModelID)
	}
	if gapClosed == nil || gapClosed.CloseReason != string(runtime.SupplementOutcomeAbandonAndContinue) {
		t.Fatalf("unexpected gapClosed = %#v", gapClosed)
	}
	if len(userSession.ClosedTokens) != 1 || userSession.ClosedTokens[0].Token != "resume-4" {
		t.Fatalf("expected closed token tombstone, got %#v", userSession.ClosedTokens)
	}
}

func TestEnrichSupplementFromPendingRejectsClosedResumeToken(t *testing.T) {
	userSession := &session.Session{
		ID: "sess-5",
		ClosedTokens: []session.ClosedResumeToken{{
			Token:    "resume-closed",
			Reason:   "abandon_and_continue",
			ClosedAt: time.Now(),
		}},
	}

	req := ChatRequest{
		Query: "补数",
		Supplement: &runtime.SupplementPayload{
			Outcome: runtime.SupplementOutcomeProvided,
			Resume:  &runtime.ResumeContext{ResumeToken: "resume-closed"},
		},
	}
	_, _, err := enrichSupplementFromPending(userSession, &req, session.DefaultDeferredQueueLimit, session.DefaultClosedResumeTokenTTL)
	if err == nil {
		t.Fatalf("expected invalid resume token error")
	}
	var invalid *InvalidResumeTokenError
	if !errors.As(err, &invalid) {
		t.Fatalf("expected InvalidResumeTokenError, got %T", err)
	}
	if invalid.Reason != "closed" {
		t.Fatalf("invalid reason = %q, want closed", invalid.Reason)
	}
}

func TestEnrichSupplementFromPendingPrunesExpiredClosedResumeToken(t *testing.T) {
	userSession := &session.Session{
		ID: "sess-6",
		ClosedTokens: []session.ClosedResumeToken{{
			Token:    "resume-expired",
			Reason:   "abandon_and_continue",
			ClosedAt: time.Now().Add(-session.DefaultClosedResumeTokenTTL - time.Minute),
		}},
	}

	req := ChatRequest{
		Query: "补数",
		Supplement: &runtime.SupplementPayload{
			Outcome: runtime.SupplementOutcomeProvided,
			Resume:  &runtime.ResumeContext{ResumeToken: "resume-expired"},
		},
	}
	_, _, err := enrichSupplementFromPending(userSession, &req, session.DefaultDeferredQueueLimit, session.DefaultClosedResumeTokenTTL)
	if err == nil {
		t.Fatalf("expected invalid resume token error")
	}
	var invalid *InvalidResumeTokenError
	if !errors.As(err, &invalid) {
		t.Fatalf("expected InvalidResumeTokenError, got %T", err)
	}
	if invalid.Reason != "expired" {
		t.Fatalf("invalid reason = %q, want expired", invalid.Reason)
	}
	if len(userSession.ClosedTokens) != 0 {
		t.Fatalf("expected expired tombstone to be pruned, got %#v", userSession.ClosedTokens)
	}
}

func TestEnrichSupplementFromPendingCapsDeferredQueue(t *testing.T) {
	userSession := &session.Session{
		ID: "sess-7",
		Pending: &session.PendingState{
			Stage:       string(runtime.StageCapabilityResolution),
			Status:      string(runtime.RequestStatusWaitingForInformation),
			ResumeToken: "resume-7",
			TimeoutAt:   time.Now().Add(time.Minute),
		},
	}
	for idx := 0; idx < session.DefaultDeferredQueueLimit; idx++ {
		userSession.DeferredQueue = append(userSession.DeferredQueue, session.DeferredMessage{
			Query:      fmt.Sprintf("old-%d", idx),
			ReceivedAt: time.Unix(int64(idx), 0),
		})
	}

	req := ChatRequest{Query: "latest-message"}
	_, _, err := enrichSupplementFromPending(userSession, &req, session.DefaultDeferredQueueLimit, session.DefaultClosedResumeTokenTTL)
	if err == nil {
		t.Fatalf("expected pending wait error")
	}
	var pendingErr *PendingWaitError
	if !errors.As(err, &pendingErr) {
		t.Fatalf("expected PendingWaitError, got %T", err)
	}
	if len(userSession.DeferredQueue) != session.DefaultDeferredQueueLimit {
		t.Fatalf("deferred queue length = %d, want %d", len(userSession.DeferredQueue), session.DefaultDeferredQueueLimit)
	}
	if userSession.DeferredQueue[0].Query != "old-1" {
		t.Fatalf("unexpected oldest retained message = %#v", userSession.DeferredQueue[0])
	}
	if userSession.DeferredQueue[len(userSession.DeferredQueue)-1].Query != "latest-message" {
		t.Fatalf("expected latest message to be retained, got %#v", userSession.DeferredQueue[len(userSession.DeferredQueue)-1])
	}
	if pendingErr.Dropped == nil || pendingErr.Dropped.Query != "old-0" {
		t.Fatalf("expected oldest deferred message to be reported as dropped, got %#v", pendingErr.Dropped)
	}
}

func TestQueueDeferredMessageSnapshotsContextAssets(t *testing.T) {
	userSession := &session.Session{ID: "sess-context-assets"}
	req := ChatRequest{
		Query: "后续问题",
		Customization: customization.UserCustomization{
			ContextAssetOverrides: []contextassets.Asset{{
				AssetID:   "persona.override",
				AssetType: "persona",
				Priority:  77,
			}},
			DisabledAssetTypes: []string{"skill"},
			AssetPriorityOverrides: map[string]int{
				"memory.weekly-review": 88,
			},
		},
		GlobalContext: map[string]any{
			"context_assets": []any{
				map[string]any{
					"asset_id":           "memory.weekly-review",
					"asset_type":         "memory_view",
					"asset_name":         "weekly-review",
					"candidate_writable": true,
					"ref": map[string]any{
						"ref_type": "compiled_asset",
						"target":   "memory.weekly-review",
					},
				},
			},
			"context_assets_resolved": []any{
				map[string]any{
					"asset": map[string]any{
						"asset_id":   "memory.weekly-review",
						"asset_type": "memory_view",
					},
					"summary":         "resident memory",
					"loaded_from_ref": true,
				},
			},
		},
	}

	queued, dropped := queueDeferredMessage(userSession, req, session.DefaultDeferredQueueLimit, session.DefaultClosedResumeTokenTTL)
	if dropped != nil {
		t.Fatalf("unexpected dropped deferred message = %#v", dropped)
	}
	if queued == nil {
		t.Fatal("expected queued deferred message")
	}
	if len(queued.ContextAssets) != 1 {
		t.Fatalf("queued context assets len = %d, want 1", len(queued.ContextAssets))
	}
	if len(queued.ContextBindings) != 1 {
		t.Fatalf("queued context bindings len = %d, want 1", len(queued.ContextBindings))
	}
	if len(queued.CompiledRefs) != 1 {
		t.Fatalf("queued compiled refs len = %d, want 1", len(queued.CompiledRefs))
	}
	if len(queued.ContextAssetOverrides) != 1 || queued.ContextAssetOverrides[0].AssetID != "persona.override" {
		t.Fatalf("queued context asset overrides = %#v", queued.ContextAssetOverrides)
	}
	if len(queued.DisabledAssetTypes) != 1 || queued.DisabledAssetTypes[0] != "skill" {
		t.Fatalf("queued disabled asset types = %#v", queued.DisabledAssetTypes)
	}
	if queued.AssetPriorityOverrides["memory.weekly-review"] != 88 {
		t.Fatalf("queued asset priority overrides = %#v", queued.AssetPriorityOverrides)
	}
}

func TestRestoreDeferredContextAssetsHydratesGlobalContext(t *testing.T) {
	req := &ChatRequest{}
	deferred := &session.DeferredMessage{
		ContextAssets: []contextassets.Asset{{
			AssetID:   "memory.weekly-review",
			AssetType: "memory_view",
			AssetName: "weekly-review",
			Ref: &contextassets.Ref{
				RefType: "compiled_asset",
				Target:  "memory.weekly-review",
			},
		}},
		ContextBindings: []contextassets.ResolvedAsset{{
			Asset: contextassets.Asset{
				AssetID:   "memory.weekly-review",
				AssetType: "memory_view",
			},
			Summary:       "resident memory",
			LoadedFromRef: true,
		}},
	}

	restoreDeferredContextAssets(req, deferred)
	if req.GlobalContext == nil {
		t.Fatal("expected global context to be created")
	}
	assets, _ := req.GlobalContext["context_assets"].([]any)
	if len(assets) != 1 {
		t.Fatalf("restored context_assets len = %d, want 1; payload=%#v", len(assets), req.GlobalContext["context_assets"])
	}
	resolved, _ := req.GlobalContext["context_assets_resolved"].([]any)
	if len(resolved) != 1 {
		t.Fatalf("restored context_assets_resolved len = %d, want 1; payload=%#v", len(resolved), req.GlobalContext["context_assets_resolved"])
	}
}

func TestOpenChatSessionUsesMatchedFastPath(t *testing.T) {
	evaluator := &matchingFastPathEvaluator{}
	service := &Service{
		SessionStore:  session.NewMemoryStoreWithOptions(session.DefaultDeferredQueueLimit, session.DefaultClosedResumeTokenTTL),
		Observability: observability.NewNoopManager(),
		FastPath:      evaluator,
		requestSlots:  make(chan struct{}, 1),
	}

	chatSession, err := service.OpenChatSession(context.Background(), "req-fast-1", ChatRequest{
		Query: "快速回答",
	})
	if err != nil {
		t.Fatalf("OpenChatSession() error = %v", err)
	}
	defer chatSession.Release()

	if !evaluator.called {
		t.Fatalf("expected fast path evaluator to be called")
	}
	if chatSession.FastPath == nil || !chatSession.FastPath.Matched {
		t.Fatalf("expected matched fast path, got %#v", chatSession.FastPath)
	}
	if chatSession.Prepared == nil || chatSession.Prepared.Initial == nil || chatSession.Prepared.Initial.Content != "fast path result" {
		t.Fatalf("unexpected prepared result = %#v", chatSession.Prepared)
	}
}

func TestOpenChatSessionSkipsFastPathWhenDisabled(t *testing.T) {
	evaluator := &matchingFastPathEvaluator{}
	service := NewService(config.Config{
		Runtime: config.RuntimeConfig{
			MaxConcurrentRequests: 2,
			MaxConcurrentTools:    2,
			RequestTimeoutSeconds: 30,
			DeferredQueueLimit:    session.DefaultDeferredQueueLimit,
			ClosedTokenTTLSecs:    int(session.DefaultClosedResumeTokenTTL / time.Second),
		},
	})
	service.FastPath = evaluator

	chatSession, err := service.OpenChatSession(context.Background(), "req-fast-2", ChatRequest{
		Query:           "快速回答",
		DisableFastPath: true,
	})
	if err != nil {
		t.Fatalf("OpenChatSession() error = %v", err)
	}
	defer chatSession.Release()
	if evaluator.called {
		t.Fatalf("fast path evaluator should not be called when disabled")
	}
	if chatSession.FastPath != nil {
		t.Fatalf("expected no fast path result when disabled, got %#v", chatSession.FastPath)
	}
}

func TestChatSessionCompletePreservesConcurrentDeferredQueueUpdates(t *testing.T) {
	evaluator := &matchingFastPathEvaluator{}
	store := session.NewMemoryStoreWithOptions(session.DefaultDeferredQueueLimit, session.DefaultClosedResumeTokenTTL)
	initial := &session.Session{ID: "sess-complete-1", CreatedAt: time.Now(), UpdatedAt: time.Now(), LastActiveAt: time.Now()}
	if err := store.Put(context.Background(), initial); err != nil {
		t.Fatalf("Put(initial) error = %v", err)
	}
	service := &Service{
		SessionStore:  store,
		Observability: observability.NewNoopManager(),
		FastPath:      evaluator,
		requestSlots:  make(chan struct{}, 1),
	}

	chatSession, err := service.OpenChatSession(context.Background(), "req-complete-1", ChatRequest{
		Query:     "first question",
		SessionID: "sess-complete-1",
	})
	if err != nil {
		t.Fatalf("OpenChatSession() error = %v", err)
	}
	defer chatSession.Release()

	if _, err := store.Update(context.Background(), "sess-complete-1", func(current *session.Session) error {
		current.DeferredQueue = append(current.DeferredQueue, session.DeferredMessage{
			Query:      "queued follow-up",
			ReceivedAt: time.Unix(1, 0),
		})
		return nil
	}); err != nil {
		t.Fatalf("SessionStore.Update() error = %v", err)
	}

	if err := chatSession.Complete(context.Background(), "assistant answer"); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	saved, ok := store.Get(context.Background(), "sess-complete-1")
	if !ok {
		t.Fatalf("expected persisted session")
	}
	if len(saved.Messages) != 2 {
		t.Fatalf("messages len = %d, want 2", len(saved.Messages))
	}
	if len(saved.DeferredQueue) != 1 || saved.DeferredQueue[0].Query != "queued follow-up" {
		t.Fatalf("expected deferred queue update to be preserved, got %#v", saved.DeferredQueue)
	}
}

func TestOpenChatSessionSecurityQueryRequiresUserID(t *testing.T) {
	service, defaultModelID := newSecurityChatTestService(t)

	chatSession, err := service.OpenChatSession(context.Background(), "req-security-wait", ChatRequest{
		Query:   "assess whether this user should enter manual review",
		ModelID: defaultModelID,
		Customization: customization.UserCustomization{
			EnabledSkills: []string{"user_overview"},
		},
		DisableFastPath: true,
	})
	if err != nil {
		t.Fatalf("OpenChatSession() error = %v", err)
	}
	defer chatSession.Release()

	if chatSession.Prepared == nil || chatSession.Prepared.Initial == nil || chatSession.Prepared.Initial.Action == nil {
		t.Fatalf("expected waiting action, got %#v", chatSession.Prepared)
	}
	if chatSession.Prepared.InitialStatus != runtime.RequestStatusWaitingForInformation {
		t.Fatalf("initial status = %q, want %q", chatSession.Prepared.InitialStatus, runtime.RequestStatusWaitingForInformation)
	}
	if chatSession.Prepared.Spec == nil || chatSession.Prepared.Spec.Skill.PrimarySkill != "user_overview" {
		t.Fatalf("primary skill = %#v, want user_overview", chatSession.Prepared.Spec)
	}
	if chatSession.Prepared.Initial.Action.InformationRequest == nil || len(chatSession.Prepared.Initial.Action.InformationRequest.Missing) != 1 {
		t.Fatalf("expected one missing information item, got %#v", chatSession.Prepared.Initial.Action)
	}
	if got := chatSession.Prepared.Initial.Action.InformationRequest.Missing[0].Field; got != "user_id" {
		t.Fatalf("missing field = %q, want user_id", got)
	}
}

func TestOpenChatSessionSecurityQueryWithUserIDBecomesRunnable(t *testing.T) {
	service, defaultModelID := newSecurityChatTestService(t)

	chatSession, err := service.OpenChatSession(context.Background(), "req-security-run", ChatRequest{
		Query:   "assess whether user u1001 should enter manual review",
		ModelID: defaultModelID,
		Customization: customization.UserCustomization{
			EnabledSkills: []string{"user_overview"},
		},
		Supplement: &runtime.SupplementPayload{
			Data: map[string]string{"user_id": "u1001"},
		},
		DisableFastPath: true,
	})
	if err != nil {
		t.Fatalf("OpenChatSession() error = %v", err)
	}
	defer chatSession.Release()

	if chatSession.Prepared == nil {
		t.Fatalf("expected prepared execution")
	}
	if chatSession.Prepared.Initial != nil {
		t.Fatalf("expected runnable execution without initial action, got %#v", chatSession.Prepared.Initial)
	}
	if chatSession.Prepared.Runner == nil {
		t.Fatalf("expected prepared runner")
	}
	if chatSession.Prepared.Spec == nil || chatSession.Prepared.Spec.Skill.PrimarySkill != "user_overview" {
		t.Fatalf("primary skill = %#v, want user_overview", chatSession.Prepared.Spec)
	}
	if got := chatSession.Prepared.Spec.Model.Requested.ModelRecordID; got != defaultModelID {
		t.Fatalf("requested model record id = %q, want %q", got, defaultModelID)
	}
	if len(chatSession.Prepared.Spec.Tools.AllowedTools) != 0 {
		t.Fatalf("allowed tools len = %d, want 0 for context-only user_overview", len(chatSession.Prepared.Spec.Tools.AllowedTools))
	}
}

func TestOpenChatSessionExplicitMissingModelReturnsInitialInvalidModel(t *testing.T) {
	service, _ := newSecurityChatTestService(t)

	chatSession, err := service.OpenChatSession(context.Background(), "req-invalid-model", ChatRequest{
		Query:           "assess the current user risk posture",
		ModelID:         "model-missing",
		DisableFastPath: true,
	})
	if err != nil {
		t.Fatalf("OpenChatSession() error = %v", err)
	}
	defer chatSession.Release()

	if chatSession.Prepared == nil || chatSession.Prepared.Initial == nil {
		t.Fatalf("expected initial invalid_model result, got %#v", chatSession.Prepared)
	}
	if chatSession.Prepared.Initial.Kind != runtime.TurnResultError {
		t.Fatalf("initial kind = %q, want %q", chatSession.Prepared.Initial.Kind, runtime.TurnResultError)
	}
	if chatSession.Prepared.InitialStatus != runtime.RequestStatusInvalidModel {
		t.Fatalf("initial status = %q, want %q", chatSession.Prepared.InitialStatus, runtime.RequestStatusInvalidModel)
	}
	if chatSession.Prepared.Initial.Error != "requested model was not found" {
		t.Fatalf("initial error = %q, want requested model was not found", chatSession.Prepared.Initial.Error)
	}
	if chatSession.Prepared.InitialError == nil || chatSession.Prepared.InitialError.Detail["model_id"] != "model-missing" {
		t.Fatalf("initial error detail = %#v, want model_id=model-missing", chatSession.Prepared.InitialError)
	}
}

func newSecurityChatTestService(t *testing.T) (*Service, string) {
	t.Helper()

	cfg := config.Config{
		Runtime: config.RuntimeConfig{
			MaxConcurrentRequests:     2,
			MaxConcurrentTools:        2,
			RequestTimeoutSeconds:     30,
			DeferredQueueLimit:        session.DefaultDeferredQueueLimit,
			ClosedTokenTTLSecs:        int(session.DefaultClosedResumeTokenTTL / time.Second),
			SkillPackageRevisionLimit: 5,
		},
		Security: config.SecurityConfig{
			EncryptionKey: "security-chat-test-encryption-key",
		},
		Observability: config.ObservabilityConfig{
			LogLevel: "warn",
		},
	}
	sessionStore := session.NewMemoryStoreWithOptions(session.DefaultDeferredQueueLimit, session.DefaultClosedResumeTokenTTL)
	modelStore := model.NewMemoryStore(cfg.Security.EncryptionKey)

	provider, err := modelStore.CreateProvider(context.Background(), model.ProviderUpsertInput{
		Name:                  "security-chat-provider",
		BaseURL:               "https://example.com/v1",
		Protocol:              model.ProtocolOpenAICompatible,
		APIKey:                "sk-security-chat",
		RequestTimeoutSeconds: 30,
		Enabled:               true,
	})
	if err != nil {
		t.Fatalf("CreateProvider() error = %v", err)
	}
	modelRecord, err := modelStore.CreateProviderModel(context.Background(), provider.ID, model.ProviderModelUpsertInput{
		ModelID:     "security-chat-model",
		DisplayName: "Security Chat Model",
		Enabled:     true,
		IsDefault:   true,
	})
	if err != nil {
		t.Fatalf("CreateProviderModel() error = %v", err)
	}

	service := NewServiceWithDependencies(cfg, observability.NewNoopManager(), sessionStore, modelStore)
	service.FastPath = &matchingFastPathEvaluator{}
	return service, modelRecord.ID
}
