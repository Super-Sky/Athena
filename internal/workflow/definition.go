// definition.go defines truth-backed workflow definitions and plan conversion.
// definition.go 定义由 truth source 驱动的 workflow definition 及其计划转换逻辑。
package workflow

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"moss/internal/systemtruth"
	"gopkg.in/yaml.v3"
)

// EntryDefinition captures one workflow entry policy.
// EntryDefinition 描述一条 workflow 入口策略。
type EntryDefinition struct {
	AllowWaiting        bool     `yaml:"allow_waiting" json:"allow_waiting,omitempty"`
	AllowResume         bool     `yaml:"allow_resume" json:"allow_resume,omitempty"`
	RequiredContracts   []string `yaml:"required_contracts" json:"required_contracts,omitempty"`
	RequiredPolicyRules []string `yaml:"required_policy_rules" json:"required_policy_rules,omitempty"`
}

// ChecksDefinition captures stage-level checks.
// ChecksDefinition 描述阶段级检查项。
type ChecksDefinition struct {
	PolicyRules []string `yaml:"policy_rules" json:"policy_rules,omitempty"`
}

// StageDefinition captures one stage in workflow.yaml.
// StageDefinition 描述 workflow.yaml 中的一条阶段定义。
type StageDefinition struct {
	ID           string           `yaml:"id" json:"id,omitempty"`
	Title        string           `yaml:"title" json:"title,omitempty"`
	Mode         string           `yaml:"mode" json:"mode,omitempty"`
	Purpose      string           `yaml:"purpose" json:"purpose,omitempty"`
	UsesSkills   []string         `yaml:"uses_skills" json:"uses_skills,omitempty"`
	UsesContracts []string        `yaml:"uses_contracts" json:"uses_contracts,omitempty"`
	Checks       ChecksDefinition `yaml:"checks" json:"checks,omitempty"`
	EntryIf      []string         `yaml:"entry_if" json:"entry_if,omitempty"`
	CompleteWhen []string         `yaml:"complete_when" json:"complete_when,omitempty"`
	BlockWhen    []string         `yaml:"block_when" json:"block_when,omitempty"`
	Next         map[string]string `yaml:"next" json:"next,omitempty"`
}

// Definition captures one truth-backed workflow definition.
// Definition 描述一份由 truth source 驱动的 workflow 定义。
type Definition struct {
	SceneID string          `json:"scene_id,omitempty"`
	ID      string          `yaml:"id" json:"id,omitempty"`
	Name    string          `yaml:"name" json:"name,omitempty"`
	Summary string          `yaml:"summary" json:"summary,omitempty"`
	Entry   EntryDefinition `yaml:"entry" json:"entry,omitempty"`
	Stages  []StageDefinition `yaml:"stages" json:"stages,omitempty"`
}

var workflowSourcesRoot string

// SetSourcesRoot overrides the workflow sources root used by GenerateDefaultPlan.
// SetSourcesRoot 用于覆盖 GenerateDefaultPlan 使用的 workflow 主源目录。
func SetSourcesRoot(path string) {
	workflowSourcesRoot = strings.TrimSpace(path)
}

// BuiltinDefinitions returns all truth-backed workflow definitions.
// BuiltinDefinitions 返回全部由 truth source 驱动的 workflow definitions。
func BuiltinDefinitions() []Definition {
	root := strings.TrimSpace(workflowSourcesRoot)
	if root == "" {
		root = systemtruth.SourcesRoot(systemtruth.DefaultTruthDir())
	}
	items, err := loadDefinitions(root)
	if err != nil {
		return nil
	}
	return items
}

// DefinitionForScene returns one workflow definition by scene id.
// DefinitionForScene 会按 scene id 返回对应的 workflow definition。
func DefinitionForScene(sceneID string) (Definition, bool) {
	for _, item := range BuiltinDefinitions() {
		if strings.TrimSpace(item.SceneID) == strings.TrimSpace(sceneID) {
			return item, true
		}
	}
	return Definition{}, false
}

