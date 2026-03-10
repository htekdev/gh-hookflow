package e2e

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestLegacyRunFileEvent tests the legacy (non-raw) run path with a file event.
// Targets: runMatchingWorkflows (main.go:829), parseEventData file branch (main.go:927)
func TestLegacyRunFileEvent(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"file-check.yml": `name: File Check
on:
  file:
    paths: ["src/**/*.ts"]
steps:
  - name: check file
    run: Write-Host "file event received"
`,
	})

	// Build pre-parsed event JSON (the format parseEventData expects)
	eventData := map[string]interface{}{
		"file": map[string]interface{}{
			"path":   filepath.Join(workspace, "src", "app.ts"),
			"action": "edit",
		},
		"cwd": workspace,
	}
	eventJSON, _ := json.Marshal(eventData)

	sessionDir, _ := os.MkdirTemp("", "hookflow-e2e-legacy-*")
	t.Cleanup(func() { _ = os.RemoveAll(sessionDir) })

	out, _ := runHookflowCmd(t, []string{
		"run", "-e", string(eventJSON), "--event-type", "preToolUse", "--dir", workspace,
	}, []string{"HOOKFLOW_SESSION_DIR=" + sessionDir})

	// Should get a JSON result
	if !strings.Contains(out, "permissionDecision") {
		t.Errorf("Expected JSON result in output:\n%s", out)
	}
}

// TestLegacyRunCommitEvent tests the legacy run path with a commit event.
// Targets: parseEventData commit branch (main.go:927), commit files parsing
func TestLegacyRunCommitEvent(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"commit-check.yml": `name: Commit Check
on:
  commit:
    paths: ["**/*"]
steps:
  - name: check commit
    run: Write-Host "commit event received"
`,
	})

	eventData := map[string]interface{}{
		"commit": map[string]interface{}{
			"sha":     "abc123def",
			"message": "feat: add new feature",
			"author":  "developer@example.com",
			"files": []interface{}{
				map[string]interface{}{
					"path":   "src/feature.ts",
					"status": "added",
				},
				map[string]interface{}{
					"path":   "src/utils.ts",
					"status": "modified",
				},
			},
		},
		"cwd":       workspace,
		"timestamp": "2024-01-15T10:30:00Z",
	}
	eventJSON, _ := json.Marshal(eventData)

	sessionDir, _ := os.MkdirTemp("", "hookflow-e2e-legcommit-*")
	t.Cleanup(func() { _ = os.RemoveAll(sessionDir) })

	out, _ := runHookflowCmd(t, []string{
		"run", "-e", string(eventJSON), "--event-type", "preToolUse", "--dir", workspace,
	}, []string{"HOOKFLOW_SESSION_DIR=" + sessionDir})

	if !strings.Contains(out, "permissionDecision") {
		t.Errorf("Expected JSON result in output:\n%s", out)
	}
}

// TestLegacyRunPushEvent tests the legacy run path with a push event.
// Targets: parseEventData push branch (main.go:927)
func TestLegacyRunPushEvent(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"push-check.yml": `name: Push Check
on:
  push:
    branches: ["main", "release/*"]
steps:
  - name: check push
    run: Write-Host "push event received"
`,
	})

	eventData := map[string]interface{}{
		"push": map[string]interface{}{
			"ref":    "refs/heads/main",
			"before": "0000000",
			"after":  "abc123def",
		},
		"cwd": workspace,
	}
	eventJSON, _ := json.Marshal(eventData)

	sessionDir, _ := os.MkdirTemp("", "hookflow-e2e-legpush-*")
	t.Cleanup(func() { _ = os.RemoveAll(sessionDir) })

	out, _ := runHookflowCmd(t, []string{
		"run", "-e", string(eventJSON), "--event-type", "preToolUse", "--dir", workspace,
	}, []string{"HOOKFLOW_SESSION_DIR=" + sessionDir})

	if !strings.Contains(out, "permissionDecision") {
		t.Errorf("Expected JSON result in output:\n%s", out)
	}
}

// TestLegacyRunHookEvent tests the legacy run path with a hook event.
// Targets: parseEventData hook branch (main.go:927) including tool sub-object
func TestLegacyRunHookEvent(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"hook-check.yml": `name: Hook Check
on:
  hooks:
    types: ["preToolUse"]
    tools: ["edit"]
steps:
  - name: check hook
    run: Write-Host "hook event received"
`,
	})

	eventData := map[string]interface{}{
		"hook": map[string]interface{}{
			"type": "preToolUse",
			"cwd":  workspace,
			"tool": map[string]interface{}{
				"name": "edit",
				"args": map[string]interface{}{
					"path": "src/app.ts",
				},
			},
		},
		"tool": map[string]interface{}{
			"name": "edit",
			"args": map[string]interface{}{
				"path": "src/app.ts",
			},
		},
		"cwd": workspace,
	}
	eventJSON, _ := json.Marshal(eventData)

	sessionDir, _ := os.MkdirTemp("", "hookflow-e2e-leghook-*")
	t.Cleanup(func() { _ = os.RemoveAll(sessionDir) })

	out, _ := runHookflowCmd(t, []string{
		"run", "-e", string(eventJSON), "--event-type", "preToolUse", "--dir", workspace,
	}, []string{"HOOKFLOW_SESSION_DIR=" + sessionDir})

	if !strings.Contains(out, "permissionDecision") {
		t.Errorf("Expected JSON result in output:\n%s", out)
	}
}

