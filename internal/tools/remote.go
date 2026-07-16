// remote.go adapts app-owned HTTP tools into Athena's provider-neutral tool catalog.
// remote.go 将应用自有 HTTP 工具适配到 Athena 的通用工具目录。
package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	einoschema "github.com/cloudwego/eino/schema"
	"github.com/eino-contrib/jsonschema"
	"github.com/google/uuid"
)

const (
	// RemoteToolContractVersion is the current HTTP execution envelope version.
	// RemoteToolContractVersion 是当前 HTTP 执行信封版本。
	RemoteToolContractVersion = "remote_tool_execution.v1"

	defaultRemoteToolTimeoutMS       = 5000
	defaultRemoteToolMaxResponseSize = 1 << 20
	maxRemoteToolTimeoutMS           = 30000
	maxRemoteToolRetries             = 3
)

// RemoteRegistration describes one app-owned tool without storing credentials.
// RemoteRegistration 描述一条不保存凭证的应用自有工具注册。
type RemoteRegistration struct {
	RegistrationID   string         `json:"registration_id"`
	AppID            string         `json:"app_id"`
	Name             string         `json:"name"`
	Description      string         `json:"description,omitempty"`
	Parameters       map[string]any `json:"parameters,omitempty"`
	Endpoint         string         `json:"endpoint"`
	Auth             *RemoteAuth    `json:"auth,omitempty"`
	ToolScope        string         `json:"tool_scope,omitempty"`
	Operation        string         `json:"operation,omitempty"`
	RiskLevel        string         `json:"risk_level,omitempty"`
	SideEffectLevel  string         `json:"side_effect_level,omitempty"`
	Idempotent       bool           `json:"idempotent,omitempty"`
	SandboxRef       string         `json:"sandbox_ref,omitempty"`
	TimeoutMS        int            `json:"timeout_ms,omitempty"`
	RetryMaxAttempts int            `json:"retry_max_attempts,omitempty"`
	Enabled          bool           `json:"enabled"`
	Metadata         map[string]any `json:"metadata,omitempty"`
	CreatedAt        string         `json:"created_at,omitempty"`
	UpdatedAt        string         `json:"updated_at,omitempty"`
}

// RemoteAuth declares how Athena authenticates one outbound callback without storing the credential.
// RemoteAuth 声明 Athena 如何鉴权一次出站回调，但不保存凭据本身。
type RemoteAuth struct {
	Type       string `json:"type"`
	SecretRef  string `json:"secret_ref"`
	HeaderName string `json:"header_name,omitempty"`
}

// RemoteResolvedSecret is the short-lived runtime value returned by a secret provider.
// RemoteResolvedSecret 是 secret provider 在运行时返回的短生命周期值。
type RemoteResolvedSecret struct {
	Value     string
	ExpiresAt time.Time
	Revoked   bool
}

// RemoteSecretResolver resolves an opaque secret reference immediately before network I/O.
// RemoteSecretResolver 在网络 I/O 前即时解析不透明的 secret reference。
type RemoteSecretResolver func(context.Context, string) (RemoteResolvedSecret, error)

// RemoteExecutionRequest is sent to the registered business application endpoint.
// RemoteExecutionRequest 会发送到已注册的业务应用 endpoint。
type RemoteExecutionRequest struct {
	ContractVersion string          `json:"contract_version"`
	RequestID       string          `json:"request_id"`
	ToolCallID      string          `json:"tool_call_id"`
	RegistrationID  string          `json:"registration_id"`
	AppID           string          `json:"app_id"`
	ToolName        string          `json:"tool_name"`
	Arguments       json.RawMessage `json:"arguments"`
	Attempt         int             `json:"attempt"`
	Metadata        map[string]any  `json:"metadata,omitempty"`
}

// RemoteExecutionError is the normalized error shape returned by a business tool.
// RemoteExecutionError 是业务工具返回的标准化错误结构。
type RemoteExecutionError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable,omitempty"`
}

// Error returns the stable remote execution error string.
// Error 返回稳定的远程执行错误文本。
func (e *RemoteExecutionError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("%s: %s", strings.TrimSpace(e.Code), strings.TrimSpace(e.Message))
}

