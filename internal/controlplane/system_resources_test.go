package controlplane

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManagerCreatesAndCompilesSystemResource(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	manager := NewManagerWithTruthDir(NewFileStore(filepath.Join(tmpDir, "overrides.json")), filepath.Join(tmpDir, "truth"))

	mutation, err := manager.CreateSystemResource(context.Background(), SystemResourceCreateRequest{
		AssetID:       "agent_profile.default",
		AssetType:     "agent_profile",
		AssetName:     "Core Agents",
		SourceContent: "# Operational Discipline\n\n- No fabrication\n- Respect evidence boundaries\n",
		Message:       "create agent profile",
	})
	if err != nil {
		t.Fatalf("CreateSystemResource() error = %v", err)
	}
	if !mutation.Accepted || mutation.Pipeline.Status != "active" {
		t.Fatalf("CreateSystemResource() = %#v, want accepted active pipeline", mutation)
	}

	items, err := manager.ListSystemResources(context.Background())
	if err != nil {
		t.Fatalf("ListSystemResources() error = %v", err)
	}
	if len(items) != 1 || items[0].AssetID != "agent_profile.default" {
		t.Fatalf("ListSystemResources() = %#v, want one agent_profile.default", items)
	}

	parseResult, err := manager.GetSystemResourceParseResult(context.Background(), "agent_profile.default")
	if err != nil {
		t.Fatalf("GetSystemResourceParseResult() error = %v", err)
	}
	if parseResult.Status != "parsed" || !strings.Contains(parseResult.Summary, "parsed") {
		t.Fatalf("parse result = %#v, want parsed AGENTS summary", parseResult)
	}

	compileResult, err := manager.GetSystemResourceCompileResult(context.Background(), "agent_profile.default")
	if err != nil {
		t.Fatalf("GetSystemResourceCompileResult() error = %v", err)
	}
	if compileResult.Status != "compiled" || strings.TrimSpace(compileResult.GuidanceText) == "" {
		t.Fatalf("compile result = %#v, want compiled guidance text", compileResult)
	}
	if got := compileResult.Payload["operational_discipline"]; got == nil {
		t.Fatalf("compile payload = %#v, want agent_profile compiled fields", compileResult.Payload)
	}

	debugPayload, err := manager.BuildSystemResourceDebugPayload(context.Background(), "agent_profile.default", "chat_respond")
	if err != nil {
		t.Fatalf("BuildSystemResourceDebugPayload() error = %v", err)
	}
	if debugPayload.Endpoint != "/api/chat/respond" {
		t.Fatalf("debug payload endpoint = %q, want /api/chat/respond", debugPayload.Endpoint)
	}
	globalContext, _ := debugPayload.Payload["global_context"].(map[string]any)
	assets, _ := globalContext["context_assets"].([]map[string]any)
	if len(assets) == 0 {
		t.Fatalf("debug payload context_assets = %#v, want non-empty", globalContext["context_assets"])
	}

	exportBytes, exportMeta, err := manager.ExportSystemResources(context.Background())
	if err != nil {
		t.Fatalf("ExportSystemResources() error = %v", err)
	}
	if len(exportBytes) == 0 || exportMeta.AssetCount != 1 {
		t.Fatalf("ExportSystemResources() = %d bytes, %#v, want non-empty one-asset export", len(exportBytes), exportMeta)
	}
}

func TestSaveSystemResourceSourceAutomaticallyActivates(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	manager := NewManagerWithTruthDir(NewFileStore(filepath.Join(tmpDir, "overrides.json")), filepath.Join(tmpDir, "truth"))
	if _, err := manager.CreateSystemResource(context.Background(), SystemResourceCreateRequest{
		AssetID:       "memory_view.default",
		AssetType:     "memory_view",
		AssetName:     "Default Memory View",
		SourceContent: "# Summary\n\n旧摘要\n\n# Facts\n- 旧事实\n",
	}); err != nil {
		t.Fatalf("CreateSystemResource() error = %v", err)
	}

	mutation, err := manager.SaveSystemResourceSource(context.Background(), "memory_view.default", SystemResourceSource{
		AssetID:       "memory_view.default",
		SourceContent: "# Summary\n\n新摘要\n\n# Facts\n- 新事实\n\n# Recent Decisions\n- 已切到 issue-7 完整实现\n",
	})
	if err != nil {
		t.Fatalf("SaveSystemResourceSource() error = %v", err)
	}
	if mutation.Pipeline.Status != "active" || mutation.Pipeline.CurrentStep != "active" {
		t.Fatalf("SaveSystemResourceSource() pipeline = %#v, want active", mutation.Pipeline)
	}

	detail, err := manager.GetSystemResource(context.Background(), "memory_view.default")
	if err != nil {
		t.Fatalf("GetSystemResource() error = %v", err)
	}
	if detail.Status != "active" {
		t.Fatalf("detail status = %q, want active", detail.Status)
	}
	if detail.ParseResult == nil || detail.CompileResult == nil {
		t.Fatalf("detail = %#v, want parse and compile results", detail)
	}
	if got := valueAsString(detail.CompileResult.Payload["summary"]); got != "新摘要" {
		t.Fatalf("compiled summary = %q, want 新摘要", got)
	}
}

