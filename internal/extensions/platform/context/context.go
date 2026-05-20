// context.go parses platform-provided context summaries and turns them into deterministic usage traces and guidance.
// context.go 负责解析 platform 注入的上下文摘要，并把它们收敛成确定性的使用痕迹与 guidance。
//
// It exists because platform now injects multiple summary objects plus one catalog, and Athena needs one stable
// place to decide which summaries matter for the current request before runtime guidance and response enrichment.
// 这个文件存在的原因是 platform 现在会注入多个摘要对象和一份目录，Athena 需要一个稳定位置来决定
// 当前请求真正相关的上下文类型，再把结果接入 runtime guidance 和结果增强。
package context

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Type captures one canonical platform context type Athena can consume.
// Type 描述 Athena 可消费的一类标准 platform 上下文类型。
type Type string

const (
	// TypeIdentity captures stable user identity and role context.
	// TypeIdentity 表示稳定的用户身份与角色上下文。
	TypeIdentity Type = "identity"
	// TypeMemory captures long-term memory and stable preferences.
	// TypeMemory 表示长期记忆与稳定偏好。
	TypeMemory Type = "memory"
	// TypeKnowledge captures knowledge inventory and knowledge summaries.
	// TypeKnowledge 表示知识范围与知识摘要。
	TypeKnowledge Type = "knowledge"
	// TypeSkills captures available skills and capability summaries.
	// TypeSkills 表示可用技能与能力摘要。
	TypeSkills Type = "skills"
	// TypePersona captures expression-level persona preference context.
	// TypePersona 表示表达层 persona 偏好上下文。
	TypePersona Type = "persona"
)

var allTypes = []Type{TypeIdentity, TypeMemory, TypeKnowledge, TypeSkills, TypePersona}

// Bundle captures the normalized platform context bundle extracted from global context.
// Bundle 描述从全局上下文提取并归一化后的 platform 上下文集合。
type Bundle struct {
	Catalog map[Type]map[string]any
	Access  *Access

	IdentitySummary  any
	MemorySummary    any
	KnowledgeSummary any
	SkillsSummary    any
	PersonaSummary   any
	PersonaContext   map[string]any
	IdentityDetail   any
	MemoryDetail     any
	KnowledgeDetail  any
	SkillsDetail     any
	PersonaDetail    any
}

// Access captures the platform-issued context access token and its observable scope metadata.
// Access 描述 platform 签发的 context access token 及其可观测范围元数据。
type Access struct {
	Token        string   `json:"token,omitempty"`
	SubjectID    string   `json:"subject_id,omitempty"`
	TenantID     string   `json:"tenant_id,omitempty"`
	AllowedTypes []string `json:"allowed_types,omitempty"`
	SessionID    string   `json:"session_id,omitempty"`
	ExpiresAt    string   `json:"expires_at,omitempty"`
}

// UsageInput captures the minimal request signals needed to decide relevant context usage.
// UsageInput 描述决定当前相关上下文使用方式所需的最小请求信号。
type UsageInput struct {
	Query             string
	TaskType          string
	Scene             string
	DesiredOutputMode string
	InteractionMode   string
}

// UsageTrace captures which contexts Athena used and which details it wants platform to fetch next.
// UsageTrace 描述 Athena 实际使用了哪些上下文，以及希望 platform 后续补取哪些 detail。
type UsageTrace struct {
	UsedContexts            []string        `json:"used_contexts,omitempty"`
	ContextUsage            map[string]bool `json:"context_usage,omitempty"`
	ContextDetailsRequested []string        `json:"context_details_requested,omitempty"`
	GuidanceLines           []string        `json:"-"`
	RelevantContexts        []string        `json:"-"`
}

