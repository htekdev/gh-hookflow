package hookify

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/htekdev/gh-hookflow/internal/schema"
)

// Evaluate evaluates a hookify rule's conditions against the event data.
// Returns a WorkflowResult if the rule fires (all conditions match),
// or nil if the rule doesn't fire (conditions not met).
// sessionDir is the path to the session directory (for transcript access).
func Evaluate(rule *Rule, event *schema.Event, sessionDir string) *schema.WorkflowResult {
	for _, cond := range rule.Conditions {
		fieldValue := extractField(cond.Field, event, sessionDir)
		if !evaluateCondition(&cond, fieldValue) {
			return nil // AND logic: any condition failing means rule doesn't fire
		}
	}

	// All conditions matched — produce result based on action
	action := rule.GetAction()
	reason := rule.Message
	if reason == "" {
		reason = fmt.Sprintf("Hookify rule %q triggered", rule.Name)
	}

	switch action {
	case ActionBlock:
		return &schema.WorkflowResult{
			PermissionDecision:       "deny",
			PermissionDecisionReason: reason,
		}
	case ActionWarn:
		return &schema.WorkflowResult{
			PermissionDecision:       "allow",
			PermissionDecisionReason: reason,
		}
	case ActionInject:
		// Inject additionalContext into the hook response.
		// The markdown body IS the content to inject as context.
		// Used for subagentStart, notification, sessionStart to provide
		// governance instructions or context to the agent.
		return &schema.WorkflowResult{
			PermissionDecision:       "allow",
			PermissionDecisionReason: fmt.Sprintf("Hookify rule %q injected context", rule.Name),
			AdditionalContext:        reason,
		}
	case ActionModify:
		// Modify tool args. The message body contains the modification content.
		// ModifyTarget specifies which argument to modify.
		// ModifyStrategy specifies how: prepend, append, replace, regex.
		modifiedArgs := applyModify(rule, event)
		return &schema.WorkflowResult{
			PermissionDecision:       "allow",
			PermissionDecisionReason: fmt.Sprintf("Hookify rule %q modified arg %q", rule.Name, rule.ModifyTarget),
			ModifiedArgs:             modifiedArgs,
		}
	case ActionContinue:
		// Force the agent to continue instead of stopping.
		// The markdown body IS the prompt to respond with when continuing.
		// Used for agentStop to override the stop decision.
		return &schema.WorkflowResult{
			PermissionDecision:       "deny",
			PermissionDecisionReason: fmt.Sprintf("Hookify rule %q forced continuation", rule.Name),
			ContinueAgent:            true,
			ContinuePrompt:           reason,
		}
	default:
		// Default to warn
		return &schema.WorkflowResult{
			PermissionDecision:       "allow",
			PermissionDecisionReason: reason,
		}
	}
}

// extractField extracts the value of a hookify field from the event data.
func extractField(field string, event *schema.Event, sessionDir string) string {
	switch field {
	case FieldCommand:
		return extractCommand(event)
	case FieldFilePath:
		return extractFilePath(event)
	case FieldNewText:
		return extractNewText(event)
	case FieldOldText:
		return extractOldText(event)
	case FieldContent:
		return extractContent(event)
	case FieldTranscript:
		return readTranscriptContent(sessionDir)
	case FieldToolName:
		return extractToolName(event)
	case FieldToolArgs:
		return extractToolArgsJSON(event)
	case FieldAgentName:
		return extractAgentName(event)
	case FieldAgentType:
		return extractAgentType(event)
	case FieldMessage:
		return extractMessage(event)
	case FieldHookEvent:
		return extractHookEventType(event)
	case FieldSessionID:
		return extractSessionID(event)
	case FieldToolResult:
		return extractToolResult(event)
	default:
		return ""
	}
}

