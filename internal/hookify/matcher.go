package hookify

import (
	"regexp"
	"strings"

	"github.com/htekdev/gh-hookflow/internal/schema"
)

// MatchEvent checks whether a hookify rule matches the given event.
// Returns false if the rule is disabled, the event type doesn't match,
// or the optional tool_matcher regex doesn't match the tool name.
func MatchEvent(rule *Rule, event *schema.Event) bool {
	if !rule.IsEnabled() {
		return false
	}

	if !matchEventType(rule.Event, event) {
		return false
	}

	if rule.ToolMatcher != "" {
		if !matchToolMatcher(rule.ToolMatcher, event) {
			return false
		}
	}

	return true
}

// matchEventType checks whether the hookify event type matches the schema event.
func matchEventType(eventType string, event *schema.Event) bool {
	switch eventType {
	case EventAll:
		return true

	case EventBash:
		if event.Tool == nil {
			return false
		}
		return BashToolNames[strings.ToLower(event.Tool.Name)]

	case EventFile:
		// Match if it's a file event or if the tool name is a file tool
		if event.File != nil {
			return true
		}
		if event.Tool != nil {
			return FileToolNames[strings.ToLower(event.Tool.Name)]
		}
		return false

	case EventStop, EventPrompt:
		// Deferred — hookflow doesn't have stop/prompt hooks
		return false

	default:
		return false
	}
}

// matchToolMatcher checks the optional tool_matcher regex against the tool name.
func matchToolMatcher(pattern string, event *schema.Event) bool {
	if event.Tool == nil {
		return false
	}
	re, err := regexp.Compile("(?i)" + pattern)
	if err != nil {
		return false
	}
	return re.MatchString(event.Tool.Name)
}
