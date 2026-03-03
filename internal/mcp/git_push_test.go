package mcp

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/htekdev/gh-hookflow/internal/activity"
)

func TestHandleGitPush_MissingCwd(t *testing.T) {
	input := gitPushInput{Cwd: "", Args: []string{"origin", "main"}}
	result, out, err := handleGitPush(context.Background(), nil, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || !result.IsError {
		t.Error("expected IsError=true for missing cwd")
	}
	if out.Message != "cwd is required" {
		t.Errorf("unexpected message: %s", out.Message)
	}
}

func TestHandleGitPush_StartsActivity(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	input := gitPushInput{Cwd: tmpDir, Args: []string{"origin", "main"}}
	result, out, err := handleGitPush(context.Background(), nil, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil && result.IsError {
		t.Errorf("unexpected error result: %s", out.Message)
	}
	if out.ActivityID == "" {
		t.Error("expected non-empty activity_id")
	}
	if out.Status != "running" {
		t.Errorf("expected status 'running', got %q", out.Status)
	}
	if out.NextStep == "" {
		t.Error("expected non-empty next_step")
	}

	// Give goroutine time to start (it will fail because no git repo, but that's fine)
	time.Sleep(100 * time.Millisecond)
}

func TestHandleGitPushStatus_MissingActivityID(t *testing.T) {
	input := gitPushStatusInput{ActivityID: ""}
	result, out, err := handleGitPushStatus(context.Background(), nil, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || !result.IsError {
		t.Error("expected IsError=true for missing activity_id")
	}
	if out.Message != "activity_id is required" {
		t.Errorf("unexpected message: %s", out.Message)
	}
}

func TestHandleGitPushStatus_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	input := gitPushStatusInput{ActivityID: "nonexistent"}
	result, out, err := handleGitPushStatus(context.Background(), nil, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || !result.IsError {
		t.Error("expected IsError=true for nonexistent activity")
	}
	if out.Message == "" {
		t.Error("expected error message")
	}
}

func TestHandleGitPushStatus_CompletedActivity(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	// Create and complete an activity manually
	act, err := activity.NewActivity([]string{"origin", "main"})
	if err != nil {
		t.Fatalf("NewActivity failed: %v", err)
	}
	act.StartPhase(activity.PhasePrePush)
	act.CompletePhase(activity.PhasePrePush, true, "1 workflow passed")
	act.StartPhase(activity.PhasePush)
	act.CompletePhase(activity.PhasePush, true, "pushed")
	act.StartPhase(activity.PhasePostPush)
	act.CompletePhase(activity.PhasePostPush, true, "1 workflow completed")
	act.Complete(activity.StatusCompleted, "Push and all checks completed successfully.")

	input := gitPushStatusInput{ActivityID: act.ID}
	result, out, err := handleGitPushStatus(context.Background(), nil, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil && result.IsError {
		t.Errorf("unexpected error result: %s", out.Message)
	}
	if out.Status != "completed" {
		t.Errorf("expected status 'completed', got %q", out.Status)
	}
	if out.NextStep != "" {
		t.Error("expected empty next_step for completed activity")
	}
	if out.PrePush == nil || !out.PrePush.Passed {
		t.Error("expected pre_push.passed = true")
	}
	if out.Push == nil || !out.Push.Success {
		t.Error("expected push.success = true")
	}
	if out.PostPush == nil || !out.PostPush.Passed {
		t.Error("expected post_push.passed = true")
	}
}

func TestHandleGitPushStatus_RunningActivity(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	// Create an activity that's still running
	act, err := activity.NewActivity([]string{"origin", "main"})
	if err != nil {
		t.Fatalf("NewActivity failed: %v", err)
	}
	act.StartPhase(activity.PhasePrePush)

	input := gitPushStatusInput{ActivityID: act.ID}
	_, out, err := handleGitPushStatus(context.Background(), nil, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Status != "running" {
		t.Errorf("expected status 'running', got %q", out.Status)
	}
	if out.NextStep == "" {
		t.Error("expected next_step for running activity")
	}
}

func TestHandleGitPushStatus_FailedActivity(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	act, err := activity.NewActivity([]string{"origin", "main"})
	if err != nil {
		t.Fatalf("NewActivity failed: %v", err)
	}
	act.StartPhase(activity.PhasePrePush)
	act.FailPhase(activity.PhasePrePush, "workflow denied")
	act.Complete(activity.StatusFailed, "Pre-push workflows denied the push")

	input := gitPushStatusInput{ActivityID: act.ID}
	_, out, err := handleGitPushStatus(context.Background(), nil, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Status != "failed" {
		t.Errorf("expected status 'failed', got %q", out.Status)
	}
	_ = os.RemoveAll(tmpDir)
}