func loadDefinitions(sourcesRoot string) ([]Definition, error) {
	sceneRoot := filepath.Join(strings.TrimSpace(sourcesRoot), "scenes")
	sceneEntries, err := os.ReadDir(sceneRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	result := make([]Definition, 0, len(sceneEntries))
	for _, entry := range sceneEntries {
		if !entry.IsDir() {
			continue
		}
		sceneID := systemtruth.NormalizeID(entry.Name())
		yamlPath := filepath.Join(sceneRoot, entry.Name(), "workflow.yaml")
		payload, err := systemtruth.ReadYAMLMap(yamlPath)
		if err != nil {
			continue
		}
		def, err := decodeDefinition(sceneID, payload)
		if err != nil {
			continue
		}
		result = append(result, def)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].SceneID < result[j].SceneID })
	return result, nil
}

func decodeDefinition(sceneID string, payload map[string]any) (Definition, error) {
	body, err := yaml.Marshal(payload)
	if err != nil {
		return Definition{}, err
	}
	var def Definition
	if err := yaml.Unmarshal(body, &def); err != nil {
		return Definition{}, err
	}
	def.SceneID = strings.TrimSpace(sceneID)
	if strings.TrimSpace(def.ID) == "" {
		def.ID = sceneID + "_main"
	}
	return def, nil
}

func buildPlanFromDefinition(input GenerateInput, def Definition) *Plan {
	if len(def.Stages) == 0 {
		return nil
	}
	plan := &Plan{
		PlanID:                      "plan-" + strings.TrimSpace(input.TaskID),
		TaskID:                      strings.TrimSpace(input.TaskID),
		Goal:                        defaultGoal(input),
		Title:                       firstNonEmpty(def.Name, strings.TrimSpace(input.Scene)),
		Summary:                     firstNonEmpty(def.Summary, "由 truth workflow 定义生成的结构化计划"),
		RiskLevel:                   defaultRiskLevel(input),
		RequiresConfirmation:        false,
		SuggestedEntryAppInstanceID: strings.TrimSpace(input.SuggestedEntryAppID),
	}
	steps := make([]Step, 0, len(def.Stages))
	for index, stage := range def.Stages {
		step := Step{
			StepID:               strings.TrimSpace(stage.ID),
			Order:                index + 1,
			Title:                firstNonEmpty(stage.Title, stage.ID),
			Description:          strings.TrimSpace(stage.Purpose),
			RequiredInputs:       append([]string(nil), stage.EntryIf...),
			ParallelGroup:        defaultParallelGroup(stage.Mode),
			ConfirmationRequired: strings.EqualFold(stage.Mode, "manual"),
			ExecutionMode:        stepExecutionMode(stage.Mode),
			CompletionCondition:  strings.Join(stage.CompleteWhen, "; "),
			FailureGuidance:      strings.Join(stage.BlockWhen, "; "),
			StepType:             stepType(stage.Mode),
		}
		if step.ExecutionMode == StepExecutionModeManualAction {
			plan.RequiresConfirmation = true
		}
		steps = append(steps, step)
	}
	for i := range steps {
		if i == 0 {
			continue
		}
		steps[i].DependsOn = systemtruth.CompactStrings(append(steps[i].DependsOn, steps[i-1].StepID))
	}
	plan.Steps = steps
	return plan
}

func defaultParallelGroup(mode string) string {
	switch strings.TrimSpace(strings.ToLower(mode)) {
	case "manual":
		return "decision"
	case "hybrid":
		return "execution"
	default:
		return "analysis"
	}
}

func stepExecutionMode(mode string) StepExecutionMode {
	switch strings.TrimSpace(strings.ToLower(mode)) {
	case "manual":
		return StepExecutionModeManualAction
	case "hybrid":
		return StepExecutionModeAutomationCandidate
	default:
		return StepExecutionModeReadonlyAnalysis
	}
}

func stepType(mode string) StepType {
	switch strings.TrimSpace(strings.ToLower(mode)) {
	case "manual":
		return StepTypeAction
	case "hybrid":
		return StepTypeInvestigation
	default:
		return StepTypeAnalysis
	}
}

func firstNonEmpty(values ...string) string {
	for _, item := range values {
		item = strings.TrimSpace(item)
		if item != "" {
			return item
		}
	}
	return ""
}
