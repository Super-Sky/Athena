// routing.go centralizes platform-specific automation routing and planning progress helpers.
// routing.go 负责收敛 platform 专项的自动化路由与规划进度辅助逻辑。
package automation

import (
	"strings"

	"moss/internal/runtime"
	intentpkg "moss/internal/runtime/intent"
)

// RouteInput captures the minimal request fields needed for platform-specific automation routing.
// RouteInput 描述 platform 专项自动化路由所需的最小请求字段。
type RouteInput struct {
	Query             string
	TaskType          string
	DesiredOutputMode string
	AutomationTaskID  string
	EntryMode         string
	UserSelectedMode  string
}

const explicitChoiceReason = "user explicitly asks to choose between continuing discussion and generating an automation draft"

// ShouldBuildPlanDraft reports whether the current request should enter the automation draft path.
// ShouldBuildPlanDraft 用于判断当前请求是否应进入自动化草案路径。
func ShouldBuildPlanDraft(input RouteInput) bool {
	if strings.EqualFold(strings.TrimSpace(input.UserSelectedMode), "automation_draft") {
		return true
	}
	if strings.EqualFold(strings.TrimSpace(input.EntryMode), "automation_create") {
		return true
	}
	if strings.EqualFold(strings.TrimSpace(input.DesiredOutputMode), "automation_plan_draft") {
		return true
	}
	return strings.TrimSpace(input.TaskType) == "scheduled_job" && strings.TrimSpace(input.AutomationTaskID) == ""
}

// EnhanceResolution applies platform-specific route-choice tightening without changing the generic runtime resolver contract.
// EnhanceResolution 会在不改动通用 runtime resolver 契约的前提下补充 platform 专项二选一路由增强。
func EnhanceResolution(input RouteInput, resolution intentpkg.Resolution) intentpkg.Resolution {
	if strings.TrimSpace(input.UserSelectedMode) != "" {
		return resolution
	}
	switch strings.TrimSpace(input.EntryMode) {
	case "automation_create", "automation_confirm", "result_explanation":
		return resolution
	}
	if !hasExplicitChoicePrompt(input.Query) {
		return resolution
	}
	resolution.InteractionMode = intentpkg.InteractionModeChoiceRequired
	resolution.SelectedRoute = "automation_draft"
	resolution.RequiresClarification = true
	resolution.Reason = explicitChoiceReason
	if strings.TrimSpace(resolution.PrimarySkill) == "" {
		resolution.PrimarySkill = "automation_planner"
	}
	return resolution
}

// BuildInteractionOptions returns stable platform-facing options for choice_required routes.
// BuildInteractionOptions 返回 choice_required 场景下稳定的 platform 选项结构。
func BuildInteractionOptions(mode intentpkg.InteractionMode) []runtime.InteractionOption {
	options := intentpkg.BuildOptions(intentpkg.Resolution{InteractionMode: mode})
	if len(options) == 0 {
		return nil
	}
	result := make([]runtime.InteractionOption, 0, len(options))
	for _, option := range options {
		result = append(result, runtime.InteractionOption{
			ID:          option.ID,
			Title:       option.Title,
			Description: option.Description,
		})
	}
	return result
}

// BuildInteractionProgress converts platform planning progress descriptors into one stable respond artifact.
// BuildInteractionProgress 会把 platform 规划进度描述转换成稳定的 respond 结果字段。
func BuildInteractionProgress(mode intentpkg.InteractionMode, enabled bool, maxPlanningSteps int) *runtime.InteractionProgress {
	if !enabled {
		return nil
	}
	steps := BuildPlanningProgressSteps(mode, "")
	if len(steps) == 0 {
		return nil
	}
	completed := make([]string, 0, len(steps))
	for _, step := range steps {
		completed = append(completed, step.StepID)
	}
	current := steps[len(steps)-1]
	if mode == intentpkg.InteractionModeAutomationDraft && len(completed) > 1 {
		completed = completed[:len(completed)-1]
		current.Summary = "Athena is building a draft plan that the user can review before any automation is created."
	}
	if mode == intentpkg.InteractionModeChoiceRequired && len(completed) > 1 {
		completed = completed[:len(completed)-1]
	}
	if maxPlanningSteps > 0 && len(completed) > maxPlanningSteps {
		completed = completed[:maxPlanningSteps]
	}
	return &runtime.InteractionProgress{
		CurrentStage:    current.StepID,
		CompletedStages: completed,
		Summary:         current.Summary,
	}
}

func hasExplicitChoicePrompt(query string) bool {
	normalized := strings.ToLower(strings.TrimSpace(query))
	if normalized == "" {
		return false
	}
	return containsAny(normalized,
		"还没决定",
		"先帮我判断",
		"还是直接",
		"还是先继续",
		"继续聊细节",
		"继续讨论还是",
		"帮我判断接下来应该继续",
		"进入待确认方案",
		"直接进入待确认方案",
		"继续讨论还是直接进入",
		"continue discussing or",
		"continue the discussion or",
		"decide whether",
		"or directly generate",
		"enter the draft plan",
		"enter a draft plan",
	)
}

func containsAny(input string, patterns ...string) bool {
	for _, pattern := range patterns {
		if strings.Contains(input, pattern) {
			return true
		}
	}
	return false
}
