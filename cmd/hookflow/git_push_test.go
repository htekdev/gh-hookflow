package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/htekdev/gh-hookflow/internal/activity"
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

	// Verify the reason mentions hookflow git-push
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

// TestBuildPushEvent verifies that buildPushEvent creates a correct event
func TestBuildPushEvent(t *testing.T) {
	tmpDir := t.TempDir()

	evt := buildPushEvent(tmpDir, "pre")

	if evt.Push == nil {
		t.Fatal("expected Push to be set")
	}
	if evt.Lifecycle != "pre" {
		t.Errorf("expected lifecycle 'pre', got %q", evt.Lifecycle)
	}
	if evt.Cwd != tmpDir {
		t.Errorf("expected cwd %q, got %q", tmpDir, evt.Cwd)
	}
	if evt.Timestamp == "" {
		t.Error("expected non-empty timestamp")
	}
}

// TestLifecycleToPhase verifies lifecycle-to-phase conversion
func TestLifecycleToPhase(t *testing.T) {
	tests := []struct {
		lifecycle string
		want      activity.Phase
	}{
		{"pre", activity.PhasePrePush},
		{"post", activity.PhasePostPush},
		{"unknown", activity.PhasePrePush},
	}

	for _, tt := range tests {
		got := lifecycleToPhase(tt.lifecycle)
		if got != tt.want {
			t.Errorf("lifecycleToPhase(%q) = %q, want %q", tt.lifecycle, got, tt.want)
		}
	}
}

// TestGitPushResponseSerialization verifies JSON serialization of GitPushResponse
func TestGitPushResponseSerialization(t *testing.T) {
	resp := &GitPushResponse{
		ActivityID: "abc123",
		Status:     activity.StatusCompleted,
		PrePush:    &PhaseResult{Passed: true, WorkflowsRun: 2},
		Push:       &PushPhaseResult{Success: true, Output: "Everything up-to-date"},
		PostPush:   &PostPushResult{Passed: true, WorkflowsRun: 1},
		Message: "Push and all checks completed successfully.",
	}

	data, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal response: %v", err)
	}

	// Parse it back
	var parsed GitPushResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if parsed.ActivityID != "abc123" {
		t.Errorf("expected activity_id 'abc123', got %q", parsed.ActivityID)
	}
	if parsed.Status != activity.StatusCompleted {
		t.Errorf("expected status 'completed', got %q", parsed.Status)
	}
	if !parsed.PrePush.Passed {
		t.Error("expected pre_push.passed = true")
	}
	if !parsed.Push.Success {
		t.Error("expected push.success = true")
	}
	if !parsed.PostPush.Passed {
		t.Error("expected post_push.passed = true")
	}
}

// TestGitPushResponseSerializationPostPushFailed verifies failed post-push serialization
func TestGitPushResponseSerializationPostPushFailed(t *testing.T) {
	resp := &GitPushResponse{
		ActivityID: "fail789",
		Status:     activity.StatusFailed,
		PrePush:    &PhaseResult{Passed: true, WorkflowsRun: 1},
		Push:       &PushPhaseResult{Success: true, Output: "pushed"},
		PostPush:   &PostPushResult{Passed: false, WorkflowsRun: 2},
		Message:    "Push succeeded but post-push checks failed.",
	}

	data, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal response: %v", err)
	}

	var parsed GitPushResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if parsed.Status != activity.StatusFailed {
		t.Errorf("expected status 'failed', got %q", parsed.Status)
	}
	if parsed.PostPush.Passed {
		t.Error("expected post_push.passed = false")
	}
}

// TestRunPushWorkflowsNoWorkflowDir verifies behavior when no hookflows directory exists
func TestRunPushWorkflowsNoWorkflowDir(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	act, err := activity.NewActivity([]string{"origin", "main"})
	if err != nil {
		t.Fatalf("NewActivity failed: %v", err)
	}

	result, err := runPushWorkflows(tmpDir, act, "pre")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !result.passed {
		t.Error("expected passed=true when no workflows exist")
	}
	if result.workflowsRun != 0 {
		t.Errorf("expected 0 workflows run, got %d", result.workflowsRun)
	}
}

