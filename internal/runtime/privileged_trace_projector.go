// privileged_trace_projector.go bridges request-local model/tool detail into the encrypted payload plane.
// privileged_trace_projector.go 把请求级 model/tool 明细接入加密 payload 平面。
package runtime

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

const privilegedTraceComponentContext = "context"

func projectPrivilegedTracePayload(
	ctx context.Context,
	store PrivilegedTracePayloadStore,
	policy PrivilegedTracePayloadPolicy,
	workspaceID string,
	component string,
	runID string,
	stepID string,
	traceID string,
	traceType string,
	source string,
	payload map[string]any,
	createdAt time.Time,
) map[string]any {
	status, reason := privilegedTraceCaptureEligibility(policy, workspaceID, component)
	metadata := map[string]any{"payload_status": status}
	if reason != "" {
		metadata["payload_unavailable_reason"] = reason
	}
	if status != "eligible" {
		return metadata
	}
	if store == nil {
		metadata["payload_status"] = "capture_failed"
		metadata["payload_unavailable_reason"] = "payload_store_unavailable"
		return metadata
	}
	record, buildStatus, err := BuildPrivilegedTracePayload(PrivilegedTracePayloadCoordinates{
		RunID: runID, StepID: stepID, TraceType: traceType, Source: source, CorrelationID: traceID,
	}, payload, policy, createdAt)
	if err != nil {
		metadata["payload_status"] = "capture_failed"
		metadata["payload_unavailable_reason"] = "payload_build_failed"
		return metadata
	}
	if buildStatus != "captured" {
		metadata["payload_status"] = buildStatus
		metadata["payload_unavailable_reason"] = buildStatus
		return metadata
	}
	if _, err := store.CreatePrivilegedTracePayload(ctx, record); err != nil {
		if errors.Is(err, ErrPrivilegedTracePayloadRunBudgetExceeded) {
			metadata["payload_status"] = "size_exceeded"
			metadata["payload_unavailable_reason"] = "run_size_exceeded"
			return metadata
		}
		metadata["payload_status"] = "capture_failed"
		metadata["payload_unavailable_reason"] = "payload_store_failed"
		return metadata
	}
	metadata["payload_status"] = "recorded"
	metadata["payload_ref"] = record.PayloadRef
	metadata["payload_expires_at"] = record.ExpiresAt.UTC().Format(time.RFC3339Nano)
	return metadata
}

func privilegedTraceCaptureEligibility(policy PrivilegedTracePayloadPolicy, workspaceID, component string) (string, string) {
	if !policy.Enabled {
		return "disabled", "capture_disabled"
	}
	if _, disabled := policy.DisabledWorkspaceIDs[strings.TrimSpace(workspaceID)]; disabled {
		return "disabled", "workspace_disabled"
	}
	switch strings.ToLower(strings.TrimSpace(component)) {
	case runtimeCallbackComponentModel:
		if !policy.CaptureModel {
			return "disabled", "model_capture_disabled"
		}
	case runtimeCallbackComponentTool:
		if !policy.CaptureTools {
			return "disabled", "tool_capture_disabled"
		}
	case privilegedTraceComponentContext:
		if !policy.CaptureContextSummary {
			return "disabled", "context_capture_disabled"
		}
	}
	return "eligible", ""
}

func privilegedContextAssemblyPayload(spec *ExecutionSpec) map[string]any {
	if spec == nil {
		return map[string]any{"status": "execution_spec_unavailable"}
	}
	manifest := BuildRunManifest(spec, time.Time{})
	constraintKeys := make([]string, 0, len(spec.Metadata.Constraints))
	for key := range spec.Metadata.Constraints {
		constraintKeys = append(constraintKeys, strings.TrimSpace(key))
	}
	sort.Strings(constraintKeys)
	return map[string]any{
		"skill": map[string]any{
			"primary":   spec.Skill.PrimarySkill,
			"auxiliary": append([]string(nil), spec.Skill.AuxiliarySkills...),
			"guidance":  spec.Skill.Guidance,
		},
		"context": map[string]any{
			"context_assets":  manifest.ContextAssets,
			"system_truth":    manifest.SystemTruth,
			"constraint_keys": constraintKeys,
		},
		"prompt_ref": manifest.Prompt,
		"status":     fmt.Sprint(manifest.Status),
	}
}
