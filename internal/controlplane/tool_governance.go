package controlplane

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"moss/internal/systemtruth"
)

const (
	toolGovernancePolicyAssetType = "tool_governance_policy"
	toolGovernanceDecisionLogFile = "tool_governance_decisions.json"
	defaultToolGovernanceDecision = "allow"
	toolGovernanceDecisionLimit   = 100
)

type toolGovernanceDecisionLog struct {
	Items []ToolGovernanceDecision `json:"items"`
}

// EffectiveToolGovernancePolicy returns the active compiled tool policy assembled from system truth.
// EffectiveToolGovernancePolicy 返回由 system truth 编译出的有效 tool 治理策略。
func (m *Manager) EffectiveToolGovernancePolicy(ctx context.Context) (ToolGovernancePolicy, error) {
	records, err := m.loadAllResources(ctx)
	if err != nil {
		return ToolGovernancePolicy{}, err
	}
	sort.Slice(records, func(i, j int) bool { return records[i].AssetID < records[j].AssetID })
	effective := ToolGovernancePolicy{
		PolicyID:        "tool_governance_policy.effective",
		AssetID:         "tool_governance_policy.effective",
		Name:            "Effective Tool Governance Policy",
		DefaultDecision: defaultToolGovernanceDecision,
		DecisionModel:   "first_match",
		Metadata:        map[string]any{"source_asset_ids": []string{}},
	}
	var sourceAssetIDs []string
	for _, record := range records {
		if normalizeAssetType(record.AssetType) != toolGovernancePolicyAssetType || strings.TrimSpace(record.Status) != "active" {
			continue
		}
		compileResult, err := m.LoadSystemResourceCompileResult(ctx, record.AssetID)
		if err != nil || compileResult == nil {
			continue
		}
		policy := decodeToolGovernancePolicyPayload(compileResult.Payload)
		if strings.TrimSpace(policy.DefaultDecision) != "" && effective.DefaultDecision == defaultToolGovernanceDecision {
			effective.DefaultDecision = normalizeToolGovernanceDecision(policy.DefaultDecision)
		}
		if strings.TrimSpace(policy.DecisionModel) != "" {
			effective.DecisionModel = strings.TrimSpace(policy.DecisionModel)
		}
		for _, rule := range policy.Rules {
			if rule.Metadata == nil {
				rule.Metadata = map[string]any{}
			}
			rule.Metadata["source_asset_id"] = record.AssetID
			effective.Rules = append(effective.Rules, rule)
		}
		sourceAssetIDs = append(sourceAssetIDs, record.AssetID)
		effective.CompiledVersion = defaultString(compileResult.CompiledVersion, effective.CompiledVersion)
		effective.TruthDirVersion = defaultString(compileResult.TruthDirVersion, effective.TruthDirVersion)
		effective.SourceChecksum = defaultString(compileResult.SourceChecksum, effective.SourceChecksum)
		effective.UpdatedAt = defaultString(compileResult.UpdatedAt, effective.UpdatedAt)
	}
	effective.Metadata["source_asset_ids"] = sourceAssetIDs
	return effective, nil
}

