package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestPushBackgroundSuccessful tests the full 3-phase push flow with no workflows.
// With no push-trigger workflows, the push should succeed.
func TestPushBackgroundSuccessful(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{})

	actID := createTestActivity(t, []string{"origin", "main"})

	output, err := runHookflowCmd(t,
		[]string{"git-push", "--background", "--dir", workspace, "origin", "main"},
		[]string{
			"HOOKFLOW_PUSH_ACTIVITY_ID=" + actID,
			"HOOKFLOW_FAKE_GIT_BRANCH=main",
			"HOOKFLOW_FAKE_GIT_PUSH_OUTPUT=Everything up-to-date",
		},
	)
	if err != nil {
		t.Fatalf("git-push --background failed: %v\nOutput: %s", err, output)
	}

	state := loadActivityState(t, actID)
	if state.Status != "completed" {
		t.Errorf("expected status completed, got %s", state.Status)
	}
}

// TestPushBackgroundPrePushDenies tests that a pre-push workflow denial prevents the push.
func TestPushBackgroundPrePushDenies(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"block-push.yml": `name: Block Push
on:
  push:
    branches:
      - main
blocking: true
steps:
  - name: Deny push
    run: |
      Write-Output "Push denied by policy"
      exit 1
`,
	})

	actID := createTestActivity(t, []string{"origin", "main"})

	_, _ = runHookflowCmd(t,
		[]string{"git-push", "--background", "--dir", workspace, "origin", "main"},
		[]string{
			"HOOKFLOW_PUSH_ACTIVITY_ID=" + actID,
			"HOOKFLOW_FAKE_GIT_BRANCH=main",
		},
	)

	state := loadActivityState(t, actID)
	if state.Status != "failed" {
		t.Errorf("expected status failed, got %s", state.Status)
	}
	if state.Summary == "" {
		t.Error("expected non-empty summary on failure")
	}
}

// TestPushBackgroundGitFailure tests behavior when the git push itself fails.
func TestPushBackgroundGitFailure(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{})

	actID := createTestActivity(t, []string{"origin", "main"})

	_, _ = runHookflowCmd(t,
		[]string{"git-push", "--background", "--dir", workspace, "origin", "main"},
		[]string{
			"HOOKFLOW_PUSH_ACTIVITY_ID=" + actID,
			"HOOKFLOW_FAKE_GIT_BRANCH=main",
			"HOOKFLOW_FAKE_GIT_PUSH_FAIL=1",
			"HOOKFLOW_FAKE_GIT_PUSH_ERROR=remote rejected: permission denied",
		},
	)

	state := loadActivityState(t, actID)
	if state.Status != "failed" {
		t.Errorf("expected status failed, got %s", state.Status)
	}
	if !strings.Contains(state.Summary, "push failed") && !strings.Contains(state.Summary, "Git push failed") {
		t.Errorf("expected summary to mention push failure, got: %s", state.Summary)
	}
}

// TestPushBackgroundWithPostPush tests the full 3-phase flow with post-push workflows.
func TestPushBackgroundWithPostPush(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"post-push-notify.yml": `name: Post Push Notify
on:
  push:
    lifecycle: post
blocking: false
steps:
  - name: Notify
    run: |
      Write-Output "Push completed, notifying team"
`,
	})

	actID := createTestActivity(t, []string{"origin", "main"})

	output, err := runHookflowCmd(t,
		[]string{"git-push", "--background", "--dir", workspace, "origin", "main"},
		[]string{
			"HOOKFLOW_PUSH_ACTIVITY_ID=" + actID,
			"HOOKFLOW_FAKE_GIT_BRANCH=main",
			"HOOKFLOW_FAKE_GIT_PUSH_OUTPUT=To github.com:test/repo.git\n   abc1234..def5678  main -> main",
		},
	)
	if err != nil {
		t.Fatalf("git-push --background failed: %v\nOutput: %s", err, output)
	}

	state := loadActivityState(t, actID)
	if state.Status != "completed" {
		t.Errorf("expected status completed, got %s (summary: %s)", state.Status, state.Summary)
	}
}

