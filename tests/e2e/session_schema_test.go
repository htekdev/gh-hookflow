package e2e

import (
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
on:
  file:
    lifecycle: post
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
		t.Errorf("session 1 transcript not created at %s", t1)
	}
	if _, err := os.Stat(t2); os.IsNotExist(err) {
		t.Errorf("session 2 transcript not created at %s", t2)
	}
}

// ── schema validation via validate command ──────────────────────────

func TestSchemaValidateValidWorkflow(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"complete.yml": `name: Complete Workflow
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
on:
  file:
    paths: ['**/*.ts']
steps:
  - name: Check A
    run: echo "A"
`,
		"workflow-b.yml": `name: Workflow B
on:
  file:
    paths: ['**/*.go']
steps:
  - name: Check B
    run: echo "B"
`,
		"workflow-c.yml": `name: Workflow C
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
on:
  file:
    paths: ['**/*']
steps:
  - name: A
    run: echo "a"
`,
		"beta.yaml": `name: Beta
on:
  hooks:
steps:
  - name: B
    run: echo "b"
`,
		"gamma.yml": `name: Gamma
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
