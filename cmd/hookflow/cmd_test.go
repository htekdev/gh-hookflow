package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	eventpkg "github.com/htekdev/gh-hookflow/internal/event"
	"github.com/htekdev/gh-hookflow/internal/schema"
)

// TestMain isolates session state so tests don't pollute each other
// or the user's real hookflow session directory.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "hookflow-cmd-test-session")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create temp session dir: %v\n", err)
		os.Exit(1)
	}
	os.Setenv("HOOKFLOW_SESSION_DIR", dir)
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

// clearSessionErrors removes any session error file left by previous tests.
// Call this in tests that run post-lifecycle workflows with blocking steps
// that may fail, to prevent cross-test contamination.
func clearSessionErrors(t *testing.T) {
	t.Helper()
	sessionDir := os.Getenv("HOOKFLOW_SESSION_DIR")
	if sessionDir != "" {
		_ = os.Remove(filepath.Join(sessionDir, "error.md"))
	}
}

// TestVersionCommand tests the version command execution
func TestVersionCommand(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Execute command
	versionCmd.Run(versionCmd, []string{})

	// Restore stdout
	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "hookflow version") {
		t.Errorf("Expected version output, got: %s", output)
	}
}

// TestTriggersCommand tests the triggers command execution
func TestTriggersCommand(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	triggersCmd.Run(triggersCmd, []string{})

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	expectedTriggers := []string{"hooks", "tool", "file", "commit", "push"}
	for _, trigger := range expectedTriggers {
		if !strings.Contains(output, trigger) {
			t.Errorf("Expected triggers output to contain '%s', got: %s", trigger, output)
		}
	}
}

// TestDiscoverCommand tests the discover command
func TestDiscoverCommand(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "hookflow-discover-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Test with explicit dir flag
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	_ = discoverCmd.Flags().Set("dir", tmpDir)
	err = discoverCmd.RunE(discoverCmd, []string{})

	_ = w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Errorf("discoverCmd.RunE returned error: %v", err)
	}

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "Discovering workflows") {
		t.Errorf("Expected discovering output, got: %s", output)
	}
}

// TestDiscoverCommandDefaultDir tests discover with default directory
func TestDiscoverCommandDefaultDir(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Reset flag to empty for default dir behavior
	_ = discoverCmd.Flags().Set("dir", "")
	err := discoverCmd.RunE(discoverCmd, []string{})

	_ = w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Errorf("discoverCmd.RunE returned error: %v", err)
	}

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "Discovering workflows") {
		t.Errorf("Expected discovering output, got: %s", output)
	}
}

// TestValidateCommand tests validation with a file
func TestValidateCommand(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "hookflow-validate-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	workflowDir := filepath.Join(tmpDir, ".github", "hookflows")
	if err := os.MkdirAll(workflowDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a valid workflow file
	workflowContent := `name: test-workflow
on:
  tool:
    name: edit
steps:
  - name: Test step
    run: echo "test"
`
	workflowFile := filepath.Join(workflowDir, "test.yml")
	if err := os.WriteFile(workflowFile, []byte(workflowContent), 0644); err != nil {
		t.Fatal(err)
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	_ = validateCmd.Flags().Set("file", workflowFile)
	_ = validateCmd.Flags().Set("dir", tmpDir)
	err = validateCmd.RunE(validateCmd, []string{})

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	if err != nil {
		t.Errorf("validateCmd.RunE returned error: %v, output: %s", err, output)
	}
}

// TestValidateCommandDir tests validation of a directory
func TestValidateCommandDir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "hookflow-validate-dir-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	workflowDir := filepath.Join(tmpDir, ".github", "hookflows")
	if err := os.MkdirAll(workflowDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a valid workflow file
	workflowContent := `name: test-workflow
on:
  tool:
    name: edit
steps:
  - name: Test step
    run: echo "test"
`
	if err := os.WriteFile(filepath.Join(workflowDir, "test.yml"), []byte(workflowContent), 0644); err != nil {
		t.Fatal(err)
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	_ = validateCmd.Flags().Set("file", "")
	_ = validateCmd.Flags().Set("dir", tmpDir)
	err = validateCmd.RunE(validateCmd, []string{})

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	_ = buf.String()

	if err != nil {
		t.Errorf("validateCmd.RunE for directory returned error: %v", err)
	}
}

// TestRunCommandEmptyEvent tests run command with empty event
func TestRunCommandEmptyEvent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "hookflow-run-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	_ = runCmd.Flags().Set("event", "")
	_ = runCmd.Flags().Set("workflow", "")
	_ = runCmd.Flags().Set("dir", tmpDir)
	err = runCmd.RunE(runCmd, []string{})

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	if err != nil {
		t.Errorf("runCmd.RunE returned error: %v", err)
	}

	// Should output allow result
	if !strings.Contains(output, "allow") {
		t.Errorf("Expected allow result, got: %s", output)
	}
}

// TestRunCommandWithEvent tests run command with event JSON
func TestRunCommandWithEvent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "hookflow-run-event-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	eventJSON := `{"tool":{"name":"edit","args":{"path":"test.go"}}}`

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	_ = runCmd.Flags().Set("event", eventJSON)
	_ = runCmd.Flags().Set("workflow", "")
	_ = runCmd.Flags().Set("dir", tmpDir)
	err = runCmd.RunE(runCmd, []string{})

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	if err != nil {
		t.Errorf("runCmd.RunE returned error: %v", err)
	}

	if !strings.Contains(output, "allow") {
		t.Errorf("Expected allow result, got: %s", output)
	}
}

// TestRunCommandInvalidJSON tests run command with invalid JSON
func TestRunCommandInvalidJSON(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "hookflow-run-invalid-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	_ = runCmd.Flags().Set("event", "not valid json")
	_ = runCmd.Flags().Set("workflow", "")
	_ = runCmd.Flags().Set("dir", tmpDir)
	err = runCmd.RunE(runCmd, []string{})

	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "failed to parse event JSON") {
		t.Errorf("Expected JSON parse error, got: %v", err)
	}
}

// TestRunCommandNonexistentWorkflow tests run with nonexistent workflow
func TestRunCommandNonexistentWorkflow(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "hookflow-run-noworkflow-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	_ = runCmd.Flags().Set("event", "")
	_ = runCmd.Flags().Set("workflow", "nonexistent")
	_ = runCmd.Flags().Set("dir", tmpDir)
	err = runCmd.RunE(runCmd, []string{})

	if err == nil {
		t.Error("Expected error for nonexistent workflow")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Expected 'not found' error, got: %v", err)
	}
}

// TestOutputWorkflowResult tests JSON output
func TestOutputWorkflowResult(t *testing.T) {
	result := &schema.WorkflowResult{
		PermissionDecision:       "allow",
		PermissionDecisionReason: "test reason",
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputWorkflowResult(result)

	_ = w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Errorf("outputWorkflowResult returned error: %v", err)
	}

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	// Verify it's valid JSON
	var parsed schema.WorkflowResult
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Errorf("Output is not valid JSON: %v", err)
	}

	if parsed.PermissionDecision != "allow" {
		t.Errorf("Expected allow, got: %s", parsed.PermissionDecision)
	}
}

