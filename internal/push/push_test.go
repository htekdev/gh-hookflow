package push

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/htekdev/gh-hookflow/internal/activity"
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
		got := LifecycleToPhase(tt.lifecycle)
		if got != tt.want {
			t.Errorf("LifecycleToPhase(%q) = %q, want %q", tt.lifecycle, got, tt.want)
		}
	}
}

func TestResponseSerialization(t *testing.T) {
	resp := &Response{
		ActivityID: "abc123",
		Status:     activity.StatusCompleted,
		PrePush:    &PhaseResult{Passed: true, WorkflowsRun: 2},
		Push:       &PushPhaseResult{Success: true, Output: "Everything up-to-date"},
		PostPush:   &PostPushResult{Passed: true, WorkflowsRun: 1},
		Message:    "Push and all checks completed successfully.",
	}

	if resp.ActivityID != "abc123" {
		t.Errorf("expected activity_id 'abc123', got %q", resp.ActivityID)
	}
	if resp.Status != activity.StatusCompleted {
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

	act, err := activity.NewActivity([]string{"origin", "main"})
	if err != nil {
		t.Fatalf("NewActivity failed: %v", err)
	}

	result, err := runPushWorkflows(tmpDir, act, "pre", false)
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

	act, err := activity.NewActivity([]string{"origin", "main"})
	if err != nil {
		t.Fatalf("NewActivity failed: %v", err)
	}

	result, err := runPushWorkflows(tmpDir, act, "pre", false)
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

	act, err := activity.NewActivity([]string{"origin", "main"})
	if err != nil {
		t.Fatalf("NewActivity failed: %v", err)
	}

	result, err := runPushWorkflows(tmpDir, act, "pre", false)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !result.passed {
		t.Error("expected passed=true for successful workflow")
	}
	if result.workflowsRun != 1 {
		t.Errorf("expected 1 workflow run, got %d", result.workflowsRun)
	}

	phase := act.Phases[activity.PhasePrePush]
	if len(phase.Workflows) != 1 {
		t.Fatalf("expected 1 workflow in activity, got %d", len(phase.Workflows))
	}
	if !phase.Workflows[0].Success {
		t.Error("expected workflow to be successful in activity")
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

	act, err := activity.NewActivity([]string{"origin", "main"})
	if err != nil {
		t.Fatalf("NewActivity failed: %v", err)
	}

	result, err := runPushWorkflows(tmpDir, act, "pre", false)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result.passed {
		t.Error("expected passed=false for failing workflow")
	}
}
