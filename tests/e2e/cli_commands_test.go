package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// ── init command tests ──────────────────────────────────────────────

func TestInitCreatesHooksJSON(t *testing.T) {
	workspace := t.TempDir()

	output, err := runHookflowCmd(t, []string{"init", "--dir", workspace}, nil)
	if err != nil {
		t.Fatalf("init failed: %v\n%s", err, output)
	}

	hooksFile := filepath.Join(workspace, ".github", "hooks", "hooks.json")
	data, err := os.ReadFile(hooksFile)
	if err != nil {
		t.Fatalf("hooks.json not created: %v", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatalf("hooks.json is not valid JSON: %v", err)
	}

	hooks, ok := config["hooks"].(map[string]interface{})
	if !ok {
		t.Fatal("hooks.json missing 'hooks' key")
	}

	for _, key := range []string{"preToolUse", "postToolUse", "sessionStart"} {
		if _, ok := hooks[key]; !ok {
			t.Errorf("hooks.json missing %q hook", key)
		}
	}

	if !strings.Contains(output, "initialized successfully") {
		t.Errorf("expected success message, got: %s", output)
	}
}

func TestInitWithRepoFlag(t *testing.T) {
	workspace := t.TempDir()

	output, err := runHookflowCmd(t, []string{"init", "--repo", "--dir", workspace}, nil)
	if err != nil {
		t.Fatalf("init --repo failed: %v\n%s", err, output)
	}

	examplePath := filepath.Join(workspace, ".github", "hookflows", "example.yml")
	if _, err := os.Stat(examplePath); os.IsNotExist(err) {
		t.Fatal("example.yml not created with --repo flag")
	}

	if !strings.Contains(output, "Next steps") {
		t.Errorf("expected next steps in output, got: %s", output)
	}
}

func TestInitForce(t *testing.T) {
	workspace := t.TempDir()

	// First init
	_, _ = runHookflowCmd(t, []string{"init", "--dir", workspace}, nil)

	// Modify the hooks.json
	hooksFile := filepath.Join(workspace, ".github", "hooks", "hooks.json")
	_ = os.WriteFile(hooksFile, []byte(`{"version":1,"hooks":{"preToolUse":[{"type":"command","bash":"echo custom"}]}}`), 0644)

	// Force init
	output, err := runHookflowCmd(t, []string{"init", "--force", "--dir", workspace}, nil)
	if err != nil {
		t.Fatalf("init --force failed: %v\n%s", err, output)
	}

	data, _ := os.ReadFile(hooksFile)
	if !strings.Contains(string(data), "hookflow") {
		t.Error("force init should restore hookflow hooks")
	}
}

func TestInitIdempotent(t *testing.T) {
	workspace := t.TempDir()

	// Run init twice
	_, _ = runHookflowCmd(t, []string{"init", "--dir", workspace}, nil)
	output, err := runHookflowCmd(t, []string{"init", "--dir", workspace}, nil)
	if err != nil {
		t.Fatalf("second init failed: %v\n%s", err, output)
	}

	if !strings.Contains(output, "initialized successfully") {
		t.Errorf("expected success on second init, got: %s", output)
	}
}

// ── validate command tests ──────────────────────────────────────────

func TestValidateValidWorkflows(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"valid.yml": `name: Valid Workflow
on:
  file:
    paths: ['**/*.env']
blocking: true
steps:
  - name: Block env files
    run: echo "blocked" && exit 1
`,
	})

	output, err := runHookflowCmd(t, []string{"validate", "--dir", workspace}, nil)
	if err != nil {
		t.Fatalf("validate failed for valid workflow: %v\n%s", err, output)
	}
}

func TestValidateInvalidWorkflow(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"invalid.yml": `name: Missing Steps
on:
  file:
    paths: ['*.ts']
# deliberately missing steps
`,
	})

	output, err := runHookflowCmd(t, []string{"validate", "--dir", workspace}, nil)
	// Validation of an incomplete workflow may or may not error; check output
	_ = err
	// The validate command should run and produce output
	if len(output) == 0 {
		t.Error("validate should produce output")
	}
}

func TestValidateSpecificFile(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"check-me.yml": `name: Check Me
on:
  file:
    paths: ['**/*.ts']
blocking: true
steps:
  - name: Lint
    run: echo "linting"
`,
	})

	filePath := filepath.Join(workspace, ".github", "hookflows", "check-me.yml")
	output, err := runHookflowCmd(t, []string{"validate", "--file", filePath}, nil)
	if err != nil {
		t.Fatalf("validate --file failed: %v\n%s", err, output)
	}
}

