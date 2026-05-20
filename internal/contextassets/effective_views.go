package contextassets

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// EffectiveViews captures the normalized runtime-facing context-asset views.
// EffectiveViews 描述归一化后的 runtime 可消费上下文资产视图。
type EffectiveViews struct {
	EffectivePersona      map[string]any   `json:"effective_persona,omitempty"`
	EffectiveAgentProfile map[string]any   `json:"effective_agent_profile,omitempty"`
	EffectiveUserProfile  map[string]any   `json:"effective_user_profile,omitempty"`
	EffectiveMemoryView   map[string]any   `json:"effective_memory_view,omitempty"`
	EffectiveScene        map[string]any   `json:"effective_scene,omitempty"`
	EffectiveWorkflow     map[string]any   `json:"effective_workflow,omitempty"`
	EffectivePolicyRules  []map[string]any `json:"effective_policy_rules,omitempty"`
	EffectiveContracts    []map[string]any `json:"effective_contracts,omitempty"`
	EffectiveSkills       []map[string]any `json:"effective_skills,omitempty"`
}

// Empty reports whether the normalized view is effectively empty.
// Empty 用于判断归一化视图是否为空。
func (v EffectiveViews) Empty() bool {
	return len(v.EffectivePersona) == 0 &&
		len(v.EffectiveAgentProfile) == 0 &&
		len(v.EffectiveUserProfile) == 0 &&
		len(v.EffectiveMemoryView) == 0 &&
		len(v.EffectiveScene) == 0 &&
		len(v.EffectiveWorkflow) == 0 &&
		len(v.EffectivePolicyRules) == 0 &&
		len(v.EffectiveContracts) == 0 &&
		len(v.EffectiveSkills) == 0
}

// AsGlobalContext converts effective views into global_context-compatible fields.
// AsGlobalContext 会把 effective views 转成可注入 global_context 的字段。
func (v EffectiveViews) AsGlobalContext() map[string]any {
	if v.Empty() {
		return nil
	}
	result := map[string]any{}
	if len(v.EffectivePersona) > 0 {
		result["effective_persona"] = cloneAnyMap(v.EffectivePersona)
	}
	if len(v.EffectiveAgentProfile) > 0 {
		result["effective_agent_profile"] = cloneAnyMap(v.EffectiveAgentProfile)
	}
	if len(v.EffectiveUserProfile) > 0 {
		result["effective_user_profile"] = cloneAnyMap(v.EffectiveUserProfile)
	}
	if len(v.EffectiveMemoryView) > 0 {
		result["effective_memory_view"] = cloneAnyMap(v.EffectiveMemoryView)
	}
	if len(v.EffectiveScene) > 0 {
		result["effective_scene"] = cloneAnyMap(v.EffectiveScene)
	}
	if len(v.EffectiveWorkflow) > 0 {
		result["effective_workflow"] = cloneAnyMap(v.EffectiveWorkflow)
	}
	if len(v.EffectivePolicyRules) > 0 {
		result["effective_policy_rules"] = cloneAnySlice(v.EffectivePolicyRules)
	}
	if len(v.EffectiveContracts) > 0 {
		result["effective_contracts"] = cloneAnySlice(v.EffectiveContracts)
	}
	if len(v.EffectiveSkills) > 0 {
		result["effective_skills"] = cloneAnySlice(v.EffectiveSkills)
	}
	return result
}