// BuildBundle extracts one normalized platform context bundle from global context.
// BuildBundle 负责从 global_context 中提取一份归一化的 platform 上下文集合。
func BuildBundle(globalContext map[string]any) *Bundle {
	if len(globalContext) == 0 {
		return nil
	}
	bundle := &Bundle{
		Catalog:          normalizeCatalog(globalContext["platform_context_catalog"]),
		Access:           normalizeAccess(globalContext["platform_context_access"]),
		IdentitySummary:  globalContext["identity_summary"],
		MemorySummary:    globalContext["memory_summary"],
		KnowledgeSummary: globalContext["knowledge_summary"],
		SkillsSummary:    globalContext["skills_summary"],
		PersonaSummary:   globalContext["persona_summary"],
		PersonaContext:   anyMap(globalContext["persona_context"]),
		IdentityDetail:   globalContext["identity_detail"],
		MemoryDetail:     globalContext["memory_detail"],
		KnowledgeDetail:  globalContext["knowledge_detail"],
		SkillsDetail:     globalContext["skills_detail"],
		PersonaDetail:    globalContext["persona_detail"],
	}
	if len(bundle.Catalog) == 0 &&
		bundle.IdentitySummary == nil &&
		bundle.MemorySummary == nil &&
		bundle.KnowledgeSummary == nil &&
		bundle.SkillsSummary == nil &&
		bundle.PersonaSummary == nil &&
		len(bundle.PersonaContext) == 0 &&
		bundle.Access == nil &&
		bundle.IdentityDetail == nil &&
		bundle.MemoryDetail == nil &&
		bundle.KnowledgeDetail == nil &&
		bundle.SkillsDetail == nil &&
		bundle.PersonaDetail == nil {
		return nil
	}
	return bundle
}

// ContextAccessToken returns the injected platform context access token when available.
// ContextAccessToken 返回已注入的 platform context access token。
func (b *Bundle) ContextAccessToken() string {
	if b == nil || b.Access == nil {
		return ""
	}
	return strings.TrimSpace(b.Access.Token)
}

// ResolveUsage returns deterministic usage traces and runtime guidance for the current request.
// ResolveUsage 返回当前请求的确定性上下文使用痕迹与 runtime guidance。
func (b *Bundle) ResolveUsage(input UsageInput) UsageTrace {
	trace := UsageTrace{
		ContextUsage: map[string]bool{},
	}
	if b == nil {
		for _, typ := range allTypes {
			trace.ContextUsage[string(typ)] = false
		}
		return trace
	}

	relevant := relevantTypes(input)
	trace.RelevantContexts = stringifyTypes(relevant)
	for _, typ := range allTypes {
		trace.ContextUsage[string(typ)] = false
	}

	for _, typ := range relevant {
		if b.hasSummary(typ) {
			trace.UsedContexts = append(trace.UsedContexts, string(typ))
			trace.ContextUsage[string(typ)] = true
			if line := b.guidanceLine(typ); line != "" {
				trace.GuidanceLines = append(trace.GuidanceLines, line)
			}
		}
	}
	trace.UsedContexts = compactStrings(trace.UsedContexts)
	trace.ContextDetailsRequested = stringifyTypes(b.requestedDetails(relevant, input))
	if len(trace.UsedContexts) > 0 {
		trace.GuidanceLines = append([]string{
			"Platform context catalog is available and Athena should prioritize the most relevant summaries before making conclusions.",
		}, trace.GuidanceLines...)
	}
	if len(trace.ContextDetailsRequested) > 0 {
		trace.GuidanceLines = append(trace.GuidanceLines,
			fmt.Sprintf("If the current summaries are insufficient, prefer requesting these platform context details instead of guessing: %s.", strings.Join(trace.ContextDetailsRequested, ", ")),
		)
	}
	return trace
}

// DomainHint returns one coarse domain hint extracted from the injected summaries.
// DomainHint 返回从已注入摘要中提取出的粗粒度领域提示。
func (b *Bundle) DomainHint() string {
	if b == nil {
		return ""
	}
	corpus := strings.Join([]string{
		RenderSummary(b.KnowledgeDetail),
		RenderSummary(b.KnowledgeSummary),
		RenderSummary(b.MemoryDetail),
		RenderSummary(b.MemorySummary),
		RenderSummary(b.IdentityDetail),
		RenderSummary(b.IdentitySummary),
		RenderSummary(b.SkillsDetail),
		RenderSummary(b.SkillsSummary),
	}, "\n")
	switch {
	case containsAny(corpus, "供应链", "supply chain"):
		return "supply_chain_security"
	case containsAny(corpus, "习惯", "habit"):
		return "habit_analysis"
	case containsAny(corpus, "画像", "profile"):
		return "profile_refresh"
	case containsAny(corpus, "运行情况", "runtime", "运行态"):
		return "runtime_daily_summary"
	case containsAny(corpus, "知识", "knowledge"):
		return "knowledge_refresh"
	default:
		return ""
	}
}