// extractCommand gets the command/script/code from tool args (for bash events).
func extractCommand(event *schema.Event) string {
	if event.Tool == nil || event.Tool.Args == nil {
		return ""
	}
	// Check command, script, code in order of priority
	for _, key := range []string{"command", "script", "code"} {
		if v, ok := event.Tool.Args[key]; ok {
			if s, ok := v.(string); ok {
				return s
			}
		}
	}
	return ""
}

// extractFilePath gets the file path from the event.
func extractFilePath(event *schema.Event) string {
	if event.File != nil && event.File.Path != "" {
		return event.File.Path
	}
	if event.Tool != nil && event.Tool.Args != nil {
		if v, ok := event.Tool.Args["path"]; ok {
			if s, ok := v.(string); ok {
				return s
			}
		}
	}
	return ""
}

// extractNewText gets the new text content from the event.
func extractNewText(event *schema.Event) string {
	if event.File != nil && event.File.Content != "" {
		return event.File.Content
	}
	if event.Tool == nil || event.Tool.Args == nil {
		return ""
	}
	for _, key := range []string{"new_str", "file_text"} {
		if v, ok := event.Tool.Args[key]; ok {
			if s, ok := v.(string); ok {
				return s
			}
		}
	}
	return ""
}

// extractOldText gets the old text content from the event (edit only).
func extractOldText(event *schema.Event) string {
	if event.Tool == nil || event.Tool.Args == nil {
		return ""
	}
	if v, ok := event.Tool.Args["old_str"]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// extractContent concatenates all string values from tool args.
func extractContent(event *schema.Event) string {
	if event.Tool == nil || event.Tool.Args == nil {
		return ""
	}
	var parts []string
	for _, v := range event.Tool.Args {
		if s, ok := v.(string); ok && s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, "\n")
}

// readTranscriptContent reads the session transcript JSONL file contents.
func readTranscriptContent(sessionDir string) string {
	if sessionDir == "" {
		return ""
	}
	tp := filepath.Join(sessionDir, "transcript.jsonl")
	data, err := os.ReadFile(tp)
	if err != nil {
		return ""
	}
	return string(data)
}

// extractToolName returns the tool name from the event.
func extractToolName(event *schema.Event) string {
	if event.Tool != nil {
		return event.Tool.Name
	}
	if event.Hook != nil && event.Hook.Tool != nil {
		return event.Hook.Tool.Name
	}
	return ""
}

// extractToolArgsJSON returns all tool args as a JSON string for regex matching.
func extractToolArgsJSON(event *schema.Event) string {
	args := getToolArgs(event)
	if args == nil || len(args) == 0 {
		return ""
	}
	data, err := json.Marshal(args)
	if err != nil {
		return ""
	}
	return string(data)
}

// extractAgentName extracts the agent name from subagentStart/subagentStop events.
func extractAgentName(event *schema.Event) string {
	args := getToolArgs(event)
	if args == nil {
		return ""
	}
	// subagentStart sends agentName in the hook payload
	if v, ok := args["agentName"]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	if v, ok := args["agent_name"]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// extractAgentType extracts the agent type from subagentStart events.
func extractAgentType(event *schema.Event) string {
	args := getToolArgs(event)
	if args == nil {
		return ""
	}
	if v, ok := args["agentType"]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	if v, ok := args["agent_type"]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// extractMessage extracts the message from notification or error events.
func extractMessage(event *schema.Event) string {
	args := getToolArgs(event)
	if args == nil {
		return ""
	}
	for _, key := range []string{"message", "error", "errorMessage", "description"} {
		if v, ok := args[key]; ok {
			if s, ok := v.(string); ok {
				return s
			}
		}
	}
	return ""
}

// extractHookEventType returns the Copilot CLI hook event type (e.g., "preToolUse").
func extractHookEventType(event *schema.Event) string {
	if event.Hook != nil {
		return event.Hook.Type
	}
	return ""
}

// extractSessionID returns the session identifier from the event.
func extractSessionID(event *schema.Event) string {
	if event.SessionID != "" {
		return event.SessionID
	}
	return ""
}

// extractToolResult extracts the tool result content for postToolUse events.
func extractToolResult(event *schema.Event) string {
	args := getToolArgs(event)
	if args == nil {
		return ""
	}
	if v, ok := args["toolResult"]; ok {
		if s, ok := v.(string); ok {
			return s
		}
		// toolResult might be a nested object — marshal to JSON
		data, err := json.Marshal(v)
		if err == nil {
			return string(data)
		}
	}
	return ""
}

// getToolArgs returns tool args, checking both Tool and Hook.Tool.
func getToolArgs(event *schema.Event) map[string]interface{} {
	if event.Tool != nil && event.Tool.Args != nil {
		return event.Tool.Args
	}
	if event.Hook != nil && event.Hook.Tool != nil && event.Hook.Tool.Args != nil {
		return event.Hook.Tool.Args
	}
	return nil
}

// evaluateCondition applies the operator to check if fieldValue matches the pattern.
func evaluateCondition(cond *Condition, fieldValue string) bool {
	switch cond.Operator {
	case OpRegexMatch:
		return regexMatch(cond.Pattern, fieldValue)
	case OpContains:
		return strings.Contains(strings.ToLower(fieldValue), strings.ToLower(cond.Pattern))
	case OpEquals:
		return fieldValue == cond.Pattern
	case OpNotContains:
		return !strings.Contains(strings.ToLower(fieldValue), strings.ToLower(cond.Pattern))
	case OpStartsWith:
		return strings.HasPrefix(strings.ToLower(fieldValue), strings.ToLower(cond.Pattern))
	case OpEndsWith:
		return strings.HasSuffix(strings.ToLower(fieldValue), strings.ToLower(cond.Pattern))
	default:
		return false
	}
}

// regexMatch performs a case-insensitive regex search on the value.
func regexMatch(pattern, value string) bool {
	re, err := regexp.Compile("(?i)" + pattern)
	if err != nil {
		return false
	}
	return re.MatchString(value)
}

// applyModify applies the modify action to the event's tool args.
// It reads the target arg from the event, applies the strategy using
// the rule's message body as the modification content, and returns
// the full modified args map.
func applyModify(rule *Rule, event *schema.Event) map[string]interface{} {
	args := getToolArgs(event)
	if args == nil {
		args = map[string]interface{}{}
	}

	// Copy args to avoid mutating the original
	result := make(map[string]interface{}, len(args))
	for k, v := range args {
		result[k] = v
	}

	target := rule.ModifyTarget
	content := rule.Message
	strategy := rule.ModifyStrategy

	// Get current value of the target arg (empty string if not present)
	currentVal := ""
	if v, ok := result[target]; ok {
		if s, ok := v.(string); ok {
			currentVal = s
		}
	}

	var newVal string
	switch strategy {
	case StrategyPrepend:
		newVal = content + currentVal
	case StrategyAppend:
		newVal = currentVal + content
	case StrategyReplace:
		newVal = content
	case StrategyRegex:
		// For regex strategy, the content is in format: /pattern/replacement/
		// Or simply treated as a replacement for the full value if no regex delimiters
		newVal = applyRegexStrategy(currentVal, content)
	default:
		newVal = content
	}

	result[target] = newVal
	return result
}

// applyRegexStrategy applies a regex substitution.
// Content format: s/pattern/replacement/ (sed-style) or just replacement text.
func applyRegexStrategy(currentVal, content string) string {
	// Try sed-style: s/pattern/replacement/
	if len(content) > 2 && content[0] == 's' && content[1] == '/' {
		parts := strings.SplitN(content[2:], "/", 3)
		if len(parts) >= 2 {
			pattern := parts[0]
			replacement := parts[1]
			re, err := regexp.Compile(pattern)
			if err != nil {
				return currentVal // invalid regex, return unchanged
			}
			return re.ReplaceAllString(currentVal, replacement)
		}
	}
	// Fallback: treat content as full replacement
	return content
}

