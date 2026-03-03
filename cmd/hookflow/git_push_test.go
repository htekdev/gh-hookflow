package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/htekdev/gh-hookflow/internal/activity"
	"github.com/htekdev/gh-hookflow/internal/push"
	"github.com/htekdev/gh-hookflow/internal/schema"
)

// TestBlockDirectGitPush verifies that direct git push is blocked in preToolUse
func TestBlockDirectGitPush(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a hookflows directory with a minimal workflow
	hookflowDir := filepath.Join(tmpDir, ".github", "hookflows")
	if err := os.MkdirAll(hookflowDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a simple push workflow
	workflowContent := `name: pre-push-check
on:
  push:
    lifecycle: pre
steps:
  - name: Check
    run: echo "checking"
`
	if err := os.WriteFile(filepath.Join(hookflowDir, "push.yml"), []byte(workflowContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Build a push event (like what the detector would produce from `git push`)
	evt := &schema.Event{
		Push: &schema.PushEvent{
			Ref:    "refs/heads/main",
			Before: "",
			After:  "",
		},
		Cwd:       tmpDir,
		Lifecycle: "pre",
		Tool: &schema.ToolEvent{
			Name:     "powershell",
			Args:     map[string]interface{}{"command": "git push origin main"},
			HookType: "preToolUse",
		},
	}

	// Capture stdout to check deny response
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runMatchingWorkflowsWithEvent(tmpDir, evt)

	_ = w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Read captured output
	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	// Parse the JSON response
	var result schema.WorkflowResult
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("failed to parse output as JSON: %v\nOutput: %s", err, output)
	}

	// Verify it's a deny
	if result.PermissionDecision != "deny" {
		t.Errorf("expected deny, got %s", result.PermissionDecision)
	}

	// Verify the reason mentions hookflow_git_push tool
	if result.PermissionDecisionReason == "" {
		t.Error("expected a deny reason")
	}
}

// TestGitPushNotBlockedForPost verifies that post lifecycle push events are NOT blocked
func TestGitPushNotBlockedForPost(t *testing.T) {
	tmpDir := t.TempDir()

	// No hookflows directory needed — the point is the event should NOT be blocked

	evt := &schema.Event{
		Push: &schema.PushEvent{
			Ref: "refs/heads/main",
		},
		Cwd:       tmpDir,
		Lifecycle: "post",
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runMatchingWorkflowsWithEvent(tmpDir, evt)

	_ = w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	var result schema.WorkflowResult
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("failed to parse output as JSON: %v\nOutput: %s", err, output)
	}

	// Post lifecycle should NOT be blocked
	if result.PermissionDecision == "deny" {
		t.Error("post lifecycle push should not be blocked")
	}
}

// TestOutputGitPushResponse verifies JSON is written to stdout
func TestOutputGitPushResponse(t *testing.T) {
	resp := &push.Response{
		ActivityID: "test123",
		Status:     activity.StatusCompleted,
		Message:    "All good",
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputGitPushResponse(resp)

	_ = w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	var parsed push.Response
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v\nOutput: %s", err, output)
	}
	if parsed.ActivityID != "test123" {
		t.Errorf("expected activity_id 'test123', got %q", parsed.ActivityID)
	}
}

// TestTailLog verifies the tailLog helper function
func TestTailLog(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	// Create a log file with 10 lines
	var content string
	for i := 1; i <= 10; i++ {
		content += fmt.Sprintf("line %d\n", i)
	}
	if err := os.WriteFile(logFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := tailLog(logFile, 3)

	_ = w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("tailLog failed: %v", err)
	}

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	// Should contain last 3 lines
	if !strings.Contains(output, "line 9") || !strings.Contains(output, "line 10") {
		t.Errorf("expected last lines, got: %s", output)
	}
}

// TestTailLogNonExistent verifies tailLog returns error for missing file
func TestTailLogNonExistent(t *testing.T) {
	err := tailLog("/nonexistent/file.log", 10)
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

// TestOutputWorkflowResultDeny verifies deny result JSON output
func TestOutputWorkflowResultDeny(t *testing.T) {
	result := schema.NewDenyResult("test denial reason")

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputWorkflowResult(result)

	_ = w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("outputWorkflowResult failed: %v", err)
	}

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	var parsed schema.WorkflowResult
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if parsed.PermissionDecision != "deny" {
		t.Errorf("expected deny, got %s", parsed.PermissionDecision)
	}
}