// RemoteExecutionResponse correlates a remote result with the originating tool call.
// RemoteExecutionResponse 将远程结果关联到原始工具调用。
type RemoteExecutionResponse struct {
	ContractVersion string                `json:"contract_version"`
	RequestID       string                `json:"request_id,omitempty"`
	ToolCallID      string                `json:"tool_call_id"`
	Status          string                `json:"status"`
	Content         string                `json:"content,omitempty"`
	Error           *RemoteExecutionError `json:"error,omitempty"`
	Metadata        map[string]any        `json:"metadata,omitempty"`
}

// RemoteGovernanceRequest contains safe metadata for the pre-execution policy gate.
// RemoteGovernanceRequest 包含执行前策略门禁使用的安全元数据。
type RemoteGovernanceRequest struct {
	ToolName     string
	ToolScope    string
	Operation    string
	RiskLevel    string
	ToolCallID   string
	AppID        string
	ArgumentKeys []string
	Metadata     map[string]any
}

// RemoteGovernanceDecision is the minimal decision consumed by the HTTP adapter.
// RemoteGovernanceDecision 是 HTTP 适配器消费的最小治理判定。
type RemoteGovernanceDecision struct {
	DecisionID   string
	Decision     string
	Reason       string
	RedactFields []string
	SandboxRef   string
}

// RemoteGovernanceEvaluator evaluates one call before any remote network request.
// RemoteGovernanceEvaluator 在任何远程网络请求前判定一次调用。
type RemoteGovernanceEvaluator func(context.Context, RemoteGovernanceRequest) (RemoteGovernanceDecision, error)

// RemoteInvocationEvent contains safe timing and decision metadata for observability.
// RemoteInvocationEvent 包含用于可观测性的安全计时与治理元数据。
type RemoteInvocationEvent struct {
	ToolCallID       string
	RegistrationID   string
	AppID            string
	ToolName         string
	EndpointOrigin   string
	Attempt          int
	Status           string
	DurationMS       int64
	DecisionID       string
	GovernanceResult string
	AuthType         string
	SecretRef        string
	AuthResult       string
	ErrorCode        string
}

// RemoteInvocationObserver receives safe remote invocation lifecycle records.
// RemoteInvocationObserver 接收安全的远程调用生命周期记录。
type RemoteInvocationObserver func(context.Context, RemoteInvocationEvent)

// RemoteToolOptions configures network policy and runtime callbacks.
// RemoteToolOptions 配置网络策略与运行时回调。
type RemoteToolOptions struct {
	AllowedOrigins   []string
	MaxResponseBytes int64
	HTTPClient       *http.Client
	Evaluate         RemoteGovernanceEvaluator
	ResolveSecret    RemoteSecretResolver
	Observe          RemoteInvocationObserver
}

type remoteTool struct {
	registration  RemoteRegistration
	maxResponse   int64
	client        *http.Client
	evaluate      RemoteGovernanceEvaluator
	resolveSecret RemoteSecretResolver
	observe       RemoteInvocationObserver
}

// NewRemoteDefinition validates one registration and creates an executable catalog definition.
// NewRemoteDefinition 校验注册并创建可执行的目录定义。
func NewRemoteDefinition(registration RemoteRegistration, options RemoteToolOptions) (Definition, error) {
	registration = normalizeRemoteRegistration(registration)
	if err := ValidateRemoteRegistration(registration, options.AllowedOrigins); err != nil {
		return Definition{}, err
	}
	maxResponse := options.MaxResponseBytes
	if maxResponse <= 0 {
		maxResponse = defaultRemoteToolMaxResponseSize
	}
	client := options.HTTPClient
	if client == nil {
		client = &http.Client{}
	}
	safeClient := *client
	safeClient.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	baseTool := &remoteTool{
		registration:  registration,
		maxResponse:   maxResponse,
		client:        &safeClient,
		evaluate:      options.Evaluate,
		resolveSecret: options.ResolveSecret,
		observe:       options.Observe,
	}
	return Definition{
		Name:                 registration.Name,
		Description:          registration.Description,
		BaseTool:             baseTool,
		ToolScope:            registration.ToolScope,
		RequiresConfirmation: !strings.EqualFold(registration.SideEffectLevel, "none"),
		SideEffectLevel:      registration.SideEffectLevel,
		InputSchemaSummary:   "remote JSON schema",
		OutputSchemaSummary:  "remote_tool_execution.v1 content",
	}, nil
}

