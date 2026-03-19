package push

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildPushEvent(t *testing.T) {
	tmpDir := t.TempDir()

	evt := BuildPushEvent(tmpDir, "pre")

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

func TestBuildPushEventWithLifecycles(t *testing.T) {
	tmpDir := t.TempDir()

	for _, lifecycle := range []string{"pre", "post"} {
		evt := BuildPushEvent(tmpDir, lifecycle)
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

func TestResponseStatusConstants(t *testing.T) {
	if StatusCompleted != "completed" {
		t.Errorf("expected StatusCompleted to be 'completed', got %q", StatusCompleted)
	}
	if StatusFailed != "failed" {
		t.Errorf("expected StatusFailed to be 'failed', got %q", StatusFailed)
	}
}

func TestResponseSerialization(t *testing.T) {
	resp := &Response{
		Status:   StatusCompleted,
		PrePush:  &PhaseResult{Passed: true, WorkflowsRun: 2},
		Push:     &PushPhaseResult{Success: true, Output: "Everything up-to-date"},
		PostPush: &PostPushResult{Passed: true, WorkflowsRun: 1},
		Message:  "Push and all checks completed successfully.",
	}

	if resp.Status != StatusCompleted {
		t.Errorf("expected status 'completed', got %q", resp.Status)
	}
	if !resp.PrePush.Passed {
		t.Error("expected pre_push.passed = true")
	}
	if !resp.Push.Success {
		t.Error("expected push.success = true")
	}
	if !resp.PostPush.Passed {
		t.Error("expected post_push.passed = true")
	}
}

func TestRunPushWorkflowsNoWorkflowDir(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	result, err := runPushWorkflows(tmpDir, "pre", false)
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

func TestRunPushWorkflowsNoMatchingWorkflows(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

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

	result, err := runPushWorkflows(tmpDir, "pre", false)
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

func TestRunPushWorkflowsWithMatchingWorkflow(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	hookflowDir := filepath.Join(tmpDir, ".github", "hookflows")
	if err := os.MkdirAll(hookflowDir, 0755); err != nil {
		t.Fatal(err)
	}

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

	result, err := runPushWorkflows(tmpDir, "pre", false)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !result.passed {
		t.Error("expected passed=true for successful workflow")
	}
	if result.workflowsRun != 1 {
		t.Errorf("expected 1 workflow run, got %d", result.workflowsRun)
	}
}

func TestRunPushWorkflowsWithFailingWorkflow(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	hookflowDir := filepath.Join(tmpDir, ".github", "hookflows")
	if err := os.MkdirAll(hookflowDir, 0755); err != nil {
		t.Fatal(err)
	}

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

	result, err := runPushWorkflows(tmpDir, "pre", false)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result.passed {
		t.Error("expected passed=false for failing workflow")
	}
}