func TestSyncSystemSourcesCompilesTruthTree(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	truthDir := filepath.Join(tmpDir, "truth")
	writeTruthMarkdown(t, filepath.Join(truthDir, "sources", "core", "SOUL.md"), `---
id: core_soul
name: Core Soul
summary: 墨思的全局人格基线
---

## Role
墨思是企业级安全咨询智能体。
`)
	writeTruthMarkdown(t, filepath.Join(truthDir, "sources", "core", "AGENTS.md"), `---
id: core_agents
name: Core Agents
summary: 墨思运行纪律
---

## Operational Discipline
- 不虚构
`)
	writeTruthMarkdown(t, filepath.Join(truthDir, "sources", "core", "policy_rule", "safety_constitution.md"), `---
id: safety_constitution
name: Safety Constitution
summary: 安全红线
severity: critical
checkpoints:
  - pre_inference
on_fail: deny
---

## Hard Gates
- 不允许跨租户泄漏
`)
	writeTruthMarkdown(t, filepath.Join(truthDir, "sources", "scenes", "default", "SCENE.md"), `---
id: default
name: Default Scene
summary: 墨思默认兜底场景
---

## Purpose
兜底回答和澄清。

## When It Applies
- 泛化问答

## Default Assets
- workflow.default.main
- skill.default.user_overview
`)
	writeTruthYAML(t, filepath.Join(truthDir, "sources", "scenes", "default", "workflow.yaml"), `id: default_main
name: Default Workflow
summary: Default workflow
entry:
  allow_waiting: true
  allow_resume: true
  required_contracts: []
  required_policy_rules: []
stages:
  - id: understand_request
    title: Understand Request
    mode: llm
    purpose: Understand request
    uses_skills:
      - skill.default.user_overview
    uses_contracts: []
    checks:
      policy_rules:
        - policy_rule.core.safety_constitution
    entry_if: []
    complete_when:
      - intent_is_clear
    block_when: []
    next:
      on_success: finalize
      on_waiting: waiting_for_information
      on_failure: failed
`)
	writeTruthYAML(t, filepath.Join(truthDir, "sources", "scenes", "default", "contract", "general_answer.yaml"), `id: general_answer
name: General Answer
summary: General answer contract
kind: output
required_fields:
  - main_answer
properties:
  main_answer:
    type: string
validation_rules:
  - contract_rule.main_answer_required
completion_rules:
  - contract_rule.answer_not_empty
`)
	writeTruthMarkdown(t, filepath.Join(truthDir, "sources", "scenes", "default", "contract", "README.md"), `# Contracts

- This directory documents contract source files.
`)
	writeTruthMarkdown(t, filepath.Join(truthDir, "sources", "scenes", "default", "skills", "user_overview", "SKILL.md"), `---
id: user_overview
name: User Overview
summary: 汇总当前上下文
description: 汇总 persona、user profile、memory view
scene: default
allowed_tools:
  - query_runtime_state
---

## When to Use
用于默认场景兜底回答前的上下文汇总。

## Process
- 汇总当前可见上下文。

## Output
- 用户上下文概览。
`)

	manager := NewManagerWithTruthDir(NewFileStore(filepath.Join(tmpDir, "overrides.json")), truthDir)
	if err := manager.SyncSystemSources(context.Background()); err != nil {
		t.Fatalf("SyncSystemSources() error = %v", err)
	}

	items, err := manager.ListSystemResources(context.Background())
	if err != nil {
		t.Fatalf("ListSystemResources() error = %v", err)
	}
	if len(items) != 7 {
		t.Fatalf("ListSystemResources() len = %d, want 7", len(items))
	}

	assertSystemResourceSourcePath(t, manager, "persona.default", "sources/core/SOUL.md")
	assertSystemResourceSourcePath(t, manager, "agent_profile.default", "sources/core/AGENTS.md")
	assertSystemResourceSourcePath(t, manager, "policy_rule.core.safety_constitution", "sources/core/policy_rule/safety_constitution.md")
	assertSystemResourceSourcePath(t, manager, "scene.default", "sources/scenes/default/SCENE.md")
	assertSystemResourceSourcePath(t, manager, "workflow.default.main", "sources/scenes/default/workflow.yaml")
	assertSystemResourceSourcePath(t, manager, "contract.default.general_answer", "sources/scenes/default/contract/general_answer.yaml")

	sceneDetail, err := manager.GetSystemResource(context.Background(), "scene.default")
	if err != nil {
		t.Fatalf("GetSystemResource(scene.default) error = %v", err)
	}
	if sceneDetail.CompileResult == nil || valueAsString(sceneDetail.CompileResult.Payload["scene_id"]) != "default" {
		t.Fatalf("scene compile result = %#v, want scene_id=default", sceneDetail.CompileResult)
	}

	workflowDetail, err := manager.GetSystemResource(context.Background(), "workflow.default.main")
	if err != nil {
		t.Fatalf("GetSystemResource(workflow.default.main) error = %v", err)
	}
	if workflowDetail.CompileResult == nil || len(stringSliceFromAny(workflowDetail.CompileResult.Payload["stage_order"])) == 0 {
		t.Fatalf("workflow compile result = %#v, want non-empty stage_order", workflowDetail.CompileResult)
	}
}