// ValidateRemoteRegistration enforces schema, endpoint, timeout, retry, and idempotency rules.
// ValidateRemoteRegistration 强制校验 schema、endpoint、超时、重试与幂等规则。
func ValidateRemoteRegistration(registration RemoteRegistration, allowedOrigins []string) error {
	if strings.TrimSpace(registration.RegistrationID) == "" {
		return fmt.Errorf("registration_id is required")
	}
	if strings.TrimSpace(registration.AppID) == "" {
		return fmt.Errorf("app_id is required")
	}
	if err := validateRemoteToolName(registration.Name); err != nil {
		return err
	}
	if len(registration.Parameters) > 0 && strings.TrimSpace(remoteStringValue(registration.Parameters["type"])) != "object" {
		return fmt.Errorf("remote tool %q parameters must use an object JSON schema", registration.Name)
	}
	origin, err := remoteEndpointOrigin(registration.Endpoint)
	if err != nil {
		return err
	}
	if _, ok := normalizedOriginSet(allowedOrigins)[origin]; !ok {
		return fmt.Errorf("remote tool endpoint origin %q is not allowed", origin)
	}
	if registration.TimeoutMS <= 0 || registration.TimeoutMS > maxRemoteToolTimeoutMS {
		return fmt.Errorf("timeout_ms must be between 1 and %d", maxRemoteToolTimeoutMS)
	}
	if registration.RetryMaxAttempts < 0 || registration.RetryMaxAttempts > maxRemoteToolRetries {
		return fmt.Errorf("retry_max_attempts must be between 0 and %d", maxRemoteToolRetries)
	}
	if registration.RetryMaxAttempts > 0 && !registration.Idempotent && !strings.EqualFold(registration.SideEffectLevel, "none") {
		return fmt.Errorf("non-idempotent side-effecting remote tool %q cannot enable retries", registration.Name)
	}
	if err := validateRemoteAuth(registration.Auth); err != nil {
		return err
	}
	return nil
}

func normalizeRemoteRegistration(registration RemoteRegistration) RemoteRegistration {
	registration.RegistrationID = strings.TrimSpace(registration.RegistrationID)
	registration.AppID = strings.TrimSpace(registration.AppID)
	registration.Name = strings.TrimSpace(registration.Name)
	registration.Description = strings.TrimSpace(registration.Description)
	registration.Endpoint = strings.TrimSpace(registration.Endpoint)
	registration.ToolScope = strings.TrimSpace(registration.ToolScope)
	registration.Operation = strings.TrimSpace(registration.Operation)
	registration.RiskLevel = strings.TrimSpace(registration.RiskLevel)
	registration.SideEffectLevel = strings.TrimSpace(registration.SideEffectLevel)
	registration.SandboxRef = strings.TrimSpace(registration.SandboxRef)
	if registration.Auth != nil {
		auth := *registration.Auth
		auth.Type = strings.ToLower(strings.TrimSpace(auth.Type))
		auth.SecretRef = strings.TrimSpace(auth.SecretRef)
		auth.HeaderName = http.CanonicalHeaderKey(strings.TrimSpace(auth.HeaderName))
		registration.Auth = &auth
	}
	if registration.TimeoutMS <= 0 {
		registration.TimeoutMS = defaultRemoteToolTimeoutMS
	}
	if registration.SideEffectLevel == "" {
		registration.SideEffectLevel = "none"
	}
	return registration
}

