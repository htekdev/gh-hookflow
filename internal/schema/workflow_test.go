package schema

import (
	"testing"

	"gopkg.in/yaml.v3"
)

// ============================================================================
// GetLifecycle Tests - Triggers
// ============================================================================

func TestFileTrigger_GetLifecycle_Default(t *testing.T) {
	f := &FileTrigger{}
	if got := f.GetLifecycle(); got != "pre" {
		t.Errorf("Expected 'pre', got '%s'", got)
	}
}

func TestFileTrigger_GetLifecycle_Explicit(t *testing.T) {
	f := &FileTrigger{Lifecycle: "post"}
	if got := f.GetLifecycle(); got != "post" {
		t.Errorf("Expected 'post', got '%s'", got)
	}
}

func TestFileTrigger_GetLifecycle_Nil(t *testing.T) {
	var f *FileTrigger
	if got := f.GetLifecycle(); got != "pre" {
		t.Errorf("Expected 'pre' for nil trigger, got '%s'", got)
	}
}

func TestCommitTrigger_GetLifecycle_Default(t *testing.T) {
	c := &CommitTrigger{}
	if got := c.GetLifecycle(); got != "pre" {
		t.Errorf("Expected 'pre', got '%s'", got)
	}
}

func TestCommitTrigger_GetLifecycle_Explicit(t *testing.T) {
	c := &CommitTrigger{Lifecycle: "post"}
	if got := c.GetLifecycle(); got != "post" {
		t.Errorf("Expected 'post', got '%s'", got)
	}
}

func TestPushTrigger_GetLifecycle_Default(t *testing.T) {
	p := &PushTrigger{}
	if got := p.GetLifecycle(); got != "pre" {
		t.Errorf("Expected 'pre', got '%s'", got)
	}
}

func TestPushTrigger_GetLifecycle_Explicit(t *testing.T) {
	p := &PushTrigger{Lifecycle: "post"}
	if got := p.GetLifecycle(); got != "post" {
		t.Errorf("Expected 'post', got '%s'", got)
	}
}

// ============================================================================
// GetLifecycle Tests - Event
// ============================================================================

func TestEvent_GetLifecycle_Default(t *testing.T) {
	e := &Event{}
	if got := e.GetLifecycle(); got != "pre" {
		t.Errorf("Expected 'pre', got '%s'", got)
	}
}

func TestEvent_GetLifecycle_Explicit(t *testing.T) {
	e := &Event{Lifecycle: "post"}
	if got := e.GetLifecycle(); got != "post" {
		t.Errorf("Expected 'post', got '%s'", got)
	}
}

// ============================================================================
// UnmarshalYAML Tests - OnConfig nil-to-empty-struct coercion
// ============================================================================

func TestOnConfig_UnmarshalYAML_EmptyHooks(t *testing.T) {
	input := `hooks:`
	var oc OnConfig
	if err := yaml.Unmarshal([]byte(input), &oc); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if oc.Hooks == nil {
		t.Error("Expected hooks to be non-nil for bare 'hooks:' key")
	}
}

func TestOnConfig_UnmarshalYAML_EmptyFile(t *testing.T) {
	input := `file:`
	var oc OnConfig
	if err := yaml.Unmarshal([]byte(input), &oc); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if oc.File == nil {
		t.Error("Expected file to be non-nil for bare 'file:' key")
	}
}

func TestOnConfig_UnmarshalYAML_EmptyPush(t *testing.T) {
	input := `push:`
	var oc OnConfig
	if err := yaml.Unmarshal([]byte(input), &oc); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if oc.Push == nil {
		t.Error("Expected push to be non-nil for bare 'push:' key")
	}
}

func TestOnConfig_UnmarshalYAML_EmptyCommit(t *testing.T) {
	input := `commit:`
	var oc OnConfig
	if err := yaml.Unmarshal([]byte(input), &oc); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if oc.Commit == nil {
		t.Error("Expected commit to be non-nil for bare 'commit:' key")
	}
}

func TestOnConfig_UnmarshalYAML_AllEmpty(t *testing.T) {
	input := `
hooks:
file:
commit:
push:
`
	var oc OnConfig
	if err := yaml.Unmarshal([]byte(input), &oc); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if oc.Hooks == nil {
		t.Error("Expected hooks non-nil")
	}
	if oc.File == nil {
		t.Error("Expected file non-nil")
	}
	if oc.Commit == nil {
		t.Error("Expected commit non-nil")
	}
	if oc.Push == nil {
		t.Error("Expected push non-nil")
	}
	// tool/tools should remain nil
	if oc.Tool != nil {
		t.Error("Expected tool to be nil")
	}
	if oc.Tools != nil {
		t.Error("Expected tools to be nil")
	}
}

func TestOnConfig_UnmarshalYAML_WithValues(t *testing.T) {
	input := `
hooks:
  types:
    - preToolUse
file:
  paths:
    - "*.go"
commit:
  branches:
    - main
push:
  tags:
    - v*
`
	var oc OnConfig
	if err := yaml.Unmarshal([]byte(input), &oc); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if oc.Hooks == nil || len(oc.Hooks.Types) != 1 {
		t.Error("Expected hooks with 1 type")
	}
	if oc.File == nil || len(oc.File.Paths) != 1 {
		t.Error("Expected file with 1 path")
	}
	if oc.Commit == nil || len(oc.Commit.Branches) != 1 {
		t.Error("Expected commit with 1 branch")
	}
	if oc.Push == nil || len(oc.Push.Tags) != 1 {
		t.Error("Expected push with 1 tag")
	}
}

func TestOnConfig_UnmarshalYAML_InvalidInput(t *testing.T) {
	input := `[invalid`
	var oc OnConfig
	err := yaml.Unmarshal([]byte(input), &oc)
	if err == nil {
		t.Error("Expected error for invalid YAML")
	}
}

func TestOnConfig_UnmarshalYAML_ToolWithName(t *testing.T) {
	input := `
tool:
  name: edit
  args:
    path: "*.go"
`
	var oc OnConfig
	if err := yaml.Unmarshal([]byte(input), &oc); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if oc.Tool == nil {
		t.Fatal("Expected tool to be non-nil")
	}
	if oc.Tool.Name != "edit" {
		t.Errorf("Expected tool name 'edit', got '%s'", oc.Tool.Name)
	}
}

func TestOnConfig_UnmarshalYAML_ToolsArray(t *testing.T) {
	input := `
tools:
  - name: edit
  - name: create
`
	var oc OnConfig
	if err := yaml.Unmarshal([]byte(input), &oc); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(oc.Tools) != 2 {
		t.Errorf("Expected 2 tools, got %d", len(oc.Tools))
	}
}
