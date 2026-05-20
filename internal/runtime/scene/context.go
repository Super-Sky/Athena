// context.go extracts scene-facing application context and guide questions from app/global payloads.
// context.go 负责从 app/global payload 中提取场景侧应用上下文和推荐问题。
package scene

import "strings"

// ApplicationDefinition captures the app-level persona and guide-question shape used by scene runtime.
// ApplicationDefinition 描述场景运行时使用的 App 级 persona 与推荐问题结构。
type ApplicationDefinition struct {
	ApplicationID  string   `json:"application_id,omitempty"`
	Title          string   `json:"title,omitempty"`
	Persona        string   `json:"persona,omitempty"`
	GuideQuestions []string `json:"guide_questions,omitempty"`
	Skills         []string `json:"skills,omitempty"`
}

// ApplicationInstance captures the app instance identity visible to scene runtime.
// ApplicationInstance 描述场景运行时可见的 App 实例标识。
type ApplicationInstance struct {
	AppInstanceID string `json:"app_instance_id,omitempty"`
	Title         string `json:"title,omitempty"`
}

// UserProfile captures the minimal user-facing profile fields used by scene runtime.
// UserProfile 描述场景运行时使用的最小用户画像字段。
type UserProfile struct {
	UserID   string `json:"user_id,omitempty"`
	Language string `json:"language,omitempty"`
}

// Context captures the first-phase app/global scene context consumed by scene runtime.
// Context 描述场景运行时消费的第一阶段 app/global 场景上下文。
type Context struct {
	ApplicationDefinition *ApplicationDefinition `json:"application_definition,omitempty"`
	ApplicationInstance   *ApplicationInstance   `json:"application_instance,omitempty"`
	UserProfile           *UserProfile           `json:"user_profile,omitempty"`
}

// BuildContext extracts the minimal scene context from app/global maps and explicit task fields.
// BuildContext 会从 app/global map 和显式任务字段中提取最小场景上下文。
func BuildContext(appContext map[string]any, globalContext map[string]any, appInstanceID, userLanguage string) *Context {
	ctx := &Context{}
	if def := buildApplicationDefinition(appContext); def != nil {
		ctx.ApplicationDefinition = def
	}
	if inst := buildApplicationInstance(appContext, appInstanceID); inst != nil {
		ctx.ApplicationInstance = inst
	}
	if profile := buildUserProfile(globalContext, userLanguage); profile != nil {
		ctx.UserProfile = profile
	}
	if ctx.ApplicationDefinition == nil && ctx.ApplicationInstance == nil && ctx.UserProfile == nil {
		return nil
	}
	return ctx
}

// ResolveGuideQuestions returns guide questions from the app context when available, otherwise falls back to defaults.
// ResolveGuideQuestions 优先返回 App 上下文中的推荐问题，否则回退到默认推荐问题。
func ResolveGuideQuestions(ctx *Context, fallback []string, userLanguage string) []string {
	if ctx != nil && ctx.ApplicationDefinition != nil && len(ctx.ApplicationDefinition.GuideQuestions) > 0 {
		return capGuideQuestions(ctx.ApplicationDefinition.GuideQuestions)
	}
	return capGuideQuestions(localizeQuestions(fallback, userLanguage))
}

func buildApplicationDefinition(appContext map[string]any) *ApplicationDefinition {
	if len(appContext) == 0 {
		return nil
	}
	def := &ApplicationDefinition{
		ApplicationID:  stringValue(appContext["application_id"]),
		Title:          firstNonEmptySceneValue(stringValue(appContext["title"]), stringValue(appContext["application_title"])),
		Persona:        stringValue(appContext["persona"]),
		GuideQuestions: stringSlice(appContext["guide_questions"]),
		Skills:         stringSlice(appContext["skills"]),
	}
	if def.ApplicationID == "" && def.Title == "" && def.Persona == "" && len(def.GuideQuestions) == 0 && len(def.Skills) == 0 {
		return nil
	}
	return def
}

func buildApplicationInstance(appContext map[string]any, appInstanceID string) *ApplicationInstance {
	appInstanceID = strings.TrimSpace(appInstanceID)
	title := stringValue(appContext["app_instance_title"])
	if appInstanceID == "" && title == "" {
		return nil
	}
	return &ApplicationInstance{
		AppInstanceID: appInstanceID,
		Title:         title,
	}
}

func buildUserProfile(globalContext map[string]any, userLanguage string) *UserProfile {
	userID := stringValue(globalContext["user_id"])
	userLanguage = firstNonEmptySceneValue(strings.TrimSpace(userLanguage), stringValue(globalContext["user_language"]), stringValue(globalContext["language"]))
	if userID == "" && userLanguage == "" {
		return nil
	}
	return &UserProfile{
		UserID:   userID,
		Language: userLanguage,
	}
}

func localizeQuestions(questions []string, userLanguage string) []string {
	if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(userLanguage)), "en") {
		return questions
	}
	result := make([]string, 0, len(questions))
	for _, item := range questions {
		switch strings.TrimSpace(item) {
		case "是否需要我总结高风险发现？":
			result = append(result, "Do you want me to summarize the high-risk findings?")
		case "是否需要我生成体检后续动作？":
			result = append(result, "Do you want me to generate the next inspection actions?")
		case "是否需要我展开关键步骤说明？":
			result = append(result, "Do you want me to expand the key workflow steps?")
		case "是否需要我标出需要确认的高风险步骤？":
			result = append(result, "Do you want me to mark the high-risk steps that require confirmation?")
		case "是否需要我解释告警原因？":
			result = append(result, "Do you want me to explain the alert reason?")
		case "是否需要我生成处置建议？":
			result = append(result, "Do you want me to generate remediation suggestions?")
		case "是否需要我给出自动化后续建议？":
			result = append(result, "Do you want me to provide automation follow-up suggestions?")
		case "是否需要我补充执行风险说明？":
			result = append(result, "Do you want me to add execution risk notes?")
		case "是否需要我解释这个集成事件的影响？":
			result = append(result, "Do you want me to explain the impact of this integration event?")
		case "是否需要我补充下一步动作？":
			result = append(result, "Do you want me to suggest the next action?")
		case "是否需要我继续展开关键结论？":
			result = append(result, "Do you want me to expand the key conclusion?")
		case "是否需要我给出下一步建议？":
			result = append(result, "Do you want me to suggest the next step?")
		default:
			result = append(result, item)
		}
	}
	return result
}

func capGuideQuestions(questions []string) []string {
	result := make([]string, 0, len(questions))
	for _, item := range questions {
		item = strings.TrimSpace(item)
		if item != "" {
			result = append(result, item)
		}
		if len(result) == 3 {
			break
		}
	}
	return result
}

func stringValue(value any) string {
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text)
	}
	return ""
}

func stringSlice(value any) []string {
	items, ok := value.([]any)
	if !ok {
		if values, ok := value.([]string); ok {
			return capGuideQuestions(values)
		}
		return nil
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		if text, ok := item.(string); ok {
			result = append(result, text)
		}
	}
	return capGuideQuestions(result)
}

func firstNonEmptySceneValue(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