// TestLegacyRunToolEvent tests the legacy run path with a tool event.
// Targets: parseEventData tool branch (main.go:927)
func TestLegacyRunToolEvent(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"tool-check.yml": `name: Tool Check
on:
  tool:
    - name: powershell
steps:
  - name: check tool
    run: Write-Host "tool event received"
`,
	})

	eventData := map[string]interface{}{
		"tool": map[string]interface{}{
			"name":      "powershell",
			"hook_type": "preToolUse",
			"args": map[string]interface{}{
				"command": "Get-Process",
			},
		},
		"cwd": workspace,
	}
	eventJSON, _ := json.Marshal(eventData)

	sessionDir, _ := os.MkdirTemp("", "hookflow-e2e-legtool-*")
	t.Cleanup(func() { _ = os.RemoveAll(sessionDir) })

	out, _ := runHookflowCmd(t, []string{
		"run", "-e", string(eventJSON), "--event-type", "preToolUse", "--dir", workspace,
	}, []string{"HOOKFLOW_SESSION_DIR=" + sessionDir})

	if !strings.Contains(out, "permissionDecision") {
		t.Errorf("Expected JSON result in output:\n%s", out)
	}
}

// TestLegacyRunNoEvent tests the legacy run path with empty event string.
// Targets: runMatchingWorkflows eventStr=="" early return (main.go:842)
func TestLegacyRunNoEvent(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"dummy.yml": `name: Dummy
on:
  file:
    paths: ["**/*"]
steps:
  - name: check
    run: Write-Host "ok"
`,
	})

	sessionDir, _ := os.MkdirTemp("", "hookflow-e2e-noev-*")
	t.Cleanup(func() { _ = os.RemoveAll(sessionDir) })

	// No -e flag → empty eventStr → allow by default
	out, _ := runHookflowCmd(t, []string{
		"run", "--event-type", "preToolUse", "--dir", workspace,
	}, []string{"HOOKFLOW_SESSION_DIR=" + sessionDir})

	if !strings.Contains(out, "allow") {
		t.Errorf("Expected allow for empty event:\n%s", out)
	}
}

// TestLegacyRunNoMatchingWorkflows tests when event doesn't match any workflow.
// Targets: runMatchingWorkflows no-match path (main.go:896)
func TestLegacyRunNoMatchingWorkflows(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"only-push.yml": `name: Only Push
on:
  push:
    branches: ["main"]
steps:
  - name: check
    run: Write-Host "ok"
`,
	})

	// Send a file event that won't match the push-only workflow
	eventData := map[string]interface{}{
		"file": map[string]interface{}{
			"path":   "README.md",
			"action": "edit",
		},
		"cwd": workspace,
	}
	eventJSON, _ := json.Marshal(eventData)

	sessionDir, _ := os.MkdirTemp("", "hookflow-e2e-nomatch-*")
	t.Cleanup(func() { _ = os.RemoveAll(sessionDir) })

	out, _ := runHookflowCmd(t, []string{
		"run", "-e", string(eventJSON), "--event-type", "preToolUse", "--dir", workspace,
	}, []string{"HOOKFLOW_SESSION_DIR=" + sessionDir})

	// No matching workflows → allow
	if !strings.Contains(out, "allow") {
		t.Errorf("Expected allow when no workflows match:\n%s", out)
	}
}

// TestLegacyRunDeniesOnStepFailure tests that a failing step in legacy mode produces deny.
// Targets: RunWithBlocking deny path (runner.go:192), runMatchingWorkflows deny path (main.go:911)
func TestLegacyRunDeniesOnStepFailure(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"fail-check.yml": `name: Fail Check
on:
  file:
    paths: ["**/*.env"]
steps:
  - name: deny env files
    run: |
      Write-Host "env files not allowed"
      exit 1
`,
	})

	eventData := map[string]interface{}{
		"file": map[string]interface{}{
			"path":   "secrets.env",
			"action": "create",
		},
		"cwd": workspace,
	}
	eventJSON, _ := json.Marshal(eventData)

	sessionDir, _ := os.MkdirTemp("", "hookflow-e2e-legdeny-*")
	t.Cleanup(func() { _ = os.RemoveAll(sessionDir) })

	out, _ := runHookflowCmd(t, []string{
		"run", "-e", string(eventJSON), "--event-type", "preToolUse", "--dir", workspace,
	}, []string{"HOOKFLOW_SESSION_DIR=" + sessionDir})

	if !strings.Contains(out, "deny") {
		t.Errorf("Expected deny for failing workflow:\n%s", out)
	}
}