// TestPushBackgroundPostPushFailure tests that post-push failure is recorded correctly.
func TestPushBackgroundPostPushFailure(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"post-push-check.yml": `name: Post Push Check
on:
  push:
    lifecycle: post
blocking: true
steps:
  - name: Check CI
    run: |
      Write-Output "CI check failed"
      exit 1
`,
	})

	actID := createTestActivity(t, []string{"origin", "main"})

	_, _ = runHookflowCmd(t,
		[]string{"git-push", "--background", "--dir", workspace, "origin", "main"},
		[]string{
			"HOOKFLOW_PUSH_ACTIVITY_ID=" + actID,
			"HOOKFLOW_FAKE_GIT_BRANCH=main",
			"HOOKFLOW_FAKE_GIT_PUSH_OUTPUT=Everything up-to-date",
		},
	)

	state := loadActivityState(t, actID)
	if state.Status != "failed" {
		t.Errorf("expected status failed, got %s", state.Status)
	}
	if !strings.Contains(state.Summary, "post-push") {
		t.Errorf("expected summary to mention post-push, got: %s", state.Summary)
	}
}

// TestPushStatusCommand tests the git-push-status subcommand.
func TestPushStatusCommand(t *testing.T) {
	actID := createTestActivity(t, []string{"origin", "main"})

	output, err := runHookflowCmd(t,
		[]string{"git-push-status", actID},
		nil,
	)
	if err != nil {
		t.Fatalf("git-push-status failed: %v\nOutput: %s", err, output)
	}

	// Activity should be in running state since background hasn't run
	if !strings.Contains(output, "progress") && !strings.Contains(output, "running") &&
		!strings.Contains(output, "pending") && !strings.Contains(output, actID) {
		t.Errorf("expected status output to reference activity state, got: %s", output)
	}
}

// TestPushActivityPhaseTracking verifies all 3 phases are recorded in the activity state.
func TestPushActivityPhaseTracking(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{})

	actID := createTestActivity(t, []string{"origin", "main"})

	_, _ = runHookflowCmd(t,
		[]string{"git-push", "--background", "--dir", workspace, "origin", "main"},
		[]string{
			"HOOKFLOW_PUSH_ACTIVITY_ID=" + actID,
			"HOOKFLOW_FAKE_GIT_BRANCH=main",
			"HOOKFLOW_FAKE_GIT_PUSH_OUTPUT=Everything up-to-date",
		},
	)

	state := loadActivityState(t, actID)

	// Check all 3 phases exist
	phases := []string{"pre_push", "push", "post_push"}
	for _, phase := range phases {
		ps, ok := state.Phases[phase]
		if !ok {
			t.Errorf("missing phase %s in activity state", phase)
			continue
		}
		if ps.Status != "completed" {
			t.Errorf("expected phase %s to be completed, got %s", phase, ps.Status)
		}
	}
}

// --- Activity helpers ---

type activityState struct {
	ID      string                        `json:"id"`
	Status  string                        `json:"status"`
	GitArgs []string                      `json:"git_args"`
	Summary string                        `json:"summary"`
	Phases  map[string]*activityPhaseJSON `json:"phases"`
}

type activityPhaseJSON struct {
	Status string `json:"status"`
	Output string `json:"output"`
	Error  string `json:"error"`
}

func createTestActivity(t *testing.T, gitArgs []string) string {
	t.Helper()

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("Failed to get home dir: %v", err)
	}

	id := "e2etest-" + time.Now().Format("150405.000")
	id = strings.ReplaceAll(id, ".", "")
	actDir := filepath.Join(home, ".hookflow", "activities", id)
	logsDir := filepath.Join(actDir, "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		t.Fatalf("Failed to create activity dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(actDir) })

	now := time.Now().UTC().Format(time.RFC3339)
	state := map[string]interface{}{
		"id":         id,
		"status":     "running",
		"git_args":   gitArgs,
		"created_at": now,
		"updated_at": now,
		"phases": map[string]interface{}{
			"pre_push":  map[string]interface{}{"status": "pending"},
			"push":      map[string]interface{}{"status": "pending"},
			"post_push": map[string]interface{}{"status": "pending"},
		},
	}
	data, _ := json.MarshalIndent(state, "", "  ")
	if err := os.WriteFile(filepath.Join(actDir, "state.json"), data, 0644); err != nil {
		t.Fatalf("Failed to write activity state: %v", err)
	}

	return id
}

func loadActivityState(t *testing.T, actID string) *activityState {
	t.Helper()

	home, _ := os.UserHomeDir()
	statePath := filepath.Join(home, ".hookflow", "activities", actID, "state.json")

	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("Failed to read activity state: %v", err)
	}

	var state activityState
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatalf("Failed to parse activity state: %v\n%s", err, data)
	}
	return &state
}