func (t *remoteTool) Info(context.Context) (*einoschema.ToolInfo, error) {
	info := &einoschema.ToolInfo{
		Name: t.registration.Name,
		Desc: t.registration.Description,
		Extra: map[string]any{
			"registration_id": t.registration.RegistrationID,
			"app_id":          t.registration.AppID,
			"remote":          true,
		},
	}
	if len(t.registration.Parameters) == 0 {
		return info, nil
	}
	payload, err := json.Marshal(t.registration.Parameters)
	if err != nil {
		return nil, err
	}
	var parameters jsonschema.Schema
	if err := json.Unmarshal(payload, &parameters); err != nil {
		return nil, err
	}
	info.ParamsOneOf = einoschema.NewParamsOneOfByJSONSchema(&parameters)
	return info, nil
}

func (t *remoteTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...einotool.Option) (string, error) {
	if t == nil {
		return "", &RemoteExecutionError{Code: "remote_tool_unconfigured", Message: "remote tool is not configured"}
	}
	arguments, argumentKeys, err := parseRemoteArguments(argumentsInJSON)
	if err != nil {
		return "", err
	}
	callID := strings.TrimSpace(compose.GetToolCallID(ctx))
	if callID == "" {
		callID = "call_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	}
	decision := RemoteGovernanceDecision{Decision: "allow"}
	if t.evaluate != nil {
		decision, err = t.evaluate(ctx, RemoteGovernanceRequest{
			ToolName:     t.registration.Name,
			ToolScope:    t.registration.ToolScope,
			Operation:    t.registration.Operation,
			RiskLevel:    t.registration.RiskLevel,
			ToolCallID:   callID,
			AppID:        t.registration.AppID,
			ArgumentKeys: argumentKeys,
			Metadata:     cloneRemoteMap(t.registration.Metadata),
		})
		if err != nil {
			normalized := &RemoteExecutionError{Code: "governance_evaluation_failed", Message: err.Error()}
			t.emitPreflight(ctx, callID, decision, normalized.Code)
			return "", normalized
		}
	}
	switch strings.ToLower(strings.TrimSpace(decision.Decision)) {
	case "allow":
	case "deny":
		normalized := &RemoteExecutionError{Code: "governance_denied", Message: defaultRemoteString(decision.Reason, "remote tool invocation denied")}
		t.emitPreflight(ctx, callID, decision, normalized.Code)
		return "", normalized
	case "require_sandbox_ref":
		if strings.TrimSpace(t.registration.SandboxRef) == "" || (strings.TrimSpace(decision.SandboxRef) != "" && decision.SandboxRef != t.registration.SandboxRef) {
			normalized := &RemoteExecutionError{Code: "sandbox_ref_required", Message: "remote tool registration does not satisfy the required sandbox reference"}
			t.emitPreflight(ctx, callID, decision, normalized.Code)
			return "", normalized
		}
	case "allow_with_redaction":
		redactRemoteFields(arguments, decision.RedactFields)
	default:
		normalized := &RemoteExecutionError{Code: "governance_invalid_decision", Message: "governance returned an unsupported decision"}
		t.emitPreflight(ctx, callID, decision, normalized.Code)
		return "", normalized
	}

	argumentPayload, err := json.Marshal(arguments)
	if err != nil {
		return "", &RemoteExecutionError{Code: "arguments_encode_failed", Message: err.Error()}
	}
	maxAttempts := t.registration.RetryMaxAttempts + 1
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		startedAt := time.Now()
		content, retryable, authResult, invokeErr := t.invokeOnce(ctx, callID, argumentPayload, attempt, decision)
		event := RemoteInvocationEvent{
			ToolCallID:       callID,
			RegistrationID:   t.registration.RegistrationID,
			AppID:            t.registration.AppID,
			ToolName:         t.registration.Name,
			EndpointOrigin:   remoteOriginOrEmpty(t.registration.Endpoint),
			Attempt:          attempt,
			Status:           "ok",
			DurationMS:       time.Since(startedAt).Milliseconds(),
			DecisionID:       decision.DecisionID,
			GovernanceResult: decision.Decision,
			AuthType:         remoteAuthType(t.registration.Auth),
			SecretRef:        remoteSecretRef(t.registration.Auth),
			AuthResult:       authResult,
		}
		if invokeErr == nil {
			t.emit(ctx, event)
			return content, nil
		}
		lastErr = invokeErr
		event.Status = "error"
		var normalized *RemoteExecutionError
		if errors.As(invokeErr, &normalized) {
			event.ErrorCode = normalized.Code
		}
		t.emit(ctx, event)
		if !retryable || attempt == maxAttempts {
			break
		}
		if err := waitRemoteRetry(ctx, attempt); err != nil {
			return "", err
		}
	}
	return "", lastErr
}

