package runtime

import "testing"

func TestNormalizeProjectionCandidateFillsDefaultSchemaVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		candidateKind string
		wantVersion   string
	}{
		{name: "minimal_output", candidateKind: "minimal_output", wantVersion: ProjectionSchemaVersionMinimalOutput},
		{name: "prepared_execution", candidateKind: "prepared_execution", wantVersion: ProjectionSchemaVersionPreparedExecution},
		{name: "terminal_output", candidateKind: "terminal_output", wantVersion: ProjectionSchemaVersionTerminalOutput},
		{name: "validation_mcp_result", candidateKind: "validation_mcp_result", wantVersion: ProjectionSchemaVersionValidationMCP},
		{name: "external_sandbox_ref", candidateKind: "external_sandbox_ref", wantVersion: ProjectionSchemaVersionExternalSandboxRef},
		{name: "assistant_message", candidateKind: "assistant_message", wantVersion: ProjectionSchemaVersionAssistantMessage},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := normalizeProjectionCandidate(ProjectionCandidate{
				RunID:         "run-1",
				CandidateKind: tc.candidateKind,
				SchemaVersion: "",
			})
			if got.SchemaVersion != tc.wantVersion {
				t.Fatalf("schema version = %q, want %q", got.SchemaVersion, tc.wantVersion)
			}
		})
	}
}

func TestNormalizeProjectionCandidatePreservesExplicitSchemaVersion(t *testing.T) {
	t.Parallel()

	got := normalizeProjectionCandidate(ProjectionCandidate{
		RunID:         "run-1",
		CandidateKind: "minimal_output",
		SchemaVersion: "runtime_projection.custom.v9",
	})
	if got.SchemaVersion != "runtime_projection.custom.v9" {
		t.Fatalf("schema version = %q, want runtime_projection.custom.v9", got.SchemaVersion)
	}
}

func TestNormalizeProjectionCandidateUnknownKindKeepsEmptySchemaVersion(t *testing.T) {
	t.Parallel()

	got := normalizeProjectionCandidate(ProjectionCandidate{
		RunID:         "run-1",
		CandidateKind: "future_kind",
		SchemaVersion: "",
	})
	if got.SchemaVersion != "" {
		t.Fatalf("schema version = %q, want empty", got.SchemaVersion)
	}
}

func TestNormalizeProjectionCandidateDefaultsMaterializationBoundary(t *testing.T) {
	t.Parallel()

	got := normalizeProjectionCandidate(ProjectionCandidate{
		RunID:         "run-1",
		CandidateKind: "minimal_output",
	})
	if got.MaterializationTarget["core_materialization_scope"] != ProjectionMaterializationScopeCandidateOnly {
		t.Fatalf("materialization scope = %#v, want %q", got.MaterializationTarget, ProjectionMaterializationScopeCandidateOnly)
	}
	if got.MaterializationTarget["target_type"] != ProjectionMaterializationTargetReadModel {
		t.Fatalf("target type = %#v, want %q", got.MaterializationTarget, ProjectionMaterializationTargetReadModel)
	}
	if got.MaterializationTarget["ownership"] != ProjectionMaterializationOwnershipRuntime {
		t.Fatalf("ownership = %#v, want %q", got.MaterializationTarget, ProjectionMaterializationOwnershipRuntime)
	}
}

func TestValidateProjectionCandidateBoundary(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		projection ProjectionCandidate
		wantErr    bool
	}{
		{
			name: "accepts runtime candidate read model",
			projection: ProjectionCandidate{
				RunID:           "run-1",
				CandidateKind:   "assistant_message",
				SchemaVersion:   ProjectionSchemaVersionAssistantMessage,
				SemanticPayload: map[string]any{"kind": "assistant_message", "answer": "safe output"},
			},
		},
		{
			name: "rejects formal evidence candidate kind",
			projection: ProjectionCandidate{
				RunID:         "run-1",
				CandidateKind: "EvidenceRecord",
				SchemaVersion: "runtime_projection.evidence_record.v1",
			},
			wantErr: true,
		},
		{
			name: "rejects schema outside runtime projection namespace",
			projection: ProjectionCandidate{
				RunID:         "run-1",
				CandidateKind: "assistant_message",
				SchemaVersion: "business.evidence_record.v1",
			},
			wantErr: true,
		},
		{
			name: "rejects materialization target evidence record",
			projection: ProjectionCandidate{
				RunID:         "run-1",
				CandidateKind: "assistant_message",
				SchemaVersion: ProjectionSchemaVersionAssistantMessage,
				MaterializationTarget: map[string]any{
					"target_type": "EvidenceRecord",
				},
			},
			wantErr: true,
		},
		{
			name: "rejects materialization scope outside projection candidate",
			projection: ProjectionCandidate{
				RunID:         "run-1",
				CandidateKind: "assistant_message",
				SchemaVersion: ProjectionSchemaVersionAssistantMessage,
				MaterializationTarget: map[string]any{
					"core_materialization_scope": "business_truth",
				},
			},
			wantErr: true,
		},
		{
			name: "rejects typed semantic payload business evidence claim",
			projection: ProjectionCandidate{
				RunID:         "run-1",
				CandidateKind: "assistant_message",
				SchemaVersion: ProjectionSchemaVersionAssistantMessage,
				SemanticPayload: map[string]any{
					"object_type": "business EvidenceRecord",
				},
			},
			wantErr: true,
		},
		{
			name: "allows explanatory summary text inside semantic payload values",
			projection: ProjectionCandidate{
				RunID:         "run-1",
				CandidateKind: "assistant_message",
				SchemaVersion: ProjectionSchemaVersionAssistantMessage,
				SemanticPayload: map[string]any{
					"summary": "candidate output stored without business evidence semantics",
				},
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotErr := validateProjectionCandidate(normalizeProjectionCandidate(tc.projection)) != nil
			if gotErr != tc.wantErr {
				t.Fatalf("validateProjectionCandidate() error=%v, want error=%v", gotErr, tc.wantErr)
			}
		})
	}
}

func TestValidateProjectionCandidateNormalizesBeforeBoundaryCheck(t *testing.T) {
	t.Parallel()

	if err := ValidateProjectionCandidate(ProjectionCandidate{
		RunID:         "run-1",
		CandidateKind: "minimal_output",
	}); err != nil {
		t.Fatalf("ValidateProjectionCandidate() error = %v, want nil for default runtime projection boundary", err)
	}
}