func TestValidateEmptyDir(t *testing.T) {
	workspace := t.TempDir()
	// No .github/hookflows directory at all
	output, err := runHookflowCmd(t, []string{"validate", "--dir", workspace}, nil)
	if err != nil {
		t.Fatalf("validate on empty dir should not crash: %v\n%s", err, output)
	}
	if strings.Contains(output, "panic") {
		t.Errorf("validate on empty dir should not panic, got: %s", output)
	}
}

// ── discover command tests are in discover_test.go ──────────────────

// ── logs command tests ──────────────────────────────────────────────

func TestLogsPath(t *testing.T) {
	output, err := runHookflowCmd(t, []string{"logs", "--path"}, nil)
	if err != nil {
		t.Fatalf("logs --path failed: %v\n%s", err, output)
	}

	if !strings.Contains(output, "hookflow") {
		t.Errorf("expected hookflow in log path, got: %s", output)
	}
}

func TestLogsTail(t *testing.T) {
	// First run a hookflow command to generate some logs
	workspace := setupWorkspace(t)
	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "test.txt"),
		"file_text": "hello",
	}, workspace)
	_, _ = runHookflow(t, workspace, eventJSON, "preToolUse", nil)

	// Now check logs
	output, err := runHookflowCmd(t, []string{"logs", "-n", "10"}, nil)
	if err != nil {
		t.Errorf("logs -n 10 should succeed: %v\n%s", err, output)
	}
}

func TestLogsDebugMode(t *testing.T) {
	workspace := setupWorkspace(t)
	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "test.txt"),
		"file_text": "hello",
	}, workspace)

	// Run with debug enabled
	opts := &hookflowOpts{
		env: []string{"HOOKFLOW_DEBUG=1"},
	}
	_, _ = runHookflow(t, workspace, eventJSON, "preToolUse", opts)

	// Verify debug logging happened by checking log file
	output, err := runHookflowCmd(t, []string{"logs", "--path"}, nil)
	if err != nil {
		t.Fatalf("logs --path failed: %v\n%s", err, output)
	}
	logPath := strings.TrimSpace(output)
	if logPath == "" {
		t.Fatal("expected non-empty log path when debug mode is enabled")
	}
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file %s: %v", logPath, err)
	}
	if len(data) == 0 {
		t.Errorf("expected log file to have content with debug enabled, but it was empty")
	}
}

// ── version command tests ───────────────────────────────────────────

func TestVersionCommand(t *testing.T) {
	output, err := runHookflowCmd(t, []string{"version"}, nil)
	if err != nil {
		t.Fatalf("version failed: %v\n%s", err, output)
	}

	if len(output) == 0 {
		t.Error("version should produce output")
	}
}

// ── triggers command tests ──────────────────────────────────────────

func TestTriggersCommand(t *testing.T) {
	output, err := runHookflowCmd(t, []string{"triggers"}, nil)
	if err != nil {
		t.Fatalf("triggers failed: %v\n%s", err, output)
	}

	// Should list trigger types
	for _, expected := range []string{"file", "hooks"} {
		if !strings.Contains(strings.ToLower(output), expected) {
			t.Errorf("triggers output should mention %q, got: %s", expected, output)
		}
	}
}

// ── check-setup command tests ───────────────────────────────────────

func TestCheckSetupCommand(t *testing.T) {
	output, err := runHookflowCmd(t, []string{"check-setup"}, nil)
	// check-setup may fail if gh hookflow isn't installed via gh extension
	// but the command itself should run
	_ = err
	if len(output) == 0 {
		t.Error("check-setup should produce output")
	}
}

// ── test command tests ──────────────────────────────────────────────

func TestTestCommandFileEvent(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"block-env.yml": `name: Block Env Files
on:
  file:
    paths: ['**/*.env']
blocking: true
steps:
  - name: Block
    run: |
      echo "Blocked env file"
      exit 1
`,
	})

	output, err := runHookflowCmd(t, []string{
		"test",
		"-e", "file",
		"--path", "secrets.env",
		"--action", "create",
		"--dir", workspace,
	}, nil)
	if err != nil {
		t.Fatalf("test command failed: %v\n%s", err, output)
	}
	// The test command should show matching workflows
	if !strings.Contains(strings.ToLower(output), "block") || !strings.Contains(strings.ToLower(output), "env") {
		t.Errorf("expected test output to reference block env workflow, got: %s", output)
	}
}

