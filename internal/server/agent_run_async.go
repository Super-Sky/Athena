// agent_run_async.go maps durable PostgreSQL jobs and Redis deliveries onto the Agent Run HTTP contract.
// agent_run_async.go 把 PostgreSQL 持久任务与 Redis 投递映射到 Agent Run HTTP 契约。
package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/google/uuid"
	"github.com/hertz-contrib/sse"
	appcore "moss/internal/app"
	"moss/internal/asyncjob"
	"moss/internal/config"
	"moss/internal/contextassets"
	"moss/internal/runtime"
)

const (
	asyncAgentRunKind           = "agent_run"
	asyncAgentRunPreferToken    = "respond-async"
	asyncAgentRunIdempotencyKey = "Idempotency-Key"
	asyncAgentRunEventPoll      = 250 * time.Millisecond
	asyncAgentRunEventHeartbeat = 15 * time.Second
)

type asyncAgentRunEnvelope struct {
	RequestID string               `json:"request_id"`
	Request   agentRunStartRequest `json:"request"`
}

type asyncAgentRunResult struct {
	HTTPStatus int              `json:"http_status"`
	Response   agentRunResponse `json:"response"`
}

type asyncAgentRunHTTPResponse struct {
	JobID        string            `json:"job_id"`
	RunID        string            `json:"run_id"`
	Status       asyncjob.Status   `json:"status"`
	Attempt      int               `json:"attempt"`
	MaxAttempts  int               `json:"max_attempts"`
	Duplicate    bool              `json:"duplicate,omitempty"`
	EventsURL    string            `json:"events_url"`
	RuntimeRunID string            `json:"runtime_run_id,omitempty"`
	Result       *agentRunResponse `json:"result,omitempty"`
	ErrorCode    string            `json:"error_code,omitempty"`
	ErrorMessage string            `json:"error_message,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

// AgentRunAsyncService owns durable Agent Run creation, reads, cancellation, resume, and worker execution.
// AgentRunAsyncService 负责持久 Agent Run 的创建、读取、取消、恢复和 worker 执行。
type AgentRunAsyncService struct {
	store       asyncjob.Store
	codec       *asyncjob.Codec
	config      config.Config
	application *appcore.Service
}

// NewAgentRunAsyncService constructs the transport/application bridge without owning queue delivery.
// NewAgentRunAsyncService 构建 transport 与 application 的桥接层，但不持有队列投递职责。
func NewAgentRunAsyncService(cfg config.Config, application *appcore.Service, store asyncjob.Store, codec *asyncjob.Codec) (*AgentRunAsyncService, error) {
	if !cfg.AsyncJobs.Enabled {
		return nil, nil
	}
	if application == nil || store == nil || codec == nil {
		return nil, fmt.Errorf("async Agent Run application, store, and codec are required")
	}
	return &AgentRunAsyncService{store: store, codec: codec, config: cfg, application: application}, nil
}

func handleCreateAgentRunWithAsync(ctx context.Context, c *app.RequestContext, cfg config.Config, application *appcore.Service, service *AgentRunAsyncService) {
	requestID := newRequestID()
	c.Header("X-Request-ID", requestID)
	req, err := parseAgentRunStartRequest(c)
	if err != nil {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := applyAgentRunIdentityScope(ctx, &req); err != nil {
		writeAppAuthError(c, err)
		return
	}
	if strings.TrimSpace(agentRunGoal(req.Goal, req.Query)) == "" && req.Supplement == nil {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": "goal is required unless supplement is provided"})
		return
	}
	if service != nil && prefersAsync(c) {
		service.enqueue(ctx, c, requestID, req)
		return
	}
	timeoutCtx, cancel := context.WithTimeout(withRequestID(ctx, requestID), time.Duration(cfg.Runtime.RequestTimeoutSeconds)*time.Second)
	defer cancel()
	response, status := executeAgentRun(timeoutCtx, c, cfg, application, requestID, req, agentRunExecutionOptions{Source: agentRunSourceCreate})
	c.JSON(status, response)
}

func handleGetAgentRunWithAsync(ctx context.Context, c *app.RequestContext, cfg config.Config, application *appcore.Service, service *AgentRunAsyncService) {
	if service != nil {
		found, err := service.writeJob(ctx, c, strings.TrimSpace(c.Param("runID")))
		if found || err != nil {
			return
		}
	}
	handleGetAgentRun(ctx, c, cfg, application)
}

func handleCancelAgentRunWithAsync(ctx context.Context, c *app.RequestContext, cfg config.Config, application *appcore.Service, service *AgentRunAsyncService) {
	if service != nil {
		found, err := service.cancel(ctx, c, strings.TrimSpace(c.Param("runID")))
		if found || err != nil {
			return
		}
	}
	handleCancelAgentRun(ctx, c, cfg, application)
}

func handleResumeAgentRunWithAsync(ctx context.Context, c *app.RequestContext, cfg config.Config, application *appcore.Service, service *AgentRunAsyncService) {
	if service != nil {
		found, err := service.resume(ctx, c, strings.TrimSpace(c.Param("runID")))
		if found || err != nil {
			return
		}
	}
	handleResumeAgentRun(ctx, c, cfg, application)
}

func handleAgentRunAsyncEvents(ctx context.Context, c *app.RequestContext, _ config.Config, _ *appcore.Service, service *AgentRunAsyncService) {
	if service == nil {
		c.JSON(consts.StatusNotFound, map[string]string{"error": "resource_not_found"})
		return
	}
	service.streamEvents(ctx, c, strings.TrimSpace(c.Param("runID")))
}

func (s *AgentRunAsyncService) enqueue(ctx context.Context, c *app.RequestContext, requestID string, req agentRunStartRequest) {
	idempotencyKey := strings.TrimSpace(string(c.Request.Header.Peek(asyncAgentRunIdempotencyKey)))
	if idempotencyKey == "" {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": "idempotency_key_required"})
		return
	}
	jobID := uuid.NewString()
	envelope := asyncAgentRunEnvelope{RequestID: requestID, Request: req}
	canonicalRequest, err := json.Marshal(req)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, map[string]string{"error": "encode_async_request_failed"})
		return
	}
	plain, err := json.Marshal(envelope)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, map[string]string{"error": "encode_async_request_failed"})
		return
	}
	digest := sha256.Sum256(canonicalRequest)
	encrypted, err := s.codec.Encrypt(plain, asyncAgentRunAAD(jobID, jobID))
	if err != nil {
		c.JSON(consts.StatusInternalServerError, map[string]string{"error": "encrypt_async_request_failed"})
		return
	}
	identity, _ := appRequestIdentity(ctx)
	job, duplicate, err := s.store.Create(ctx, asyncjob.CreateInput{
		ID: jobID, RunID: jobID, Kind: asyncAgentRunKind, RequestSHA256: hex.EncodeToString(digest[:]), EncryptedRequest: encrypted,
		IdempotencyScope: asyncAgentRunScope(identity, req), IdempotencyKey: idempotencyKey, MaxAttempts: s.config.AsyncJobs.MaxAttempts,
		AppID: identity.AppID, WorkspaceID: req.WorkspaceID, AppInstanceID: req.AppInstanceID,
	})
	if err != nil {
		if errors.Is(err, asyncjob.ErrIdempotencyConflict) {
			c.JSON(consts.StatusConflict, map[string]string{"error": "idempotency_conflict"})
			return
		}
		c.JSON(consts.StatusServiceUnavailable, map[string]string{"error": "async_job_persistence_unavailable"})
		return
	}
	c.Header("Location", asyncAgentRunLocation(job.RunID))
	c.JSON(consts.StatusAccepted, asyncAgentRunResponse(job, duplicate, nil))
}

func (s *AgentRunAsyncService) writeJob(ctx context.Context, c *app.RequestContext, id string) (bool, error) {
	job, err := s.store.Get(ctx, id)
	if errors.Is(err, asyncjob.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		c.JSON(consts.StatusServiceUnavailable, map[string]string{"error": "async_job_persistence_unavailable"})
		return true, err
	}
	if !authorizeAsyncJob(ctx, job) {
		c.JSON(consts.StatusNotFound, map[string]string{"error": "resource_not_found"})
		return true, errAgentRunNotFound
	}
	result, decodeErr := s.decodeResult(job)
	if decodeErr != nil {
		c.JSON(consts.StatusInternalServerError, map[string]string{"error": "decode_async_result_failed"})
		return true, decodeErr
	}
	c.JSON(consts.StatusOK, asyncAgentRunResponse(job, false, result))
	return true, nil
}

func (s *AgentRunAsyncService) cancel(ctx context.Context, c *app.RequestContext, id string) (bool, error) {
	job, err := s.store.Get(ctx, id)
	if errors.Is(err, asyncjob.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		c.JSON(consts.StatusServiceUnavailable, map[string]string{"error": "async_job_persistence_unavailable"})
		return true, err
	}
	if !authorizeAsyncJob(ctx, job) {
		c.JSON(consts.StatusNotFound, map[string]string{"error": "resource_not_found"})
		return true, errAgentRunNotFound
	}
	var req agentRunCancelRequest
	if len(c.Request.Body()) > 0 {
		if err := json.Unmarshal(c.Request.Body(), &req); err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": fmt.Sprintf("invalid json body: %v", err)})
			return true, err
		}
	}
	job, err = s.store.RequestCancel(ctx, id, req.Reason)
	if errors.Is(err, asyncjob.ErrAlreadyTerminal) {
		c.JSON(consts.StatusConflict, asyncAgentRunResponse(job, false, nil))
		return true, err
	}
	if err != nil {
		c.JSON(consts.StatusConflict, map[string]string{"error": "cancel_rejected"})
		return true, err
	}
	c.JSON(consts.StatusAccepted, asyncAgentRunResponse(job, false, nil))
	return true, nil
}

func (s *AgentRunAsyncService) resume(ctx context.Context, c *app.RequestContext, id string) (bool, error) {
	job, err := s.store.Get(ctx, id)
	if errors.Is(err, asyncjob.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		c.JSON(consts.StatusServiceUnavailable, map[string]string{"error": "async_job_persistence_unavailable"})
		return true, err
	}
	if !authorizeAsyncJob(ctx, job) {
		c.JSON(consts.StatusNotFound, map[string]string{"error": "resource_not_found"})
		return true, errAgentRunNotFound
	}
	resumeReq, err := parseAgentRunResumeRequest(c)
	if err != nil {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
		return true, err
	}
	if strings.TrimSpace(agentRunGoal(resumeReq.Goal, resumeReq.Query)) == "" && resumeReq.Supplement == nil {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": "goal is required unless supplement is provided"})
		return true, fmt.Errorf("goal is required unless supplement is provided")
	}
	plain, err := s.codec.Decrypt(job.EncryptedRequest, asyncAgentRunAAD(job.ID, job.RunID))
	if err != nil {
		c.JSON(consts.StatusInternalServerError, map[string]string{"error": "decode_async_request_failed"})
		return true, err
	}
	var envelope asyncAgentRunEnvelope
	if err := json.Unmarshal(plain, &envelope); err != nil {
		c.JSON(consts.StatusInternalServerError, map[string]string{"error": "decode_async_request_failed"})
		return true, err
	}
	envelope.Request = asyncAgentRunResumeRequest(envelope.Request, resumeReq, id)
	envelope.RequestID = newRequestID()
	updated, err := json.Marshal(envelope)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, map[string]string{"error": "encode_async_request_failed"})
		return true, err
	}
	digest := sha256.Sum256(updated)
	encrypted, err := s.codec.Encrypt(updated, asyncAgentRunAAD(job.ID, job.RunID))
	if err != nil {
		c.JSON(consts.StatusInternalServerError, map[string]string{"error": "encrypt_async_request_failed"})
		return true, err
	}
	job, err = s.store.Resume(ctx, id, encrypted, hex.EncodeToString(digest[:]), time.Now().UTC())
	if err != nil {
		c.JSON(consts.StatusConflict, map[string]string{"error": "resume_rejected"})
		return true, err
	}
	c.JSON(consts.StatusAccepted, asyncAgentRunResponse(job, false, nil))
	return true, nil
}

// Handler returns the worker callback that decrypts one request and persists only encrypted results.
// Handler 返回 worker 回调，负责解密请求并只持久化加密结果。
func (s *AgentRunAsyncService) Handler() asyncjob.Handler {
	return func(ctx context.Context, job asyncjob.Job) (asyncjob.Completion, *asyncjob.Failure) {
		plain, err := s.codec.Decrypt(job.EncryptedRequest, asyncAgentRunAAD(job.ID, job.RunID))
		if err != nil {
			return asyncjob.Completion{}, &asyncjob.Failure{Code: "invalid_encrypted_request", Message: err.Error()}
		}
		var envelope asyncAgentRunEnvelope
		if err := json.Unmarshal(plain, &envelope); err != nil {
			return asyncjob.Completion{}, &asyncjob.Failure{Code: "invalid_async_request", Message: err.Error()}
		}
		canonicalTools, err := canonicalAgentRunTools(envelope.Request.Tools)
		if err != nil {
			return asyncjob.Completion{}, &asyncjob.Failure{Code: "invalid_tool_contract", Message: err.Error()}
		}
		canonicalChoice, err := canonicalAgentRunToolChoice(envelope.Request.ToolChoice, canonicalTools)
		if err != nil {
			return asyncjob.Completion{}, &asyncjob.Failure{Code: "invalid_tool_choice", Message: err.Error()}
		}
		envelope.Request.CanonicalTools, envelope.Request.CanonicalToolChoice = canonicalTools, canonicalChoice
		requestContext := &app.RequestContext{}
		response, status := executeAgentRun(withRequestID(ctx, envelope.RequestID), requestContext, s.config, s.application, envelope.RequestID, envelope.Request, agentRunExecutionOptions{Source: agentRunSourceCreate})
		result := asyncAgentRunResult{HTTPStatus: status, Response: response}
		resultPlain, err := json.Marshal(result)
		if err != nil {
			return asyncjob.Completion{}, &asyncjob.Failure{Code: "encode_async_result_failed", Message: err.Error()}
		}
		encrypted, err := s.codec.Encrypt(resultPlain, asyncAgentRunAAD(job.ID, job.RunID))
		if err != nil {
			return asyncjob.Completion{}, &asyncjob.Failure{Code: "encrypt_async_result_failed", Message: err.Error()}
		}
		waiting := response.WaitState != nil || response.StopReason == string(runtime.ExecutionStopAwaitingInput) || response.StopReason == string(runtime.ExecutionStopAwaitingExternalData)
		if status >= consts.StatusBadRequest {
			code := strings.TrimSpace(response.StopReason)
			if code == "" {
				code = "agent_run_failed"
			}
			return asyncjob.Completion{}, &asyncjob.Failure{Code: code, Message: response.Error, EncryptedResult: encrypted,
				Retryable: status >= consts.StatusInternalServerError && response.RunID == "" && len(response.ToolCalls) == 0}
		}
		return asyncjob.Completion{EncryptedResult: encrypted, Waiting: waiting, WaitingReason: response.StopReason}, nil
	}
}

func (s *AgentRunAsyncService) decodeResult(job asyncjob.Job) (*asyncAgentRunResult, error) {
	if strings.TrimSpace(job.EncryptedResult) == "" {
		return nil, nil
	}
	plain, err := s.codec.Decrypt(job.EncryptedResult, asyncAgentRunAAD(job.ID, job.RunID))
	if err != nil {
		return nil, err
	}
	var result asyncAgentRunResult
	if err := json.Unmarshal(plain, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func asyncAgentRunResponse(job asyncjob.Job, duplicate bool, result *asyncAgentRunResult) asyncAgentRunHTTPResponse {
	response := asyncAgentRunHTTPResponse{
		JobID: job.ID, RunID: job.RunID, Status: job.Status, Attempt: job.Attempt, MaxAttempts: job.MaxAttempts,
		Duplicate: duplicate, EventsURL: asyncAgentRunLocation(job.RunID) + "/events", ErrorCode: job.LastErrorCode,
		ErrorMessage: job.LastErrorMessage, CreatedAt: job.CreatedAt, UpdatedAt: job.UpdatedAt,
	}
	if result != nil {
		response.RuntimeRunID = result.Response.RunID
		response.Result = &result.Response
	}
	return response
}

func asyncAgentRunResumeRequest(original agentRunStartRequest, resume agentRunResumeRequest, runID string) agentRunStartRequest {
	return agentRunStartRequest{
		Goal: resume.Goal, Query: resume.Query, ContextAssets: append([]contextassets.Asset(nil), resume.ContextAssets...),
		SessionID: resume.SessionID, MainSessionID: resume.MainSessionID, ModelID: resume.ModelID,
		EnabledSkills: append([]string(nil), resume.EnabledSkills...), EnabledTools: append([]string(nil), resume.EnabledTools...),
		GlobalContext: cloneAnyMap(resume.GlobalContext), AppContext: cloneAnyMap(resume.AppContext), InputPayload: cloneAnyMap(resume.InputPayload),
		Supplement: resume.Supplement, SupplementOutcome: resume.SupplementOutcome, ResumeToken: resume.ResumeToken,
		TimeoutAfterSeconds: resume.TimeoutAfterSeconds, DisableFastPath: resume.DisableFastPath, PromptTemplate: resume.PromptTemplate,
		DisabledAssetTypes: append([]string(nil), resume.DisabledAssetTypes...), AssetPriorityOverrides: cloneIntMap(resume.AssetPriorityOverrides),
		ResumedFromRunID: runID, WorkspaceID: original.WorkspaceID, AppInstanceID: original.AppInstanceID,
		SuccessCriteria: append([]string(nil), original.SuccessCriteria...), Constraints: cloneAnyMap(original.Constraints), Budget: cloneAnyMap(original.Budget),
		Tools: append([]agentRunToolDeclaration(nil), original.Tools...), ToolChoice: original.ToolChoice, MemoryScope: cloneAnyMap(original.MemoryScope),
		GovernanceRefs: append([]string(nil), original.GovernanceRefs...), TaskType: original.TaskType, TaskSubtype: original.TaskSubtype, Scene: original.Scene,
	}
}

func prefersAsync(c *app.RequestContext) bool {
	for _, token := range strings.Split(strings.ToLower(string(c.Request.Header.Peek("Prefer"))), ",") {
		if strings.TrimSpace(token) == asyncAgentRunPreferToken {
			return true
		}
	}
	return false
}

func asyncAgentRunScope(identity AppRequestIdentity, req agentRunStartRequest) string {
	appID := strings.TrimSpace(identity.AppID)
	workspaceID := strings.TrimSpace(req.WorkspaceID)
	instanceID := strings.TrimSpace(req.AppInstanceID)
	return fmt.Sprintf("%d:%s|%d:%s|%d:%s", len(appID), appID, len(workspaceID), workspaceID, len(instanceID), instanceID)
}

func asyncAgentRunAAD(jobID, runID string) string {
	return "athena.async-job.v1\x00" + strings.TrimSpace(jobID) + "\x00" + strings.TrimSpace(runID)
}

func asyncAgentRunLocation(runID string) string {
	return "/api/agent/runs/" + strings.TrimSpace(runID)
}

func authorizeAsyncJob(ctx context.Context, job asyncjob.Job) bool {
	identity, ok := appRequestIdentity(ctx)
	if !ok {
		return true
	}
	return strings.TrimSpace(job.AppID) == identity.AppID && strings.TrimSpace(job.WorkspaceID) == identity.WorkspaceID && strings.TrimSpace(job.AppInstanceID) == identity.AppInstanceID
}

func parseAsyncEventCursor(c *app.RequestContext) (uint64, error) {
	raw := strings.TrimSpace(sse.GetLastEventID(c))
	if raw == "" {
		raw = strings.TrimSpace(string(c.Query("after")))
	}
	if raw == "" {
		return 0, nil
	}
	return strconv.ParseUint(raw, 10, 64)
}
