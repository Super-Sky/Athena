package runtime

import (
	"os"
	"path/filepath"
	"testing"

	"moss/internal/session"
	"moss/internal/workflow"
)

func TestWriteArtifactRejectsUnsafeRelativePath(t *testing.T) {
	sharedRoot := t.TempDir()
	_, err := WriteArtifact(ArtifactWriteInput{
		SharedRootDir: sharedRoot,
		Request: ArtifactWriteRequest{
			ArtifactOwnerKey:  "u1001",
			ArtifactOwnerType: ArtifactOwnerTypeUser,
			Filename:          "report.md",
			RelativePath:      "../escape/report.md",
		},
		Content: "hello",
	})
	if err == nil {
		t.Fatalf("expected unsafe relative path rejection")
	}
}

func TestWriteArtifactRejectsNonUTF8Text(t *testing.T) {
	sharedRoot := t.TempDir()
	_, err := WriteArtifact(ArtifactWriteInput{
		SharedRootDir: sharedRoot,
		Request: ArtifactWriteRequest{
			ArtifactOwnerKey:  "u1001",
			ArtifactOwnerType: ArtifactOwnerTypeUser,
			Filename:          "report.txt",
		},
		Content: string([]byte{0xff, 0xfe, 0xfd}),
	})
	if err == nil {
		t.Fatalf("expected UTF-8 validation error")
	}
}

func TestWriteArtifactWritesOneCompleteFileAndVersionsConflicts(t *testing.T) {
	sharedRoot := t.TempDir()
	input := ArtifactWriteInput{
		SharedRootDir: sharedRoot,
		Request: ArtifactWriteRequest{
			ArtifactOwnerKey:  "u1001",
			ArtifactOwnerType: ArtifactOwnerTypeUser,
			Filename:          "risk-report.md",
			Kind:              "markdown",
			Title:             "风险报告",
		},
		Content:          "# report\nbody",
		GenerationReason: "generate report",
	}

	first, err := WriteArtifact(input)
	if err != nil {
		t.Fatalf("WriteArtifact() first error = %v", err)
	}
	if first.RelativePath != "users/u1001/reports/risk-report.md" {
		t.Fatalf("relative_path = %q, want users/u1001/reports/risk-report.md", first.RelativePath)
	}
	payload, err := os.ReadFile(filepath.Join(sharedRoot, filepath.FromSlash(first.RelativePath)))
	if err != nil {
		t.Fatalf("ReadFile() first error = %v", err)
	}
	if string(payload) != "# report\nbody" {
		t.Fatalf("first content = %q, want complete payload", string(payload))
	}

	second, err := WriteArtifact(input)
	if err != nil {
		t.Fatalf("WriteArtifact() second error = %v", err)
	}
	if second.RelativePath != "users/u1001/reports/risk-report.v2.md" {
		t.Fatalf("relative_path = %q, want users/u1001/reports/risk-report.v2.md", second.RelativePath)
	}
	secondPayload, err := os.ReadFile(filepath.Join(sharedRoot, filepath.FromSlash(second.RelativePath)))
	if err != nil {
		t.Fatalf("ReadFile() second error = %v", err)
	}
	if string(secondPayload) != "# report\nbody" {
		t.Fatalf("second content = %q, want complete payload", string(secondPayload))
	}
}

func TestReadOnlyResourceReadHonorsProjection(t *testing.T) {
	request := ReadOnlyResourceReadRequest{
		ResourceID:    "doc-1",
		ResourceKind:  ResourceKindInjectedDocument,
		ResourceScope: ResourceScopeTask,
		Projection:    ResourceProjectionMetadata,
	}
	inputPayload := map[string]any{
		"read_only_resources": map[string]any{
			"doc-1": map[string]any{
				"resource_id":    "doc-1",
				"resource_kind":  "injected_document",
				"resource_scope": "task",
				"content":        "full body",
				"summary":        "body summary",
				"metadata": map[string]any{
					"source": "platform",
				},
			},
		},
	}

	result := ReadOnlyResourceRead(request, inputPayload, nil, nil)
	if result == nil {
		t.Fatalf("expected resource read result")
	}
	if result.Content != "" || result.Summary != "" {
		t.Fatalf("metadata projection should leave content and summary empty, got %#v", result)
	}
	if result.Metadata["source"] != "platform" {
		t.Fatalf("metadata = %#v, want source=platform", result.Metadata)
	}
}

func TestParseStructuredDataSupportsJSONYAMLCSVAndFrontMatter(t *testing.T) {
	jsonResult, err := ParseStructuredData(StructuredDataParseRequest{
		Format:  "json",
		Content: `{"name":"athena","enabled":true}`,
	})
	if err != nil || jsonResult == nil || len(jsonResult.Keys) != 2 {
		t.Fatalf("json parse result = %#v, err=%v", jsonResult, err)
	}

	yamlResult, err := ParseStructuredData(StructuredDataParseRequest{
		Format:  "yaml",
		Content: "name: athena\nenabled: true\n",
	})
	if err != nil || yamlResult == nil || len(yamlResult.Keys) != 2 {
		t.Fatalf("yaml parse result = %#v, err=%v", yamlResult, err)
	}

	csvResult, err := ParseStructuredData(StructuredDataParseRequest{
		Format:    "csv",
		Content:   "name,enabled\nathena,true\n",
		HasHeader: true,
	})
	if err != nil || csvResult == nil || csvResult.RowCount != 1 {
		t.Fatalf("csv parse result = %#v, err=%v", csvResult, err)
	}

	frontMatterResult, err := ParseStructuredData(StructuredDataParseRequest{
		Format:  "frontmatter",
		Content: "---\ntitle: Athena\n---\nhello\n",
	})
	if err != nil || frontMatterResult == nil {
		t.Fatalf("frontmatter parse result = %#v, err=%v", frontMatterResult, err)
	}
}