// CapabilityHints returns coarse capability hints distilled from skills summaries and catalog visibility.
// CapabilityHints 返回从 skills summary 与目录可见性中提炼出的粗粒度能力提示。
func (b *Bundle) CapabilityHints() []string {
	if b == nil {
		return nil
	}
	hints := []string{}
	text := strings.Join([]string{RenderSummary(b.SkillsDetail), RenderSummary(b.SkillsSummary)}, "\n")
	switch {
	case containsAny(text, "分析", "analysis"):
		hints = append(hints, "analysis")
	case containsAny(text, "自动化", "automation"):
		hints = append(hints, "automation")
	}
	if b.catalogContains(TypeKnowledge) {
		hints = append(hints, "knowledge_summary")
	}
	if b.catalogContains(TypeSkills) {
		hints = append(hints, "skills_summary")
	}
	return compactStrings(hints)
}

// HasDetail reports whether the bundle currently carries the requested detail payload.
// HasDetail 用于判断当前 bundle 是否已经携带某类 detail 数据。
func (b *Bundle) HasDetail(typ Type) bool {
	if b == nil {
		return false
	}
	switch typ {
	case TypeIdentity:
		return b.IdentityDetail != nil
	case TypeMemory:
		return b.MemoryDetail != nil
	case TypeKnowledge:
		return b.KnowledgeDetail != nil
	case TypeSkills:
		return b.SkillsDetail != nil
	case TypePersona:
		return b.PersonaDetail != nil
	default:
		return false
	}
}

func (b *Bundle) hasSummary(typ Type) bool {
	switch typ {
	case TypeIdentity:
		return strings.TrimSpace(RenderSummary(b.IdentityDetail)) != "" || strings.TrimSpace(RenderSummary(b.IdentitySummary)) != ""
	case TypeMemory:
		return strings.TrimSpace(RenderSummary(b.MemoryDetail)) != "" || strings.TrimSpace(RenderSummary(b.MemorySummary)) != ""
	case TypeKnowledge:
		return strings.TrimSpace(RenderSummary(b.KnowledgeDetail)) != "" || strings.TrimSpace(RenderSummary(b.KnowledgeSummary)) != ""
	case TypeSkills:
		return strings.TrimSpace(RenderSummary(b.SkillsDetail)) != "" || strings.TrimSpace(RenderSummary(b.SkillsSummary)) != ""
	case TypePersona:
		return strings.TrimSpace(RenderSummary(b.PersonaDetail)) != "" || strings.TrimSpace(RenderSummary(b.PersonaSummary)) != "" || len(b.PersonaContext) > 0
	default:
		return false
	}
}

func (b *Bundle) guidanceLine(typ Type) string {
	switch typ {
	case TypeIdentity:
		if text := RenderSummary(b.IdentityDetail); text != "" {
			return "Relevant platform identity detail: " + text
		}
		if text := RenderSummary(b.IdentitySummary); text != "" {
			return "Relevant platform identity summary: " + text
		}
	case TypeMemory:
		if text := RenderSummary(b.MemoryDetail); text != "" {
			return "Relevant platform memory detail: " + text
		}
		if text := RenderSummary(b.MemorySummary); text != "" {
			return "Relevant platform memory summary: " + text
		}
	case TypeKnowledge:
		if text := RenderSummary(b.KnowledgeDetail); text != "" {
			return "Relevant platform knowledge detail: " + text
		}
		if text := RenderSummary(b.KnowledgeSummary); text != "" {
			return "Relevant platform knowledge summary: " + text
		}
	case TypeSkills:
		if text := RenderSummary(b.SkillsDetail); text != "" {
			return "Relevant platform skill detail: " + text
		}
		if text := RenderSummary(b.SkillsSummary); text != "" {
			return "Relevant platform skills summary: " + text
		}
	case TypePersona:
		if text := RenderSummary(b.PersonaDetail); text != "" {
			return "Relevant platform persona detail: " + text
		}
		if text := RenderSummary(b.PersonaSummary); text != "" {
			return "Relevant platform persona summary: " + text
		}
	}
	return ""
}

