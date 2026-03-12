package hookify

import (
	"testing"

	"github.com/htekdev/gh-hookflow/internal/schema"
)

func TestMatchEvent_DisabledRule(t *testing.T) {
	rule := &Rule{
		Name:    "disabled",
		Enabled: boolPtr(false),
		Event:   EventBash,
		Conditions: []Condition{
			{Field: FieldCommand, Operator: OpContains, Pattern: "test"},
		},
	}
	event := &schema.Event{
		Tool: &schema.ToolEvent{Name: "powershell"},
	}
	if MatchEvent(rule, event) {
		t.Error("expected disabled rule not to match")
	}
}

func TestMatchEvent_EnabledNilDefaultsToTrue(t *testing.T) {
	rule := &Rule{
		Name:  "enabled-nil",
		Event: EventBash,
		Conditions: []Condition{
			{Field: FieldCommand, Operator: OpContains, Pattern: "test"},
		},
	}
	event := &schema.Event{
		Tool: &schema.ToolEvent{Name: "powershell"},
	}
	if !MatchEvent(rule, event) {
		t.Error("expected rule with nil Enabled to match (default true)")
	}
}

func TestMatchEvent_BashEventType(t *testing.T) {
	bashTools := []string{"powershell", "bash", "shell", "terminal"}
	nonBashTools := []string{"create", "edit", "view", "grep", "unknown"}

	rule := &Rule{
		Name:  "bash-rule",
		Event: EventBash,
		Conditions: []Condition{
			{Field: FieldCommand, Operator: OpContains, Pattern: "test"},
		},
	}

	for _, toolName := range bashTools {
		t.Run("matches_"+toolName, func(t *testing.T) {
			event := &schema.Event{
				Tool: &schema.ToolEvent{Name: toolName},
			}
			if !MatchEvent(rule, event) {
				t.Errorf("expected bash rule to match tool %q", toolName)
			}
		})
	}

	for _, toolName := range nonBashTools {
		t.Run("no_match_"+toolName, func(t *testing.T) {
			event := &schema.Event{
				Tool: &schema.ToolEvent{Name: toolName},
			}
			if MatchEvent(rule, event) {
				t.Errorf("expected bash rule NOT to match tool %q", toolName)
			}
		})
	}
}

func TestMatchEvent_BashEventNilTool(t *testing.T) {
	rule := &Rule{
		Name:  "bash-nil-tool",
		Event: EventBash,
		Conditions: []Condition{
			{Field: FieldCommand, Operator: OpContains, Pattern: "test"},
		},
	}
	event := &schema.Event{}
	if MatchEvent(rule, event) {
		t.Error("expected bash rule NOT to match event with nil tool")
	}
}

func TestMatchEvent_FileEventType(t *testing.T) {
	rule := &Rule{
		Name:  "file-rule",
		Event: EventFile,
		Conditions: []Condition{
			{Field: FieldFilePath, Operator: OpContains, Pattern: "test"},
		},
	}

	t.Run("matches_create_tool", func(t *testing.T) {
		event := &schema.Event{
			Tool: &schema.ToolEvent{Name: "create"},
		}
		if !MatchEvent(rule, event) {
			t.Error("expected file rule to match 'create' tool")
		}
	})

	t.Run("matches_edit_tool", func(t *testing.T) {
		event := &schema.Event{
			Tool: &schema.ToolEvent{Name: "edit"},
		}
		if !MatchEvent(rule, event) {
			t.Error("expected file rule to match 'edit' tool")
		}
	})

	t.Run("matches_file_event", func(t *testing.T) {
		event := &schema.Event{
			File: &schema.FileEvent{Path: "test.txt", Action: "create"},
		}
		if !MatchEvent(rule, event) {
			t.Error("expected file rule to match event with File set")
		}
	})

	t.Run("no_match_bash_tool", func(t *testing.T) {
		event := &schema.Event{
			Tool: &schema.ToolEvent{Name: "powershell"},
		}
		if MatchEvent(rule, event) {
			t.Error("expected file rule NOT to match 'powershell' tool")
		}
	})

	t.Run("no_match_nil_everything", func(t *testing.T) {
		event := &schema.Event{}
		if MatchEvent(rule, event) {
			t.Error("expected file rule NOT to match empty event")
		}
	})
}

func TestMatchEvent_AllEventType(t *testing.T) {
	rule := &Rule{
		Name:  "all-rule",
		Event: EventAll,
		Conditions: []Condition{
			{Field: FieldContent, Operator: OpContains, Pattern: "test"},
		},
	}

	t.Run("matches_bash_tool", func(t *testing.T) {
		event := &schema.Event{
			Tool: &schema.ToolEvent{Name: "powershell"},
		}
		if !MatchEvent(rule, event) {
			t.Error("expected 'all' rule to match any event")
		}
	})

	t.Run("matches_file_tool", func(t *testing.T) {
		event := &schema.Event{
			Tool: &schema.ToolEvent{Name: "create"},
		}
		if !MatchEvent(rule, event) {
			t.Error("expected 'all' rule to match any event")
		}
	})

	t.Run("matches_empty_event", func(t *testing.T) {
		event := &schema.Event{}
		if !MatchEvent(rule, event) {
			t.Error("expected 'all' rule to match empty event")
		}
	})
}