// EvaluateToolGovernance evaluates and persists one pre-execution tool governance decision.
// EvaluateToolGovernance 会判定并持久化一次 tool 执行前治理决策。
func (m *Manager) EvaluateToolGovernance(ctx context.Context, input ToolGovernanceDecisionRequest) (ToolGovernanceDecision, error) {
	toolName := strings.TrimSpace(input.ToolName)
	if toolName == "" {
		return ToolGovernanceDecision{}, fmt.Errorf("tool_name is required")
	}
	policy, err := m.EffectiveToolGovernancePolicy(ctx)
	if err != nil {
		return ToolGovernanceDecision{}, err
	}
	decision := ToolGovernanceDecision{
		DecisionID:      fmt.Sprintf("tool_gov_%s", newResourceVersion()),
		Decision:        normalizeToolGovernanceDecision(policy.DefaultDecision),
		Reason:          "default policy decision",
		PolicyAssetID:   policy.AssetID,
		PolicyVersion:   policy.CompiledVersion,
		ToolName:        toolName,
		ToolScope:       strings.TrimSpace(input.ToolScope),
		Operation:       strings.TrimSpace(input.Operation),
		RiskLevel:       strings.TrimSpace(input.RiskLevel),
		EvaluatedAt:     time.Now().UTC().Format(time.RFC3339),
		TruthDirVersion: policy.TruthDirVersion,
		Metadata:        scrubGovernanceMetadata(input.Metadata),
	}
	for _, rule := range policy.Rules {
		if !toolGovernanceRuleMatches(rule, input) {
			continue
		}
		decision.Decision = normalizeToolGovernanceDecision(rule.Decision)
		decision.Reason = defaultString(strings.TrimSpace(rule.Reason), "matched tool governance rule")
		decision.MatchedRuleID = strings.TrimSpace(rule.RuleID)
		decision.RedactFields = append([]string(nil), rule.RedactFields...)
		decision.SandboxRef = strings.TrimSpace(rule.SandboxRef)
		if sourceAssetID := valueAsString(rule.Metadata["source_asset_id"]); sourceAssetID != "" {
			decision.PolicyAssetID = sourceAssetID
		}
		break
	}
	if err := m.appendToolGovernanceDecision(ctx, decision); err != nil {
		return ToolGovernanceDecision{}, err
	}
	return decision, nil
}

// ListToolGovernanceDecisions returns recent persisted tool governance decisions.
// ListToolGovernanceDecisions 返回最近持久化的 tool 治理判定。
func (m *Manager) ListToolGovernanceDecisions(ctx context.Context) ([]ToolGovernanceDecision, error) {
	log, err := m.loadToolGovernanceDecisionLog(ctx)
	if err != nil {
		return nil, err
	}
	return append([]ToolGovernanceDecision(nil), log.Items...), nil
}

func buildToolGovernancePolicyPayload(record storedSystemResource, source string) map[string]any {
	doc := systemtruth.ParseMarkdownDocumentText(record.SourcePath, source)
	frontmatter := mergeMetadata(frontmatterMap(record), doc.Frontmatter)
	rules := toolGovernanceRulesFromAny(frontmatter["rules"])
	policy := ToolGovernancePolicy{
		PolicyID:        defaultString(valueAsString(frontmatter["id"]), record.AssetID),
		AssetID:         record.AssetID,
		Name:            defaultString(valueAsString(frontmatter["name"]), record.AssetName),
		DefaultDecision: normalizeToolGovernanceDecision(defaultString(valueAsString(frontmatter["default_decision"]), defaultToolGovernanceDecision)),
		DecisionModel:   defaultString(valueAsString(frontmatter["decision_model"]), "first_match"),
		Rules:           rules,
		Metadata:        map[string]any{"frontmatter": cloneAnyMap(frontmatter)},
	}
	payload, _ := json.Marshal(policy)
	var result map[string]any
	_ = json.Unmarshal(payload, &result)
	return result
}

func decodeToolGovernancePolicyPayload(payload map[string]any) ToolGovernancePolicy {
	raw, err := json.Marshal(payload)
	if err != nil {
		return ToolGovernancePolicy{}
	}
	var policy ToolGovernancePolicy
	_ = json.Unmarshal(raw, &policy)
	policy.DefaultDecision = normalizeToolGovernanceDecision(policy.DefaultDecision)
	for i := range policy.Rules {
		policy.Rules[i].Decision = normalizeToolGovernanceDecision(policy.Rules[i].Decision)
	}
	return policy
}