func (b *Bundle) requestedDetails(relevant []Type, input UsageInput) []Type {
	requested := []Type{}
	for _, typ := range relevant {
		if !b.catalogContains(typ) {
			continue
		}
		if !b.hasSummary(typ) {
			requested = append(requested, typ)
			continue
		}
		summary := RenderSummary(b.summaryValue(typ))
		if explicitlyAsksForDetail(input.Query, typ) && summaryTooThin(summary) {
			requested = append(requested, typ)
			continue
		}
		if isAutomationRequest(input) && (typ == TypeKnowledge || typ == TypeSkills) && summaryTooThin(summary) {
			requested = append(requested, typ)
			continue
		}
		if isAnalysisRequest(input) && typ == TypeKnowledge && summaryTooThin(summary) {
			requested = append(requested, typ)
		}
	}
	return uniqueTypes(requested)
}

func (b *Bundle) summaryValue(typ Type) any {
	switch typ {
	case TypeIdentity:
		if b.IdentityDetail != nil {
			return b.IdentityDetail
		}
		return b.IdentitySummary
	case TypeMemory:
		if b.MemoryDetail != nil {
			return b.MemoryDetail
		}
		return b.MemorySummary
	case TypeKnowledge:
		if b.KnowledgeDetail != nil {
			return b.KnowledgeDetail
		}
		return b.KnowledgeSummary
	case TypeSkills:
		if b.SkillsDetail != nil {
			return b.SkillsDetail
		}
		return b.SkillsSummary
	case TypePersona:
		if b.PersonaDetail != nil {
			return b.PersonaDetail
		}
		if b.PersonaSummary != nil {
			return b.PersonaSummary
		}
		return b.PersonaContext
	default:
		return nil
	}
}

func (b *Bundle) catalogContains(typ Type) bool {
	if b == nil {
		return false
	}
	if len(b.Catalog) == 0 {
		return b.hasSummary(typ)
	}
	_, ok := b.Catalog[typ]
	return ok
}

func relevantTypes(input UsageInput) []Type {
	set := []Type{}
	add := func(typ Type) {
		for _, existing := range set {
			if existing == typ {
				return
			}
		}
		set = append(set, typ)
	}

	if isAutomationRequest(input) {
		add(TypeIdentity)
		add(TypeMemory)
		add(TypeKnowledge)
		add(TypeSkills)
		add(TypePersona)
	} else if isAnalysisRequest(input) {
		add(TypeKnowledge)
		add(TypeMemory)
		add(TypeIdentity)
		add(TypePersona)
	} else {
		add(TypeIdentity)
		add(TypeMemory)
		add(TypePersona)
	}

	query := strings.ToLower(strings.TrimSpace(input.Query))
	if containsAny(query, "知识", "knowledge", "摘要", "summary") {
		add(TypeKnowledge)
	}
	if containsAny(query, "技能", "能力", "skill", "tool", "工具") {
		add(TypeSkills)
	}
	if containsAny(query, "身份", "背景", "角色", "identity", "profile") {
		add(TypeIdentity)
	}
	if containsAny(query, "记忆", "偏好", "memory", "preference", "长期") {
		add(TypeMemory)
	}
	return set
}

func isAutomationRequest(input UsageInput) bool {
	switch strings.TrimSpace(input.InteractionMode) {
	case "automation_draft":
		return true
	}
	if strings.EqualFold(strings.TrimSpace(input.DesiredOutputMode), "automation_plan_draft") {
		return true
	}
	switch strings.TrimSpace(input.TaskType) {
	case "scheduled_job", "workflow_step_request":
		return true
	}
	query := strings.ToLower(strings.TrimSpace(input.Query))
	return containsAny(query, "自动化", "automation", "周期", "recurring", "schedule", "daily", "weekly", "monthly", "每日", "每周", "每月")
}

func isAnalysisRequest(input UsageInput) bool {
	if strings.TrimSpace(input.Scene) == "security_review" {
		return true
	}
	query := strings.ToLower(strings.TrimSpace(input.Query))
	return containsAny(query, "分析", "研判", "总结", "关注点", "analysis", "review", "summary", "risk", "安全")
}