// GuidanceLines renders concise runtime guidance derived from effective views.
// GuidanceLines 负责把 effective views 渲染成简明 runtime guidance。
func (v EffectiveViews) GuidanceLines() []string {
	lines := make([]string, 0, 10)
	if len(v.EffectivePersona) > 0 {
		summary := firstNonEmptyString(v.EffectivePersona["summary"], v.EffectivePersona["role"], v.EffectivePersona["communication_style"])
		if summary != "" {
			lines = append(lines, fmt.Sprintf("Effective persona is active: %s", summary))
		}
	}
	if len(v.EffectiveAgentProfile) > 0 {
		if discipline := stringSlice(v.EffectiveAgentProfile["operational_discipline"]); len(discipline) > 0 {
			lines = append(lines, fmt.Sprintf("Effective agent discipline: %s", strings.Join(firstNStrings(discipline, 6), "; ")))
		}
	}
	if len(v.EffectiveUserProfile) > 0 {
		if summary := firstNonEmptyString(v.EffectiveUserProfile["identity_summary"], v.EffectiveUserProfile["role_summary"]); summary != "" {
			lines = append(lines, fmt.Sprintf("Effective user profile: %s", summary))
		}
	}
	if len(v.EffectiveMemoryView) > 0 {
		if summary := firstNonEmptyString(v.EffectiveMemoryView["summary"]); summary != "" {
			lines = append(lines, fmt.Sprintf("Effective memory view: %s", summary))
		}
	}
	if len(v.EffectiveScene) > 0 {
		if summary := firstNonEmptyString(v.EffectiveScene["summary"], v.EffectiveScene["description"]); summary != "" {
			lines = append(lines, fmt.Sprintf("Effective scene: %s", summary))
		}
	}
	if len(v.EffectiveWorkflow) > 0 {
		if stages := stringSlice(v.EffectiveWorkflow["stage_order"]); len(stages) > 0 {
			lines = append(lines, fmt.Sprintf("Effective workflow stages: %s", strings.Join(firstNStrings(stages, 8), " -> ")))
		}
	}
	if len(v.EffectivePolicyRules) > 0 {
		names := make([]string, 0, len(v.EffectivePolicyRules))
		for _, item := range v.EffectivePolicyRules {
			if name := firstNonEmptyString(item["title"], item["rule_id"], item["asset_name"]); name != "" {
				names = append(names, name)
			}
		}
		if len(names) > 0 {
			lines = append(lines, fmt.Sprintf("Effective policy rules: %s", strings.Join(firstNStrings(names, 8), ", ")))
		}
	}
	if len(v.EffectiveContracts) > 0 {
		names := make([]string, 0, len(v.EffectiveContracts))
		for _, item := range v.EffectiveContracts {
			if name := firstNonEmptyString(item["name"], item["contract_id"], item["asset_name"]); name != "" {
				names = append(names, name)
			}
		}
		if len(names) > 0 {
			lines = append(lines, fmt.Sprintf("Effective contracts: %s", strings.Join(firstNStrings(names, 8), ", ")))
		}
	}
	if len(v.EffectiveSkills) > 0 {
		names := make([]string, 0, len(v.EffectiveSkills))
		for _, item := range v.EffectiveSkills {
			if name := firstNonEmptyString(item["name"], item["skill_id"], item["asset_name"]); name != "" {
				names = append(names, name)
			}
		}
		if len(names) > 0 {
			lines = append(lines, fmt.Sprintf("Effective skills: %s", strings.Join(firstNStrings(names, 8), ", ")))
		}
	}
	return compactStrings(lines)
}

// BuildEffectiveViews normalizes the currently active context assets into runtime-facing views.
// BuildEffectiveViews 负责把当前生效的 context assets 归一化为 runtime-facing views。
func BuildEffectiveViews(bundle *Bundle, trace UsageTrace) EffectiveViews {
	views := EffectiveViews{}
	if bundle == nil {
		return views
	}

	activeSet := map[string]struct{}{}
	for _, item := range compactStrings(append(append([]string(nil), trace.UsedContextAssets...), append(trace.ResidentAssets, trace.OnDemandAssets...)...)) {
		activeSet[item] = struct{}{}
	}
	suppressedSet := map[string]struct{}{}
	for _, item := range trace.SuppressedAssets {
		suppressedSet[strings.TrimSpace(item)] = struct{}{}
	}
	resolvedIndex := map[string]ResolvedAsset{}
	for _, item := range bundle.ResolvedAssets {
		resolvedIndex[strings.TrimSpace(item.Asset.AssetID)] = item
	}

	assets := append([]Asset(nil), bundle.Assets...)
	sort.SliceStable(assets, func(i, j int) bool {
		if assets[i].Priority == assets[j].Priority {
			return strings.TrimSpace(assets[i].AssetID) < strings.TrimSpace(assets[j].AssetID)
		}
		return assets[i].Priority > assets[j].Priority
	})

	for _, asset := range assets {
		assetID := strings.TrimSpace(asset.AssetID)
		if assetID == "" {
			continue
		}
		if _, suppressed := suppressedSet[assetID]; suppressed {
			continue
		}
		if len(activeSet) > 0 {
			if _, active := activeSet[assetID]; !active {
				continue
			}
		} else if asset.Resolution.BackgroundOnly {
			continue
		}

		payload := normalizedAssetPayload(asset, resolvedIndex[assetID])
		if len(payload) == 0 {
			continue
		}
		switch strings.TrimSpace(asset.AssetType) {
		case "persona":
			if len(views.EffectivePersona) == 0 {
				views.EffectivePersona = payload
			}
		case "agent_profile":
			if len(views.EffectiveAgentProfile) == 0 {
				views.EffectiveAgentProfile = payload
			}
		case "user_profile":
			if len(views.EffectiveUserProfile) == 0 {
				views.EffectiveUserProfile = payload
			}
		case "memory_view":
			if len(views.EffectiveMemoryView) == 0 {
				views.EffectiveMemoryView = payload
			}
		case "scene":
			if len(views.EffectiveScene) == 0 {
				views.EffectiveScene = payload
			}
		case "workflow":
			if len(views.EffectiveWorkflow) == 0 {
				views.EffectiveWorkflow = payload
			}
		case "policy_rule":
			views.EffectivePolicyRules = append(views.EffectivePolicyRules, payload)
		case "contract":
			views.EffectiveContracts = append(views.EffectiveContracts, payload)
		case "skill":
			views.EffectiveSkills = append(views.EffectiveSkills, payload)
		}
	}
	return views
}

