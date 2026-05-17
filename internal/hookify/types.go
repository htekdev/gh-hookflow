package hookify

// Rule represents a hookify-format governance rule parsed from a markdown file
// with YAML frontmatter.
type Rule struct {
	Name          string      `yaml:"name"`
	Description   string      `yaml:"description,omitempty"`
	Enabled       *bool       `yaml:"enabled,omitempty"` // pointer to distinguish unset (default true) from explicit false
	Event         string      `yaml:"event"`
	Action        string      `yaml:"action,omitempty"` // block, warn, inject, modify, continue (default: warn)
	Pattern       string      `yaml:"pattern,omitempty"`
	Conditions    []Condition `yaml:"conditions,omitempty"`
	ToolMatcher   string      `yaml:"tool_matcher,omitempty"` // optional regex to match tool name
	Lifecycle     string      `yaml:"lifecycle,omitempty"`    // pre or post (default: pre)
	Message       string      `yaml:"-"`                      // markdown body (not from YAML)
	FilePath      string      `yaml:"-"`                      // source file path (not from YAML)
	HookEventType string      `yaml:"-"`                      // resolved Copilot CLI hook event type (set at runtime)
}

// IsEnabled returns whether the rule is enabled (default: true).
func (r *Rule) IsEnabled() bool {
	if r.Enabled == nil {
		return true
	}
	return *r.Enabled
}

// GetAction returns the rule's action, defaulting to "warn".
func (r *Rule) GetAction() string {
	if r.Action == "" {
		return ActionWarn
	}
	return r.Action
}

// GetLifecycle returns the rule's lifecycle, defaulting to "pre".
func (r *Rule) GetLifecycle() string {
	if r.Lifecycle == "" {
		return LifecyclePre
	}
	return r.Lifecycle
}

// Condition represents a single condition in a hookify rule.
// All conditions in a rule must match (AND logic).
type Condition struct {
	Field    string `yaml:"field"`
	Operator string `yaml:"operator"`
	Pattern  string `yaml:"pattern"`
}

// Event type constants — hookify aliases that map to Copilot CLI hook events
const (
	EventBash   = "bash"   // shell tools: powershell, bash, shell, terminal
	EventFile   = "file"   // file tools: create, edit
	EventAll    = "all"    // matches any event type
	EventStop   = "stop"   // agentStop / subagentStop events
	EventPrompt = "prompt" // userPromptSubmitted event

	// New Copilot CLI hook event types (1:1 mapping)
	EventSessionStart      = "sessionStart"
	EventSessionEnd        = "sessionEnd"
	EventPreToolUse        = "preToolUse"
	EventPostToolUse       = "postToolUse"
	EventPostToolUseFail   = "postToolUseFailure"
	EventAgentStop         = "agentStop"
	EventSubagentStart     = "subagentStart"
	EventSubagentStop      = "subagentStop"
	EventPermissionRequest = "permissionRequest"
	EventNotification      = "notification"
	EventPreCompact        = "preCompact"
	EventErrorOccurred     = "errorOccurred"
)

// Action constants
const (
	ActionBlock    = "block"
	ActionWarn     = "warn"
	ActionInject   = "inject"   // output additionalContext (for subagentStart, notification, sessionStart)
	ActionModify   = "modify"   // output modifiedArgs (for preToolUse arg rewriting)
	ActionContinue = "continue" // force agent to continue (for agentStop)
)

// Lifecycle constants
const (
	LifecyclePre  = "pre"
	LifecyclePost = "post"
)

// Operator constants
const (
	OpRegexMatch  = "regex_match"
	OpContains    = "contains"
	OpEquals      = "equals"
	OpNotContains = "not_contains"
	OpStartsWith  = "starts_with"
	OpEndsWith    = "ends_with"
)

// Field constants — the hookify fields that can be checked in conditions
const (
	FieldCommand     = "command"
	FieldFilePath    = "file_path"
	FieldNewText     = "new_text"
	FieldOldText     = "old_text"
	FieldContent     = "content"
	FieldTranscript  = "transcript"
	FieldToolName    = "tool_name"    // name of the tool being invoked
	FieldToolArgs    = "tool_args"    // JSON-encoded tool arguments
	FieldAgentName   = "agent_name"   // subagentStart/subagentStop agent name
	FieldAgentType   = "agent_type"   // subagentStart agent type
	FieldMessage     = "message"      // notification message, error message
	FieldHookEvent   = "hook_event"   // the Copilot CLI hook event type (e.g., "preToolUse")
	FieldSessionID   = "session_id"   // session identifier
	FieldToolResult  = "tool_result"  // postToolUse result content
)

// ValidEvents contains all recognized event types.
var ValidEvents = map[string]bool{
	EventBash:              true,
	EventFile:              true,
	EventAll:               true,
	EventStop:              true,
	EventPrompt:            true,
	EventSessionStart:      true,
	EventSessionEnd:        true,
	EventPreToolUse:        true,
	EventPostToolUse:       true,
	EventPostToolUseFail:   true,
	EventAgentStop:         true,
	EventSubagentStart:     true,
	EventSubagentStop:      true,
	EventPermissionRequest: true,
	EventNotification:      true,
	EventPreCompact:        true,
	EventErrorOccurred:     true,
}

// ValidActions contains all recognized action types.
var ValidActions = map[string]bool{
	ActionBlock:    true,
	ActionWarn:     true,
	ActionInject:   true,
	ActionModify:   true,
	ActionContinue: true,
}

// ValidOperators contains all recognized condition operators.
var ValidOperators = map[string]bool{
	OpRegexMatch:  true,
	OpContains:    true,
	OpEquals:      true,
	OpNotContains: true,
	OpStartsWith:  true,
	OpEndsWith:    true,
}

// ValidFields contains all recognized condition fields.
var ValidFields = map[string]bool{
	FieldCommand:    true,
	FieldFilePath:   true,
	FieldNewText:    true,
	FieldOldText:    true,
	FieldContent:    true,
	FieldTranscript: true,
	FieldToolName:   true,
	FieldToolArgs:   true,
	FieldAgentName:  true,
	FieldAgentType:  true,
	FieldMessage:    true,
	FieldHookEvent:  true,
	FieldSessionID:  true,
	FieldToolResult: true,
}

// contentPositionalOperators are operators that depend on string position and
// are therefore unsafe to use with the "content" field, whose value is built
// from map iteration (non-deterministic order).
var contentPositionalOperators = map[string]bool{
	OpEquals:     true,
	OpStartsWith: true,
	OpEndsWith:   true,
}

// BashToolNames are tool names that map to the "bash" hookify event type.
var BashToolNames = map[string]bool{
	"powershell": true,
	"bash":       true,
	"shell":      true,
	"terminal":   true,
}

// FileToolNames are tool names that map to the "file" hookify event type.
var FileToolNames = map[string]bool{
	"create": true,
	"edit":   true,
}

// NoConditionEvents are event types that can fire without pattern/conditions.
// These represent lifecycle or notification hooks where the event itself is the trigger.
var NoConditionEvents = map[string]bool{
	EventSessionStart:      true,
	EventSessionEnd:        true,
	EventAgentStop:         true,
	EventSubagentStart:     true,
	EventSubagentStop:      true,
	EventPermissionRequest: true,
	EventNotification:      true,
	EventPreCompact:        true,
	EventErrorOccurred:     true,
	EventStop:              true,
	EventPrompt:            true,
}

// StopAliasEvents are event types that the "stop" alias resolves to.
var StopAliasEvents = map[string]bool{
	EventAgentStop:    true,
	EventSubagentStop: true,
}
