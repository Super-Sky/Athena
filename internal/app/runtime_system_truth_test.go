package app

import (
	"context"
	"errors"
	"testing"

	"moss/internal/runtime"
)

// TestSystemTruthLifecycleWritePathAppendsAuditedRecords verifies the source-to-rollback lifecycle path.
// TestSystemTruthLifecycleWritePathAppendsAuditedRecords 验证 source 到 rollback 的生命周期写入路径。
func TestSystemTruthLifecycleWritePathAppendsAuditedRecords(t *testing.T) {
	t.Parallel()

	store := newRuntimeFoundationWriteTestStore()
	service := &Service{RuntimeStore: store}
	ctx := context.Background()

	source, err := service.CreateSystemTruthSource(ctx, runtime.SystemTruthSource{
		AssetID:    "persona.default",
		SourceKind: "operator_input",
		SourceRef:  "system-validation",
		Content:    map[string]any{"summary": "source v1"},
	})
	if err != nil {
		t.Fatalf("CreateSystemTruthSource() error = %v", err)
	}
	if source.Status != runtime.SystemTruthSourceStatusImported || source.ContentHash == "" {
		t.Fatalf("source = %#v, want imported source with content hash", source)
	}

	firstDraft, err := service.CreateSystemTruthDraft(ctx, runtime.SystemTruthDraft{
		SourceID:    source.ID,
		Author:      "operator",
		Reason:      "initial draft",
		Content:     map[string]any{"summary": "draft v1"},
		DiffSummary: "initial lifecycle draft",
	})
	if err != nil {
		t.Fatalf("CreateSystemTruthDraft(first) error = %v", err)
	}
	firstCompile, err := service.CompileSystemTruthDraft(ctx, firstDraft.ID, runtime.SystemTruthCompileResult{
		Summary:         "compiled v1",
		CompiledPayload: map[string]any{"summary": "compiled v1"},
	})
	if err != nil {
		t.Fatalf("CompileSystemTruthDraft(first) error = %v", err)
	}
	firstActive, err := service.ActivateSystemTruthCompileResult(ctx, firstCompile.ID, runtime.SystemTruthActiveVersion{
		ActivatedBy: "operator",
		Reason:      "activate v1",
	})
	if err != nil {
		t.Fatalf("ActivateSystemTruthCompileResult(first) error = %v", err)
	}

	secondDraft, err := service.CreateSystemTruthDraft(ctx, runtime.SystemTruthDraft{
		SourceID:     source.ID,
		Author:       "operator",
		Reason:       "edit truth",
		BaseActiveID: firstActive.ID,
		Content:      map[string]any{"summary": "draft v2"},
		DiffSummary:  "update summary",
	})
	if err != nil {
		t.Fatalf("CreateSystemTruthDraft(second) error = %v", err)
	}
	secondCompile, err := service.CompileSystemTruthDraft(ctx, secondDraft.ID, runtime.SystemTruthCompileResult{
		Summary:         "compiled v2",
		CompiledPayload: map[string]any{"summary": "compiled v2"},
	})
	if err != nil {
		t.Fatalf("CompileSystemTruthDraft(second) error = %v", err)
	}
	secondActive, err := service.ActivateSystemTruthCompileResult(ctx, secondCompile.ID, runtime.SystemTruthActiveVersion{
		ActivatedBy: "operator",
		Reason:      "activate v2",
	})
	if err != nil {
		t.Fatalf("ActivateSystemTruthCompileResult(second) error = %v", err)
	}
	if secondActive.RollbackFromID != "" {
		t.Fatalf("secondActive.RollbackFromID = %q, want empty", secondActive.RollbackFromID)
	}

	rollback, err := service.RollbackSystemTruthActiveVersion(ctx, firstActive.ID, runtime.SystemTruthActiveVersion{
		ActivatedBy: "operator",
		Reason:      "rollback to v1",
	})
	if err != nil {
		t.Fatalf("RollbackSystemTruthActiveVersion() error = %v", err)
	}
	if rollback.CompileResultID != firstCompile.ID || rollback.RollbackFromID != secondActive.ID {
		t.Fatalf("rollback = %#v, want target compile %q rollback_from %q", rollback, firstCompile.ID, secondActive.ID)
	}

	readout, err := service.GetSystemTruthLifecycleReadout(ctx, SystemTruthLifecycleReadQuery{AssetID: source.AssetID})
	if err != nil {
		t.Fatalf("GetSystemTruthLifecycleReadout() error = %v", err)
	}
	if len(readout.Sources) != 1 || len(readout.Drafts) != 2 || len(readout.CompileResults) != 2 || len(readout.ActiveVersions) != 3 {
		t.Fatalf("readout = %#v, want 1 source, 2 drafts, 2 compiles, 3 active versions", readout)
	}
}

// TestSystemTruthFailedCompileCannotActivate verifies failed compile records never become active pointers.
// TestSystemTruthFailedCompileCannotActivate 验证失败 compile 不能变成 active pointer。
func TestSystemTruthFailedCompileCannotActivate(t *testing.T) {
	t.Parallel()

	store := newRuntimeFoundationWriteTestStore()
	service := &Service{RuntimeStore: store}
	ctx := context.Background()
	source, err := service.CreateSystemTruthSource(ctx, runtime.SystemTruthSource{
		AssetID:    "persona.default",
		SourceKind: "operator_input",
		Content:    map[string]any{"summary": "source"},
	})
	if err != nil {
		t.Fatalf("CreateSystemTruthSource() error = %v", err)
	}
	draft, err := service.CreateSystemTruthDraft(ctx, runtime.SystemTruthDraft{
		SourceID: source.ID,
		Content:  map[string]any{"summary": "draft"},
	})
	if err != nil {
		t.Fatalf("CreateSystemTruthDraft() error = %v", err)
	}
	compiled, err := service.CompileSystemTruthDraft(ctx, draft.ID, runtime.SystemTruthCompileResult{
		Status:      runtime.SystemTruthCompileStatusFailed,
		Summary:     "compile failed",
		Diagnostics: map[string]any{"errors": []any{"invalid structure"}},
	})
	if err != nil {
		t.Fatalf("CompileSystemTruthDraft() error = %v", err)
	}
	_, err = service.ActivateSystemTruthCompileResult(ctx, compiled.ID, runtime.SystemTruthActiveVersion{
		ActivatedBy: "operator",
		Reason:      "should fail",
	})
	if !errors.Is(err, runtime.ErrInvalidRuntimePersistenceInput) {
		t.Fatalf("ActivateSystemTruthCompileResult() error = %v, want ErrInvalidRuntimePersistenceInput", err)
	}
}