func toolGovernanceRulesFromAny(value any) []ToolGovernanceRule {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	rules := make([]ToolGovernanceRule, 0, len(items))
	for index, item := range items {
		rawMap := decodeStoredMap(item)
		if len(rawMap) == 0 {
			continue
		}
		rule := ToolGovernanceRule{
			RuleID:         defaultString(valueAsString(rawMap["id"]), fmt.Sprintf("rule_%d", index+1)),
			MatchTool:      strings.TrimSpace(valueAsString(rawMap["match_tool"])),
			MatchScope:     strings.TrimSpace(valueAsString(rawMap["match_scope"])),
			MatchOperation: strings.TrimSpace(valueAsString(rawMap["match_operation"])),
			MatchRisk:      strings.TrimSpace(valueAsString(rawMap["match_risk"])),
			Decision:       normalizeToolGovernanceDecision(valueAsString(rawMap["decision"])),
			Reason:         strings.TrimSpace(valueAsString(rawMap["reason"])),
			RedactFields:   stringSliceFromAny(rawMap["redact_fields"]),
			SandboxRef:     strings.TrimSpace(valueAsString(rawMap["sandbox_ref"])),
			Metadata:       cloneAnyMap(rawMap),
		}
		if rule.Decision == "" {
			rule.Decision = defaultToolGovernanceDecision
		}
		rules = append(rules, rule)
	}
	return rules
}

func toolGovernanceRuleMatches(rule ToolGovernanceRule, input ToolGovernanceDecisionRequest) bool {
	return governancePatternMatches(rule.MatchTool, input.ToolName) &&
		governancePatternMatches(rule.MatchScope, input.ToolScope) &&
		governancePatternMatches(rule.MatchOperation, input.Operation) &&
		governancePatternMatches(rule.MatchRisk, input.RiskLevel)
}

func governancePatternMatches(pattern string, value string) bool {
	pattern = strings.TrimSpace(strings.ToLower(pattern))
	value = strings.TrimSpace(strings.ToLower(value))
	if pattern == "" || pattern == "*" {
		return true
	}
	return pattern == value
}

func normalizeToolGovernanceDecision(input string) string {
	switch strings.TrimSpace(strings.ToLower(input)) {
	case "deny":
		return "deny"
	case "allow_with_redaction":
		return "allow_with_redaction"
	case "require_sandbox_ref":
		return "require_sandbox_ref"
	case "allow":
		return "allow"
	default:
		return defaultToolGovernanceDecision
	}
}

func scrubGovernanceMetadata(metadata map[string]any) map[string]any {
	if len(metadata) == 0 {
		return nil
	}
	result := make(map[string]any, len(metadata))
	for key, value := range metadata {
		lower := strings.ToLower(strings.TrimSpace(key))
		if strings.Contains(lower, "token") || strings.Contains(lower, "secret") || strings.Contains(lower, "password") || strings.Contains(lower, "credential") || strings.Contains(lower, "api_key") {
			result[key] = "[redacted]"
			continue
		}
		result[key] = value
	}
	return result
}

func (m *Manager) appendToolGovernanceDecision(ctx context.Context, decision ToolGovernanceDecision) error {
	log, err := m.loadToolGovernanceDecisionLog(ctx)
	if err != nil {
		return err
	}
	log.Items = append([]ToolGovernanceDecision{decision}, log.Items...)
	if len(log.Items) > toolGovernanceDecisionLimit {
		log.Items = log.Items[:toolGovernanceDecisionLimit]
	}
	if err := os.MkdirAll(filepath.Dir(m.toolGovernanceDecisionLogPath()), 0o755); err != nil {
		return err
	}
	return writeJSON(m.toolGovernanceDecisionLogPath(), log)
}

func (m *Manager) loadToolGovernanceDecisionLog(_ context.Context) (toolGovernanceDecisionLog, error) {
	var log toolGovernanceDecisionLog
	if err := readJSON(m.toolGovernanceDecisionLogPath(), &log); err != nil {
		if errorsIsNotExist(err) {
			return toolGovernanceDecisionLog{}, nil
		}
		return toolGovernanceDecisionLog{}, err
	}
	return log, nil
}

func (m *Manager) toolGovernanceDecisionLogPath() string {
	root := strings.TrimSpace(m.activeStateRoot())
	if root == "" {
		root = strings.TrimSpace(m.truthDir)
	}
	return filepath.Join(root, toolGovernanceDecisionLogFile)
}
