// types.go defines the stable intent-resolution context and result contract used by Athena routing.
// types.go 定义 Athena 路由使用的稳定意图解析上下文与结果契约。
package intent

// EntryMode captures the platform-provided entry hint for one interaction.
// EntryMode 描述一次交互中由 platform 提供的入口提示。
type EntryMode string

const (
	// EntryModeDefaultChat means the request entered through the normal main chat path.
	// EntryModeDefaultChat 表示请求通过普通主会话路径进入。
	EntryModeDefaultChat EntryMode = "default_chat"

	// EntryModeAutomationCreate means the request entered through the explicit automation-create entry.
	// EntryModeAutomationCreate 表示请求通过显式创建自动化入口进入。
	EntryModeAutomationCreate EntryMode = "automation_create"

	// EntryModeAutomationConfirm means the request entered through the explicit automation-confirm entry.
	// EntryModeAutomationConfirm 表示请求通过显式自动化确认入口进入。
	EntryModeAutomationConfirm EntryMode = "automation_confirm"

	// EntryModeResultExplanation means the request entered through a result-explanation route.
	// EntryModeResultExplanation 表示请求通过结果解释路径进入。
	EntryModeResultExplanation EntryMode = "result_explanation"
)

// UserSelectedMode captures the route the user selected after Athena returned multiple route options.
// UserSelectedMode 描述用户在 Athena 返回多个候选路由后手动选择的目标路径。
type UserSelectedMode string

const (
	// UserSelectedModeChat means the user chose to continue normal chat.
	// UserSelectedModeChat 表示用户选择继续普通聊天。
	UserSelectedModeChat UserSelectedMode = "chat"

	// UserSelectedModeAutomationDraft means the user chose to enter the automation draft path.
	// UserSelectedModeAutomationDraft 表示用户选择进入自动化草案路径。
	UserSelectedModeAutomationDraft UserSelectedMode = "automation_draft"
)

// InteractionMode captures Athena's final interaction routing mode for one request.
// InteractionMode 描述 Athena 对一次请求给出的最终交互路由模式。
type InteractionMode string

const (
	// InteractionModeChat means the request should remain in the normal chat path.
	// InteractionModeChat 表示请求应继续走普通聊天路径。
	InteractionModeChat InteractionMode = "chat"

	// InteractionModeAutomationDraft means the request should enter the automation draft path.
	// InteractionModeAutomationDraft 表示请求应进入自动化草案路径。
	InteractionModeAutomationDraft InteractionMode = "automation_draft"

	// InteractionModeClarification means Athena needs clarification before continuing.
	// InteractionModeClarification 表示 Athena 需要先反问澄清。
	InteractionModeClarification InteractionMode = "clarification"

	// InteractionModeResultExplanation means the request should explain an existing result.
	// InteractionModeResultExplanation 表示请求应进入结果解释路径。
	InteractionModeResultExplanation InteractionMode = "result_explanation"

	// InteractionModeChoiceRequired means Athena needs the user to choose one route.
	// InteractionModeChoiceRequired 表示 Athena 需要用户在多个路径之间做选择。
	InteractionModeChoiceRequired InteractionMode = "choice_required"
)

// Context captures the minimal signals needed to resolve one interaction route.
// Context 描述解析一次交互路由所需的最小信号集合。
type Context struct {
	Query             string
	TaskType          string
	DesiredOutputMode string
	AutomationTaskID  string
	EntryMode         EntryMode
	UserSelectedMode  UserSelectedMode
	Scene             string
	PrimarySkill      string
	AllowedTools      []string
}

// Resolution captures Athena's canonical route interpretation for one request.
// Resolution 描述 Athena 对一次请求给出的标准路由解释结果。
type Resolution struct {
	InteractionMode       InteractionMode
	Scene                 string
	PrimarySkill          string
	AllowedTools          []string
	SelectedRoute         string
	RequiresClarification bool
	Reason                string
}

// Option captures one user-selectable route when Athena cannot confidently choose one path.
// Option 描述当 Athena 无法高置信度选择单一路径时返回的一条用户可选路由项。
type Option struct {
	ID          string
	Title       string
	Description string
}