func TestSyncSystemSourcesRejectsEmptySource(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	truthDir := filepath.Join(tmpDir, "truth")
	writeTruthMarkdown(t, filepath.Join(truthDir, "sources", "core", "SOUL.md"), "\n")

	manager := NewManagerWithTruthDir(NewFileStore(filepath.Join(tmpDir, "overrides.json")), truthDir)
	err := manager.SyncSystemSources(context.Background())
	if err == nil {
		t.Fatalf("SyncSystemSources() error = nil, want empty source error")
	}
	if !strings.Contains(err.Error(), "system truth source") || !strings.Contains(err.Error(), "is empty") {
		t.Fatalf("SyncSystemSources() error = %v, want empty source error", err)
	}
}

func TestBuildCompiledAssetPayloadUsesTypedSchema(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		record   storedSystemResource
		source   string
		required []string
	}{
		{
			name: "persona",
			record: storedSystemResource{
				SystemResourceSummary: SystemResourceSummary{AssetID: "persona.default", AssetType: "persona", AssetName: "Core Soul"},
			},
			source: `---
id: core_soul
name: Core Soul
summary: 墨思人格
---

## Role
企业安全顾问

## Bottom Lines
- 不虚构
`,
			required: []string{"id", "role", "bottom_lines"},
		},
		{
			name: "policy rule",
			record: storedSystemResource{
				SystemResourceSummary: SystemResourceSummary{AssetID: "policy_rule.core.safety_constitution", AssetType: "policy_rule", AssetName: "Safety Constitution"},
			},
			source: `---
id: safety_constitution
name: Safety Constitution
summary: 安全红线
severity: critical
checkpoints:
  - pre_inference
on_fail: deny
---

## Hard Gates
- 不允许跨租户泄漏

## Check Rules
- 高风险结论必须带 evidence
`,
			required: []string{"rule_id", "hard_gates", "check_rules", "checkpoints"},
		},
		{
			name: "tool governance policy",
			record: storedSystemResource{
				SystemResourceSummary: SystemResourceSummary{AssetID: "tool_governance_policy.core.default", AssetType: "tool_governance_policy", AssetName: "Default Tool Governance"},
			},
			source: `---
id: default_tool_governance
name: Default Tool Governance
default_decision: allow
decision_model: first_match
rules:
  - id: redact_external_read
    match_tool: demo_browser
    match_scope: external_web
    match_operation: read
    match_risk: medium
    decision: allow_with_redaction
    reason: External reads may proceed with redaction.
    redact_fields:
      - headers.authorization
---

## Purpose
Govern runtime tool requests before execution.
`,
			required: []string{"policy_id", "default_decision", "decision_model", "rules"},
		},
		{
			name: "scene",
			record: storedSystemResource{
				SystemResourceSummary: SystemResourceSummary{AssetID: "scene.security_review", AssetType: "scene", AssetName: "Security Review"},
				Metadata: map[string]any{
					"frontmatter": map[string]any{"id": "security_review", "summary": "安全评审场景"},
				},
			},
			source: `---
id: security_review
name: 安全评审
summary: 安全评审场景
---

## Purpose
进行安全评审。

## Default Assets
- workflow.security_review.main
- skill.security_review.cso_review
`,
			required: []string{"scene_id", "default_workflow_ref", "default_skill_refs"},
		},
		{
			name: "workflow",
			record: storedSystemResource{
				SystemResourceSummary: SystemResourceSummary{AssetID: "workflow.security_review.main", AssetType: "workflow", AssetName: "Security Review Workflow"},
				Metadata: map[string]any{
					"parsed_yaml": map[string]any{
						"id":      "security_review_main",
						"name":    "Security Review Workflow",
						"summary": "主 workflow",
						"stages": []any{
							map[string]any{"id": "understand_request"},
							map[string]any{"id": "finalize_review"},
						},
					},
				},
			},
			source: `id: security_review_main
name: Security Review Workflow
summary: 主 workflow
stages:
  - id: understand_request
  - id: finalize_review
`,
			required: []string{"workflow_id", "stage_order", "stages"},
		},
		{
			name: "contract",
			record: storedSystemResource{
				SystemResourceSummary: SystemResourceSummary{AssetID: "contract.security_review.risk_profile", AssetType: "contract", AssetName: "Risk Profile"},
				Metadata: map[string]any{
					"parsed_yaml": map[string]any{
						"id":              "risk_profile",
						"name":            "Risk Profile",
						"summary":         "风险画像",
						"kind":            "output",
						"required_fields": []any{"summary"},
						"properties":      map[string]any{"summary": map[string]any{"type": "string"}},
						"validation_rules": []any{
							"contract_rule.summary_required",
						},
						"completion_rules": []any{
							"contract_rule.summary_not_empty",
						},
					},
				},
			},
			source: `id: risk_profile
name: Risk Profile
summary: 风险画像
kind: output
required_fields:
  - summary
properties:
  summary:
    type: string
validation_rules:
  - contract_rule.summary_required
completion_rules:
  - contract_rule.summary_not_empty
`,
			required: []string{"contract_id", "required_fields", "validation_rules", "completion_rules"},
		},
		{
			name: "skill",
			record: storedSystemResource{
				SystemResourceSummary: SystemResourceSummary{AssetID: "skill.security_review.risk_profiling", AssetType: "skill", AssetName: "Risk Profiling"},
				Metadata: map[string]any{
					"frontmatter": map[string]any{
						"id":            "risk_profiling",
						"description":   "Build a structured risk profile",
						"allowed_tools": []any{"query_runtime_state"},
					},
				},
			},
			source: `---
id: risk_profiling
name: Risk Profiling
summary: 风险画像
description: Build a structured risk profile
scene: security_review
allowed_tools:
  - query_runtime_state
---

## Process
- 构建风险画像
`,
			required: []string{"skill_id", "description", "allowed_tools", "guidance"},
		},
		{
			name: "user profile",
			record: storedSystemResource{
				SystemResourceSummary: SystemResourceSummary{AssetID: "user_profile.default", AssetType: "user_profile", AssetName: "Default User"},
			},
			source:   "# Identity Summary\n\n默认用户\n\n# Long Term Goals\n- 降低风险\n",
			required: []string{"id", "identity_summary", "long_term_goals"},
		},
		{
			name: "memory view",
			record: storedSystemResource{
				SystemResourceSummary: SystemResourceSummary{AssetID: "memory_view.default", AssetType: "memory_view", AssetName: "Default Memory"},
			},
			source:   "# Summary\n\n偏好直接表达\n\n# Facts\n- 已采用 issue 驱动\n",
			required: []string{"id", "summary", "facts"},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			payload := buildCompiledAssetPayload(tc.record, tc.source)
			for _, key := range tc.required {
				if _, ok := payload[key]; !ok {
					t.Fatalf("payload = %#v, missing key %q", payload, key)
				}
			}
		})
	}
}

