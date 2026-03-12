package hookify

import (
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
