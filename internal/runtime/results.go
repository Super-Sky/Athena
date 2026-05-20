// results.go defines the canonical complete-result envelope and its reusable sub-objects.
// results.go 定义标准完整结果包及其可复用子结构。
package runtime

// ResultSummary captures the compact summary block that platforms can render without reparsing the full result.
// ResultSummary 描述平台可直接渲染的紧凑摘要块，而无需重新解析完整结果。
type ResultSummary struct {
	Title    string         `json:"title,omitempty"`
	Verdict  string         `json:"verdict,omitempty"`
	Severity string         `json:"severity,omitempty"`
	Summary  string         `json:"summary,omitempty"`
	Detail   map[string]any `json:"detail,omitempty"`
}

// ContentCard captures one card-style result item that the platform can render directly.
// ContentCard 描述一条平台可直接渲染的卡片式结果项。
type ContentCard struct {
	CardID   string         `json:"card_id,omitempty"`
	CardType string         `json:"card_type,omitempty"`
	Title    string         `json:"title,omitempty"`
	Summary  string         `json:"summary,omitempty"`
	Source   string         `json:"source,omitempty"`
	Payload  map[string]any `json:"payload,omitempty"`
}

// RightPanelSection captures one section rendered inside the platform right panel.
// RightPanelSection 描述平台右栏中渲染的一段 section。
type RightPanelSection struct {
	SectionID string         `json:"section_id,omitempty"`
	Title     string         `json:"title,omitempty"`
	Body      string         `json:"body,omitempty"`
	Payload   map[string]any `json:"payload,omitempty"`
}

// RightPanelView captures the structured right-panel projection for one runtime result.
// RightPanelView 描述单次 runtime 结果的结构化右栏投影视图。
type RightPanelView struct {
	ViewID   string              `json:"view_id,omitempty"`
	ViewType string              `json:"view_type,omitempty"`
	View     string              `json:"view,omitempty"`
	Title    string              `json:"title,omitempty"`
	Content  string              `json:"content,omitempty"`
	Summary  string              `json:"summary,omitempty"`
	Sections []RightPanelSection `json:"sections,omitempty"`
	Payload  map[string]any      `json:"payload,omitempty"`
}

// ScoreDelta captures one platform-consumable growth or score change signal.
// ScoreDelta 描述一条平台可消费的成长或分数变化信号。
type ScoreDelta struct {
	Dimension     string   `json:"dimension,omitempty"`
	Delta         int      `json:"delta,omitempty"`
	ReasonSummary string   `json:"reason_summary,omitempty"`
	EvidenceRefs  []string `json:"evidence_refs,omitempty"`
}

// ResultDeliveryProfile captures the stable richer-result contract selected for one task.
// ResultDeliveryProfile 描述单个任务当前选择的稳定增强结果交付契约。
type ResultDeliveryProfile struct {
	TaskType          string   `json:"task_type,omitempty"`
	Scene             string   `json:"scene,omitempty"`
	DesiredOutputMode string   `json:"desired_output_mode,omitempty"`
	StableFields      []string `json:"stable_fields,omitempty"`
	OptionalFields    []string `json:"optional_fields,omitempty"`
}

// InteractionModeResult captures the canonical interaction routing mode Athena wants platform to consume.
// InteractionModeResult 描述 Athena 希望 platform 消费的标准交互路由模式。
type InteractionModeResult struct {
	Mode   string `json:"mode,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// InteractionOption captures one user-selectable route option when Athena cannot confidently choose a single path.
// InteractionOption 描述当 Athena 无法高置信度选择单一路径时返回的一条用户可选路由项。
type InteractionOption struct {
	ID          string `json:"id,omitempty"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
}

// InteractionProgress captures one non-stream interaction-stage summary for platform pages that do not consume SSE.
// InteractionProgress 描述供不消费 SSE 的 platform 页面使用的非流式交互阶段摘要。
type InteractionProgress struct {
	CurrentStage    string   `json:"current_stage,omitempty"`
	CompletedStages []string `json:"completed_stages,omitempty"`
	Summary         string   `json:"summary,omitempty"`
}

// IntentResolutionResult captures the deeper route interpretation Athena produced for one request.
// IntentResolutionResult 描述 Athena 对一次请求生成的更深层路由解释结果。
type IntentResolutionResult struct {
	InteractionMode       string   `json:"interaction_mode,omitempty"`
	Scene                 string   `json:"scene,omitempty"`
	PrimarySkill          string   `json:"primary_skill,omitempty"`
	AllowedTools          []string `json:"allowed_tools,omitempty"`
	SelectedRoute         string   `json:"selected_route,omitempty"`
	RequiresClarification bool     `json:"requires_clarification,omitempty"`
	Reason                string   `json:"reason,omitempty"`
}

// CompleteResult captures the full platform-facing result bundle for one Athena task.
// CompleteResult 描述一次 Athena 任务面向平台的完整结果包。
type CompleteResult struct {
	MainAnswer       string                 `json:"main_answer,omitempty"`
	StructuredResult map[string]any         `json:"structured_result,omitempty"`
	ResultSummary    *ResultSummary         `json:"result_summary,omitempty"`
	ContentCards     []ContentCard          `json:"content_cards,omitempty"`
	RightPanelView   *RightPanelView        `json:"right_panel_view,omitempty"`
	NextQuestions    []string               `json:"next_questions,omitempty"`
	ScoreDelta       *ScoreDelta            `json:"score_delta,omitempty"`
	DeliveryProfile  *ResultDeliveryProfile `json:"delivery_profile,omitempty"`
}