func TestTransformLocalDataSupportsMergeAndProject(t *testing.T) {
	inputPayload := map[string]any{
		"local_data_sources": map[string]any{
			"risk": map[string]any{
				"score":  "medium",
				"flags":  []any{"manual_review"},
				"nested": map[string]any{"reason": "velocity"},
			},
			"orders": map[string]any{
				"count": 3,
			},
		},
	}

	merged, err := TransformLocalData(LocalDataTransformRequest{
		Operation:  "merge_objects",
		SourceKeys: []string{"risk", "orders"},
	}, inputPayload, nil, nil)
	if err != nil || merged == nil {
		t.Fatalf("merge result = %#v, err=%v", merged, err)
	}
	data, ok := merged.Data.(map[string]any)
	if !ok || data["count"] != 3 || data["score"] != "medium" {
		t.Fatalf("merged data = %#v", merged.Data)
	}

	projected, err := TransformLocalData(LocalDataTransformRequest{
		Operation:  "project_fields",
		SourceKeys: []string{"risk"},
		FieldPaths: []string{"score", "nested.reason", "missing.field"},
	}, inputPayload, nil, nil)
	if err != nil || projected == nil {
		t.Fatalf("project result = %#v, err=%v", projected, err)
	}
	projectedData, ok := projected.Data.(map[string]any)
	if !ok || projectedData["score"] != "medium" || projectedData["reason"] != "velocity" {
		t.Fatalf("projected data = %#v", projected.Data)
	}
	if len(projected.MissingPaths) != 1 || projected.MissingPaths[0] != "missing.field" {
		t.Fatalf("missing paths = %#v, want [missing.field]", projected.MissingPaths)
	}
}

func TestEvaluateFactQualityGateReturnsClarificationForCurrentStateWithoutFreshData(t *testing.T) {
	result := EvaluateFactQualityGate(FactQualityGateRequest{
		QuestionScope:        FactQuestionScopeCurrentState,
		SessionContextUsed:   true,
		FreshLookupPerformed: false,
		MissingData:          []string{"risk_flags", "review_status"},
	})
	if result == nil {
		t.Fatalf("expected fact quality result")
	}
	if result.SourceMode != FactSourceModeSessionOnly {
		t.Fatalf("source_mode = %q, want session_only", result.SourceMode)
	}
	if result.AnswerMode != FactAnswerModeClarification {
		t.Fatalf("answer_mode = %q, want clarification", result.AnswerMode)
	}
	if !result.NeedsClarification || result.ClarifyingQuestion == "" {
		t.Fatalf("expected clarification question, got %#v", result)
	}
	if result.EvidenceGatePassed {
		t.Fatalf("expected evidence gate to fail")
	}
}

func TestQueryRuntimeStateBuildsBoundedStateView(t *testing.T) {
	currentSession := &session.Session{
		ID:       "sess-1",
		Messages: []session.Message{{Role: "user", Content: "hello"}},
		Pending: &session.PendingState{
			Stage: "waiting_for_information",
		},
	}
	plan := &workflow.Plan{
		PlanID:  "plan-1",
		TaskID:  "task-1",
		Title:   "plan",
		Summary: "plan summary",
		Steps: []workflow.Step{{
			StepID:        "step-1",
			Order:         1,
			Title:         "Inspect",
			ExecutionMode: workflow.StepExecutionModeReadonlyAnalysis,
			StepType:      workflow.StepTypeAnalysis,
		}},
	}
	state := QueryRuntimeState(RuntimeStateQueryRequest{
		SessionID: "sess-1",
		Include: []string{
			"session_snapshot",
			"workflow_plan",
			"structured_result",
			"governance_summary",
			"last_turn_summary",
		},
	}, currentSession, map[string]any{
		"task_type":            "workflow_step_request",
		"workflow_plan":        plan,
		"execution_intent":     &ExecutionIntent{RiskLevel: ExecutionRiskLevelMedium, RequiresConfirmation: true},
		"execution_result":     &ExecutionResult{Status: "failed"},
		"workflow_step_result": workflow.StepResult{StepID: "step-1"},
	}, plan, &ExecutionIntent{RiskLevel: ExecutionRiskLevelMedium, RequiresConfirmation: true}, &ExecutionResult{Status: "failed"}, "summary")
	if state == nil {
		t.Fatalf("expected runtime state result")
	}
	if state.SessionSnapshot == nil || !state.SessionSnapshot.Pending {
		t.Fatalf("unexpected session snapshot = %#v", state.SessionSnapshot)
	}
	if state.WorkflowPlan == nil || state.WorkflowPlan.PlanID != "plan-1" {
		t.Fatalf("unexpected workflow plan = %#v", state.WorkflowPlan)
	}
	if state.StructuredResult["runtime_state"] != nil {
		t.Fatalf("runtime_state should not recursively include itself")
	}
	if state.GovernanceSummary == nil || state.GovernanceSummary.Decision != "confirm" {
		t.Fatalf("unexpected governance summary = %#v", state.GovernanceSummary)
	}
	if state.LastTurnSummary != "summary" {
		t.Fatalf("last_turn_summary = %q, want summary", state.LastTurnSummary)
	}
}