// TestRunPushWorkflowsNoMatchingWorkflows verifies behavior when workflows exist but none match
func TestRunPushWorkflowsNoMatchingWorkflows(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	// Create a hookflows directory with a file-trigger workflow (won't match push)
	hookflowDir := filepath.Join(tmpDir, ".github", "hookflows")
	if err := os.MkdirAll(hookflowDir, 0755); err != nil {
		t.Fatal(err)
	}
	workflowContent := `name: file-only
on:
  file:
    types: [create]
    paths: ["*.txt"]
steps:
  - name: Check
    run: echo "checking"
`
	if err := os.WriteFile(filepath.Join(hookflowDir, "file-only.yml"), []byte(workflowContent), 0644); err != nil {
		t.Fatal(err)
	}

	act, err := activity.NewActivity([]string{"origin", "main"})
	if err != nil {
		t.Fatalf("NewActivity failed: %v", err)
	}

	result, err := runPushWorkflows(tmpDir, act, "pre")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !result.passed {
		t.Error("expected passed=true when no workflows match")
	}
	if result.workflowsRun != 0 {
		t.Errorf("expected 0 workflows run, got %d", result.workflowsRun)
	}
}

// TestRunPushWorkflowsWithMatchingWorkflow verifies a matching push workflow runs
func TestRunPushWorkflowsWithMatchingWorkflow(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	hookflowDir := filepath.Join(tmpDir, ".github", "hookflows")
	if err := os.MkdirAll(hookflowDir, 0755); err != nil {
		t.Fatal(err)
	}

	// A push workflow that succeeds
	workflowContent := `name: pre-push-lint
on:
  push:
    lifecycle: pre
steps:
  - name: Lint check
    run: echo "all good"
`
	if err := os.WriteFile(filepath.Join(hookflowDir, "push-lint.yml"), []byte(workflowContent), 0644); err != nil {
		t.Fatal(err)
	}

	act, err := activity.NewActivity([]string{"origin", "main"})
	if err != nil {
		t.Fatalf("NewActivity failed: %v", err)
	}

	result, err := runPushWorkflows(tmpDir, act, "pre")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !result.passed {
		t.Error("expected passed=true for successful workflow")
	}
	if result.workflowsRun != 1 {
		t.Errorf("expected 1 workflow run, got %d", result.workflowsRun)
	}

	// Verify the workflow result was recorded in the activity
	phase := act.Phases[activity.PhasePrePush]
	if len(phase.Workflows) != 1 {
		t.Fatalf("expected 1 workflow in activity, got %d", len(phase.Workflows))
	}
	if !phase.Workflows[0].Success {
		t.Error("expected workflow to be successful in activity")
	}
}

// TestRunPushWorkflowsWithFailingWorkflow verifies a failing push workflow denies
func TestRunPushWorkflowsWithFailingWorkflow(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	hookflowDir := filepath.Join(tmpDir, ".github", "hookflows")
	if err := os.MkdirAll(hookflowDir, 0755); err != nil {
		t.Fatal(err)
	}

	// A push workflow that fails
	workflowContent := `name: pre-push-check
on:
  push:
    lifecycle: pre
steps:
  - name: Failing check
    run: exit 1
`
	if err := os.WriteFile(filepath.Join(hookflowDir, "push-fail.yml"), []byte(workflowContent), 0644); err != nil {
		t.Fatal(err)
	}

	act, err := activity.NewActivity([]string{"origin", "main"})
	if err != nil {
		t.Fatalf("NewActivity failed: %v", err)
	}

	result, err := runPushWorkflows(tmpDir, act, "pre")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result.passed {
		t.Error("expected passed=false for failing workflow")
	}
}

// TestOutputGitPushResponse verifies JSON is written to stdout
func TestOutputGitPushResponse(t *testing.T) {
	resp := &GitPushResponse{
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

	var parsed GitPushResponse
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v\nOutput: %s", err, output)
	}
	if parsed.ActivityID != "test123" {
		t.Errorf("expected activity_id 'test123', got %q", parsed.ActivityID)
	}
}

// TestBuildPushEventWithLifecycles verifies event construction for both lifecycles
func TestBuildPushEventWithLifecycles(t *testing.T) {
	tmpDir := t.TempDir()

	for _, lifecycle := range []string{"pre", "post"} {
		evt := buildPushEvent(tmpDir, lifecycle)
		if evt.Lifecycle != lifecycle {
			t.Errorf("expected lifecycle %q, got %q", lifecycle, evt.Lifecycle)
		}
		if evt.Push == nil {
			t.Fatal("expected Push to be set")
		}
		if evt.Cwd != tmpDir {
			t.Errorf("expected cwd %q, got %q", tmpDir, evt.Cwd)
		}
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
