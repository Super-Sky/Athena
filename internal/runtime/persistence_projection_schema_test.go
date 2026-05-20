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
		SchemaVersion: "projection.custom.v9",
	})
	if got.SchemaVersion != "projection.custom.v9" {
		t.Fatalf("schema version = %q, want projection.custom.v9", got.SchemaVersion)
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
