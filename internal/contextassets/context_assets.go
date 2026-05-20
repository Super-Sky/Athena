// context_assets.go parses request-level context_assets bindings and derives runtime usage traces.
// context_assets.go 负责解析请求级 context_assets 绑定，并生成运行时使用痕迹。
package contextassets

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Asset captures one request-level context asset binding.
// Asset 描述一条请求级上下文资产绑定。
type Asset struct {
	AssetID           string         `json:"asset_id,omitempty"`
	AssetType         string         `json:"asset_type,omitempty"`
	AssetName         string         `json:"asset_name,omitempty"`
	Scope             string         `json:"scope,omitempty"`
	SourceKind        string         `json:"source_kind,omitempty"`
	Mode              string         `json:"mode,omitempty"`
	Priority          int            `json:"priority,omitempty"`
	ReadOnly          bool           `json:"read_only,omitempty"`
	CandidateWritable bool           `json:"candidate_writable,omitempty"`
	AuthScope         string         `json:"auth_scope,omitempty"`
	Content           map[string]any `json:"content,omitempty"`
	Ref               *Ref           `json:"ref,omitempty"`
	Resolution        Resolution     `json:"resolution,omitempty"`
	Metadata          Metadata       `json:"metadata,omitempty"`
}

// Ref captures one ref-first binding target.
// Ref 描述一条 ref-first 绑定目标。
type Ref struct {
	RefType         string `json:"ref_type,omitempty"`
	Target          string `json:"target,omitempty"`
	Version         string `json:"version,omitempty"`
	Checksum        string `json:"checksum,omitempty"`
	TruthDirVersion string `json:"truth_dir_version,omitempty"`
	DetailEndpoint  string `json:"detail_endpoint,omitempty"`
}

// Resolution captures the runtime resolution policy for one asset.
// Resolution 描述单条资产的运行时解析策略。
type Resolution struct {
	PreferCompiled      bool `json:"prefer_compiled,omitempty"`
	AllowDetailFetch    bool `json:"allow_detail_fetch,omitempty"`
	AllowInlineFallback bool `json:"allow_inline_fallback,omitempty"`
	ResidentHint        bool `json:"resident_hint,omitempty"`
	BackgroundOnly      bool `json:"background_only,omitempty"`
}