func TestToolGovernancePolicyCompilesAndLogsDecisions(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	truthDir := filepath.Join(tmpDir, "truth")
	writeTruthMarkdown(t, filepath.Join(truthDir, "sources", "core", "tool_governance_policy", "default.md"), `---
id: default_tool_governance
name: Default Tool Governance
default_decision: allow
decision_model: first_match
rules:
  - id: deny_credential_export
    match_tool: credential_export
    match_scope: secret
    match_operation: read
    match_risk: high
    decision: deny
    reason: Raw credential export is blocked.
  - id: redact_external_read
    match_tool: demo_browser
    match_scope: external_web
    match_operation: read
    match_risk: medium
    decision: allow_with_redaction
    reason: External reads may proceed with redaction.
    redact_fields:
      - headers.authorization
---

## Purpose
Govern runtime tool requests before execution.
`)

	manager := NewManagerWithTruthDir(NewFileStore(filepath.Join(tmpDir, "overrides.json")), truthDir)
	if err := manager.SyncSystemSources(context.Background()); err != nil {
		t.Fatalf("SyncSystemSources() error = %v", err)
	}

	policy, err := manager.EffectiveToolGovernancePolicy(context.Background())
	if err != nil {
		t.Fatalf("EffectiveToolGovernancePolicy() error = %v", err)
	}
	if len(policy.Rules) != 2 || policy.DefaultDecision != "allow" {
		t.Fatalf("policy = %#v, want 2 rules and default allow", policy)
	}

	decision, err := manager.EvaluateToolGovernance(context.Background(), ToolGovernanceDecisionRequest{
		ToolName:  "demo_browser",
		ToolScope: "external_web",
		Operation: "read",
		RiskLevel: "medium",
		Metadata: map[string]any{
			"api_key":    "must-not-persist",
			"request_id": "req_123",
		},
	})
	if err != nil {
		t.Fatalf("EvaluateToolGovernance() error = %v", err)
	}
	if decision.Decision != "allow_with_redaction" || decision.MatchedRuleID != "redact_external_read" {
		t.Fatalf("decision = %#v, want redaction rule", decision)
	}
	if got := valueAsString(decision.Metadata["api_key"]); got != "[redacted]" {
		t.Fatalf("decision metadata api_key = %q, want redacted", got)
	}

	items, err := manager.ListToolGovernanceDecisions(context.Background())
	if err != nil {
		t.Fatalf("ListToolGovernanceDecisions() error = %v", err)
	}
	if len(items) != 1 || items[0].DecisionID != decision.DecisionID {
		t.Fatalf("decisions = %#v, want persisted decision", items)
	}
}