func (t *remoteTool) invokeOnce(ctx context.Context, callID string, arguments json.RawMessage, attempt int, decision RemoteGovernanceDecision) (string, bool, string, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(t.registration.TimeoutMS)*time.Millisecond)
	defer cancel()
	authResult := "not_attempted"
	requestID := "remote_req_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	requestPayload, err := json.Marshal(RemoteExecutionRequest{
		ContractVersion: RemoteToolContractVersion,
		RequestID:       requestID,
		ToolCallID:      callID,
		RegistrationID:  t.registration.RegistrationID,
		AppID:           t.registration.AppID,
		ToolName:        t.registration.Name,
		Arguments:       arguments,
		Attempt:         attempt,
		Metadata: map[string]any{
			"governance_decision_id": decision.DecisionID,
			"sandbox_ref":            t.registration.SandboxRef,
		},
	})
	if err != nil {
		return "", false, authResult, &RemoteExecutionError{Code: "request_encode_failed", Message: err.Error()}
	}
	request, err := http.NewRequestWithContext(timeoutCtx, http.MethodPost, t.registration.Endpoint, bytes.NewReader(requestPayload))
	if err != nil {
		return "", false, authResult, &RemoteExecutionError{Code: "request_build_failed", Message: err.Error()}
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	headerName, headerValue, resolvedAuthResult, authErr := t.resolveRemoteAuth(timeoutCtx)
	authResult = resolvedAuthResult
	if authErr != nil {
		return "", false, authResult, authErr
	}
	if headerName != "" {
		request.Header.Set(headerName, headerValue)
	}
	response, err := t.client.Do(request)
	if err != nil {
		code := "remote_network_error"
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(timeoutCtx.Err(), context.DeadlineExceeded) {
			code = "remote_timeout"
		}
		return "", true, authResult, &RemoteExecutionError{Code: code, Message: err.Error(), Retryable: true}
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(response.Body, t.maxResponse+1))
	if err != nil {
		return "", true, authResult, &RemoteExecutionError{Code: "response_read_failed", Message: err.Error(), Retryable: true}
	}
	if int64(len(payload)) > t.maxResponse {
		return "", false, authResult, &RemoteExecutionError{Code: "response_too_large", Message: "remote tool response exceeded the configured limit"}
	}
	var result RemoteExecutionResponse
	if err := json.Unmarshal(payload, &result); err != nil {
		return "", response.StatusCode >= http.StatusInternalServerError, authResult, &RemoteExecutionError{Code: "invalid_remote_response", Message: err.Error(), Retryable: response.StatusCode >= http.StatusInternalServerError}
	}
	if strings.TrimSpace(result.ContractVersion) != RemoteToolContractVersion {
		return "", false, authResult, &RemoteExecutionError{Code: "contract_version_mismatch", Message: "remote tool response contract_version mismatch"}
	}
	if strings.TrimSpace(result.ToolCallID) != callID {
		return "", false, authResult, &RemoteExecutionError{Code: "tool_call_id_mismatch", Message: "remote tool response tool_call_id mismatch"}
	}
	if strings.TrimSpace(result.RequestID) != requestID {
		return "", false, authResult, &RemoteExecutionError{Code: "request_id_mismatch", Message: "remote tool response request_id mismatch"}
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices || !strings.EqualFold(strings.TrimSpace(result.Status), "ok") {
		if result.Error == nil {
			result.Error = &RemoteExecutionError{
				Code:      "remote_http_error",
				Message:   fmt.Sprintf("remote tool returned HTTP %d", response.StatusCode),
				Retryable: response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= http.StatusInternalServerError,
			}
		}
		return "", result.Error.Retryable, authResult, result.Error
	}
	return result.Content, false, authResult, nil
}

func (t *remoteTool) resolveRemoteAuth(ctx context.Context) (string, string, string, error) {
	if t.registration.Auth == nil {
		return "", "", "not_configured", nil
	}
	if t.resolveSecret == nil {
		return "", "", "unavailable", remoteAuthError("remote_auth_secret_unavailable", "remote authentication secret is unavailable")
	}
	secret, err := t.resolveSecret(ctx, t.registration.Auth.SecretRef)
	if err != nil {
		return "", "", "unavailable", remoteAuthError("remote_auth_secret_unavailable", "remote authentication secret is unavailable")
	}
	if secret.Revoked {
		return "", "", "revoked", remoteAuthError("remote_auth_secret_revoked", "remote authentication secret is revoked")
	}
	if !secret.ExpiresAt.IsZero() && !time.Now().Before(secret.ExpiresAt) {
		return "", "", "expired", remoteAuthError("remote_auth_secret_expired", "remote authentication secret is expired")
	}
	if strings.TrimSpace(secret.Value) == "" {
		return "", "", "unavailable", remoteAuthError("remote_auth_secret_unavailable", "remote authentication secret is unavailable")
	}
	if !validRemoteHeaderValue(secret.Value) {
		return "", "", "invalid", remoteAuthError("remote_auth_secret_invalid", "remote authentication secret is invalid")
	}
	if t.registration.Auth.Type == "bearer" {
		return "Authorization", "Bearer " + secret.Value, "injected", nil
	}
	return t.registration.Auth.HeaderName, secret.Value, "injected", nil
}

func remoteAuthError(code, message string) *RemoteExecutionError {
	return &RemoteExecutionError{Code: code, Message: message}
}

func (t *remoteTool) emit(ctx context.Context, event RemoteInvocationEvent) {
	if t.observe != nil {
		t.observe(ctx, event)
	}
}

func (t *remoteTool) emitPreflight(ctx context.Context, callID string, decision RemoteGovernanceDecision, errorCode string) {
	t.emit(ctx, RemoteInvocationEvent{
		ToolCallID:       callID,
		RegistrationID:   t.registration.RegistrationID,
		AppID:            t.registration.AppID,
		ToolName:         t.registration.Name,
		EndpointOrigin:   remoteOriginOrEmpty(t.registration.Endpoint),
		Status:           "error",
		DecisionID:       decision.DecisionID,
		GovernanceResult: decision.Decision,
		AuthType:         remoteAuthType(t.registration.Auth),
		SecretRef:        remoteSecretRef(t.registration.Auth),
		AuthResult:       "not_attempted",
		ErrorCode:        errorCode,
	})
}

func validateRemoteAuth(auth *RemoteAuth) error {
	if auth == nil {
		return nil
	}
	if auth.SecretRef == "" {
		return fmt.Errorf("remote tool auth secret_ref is required")
	}
	if len(auth.SecretRef) > 256 {
		return fmt.Errorf("remote tool auth secret_ref exceeds 256 characters")
	}
	parsedRef, err := url.Parse(auth.SecretRef)
	if err != nil || parsedRef.Scheme == "" || parsedRef.Host == "" || parsedRef.User != nil || parsedRef.RawQuery != "" || parsedRef.Fragment != "" {
		return fmt.Errorf("remote tool auth secret_ref must be an opaque provider reference without user info, query, or fragment")
	}
	if strings.EqualFold(parsedRef.Scheme, "env") && (parsedRef.Path != "" || parsedRef.Port() != "" || !validRemoteEnvName(parsedRef.Host)) {
		return fmt.Errorf("remote tool env secret_ref must use env://UPPER_CASE_VARIABLE_NAME")
	}
	switch auth.Type {
	case "bearer":
		if auth.HeaderName != "" {
			return fmt.Errorf("remote tool bearer auth cannot set header_name")
		}
	case "header":
		if !validRemoteAuthHeaderName(auth.HeaderName) {
			return fmt.Errorf("remote tool header auth requires a valid X-* header_name")
		}
	default:
		return fmt.Errorf("remote tool auth type must be bearer or header")
	}
	return nil
}

func validRemoteEnvName(value string) bool {
	if value == "" || !((value[0] >= 'A' && value[0] <= 'Z') || value[0] == '_') {
		return false
	}
	for index := 1; index < len(value); index++ {
		if (value[index] >= 'A' && value[index] <= 'Z') || (value[index] >= '0' && value[index] <= '9') || value[index] == '_' {
			continue
		}
		return false
	}
	return true
}

func validRemoteAuthHeaderName(value string) bool {
	if len(value) <= 2 || !strings.EqualFold(value[:2], "X-") {
		return false
	}
	for index := 2; index < len(value); index++ {
		if (value[index] >= 'a' && value[index] <= 'z') || (value[index] >= 'A' && value[index] <= 'Z') || (value[index] >= '0' && value[index] <= '9') || value[index] == '-' {
			continue
		}
		return false
	}
	return true
}

func validRemoteHeaderValue(value string) bool {
	for index := 0; index < len(value); index++ {
		if value[index] < 32 || value[index] == 127 {
			return false
		}
	}
	return true
}

func remoteAuthType(auth *RemoteAuth) string {
	if auth == nil {
		return "none"
	}
	return auth.Type
}

func remoteSecretRef(auth *RemoteAuth) string {
	if auth == nil {
		return ""
	}
	return auth.SecretRef
}

func parseRemoteArguments(raw string) (map[string]any, []string, error) {
	var arguments map[string]any
	if err := json.Unmarshal([]byte(raw), &arguments); err != nil {
		return nil, nil, &RemoteExecutionError{Code: "invalid_arguments", Message: "tool arguments must be a JSON object"}
	}
	if arguments == nil {
		return nil, nil, &RemoteExecutionError{Code: "invalid_arguments", Message: "tool arguments must be a JSON object"}
	}
	keys := make([]string, 0, len(arguments))
	for key := range arguments {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return arguments, keys, nil
}

func redactRemoteFields(arguments map[string]any, paths []string) {
	for _, path := range paths {
		segments := strings.Split(strings.TrimSpace(path), ".")
		current := arguments
		for index, segment := range segments {
			segment = strings.TrimSpace(segment)
			if segment == "" {
				break
			}
			if index == len(segments)-1 {
				if _, ok := current[segment]; ok {
					current[segment] = "[redacted]"
				}
				break
			}
			next, ok := current[segment].(map[string]any)
			if !ok {
				break
			}
			current = next
		}
	}
}

func waitRemoteRetry(ctx context.Context, attempt int) error {
	timer := time.NewTimer(time.Duration(attempt*25) * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func validateRemoteToolName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("remote tool name is required")
	}
	if len(name) > 64 {
		return fmt.Errorf("remote tool name exceeds 64 characters")
	}
	for _, char := range name {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '_' || char == '-' {
			continue
		}
		return fmt.Errorf("remote tool name %q contains unsupported characters", name)
	}
	return nil
}

func remoteEndpointOrigin(endpoint string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil || parsed == nil {
		return "", fmt.Errorf("remote tool endpoint is invalid")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("remote tool endpoint must use http or https")
	}
	if parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" {
		return "", fmt.Errorf("remote tool endpoint must have a host and no user info or fragment")
	}
	return strings.ToLower(parsed.Scheme + "://" + parsed.Host), nil
}

func normalizedOriginSet(origins []string) map[string]struct{} {
	result := make(map[string]struct{}, len(origins))
	for _, candidate := range origins {
		origin, err := remoteEndpointOrigin(candidate)
		if err != nil {
			continue
		}
		result[origin] = struct{}{}
	}
	return result
}

func remoteOriginOrEmpty(endpoint string) string {
	origin, _ := remoteEndpointOrigin(endpoint)
	return origin
}

func cloneRemoteMap(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	output := make(map[string]any, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

func remoteStringValue(value any) string {
	typed, _ := value.(string)
	return typed
}

func defaultRemoteString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}
