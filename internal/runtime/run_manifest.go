// run_manifest.go freezes safe execution revision references at TaskRun creation time.
// run_manifest.go 在 TaskRun 创建时冻结安全的执行版本引用。
package runtime

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
	"time"
)

const (
	// RunManifestSchemaVersion identifies the first immutable run-manifest contract.
	// RunManifestSchemaVersion 标识第一版不可变运行清单契约。
	RunManifestSchemaVersion = "agent_run_manifest.v1"
)

// RunRevisionRef identifies immutable execution input without retaining its raw content.
// RunRevisionRef 用摘要而非原文标识不可变执行输入。
type RunRevisionRef struct {
	Kind          string `json:"kind"`
	ID            string `json:"id"`
	Version       string `json:"version,omitempty"`
	ContentSHA256 string `json:"content_sha256,omitempty"`
	Source        string `json:"source,omitempty"`
}

// RunManifestReferences carries resolver-owned references into the persistence boundary.
// RunManifestReferences 把 resolver 已解析的版本引用传递到持久化边界。
type RunManifestReferences struct {
	Prompt          *RunRevisionRef  `json:"prompt,omitempty"`
	Governance      []RunRevisionRef `json:"governance,omitempty"`
	ContextAssets   []RunRevisionRef `json:"context_assets,omitempty"`
	Evaluators      []RunRevisionRef `json:"evaluators,omitempty"`
	SystemTruth     []RunRevisionRef `json:"system_truth,omitempty"`
	RuntimeContract *RunRevisionRef  `json:"runtime_contract,omitempty"`
}

// RunManifest is the immutable, redaction-safe execution revision envelope.
// RunManifest 是不可变且可安全展示的执行版本包络。
type RunManifest struct {
	SchemaVersion   string           `json:"schema_version"`
	Status          string           `json:"status"`
	CapturedAt      time.Time        `json:"captured_at"`
	ManifestSHA256  string           `json:"manifest_sha256"`
	Model           *RunRevisionRef  `json:"model,omitempty"`
	Prompt          *RunRevisionRef  `json:"prompt,omitempty"`
	Skills          []RunRevisionRef `json:"skills,omitempty"`
	Tools           []RunRevisionRef `json:"tools,omitempty"`
	Governance      []RunRevisionRef `json:"governance,omitempty"`
	ContextAssets   []RunRevisionRef `json:"context_assets,omitempty"`
	Evaluators      []RunRevisionRef `json:"evaluators,omitempty"`
	SystemTruth     []RunRevisionRef `json:"system_truth,omitempty"`
	RuntimeContract *RunRevisionRef  `json:"runtime_contract,omitempty"`
	Missing         []string         `json:"missing,omitempty"`
}

// NewRunRevisionRef creates one canonical revision reference and hashes payload without retaining it.
// NewRunRevisionRef 创建规范版本引用，并仅保留 payload 摘要。
func NewRunRevisionRef(kind, id, version, source string, payload any) RunRevisionRef {
	return RunRevisionRef{
		Kind: strings.TrimSpace(kind), ID: strings.TrimSpace(id), Version: strings.TrimSpace(version),
		ContentSHA256: stableRunManifestDigest(payload), Source: strings.TrimSpace(source),
	}
}