// BuildEffectiveViewsFromGlobalContext reconstructs normalized views from one global_context payload.
// BuildEffectiveViewsFromGlobalContext 负责从 global_context 中恢复 normalized views。
func BuildEffectiveViewsFromGlobalContext(globalContext map[string]any) EffectiveViews {
	views := EffectiveViews{
		EffectivePersona:      decodeAnyMap(globalContext["effective_persona"]),
		EffectiveAgentProfile: decodeAnyMap(globalContext["effective_agent_profile"]),
		EffectiveUserProfile:  decodeAnyMap(globalContext["effective_user_profile"]),
		EffectiveMemoryView:   decodeAnyMap(globalContext["effective_memory_view"]),
		EffectiveScene:        decodeAnyMap(globalContext["effective_scene"]),
		EffectiveWorkflow:     decodeAnyMap(globalContext["effective_workflow"]),
		EffectivePolicyRules:  decodeAnyMapSlice(globalContext["effective_policy_rules"]),
		EffectiveContracts:    decodeAnyMapSlice(globalContext["effective_contracts"]),
		EffectiveSkills:       decodeAnyMapSlice(globalContext["effective_skills"]),
	}
	if !views.Empty() {
		return views
	}
	bundle := BuildBundle(globalContext)
	if bundle == nil {
		return EffectiveViews{}
	}
	trace := bundle.ResolveUsage(UsageInput{})
	return BuildEffectiveViews(bundle, trace)
}

func normalizedAssetPayload(asset Asset, resolved ResolvedAsset) map[string]any {
	payload := cloneAnyMap(resolved.Payload)
	if len(payload) == 0 {
		payload = cloneAnyMap(asset.Content)
	}
	if len(payload) == 0 {
		payload = map[string]any{}
	}
	if _, ok := payload["asset_id"]; !ok {
		payload["asset_id"] = asset.AssetID
	}
	if _, ok := payload["asset_type"]; !ok {
		payload["asset_type"] = asset.AssetType
	}
	if _, ok := payload["asset_name"]; !ok && strings.TrimSpace(asset.AssetName) != "" {
		payload["asset_name"] = asset.AssetName
	}
	if _, ok := payload["scope"]; !ok && strings.TrimSpace(asset.Scope) != "" {
		payload["scope"] = asset.Scope
	}
	if _, ok := payload["source_kind"]; !ok && strings.TrimSpace(asset.SourceKind) != "" {
		payload["source_kind"] = asset.SourceKind
	}
	if _, ok := payload["summary"]; !ok && strings.TrimSpace(resolved.Summary) != "" {
		payload["summary"] = resolved.Summary
	}
	if _, ok := payload["guidance_text"]; !ok && strings.TrimSpace(resolved.GuidanceText) != "" {
		payload["guidance_text"] = resolved.GuidanceText
	}
	if _, ok := payload["compiled_version"]; !ok && strings.TrimSpace(resolved.CompiledVersion) != "" {
		payload["compiled_version"] = resolved.CompiledVersion
	}
	if _, ok := payload["truth_dir_version"]; !ok && strings.TrimSpace(resolved.TruthDirVersion) != "" {
		payload["truth_dir_version"] = resolved.TruthDirVersion
	}
	if _, ok := payload["loaded_from_ref"]; !ok && resolved.LoadedFromRef {
		payload["loaded_from_ref"] = true
	}
	return payload
}

func decodeAnyMap(value any) map[string]any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneAnyMap(typed)
	case nil:
		return nil
	default:
		payload, err := json.Marshal(typed)
		if err != nil {
			return nil
		}
		var result map[string]any
		if err := json.Unmarshal(payload, &result); err != nil {
			return nil
		}
		return result
	}
}

func decodeAnyMapSlice(value any) []map[string]any {
	switch typed := value.(type) {
	case []map[string]any:
		return cloneAnySlice(typed)
	case []any:
		result := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			if decoded := decodeAnyMap(item); len(decoded) > 0 {
				result = append(result, decoded)
			}
		}
		return result
	default:
		payload, err := json.Marshal(typed)
		if err != nil {
			return nil
		}
		var result []map[string]any
		if err := json.Unmarshal(payload, &result); err != nil {
			return nil
		}
		return result
	}
}

func stringSlice(value any) []string {
	switch typed := value.(type) {
	case []string:
		return compactStrings(typed)
	case []any:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			if text, ok := item.(string); ok {
				result = append(result, text)
			}
		}
		return compactStrings(result)
	default:
		return nil
	}
}

func firstNonEmptyString(values ...any) string {
	for _, item := range values {
		if text, ok := item.(string); ok && strings.TrimSpace(text) != "" {
			return strings.TrimSpace(text)
		}
	}
	return ""
}

func firstNStrings(values []string, limit int) []string {
	values = compactStrings(values)
	if len(values) <= limit {
		return values
	}
	return append([]string(nil), values[:limit]...)
}