func TestMatchEvent_DeferredEventTypes(t *testing.T) {
	for _, eventType := range []string{EventStop, EventPrompt} {
		t.Run(eventType, func(t *testing.T) {
			rule := &Rule{
				Name:  "deferred-" + eventType,
				Event: eventType,
				Conditions: []Condition{
					{Field: FieldCommand, Operator: OpContains, Pattern: "test"},
				},
			}
			event := &schema.Event{
				Tool: &schema.ToolEvent{Name: "powershell"},
			}
			if MatchEvent(rule, event) {
				t.Errorf("expected deferred event type %q NOT to match", eventType)
			}
		})
	}
}

func TestMatchEvent_UnknownEventType(t *testing.T) {
	rule := &Rule{
		Name:  "unknown-event",
		Event: "unknown",
		Conditions: []Condition{
			{Field: FieldCommand, Operator: OpContains, Pattern: "test"},
		},
	}
	event := &schema.Event{
		Tool: &schema.ToolEvent{Name: "powershell"},
	}
	if MatchEvent(rule, event) {
		t.Error("expected unknown event type NOT to match")
	}
}

func TestMatchEvent_ToolMatcherMatches(t *testing.T) {
	rule := &Rule{
		Name:        "tool-matcher-rule",
		Event:       EventAll,
		ToolMatcher: "^power",
		Conditions: []Condition{
			{Field: FieldContent, Operator: OpContains, Pattern: "test"},
		},
	}

	t.Run("matches", func(t *testing.T) {
		event := &schema.Event{
			Tool: &schema.ToolEvent{Name: "powershell"},
		}
		if !MatchEvent(rule, event) {
			t.Error("expected tool_matcher to match 'powershell'")
		}
	})

	t.Run("no_match", func(t *testing.T) {
		event := &schema.Event{
			Tool: &schema.ToolEvent{Name: "bash"},
		}
		if MatchEvent(rule, event) {
			t.Error("expected tool_matcher NOT to match 'bash'")
		}
	})

	t.Run("nil_tool", func(t *testing.T) {
		event := &schema.Event{}
		if MatchEvent(rule, event) {
			t.Error("expected tool_matcher NOT to match nil tool")
		}
	})
}

func TestMatchEvent_ToolMatcherCaseInsensitive(t *testing.T) {
	rule := &Rule{
		Name:        "case-insensitive",
		Event:       EventAll,
		ToolMatcher: "powershell",
		Conditions: []Condition{
			{Field: FieldContent, Operator: OpContains, Pattern: "test"},
		},
	}
	event := &schema.Event{
		Tool: &schema.ToolEvent{Name: "PowerShell"},
	}
	if !MatchEvent(rule, event) {
		t.Error("expected tool_matcher to be case-insensitive")
	}
}

func TestMatchEvent_ToolMatcherInvalidRegex(t *testing.T) {
	rule := &Rule{
		Name:        "invalid-regex",
		Event:       EventAll,
		ToolMatcher: "[invalid(regex",
		Conditions: []Condition{
			{Field: FieldContent, Operator: OpContains, Pattern: "test"},
		},
	}
	event := &schema.Event{
		Tool: &schema.ToolEvent{Name: "powershell"},
	}
	if MatchEvent(rule, event) {
		t.Error("expected invalid regex in tool_matcher NOT to match")
	}
}

func TestMatchEvent_BashToolNameCaseInsensitive(t *testing.T) {
	rule := &Rule{
		Name:  "case-bash",
		Event: EventBash,
		Conditions: []Condition{
			{Field: FieldCommand, Operator: OpContains, Pattern: "test"},
		},
	}
	// BashToolNames uses lowercase keys, and matchEventType lowercases the tool name
	event := &schema.Event{
		Tool: &schema.ToolEvent{Name: "PowerShell"},
	}
	if !MatchEvent(rule, event) {
		t.Error("expected bash event matching to be case-insensitive")
	}
}

func TestMatchEvent_FileToolNameCaseInsensitive(t *testing.T) {
	rule := &Rule{
		Name:  "case-file",
		Event: EventFile,
		Conditions: []Condition{
			{Field: FieldFilePath, Operator: OpContains, Pattern: "test"},
		},
	}
	event := &schema.Event{
		Tool: &schema.ToolEvent{Name: "Create"},
	}
	if !MatchEvent(rule, event) {
		t.Error("expected file event matching to be case-insensitive")
	}
}

func TestMatchEvent_NoToolMatcherSet(t *testing.T) {
	rule := &Rule{
		Name:  "no-tool-matcher",
		Event: EventAll,
		Conditions: []Condition{
			{Field: FieldContent, Operator: OpContains, Pattern: "test"},
		},
	}
	event := &schema.Event{
		Tool: &schema.ToolEvent{Name: "anything"},
	}
	if !MatchEvent(rule, event) {
		t.Error("expected rule without tool_matcher to match any tool (when event type matches)")
	}
}