// BuildRunManifest freezes the effective ExecutionSpec into a safe manifest.
// BuildRunManifest 把实际 ExecutionSpec 冻结为安全运行清单。
func BuildRunManifest(spec *ExecutionSpec, capturedAt time.Time) RunManifest {
	manifest := RunManifest{SchemaVersion: RunManifestSchemaVersion, CapturedAt: capturedAt.UTC()}
	if spec == nil {
		manifest.Status = "partial"
		manifest.Missing = []string{"execution_spec"}
		manifest.ManifestSHA256 = stableRunManifestDigest(manifestDigestPayload(manifest))
		return manifest
	}
	endpoint := spec.Model.Executed
	if strings.TrimSpace(endpoint.ProviderModelID) == "" && strings.TrimSpace(endpoint.ModelRecordID) == "" {
		endpoint = spec.Model.Requested
	}
	modelID := firstNonEmptyRunManifestValue(endpoint.ModelRecordID, endpoint.ProviderModelID, endpoint.ModelDisplayName)
	if modelID != "" {
		ref := NewRunRevisionRef("model", modelID, endpoint.ProviderModelID, endpoint.ProviderID, runModelDigestPayload(spec, endpoint))
		manifest.Model = &ref
	} else {
		manifest.Missing = append(manifest.Missing, "model")
	}
	if spec.Metadata.ManifestRefs.Prompt != nil {
		manifest.Prompt = cloneRunRevisionRef(spec.Metadata.ManifestRefs.Prompt)
	} else if guidance := strings.TrimSpace(spec.Skill.Guidance); guidance != "" {
		ref := NewRunRevisionRef("prompt", "resolved_skill_guidance", RunManifestSchemaVersion, "runtime_resolver", guidance)
		manifest.Prompt = &ref
	} else {
		manifest.Missing = append(manifest.Missing, "prompt")
	}
	manifest.Skills = canonicalRunRevisionRefs(spec.Skill.RevisionRefs)
	manifest.Tools = canonicalRunRevisionRefs(spec.Tools.RevisionRefs)
	manifest.Governance = canonicalRunRevisionRefs(spec.Metadata.ManifestRefs.Governance)
	manifest.ContextAssets = canonicalRunRevisionRefs(spec.Metadata.ManifestRefs.ContextAssets)
	manifest.Evaluators = canonicalRunRevisionRefs(spec.Metadata.ManifestRefs.Evaluators)
	manifest.SystemTruth = canonicalRunRevisionRefs(spec.Metadata.ManifestRefs.SystemTruth)
	manifest.RuntimeContract = cloneRunRevisionRef(spec.Metadata.ManifestRefs.RuntimeContract)
	if len(manifest.Skills) < 1+len(spec.Skill.AuxiliarySkills) || hasIncompleteRunRevisionRef(manifest.Skills) {
		manifest.Missing = append(manifest.Missing, "skills")
	}
	if len(spec.Tools.AllowedTools) > 0 && (len(manifest.Tools) < len(spec.Tools.AllowedTools) || hasIncompleteRunRevisionRef(manifest.Tools)) {
		manifest.Missing = append(manifest.Missing, "tools")
	}
	if expectedRunManifestRefs(spec.Metadata.Constraints, "used_context_assets") > 0 && len(manifest.ContextAssets) == 0 {
		manifest.Missing = append(manifest.Missing, "context_assets")
	}
	if firstNonEmptyRunManifestValue(anyStringValue(spec.Metadata.Constraints["runtime_contract_id"]), anyStringValue(spec.Metadata.Constraints["contract_id"])) != "" && manifest.RuntimeContract == nil {
		manifest.Missing = append(manifest.Missing, "runtime_contract")
	}
	if anyStringValue(spec.Metadata.Constraints["model_policy"]) != "" && len(manifest.Governance) == 0 {
		manifest.Missing = append(manifest.Missing, "governance")
	}
	manifest.Status = "complete"
	if len(manifest.Missing) > 0 {
		manifest.Status = "partial"
	}
	manifest.ManifestSHA256 = stableRunManifestDigest(manifestDigestPayload(manifest))
	return manifest
}

func runModelDigestPayload(spec *ExecutionSpec, endpoint ModelEndpoint) map[string]any {
	payload := map[string]any{
		"provider_id": endpoint.ProviderID, "provider_name": endpoint.ProviderName,
		"provider_protocol": endpoint.ProviderProtocol, "model_record_id": endpoint.ModelRecordID,
		"provider_model_id": endpoint.ProviderModelID, "model_display_name": endpoint.ModelDisplayName,
		"fallback_used": spec.Model.FallbackUsed, "resolved_parameters": spec.Model.ResolvedParameters,
	}
	if config := spec.Model.ExecutedConfig; config != nil {
		payload["execution_config"] = map[string]any{
			"provider_id": config.ProviderID, "provider_name": config.ProviderName,
			"provider_protocol": config.ProviderProtocol, "endpoint_identity": config.BaseURL,
			"request_timeout_ns": config.RequestTimeout.Nanoseconds(),
			"model_record_id":    config.ModelRecordID, "provider_model_id": config.ProviderModelID,
			"model_display_name": config.ModelDisplayName,
		}
	}
	return payload
}

func hasIncompleteRunRevisionRef(refs []RunRevisionRef) bool {
	for _, ref := range refs {
		if strings.TrimSpace(ref.ID) == "" || strings.TrimSpace(ref.ContentSHA256) == "" {
			return true
		}
	}
	return false
}

func expectedRunManifestRefs(constraints map[string]any, key string) int {
	value := constraints[key]
	switch items := value.(type) {
	case []string:
		return len(items)
	case []any:
		return len(items)
	default:
		return 0
	}
}

// ValidRunManifestDigest verifies that persisted manifest content still matches its frozen digest.
// ValidRunManifestDigest 验证持久化清单内容仍与冻结摘要一致。
func ValidRunManifestDigest(manifest RunManifest) bool {
	want := strings.TrimSpace(manifest.ManifestSHA256)
	return want != "" && want == stableRunManifestDigest(manifestDigestPayload(manifest))
}

func manifestDigestPayload(manifest RunManifest) RunManifest {
	manifest.ManifestSHA256 = ""
	return manifest
}

func stableRunManifestDigest(value any) string {
	payload, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func canonicalRunRevisionRefs(items []RunRevisionRef) []RunRevisionRef {
	result := append([]RunRevisionRef(nil), items...)
	sort.Slice(result, func(i, j int) bool {
		if result[i].Kind != result[j].Kind {
			return result[i].Kind < result[j].Kind
		}
		if result[i].ID != result[j].ID {
			return result[i].ID < result[j].ID
		}
		return result[i].Version < result[j].Version
	})
	return result
}

func cloneRunRevisionRef(ref *RunRevisionRef) *RunRevisionRef {
	if ref == nil {
		return nil
	}
	cloned := *ref
	return &cloned
}

func firstNonEmptyRunManifestValue(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