// Metadata captures UI and audit helper fields for one asset binding.
// Metadata 描述一条资产绑定附带的 UI 与审计辅助字段。
type Metadata struct {
	SourceLabel string   `json:"source_label,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

// ResolvedAsset captures one hydrated runtime-ready asset.
// ResolvedAsset 描述一条已经水合、可被运行时直接消费的资产。
type ResolvedAsset struct {
	Asset            Asset          `json:"asset"`
	Summary          string         `json:"summary,omitempty"`
	GuidanceText     string         `json:"guidance_text,omitempty"`
	SourceContent    string         `json:"source_content,omitempty"`
	CompiledVersion  string         `json:"compiled_version,omitempty"`
	CompiledChecksum string         `json:"compiled_checksum,omitempty"`
	TruthDirVersion  string         `json:"truth_dir_version,omitempty"`
	Payload          map[string]any `json:"payload,omitempty"`
	LoadedFromRef    bool           `json:"loaded_from_ref,omitempty"`
}

// Bundle captures both raw bindings and their resolved runtime views.
// Bundle 描述原始绑定及其对应的运行时解析视图。
type Bundle struct {
	Assets         []Asset
	ResolvedAssets []ResolvedAsset
}

// UsageInput captures minimal routing signals for asset relevance decisions.
// UsageInput 描述做资产相关性判断时需要的最小路由信号。
type UsageInput struct {
	Query             string
	TaskType          string
	Scene             string
	DesiredOutputMode string
}

// BindingOverrides captures request-scoped customization over context asset bindings.
// BindingOverrides 描述当前请求对 context asset binding 的临时覆盖规则。
type BindingOverrides struct {
	ContextAssetOverrides  []Asset
	DisabledAssetTypes     []string
	AssetPriorityOverrides map[string]int
}

// UsageTrace captures runtime-visible usage and suppression outcomes.
// UsageTrace 描述运行时可观测的使用与压制结果。
type UsageTrace struct {
	UsedContextAssets      []string         `json:"used_context_assets,omitempty"`
	ResidentAssets         []string         `json:"resident_assets,omitempty"`
	OnDemandAssets         []string         `json:"on_demand_assets,omitempty"`
	SuppressedAssets       []string         `json:"suppressed_assets,omitempty"`
	AssetConflictsResolved []map[string]any `json:"asset_conflicts_resolved,omitempty"`
	RequestedAssetDetails  []string         `json:"requested_asset_details,omitempty"`
	LoadedAssetDetails     []string         `json:"loaded_asset_details,omitempty"`
	CandidateAssetTargets  []map[string]any `json:"candidate_asset_targets,omitempty"`
	CandidateAssetDiffs    []map[string]any `json:"candidate_asset_diffs,omitempty"`
	CandidateAssetUpdates  []map[string]any `json:"candidate_asset_updates,omitempty"`
	AssetUsageTrace        []map[string]any `json:"asset_usage_trace,omitempty"`
	GuidanceLines          []string         `json:"-"`
}

// CandidateInput captures the minimal signals used to build candidate update artifacts.
// CandidateInput 描述构建 candidate update 产物所需的最小输入。
type CandidateInput struct {
	Query      string
	MainAnswer string
	Answer     string
	TaskType   string
	Scene      string
}

// BuildBundle parses one context_assets bundle from global_context.
// BuildBundle 负责从 global_context 中解析一份 context_assets 资产集合。
func BuildBundle(globalContext map[string]any) *Bundle {
	if len(globalContext) == 0 {
		return nil
	}
	rawAssets := parseAssets(globalContext["context_assets"])
	rawResolved := parseResolvedAssets(globalContext["context_assets_resolved"])
	if len(rawAssets) == 0 && len(rawResolved) == 0 {
		return nil
	}
	return &Bundle{
		Assets:         rawAssets,
		ResolvedAssets: rawResolved,
	}
}

// ApplyOverrides returns one derived bundle with request-level binding overrides applied.
// ApplyOverrides 返回一份应用了请求级 binding override 的派生 bundle。
func (b *Bundle) ApplyOverrides(overrides BindingOverrides) *Bundle {
	if b == nil {
		return nil
	}
	assets := append([]Asset(nil), b.Assets...)
	resolved := append([]ResolvedAsset(nil), b.ResolvedAssets...)
	disabledTypes := map[string]struct{}{}
	for _, item := range overrides.DisabledAssetTypes {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		disabledTypes[item] = struct{}{}
	}
	if len(disabledTypes) > 0 {
		filteredAssets := make([]Asset, 0, len(assets))
		for _, asset := range assets {
			if _, blocked := disabledTypes[strings.TrimSpace(asset.AssetType)]; blocked {
				continue
			}
			filteredAssets = append(filteredAssets, asset)
		}
		assets = filteredAssets
		filteredResolved := make([]ResolvedAsset, 0, len(resolved))
		for _, item := range resolved {
			if _, blocked := disabledTypes[strings.TrimSpace(item.Asset.AssetType)]; blocked {
				continue
			}
			filteredResolved = append(filteredResolved, item)
		}
		resolved = filteredResolved
	}
	if len(overrides.ContextAssetOverrides) > 0 {
		indexByID := map[string]int{}
		for idx, asset := range assets {
			assetID := strings.TrimSpace(asset.AssetID)
			if assetID == "" {
				continue
			}
			indexByID[assetID] = idx
		}
		for _, override := range overrides.ContextAssetOverrides {
			override.AssetID = strings.TrimSpace(override.AssetID)
			override.AssetType = strings.TrimSpace(override.AssetType)
			override.AssetName = strings.TrimSpace(override.AssetName)
			override.Scope = strings.TrimSpace(override.Scope)
			override.SourceKind = strings.TrimSpace(override.SourceKind)
			override.Mode = strings.TrimSpace(override.Mode)
			override.AuthScope = strings.TrimSpace(override.AuthScope)
			override.Metadata.Tags = compactStrings(override.Metadata.Tags)
			if override.AssetID == "" {
				continue
			}
			if idx, ok := indexByID[override.AssetID]; ok {
				assets[idx] = override
			} else {
				indexByID[override.AssetID] = len(assets)
				assets = append(assets, override)
			}
		}
		if len(resolved) > 0 {
			filteredResolved := make([]ResolvedAsset, 0, len(resolved))
			overrideIDs := map[string]struct{}{}
			for _, item := range overrides.ContextAssetOverrides {
				if id := strings.TrimSpace(item.AssetID); id != "" {
					overrideIDs[id] = struct{}{}
				}
			}
			for _, item := range resolved {
				if _, replaced := overrideIDs[strings.TrimSpace(item.Asset.AssetID)]; replaced {
					continue
				}
				filteredResolved = append(filteredResolved, item)
			}
			resolved = filteredResolved
		}
	}
	if len(overrides.AssetPriorityOverrides) > 0 {
		for idx := range assets {
			assetID := strings.TrimSpace(assets[idx].AssetID)
			assetType := strings.TrimSpace(assets[idx].AssetType)
			if priority, ok := overrides.AssetPriorityOverrides[assetID]; ok {
				assets[idx].Priority = priority
				continue
			}
			if priority, ok := overrides.AssetPriorityOverrides[assetType]; ok {
				assets[idx].Priority = priority
			}
		}
	}
	return &Bundle{
		Assets:         assets,
		ResolvedAssets: resolved,
	}
}

// ResolveUsage returns the effective runtime usage trace for the current request.
// ResolveUsage 返回当前请求的有效运行时使用痕迹。
func (b *Bundle) ResolveUsage(_ UsageInput) UsageTrace {
	trace := UsageTrace{}
	if b == nil {
		return trace
	}

	resolvedIndex := map[string]ResolvedAsset{}
	for _, item := range b.ResolvedAssets {
		resolvedIndex[strings.TrimSpace(item.Asset.AssetID)] = item
	}

	assets := append([]Asset(nil), b.Assets...)
	sort.SliceStable(assets, func(i, j int) bool {
		if assets[i].Priority == assets[j].Priority {
			return assets[i].AssetID < assets[j].AssetID
		}
		return assets[i].Priority > assets[j].Priority
	})

	singletonWinner := map[string]string{}
	for _, asset := range assets {
		assetID := strings.TrimSpace(asset.AssetID)
		if assetID == "" {
			continue
		}
		if asset.Resolution.BackgroundOnly {
			trace.AssetUsageTrace = append(trace.AssetUsageTrace, map[string]any{
				"asset_id": assetID,
				"status":   "background_only",
			})
			continue
		}
		if singletonAssetType(asset.AssetType) {
			if winner, exists := singletonWinner[asset.AssetType]; exists {
				trace.SuppressedAssets = append(trace.SuppressedAssets, assetID)
				trace.AssetConflictsResolved = append(trace.AssetConflictsResolved, map[string]any{
					"asset_type":          asset.AssetType,
					"winner_asset_id":     winner,
					"suppressed_asset_id": assetID,
					"reason":              "higher_priority_singleton_asset",
				})
				continue
			}
			singletonWinner[asset.AssetType] = assetID
		}

		resolved, hasResolved := resolvedIndex[assetID]
		if hasResolved && resolved.LoadedFromRef {
			trace.LoadedAssetDetails = append(trace.LoadedAssetDetails, assetID)
		} else if asset.Ref != nil && asset.Resolution.AllowDetailFetch {
			trace.RequestedAssetDetails = append(trace.RequestedAssetDetails, assetID)
		}

		if asset.Resolution.ResidentHint || hasTag(asset.Metadata.Tags, "resident") {
			trace.ResidentAssets = append(trace.ResidentAssets, assetID)
		} else {
			trace.OnDemandAssets = append(trace.OnDemandAssets, assetID)
		}
		trace.UsedContextAssets = append(trace.UsedContextAssets, assetID)
		trace.AssetUsageTrace = append(trace.AssetUsageTrace, map[string]any{
			"asset_id":         assetID,
			"asset_type":       asset.AssetType,
			"scope":            asset.Scope,
			"source_kind":      asset.SourceKind,
			"loaded_from_ref":  hasResolved && resolved.LoadedFromRef,
			"used_as_resident": containsString(trace.ResidentAssets, assetID),
		})
		if line := guidanceLine(asset, resolved); line != "" {
			trace.GuidanceLines = append(trace.GuidanceLines, line)
		}
	}

	trace.UsedContextAssets = compactStrings(trace.UsedContextAssets)
	trace.ResidentAssets = compactStrings(trace.ResidentAssets)
	trace.OnDemandAssets = compactStrings(trace.OnDemandAssets)
	trace.SuppressedAssets = compactStrings(trace.SuppressedAssets)
	trace.RequestedAssetDetails = compactStrings(trace.RequestedAssetDetails)
	trace.LoadedAssetDetails = compactStrings(trace.LoadedAssetDetails)
	trace.GuidanceLines = compactStrings(trace.GuidanceLines)
	return trace
}

// ResolvedMaps converts resolved assets into one JSON-compatible payload for global_context.
// ResolvedMaps 会把 resolved assets 转成可放入 global_context 的 JSON 兼容对象。
func ResolvedMaps(items []ResolvedAsset) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, map[string]any{
			"asset": map[string]any{
				"asset_id":           item.Asset.AssetID,
				"asset_type":         item.Asset.AssetType,
				"asset_name":         item.Asset.AssetName,
				"scope":              item.Asset.Scope,
				"source_kind":        item.Asset.SourceKind,
				"mode":               item.Asset.Mode,
				"priority":           item.Asset.Priority,
				"read_only":          item.Asset.ReadOnly,
				"candidate_writable": item.Asset.CandidateWritable,
				"auth_scope":         item.Asset.AuthScope,
				"content":            cloneAnyMap(item.Asset.Content),
				"ref":                refMap(item.Asset.Ref),
				"resolution": map[string]any{
					"prefer_compiled":       item.Asset.Resolution.PreferCompiled,
					"allow_detail_fetch":    item.Asset.Resolution.AllowDetailFetch,
					"allow_inline_fallback": item.Asset.Resolution.AllowInlineFallback,
					"resident_hint":         item.Asset.Resolution.ResidentHint,
					"background_only":       item.Asset.Resolution.BackgroundOnly,
				},
				"metadata": map[string]any{
					"source_label": item.Asset.Metadata.SourceLabel,
					"tags":         append([]string(nil), item.Asset.Metadata.Tags...),
				},
			},
			"summary":           item.Summary,
			"guidance_text":     item.GuidanceText,
			"source_content":    item.SourceContent,
			"compiled_version":  item.CompiledVersion,
			"compiled_checksum": item.CompiledChecksum,
			"truth_dir_version": item.TruthDirVersion,
			"payload":           cloneAnyMap(item.Payload),
			"loaded_from_ref":   item.LoadedFromRef,
		})
	}
	return result
}

// AssetMaps converts raw assets into JSON-compatible payloads.
// AssetMaps 会把原始资产对象转换为 JSON 兼容负载。
func AssetMaps(items []Asset) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, map[string]any{
			"asset_id":           item.AssetID,
			"asset_type":         item.AssetType,
			"asset_name":         item.AssetName,
			"scope":              item.Scope,
			"source_kind":        item.SourceKind,
			"mode":               item.Mode,
			"priority":           item.Priority,
			"read_only":          item.ReadOnly,
			"candidate_writable": item.CandidateWritable,
			"auth_scope":         item.AuthScope,
			"content":            cloneAnyMap(item.Content),
			"ref":                refMap(item.Ref),
			"resolution": map[string]any{
				"prefer_compiled":       item.Resolution.PreferCompiled,
				"allow_detail_fetch":    item.Resolution.AllowDetailFetch,
				"allow_inline_fallback": item.Resolution.AllowInlineFallback,
				"resident_hint":         item.Resolution.ResidentHint,
				"background_only":       item.Resolution.BackgroundOnly,
			},
			"metadata": map[string]any{
				"source_label": item.Metadata.SourceLabel,
				"tags":         append([]string(nil), item.Metadata.Tags...),
			},
		})
	}
	return result
}

// BuildCandidateTrace enriches one usage trace with candidate targets, diffs, and update payloads.
// BuildCandidateTrace 会把 target、diff 和 update 产物补进当前 usage trace。
func BuildCandidateTrace(bundle *Bundle, trace UsageTrace, input CandidateInput) UsageTrace {
	if bundle == nil {
		return trace
	}
	resolvedIndex := map[string]ResolvedAsset{}
	for _, item := range bundle.ResolvedAssets {
		resolvedIndex[strings.TrimSpace(item.Asset.AssetID)] = item
	}
	query := strings.TrimSpace(input.Query)
	answer := strings.TrimSpace(defaultString(input.MainAnswer, input.Answer))
	if query == "" && answer == "" {
		return trace
	}
	seenTargets := map[string]struct{}{}
	for _, asset := range bundle.Assets {
		assetID := strings.TrimSpace(asset.AssetID)
		if assetID == "" || !asset.CandidateWritable || asset.Resolution.BackgroundOnly {
			continue
		}
		if _, ok := seenTargets[assetID]; ok {
			continue
		}
		seenTargets[assetID] = struct{}{}
		resolved := resolvedIndex[assetID]
		beforeSummary := strings.TrimSpace(resolved.Summary)
		if beforeSummary == "" {
			beforeSummary = strings.TrimSpace(fmt.Sprintf("%v", asset.Content["summary"]))
		}
		proposedSummary := candidateSummary(query, answer, beforeSummary)
		target := map[string]any{
			"asset_id":    assetID,
			"asset_type":  asset.AssetType,
			"asset_name":  asset.AssetName,
			"scope":       asset.Scope,
			"source_kind": asset.SourceKind,
			"read_only":   asset.ReadOnly,
		}
		if asset.Ref != nil {
			target["ref"] = refMap(asset.Ref)
		}
		diff := map[string]any{
			"asset_id":         assetID,
			"before_summary":   beforeSummary,
			"proposed_summary": proposedSummary,
			"change_kind":      "candidate_summary_update",
		}
		update := map[string]any{
			"asset_id":   assetID,
			"asset_type": asset.AssetType,
			"task_type":  strings.TrimSpace(input.TaskType),
			"scene":      strings.TrimSpace(input.Scene),
			"operation":  "candidate_update",
			"proposed_content": map[string]any{
				"summary":          proposedSummary,
				"supporting_query": query,
				"answer_excerpt":   excerpt(answer, 280),
			},
		}
		trace.CandidateAssetTargets = append(trace.CandidateAssetTargets, target)
		trace.CandidateAssetDiffs = append(trace.CandidateAssetDiffs, diff)
		trace.CandidateAssetUpdates = append(trace.CandidateAssetUpdates, update)
	}
	return trace
}

func parseAssets(raw any) []Asset {
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	result := make([]Asset, 0, len(items))
	for _, item := range items {
		asset := Asset{}
		payload, err := json.Marshal(item)
		if err != nil {
			continue
		}
		if err := json.Unmarshal(payload, &asset); err != nil {
			continue
		}
		asset.AssetID = strings.TrimSpace(asset.AssetID)
		asset.AssetType = strings.TrimSpace(asset.AssetType)
		asset.AssetName = strings.TrimSpace(asset.AssetName)
		asset.Scope = strings.TrimSpace(asset.Scope)
		asset.SourceKind = strings.TrimSpace(asset.SourceKind)
		asset.Mode = strings.TrimSpace(asset.Mode)
		asset.AuthScope = strings.TrimSpace(asset.AuthScope)
		asset.Metadata.Tags = compactStrings(asset.Metadata.Tags)
		result = append(result, asset)
	}
	return result
}

func parseResolvedAssets(raw any) []ResolvedAsset {
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	result := make([]ResolvedAsset, 0, len(items))
	for _, item := range items {
		payload, err := json.Marshal(item)
		if err != nil {
			continue
		}
		var resolved ResolvedAsset
		if err := json.Unmarshal(payload, &resolved); err != nil {
			continue
		}
		resolved.Asset.AssetID = strings.TrimSpace(resolved.Asset.AssetID)
		resolved.Asset.AssetType = strings.TrimSpace(resolved.Asset.AssetType)
		resolved.Asset.AssetName = strings.TrimSpace(resolved.Asset.AssetName)
		result = append(result, resolved)
	}
	return result
}

func singletonAssetType(assetType string) bool {
	switch strings.TrimSpace(assetType) {
	case "persona", "agent_profile", "user_profile", "memory_view", "scene", "workflow":
		return true
	default:
		return false
	}
}

func guidanceLine(asset Asset, resolved ResolvedAsset) string {
	summary := strings.TrimSpace(resolved.Summary)
	if summary == "" {
		summary = strings.TrimSpace(fmt.Sprintf("%v", asset.Content["summary"]))
	}
	text := strings.TrimSpace(resolved.GuidanceText)
	if text == "" {
		text = summary
	}
	if text == "" {
		return ""
	}
	return fmt.Sprintf("Context asset %s (%s) is active: %s", asset.AssetID, defaultString(asset.AssetType, "unknown"), text)
}

func hasTag(tags []string, target string) bool {
	target = strings.TrimSpace(target)
	for _, item := range tags {
		if strings.TrimSpace(item) == target {
			return true
		}
	}
	return false
}

func containsString(items []string, target string) bool {
	target = strings.TrimSpace(target)
	for _, item := range items {
		if strings.TrimSpace(item) == target {
			return true
		}
	}
	return false
}

func compactStrings(values []string) []string {
	result := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func cloneAnyMap(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	result := make(map[string]any, len(input))
	for key, value := range input {
		result[key] = value
	}
	return result
}

func cloneAnySlice(input []map[string]any) []map[string]any {
	if len(input) == 0 {
		return nil
	}
	result := make([]map[string]any, 0, len(input))
	for _, item := range input {
		result = append(result, cloneAnyMap(item))
	}
	return result
}

func refMap(ref *Ref) map[string]any {
	if ref == nil {
		return nil
	}
	return map[string]any{
		"ref_type":          ref.RefType,
		"target":            ref.Target,
		"version":           ref.Version,
		"checksum":          ref.Checksum,
		"truth_dir_version": ref.TruthDirVersion,
		"detail_endpoint":   ref.DetailEndpoint,
	}
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

func candidateSummary(query, answer, fallback string) string {
	switch {
	case strings.TrimSpace(answer) != "":
		return excerpt(strings.TrimSpace(answer), 240)
	case strings.TrimSpace(query) != "":
		return fmt.Sprintf("Candidate update derived from request: %s", excerpt(strings.TrimSpace(query), 220))
	default:
		return strings.TrimSpace(fallback)
	}
}

func excerpt(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return strings.TrimSpace(value[:limit]) + "..."
}