func TestSystemResourceVersionsAndRollback(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	manager := NewManagerWithTruthDir(NewFileStore(filepath.Join(tmpDir, "overrides.json")), filepath.Join(tmpDir, "truth"))
	if _, err := manager.CreateSystemResource(context.Background(), SystemResourceCreateRequest{
		AssetID:       "policy_rule.core.rollback_rule",
		AssetType:     "policy_rule",
		AssetName:     "Rollback Rule",
		SourceContent: "# Hard Gates\n- 初始规则\n",
		Message:       "seed rollback rule",
	}); err != nil {
		t.Fatalf("CreateSystemResource() error = %v", err)
	}
	if _, err := manager.SaveSystemResourceSource(context.Background(), "policy_rule.core.rollback_rule", SystemResourceSource{
		AssetID:       "policy_rule.core.rollback_rule",
		SourceContent: "# Hard Gates\n- 更新后的规则\n",
		Message:       "update rollback rule",
	}); err != nil {
		t.Fatalf("SaveSystemResourceSource() error = %v", err)
	}

	versions, err := manager.ListSystemResourceVersions(context.Background(), "policy_rule.core.rollback_rule")
	if err != nil {
		t.Fatalf("ListSystemResourceVersions() error = %v", err)
	}
	if len(versions) < 2 {
		t.Fatalf("versions len = %d, want at least 2", len(versions))
	}

	var targetVersion SystemResourceVersionSummary
	for _, item := range versions {
		detail, err := manager.GetSystemResourceVersion(context.Background(), "policy_rule.core.rollback_rule", item.VersionID)
		if err != nil {
			t.Fatalf("GetSystemResourceVersion(%s) error = %v", item.VersionID, err)
		}
		if strings.Contains(detail.SourceContent, "初始规则") {
			targetVersion = item
			break
		}
	}
	if targetVersion.VersionID == "" {
		t.Fatalf("failed to find version containing initial source: %#v", versions)
	}

	mutation, err := manager.RollbackSystemResourceVersion(context.Background(), "policy_rule.core.rollback_rule", targetVersion.VersionID)
	if err != nil {
		t.Fatalf("RollbackSystemResourceVersion() error = %v", err)
	}
	if !mutation.Accepted || mutation.Pipeline.Status != "active" {
		t.Fatalf("rollback mutation = %#v, want accepted active pipeline", mutation)
	}

	source, err := manager.GetSystemResourceSource(context.Background(), "policy_rule.core.rollback_rule")
	if err != nil {
		t.Fatalf("GetSystemResourceSource() error = %v", err)
	}
	if !strings.Contains(source.SourceContent, "初始规则") {
		t.Fatalf("source after rollback = %q, want initial source restored", source.SourceContent)
	}
}