func explicitlyAsksForDetail(query string, typ Type) bool {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return false
	}
	detailWords := containsAny(query, "详细", "详情", "具体", "条目", "明细", "细节", "detail", "details", "specific", "full")
	if !detailWords {
		return false
	}
	switch typ {
	case TypeIdentity:
		return containsAny(query, "身份", "背景", "角色", "identity")
	case TypeMemory:
		return containsAny(query, "记忆", "偏好", "memory", "preference")
	case TypeKnowledge:
		return containsAny(query, "知识", "知识库", "knowledge", "关注点", "条目")
	case TypeSkills:
		return containsAny(query, "技能", "能力", "skill", "tool", "工具")
	case TypePersona:
		return containsAny(query, "persona", "风格", "表达", "语气")
	default:
		return false
	}
}

func summaryTooThin(summary string) bool {
	summary = strings.TrimSpace(summary)
	if summary == "" {
		return true
	}
	return len([]rune(summary)) < 36
}

// RenderSummary returns one stable short text projection from a summary object or scalar value.
// RenderSummary 返回一段从摘要对象或标量值中提炼出的稳定短文本。
func RenderSummary(value any) string {
	switch raw := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(raw)
	case []string:
		return strings.Join(compactStrings(raw), "; ")
	case []any:
		items := make([]string, 0, len(raw))
		for _, item := range raw {
			if text := RenderSummary(item); text != "" {
				items = append(items, text)
			}
			if len(items) == 4 {
				break
			}
		}
		return strings.Join(items, "; ")
	case map[string]any:
		for _, key := range []string{"summary", "description", "overview", "text", "content", "title"} {
			if text := RenderSummary(raw[key]); text != "" {
				return text
			}
		}
		keys := make([]string, 0, len(raw))
		for key := range raw {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, key := range keys {
			if text := RenderSummary(raw[key]); text != "" {
				parts = append(parts, fmt.Sprintf("%s=%s", key, text))
			}
			if len(parts) == 4 {
				break
			}
		}
		return strings.Join(parts, "; ")
	default:
		encoded, err := json.Marshal(raw)
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(encoded))
	}
}

func normalizeCatalog(value any) map[Type]map[string]any {
	raw, ok := value.(map[string]any)
	if !ok || len(raw) == 0 {
		return nil
	}
	result := map[Type]map[string]any{}
	for _, typ := range allTypes {
		if entry, ok := raw[string(typ)]; ok {
			result[typ] = anyMap(entry)
		}
	}
	return result
}

func normalizeAccess(value any) *Access {
	raw, ok := value.(map[string]any)
	if !ok || len(raw) == 0 {
		return nil
	}
	access := &Access{
		Token:        strings.TrimSpace(RenderSummary(raw["token"])),
		SubjectID:    strings.TrimSpace(RenderSummary(raw["subject_id"])),
		TenantID:     strings.TrimSpace(RenderSummary(raw["tenant_id"])),
		SessionID:    strings.TrimSpace(RenderSummary(raw["session_id"])),
		ExpiresAt:    strings.TrimSpace(RenderSummary(raw["expires_at"])),
		AllowedTypes: nil,
	}
	switch values := raw["allowed_types"].(type) {
	case []string:
		access.AllowedTypes = compactStrings(values)
	case []any:
		items := make([]string, 0, len(values))
		for _, item := range values {
			if text := strings.TrimSpace(RenderSummary(item)); text != "" {
				items = append(items, text)
			}
		}
		access.AllowedTypes = compactStrings(items)
	}
	if access.Token == "" && access.SubjectID == "" && access.TenantID == "" && access.SessionID == "" && access.ExpiresAt == "" && len(access.AllowedTypes) == 0 {
		return nil
	}
	return access
}

func detailKey(typ Type) string {
	return string(typ) + "_detail"
}

func anyMap(value any) map[string]any {
	switch mapped := value.(type) {
	case map[string]any:
		return mapped
	default:
		return nil
	}
}

func containsAny(input string, terms ...string) bool {
	for _, term := range terms {
		if strings.Contains(input, strings.ToLower(strings.TrimSpace(term))) {
			return true
		}
	}
	return false
}

func compactStrings(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			result = append(result, value)
		}
	}
	return result
}

func uniqueTypes(values []Type) []Type {
	result := make([]Type, 0, len(values))
	seen := map[Type]struct{}{}
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func stringifyTypes(values []Type) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, string(value))
	}
	return compactStrings(result)
}
