// hook_bridge.go maps internal HookBinding records into safe runtime graph projections.
// hook_bridge.go 将内部 HookBinding 记录映射为安全的 runtime graph 投影。
package runtime

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// RuntimeHookBridge projects allowlisted runtime hooks through Athena persistence records.
// RuntimeHookBridge 通过 Athena 持久化记录投影 allowlisted runtime hooks。
type RuntimeHookBridge struct {
	Store      RuntimePersistenceStore
	HookStore  HookBindingStore
	Now        func() time.Time
	RecordSet  *MinimalPersistenceRecordSet
	ContractID string
}

// ProjectHookPoint records enabled hook bindings for one lifecycle hook point.
// ProjectHookPoint 记录一个生命周期 hook point 下启用的 hook bindings。
func (b RuntimeHookBridge) ProjectHookPoint(ctx context.Context, hookPoint string) (int, error) {
	if b.Store == nil || b.HookStore == nil || b.RecordSet == nil || strings.TrimSpace(b.RecordSet.Run.ID) == "" || strings.TrimSpace(b.ContractID) == "" {
		return 0, nil
	}
	enabled := true
	hooks, err := b.HookStore.ListHookBindings(ctx, HookBindingListFilter{ContractID: b.ContractID, HookPoint: hookPoint, Enabled: &enabled})
	if err != nil {
		return 0, err
	}
	if len(hooks) == 0 {
		return 0, nil
	}
	now := time.Now().UTC()
	if b.Now != nil {
		now = b.Now().UTC()
	}
	for idx, hook := range hooks {
		if err := b.projectHook(ctx, hook, now.Add(time.Duration(idx)*time.Millisecond)); err != nil {
			return idx, err
		}
	}
	return len(hooks), nil
}

func (b RuntimeHookBridge) projectHook(ctx context.Context, hook HookBinding, observedAt time.Time) error {
	runID := b.RecordSet.Run.ID
	stepID := b.RecordSet.Step.ID
	metadata := map[string]any{
		"contract_id":    b.ContractID,
		"hook_id":        hook.ID,
		"hook_point":     hook.HookPoint,
		"binding_kind":   hook.BindingKind,
		"binding_ref":    hook.BindingRef,
		"failure_policy": hook.FailurePolicy,
		"projection":     "eino_hook_bridge",
	}
	if _, err := b.Store.CreateLifecycleEvent(ctx, TaskRunLifecycleEvent{
		RunID:       runID,
		StepID:      stepID,
		EventType:   "runtime_hook_observed",
		SubjectType: LifecycleSubjectStep,
		SubjectID:   stepID,
		FromStatus:  b.RecordSet.Step.Status,
		ToStatus:    b.RecordSet.Step.Status,
		Reason:      "allowlisted_runtime_hook_projected",
		Metadata:    metadata,
		OccurredAt:  observedAt,
	}); err != nil {
		return err
	}
	if _, err := b.Store.CreateRuntimeTrace(ctx, RuntimeTrace{
		RunID:     runID,
		StepID:    stepID,
		TraceType: "runtime_hook_binding",
		Summary:   fmt.Sprintf("runtime hook %s projected through %s", hook.HookPoint, hook.BindingKind),
		SafeLabels: map[string]string{
			"component":    "internal/runtime",
			"source":       "eino_hook_bridge",
			"hook_point":   hook.HookPoint,
			"binding_kind": hook.BindingKind,
		},
		RedactedPayload: map[string]any{
			"contract_id":  b.ContractID,
			"hook_id":      hook.ID,
			"binding_ref":  hook.BindingRef,
			"order_index":  hook.OrderIndex,
			"recorded_via": "eino_graph_node",
		},
		Metadata:  metadata,
		CreatedAt: observedAt.Add(time.Millisecond),
	}); err != nil {
		return err
	}
	_, err := b.Store.CreateUsage(ctx, Usage{
		RunID:        runID,
		StepID:       stepID,
		ResourceType: "runtime_hook",
		Provider:     "athena",
		ResourceName: hook.BindingRef,
		Unit:         "binding",
		Amount:       1,
		Metadata:     metadata,
		CreatedAt:    observedAt.Add(2 * time.Millisecond),
	})
	return err
}
