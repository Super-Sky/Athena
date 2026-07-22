// privileged_trace_payload.go exposes fail-closed, audited control-plane reads for encrypted trace details.
// privileged_trace_payload.go 为加密 trace 明细提供 fail-closed、可审计的控制面读取入口。
package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/google/uuid"
	appcore "moss/internal/app"
	"moss/internal/config"
	"moss/internal/controlplane"
	"moss/internal/runtime"
)

type privilegedTracePayloadResponse struct {
	PayloadRef         string         `json:"payload_ref"`
	RunID              string         `json:"run_id"`
	StepID             string         `json:"step_id,omitempty"`
	TraceType          string         `json:"trace_type"`
	Source             string         `json:"source"`
	SchemaVersion      string         `json:"schema_version"`
	Payload            map[string]any `json:"payload"`
	PayloadSize        int            `json:"payload_size"`
	RedactedFieldCount int            `json:"redacted_field_count"`
	CreatedAt          time.Time      `json:"created_at"`
	ExpiresAt          time.Time      `json:"expires_at"`
}

type privilegedTraceAuditContext struct {
	RunID            string
	PayloadRef       string
	RequestID        string
	ActorSessionHash string
	RemoteIP         string
}

func withPrivilegedTracePayloadReadAuth(cfg config.Config, application *appcore.Service, next controlPlaneHandler) controlPlaneHandler {
	return func(ctx context.Context, c *hertzapp.RequestContext, cfg config.Config, application *appcore.Service) {
		applyControlPlaneCORS(c, cfg)
		c.Response.Header.Set("Cache-Control", "no-store")
		if string(c.Method()) == http.MethodOptions {
			c.Status(consts.StatusNoContent)
			return
		}
		audit := privilegedTraceAuditContextFromRequest(c)
		if !cfg.Observability.PrivilegedTracePayload.ReadEnabled {
			if err := recordPrivilegedTraceAudit(application, audit, "denied", "read_disabled", 0, 0); err != nil {
				c.JSON(consts.StatusServiceUnavailable, map[string]string{"error": "privileged trace audit unavailable"})
				return
			}
			c.JSON(consts.StatusForbidden, map[string]string{"error": "privileged trace payload reads are disabled"})
			return
		}
		if application == nil || application.ControlPlane == nil || strings.TrimSpace(cfg.ControlPlane.AuthToken) == "" {
			_ = recordPrivilegedTraceAudit(application, audit, "denied", "control_plane_auth_unavailable", 0, 0)
			c.JSON(consts.StatusForbidden, map[string]string{"error": "privileged trace payload access is unavailable"})
			return
		}
		status, err := application.ControlPlane.Authorize(ctx, string(c.Cookie(controlplane.ControlPlaneSessionCookie)), c.ClientIP())
		if err != nil {
			outcome, reason, code := "denied", "authentication_required", consts.StatusUnauthorized
			if errors.Is(err, controlplane.ErrControlPlaneAuthLocked) {
				reason, code = "authentication_locked", consts.StatusLocked
			}
			if auditErr := recordPrivilegedTraceAudit(application, audit, outcome, reason, 0, 0); auditErr != nil {
				c.JSON(consts.StatusServiceUnavailable, map[string]string{"error": "privileged trace audit unavailable"})
				return
			}
			c.JSON(code, status)
			return
		}
		next(ctx, c, cfg, application)
	}
}

