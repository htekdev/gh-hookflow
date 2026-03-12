package hookify

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func boolPtr(b bool) *bool { return &b }

func TestParseRuleFromBytes_ValidSimplePattern(t *testing.T) {
	input := `---
name: block-rm-rf
description: Block dangerous rm commands
event: bash
action: block
pattern: rm\s+-rf
---

⚠️ **Dangerous command blocked**

Do not use rm -rf.
`
	rule, err := ParseRuleFromBytes([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rule.Name != "block-rm-rf" {
		t.Errorf("expected name %q, got %q", "block-rm-rf", rule.Name)
	}
	if rule.Description != "Block dangerous rm commands" {
		t.Errorf("expected description %q, got %q", "Block dangerous rm commands", rule.Description)
	}
	if rule.Event != "bash" {
		t.Errorf("expected event %q, got %q", "bash", rule.Event)
	}
	if rule.Action != "block" {
		t.Errorf("expected action %q, got %q", "block", rule.Action)
	}
	if rule.Pattern != `rm\s+-rf` {
		t.Errorf("expected pattern %q, got %q", `rm\s+-rf`, rule.Pattern)
	}
	if rule.Message != "⚠️ **Dangerous command blocked**\n\nDo not use rm -rf." {
		t.Errorf("unexpected message: %q", rule.Message)
	}
	// Pattern should be auto-converted to condition
	if len(rule.Conditions) != 1 {
		t.Fatalf("expected 1 condition, got %d", len(rule.Conditions))
	}
	if rule.Conditions[0].Field != FieldCommand {
		t.Errorf("expected field %q (bash event → command), got %q", FieldCommand, rule.Conditions[0].Field)
	}
	if rule.Conditions[0].Operator != OpRegexMatch {
		t.Errorf("expected operator %q, got %q", OpRegexMatch, rule.Conditions[0].Operator)
	}
	if rule.Conditions[0].Pattern != `rm\s+-rf` {
		t.Errorf("expected condition pattern %q, got %q", `rm\s+-rf`, rule.Conditions[0].Pattern)
	}
}

func TestParseRuleFromBytes_ValidConditions(t *testing.T) {
	input := `---
name: require-tests
event: file
action: block
conditions:
  - field: file_path
    operator: regex_match
    pattern: ^src/.*\.ts$
  - field: transcript
    operator: not_contains
    pattern: \.test\.ts
---

Source changes require corresponding test files.
`
	rule, err := ParseRuleFromBytes([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rule.Name != "require-tests" {
		t.Errorf("expected name %q, got %q", "require-tests", rule.Name)
	}
	if len(rule.Conditions) != 2 {
		t.Fatalf("expected 2 conditions, got %d", len(rule.Conditions))
	}
	if rule.Conditions[0].Field != FieldFilePath {
		t.Errorf("expected first condition field %q, got %q", FieldFilePath, rule.Conditions[0].Field)
	}
	if rule.Conditions[0].Operator != OpRegexMatch {
		t.Errorf("expected first condition operator %q, got %q", OpRegexMatch, rule.Conditions[0].Operator)
	}
	if rule.Conditions[1].Field != FieldTranscript {
		t.Errorf("expected second condition field %q, got %q", FieldTranscript, rule.Conditions[1].Field)
	}
	if rule.Conditions[1].Operator != OpNotContains {
		t.Errorf("expected second condition operator %q, got %q", OpNotContains, rule.Conditions[1].Operator)
	}
}

func TestParseRuleFromBytes_DefaultAction(t *testing.T) {
	input := `---
name: warn-console
event: file
pattern: console\.log
---

Console.log detected.
`
	rule, err := ParseRuleFromBytes([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Action should be empty (GetAction() returns "warn" by default)
	if rule.Action != "" {
		t.Errorf("expected empty action (default warn), got %q", rule.Action)
	}
	if rule.GetAction() != ActionWarn {
		t.Errorf("GetAction() expected %q, got %q", ActionWarn, rule.GetAction())
	}
}

func TestParseRuleFromBytes_EnabledDefaults(t *testing.T) {
	input := `---
name: test-rule
event: bash
pattern: test
---

Test.
`
	rule, err := ParseRuleFromBytes([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rule.Enabled != nil {
		t.Errorf("expected nil Enabled (unset), got %v", *rule.Enabled)
	}
	if !rule.IsEnabled() {
		t.Error("expected IsEnabled() to return true when Enabled is nil")
	}
}

func TestParseRuleFromBytes_ExplicitlyDisabled(t *testing.T) {
	input := `---
name: disabled-rule
enabled: false
event: bash
pattern: test
---

Disabled.
`
	rule, err := ParseRuleFromBytes([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rule.Enabled == nil {
		t.Fatal("expected Enabled to be non-nil")
	}
	if *rule.Enabled != false {
		t.Error("expected Enabled to be false")
	}
	if rule.IsEnabled() {
		t.Error("expected IsEnabled() to return false")
	}
}

func TestParseRuleFromBytes_ExplicitlyEnabled(t *testing.T) {
	input := `---
name: enabled-rule
enabled: true
event: bash
pattern: test
---

Enabled.
`
	rule, err := ParseRuleFromBytes([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rule.Enabled == nil {
		t.Fatal("expected Enabled to be non-nil")
	}
	if *rule.Enabled != true {
		t.Error("expected Enabled to be true")
	}
	if !rule.IsEnabled() {
		t.Error("expected IsEnabled() to return true")
	}
}

func TestParseRuleFromBytes_LifecycleDefaults(t *testing.T) {
	input := `---
name: test-rule
event: bash
pattern: test
---

Test.
`
	rule, err := ParseRuleFromBytes([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rule.Lifecycle != "" {
		t.Errorf("expected empty lifecycle (default pre), got %q", rule.Lifecycle)
	}
	if rule.GetLifecycle() != LifecyclePre {
		t.Errorf("GetLifecycle() expected %q, got %q", LifecyclePre, rule.GetLifecycle())
	}
}

func TestParseRuleFromBytes_ExplicitLifecycle(t *testing.T) {
	input := `---
name: post-rule
event: file
lifecycle: post
pattern: test
---

Post lifecycle.
`
	rule, err := ParseRuleFromBytes([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rule.GetLifecycle() != LifecyclePost {
		t.Errorf("GetLifecycle() expected %q, got %q", LifecyclePost, rule.GetLifecycle())
	}
}

func TestParseRuleFromBytes_ToolMatcher(t *testing.T) {
	input := `---
name: tool-match-rule
event: all
pattern: test
tool_matcher: ^(powershell|bash)$
---

Tool matcher test.
`
	rule, err := ParseRuleFromBytes([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rule.ToolMatcher != "^(powershell|bash)$" {
		t.Errorf("expected tool_matcher %q, got %q", "^(powershell|bash)$", rule.ToolMatcher)
	}
}

func TestParseRuleFromBytes_EmptyBody(t *testing.T) {
	input := `---
name: no-body-rule
event: bash
pattern: test
---
`
	rule, err := ParseRuleFromBytes([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rule.Message != "" {
		t.Errorf("expected empty message, got %q", rule.Message)
	}
}

func TestParseRuleFromBytes_PatternConversion(t *testing.T) {
	tests := []struct {
		name          string
		event         string
		expectedField string
	}{
		{"bash event converts to command field", EventBash, FieldCommand},
		{"file event converts to file_path field", EventFile, FieldFilePath},
		{"all event converts to content field", EventAll, FieldContent},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := "---\nname: test\nevent: " + tt.event + "\npattern: testpattern\n---\nMsg.\n"
			rule, err := ParseRuleFromBytes([]byte(input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(rule.Conditions) != 1 {
				t.Fatalf("expected 1 condition, got %d", len(rule.Conditions))
			}
			if rule.Conditions[0].Field != tt.expectedField {
				t.Errorf("expected field %q for event %q, got %q", tt.expectedField, tt.event, rule.Conditions[0].Field)
			}
			if rule.Conditions[0].Operator != OpRegexMatch {
				t.Errorf("expected operator %q, got %q", OpRegexMatch, rule.Conditions[0].Operator)
			}
		})
	}
}

// Error cases

func TestParseRuleFromBytes_MissingFrontmatter(t *testing.T) {
	input := `name: no-delimiters
event: bash
pattern: test
`
	_, err := ParseRuleFromBytes([]byte(input))
	if err == nil {
		t.Fatal("expected error for missing frontmatter delimiters")
	}
}

func TestParseRuleFromBytes_MissingClosingDelimiter(t *testing.T) {
	input := `---
name: no-closing
event: bash
pattern: test
`
	_, err := ParseRuleFromBytes([]byte(input))
	if err == nil {
		t.Fatal("expected error for missing closing delimiter")
	}
}

func TestParseRuleFromBytes_MissingName(t *testing.T) {
	input := `---
event: bash
pattern: test
---

Msg.
`
	_, err := ParseRuleFromBytes([]byte(input))
	if err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestParseRuleFromBytes_MissingEvent(t *testing.T) {
	input := `---
name: no-event
pattern: test
---

Msg.
`
	_, err := ParseRuleFromBytes([]byte(input))
	if err == nil {
		t.Fatal("expected error for missing event")
	}
}

func TestParseRuleFromBytes_InvalidEvent(t *testing.T) {
	input := `---
name: bad-event
event: invalid_event
pattern: test
---

Msg.
`
	_, err := ParseRuleFromBytes([]byte(input))
	if err == nil {
		t.Fatal("expected error for invalid event type")
	}
}

func TestParseRuleFromBytes_InvalidAction(t *testing.T) {
	input := `---
name: bad-action
event: bash
action: invalid_action
pattern: test
---

Msg.
`
	_, err := ParseRuleFromBytes([]byte(input))
	if err == nil {
		t.Fatal("expected error for invalid action")
	}
}

func TestParseRuleFromBytes_NeitherPatternNorConditions(t *testing.T) {
	input := `---
name: no-pattern-or-conditions
event: bash
---

Msg.
`
	_, err := ParseRuleFromBytes([]byte(input))
	if err == nil {
		t.Fatal("expected error when neither pattern nor conditions are provided")
	}
}

func TestParseRuleFromBytes_BothPatternAndConditions(t *testing.T) {
	input := `---
name: both
event: bash
pattern: test
conditions:
  - field: command
    operator: contains
    pattern: test
---

Msg.
`
	_, err := ParseRuleFromBytes([]byte(input))
	if err == nil {
		t.Fatal("expected error when both pattern and conditions are provided")
	}
}

func TestParseRuleFromBytes_ConditionMissingField(t *testing.T) {
	input := `---
name: missing-field
event: bash
conditions:
  - operator: contains
    pattern: test
---

Msg.
`
	_, err := ParseRuleFromBytes([]byte(input))
	if err == nil {
		t.Fatal("expected error for condition missing field")
	}
}

func TestParseRuleFromBytes_ConditionInvalidField(t *testing.T) {
	input := `---
name: invalid-field
event: bash
conditions:
  - field: nonexistent
    operator: contains
    pattern: test
---

Msg.
`
	_, err := ParseRuleFromBytes([]byte(input))
	if err == nil {
		t.Fatal("expected error for invalid condition field")
	}
}

func TestParseRuleFromBytes_ConditionMissingOperator(t *testing.T) {
	input := `---
name: missing-operator
event: bash
conditions:
  - field: command
    pattern: test
---

Msg.
`
	_, err := ParseRuleFromBytes([]byte(input))
	if err == nil {
		t.Fatal("expected error for condition missing operator")
	}
}

func TestParseRuleFromBytes_ConditionInvalidOperator(t *testing.T) {
	input := `---
name: invalid-operator
event: bash
conditions:
  - field: command
    operator: nonexistent
    pattern: test
---

Msg.
`
	_, err := ParseRuleFromBytes([]byte(input))
	if err == nil {
		t.Fatal("expected error for invalid condition operator")
	}
}

func TestParseRuleFromBytes_ConditionMissingPattern(t *testing.T) {
	input := `---
name: missing-pattern
event: bash
conditions:
  - field: command
    operator: contains
---

Msg.
`
	_, err := ParseRuleFromBytes([]byte(input))
	if err == nil {
		t.Fatal("expected error for condition missing pattern")
	}
}

func TestParseRuleFromBytes_ContentFieldRejectsPositionalOperators(t *testing.T) {
	operators := []string{"equals", "starts_with", "ends_with"}
	for _, op := range operators {
		t.Run(op, func(t *testing.T) {
			input := fmt.Sprintf(`---
name: content-%s
event: all
conditions:
  - field: content
    operator: %s
    pattern: test
---

Msg.
`, op, op)
			_, err := ParseRuleFromBytes([]byte(input))
			if err == nil {
				t.Fatalf("expected error for content field with operator %q, but got nil", op)
			}
			if !strings.Contains(err.Error(), "not supported with field") {
				t.Errorf("expected 'not supported with field' error, got: %v", err)
			}
		})
	}
}

func TestParseRuleFromBytes_ContentFieldAllowsSafeOperators(t *testing.T) {
	operators := []string{"contains", "not_contains", "regex_match"}
	for _, op := range operators {
		t.Run(op, func(t *testing.T) {
			input := fmt.Sprintf(`---
name: content-%s
event: all
conditions:
  - field: content
    operator: %s
    pattern: test
---

Msg.
`, op, op)
			_, err := ParseRuleFromBytes([]byte(input))
			if err != nil {
				t.Fatalf("unexpected error for content field with operator %q: %v", op, err)
			}
		})
	}
}

func TestParseRuleFromBytes_NonContentFieldAllowsAllOperators(t *testing.T) {
	// equals/starts_with/ends_with should be fine for non-content fields
	operators := []string{"equals", "starts_with", "ends_with"}
	for _, op := range operators {
		t.Run(op, func(t *testing.T) {
			input := fmt.Sprintf(`---
name: filepath-%s
event: file
conditions:
  - field: file_path
    operator: %s
    pattern: test.go
---

Msg.
`, op, op)
			_, err := ParseRuleFromBytes([]byte(input))
			if err != nil {
				t.Fatalf("unexpected error for file_path field with operator %q: %v", op, err)
			}
		})
	}
}

func TestParseRuleFromBytes_MalformedYAML(t *testing.T) {
	input := `---
name: bad-yaml
event: [invalid yaml
---

Msg.
`
	_, err := ParseRuleFromBytes([]byte(input))
	if err == nil {
		t.Fatal("expected error for malformed YAML")
	}
}

func TestParseRuleFromBytes_WindowsLineEndings(t *testing.T) {
	input := "---\r\nname: windows-rule\r\nevent: bash\r\npattern: test\r\n---\r\nWindows message.\r\n"
	rule, err := ParseRuleFromBytes([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rule.Name != "windows-rule" {
		t.Errorf("expected name %q, got %q", "windows-rule", rule.Name)
	}
}

func TestParseRule_FromFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test-rule.md")
	content := `---
name: file-rule
event: file
action: block
pattern: \.env$
---

Sensitive file blocked.
`
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	rule, err := ParseRule(filePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rule.Name != "file-rule" {
		t.Errorf("expected name %q, got %q", "file-rule", rule.Name)
	}
	if rule.FilePath != filePath {
		t.Errorf("expected FilePath %q, got %q", filePath, rule.FilePath)
	}
}

func TestParseRule_NonexistentFile(t *testing.T) {
	_, err := ParseRule("/nonexistent/path/rule.md")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestParseRuleFromBytes_AllValidOperatorsInConditions(t *testing.T) {
	operators := []string{
		OpRegexMatch, OpContains, OpEquals,
		OpNotContains, OpStartsWith, OpEndsWith,
	}
	for _, op := range operators {
		t.Run(op, func(t *testing.T) {
			input := "---\nname: op-test\nevent: bash\nconditions:\n  - field: command\n    operator: " + op + "\n    pattern: test\n---\nMsg.\n"
			rule, err := ParseRuleFromBytes([]byte(input))
			if err != nil {
				t.Fatalf("unexpected error for operator %q: %v", op, err)
			}
			if rule.Conditions[0].Operator != op {
				t.Errorf("expected operator %q, got %q", op, rule.Conditions[0].Operator)
			}
		})
	}
}

func TestParseRuleFromBytes_AllValidFieldsInConditions(t *testing.T) {
	fields := []string{
		FieldCommand, FieldFilePath, FieldNewText,
		FieldOldText, FieldContent, FieldTranscript,
	}
	for _, f := range fields {
		t.Run(f, func(t *testing.T) {
			input := "---\nname: field-test\nevent: all\nconditions:\n  - field: " + f + "\n    operator: contains\n    pattern: test\n---\nMsg.\n"
			rule, err := ParseRuleFromBytes([]byte(input))
			if err != nil {
				t.Fatalf("unexpected error for field %q: %v", f, err)
			}
			if rule.Conditions[0].Field != f {
				t.Errorf("expected field %q, got %q", f, rule.Conditions[0].Field)
			}
		})
	}
}

func TestParseRuleFromBytes_AllValidEventTypes(t *testing.T) {
	events := []string{EventBash, EventFile, EventAll, EventStop, EventPrompt}
	for _, e := range events {
		t.Run(e, func(t *testing.T) {
			input := "---\nname: event-test\nevent: " + e + "\npattern: test\n---\nMsg.\n"
			rule, err := ParseRuleFromBytes([]byte(input))
			if err != nil {
				t.Fatalf("unexpected error for event %q: %v", e, err)
			}
			if rule.Event != e {
				t.Errorf("expected event %q, got %q", e, rule.Event)
			}
		})
	}
}

func TestParseRuleFromBytes_ConditionsNotAutoConverted(t *testing.T) {
	input := `---
name: explicit-conditions
event: bash
conditions:
  - field: file_path
    operator: contains
    pattern: src/
---

Msg.
`
	rule, err := ParseRuleFromBytes([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should use the explicit condition, not auto-convert
	if len(rule.Conditions) != 1 {
		t.Fatalf("expected 1 condition, got %d", len(rule.Conditions))
	}
	if rule.Conditions[0].Field != FieldFilePath {
		t.Errorf("expected field %q, got %q", FieldFilePath, rule.Conditions[0].Field)
	}
}