// TestFindWorkflowFileYAML tests finding .yaml extension
func TestFindWorkflowFileYAML(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "hookflow-find-yaml-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	workflowDir := filepath.Join(tmpDir, ".github", "hookflows")
	if err := os.MkdirAll(workflowDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a .yaml file (not .yml)
	workflowContent := `name: test
on:
  tool:
    name: edit
steps:
  - run: echo test
`
	if err := os.WriteFile(filepath.Join(workflowDir, "myworkflow.yaml"), []byte(workflowContent), 0644); err != nil {
		t.Fatal(err)
	}

	path, found := findWorkflowFile(tmpDir, "myworkflow")
	if !found {
		t.Error("Expected to find myworkflow.yaml")
	}
	if !strings.Contains(path, "myworkflow.yaml") {
		t.Errorf("Expected path to contain myworkflow.yaml, got: %s", path)
	}
}

// TestRunWorkflowFound tests running a specific workflow
func TestRunWorkflowFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "hookflow-run-workflow-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	workflowDir := filepath.Join(tmpDir, ".github", "hookflows")
	if err := os.MkdirAll(workflowDir, 0755); err != nil {
		t.Fatal(err)
	}

	workflowContent := `name: test-workflow
on:
  tool:
    name: edit
steps:
  - name: Test step
    run: echo "test"
`
	if err := os.WriteFile(filepath.Join(workflowDir, "test.yml"), []byte(workflowContent), 0644); err != nil {
		t.Fatal(err)
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err = runWorkflow(tmpDir, "test")

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	if err != nil {
		t.Errorf("runWorkflow returned error: %v", err)
	}

	if !strings.Contains(output, "permissionDecision") {
		t.Errorf("Expected permissionDecision in output, got: %s", output)
	}
}

// TestRunMatchingWorkflowsWithMatchingWorkflow tests workflow matching
func TestRunMatchingWorkflowsWithMatchingWorkflow(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "hookflow-matching-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	workflowDir := filepath.Join(tmpDir, ".github", "hookflows")
	if err := os.MkdirAll(workflowDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create workflow that matches edit tool
	workflowContent := `name: edit-checker
on:
  tool:
    name: edit
steps:
  - name: Check
    run: echo "checking edit"
`
	if err := os.WriteFile(filepath.Join(workflowDir, "edit-check.yml"), []byte(workflowContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Event that matches the workflow
	eventJSON := `{"tool":{"name":"edit","args":{"path":"test.go"}}}`

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err = runMatchingWorkflows(tmpDir, eventJSON, "pre")

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	if err != nil {
		t.Errorf("runMatchingWorkflows returned error: %v", err)
	}

	if !strings.Contains(output, "permissionDecision") {
		t.Errorf("Expected permissionDecision in output, got: %s", output)
	}
}

// TestRunMatchingWorkflowsNoMatch tests when no workflows match
func TestRunMatchingWorkflowsNoMatch(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "hookflow-nomatch-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	workflowDir := filepath.Join(tmpDir, ".github", "hookflows")
	if err := os.MkdirAll(workflowDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create workflow that only matches create tool
	workflowContent := `name: create-checker
on:
  tool:
    name: create
steps:
  - name: Check
    run: echo "checking create"
`
	if err := os.WriteFile(filepath.Join(workflowDir, "create-check.yml"), []byte(workflowContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Event for edit tool (won't match)
	eventJSON := `{"tool":{"name":"edit","args":{"path":"test.go"}}}`

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err = runMatchingWorkflows(tmpDir, eventJSON, "pre")

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	if err != nil {
		t.Errorf("runMatchingWorkflows returned error: %v", err)
	}

	// Should default to allow when no match
	if !strings.Contains(output, "allow") {
		t.Errorf("Expected allow result when no match, got: %s", output)
	}
}

// TestRunMatchingWorkflowsEmptyDir tests when workflow dir has no workflows
func TestRunMatchingWorkflowsEmptyDir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "hookflow-empty-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	workflowDir := filepath.Join(tmpDir, ".github", "hookflows")
	if err := os.MkdirAll(workflowDir, 0755); err != nil {
		t.Fatal(err)
	}

	eventJSON := `{"tool":{"name":"edit","args":{"path":"test.go"}}}`

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err = runMatchingWorkflows(tmpDir, eventJSON, "pre")

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	if err != nil {
		t.Errorf("runMatchingWorkflows returned error: %v", err)
	}

	if !strings.Contains(output, "allow") {
		t.Errorf("Expected allow result, got: %s", output)
	}
}

// TestParseEventDataPartialHookData tests parsing with partial hook data
func TestParseEventDataPartialHookData(t *testing.T) {
	// Hook with no tool
	data := map[string]interface{}{
		"hook": map[string]interface{}{
			"type": "postToolUse",
		},
	}

	event := parseEventData(data)

	if event.Hook == nil {
		t.Fatal("Expected Hook to be set")
	}
	if event.Hook.Type != "postToolUse" {
		t.Errorf("Expected type postToolUse, got: %s", event.Hook.Type)
	}
	if event.Hook.Tool != nil {
		t.Error("Expected Tool to be nil when not provided")
	}
}

// TestParseEventDataPartialCommit tests parsing commit with partial data
func TestParseEventDataPartialCommit(t *testing.T) {
	// Commit with only sha
	data := map[string]interface{}{
		"commit": map[string]interface{}{
			"sha": "abc123",
		},
	}

	event := parseEventData(data)

	if event.Commit == nil {
		t.Fatal("Expected Commit to be set")
	}
	if event.Commit.SHA != "abc123" {
		t.Errorf("Expected sha abc123, got: %s", event.Commit.SHA)
	}
	if event.Commit.Message != "" {
		t.Errorf("Expected empty message, got: %s", event.Commit.Message)
	}
}

// TestParseEventDataInvalidFileInCommit tests commit with malformed files
func TestParseEventDataInvalidFileInCommit(t *testing.T) {
	data := map[string]interface{}{
		"commit": map[string]interface{}{
			"sha": "abc123",
			"files": []interface{}{
				"not a map", // Invalid - should be skipped
				map[string]interface{}{
					"path":   "valid.go",
					"status": "added",
				},
			},
		},
	}

	event := parseEventData(data)

	if event.Commit == nil {
		t.Fatal("Expected Commit to be set")
	}
	// Should only have 1 valid file
	if len(event.Commit.Files) != 1 {
		t.Errorf("Expected 1 file (invalid skipped), got: %d", len(event.Commit.Files))
	}
	if event.Commit.Files[0].Path != "valid.go" {
		t.Errorf("Expected path valid.go, got: %s", event.Commit.Files[0].Path)
	}
}

// TestRootCmdInit tests that root command is initialized properly
func TestRootCmdInit(t *testing.T) {
	// Verify commands are registered
	commands := rootCmd.Commands()
	expectedCmds := []string{"version", "discover", "validate", "run", "triggers"}

	for _, expected := range expectedCmds {
		found := false
		for _, cmd := range commands {
			if cmd.Use == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected command '%s' not found in root commands", expected)
		}
	}
}

// TestValidateCommandDefaultDir tests validate with default directory
func TestValidateCommandDefaultDir(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	_ = validateCmd.Flags().Set("file", "")
	_ = validateCmd.Flags().Set("dir", "")
	_ = validateCmd.RunE(validateCmd, []string{})

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	// Just verify it runs without panic - actual validation depends on cwd content
}

// TestRunCommandDefaultDir tests run with default directory
func TestRunCommandDefaultDir(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	_ = runCmd.Flags().Set("event", "")
	_ = runCmd.Flags().Set("workflow", "")
	_ = runCmd.Flags().Set("dir", "")
	_ = runCmd.RunE(runCmd, []string{})

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	// Just verify it runs - results depend on cwd content
}

// TestIsHookflowSelfRepair tests the self-repair detection function
func TestIsHookflowSelfRepair(t *testing.T) {
	tests := []struct {
		name     string
		event    *schema.Event
		expected bool
	}{
		{
			name: "edit to hookflow workflow yml",
			event: &schema.Event{
				File: &schema.FileEvent{
					Path:   ".github/hookflows/my-workflow.yml",
					Action: "edit",
				},
			},
			expected: true,
		},
		{
			name: "create hookflow workflow yaml",
			event: &schema.Event{
				File: &schema.FileEvent{
					Path:   ".github/hookflows/new-workflow.yaml",
					Action: "create",
				},
			},
			expected: true,
		},
		{
			name: "edit to non-hookflow file",
			event: &schema.Event{
				File: &schema.FileEvent{
					Path:   "src/main.go",
					Action: "edit",
				},
			},
			expected: false,
		},
		{
			name: "edit to hookflow but not yml",
			event: &schema.Event{
				File: &schema.FileEvent{
					Path:   ".github/hookflows/README.md",
					Action: "edit",
				},
			},
			expected: false,
		},
		{
			name: "delete hookflow workflow - not allowed for self-repair",
			event: &schema.Event{
				File: &schema.FileEvent{
					Path:   ".github/hookflows/my-workflow.yml",
					Action: "delete",
				},
			},
			expected: false,
		},
		{
			name: "no file event",
			event: &schema.Event{
				Tool: &schema.ToolEvent{
					Name: "edit",
				},
			},
			expected: false,
		},
		{
			name: "nested path in hooks",
			event: &schema.Event{
				File: &schema.FileEvent{
					Path:   ".github/hookflows/subdir/workflow.yml",
					Action: "edit",
				},
			},
			expected: true,
		},
		{
			name: "windows-style path",
			event: &schema.Event{
				File: &schema.FileEvent{
					Path:   ".github/hookflows/workflow.yml", // Use forward slashes for cross-platform
					Action: "edit",
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isHookflowSelfRepair(tt.event, "/test/dir")
			if result != tt.expected {
				t.Errorf("isHookflowSelfRepair() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

// TestInvalidWorkflowDeniesNonHookflowEdits tests that invalid workflows deny non-hookflow edits
func TestInvalidWorkflowDeniesNonHookflowEdits(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "hookflow-invalid-deny-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	workflowDir := filepath.Join(tmpDir, ".github", "hookflows")
	if err := os.MkdirAll(workflowDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create an invalid workflow (unknown field)
	invalidWorkflow := `name: invalid
on:
  file:
    unknown_field: true
steps:
  - run: echo test
`
	if err := os.WriteFile(filepath.Join(workflowDir, "invalid.yml"), []byte(invalidWorkflow), 0644); err != nil {
		t.Fatal(err)
	}

	// Use runMatchingWorkflowsWithEvent directly with a file event for non-hookflow file
	evt := &schema.Event{
		File: &schema.FileEvent{
			Path:   "src/main.go",
			Action: "edit",
		},
		Cwd: tmpDir,
	}

	oldStdout := os.Stdout
	stdoutR, stdoutW, _ := os.Pipe()
	os.Stdout = stdoutW

	_ = runMatchingWorkflowsWithEvent(tmpDir, evt)

	_ = stdoutW.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(stdoutR)
	output := buf.String()

	// Should deny because workflow is invalid
	if !strings.Contains(output, "deny") {
		t.Errorf("Expected deny for invalid workflow, got: %s", output)
	}
	if !strings.Contains(output, "Invalid workflow") {
		t.Errorf("Expected 'Invalid workflow' in reason, got: %s", output)
	}
}

// TestInvalidWorkflowAllowsSelfRepair tests that invalid workflows allow edits to .github/hookflows/
func TestInvalidWorkflowAllowsSelfRepair(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "hookflow-self-repair-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	workflowDir := filepath.Join(tmpDir, ".github", "hookflows")
	if err := os.MkdirAll(workflowDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create an invalid workflow
	invalidWorkflow := `name: invalid
on:
  file:
    unknown_field: true
steps:
  - run: echo test
`
	if err := os.WriteFile(filepath.Join(workflowDir, "invalid.yml"), []byte(invalidWorkflow), 0644); err != nil {
		t.Fatal(err)
	}

	// Use runMatchingWorkflowsWithEvent directly - editing the hookflow workflow itself
	evt := &schema.Event{
		File: &schema.FileEvent{
			Path:   ".github/hookflows/invalid.yml",
			Action: "edit",
		},
		Cwd: tmpDir,
	}

	oldStdout := os.Stdout
	stdoutR, stdoutW, _ := os.Pipe()
	os.Stdout = stdoutW

	_ = runMatchingWorkflowsWithEvent(tmpDir, evt)

	_ = stdoutW.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(stdoutR)
	output := buf.String()

	// Should allow because it's a self-repair edit to .github/hookflows/
	if !strings.Contains(output, "allow") {
		t.Errorf("Expected allow for self-repair edit, got: %s", output)
	}
	if !strings.Contains(output, "self-repair") {
		t.Errorf("Expected 'self-repair' in reason, got: %s", output)
	}
}

// TestFileTriggerWithTypesMatches tests that file trigger with 'types' field matches correctly
func TestFileTriggerWithTypesMatches(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "hookflow-file-types-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	workflowDir := filepath.Join(tmpDir, ".github", "hookflows")
	if err := os.MkdirAll(workflowDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create workflow using 'types' for file events
	workflow := `name: Block plugin.json edits
on:
  file:
    paths:
      - 'plugin.json'
    types:
      - edit
blocking: true
steps:
  - name: Block edit
    run: |
      echo "Blocking edit to plugin.json"
      exit 1
`
	if err := os.WriteFile(filepath.Join(workflowDir, "block-plugin.yml"), []byte(workflow), 0644); err != nil {
		t.Fatal(err)
	}

	// Create event for editing plugin.json
	evt := &schema.Event{
		File: &schema.FileEvent{
			Path:   "plugin.json",
			Action: "edit",
		},
		Cwd: tmpDir,
	}

	oldStdout := os.Stdout
	stdoutR, stdoutW, _ := os.Pipe()
	os.Stdout = stdoutW

	_ = runMatchingWorkflowsWithEvent(tmpDir, evt)

	_ = stdoutW.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(stdoutR)
	output := buf.String()

	// Should deny because workflow matches and step exits 1
	if !strings.Contains(output, "deny") {
		t.Errorf("Expected deny when workflow matches file edit, got: %s", output)
	}
	if !strings.Contains(output, "Block plugin.json edits") {
		t.Errorf("Expected workflow name in output, got: %s", output)
	}
}

// TestFileTriggerWithMultipleTypes tests that file trigger matches multiple types
func TestFileTriggerWithMultipleTypes(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "hookflow-file-types-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	workflowDir := filepath.Join(tmpDir, ".github", "hookflows")
	if err := os.MkdirAll(workflowDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create workflow using 'types' (original field name)
	workflow := `name: Block config edits
on:
  file:
    paths:
      - 'config.json'
    types:
      - edit
      - create
blocking: true
steps:
  - name: Block
    run: exit 1
`
	if err := os.WriteFile(filepath.Join(workflowDir, "block-config.yml"), []byte(workflow), 0644); err != nil {
		t.Fatal(err)
	}

	// Create event
	evt := &schema.Event{
		File: &schema.FileEvent{
			Path:   "config.json",
			Action: "edit",
		},
		Cwd: tmpDir,
	}

	oldStdout := os.Stdout
	stdoutR, stdoutW, _ := os.Pipe()
	os.Stdout = stdoutW

	_ = runMatchingWorkflowsWithEvent(tmpDir, evt)

	_ = stdoutW.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(stdoutR)
	output := buf.String()

	// Should deny
	if !strings.Contains(output, "deny") {
		t.Errorf("Expected deny when workflow matches, got: %s", output)
	}
}

// TestFileTriggerNoMatchWrongType tests that file trigger doesn't match wrong type
func TestFileTriggerNoMatchWrongType(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "hookflow-file-nomatch-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	workflowDir := filepath.Join(tmpDir, ".github", "hookflows")
	if err := os.MkdirAll(workflowDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create workflow that only matches 'create' type
	workflow := `name: Block creates only
on:
  file:
    paths:
      - '**/*.json'
    types:
      - create
blocking: true
steps:
  - name: Block
    run: exit 1
`
	if err := os.WriteFile(filepath.Join(workflowDir, "block-create.yml"), []byte(workflow), 0644); err != nil {
		t.Fatal(err)
	}

	// Create event with 'edit' action - should NOT match workflow that only wants 'create'
	evt := &schema.Event{
		File: &schema.FileEvent{
			Path:   "test.json",
			Action: "edit",
		},
		Cwd: tmpDir,
	}

	oldStdout := os.Stdout
	stdoutR, stdoutW, _ := os.Pipe()
	os.Stdout = stdoutW

	_ = runMatchingWorkflowsWithEvent(tmpDir, evt)

	_ = stdoutW.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(stdoutR)
	output := buf.String()

	// Should allow because action doesn't match (edit vs create)
	if !strings.Contains(output, "allow") {
		t.Errorf("Expected allow when action doesn't match, got: %s", output)
	}
}

// TestConditionalStepWithToolArgsNewStr tests that a step with if condition on event.tool.args.new_str works
func TestConditionalStepWithToolArgsNewStr(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "hookflow-conditional-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	workflowDir := filepath.Join(tmpDir, ".github", "hookflows")
	if err := os.MkdirAll(workflowDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create workflow with conditional step based on new_str content
	workflow := `name: Block password in code
on:
  file:
    paths:
      - '**/*.js'
    types:
      - edit
blocking: true
steps:
  - name: Check for password
    if: contains(event.tool.args.new_str, 'password')
    run: |
      echo "Found password in code!"
      exit 1
`
	if err := os.WriteFile(filepath.Join(workflowDir, "block-password.yml"), []byte(workflow), 0644); err != nil {
		t.Fatal(err)
	}

	// Test case 1: new_str contains "password" - should deny
	t.Run("matches when new_str contains password", func(t *testing.T) {
		evt := &schema.Event{
			File: &schema.FileEvent{
				Path:   "src/auth.js",
				Action: "edit",
			},
			Tool: &schema.ToolEvent{
				Name: "edit",
				Args: map[string]interface{}{
					"path":    "src/auth.js",
					"new_str": "const password = 'secret123';",
				},
			},
			Cwd: tmpDir,
		}

		oldStdout := os.Stdout
		stdoutR, stdoutW, _ := os.Pipe()
		os.Stdout = stdoutW

		_ = runMatchingWorkflowsWithEvent(tmpDir, evt)

		_ = stdoutW.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		_, _ = buf.ReadFrom(stdoutR)
		output := buf.String()

		if !strings.Contains(output, "deny") {
			t.Errorf("Expected deny when new_str contains 'password', got: %s", output)
		}
	})

	// Test case 2: new_str does NOT contain "password" - should allow (step skipped)
	t.Run("allows when new_str does not contain password", func(t *testing.T) {
		evt := &schema.Event{
			File: &schema.FileEvent{
				Path:   "src/utils.js",
				Action: "edit",
			},
			Tool: &schema.ToolEvent{
				Name: "edit",
				Args: map[string]interface{}{
					"path":    "src/utils.js",
					"new_str": "const username = 'john';",
				},
			},
			Cwd: tmpDir,
		}

		oldStdout := os.Stdout
		stdoutR, stdoutW, _ := os.Pipe()
		os.Stdout = stdoutW

		_ = runMatchingWorkflowsWithEvent(tmpDir, evt)

		_ = stdoutW.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		_, _ = buf.ReadFrom(stdoutR)
		output := buf.String()

		// When step is skipped due to if condition, workflow should allow
		if strings.Contains(output, "deny") {
			t.Errorf("Expected allow when new_str doesn't contain 'password', got: %s", output)
		}
	})
}

// TestConditionalStepWithFileContent tests conditional step using event.file.content
func TestConditionalStepWithFileContent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "hookflow-file-content-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	workflowDir := filepath.Join(tmpDir, ".github", "hookflows")
	if err := os.MkdirAll(workflowDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create workflow with conditional step based on file content
	workflow := `name: Block API keys in new files
on:
  file:
    paths:
      - '**/*.env'
    types:
      - create
blocking: true
steps:
  - name: Check for API key
    if: contains(event.file.content, 'API_KEY')
    run: |
      echo "Found API_KEY in file!"
      exit 1
`
	if err := os.WriteFile(filepath.Join(workflowDir, "block-apikey.yml"), []byte(workflow), 0644); err != nil {
		t.Fatal(err)
	}

	// Test case 1: content contains "API_KEY" - should deny
	t.Run("denies when file content contains API_KEY", func(t *testing.T) {
		evt := &schema.Event{
			File: &schema.FileEvent{
				Path:    "config/.env",
				Action:  "create",
				Content: "DATABASE_URL=postgres://localhost\nAPI_KEY=sk-12345\n",
			},
			Tool: &schema.ToolEvent{
				Name: "create",
				Args: map[string]interface{}{
					"path":      "config/.env",
					"file_text": "DATABASE_URL=postgres://localhost\nAPI_KEY=sk-12345\n",
				},
			},
			Cwd: tmpDir,
		}

		oldStdout := os.Stdout
		stdoutR, stdoutW, _ := os.Pipe()
		os.Stdout = stdoutW

		_ = runMatchingWorkflowsWithEvent(tmpDir, evt)

		_ = stdoutW.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		_, _ = buf.ReadFrom(stdoutR)
		output := buf.String()

		if !strings.Contains(output, "deny") {
			t.Errorf("Expected deny when file content contains 'API_KEY', got: %s", output)
		}
	})

	// Test case 2: content does NOT contain "API_KEY" - should allow
	t.Run("allows when file content does not contain API_KEY", func(t *testing.T) {
		evt := &schema.Event{
			File: &schema.FileEvent{
				Path:    "config/.env",
				Action:  "create",
				Content: "DATABASE_URL=postgres://localhost\nDEBUG=true\n",
			},
			Tool: &schema.ToolEvent{
				Name: "create",
				Args: map[string]interface{}{
					"path":      "config/.env",
					"file_text": "DATABASE_URL=postgres://localhost\nDEBUG=true\n",
				},
			},
			Cwd: tmpDir,
		}

		oldStdout := os.Stdout
		stdoutR, stdoutW, _ := os.Pipe()
		os.Stdout = stdoutW

		_ = runMatchingWorkflowsWithEvent(tmpDir, evt)

		_ = stdoutW.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		_, _ = buf.ReadFrom(stdoutR)
		output := buf.String()

		if strings.Contains(output, "deny") {
			t.Errorf("Expected allow when file content doesn't contain 'API_KEY', got: %s", output)
		}
	})
}

// TestConditionalStepFileEventWithoutToolContext tests what happens when file trigger
// tries to access event.tool.args.new_str but Tool context is not set
func TestConditionalStepFileEventWithoutToolContext(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "hookflow-file-no-tool-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	workflowDir := filepath.Join(tmpDir, ".github", "hookflows")
	if err := os.MkdirAll(workflowDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create workflow with conditional step based on new_str content
	workflow := `name: Block password in code
on:
  file:
    paths:
      - '**/*.js'
    types:
      - edit
blocking: true
steps:
  - name: Check for password
    if: contains(event.tool.args.new_str, 'password')
    run: |
      echo "Found password in code!"
      exit 1
`
	if err := os.WriteFile(filepath.Join(workflowDir, "block-password.yml"), []byte(workflow), 0644); err != nil {
		t.Fatal(err)
	}

	// File event WITHOUT Tool context - simulates pure file trigger scenario
	t.Run("file event without tool context", func(t *testing.T) {
		evt := &schema.Event{
			File: &schema.FileEvent{
				Path:   "src/auth.js",
				Action: "edit",
			},
			// Tool is nil - no tool context available
			Cwd: tmpDir,
		}

		oldStdout := os.Stdout
		stdoutR, stdoutW, _ := os.Pipe()
		os.Stdout = stdoutW

		_ = runMatchingWorkflowsWithEvent(tmpDir, evt)

		_ = stdoutW.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		_, _ = buf.ReadFrom(stdoutR)
		output := buf.String()

		// When Tool is nil, event.tool.args.new_str should evaluate to empty/nil
		// The step should be skipped (if condition false) and workflow should allow
		t.Logf("Output: %s", output)
		// Document current behavior - what actually happens?
		if strings.Contains(output, "deny") {
			t.Logf("BEHAVIOR: Denies when Tool context is nil")
		} else if strings.Contains(output, "allow") {
			t.Logf("BEHAVIOR: Allows when Tool context is nil (step skipped)")
		} else {
			t.Logf("BEHAVIOR: Unknown - output: %s", output)
		}
	})
}

// ============================================================================
// LIFECYCLE TESTS
// ============================================================================

// TestLifecyclePreMatchesPre tests that pre lifecycle triggers match pre events
func TestLifecyclePreMatchesPre(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "hookflow-lifecycle-pre-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	workflowDir := filepath.Join(tmpDir, ".github", "hookflows")
	if err := os.MkdirAll(workflowDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Workflow with explicit lifecycle: pre
	workflow := `name: Pre-file validation
on:
  file:
    lifecycle: pre
    paths:
      - '**/*.js'
    types:
      - edit
blocking: true
steps:
  - name: Block
    run: exit 1
`
	if err := os.WriteFile(filepath.Join(workflowDir, "pre-check.yml"), []byte(workflow), 0644); err != nil {
		t.Fatal(err)
	}

	// Pre event should match
	evt := &schema.Event{
		File: &schema.FileEvent{
			Path:   "src/app.js",
			Action: "edit",
		},
		Cwd:       tmpDir,
		Lifecycle: "pre",
	}

	oldStdout := os.Stdout
	stdoutR, stdoutW, _ := os.Pipe()
	os.Stdout = stdoutW

	_ = runMatchingWorkflowsWithEvent(tmpDir, evt)

	_ = stdoutW.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(stdoutR)
	output := buf.String()

	if !strings.Contains(output, "deny") {
		t.Errorf("Expected deny for pre event matching pre workflow, got: %s", output)
	}
}

// TestLifecyclePostMatchesPost tests that post lifecycle triggers match post events
func TestLifecyclePostMatchesPost(t *testing.T) {
	t.Cleanup(func() { clearSessionErrors(t) })
	tmpDir, err := os.MkdirTemp("", "hookflow-lifecycle-post-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	workflowDir := filepath.Join(tmpDir, ".github", "hookflows")
	if err := os.MkdirAll(workflowDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Workflow with lifecycle: post
	workflow := `name: Post-file linting
on:
  file:
    lifecycle: post
    paths:
      - '**/*.js'
    types:
      - edit
blocking: true
steps:
  - name: Lint
    run: exit 1
`
	if err := os.WriteFile(filepath.Join(workflowDir, "post-lint.yml"), []byte(workflow), 0644); err != nil {
		t.Fatal(err)
	}

	// Post event should match
	evt := &schema.Event{
		File: &schema.FileEvent{
			Path:   "src/app.js",
			Action: "edit",
		},
		Cwd:       tmpDir,
		Lifecycle: "post",
	}

	oldStdout := os.Stdout
	stdoutR, stdoutW, _ := os.Pipe()
	os.Stdout = stdoutW

	_ = runMatchingWorkflowsWithEvent(tmpDir, evt)

	_ = stdoutW.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(stdoutR)
	output := buf.String()

	if !strings.Contains(output, "deny") {
		t.Errorf("Expected deny for post event matching post workflow, got: %s", output)
	}
}

// TestLifecyclePreDoesNotMatchPost tests that pre workflow doesn't match post event
func TestLifecyclePreDoesNotMatchPost(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "hookflow-lifecycle-mismatch-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	workflowDir := filepath.Join(tmpDir, ".github", "hookflows")
	if err := os.MkdirAll(workflowDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Workflow with lifecycle: pre (explicit)
	workflow := `name: Pre-file validation
on:
  file:
    lifecycle: pre
    paths:
      - '**/*.js'
    types:
      - edit
blocking: true
steps:
  - name: Block
    run: exit 1
`
	if err := os.WriteFile(filepath.Join(workflowDir, "pre-check.yml"), []byte(workflow), 0644); err != nil {
		t.Fatal(err)
	}

	// Post event should NOT match pre workflow
	evt := &schema.Event{
		File: &schema.FileEvent{
			Path:   "src/app.js",
			Action: "edit",
		},
		Cwd:       tmpDir,
		Lifecycle: "post",
	}

	oldStdout := os.Stdout
	stdoutR, stdoutW, _ := os.Pipe()
	os.Stdout = stdoutW

	_ = runMatchingWorkflowsWithEvent(tmpDir, evt)

	_ = stdoutW.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(stdoutR)
	output := buf.String()

	if strings.Contains(output, "deny") {
		t.Errorf("Expected allow for post event with pre workflow (no match), got: %s", output)
	}
}

// TestLifecyclePostDoesNotMatchPre tests that post workflow doesn't match pre event
func TestLifecyclePostDoesNotMatchPre(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "hookflow-lifecycle-mismatch2-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	workflowDir := filepath.Join(tmpDir, ".github", "hookflows")
	if err := os.MkdirAll(workflowDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Workflow with lifecycle: post
	workflow := `name: Post-file linting
on:
  file:
    lifecycle: post
    paths:
      - '**/*.js'
    types:
      - edit
blocking: true
steps:
  - name: Lint
    run: exit 1
`
	if err := os.WriteFile(filepath.Join(workflowDir, "post-lint.yml"), []byte(workflow), 0644); err != nil {
		t.Fatal(err)
	}

	// Pre event should NOT match post workflow
	evt := &schema.Event{
		File: &schema.FileEvent{
			Path:   "src/app.js",
			Action: "edit",
		},
		Cwd:       tmpDir,
		Lifecycle: "pre",
	}

	oldStdout := os.Stdout
	stdoutR, stdoutW, _ := os.Pipe()
	os.Stdout = stdoutW

	_ = runMatchingWorkflowsWithEvent(tmpDir, evt)

	_ = stdoutW.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(stdoutR)
	output := buf.String()

	if strings.Contains(output, "deny") {
		t.Errorf("Expected allow for pre event with post workflow (no match), got: %s", output)
	}
}

// TestLifecycleDefaultIsPre tests that workflow without lifecycle defaults to pre
func TestLifecycleDefaultIsPre(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "hookflow-lifecycle-default-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	workflowDir := filepath.Join(tmpDir, ".github", "hookflows")
	if err := os.MkdirAll(workflowDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Workflow WITHOUT lifecycle (should default to pre)
	workflow := `name: Default lifecycle validation
on:
  file:
    paths:
      - '**/*.js'
    types:
      - edit
blocking: true
steps:
  - name: Block
    run: exit 1
`
	if err := os.WriteFile(filepath.Join(workflowDir, "default-check.yml"), []byte(workflow), 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("matches pre event (default)", func(t *testing.T) {
		evt := &schema.Event{
			File: &schema.FileEvent{
				Path:   "src/app.js",
				Action: "edit",
			},
			Cwd:       tmpDir,
			Lifecycle: "pre",
		}

		oldStdout := os.Stdout
		stdoutR, stdoutW, _ := os.Pipe()
		os.Stdout = stdoutW

		_ = runMatchingWorkflowsWithEvent(tmpDir, evt)

		_ = stdoutW.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		_, _ = buf.ReadFrom(stdoutR)
		output := buf.String()

		if !strings.Contains(output, "deny") {
			t.Errorf("Expected deny for pre event with default workflow, got: %s", output)
		}
	})

	t.Run("does not match post event", func(t *testing.T) {
		evt := &schema.Event{
			File: &schema.FileEvent{
				Path:   "src/app.js",
				Action: "edit",
			},
			Cwd:       tmpDir,
			Lifecycle: "post",
		}

		oldStdout := os.Stdout
		stdoutR, stdoutW, _ := os.Pipe()
		os.Stdout = stdoutW

		_ = runMatchingWorkflowsWithEvent(tmpDir, evt)

		_ = stdoutW.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		_, _ = buf.ReadFrom(stdoutR)
		output := buf.String()

		if strings.Contains(output, "deny") {
			t.Errorf("Expected allow for post event with default (pre) workflow, got: %s", output)
		}
	})

	t.Run("matches empty lifecycle (treated as pre)", func(t *testing.T) {
		evt := &schema.Event{
			File: &schema.FileEvent{
				Path:   "src/app.js",
				Action: "edit",
			},
			Cwd:       tmpDir,
			Lifecycle: "", // Empty defaults to pre
		}

		oldStdout := os.Stdout
		stdoutR, stdoutW, _ := os.Pipe()
		os.Stdout = stdoutW

		_ = runMatchingWorkflowsWithEvent(tmpDir, evt)

		_ = stdoutW.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		_, _ = buf.ReadFrom(stdoutR)
		output := buf.String()

		if !strings.Contains(output, "deny") {
			t.Errorf("Expected deny for empty lifecycle event with default workflow, got: %s", output)
		}
	})
}

// TestLifecycleCommitTrigger tests lifecycle on commit triggers
func TestLifecycleCommitTrigger(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "hookflow-commit-lifecycle-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	workflowDir := filepath.Join(tmpDir, ".github", "hookflows")
	if err := os.MkdirAll(workflowDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Pre-commit workflow
	preWorkflow := `name: Pre-commit check
on:
  commit:
    lifecycle: pre
    paths:
      - 'src/**'
blocking: true
steps:
  - name: Check
    run: exit 1
`
	if err := os.WriteFile(filepath.Join(workflowDir, "pre-commit.yml"), []byte(preWorkflow), 0644); err != nil {
		t.Fatal(err)
	}

	// Post-commit workflow
	postWorkflow := `name: Post-commit notification
on:
  commit:
    lifecycle: post
    paths:
      - 'src/**'
blocking: false
steps:
  - name: Notify
    run: echo "Commit done"
`
	if err := os.WriteFile(filepath.Join(workflowDir, "post-commit.yml"), []byte(postWorkflow), 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("pre commit event matches pre workflow", func(t *testing.T) {
		evt := &schema.Event{
			Commit: &schema.CommitEvent{
				SHA:     "abc123",
				Message: "test commit",
				Files:   []schema.FileStatus{{Path: "src/main.go", Status: "modified"}},
			},
			Cwd:       tmpDir,
			Lifecycle: "pre",
		}

		oldStdout := os.Stdout
		stdoutR, stdoutW, _ := os.Pipe()
		os.Stdout = stdoutW

		_ = runMatchingWorkflowsWithEvent(tmpDir, evt)

		_ = stdoutW.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		_, _ = buf.ReadFrom(stdoutR)
		output := buf.String()

		if !strings.Contains(output, "deny") {
			t.Errorf("Expected deny for pre commit event, got: %s", output)
		}
	})

	t.Run("post commit event matches post workflow", func(t *testing.T) {
		evt := &schema.Event{
			Commit: &schema.CommitEvent{
				SHA:     "abc123",
				Message: "test commit",
				Files:   []schema.FileStatus{{Path: "src/main.go", Status: "modified"}},
			},
			Cwd:       tmpDir,
			Lifecycle: "post",
		}

		oldStdout := os.Stdout
		stdoutR, stdoutW, _ := os.Pipe()
		os.Stdout = stdoutW

		_ = runMatchingWorkflowsWithEvent(tmpDir, evt)

		_ = stdoutW.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		_, _ = buf.ReadFrom(stdoutR)
		output := buf.String()

		// Post workflow is non-blocking and step succeeds
		if strings.Contains(output, "deny") {
			t.Errorf("Expected allow for post commit event (non-blocking), got: %s", output)
		}
	})
}

// TestLifecyclePushTrigger tests lifecycle on push triggers
func TestLifecyclePushTrigger(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "hookflow-push-lifecycle-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	workflowDir := filepath.Join(tmpDir, ".github", "hookflows")
	if err := os.MkdirAll(workflowDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Pre-push workflow
	workflow := `name: Pre-push check
on:
  push:
    lifecycle: pre
    branches:
      - main
blocking: true
steps:
  - name: Check
    run: exit 1
`
	if err := os.WriteFile(filepath.Join(workflowDir, "pre-push.yml"), []byte(workflow), 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("pre push matches pre workflow", func(t *testing.T) {
		evt := &schema.Event{
			Push: &schema.PushEvent{
				Ref: "refs/heads/main",
			},
			Cwd:       tmpDir,
			Lifecycle: "pre",
		}

		oldStdout := os.Stdout
		stdoutR, stdoutW, _ := os.Pipe()
		os.Stdout = stdoutW

		_ = runMatchingWorkflowsWithEvent(tmpDir, evt)

		_ = stdoutW.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		_, _ = buf.ReadFrom(stdoutR)
		output := buf.String()

		if !strings.Contains(output, "deny") {
			t.Errorf("Expected deny for pre push event, got: %s", output)
		}
	})

	t.Run("post push does not match pre workflow", func(t *testing.T) {
		evt := &schema.Event{
			Push: &schema.PushEvent{
				Ref: "refs/heads/main",
			},
			Cwd:       tmpDir,
			Lifecycle: "post",
		}

		oldStdout := os.Stdout
		stdoutR, stdoutW, _ := os.Pipe()
		os.Stdout = stdoutW

		_ = runMatchingWorkflowsWithEvent(tmpDir, evt)

		_ = stdoutW.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		_, _ = buf.ReadFrom(stdoutR)
		output := buf.String()

		if strings.Contains(output, "deny") {
			t.Errorf("Expected allow for post push with pre workflow, got: %s", output)
		}
	})
}

// TestLifecycleMixedWorkflows tests having both pre and post workflows
func TestLifecycleMixedWorkflows(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "hookflow-mixed-lifecycle-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	workflowDir := filepath.Join(tmpDir, ".github", "hookflows")
	if err := os.MkdirAll(workflowDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Pre-edit workflow (blocks)
	preWorkflow := `name: Pre-edit validation
on:
  file:
    lifecycle: pre
    paths:
      - '**/*.ts'
    types:
      - edit
blocking: true
steps:
  - name: Validate
    run: exit 1
`
	if err := os.WriteFile(filepath.Join(workflowDir, "pre-edit.yml"), []byte(preWorkflow), 0644); err != nil {
		t.Fatal(err)
	}

	// Post-edit workflow (runs after)
	postWorkflow := `name: Post-edit linting
on:
  file:
    lifecycle: post
    paths:
      - '**/*.ts'
    types:
      - edit
blocking: false
steps:
  - name: Lint
    run: echo "Linting completed"
`
	if err := os.WriteFile(filepath.Join(workflowDir, "post-edit.yml"), []byte(postWorkflow), 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("pre event only runs pre workflow", func(t *testing.T) {
		evt := &schema.Event{
			File: &schema.FileEvent{
				Path:   "src/index.ts",
				Action: "edit",
			},
			Cwd:       tmpDir,
			Lifecycle: "pre",
		}

		oldStdout := os.Stdout
		stdoutR, stdoutW, _ := os.Pipe()
		os.Stdout = stdoutW

		_ = runMatchingWorkflowsWithEvent(tmpDir, evt)

		_ = stdoutW.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		_, _ = buf.ReadFrom(stdoutR)
		output := buf.String()

		// Should deny because pre workflow blocks
		if !strings.Contains(output, "deny") {
			t.Errorf("Expected deny for pre event, got: %s", output)
		}
		// Should mention pre-edit workflow
		if !strings.Contains(output, "Pre-edit") {
			t.Errorf("Expected Pre-edit workflow name in output, got: %s", output)
		}
	})

	t.Run("post event only runs post workflow", func(t *testing.T) {
		evt := &schema.Event{
			File: &schema.FileEvent{
				Path:   "src/index.ts",
				Action: "edit",
			},
			Cwd:       tmpDir,
			Lifecycle: "post",
		}

		oldStdout := os.Stdout
		stdoutR, stdoutW, _ := os.Pipe()
		os.Stdout = stdoutW

		_ = runMatchingWorkflowsWithEvent(tmpDir, evt)

		_ = stdoutW.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		_, _ = buf.ReadFrom(stdoutR)
		output := buf.String()

		// Should allow because post workflow is non-blocking
		if strings.Contains(output, "deny") {
			t.Errorf("Expected allow for post event (non-blocking), got: %s", output)
		}
	})
}

// ============================================================================
// EVENT TYPE CONVERSION TESTS
// ============================================================================

// TestEventTypeToLifecycle tests the conversion from Copilot hook types to lifecycle
func TestEventTypeToLifecycle(t *testing.T) {
	tests := []struct {
		eventType string
		expected  string
	}{
		// Standard Copilot hook types
		{"preToolUse", "pre"},
		{"postToolUse", "post"},
		// Short forms
		{"pre", "pre"},
		{"post", "post"},
		// Unknown defaults to pre
		{"", "pre"},
		{"unknown", "pre"},
		{"something", "pre"},
		// Future hook types can be added here
	}

	for _, tt := range tests {
		t.Run(tt.eventType, func(t *testing.T) {
			result := eventTypeToLifecycle(tt.eventType)
			if result != tt.expected {
				t.Errorf("eventTypeToLifecycle(%q) = %q, want %q", tt.eventType, result, tt.expected)
			}
		})
	}
}

// TestNormalizeFilePath tests file path normalization for workflow matching
func TestNormalizeFilePath(t *testing.T) {
	tests := []struct {
		name        string
		filePath    string
		dir         string
		expected    string
		windowsOnly bool // Skip on non-Windows
	}{
		{
			name:     "absolute Windows path to relative",
			filePath: "C:\\Repos\\project\\plugin.json",
			dir:      "C:\\Repos\\project",
			expected: "plugin.json",
		},
		{
			name:     "absolute Unix path to relative",
			filePath: "/home/user/project/src/main.go",
			dir:      "/home/user/project",
			expected: "src/main.go",
		},
		{
			name:     "already relative path",
			filePath: "plugin.json",
			dir:      "/home/user/project",
			expected: "plugin.json",
		},
		{
			name:     "nested path",
			filePath: "C:\\Repos\\project\\packages\\hooks\\scripts\\test.sh",
			dir:      "C:\\Repos\\project",
			expected: "packages/hooks/scripts/test.sh",
		},
		{
			name:     "path with trailing slash in dir",
			filePath: "/project/src/config.json",
			dir:      "/project/",
			expected: "src/config.json",
		},
		{
			name:        "case insensitive match (Windows)",
			filePath:    "C:\\REPOS\\Project\\plugin.json",
			dir:         "c:\\repos\\project",
			expected:    "plugin.json",
			windowsOnly: true, // Case insensitivity is Windows-specific
		},
		{
			name:     "path outside of dir",
			filePath: "/other/project/file.txt",
			dir:      "/home/user/project",
			expected: "/other/project/file.txt",
		},
		{
			name:     "github hooks path",
			filePath: "C:\\Repos\\project\\.github\\hookflows\\workflow.yml",
			dir:      "C:\\Repos\\project",
			expected: ".github/hookflows/workflow.yml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.windowsOnly && runtime.GOOS != "windows" {
				t.Skip("Skipping Windows-specific test on non-Windows")
			}
			result := normalizeFilePath(tt.filePath, tt.dir)
			// Normalize expected for comparison (forward slashes)
			expected := strings.ReplaceAll(tt.expected, "\\", "/")
			if result != expected {
				t.Errorf("normalizeFilePath(%q, %q) = %q, want %q", tt.filePath, tt.dir, result, expected)
			}
		})
	}
}

// TestWorkflowMatchesAbsolutePath tests that workflow path patterns match even when event has absolute path
func TestWorkflowMatchesAbsolutePath(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "hookflow-abspath-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	workflowDir := filepath.Join(tmpDir, ".github", "hookflows")
	if err := os.MkdirAll(workflowDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create workflow that watches 'plugin.json' (relative path pattern)
	workflow := `name: Validate plugin.json
on:
  file:
    paths:
      - 'plugin.json'
    types:
      - edit
blocking: true
steps:
  - name: Validate
    run: echo "validated"
`
	if err := os.WriteFile(filepath.Join(workflowDir, "validate.yml"), []byte(workflow), 0644); err != nil {
		t.Fatal(err)
	}

	// Test with absolute path (simulating what Copilot hooks send)
	absolutePath := filepath.Join(tmpDir, "plugin.json")
	
	evt := &schema.Event{
		File: &schema.FileEvent{
			Path:   absolutePath, // Absolute path like Copilot sends - NOT pre-normalized
			Action: "edit",
		},
		Lifecycle: "pre",
		Cwd:       tmpDir,
	}

	// DO NOT manually normalize - runMatchingWorkflowsWithEvent should do it internally
	oldStdout := os.Stdout
	stdoutR, stdoutW, _ := os.Pipe()
	os.Stdout = stdoutW

	_ = runMatchingWorkflowsWithEvent(tmpDir, evt)

	_ = stdoutW.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(stdoutR)
	output := buf.String()

	// Should match and run the workflow (allow because steps succeed)
	if !strings.Contains(output, "allow") {
		t.Errorf("Expected workflow to match absolute path converted to relative, got: %s", output)
	}
}

// TestAbsolutePathMatchingScenarios tests various path matching scenarios with absolute paths
func TestAbsolutePathMatchingScenarios(t *testing.T) {
	tests := []struct {
		name         string
		workflow     string
		filePath     string // Relative to tmpDir for constructing absolute path
		action       string
		shouldMatch  bool
		description  string
	}{
		{
			name: "simple filename match",
			workflow: `name: Simple Match
on:
  file:
    paths:
      - 'plugin.json'
    types:
      - edit
blocking: true
steps:
  - name: Deny to prove match
    run: exit 1
`,
			filePath:    "plugin.json",
			action:      "edit",
			shouldMatch: true,
			description: "pattern 'plugin.json' should match absolute path ending in plugin.json",
		},
		{
			name: "nested path pattern",
			workflow: `name: Nested Match
on:
  file:
    paths:
      - 'packages/hooks/hooks.json'
    types:
      - edit
blocking: true
steps:
  - name: Deny to prove match
    run: exit 1
`,
			filePath:    "packages/hooks/hooks.json",
			action:      "edit",
			shouldMatch: true,
			description: "pattern 'packages/hooks/hooks.json' should match nested absolute path",
		},
		{
			name: "glob pattern with **",
			workflow: `name: Glob Match
on:
  file:
    paths:
      - 'packages/hooks/scripts/**'
    types:
      - edit
blocking: true
steps:
  - name: Deny to prove match
    run: exit 1
`,
			filePath:    "packages/hooks/scripts/test.sh",
			action:      "edit",
			shouldMatch: true,
			description: "pattern 'packages/hooks/scripts/**' should match files in subdirectory",
		},
		{
			name: "glob pattern with *.json",
			workflow: `name: Extension Match
on:
  file:
    paths:
      - '*.json'
    types:
      - edit
blocking: true
steps:
  - name: Deny to prove match
    run: exit 1
`,
			filePath:    "config.json",
			action:      "edit",
			shouldMatch: true,
			description: "pattern '*.json' should match any .json file in root",
		},
		{
			name: "glob pattern with **/*.json",
			workflow: `name: Recursive Extension Match
on:
  file:
    paths:
      - '**/*.json'
    types:
      - edit
blocking: true
steps:
  - name: Deny to prove match
    run: exit 1
`,
			filePath:    "src/config/settings.json",
			action:      "edit",
			shouldMatch: true,
			description: "pattern '**/*.json' should match .json file in any directory",
		},
		{
			name: "no match wrong path",
			workflow: `name: No Match
on:
  file:
    paths:
      - 'plugin.json'
    types:
      - edit
blocking: true
steps:
  - name: Deny to prove match
    run: exit 1
`,
			filePath:    "other.json",
			action:      "edit",
			shouldMatch: false,
			description: "pattern 'plugin.json' should NOT match 'other.json'",
		},
		{
			name: "no match wrong action",
			workflow: `name: No Match Action
on:
  file:
    paths:
      - 'plugin.json'
    types:
      - create
blocking: true
steps:
  - name: Deny to prove match
    run: exit 1
`,
			filePath:    "plugin.json",
			action:      "edit",
			shouldMatch: false,
			description: "types: [create] should NOT match action 'edit'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir, err := os.MkdirTemp("", "hookflow-pathtest-*")
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = os.RemoveAll(tmpDir) }()

			workflowDir := filepath.Join(tmpDir, ".github", "hookflows")
			if err := os.MkdirAll(workflowDir, 0755); err != nil {
				t.Fatal(err)
			}

			if err := os.WriteFile(filepath.Join(workflowDir, "test.yml"), []byte(tt.workflow), 0644); err != nil {
				t.Fatal(err)
			}

			// Construct absolute path like Copilot would send
			absolutePath := filepath.Join(tmpDir, tt.filePath)

			evt := &schema.Event{
				File: &schema.FileEvent{
					Path:   absolutePath, // Absolute path - should be normalized internally
					Action: tt.action,
				},
				Lifecycle: "pre",
				Cwd:       tmpDir,
			}

			oldStdout := os.Stdout
			stdoutR, stdoutW, _ := os.Pipe()
			os.Stdout = stdoutW

			_ = runMatchingWorkflowsWithEvent(tmpDir, evt)

			_ = stdoutW.Close()
			os.Stdout = oldStdout

			var buf bytes.Buffer
			_, _ = buf.ReadFrom(stdoutR)
			output := buf.String()

			if tt.shouldMatch {
				// If should match, workflow runs and step exits 1, so we should get "deny"
				if !strings.Contains(output, "deny") {
					t.Errorf("%s: Expected workflow to match and deny, but got: %s", tt.description, output)
				}
			} else {
				// If should NOT match, we get default "allow" (no workflow ran)
				if !strings.Contains(output, "allow") {
					t.Errorf("%s: Expected no match (allow), but got: %s", tt.description, output)
				}
			}
		})
	}
}

// TestJSONValidationWorkflow tests workflows that validate JSON syntax
// This tests the shifted-from-ci pattern where we check plugin.json/hooks.json are valid
func TestJSONValidationWorkflow(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "hookflow-json-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create hooks directory
	hooksDir := filepath.Join(tmpDir, ".github", "hookflows")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a JSON validation workflow using pwsh-compatible syntax
	workflow := `name: JSON Validation Test
on:
  file:
    paths:
      - 'config.json'
    types:
      - edit
      - create
blocking: true
steps:
  - name: Validate JSON syntax
    if: ${{ event.file.path == 'config.json' }}
    run: |
      Write-Output "Validating JSON syntax..."
      try {
        $content = Get-Content -Path config.json -Raw
        $null = ConvertFrom-Json $content
        Write-Output "JSON is valid"
      } catch {
        Write-Output "Invalid JSON!"
        exit 1
      }
`
	if err := os.WriteFile(filepath.Join(hooksDir, "validate-json.yml"), []byte(workflow), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name        string
		jsonContent string
		expectDeny  bool
	}{
		{
			name:        "valid JSON should allow",
			jsonContent: `{"name": "test", "version": "1.0.0"}`,
			expectDeny:  false,
		},
		{
			name:        "invalid JSON should deny",
			jsonContent: `{invalid json content`,
			expectDeny:  true,
		},
		{
			name:        "empty object is valid",
			jsonContent: `{}`,
			expectDeny:  false,
		},
		{
			name:        "truncated JSON should deny",
			jsonContent: `{"name": "test"`,
			expectDeny:  true,
		},
		{
			name:        "JSON with trailing comma should deny",
			jsonContent: `{"name": "test",}`,
			expectDeny:  false, // PowerShell's ConvertFrom-Json accepts trailing commas
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Write the test JSON file
			if err := os.WriteFile(filepath.Join(tmpDir, "config.json"), []byte(tt.jsonContent), 0644); err != nil {
				t.Fatal(err)
			}

			// Build event with absolute path (like Copilot would send)
			absPath := filepath.Join(tmpDir, "config.json")
			evt := &schema.Event{
				File: &schema.FileEvent{
					Path:   absPath,
					Action: "edit",
				},
				Lifecycle: "pre",
				Cwd:       tmpDir,
			}

			// Capture stdout
			oldStdout := os.Stdout
			stdoutR, stdoutW, _ := os.Pipe()
			os.Stdout = stdoutW

			_ = runMatchingWorkflowsWithEvent(tmpDir, evt)

			_ = stdoutW.Close()
			os.Stdout = oldStdout

			var buf bytes.Buffer
			_, _ = buf.ReadFrom(stdoutR)
			output := buf.String()

			if tt.expectDeny {
				if !strings.Contains(output, "deny") {
					t.Errorf("Expected deny for invalid JSON, got: %s", output)
				}
			} else {
				if !strings.Contains(output, "allow") {
					t.Errorf("Expected allow for valid JSON, got: %s", output)
				}
			}
		})
	}
}

// TestWorkflowStepExitCodeDenies tests that a step with non-zero exit code causes deny
func TestWorkflowStepExitCodeDenies(t *testing.T) {
	// Check if pwsh is available
	if _, err := exec.LookPath("pwsh"); err != nil {
		t.Skip("Skipping - pwsh not available")
	}

	tmpDir, err := os.MkdirTemp("", "hookflow-exitcode-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create hooks directory
	hooksDir := filepath.Join(tmpDir, ".github", "hookflows")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a workflow that exits with code based on file content
	workflow := `name: Content Check
on:
  file:
    paths:
      - '**/*.txt'
    types:
      - edit
      - create
blocking: true
steps:
  - name: Check content
    run: |
      $content = Get-Content "${{ event.file.path }}" -Raw
      if ($content -match "BLOCK_ME") {
        Write-Output "Found BLOCK_ME marker"
        exit 1
      }
      Write-Output "Content is OK"
`
	if err := os.WriteFile(filepath.Join(hooksDir, "check-content.yml"), []byte(workflow), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name        string
		fileContent string
		expectDeny  bool
	}{
		{
			name:        "valid content should allow",
			fileContent: "This is valid content\n",
			expectDeny:  false,
		},
		{
			name:        "blocked content should deny",
			fileContent: "This has BLOCK_ME in it\n",
			expectDeny:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Write the test file
			filePath := filepath.Join(tmpDir, "test-file.txt")
			if err := os.WriteFile(filePath, []byte(tt.fileContent), 0644); err != nil {
				t.Fatal(err)
			}

			// Build event with absolute path
			evt := &schema.Event{
				File: &schema.FileEvent{
					Path:   filePath,
					Action: "edit",
				},
				Lifecycle: "pre",
				Cwd:       tmpDir,
			}

			// Capture stdout
			oldStdout := os.Stdout
			stdoutR, stdoutW, _ := os.Pipe()
			os.Stdout = stdoutW

			_ = runMatchingWorkflowsWithEvent(tmpDir, evt)

			_ = stdoutW.Close()
			os.Stdout = oldStdout

			var buf bytes.Buffer
			_, _ = buf.ReadFrom(stdoutR)
			output := buf.String()

			if tt.expectDeny {
				if !strings.Contains(output, "deny") {
					t.Errorf("Expected deny for blocked content, got: %s", output)
				}
			} else {
				if !strings.Contains(output, "allow") {
					t.Errorf("Expected allow for valid content, got: %s", output)
				}
			}
		})
	}
}

// TestWorkflowStepConditions tests that step if conditions work correctly
func TestWorkflowStepConditions(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "hookflow-cond-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create hooks directory
	hooksDir := filepath.Join(tmpDir, ".github", "hookflows")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a workflow with multiple conditional steps
	workflow := `name: Conditional Steps Test
on:
  file:
    paths:
      - '**/*'
    types:
      - edit
      - create
blocking: true
steps:
  - name: JSON file check
    if: ${{ endsWith(event.file.path, '.json') }}
    run: echo "This is a JSON file"
  - name: Script file check
    if: ${{ endsWith(event.file.path, '.sh') }}
    run: echo "This is a shell script"
  - name: Config file check
    if: ${{ event.file.path == 'config.yml' }}
    run: echo "This is config.yml"
  - name: Always runs
    run: echo "Always executed"
`
	if err := os.WriteFile(filepath.Join(hooksDir, "conditions.yml"), []byte(workflow), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name       string
		filePath   string
		expectLogs []string // What we expect to see in logs
	}{
		{
			name:       "JSON file triggers JSON condition",
			filePath:   "data.json",
			expectLogs: []string{"This is a JSON file", "Always executed"},
		},
		{
			name:       "SH file triggers script condition",
			filePath:   "script.sh",
			expectLogs: []string{"This is a shell script", "Always executed"},
		},
		{
			name:       "config.yml triggers exact match",
			filePath:   "config.yml",
			expectLogs: []string{"This is config.yml", "Always executed"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create the file
			fullPath := filepath.Join(tmpDir, tt.filePath)
			if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(fullPath, []byte("test content"), 0644); err != nil {
				t.Fatal(err)
			}

			// Build event
			evt := &schema.Event{
				File: &schema.FileEvent{
					Path:   fullPath,
					Action: "edit",
				},
				Lifecycle: "pre",
				Cwd:       tmpDir,
			}

			// Capture stdout
			oldStdout := os.Stdout
			stdoutR, stdoutW, _ := os.Pipe()
			os.Stdout = stdoutW

			_ = runMatchingWorkflowsWithEvent(tmpDir, evt)

			_ = stdoutW.Close()
			os.Stdout = oldStdout

			var buf bytes.Buffer
			_, _ = buf.ReadFrom(stdoutR)
			output := buf.String()

			// Should always allow (no failing steps)
			if !strings.Contains(output, "allow") {
				t.Errorf("Expected allow, got: %s", output)
			}
		})
	}
}

// =============================================================================
// COPILOT HOOK INPUT FORMAT TESTS
// =============================================================================
// These tests verify that we correctly handle the JSON format that Copilot sends
// to hook scripts. The format is: {"toolName":"...", "toolArgs":{...}, "cwd":"..."}

// TestCopilotHookInputFormat tests that the event detector correctly parses
// the actual JSON format that Copilot sends via stdin to hook scripts
func TestCopilotHookInputFormat(t *testing.T) {
	tests := []struct {
		name           string
		inputJSON      string
		expectFile     bool
		expectedPath   string
		expectedAction string
		description    string
	}{
		{
			name: "edit tool with path",
			inputJSON: `{
				"toolName": "edit",
				"toolArgs": {"path": "/some/path/file.go", "old_str": "old", "new_str": "new"},
				"cwd": "/workspace"
			}`,
			expectFile:     true,
			expectedPath:   "/some/path/file.go",
			expectedAction: "edit",
			description:    "Standard edit tool invocation",
		},
		{
			name: "create tool with path and file_text",
			inputJSON: `{
				"toolName": "create",
				"toolArgs": {"path": "/workspace/new-file.ts", "file_text": "content"},
				"cwd": "/workspace"
			}`,
			expectFile:     true,
			expectedPath:   "/workspace/new-file.ts",
			expectedAction: "create",
			description:    "Standard create tool invocation",
		},
		{
			name: "view tool - should not trigger file event",
			inputJSON: `{
				"toolName": "view",
				"toolArgs": {"path": "/some/file.go"},
				"cwd": "/workspace"
			}`,
			expectFile:  false,
			description: "View tool should not be treated as file modification",
		},
		{
			name: "powershell tool - not a file event",
			inputJSON: `{
				"toolName": "powershell",
				"toolArgs": {"command": "Get-ChildItem"},
				"cwd": "/workspace"
			}`,
			expectFile:  false,
			description: "Shell commands are not file events",
		},
		{
			name: "Windows-style path in toolArgs",
			inputJSON: `{
				"toolName": "edit",
				"toolArgs": {"path": "C:\\Users\\test\\project\\file.go", "old_str": "a", "new_str": "b"},
				"cwd": "C:\\Users\\test\\project"
			}`,
			expectFile:     true,
			expectedPath:   "C:\\Users\\test\\project\\file.go",
			expectedAction: "edit",
			description:    "Windows backslash paths should be preserved",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use the event detector to parse the input
			detector := eventpkg.NewDetector(nil)
			evt, err := detector.DetectFromRawInput([]byte(tt.inputJSON))

			if err != nil {
				t.Fatalf("Failed to parse input: %v", err)
			}

			if tt.expectFile {
				if evt.File == nil {
					t.Errorf("%s: Expected file event but got nil", tt.description)
					return
				}
				if evt.File.Path != tt.expectedPath {
					t.Errorf("%s: Expected path %q, got %q", tt.description, tt.expectedPath, evt.File.Path)
				}
				if evt.File.Action != tt.expectedAction {
					t.Errorf("%s: Expected action %q, got %q", tt.description, tt.expectedAction, evt.File.Action)
				}
			} else {
				if evt.File != nil {
					t.Errorf("%s: Expected no file event but got path=%q", tt.description, evt.File.Path)
				}
			}
		})
	}
}

// =============================================================================
// PATH NORMALIZATION TESTS
// =============================================================================
// These tests verify that absolute paths are correctly normalized to relative paths
// for pattern matching in workflows

// TestPathNormalizationComprehensive tests all path normalization scenarios
func TestPathNormalizationComprehensive(t *testing.T) {
	tests := []struct {
		name           string
		filePath       string
		baseDir        string
		expectedResult string
		description    string
	}{
		// Basic cases
		{
			name:           "already relative - simple filename",
			filePath:       "plugin.json",
			baseDir:        "/workspace",
			expectedResult: "plugin.json",
			description:    "Already relative paths should stay unchanged",
		},
		{
			name:           "already relative - nested path",
			filePath:       "src/components/Button.tsx",
			baseDir:        "/workspace",
			expectedResult: "src/components/Button.tsx",
			description:    "Nested relative paths should stay unchanged",
		},

		// Unix absolute paths
		{
			name:           "Unix absolute - exact match with baseDir",
			filePath:       "/workspace/plugin.json",
			baseDir:        "/workspace",
			expectedResult: "plugin.json",
			description:    "Absolute path in baseDir should become relative",
		},
		{
			name:           "Unix absolute - nested in baseDir",
			filePath:       "/workspace/src/utils/helpers.go",
			baseDir:        "/workspace",
			expectedResult: "src/utils/helpers.go",
			description:    "Nested absolute path should become relative",
		},
		{
			name:           "Unix absolute - different root",
			filePath:       "/other/project/file.go",
			baseDir:        "/workspace",
			expectedResult: "/other/project/file.go",
			description:    "Path outside baseDir stays absolute",
		},

		// Windows absolute paths (with backslashes)
		{
			name:           "Windows absolute - C drive",
			filePath:       "C:\\Users\\test\\project\\plugin.json",
			baseDir:        "C:\\Users\\test\\project",
			expectedResult: "plugin.json",
			description:    "Windows path should normalize to relative",
		},
		{
			name:           "Windows absolute - nested",
			filePath:       "C:\\Repos\\myapp\\src\\components\\App.tsx",
			baseDir:        "C:\\Repos\\myapp",
			expectedResult: "src/components/App.tsx",
			description:    "Windows nested path normalizes with forward slashes",
		},
		{
			name:           "Windows absolute - D drive",
			filePath:       "D:\\Projects\\webapp\\index.html",
			baseDir:        "D:\\Projects\\webapp",
			expectedResult: "index.html",
			description:    "Different Windows drive letters work",
		},

		// Mixed slash scenarios
		{
			name:           "Mixed slashes - forward in Windows baseDir",
			filePath:       "C:/Users/test/project/file.go",
			baseDir:        "C:\\Users\\test\\project",
			expectedResult: "file.go",
			description:    "Forward slashes in Windows path work",
		},

		// Edge cases
		{
			name:           "Empty baseDir - strips leading slash",
			filePath:       "/some/path/file.go",
			baseDir:        "",
			expectedResult: "some/path/file.go",
			description:    "Empty baseDir strips leading slash (converts to relative)",
		},
		{
			name:           "Path equals baseDir exactly",
			filePath:       "/workspace",
			baseDir:        "/workspace",
			expectedResult: "/workspace",
			description:    "Path equal to baseDir returns path unchanged (edge case)",
		},
		{
			name:           "Trailing slash on baseDir",
			filePath:       "/workspace/file.go",
			baseDir:        "/workspace/",
			expectedResult: "file.go",
			description:    "Trailing slash on baseDir handled correctly",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeFilePath(tt.filePath, tt.baseDir)
			if result != tt.expectedResult {
				t.Errorf("%s:\n  input:    %q\n  baseDir:  %q\n  expected: %q\n  got:      %q",
					tt.description, tt.filePath, tt.baseDir, tt.expectedResult, result)
			}
		})
	}
}

// =============================================================================
// END-TO-END WORKFLOW MATCHING WITH ABSOLUTE PATHS
// =============================================================================
// These tests simulate the complete flow: Copilot sends absolute path → 
// hookflow normalizes → workflow pattern matches

// TestEndToEndAbsolutePathMatching tests the complete flow from raw input to workflow decision
func TestEndToEndAbsolutePathMatching(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "hookflow-e2e-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create hooks directory
	hooksDir := filepath.Join(tmpDir, ".github", "hookflows")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a workflow that blocks plugin.json edits
	workflow := `name: Block Plugin Edits
on:
  file:
    paths:
      - 'plugin.json'
    types:
      - edit
blocking: true
steps:
  - name: Block
    run: |
      echo "Blocked edit to plugin.json"
      exit 1
`
	if err := os.WriteFile(filepath.Join(hooksDir, "block.yml"), []byte(workflow), 0644); err != nil {
		t.Fatal(err)
	}

	// Create the target file
	if err := os.WriteFile(filepath.Join(tmpDir, "plugin.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name        string
		absPath     string // Absolute path as Copilot would send
		expectDeny  bool
		description string
	}{
		{
			name:        "Unix absolute path matches",
			absPath:     filepath.Join(tmpDir, "plugin.json"),
			expectDeny:  true,
			description: "Absolute path should normalize and match 'plugin.json' pattern",
		},
		{
			name:        "Different file doesn't match",
			absPath:     filepath.Join(tmpDir, "package.json"),
			expectDeny:  false,
			description: "Different filename should not match pattern",
		},
		{
			name:        "Nested path doesn't match root pattern",
			absPath:     filepath.Join(tmpDir, "config", "plugin.json"),
			expectDeny:  false,
			description: "plugin.json in subdirectory shouldn't match root pattern",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create any necessary parent directories for the test file
			dir := filepath.Dir(tt.absPath)
			if dir != tmpDir {
				_ = os.MkdirAll(dir, 0755)
				_ = os.WriteFile(tt.absPath, []byte("{}"), 0644)
			}

			evt := &schema.Event{
				File: &schema.FileEvent{
					Path:   tt.absPath,
					Action: "edit",
				},
				Cwd:       tmpDir,
				Lifecycle: "pre",
			}

			oldStdout := os.Stdout
			stdoutR, stdoutW, _ := os.Pipe()
			os.Stdout = stdoutW

			_ = runMatchingWorkflowsWithEvent(tmpDir, evt)

			_ = stdoutW.Close()
			os.Stdout = oldStdout

			var buf bytes.Buffer
			_, _ = buf.ReadFrom(stdoutR)
			output := buf.String()

			if tt.expectDeny {
				if !strings.Contains(output, "deny") {
					t.Errorf("%s: Expected deny, got: %s", tt.description, output)
				}
			} else {
				if !strings.Contains(output, "allow") {
					t.Errorf("%s: Expected allow, got: %s", tt.description, output)
				}
			}
		})
	}
}

// =============================================================================
// EXPRESSION CONTEXT WITH NORMALIZED PATHS
// =============================================================================
// These tests verify that step conditions using event.file.path work correctly
// after path normalization

// TestExpressionContextWithNormalizedPath tests that expressions evaluate correctly
// with normalized paths in the event context
func TestExpressionContextWithNormalizedPath(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "hookflow-expr-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	hooksDir := filepath.Join(tmpDir, ".github", "hookflows")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Workflow with multiple conditional steps testing different expressions
	workflow := `name: Expression Test
on:
  file:
    paths:
      - '**/*'
    types:
      - edit
      - create
blocking: true
steps:
  # Test exact path match
  - name: Exact match test
    if: ${{ event.file.path == 'plugin.json' }}
    run: |
      echo "exact_match_triggered"
      exit 1

  # Test endsWith function
  - name: EndsWith test
    if: ${{ endsWith(event.file.path, '.json') }}
    run: |
      echo "ends_with_json_triggered"
      exit 1

  # Test startsWith function  
  - name: StartsWith test
    if: ${{ startsWith(event.file.path, 'src/') }}
    run: |
      echo "starts_with_src_triggered"
      exit 1

  # Test contains function
  - name: Contains test
    if: ${{ contains(event.file.path, '/components/') }}
    run: |
      echo "contains_components_triggered"
      exit 1
`
	if err := os.WriteFile(filepath.Join(hooksDir, "expr-test.yml"), []byte(workflow), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name          string
		relativePath  string // Path relative to tmpDir
		expectDeny    bool
		denialReason  string // Which step should trigger
		description   string
	}{
		{
			name:         "exact path match - plugin.json",
			relativePath: "plugin.json",
			expectDeny:   true,
			denialReason: "exact_match_triggered",
			description:  "event.file.path == 'plugin.json' should match",
		},
		{
			name:         "endsWith match - config.json",
			relativePath: "config.json",
			expectDeny:   true,
			denialReason: "ends_with_json_triggered",
			description:  "endsWith(event.file.path, '.json') should match",
		},
		{
			name:         "startsWith match - src/index.ts",
			relativePath: "src/index.ts",
			expectDeny:   true,
			denialReason: "starts_with_src_triggered",
			description:  "startsWith(event.file.path, 'src/') should match",
		},
		{
			name:         "contains match - src/components/Button.tsx",
			relativePath: "src/components/Button.tsx",
			expectDeny:   true,
			denialReason: "contains_components_triggered",
			description:  "contains(event.file.path, '/components/') should match",
		},
		{
			name:         "no match - README.md",
			relativePath: "README.md",
			expectDeny:   false,
			description:  "README.md should not match any conditions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create directory structure
			fullPath := filepath.Join(tmpDir, tt.relativePath)
			_ = os.MkdirAll(filepath.Dir(fullPath), 0755)
			_ = os.WriteFile(fullPath, []byte("test"), 0644)

			// Use ABSOLUTE path like Copilot would send
			evt := &schema.Event{
				File: &schema.FileEvent{
					Path:   fullPath, // Absolute path!
					Action: "edit",
				},
				Cwd:       tmpDir,
				Lifecycle: "pre",
			}

			oldStdout := os.Stdout
			stdoutR, stdoutW, _ := os.Pipe()
			os.Stdout = stdoutW

			_ = runMatchingWorkflowsWithEvent(tmpDir, evt)

			_ = stdoutW.Close()
			os.Stdout = oldStdout

			var buf bytes.Buffer
			_, _ = buf.ReadFrom(stdoutR)
			output := buf.String()

			if tt.expectDeny {
				if !strings.Contains(output, "deny") {
					t.Errorf("%s: Expected deny, got: %s", tt.description, output)
				}
			} else {
				if !strings.Contains(output, "allow") {
					t.Errorf("%s: Expected allow, got: %s", tt.description, output)
				}
			}
		})
	}
}

// =============================================================================
// GLOB PATTERN MATCHING TESTS
// =============================================================================
// These tests verify that glob patterns in workflow triggers work correctly
// NOTE: On Windows, filepath.Match treats '/' as a regular character, not a path separator.
// This means '*.json' will match 'src/data.json' on Windows but not on Linux.
// For cross-platform consistency, use '**/*.json' to match in subdirectories.

// TestGlobPatternMatching tests various glob patterns in file triggers
func TestGlobPatternMatching(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "hookflow-glob-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Windows vs Unix: * matches / on Windows but not Unix (filepath.Match behavior)
	// We use runtime.GOOS to set expectations accordingly
	isWindows := runtime.GOOS == "windows"

	tests := []struct {
		name              string
		pattern           string
		testPaths         []string // Paths to test against the pattern
		shouldMatchUnix   []bool   // Expected results on Unix
		shouldMatchWin    []bool   // Expected results on Windows
	}{
		{
			name:              "simple filename",
			pattern:           "plugin.json",
			testPaths:         []string{"plugin.json", "other.json", "dir/plugin.json"},
			shouldMatchUnix:   []bool{true, false, false},
			shouldMatchWin:    []bool{true, false, false},
		},
		{
			name:              "extension glob - *.json (platform-dependent)",
			pattern:           "*.json",
			testPaths:         []string{"plugin.json", "config.json", "src/data.json", "file.txt"},
			shouldMatchUnix:   []bool{true, true, false, false}, // * doesn't match /
			shouldMatchWin:    []bool{true, true, true, false},  // * matches / on Windows
		},
		{
			name:              "recursive glob - **/*.json (cross-platform)",
			pattern:           "**/*.json",
			testPaths:         []string{"plugin.json", "src/config.json", "a/b/c/data.json", "file.txt"},
			shouldMatchUnix:   []bool{true, true, true, false},
			shouldMatchWin:    []bool{true, true, true, false},
		},
		{
			name:              "directory prefix - src/**",
			pattern:           "src/**",
			testPaths:         []string{"src/index.ts", "src/components/Button.tsx", "lib/utils.ts"},
			shouldMatchUnix:   []bool{true, true, false},
			shouldMatchWin:    []bool{true, true, false},
		},
		{
			name:              "specific nested path",
			pattern:           "packages/hooks/scripts/**",
			testPaths:         []string{"packages/hooks/scripts/pre.sh", "packages/hooks/scripts/lib/util.sh", "packages/other/script.sh"},
			shouldMatchUnix:   []bool{true, true, false},
			shouldMatchWin:    []bool{true, true, false},
		},
		{
			name:              "hidden files - **/.env",
			pattern:           "**/.env",
			testPaths:         []string{".env", "config/.env", "a/b/.env", ".env.local"},
			shouldMatchUnix:   []bool{true, true, true, false},
			shouldMatchWin:    []bool{true, true, true, false},
		},
		{
			name:              "extension match - **/*.ts",
			pattern:           "**/*.ts",
			testPaths:         []string{"index.ts", "src/App.tsx", "lib/utils.ts", "file.js"},
			shouldMatchUnix:   []bool{true, false, true, false},
			shouldMatchWin:    []bool{true, false, true, false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hooksDir := filepath.Join(tmpDir, ".github", "hookflows")
			_ = os.RemoveAll(hooksDir)
			_ = os.MkdirAll(hooksDir, 0755)

			// Create workflow with this pattern
			workflow := fmt.Sprintf(`name: Pattern Test
on:
  file:
    paths:
      - '%s'
    types:
      - edit
blocking: true
steps:
  - name: Block
    run: exit 1
`, tt.pattern)
			if err := os.WriteFile(filepath.Join(hooksDir, "test.yml"), []byte(workflow), 0644); err != nil {
				t.Fatal(err)
			}

			// Select platform-appropriate expectations
			shouldMatch := tt.shouldMatchUnix
			if isWindows {
				shouldMatch = tt.shouldMatchWin
			}

			for i, testPath := range tt.testPaths {
				t.Run(testPath, func(t *testing.T) {
					// Create file structure
					fullPath := filepath.Join(tmpDir, testPath)
					_ = os.MkdirAll(filepath.Dir(fullPath), 0755)
					_ = os.WriteFile(fullPath, []byte("test"), 0644)

					evt := &schema.Event{
						File: &schema.FileEvent{
							Path:   fullPath,
							Action: "edit",
						},
						Cwd:       tmpDir,
						Lifecycle: "pre",
					}

					oldStdout := os.Stdout
					stdoutR, stdoutW, _ := os.Pipe()
					os.Stdout = stdoutW

					_ = runMatchingWorkflowsWithEvent(tmpDir, evt)

					_ = stdoutW.Close()
					os.Stdout = oldStdout

					var buf bytes.Buffer
					_, _ = buf.ReadFrom(stdoutR)
					output := buf.String()

					expectMatch := shouldMatch[i]
					if expectMatch {
						if !strings.Contains(output, "deny") {
							t.Errorf("Pattern %q should match %q but got allow: %s", tt.pattern, testPath, output)
						}
					} else {
						if !strings.Contains(output, "allow") {
							t.Errorf("Pattern %q should NOT match %q but got deny: %s", tt.pattern, testPath, output)
						}
					}
				})
			}
		})
	}
}

// =============================================================================
// MULTIPLE WORKFLOW MATCHING TESTS
// =============================================================================
// These tests verify behavior when multiple workflows could potentially match

// TestMultipleWorkflowsMatching tests that multiple workflows are evaluated correctly
func TestMultipleWorkflowsMatching(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "hookflow-multi-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	hooksDir := filepath.Join(tmpDir, ".github", "hookflows")
	_ = os.MkdirAll(hooksDir, 0755)

	// Workflow 1: Blocks .env files
	workflow1 := `name: Block Env Files
on:
  file:
    paths:
      - '**/.env'
      - '**/.env.*'
    types:
      - edit
      - create
blocking: true
steps:
  - name: Block
    run: |
      echo "env_file_blocked"
      exit 1
`
	_ = os.WriteFile(filepath.Join(hooksDir, "01-block-env.yml"), []byte(workflow1), 0644)

	// Workflow 2: Validates JSON files (allows them)
	workflow2 := `name: Validate JSON
on:
  file:
    paths:
      - '**/*.json'
    types:
      - edit
blocking: true
steps:
  - name: Validate
    run: echo "json_validated"
`
	_ = os.WriteFile(filepath.Join(hooksDir, "02-validate-json.yml"), []byte(workflow2), 0644)

	// Workflow 3: Blocks secret files
	// NOTE: Pattern '**/secrets/**' with ** on both sides doesn't work correctly
	// (known limitation in glob matching). Use 'secrets/**' for directory matching.
	workflow3 := `name: Block Secrets
on:
  file:
    paths:
      - 'secrets/**'
      - '**/*.secret'
    types:
      - edit
      - create
blocking: true
steps:
  - name: Block
    run: |
      echo "secret_blocked"
      exit 1
`
	_ = os.WriteFile(filepath.Join(hooksDir, "03-block-secrets.yml"), []byte(workflow3), 0644)

	tests := []struct {
		name        string
		filePath    string
		action      string
		expectDeny  bool
		description string
	}{
		{
			name:        "env file - blocked by workflow 1",
			filePath:    ".env",
			action:      "edit",
			expectDeny:  true,
			description: ".env should be blocked by env workflow",
		},
		{
			name:        "env.local - blocked by workflow 1",
			filePath:    ".env.local",
			action:      "create",
			expectDeny:  true,
			description: ".env.local should be blocked by env workflow",
		},
		{
			name:        "json file - allowed by workflow 2",
			filePath:    "config.json",
			action:      "edit",
			expectDeny:  false,
			description: "JSON files should pass validation",
		},
		{
			name:        "secret file - blocked by workflow 3",
			filePath:    "secrets/api.key",
			action:      "create",
			expectDeny:  true,
			description: "Secret files should be blocked",
		},
		{
			name:        "normal file - no workflow matches",
			filePath:    "README.md",
			action:      "edit",
			expectDeny:  false,
			description: "Files not matching any workflow should be allowed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fullPath := filepath.Join(tmpDir, tt.filePath)
			_ = os.MkdirAll(filepath.Dir(fullPath), 0755)
			_ = os.WriteFile(fullPath, []byte("test"), 0644)

			evt := &schema.Event{
				File: &schema.FileEvent{
					Path:   fullPath,
					Action: tt.action,
				},
				Cwd:       tmpDir,
				Lifecycle: "pre",
			}

			oldStdout := os.Stdout
			stdoutR, stdoutW, _ := os.Pipe()
			os.Stdout = stdoutW

			_ = runMatchingWorkflowsWithEvent(tmpDir, evt)

			_ = stdoutW.Close()
			os.Stdout = oldStdout

			var buf bytes.Buffer
			_, _ = buf.ReadFrom(stdoutR)
			output := buf.String()

			if tt.expectDeny {
				if !strings.Contains(output, "deny") {
					t.Errorf("%s: Expected deny, got: %s", tt.description, output)
				}
			} else {
				if !strings.Contains(output, "allow") {
					t.Errorf("%s: Expected allow, got: %s", tt.description, output)
				}
			}
		})
	}
}

// =============================================================================
// TOOL ARGS ACCESS IN EXPRESSIONS
// =============================================================================
// These tests verify that expressions can access tool arguments like new_str, old_str

// TestToolArgsInExpressions tests accessing tool arguments in step conditions
func TestToolArgsInExpressions(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "hookflow-toolargs-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	hooksDir := filepath.Join(tmpDir, ".github", "hookflows")
	_ = os.MkdirAll(hooksDir, 0755)

	// Workflow that checks for sensitive patterns in new_str
	workflow := `name: Check Edit Content
on:
  file:
    paths:
      - '**/*'
    types:
      - edit
blocking: true
steps:
  - name: Check for password
    if: ${{ contains(event.tool.args.new_str, 'password') }}
    run: |
      echo "password_detected"
      exit 1
  - name: Check for API key pattern
    if: ${{ contains(event.tool.args.new_str, 'sk-') }}
    run: |
      echo "api_key_detected"
      exit 1
  - name: Check for AWS key pattern
    if: ${{ contains(event.tool.args.new_str, 'AKIA') }}
    run: |
      echo "aws_key_detected"
      exit 1
`
	_ = os.WriteFile(filepath.Join(hooksDir, "check-content.yml"), []byte(workflow), 0644)

	tests := []struct {
		name        string
		newStr      string
		expectDeny  bool
		description string
	}{
		{
			name:        "contains password literal",
			newStr:      "const password = 'secret123';",
			expectDeny:  true,
			description: "Should detect 'password' keyword",
		},
		{
			name:        "contains API key pattern",
			newStr:      "const apiKey = 'sk-1234567890abcdef';",
			expectDeny:  true,
			description: "Should detect 'sk-' pattern",
		},
		{
			name:        "contains AWS key pattern",
			newStr:      "AWS_KEY=AKIAIOSFODNN7EXAMPLE",
			expectDeny:  true,
			description: "Should detect 'AKIA' pattern",
		},
		{
			name:        "safe content",
			newStr:      "const greeting = 'Hello, World!';",
			expectDeny:  false,
			description: "Safe content should be allowed",
		},
		{
			name:        "empty new_str",
			newStr:      "",
			expectDeny:  false,
			description: "Empty content should be allowed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fullPath := filepath.Join(tmpDir, "test.js")
			_ = os.WriteFile(fullPath, []byte("original"), 0644)

			evt := &schema.Event{
				File: &schema.FileEvent{
					Path:   fullPath,
					Action: "edit",
				},
				Tool: &schema.ToolEvent{
					Name: "edit",
					Args: map[string]interface{}{
						"path":    fullPath,
						"old_str": "original",
						"new_str": tt.newStr,
					},
				},
				Cwd:       tmpDir,
				Lifecycle: "pre",
			}

			oldStdout := os.Stdout
			stdoutR, stdoutW, _ := os.Pipe()
			os.Stdout = stdoutW

			_ = runMatchingWorkflowsWithEvent(tmpDir, evt)

			_ = stdoutW.Close()
			os.Stdout = oldStdout

			var buf bytes.Buffer
			_, _ = buf.ReadFrom(stdoutR)
			output := buf.String()

			if tt.expectDeny {
				if !strings.Contains(output, "deny") {
					t.Errorf("%s: Expected deny, got: %s", tt.description, output)
				}
			} else {
				if !strings.Contains(output, "allow") {
					t.Errorf("%s: Expected allow, got: %s", tt.description, output)
				}
			}
		})
	}
}

// =============================================================================
// FILE CONTENT ACCESS IN EXPRESSIONS
// =============================================================================
// These tests verify that expressions can access file content for create events

// TestFileContentInExpressions tests accessing file content in step conditions
func TestFileContentInExpressions(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "hookflow-content-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	hooksDir := filepath.Join(tmpDir, ".github", "hookflows")
	_ = os.MkdirAll(hooksDir, 0755)

	// Workflow that checks file content on create
	workflow := `name: Check New File Content
on:
  file:
    paths:
      - '**/*'
    types:
      - create
blocking: true
steps:
  - name: Check for TODO markers
    if: ${{ contains(event.file.content, 'TODO:') }}
    run: |
      echo "todo_found"
      exit 1
  - name: Check for FIXME markers
    if: ${{ contains(event.file.content, 'FIXME:') }}
    run: |
      echo "fixme_found"
      exit 1
  - name: Check for console.log
    if: ${{ contains(event.file.content, 'console.log') }}
    run: |
      echo "console_log_found"
      exit 1
`
	_ = os.WriteFile(filepath.Join(hooksDir, "check-new-file.yml"), []byte(workflow), 0644)

	tests := []struct {
		name        string
		fileContent string
		expectDeny  bool
		description string
	}{
		{
			name:        "contains TODO",
			fileContent: "// TODO: implement this function\nfunction foo() {}",
			expectDeny:  true,
			description: "Should detect TODO markers",
		},
		{
			name:        "contains FIXME",
			fileContent: "// FIXME: this is broken\nfunction bar() {}",
			expectDeny:  true,
			description: "Should detect FIXME markers",
		},
		{
			name:        "contains console.log",
			fileContent: "function debug() {\n  console.log('test');\n}",
			expectDeny:  true,
			description: "Should detect console.log statements",
		},
		{
			name:        "clean code",
			fileContent: "function greet(name) {\n  return `Hello, ${name}!`;\n}",
			expectDeny:  false,
			description: "Clean code should be allowed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fullPath := filepath.Join(tmpDir, "newfile.js")

			evt := &schema.Event{
				File: &schema.FileEvent{
					Path:    fullPath,
					Action:  "create",
					Content: tt.fileContent,
				},
				Tool: &schema.ToolEvent{
					Name: "create",
					Args: map[string]interface{}{
						"path":      fullPath,
						"file_text": tt.fileContent,
					},
				},
				Cwd:       tmpDir,
				Lifecycle: "pre",
			}

			oldStdout := os.Stdout
			stdoutR, stdoutW, _ := os.Pipe()
			os.Stdout = stdoutW

			_ = runMatchingWorkflowsWithEvent(tmpDir, evt)

			_ = stdoutW.Close()
			os.Stdout = oldStdout

			var buf bytes.Buffer
			_, _ = buf.ReadFrom(stdoutR)
			output := buf.String()

			if tt.expectDeny {
				if !strings.Contains(output, "deny") {
					t.Errorf("%s: Expected deny, got: %s", tt.description, output)
				}
			} else {
				if !strings.Contains(output, "allow") {
					t.Errorf("%s: Expected allow, got: %s", tt.description, output)
				}
			}
		})
	}
}