func handleGetPrivilegedTracePayload(ctx context.Context, c *hertzapp.RequestContext, _ config.Config, application *appcore.Service) {
	c.Response.Header.Set("Cache-Control", "no-store")
	audit := privilegedTraceAuditContextFromRequest(c)
	readout, found, err := application.GetPrivilegedTracePayload(ctx, audit.RunID, audit.PayloadRef)
	if err != nil {
		if auditErr := recordPrivilegedTraceAudit(application, audit, "error", "read_or_decrypt_failed", 0, 0); auditErr != nil {
			c.JSON(consts.StatusServiceUnavailable, map[string]string{"error": "privileged trace audit unavailable"})
			return
		}
		c.JSON(consts.StatusInternalServerError, map[string]string{"error": "privileged trace payload could not be read"})
		return
	}
	if !found {
		if auditErr := recordPrivilegedTraceAudit(application, audit, "not_found", "run_or_payload_mismatch", 0, 0); auditErr != nil {
			c.JSON(consts.StatusServiceUnavailable, map[string]string{"error": "privileged trace audit unavailable"})
			return
		}
		c.JSON(consts.StatusNotFound, map[string]string{"error": "privileged trace payload not found"})
		return
	}
	if !readout.ExpiresAt.After(time.Now().UTC()) {
		if auditErr := recordPrivilegedTraceAudit(application, audit, "expired", "retention_expired", 0, readout.RedactedFieldCount); auditErr != nil {
			c.JSON(consts.StatusServiceUnavailable, map[string]string{"error": "privileged trace audit unavailable"})
			return
		}
		if err := application.PrivilegedTraceStore.DeletePrivilegedTracePayload(ctx, readout.PayloadRef); err != nil {
			c.JSON(consts.StatusServiceUnavailable, map[string]string{"error": "expired privileged trace payload could not be removed"})
			return
		}
		c.JSON(consts.StatusGone, map[string]string{"error": "privileged trace payload expired"})
		return
	}
	response := privilegedTracePayloadResponse{
		PayloadRef: readout.PayloadRef, RunID: readout.RunID, StepID: readout.StepID, TraceType: readout.TraceType,
		Source: readout.Source, SchemaVersion: readout.SchemaVersion, Payload: readout.Payload, PayloadSize: readout.PayloadSize,
		RedactedFieldCount: readout.RedactedFieldCount, CreatedAt: readout.CreatedAt, ExpiresAt: readout.ExpiresAt,
	}
	payload, err := json.Marshal(response)
	if err != nil {
		_ = recordPrivilegedTraceAudit(application, audit, "error", "response_encode_failed", 0, readout.RedactedFieldCount)
		c.JSON(consts.StatusInternalServerError, map[string]string{"error": "privileged trace payload could not be encoded"})
		return
	}
	if err := recordPrivilegedTraceAudit(application, audit, "success", "control_plane_trace_inspection", len(payload), readout.RedactedFieldCount); err != nil {
		c.JSON(consts.StatusServiceUnavailable, map[string]string{"error": "privileged trace audit unavailable"})
		return
	}
	c.Data(consts.StatusOK, "application/json; charset=utf-8", payload)
}

func privilegedTraceAuditContextFromRequest(c *hertzapp.RequestContext) privilegedTraceAuditContext {
	requestID := strings.TrimSpace(string(c.GetHeader("X-Request-ID")))
	if requestID == "" {
		requestID = uuid.NewString()
	}
	session := strings.TrimSpace(string(c.Cookie(controlplane.ControlPlaneSessionCookie)))
	digest := sha256.Sum256([]byte(session))
	return privilegedTraceAuditContext{
		RunID: strings.TrimSpace(c.Param("runID")), PayloadRef: strings.TrimSpace(c.Param("payloadRef")), RequestID: requestID,
		ActorSessionHash: hex.EncodeToString(digest[:]), RemoteIP: c.ClientIP(),
	}
}

func recordPrivilegedTraceAudit(application *appcore.Service, input privilegedTraceAuditContext, outcome, reason string, returnedBytes, redactedFieldCount int) error {
	if application == nil {
		return appcore.ErrRuntimeStoreNotConfigured
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return application.RecordPrivilegedTracePayloadAccess(ctx, runtime.PrivilegedTracePayloadAccessAudit{
		RunID: input.RunID, PayloadRef: input.PayloadRef, RequestID: input.RequestID, ActorSessionHash: input.ActorSessionHash,
		RemoteIP: input.RemoteIP, Outcome: outcome, Reason: reason, ReturnedBytes: returnedBytes,
		RedactedFieldCount: redactedFieldCount, AccessedAt: time.Now().UTC(),
	})
}