// TestLegacyRunStdinInput tests reading event from stdin via -e "-".
// Targets: runMatchingWorkflows stdin branch (main.go:834)
func TestLegacyRunStdinInput(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"stdin-check.yml": `name: Stdin Check
on:
  file:
    paths: ["**/*"]
steps:
  - name: check
    run: Write-Host "stdin event"
`,
	})

	eventData := map[string]interface{}{
		"file": map[string]interface{}{
			"path":   "test.txt",
			"action": "create",
		},
		"cwd": workspace,
	}
	stdinJSON, _ := json.Marshal(eventData)

	sessionDir, _ := os.MkdirTemp("", "hookflow-e2e-stdin-*")
	t.Cleanup(func() { _ = os.RemoveAll(sessionDir) })

	// Use "-" for stdin reading
	out, err := runHookflowWithStdin(t, []string{
		"run", "-e", "-", "--event-type", "preToolUse", "--dir", workspace,
	}, string(stdinJSON), []string{"HOOKFLOW_SESSION_DIR=" + sessionDir})

	if err != nil {
		// Non-zero exit is OK if it's just a deny
		if !strings.Contains(out, "permissionDecision") {
			t.Errorf("Expected JSON result from stdin input:\n%s\nerr: %v", out, err)
		}
	}

	if !strings.Contains(out, "permissionDecision") {
		t.Errorf("Expected JSON result in output:\n%s", out)
	}
}

// TestTestCmdHookEvent tests the hookflow test command with hook event type.
// Targets: buildMockEvent "hook"/"tool" branch (test.go:170)
func TestTestCmdHookEvent(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"hook-test.yml": `name: Hook Test
on:
  hooks:
    types: ["preToolUse"]
steps:
  - name: check
    run: Write-Host "ok"
`,
	})

	sessionDir, _ := os.MkdirTemp("", "hookflow-e2e-testhook-*")
	t.Cleanup(func() { _ = os.RemoveAll(sessionDir) })

	out, _ := runHookflowCmd(t, []string{
		"test", "--event", "hook", "--path", "src/app.ts",
		"--workflow", "hook-test", "--dir", workspace,
	}, []string{"HOOKFLOW_SESSION_DIR=" + sessionDir})

	if !strings.Contains(out, "Testing with mock hook event") {
		t.Errorf("Expected hook event test output:\n%s", out)
	}
}

// TestTestCmdFileCreateEvent tests the hookflow test command with file create event.
// Targets: buildMockEvent "file" branch with action override (test.go:198)
func TestTestCmdFileCreateEvent(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"file-test.yml": `name: File Test
on:
  file:
    paths: ["**/*"]
steps:
  - name: check
    run: Write-Host "ok"
`,
	})

	sessionDir, _ := os.MkdirTemp("", "hookflow-e2e-testfile-*")
	t.Cleanup(func() { _ = os.RemoveAll(sessionDir) })

	out, _ := runHookflowCmd(t, []string{
		"test", "--event", "file", "--action", "create", "--path", "src/new.ts",
		"--workflow", "file-test", "--dir", workspace,
	}, []string{"HOOKFLOW_SESSION_DIR=" + sessionDir})

	if !strings.Contains(out, "Testing with mock file event") {
		t.Errorf("Expected file event test output:\n%s", out)
	}
}

// TestTestCmdCommitWithPathOverride tests commit event with custom path.
// Targets: buildMockEvent commit Path override (test.go:185)
func TestTestCmdCommitWithPathOverride(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"commit-paths.yml": `name: Commit Paths
on:
  commit:
    paths: ["docs/**/*"]
steps:
  - name: check
    run: Write-Host "ok"
`,
	})

	sessionDir, _ := os.MkdirTemp("", "hookflow-e2e-testcommitpath-*")
	t.Cleanup(func() { _ = os.RemoveAll(sessionDir) })

	out, _ := runHookflowCmd(t, []string{
		"test", "--event", "commit", "--path", "docs/README.md",
		"--workflow", "commit-paths", "--dir", workspace,
	}, []string{"HOOKFLOW_SESSION_DIR=" + sessionDir})

	if !strings.Contains(out, "Testing with mock commit event") {
		t.Errorf("Expected commit event test output:\n%s", out)
	}
	if !strings.Contains(out, "docs/README.md") {
		t.Errorf("Expected custom path in event output:\n%s", out)
	}
}

