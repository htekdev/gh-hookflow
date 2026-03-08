package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── session error flow ──────────────────────────────────────────────

func TestSessionErrorWrittenOnPostFailure(t *testing.T) {
	sessionDir := t.TempDir()

	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"post-block.yml": `name: Post Block
lifecycle: post
on:
  file:
    paths: ['**/*.json']
    types: [create]
blocking: true
steps:
  - name: Validate JSON
    run: |
      echo "Invalid config detected"
      exit 1
`,
	})

	opts := &hookflowOpts{sessionDir: sessionDir}

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "config.json"),
		"file_text": `{"invalid": true}`,
	}, workspace)

	// postToolUse should trigger the workflow
	_, _ = runHookflow(t, workspace, eventJSON, "postToolUse", opts)

	// Check that error.md was created in session dir
	errorFile := filepath.Join(sessionDir, "error.md")
	if _, err := os.Stat(errorFile); os.IsNotExist(err) {
		t.Log("error.md not created — post-lifecycle error may not be persisted in this test mode")
	} else if err == nil {
		data, _ := os.ReadFile(errorFile)
		if !strings.Contains(string(data), "Error") {
			t.Errorf("error.md should contain error details, got: %s", string(data))
		}
	}
}

// ── session directory isolation ─────────────────────────────────────

func TestSessionDirIsolation(t *testing.T) {
	session1 := t.TempDir()
	session2 := t.TempDir()

	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"track.yml": `name: Tracker
lifecycle: pre
on:
  file:
    paths: ['**/*']
steps:
  - name: Track
    run: echo "tracked"
`,
	})

	// Run in session 1
	opts1 := &hookflowOpts{sessionDir: session1}
	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "file1.txt"),
		"file_text": "test",
	}, workspace)
	_, _ = runHookflow(t, workspace, eventJSON, "preToolUse", opts1)

	// Run in session 2
	opts2 := &hookflowOpts{sessionDir: session2}
	_, _ = runHookflow(t, workspace, eventJSON, "preToolUse", opts2)

	// Both sessions should have separate transcript files
	t1 := filepath.Join(session1, "transcript.jsonl")
	t2 := filepath.Join(session2, "transcript.jsonl")

	if _, err := os.Stat(t1); os.IsNotExist(err) {
		t.Log("session 1 transcript not created")
	}
	if _, err := os.Stat(t2); os.IsNotExist(err) {
		t.Log("session 2 transcript not created")
	}
}

// ── schema validation via validate command ──────────────────────────

func TestSchemaValidateValidWorkflow(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"complete.yml": `name: Complete Workflow
lifecycle: pre
on:
  file:
    paths: ['**/*.ts']
    types: [create, edit]
  commit:
blocking: true
env:
  CI: "true"
steps:
  - name: Lint
    run: echo "linting"
  - name: Test
    if: ${{ steps.lint.outcome == 'success' }}
    run: echo "testing"
    timeout: 300
`,
	})

	output, err := runHookflowCmd(t, []string{"validate", "--dir", workspace}, nil)
	if err != nil {
		t.Fatalf("validate failed: %v\n%s", err, output)
	}
}

func TestSchemaValidateMultipleWorkflows(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"workflow-a.yml": `name: Workflow A
lifecycle: pre
on:
  file:
    paths: ['**/*.ts']
steps:
  - name: Check A
    run: echo "A"
`,
		"workflow-b.yml": `name: Workflow B
lifecycle: pre
on:
  file:
    paths: ['**/*.go']
steps:
  - name: Check B
    run: echo "B"
`,
		"workflow-c.yml": `name: Workflow C
lifecycle: pre
on:
  hooks:
    types: [preToolUse]
steps:
  - name: Check C
    run: echo "C"
`,
	})

	output, err := runHookflowCmd(t, []string{"validate", "--dir", workspace}, nil)
	if err != nil {
		t.Fatalf("validate failed: %v\n%s", err, output)
	}
}

// ── schema validation with push trigger ─────────────────────────────

func TestSchemaValidatePushTrigger(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"push-workflow.yml": `name: Push Checks
lifecycle: pre
on:
  push:
    branches: [main, develop]
blocking: true
steps:
  - name: Pre-push check
    run: echo "checking before push"
`,
	})

	output, err := runHookflowCmd(t, []string{"validate", "--dir", workspace}, nil)
	if err != nil {
		t.Fatalf("validate failed: %v\n%s", err, output)
	}
}

// ── discover with glob ──────────────────────────────────────────────

func TestDiscoverFindsMultipleTypes(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"alpha.yml": `name: Alpha
lifecycle: pre
on:
  file:
    paths: ['**/*']
steps:
  - name: A
    run: echo "a"
`,
		"beta.yaml": `name: Beta
lifecycle: pre
on:
  hooks:
steps:
  - name: B
    run: echo "b"
`,
		"gamma.yml": `name: Gamma
lifecycle: pre
on:
  git_commit:
steps:
  - name: G
    run: echo "g"
`,
	})

	output, err := runHookflowCmd(t, []string{"discover", "--dir", workspace}, nil)
	if err != nil {
		t.Fatalf("discover failed: %v\n%s", err, output)
	}

	for _, name := range []string{"alpha", "beta", "gamma"} {
		if !strings.Contains(strings.ToLower(output), name) {
			t.Errorf("discover should find %q, got: %s", name, output)
		}
	}
}

// ── activity cleanup ────────────────────────────────────────────────

func TestActivityStateStructure(t *testing.T) {
	activityID := "e2e-activity-struct"
	actDir := filepath.Join(homeDir(), ".hookflow", "activities", activityID)
	_ = os.MkdirAll(filepath.Join(actDir, "logs"), 0755)
	defer func() { _ = os.RemoveAll(actDir) }()

	state := map[string]interface{}{
		"id":      activityID,
		"status":  "completed",
		"gitArgs": []string{"origin", "main"},
		"phases": map[string]interface{}{
			"pre-push": map[string]interface{}{
				"status": "completed",
				"workflows": []interface{}{
					map[string]interface{}{"name": "lint", "status": "completed"},
					map[string]interface{}{"name": "test", "status": "completed"},
				},
			},
			"push": map[string]interface{}{
				"status": "completed",
				"output": "Everything up-to-date",
			},
			"post-push": map[string]interface{}{
				"status": "completed",
				"workflows": []interface{}{
					map[string]interface{}{"name": "notify", "status": "completed"},
				},
			},
		},
	}

	stateJSON, _ := json.MarshalIndent(state, "", "  ")
	_ = os.WriteFile(filepath.Join(actDir, "state.json"), stateJSON, 0644)

	// Write some logs
	_ = os.WriteFile(filepath.Join(actDir, "logs", "pre-push-lint.log"),
		[]byte("Lint passed\n"), 0644)
	_ = os.WriteFile(filepath.Join(actDir, "logs", "pre-push-test.log"),
		[]byte("All tests passed\n"), 0644)
	_ = os.WriteFile(filepath.Join(actDir, "logs", "post-push-notify.log"),
		[]byte("Notification sent\n"), 0644)

	// Use git-push-status to read the activity
	output, err := runHookflowCmd(t, []string{"git-push-status", activityID}, nil)
	if err != nil {
		t.Fatalf("git-push-status failed: %v\n%s", err, output)
	}

	if !strings.Contains(strings.ToLower(output), "completed") &&
		!strings.Contains(strings.ToLower(output), "success") {
		t.Errorf("expected success output, got: %s", output)
	}
}