func TestTestCommandCommitEvent(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"commit-lint.yml": `name: Commit Lint
on:
  git_commit:
blocking: true
steps:
  - name: Check format
    run: echo "checking"
`,
	})

	output, err := runHookflowCmd(t, []string{
		"test",
		"-e", "git_commit",
		"--message", "fix: something",
		"--dir", workspace,
	}, nil)
	if err != nil {
		t.Fatalf("test command for git_commit should succeed: %v\n%s", err, output)
	}
}

func TestTestCommandPushEvent(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"pre-push.yml": `name: Pre Push
on:
  push:
    branches: [main]
blocking: true
steps:
  - name: Check
    run: echo "checking"
`,
	})

	output, err := runHookflowCmd(t, []string{
		"test",
		"-e", "push",
		"--branch", "main",
		"--dir", workspace,
	}, nil)
	if err != nil {
		t.Fatalf("test command for push should succeed: %v\n%s", err, output)
	}
}

// ── git-push-status tests ───────────────────────────────────────────

func TestGitPushStatusNotFound(t *testing.T) {
	output, err := runHookflowCmd(t, []string{"git-push-status", "nonexistent-id-12345"}, nil)
	if err == nil {
		t.Fatal("expected error for nonexistent activity")
	}
	if !strings.Contains(output, "not found") && !strings.Contains(strings.ToLower(output), "error") {
		t.Errorf("expected 'not found' error, got: %s", output)
	}
}

func TestGitPushStatusCompleted(t *testing.T) {
	// Create a completed activity on disk
	activityID := "e2e-status-completed"
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
					map[string]interface{}{"name": "test-workflow", "status": "completed"},
				},
			},
			"push": map[string]interface{}{
				"status": "completed",
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

	output, err := runHookflowCmd(t, []string{"git-push-status", activityID}, nil)
	if err != nil {
		t.Fatalf("git-push-status failed: %v\n%s", err, output)
	}
	if !strings.Contains(output, "completed") && !strings.Contains(output, "success") {
		t.Errorf("expected completed status, got: %s", output)
	}
}

func TestGitPushStatusFailed(t *testing.T) {
	activityID := "e2e-status-failed"
	actDir := filepath.Join(homeDir(), ".hookflow", "activities", activityID)
	_ = os.MkdirAll(filepath.Join(actDir, "logs"), 0755)
	defer func() { _ = os.RemoveAll(actDir) }()

	// Write a log file for the failed phase
	_ = os.WriteFile(filepath.Join(actDir, "logs", "pre-push-test-workflow.log"),
		[]byte("Running tests...\nTest failed: expected 5 got 3\n"), 0644)

	state := map[string]interface{}{
		"id":      activityID,
		"status":  "failed",
		"gitArgs": []string{"origin", "main"},
		"phases": map[string]interface{}{
			"pre-push": map[string]interface{}{
				"status": "failed",
				"error":  "workflow denied push",
				"workflows": []interface{}{
					map[string]interface{}{"name": "test-workflow", "status": "failed", "error": "tests failed"},
				},
			},
		},
	}
	stateJSON, _ := json.MarshalIndent(state, "", "  ")
	_ = os.WriteFile(filepath.Join(actDir, "state.json"), stateJSON, 0644)

	output, err := runHookflowCmd(t, []string{"git-push-status", activityID}, nil)
	_ = err // status of failed push may or may not return error
	if !strings.Contains(strings.ToLower(output), "fail") {
		t.Errorf("expected failure message, got: %s", output)
	}
}

func TestGitPushStatusRunning(t *testing.T) {
	activityID := "e2e-status-running"
	actDir := filepath.Join(homeDir(), ".hookflow", "activities", activityID)
	_ = os.MkdirAll(filepath.Join(actDir, "logs"), 0755)
	defer func() { _ = os.RemoveAll(actDir) }()

	state := map[string]interface{}{
		"id":      activityID,
		"status":  "running",
		"gitArgs": []string{"origin", "main"},
		"phases": map[string]interface{}{
			"pre-push": map[string]interface{}{
				"status": "running",
			},
		},
	}
	stateJSON, _ := json.MarshalIndent(state, "", "  ")
	_ = os.WriteFile(filepath.Join(actDir, "state.json"), stateJSON, 0644)

	output, err := runHookflowCmd(t, []string{"git-push-status", activityID}, nil)
	if err != nil {
		t.Fatalf("git-push-status failed: %v\n%s", err, output)
	}
	if !strings.Contains(strings.ToLower(output), "progress") && !strings.Contains(strings.ToLower(output), "running") {
		t.Errorf("expected in-progress message, got: %s", output)
	}
}

// ── helper ──────────────────────────────────────────────────────────

func homeDir() string {
	if runtime.GOOS == "windows" {
		return os.Getenv("USERPROFILE")
	}
	return os.Getenv("HOME")
}