// TestTestCmdNonBlockingWorkflow tests the test command with a non-blocking workflow.
// Targets: runTest blocking display branch (test.go:153-157)
func TestTestCmdNonBlockingWorkflow(t *testing.T) {
	f := false
	_ = f // Prevent golangci-lint warning about unused var

	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"nonblocking.yml": `name: Non-Blocking
blocking: false
on:
  file:
    paths: ["**/*"]
steps:
  - name: notify
    run: Write-Host "notification"
`,
	})

	sessionDir, _ := os.MkdirTemp("", "hookflow-e2e-testnonblock-*")
	t.Cleanup(func() { _ = os.RemoveAll(sessionDir) })

	out, _ := runHookflowCmd(t, []string{
		"test", "--event", "file", "--path", "test.txt",
		"--workflow", "nonblocking", "--dir", workspace,
	}, []string{"HOOKFLOW_SESSION_DIR=" + sessionDir})

	if !strings.Contains(out, "Blocking: no") {
		t.Errorf("Expected 'Blocking: no' in output:\n%s", out)
	}
}

// TestTestCmdWorkflowLoadError tests what happens when the test command
// encounters an invalid workflow file.
// Targets: runTest LoadWorkflow error path (test.go:132-136)
func TestTestCmdWorkflowLoadError(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"invalid.yml": `this is not valid YAML: [`,
	})

	sessionDir, _ := os.MkdirTemp("", "hookflow-e2e-testloaderr-*")
	t.Cleanup(func() { _ = os.RemoveAll(sessionDir) })

	out, _ := runHookflowCmd(t, []string{
		"test", "--event", "file", "--path", "test.txt",
		"--workflow", "invalid", "--dir", workspace,
	}, []string{"HOOKFLOW_SESSION_DIR=" + sessionDir})

	// Should report error for the invalid workflow
	if !strings.Contains(out, "Error") && !strings.Contains(out, "error") {
		t.Errorf("Expected error report for invalid workflow:\n%s", out)
	}
}

// TestRunWorkflowDirectExecution tests `hookflow run --workflow <name>` path.
// Targets: runWorkflow (main.go:342) — loads and runs a specific workflow
func TestRunWorkflowDirectExecution(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"direct-run.yml": `name: Direct Run
on:
  file:
    paths: ["**/*"]
steps:
  - name: always pass
    run: Write-Host "direct execution"
`,
	})

	sessionDir, _ := os.MkdirTemp("", "hookflow-e2e-directrun-*")
	t.Cleanup(func() { _ = os.RemoveAll(sessionDir) })

	out, _ := runHookflowCmd(t, []string{
		"run", "--workflow", "direct-run", "--dir", workspace,
	}, []string{"HOOKFLOW_SESSION_DIR=" + sessionDir})

	if !strings.Contains(out, "permissionDecision") {
		t.Errorf("Expected JSON result from direct workflow run:\n%s", out)
	}
}

// TestRunWorkflowMissing tests `hookflow run --workflow nonexistent` error path.
// Targets: runWorkflow not-found path (main.go:345)
func TestRunWorkflowMissing(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"exists.yml": `name: Exists
on:
  file:
    paths: ["**/*"]
steps:
  - name: ok
    run: Write-Host "ok"
`,
	})

	sessionDir, _ := os.MkdirTemp("", "hookflow-e2e-missingwf-*")
	t.Cleanup(func() { _ = os.RemoveAll(sessionDir) })

	out, err := runHookflowCmd(t, []string{
		"run", "--workflow", "nonexistent", "--dir", workspace,
	}, []string{"HOOKFLOW_SESSION_DIR=" + sessionDir})

	if err == nil {
		t.Errorf("Expected error for missing workflow, got success:\n%s", out)
	}
	if !strings.Contains(out, "not found") {
		t.Errorf("Expected 'not found' error:\n%s", out)
	}
}

// runHookflowWithStdin executes the hookflow binary with stdin piped.
func runHookflowWithStdin(t *testing.T, args []string, stdin string, env []string) (string, error) {
	t.Helper()

	safeName := strings.ReplaceAll(t.Name(), "/", "_")
	safeName = strings.ReplaceAll(safeName, "\\", "_")
	coverSubDir := filepath.Join(globalCoverDir, safeName)
	_ = os.MkdirAll(coverSubDir, 0755)

	cmd := newCoverCmd(t, args, coverSubDir)
	cmd.Env = append(cmd.Env, env...)
	cmd.Stdin = strings.NewReader(stdin)

	out, err := cmd.CombinedOutput()
	return string(out), err
}

// newCoverCmd builds an exec.Cmd for the coverage-instrumented binary.
func newCoverCmd(t *testing.T, args []string, coverDir string) *exec.Cmd {
	t.Helper()

	cmd := exec.Command(binaryPath, args...)
	cmd.Env = append(os.Environ(), "GOCOVERDIR="+coverDir)
	return cmd
}