func TestSyncSystemSourcesWritesGeneratedStateOutsideTruthDir(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	truthDir := filepath.Join(tmpDir, "truth")
	stateDir := filepath.Join(tmpDir, "state")
	writeTruthMarkdown(t, filepath.Join(truthDir, "sources", "core", "SOUL.md"), `---
id: core_soul
name: Core Soul
summary: 墨思人格
---

## Role
分析助手
`)

	manager := NewManagerWithTruthAndStateDirs(NewFileStore(filepath.Join(tmpDir, "overrides.json")), truthDir, stateDir)
	if err := manager.SyncSystemSources(context.Background()); err != nil {
		t.Fatalf("SyncSystemSources() error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(stateDir, "persona.default", "meta.json")); err != nil {
		t.Fatalf("generated state meta missing from state dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(truthDir, "persona.default")); !os.IsNotExist(err) {
		t.Fatalf("truth dir generated state exists err = %v, want not exist", err)
	}
	if _, err := os.Stat(filepath.Join(truthDir, "sources", "core", "SOUL.md")); err != nil {
		t.Fatalf("source truth file missing: %v", err)
	}
}

func TestBuildCompiledAssetsPackage(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	truthDir := filepath.Join(tmpDir, "truth")
	writeTruthMarkdown(t, filepath.Join(truthDir, "sources", "core", "SOUL.md"), `---
id: core_soul
name: Core Soul
summary: 墨思人格
---

## Role
分析助手
`)
	writeTruthMarkdown(t, filepath.Join(truthDir, "sources", "core", "user_profile", "default.md"), "# Identity Summary\n\n默认用户\n")
	writeTruthMarkdown(t, filepath.Join(truthDir, "sources", "core", "memory_view", "default.md"), "# Summary\n\n默认记忆\n")

	manager := NewManagerWithTruthDir(NewFileStore(filepath.Join(tmpDir, "overrides.json")), truthDir)
	outputDir := filepath.Join(tmpDir, "compiled")
	manifest, err := manager.BuildCompiledAssetsPackage(context.Background(), outputDir)
	if err != nil {
		t.Fatalf("BuildCompiledAssetsPackage() error = %v", err)
	}
	if manifest.AssetCount != 3 {
		t.Fatalf("manifest asset_count = %d, want 3", manifest.AssetCount)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "manifest.json")); err != nil {
		t.Fatalf("manifest.json missing: %v", err)
	}
	for _, want := range []string{
		"persona.default.json",
		"user_profile.default.json",
		"memory_view.default.json",
	} {
		if _, err := os.Stat(filepath.Join(outputDir, "assets", want)); err != nil {
			t.Fatalf("compiled asset %s missing: %v", want, err)
		}
	}
}

func writeTruthMarkdown(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%s) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", path, err)
	}
}

func writeTruthYAML(t *testing.T, path string, content string) {
	t.Helper()
	writeTruthMarkdown(t, path, content)
}

func assertSystemResourceSourcePath(t *testing.T, manager *Manager, assetID, want string) {
	t.Helper()
	item, err := manager.GetSystemResource(context.Background(), assetID)
	if err != nil {
		t.Fatalf("GetSystemResource(%s) error = %v", assetID, err)
	}
	if got := filepath.ToSlash(item.SourcePath); got != want {
		t.Fatalf("%s source path = %q, want %s", assetID, got, want)
	}
}
