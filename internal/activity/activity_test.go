package activity

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewActivity(t *testing.T) {
	// Use temp dir for testing
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	a, err := NewActivity([]string{"origin", "main"})
	if err != nil {
		t.Fatalf("NewActivity failed: %v", err)
	}

	if a.ID == "" {
		t.Error("expected non-empty ID")
	}
	if a.Status != StatusRunning {
		t.Errorf("expected status %q, got %q", StatusRunning, a.Status)
	}
	if len(a.GitArgs) != 2 || a.GitArgs[0] != "origin" || a.GitArgs[1] != "main" {
		t.Errorf("unexpected git args: %v", a.GitArgs)
	}
	if len(a.Phases) != 3 {
		t.Errorf("expected 3 phases, got %d", len(a.Phases))
	}

	// Verify state file was written
	stateFile := filepath.Join(a.GetDir(), "state.json")
	if _, err := os.Stat(stateFile); err != nil {
		t.Errorf("state file not found: %v", err)
	}

	// Verify logs directory was created
	logsDir := filepath.Join(a.GetDir(), "logs")
	if _, err := os.Stat(logsDir); err != nil {
		t.Errorf("logs directory not found: %v", err)
	}
}

func TestLoadActivity(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	// Create an activity
	a, err := NewActivity([]string{"origin", "feature"})
	if err != nil {
		t.Fatalf("NewActivity failed: %v", err)
	}

	// Load it back
	loaded, err := LoadActivity(a.ID)
	if err != nil {
		t.Fatalf("LoadActivity failed: %v", err)
	}

	if loaded.ID != a.ID {
		t.Errorf("expected ID %q, got %q", a.ID, loaded.ID)
	}
	if loaded.Status != StatusRunning {
		t.Errorf("expected status %q, got %q", StatusRunning, loaded.Status)
	}
	if len(loaded.GitArgs) != 2 {
		t.Errorf("expected 2 git args, got %d", len(loaded.GitArgs))
	}
}

func TestLoadActivityNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	_, err := LoadActivity("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent activity")
	}
}

func TestPhaseLifecycle(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	a, err := NewActivity([]string{"origin", "main"})
	if err != nil {
		t.Fatalf("NewActivity failed: %v", err)
	}

	// Start pre-push phase
	a.StartPhase(PhasePrePush)
	if a.Phases[PhasePrePush].Status != StatusRunning {
		t.Errorf("expected pre_push status %q, got %q", StatusRunning, a.Phases[PhasePrePush].Status)
	}

	// Complete pre-push phase
	a.CompletePhase(PhasePrePush, true, "all checks passed")
	if a.Phases[PhasePrePush].Status != StatusCompleted {
		t.Errorf("expected pre_push status %q, got %q", StatusCompleted, a.Phases[PhasePrePush].Status)
	}

	// Fail push phase
	a.StartPhase(PhasePush)
	a.FailPhase(PhasePush, "push rejected")
	if a.Phases[PhasePush].Status != StatusFailed {
		t.Errorf("expected push status %q, got %q", StatusFailed, a.Phases[PhasePush].Status)
	}
	if a.Phases[PhasePush].Error != "push rejected" {
		t.Errorf("expected error %q, got %q", "push rejected", a.Phases[PhasePush].Error)
	}

	// Verify persisted state
	loaded, err := LoadActivity(a.ID)
	if err != nil {
		t.Fatalf("LoadActivity failed: %v", err)
	}
	if loaded.Phases[PhasePrePush].Status != StatusCompleted {
		t.Error("persisted pre_push should be completed")
	}
	if loaded.Phases[PhasePush].Status != StatusFailed {
		t.Error("persisted push should be failed")
	}
}

func TestAddWorkflowResult(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	a, err := NewActivity([]string{"origin", "main"})
	if err != nil {
		t.Fatalf("NewActivity failed: %v", err)
	}

	a.StartPhase(PhasePrePush)
	a.AddWorkflowResult(PhasePrePush, "lint-check", true, "")
	a.AddWorkflowResult(PhasePrePush, "test-check", false, "tests failed")

	phase := a.Phases[PhasePrePush]
	if len(phase.Workflows) != 2 {
		t.Fatalf("expected 2 workflows, got %d", len(phase.Workflows))
	}
	if !phase.Workflows[0].Success {
		t.Error("first workflow should be successful")
	}
	if phase.Workflows[1].Success {
		t.Error("second workflow should have failed")
	}
	if phase.Workflows[1].Error != "tests failed" {
		t.Errorf("expected error %q, got %q", "tests failed", phase.Workflows[1].Error)
	}
}

func TestComplete(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	a, err := NewActivity([]string{"origin", "main"})
	if err != nil {
		t.Fatalf("NewActivity failed: %v", err)
	}

	a.Complete(StatusCompleted, "All phases passed")
	if a.Status != StatusCompleted {
		t.Errorf("expected status %q, got %q", StatusCompleted, a.Status)
	}
	if a.Summary != "All phases passed" {
		t.Errorf("expected summary %q, got %q", "All phases passed", a.Summary)
	}
}

func TestWriteAndReadLogs(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	a, err := NewActivity([]string{"origin", "main"})
	if err != nil {
		t.Fatalf("NewActivity failed: %v", err)
	}

	// Write some logs
	if err := a.WriteLog(PhasePrePush, "lint", "Running lint...\nAll good!\n"); err != nil {
		t.Fatalf("WriteLog failed: %v", err)
	}
	if err := a.WriteLog(PhasePush, "git-push", "Pushing to origin...\n"); err != nil {
		t.Fatalf("WriteLog failed: %v", err)
	}

	// Read logs back
	logs, err := a.ReadLogs()
	if err != nil {
		t.Fatalf("ReadLogs failed: %v", err)
	}

	if len(logs) != 2 {
		t.Errorf("expected 2 log files, got %d", len(logs))
	}
}

func TestCleanupOldActivities(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	// Create an activity
	a, err := NewActivity([]string{"origin", "main"})
	if err != nil {
		t.Fatalf("NewActivity failed: %v", err)
	}

	// Make its state file old by modifying its mod time
	stateFile := filepath.Join(a.GetDir(), "state.json")
	oldTime := time.Now().Add(-8 * 24 * time.Hour)
	_ = os.Chtimes(stateFile, oldTime, oldTime)

	// Cleanup activities older than 7 days
	if err := CleanupOldActivities(7 * 24 * time.Hour); err != nil {
		t.Fatalf("CleanupOldActivities failed: %v", err)
	}

	// Activity should be gone
	_, err = LoadActivity(a.ID)
	if err == nil {
		t.Error("expected activity to be cleaned up")
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{"with space", "with_space"},
		{"with/slash", "with-slash"},
		{"with:colon", "with-colon"},
	}
	for _, tt := range tests {
		result := sanitizeFilename(tt.input)
		if result != tt.expected {
			t.Errorf("sanitizeFilename(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}
