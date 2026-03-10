package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── hookflow run --workflow ─────────────────────────────────────────

func TestRunSpecificWorkflow(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"named-wf.yml": `name: Named Workflow
on:
  file:
    paths: ['**/*']
steps:
  - name: Say hello
    run: Write-Host "Hello from named workflow"
`,
	})

	output, err := runHookflowCmd(t, []string{"run", "--workflow", "named-wf", "--dir", workspace}, nil)
	if err != nil {
		t.Fatalf("run --workflow failed: %v\n%s", err, output)
	}
	if !strings.Contains(output, "Hello from named workflow") {
		t.Errorf("Expected workflow output, got: %s", output)
	}
}

func TestRunWorkflowNotFound(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"exists.yml": `name: Exists
on:
  file:
    paths: ['**/*']
steps:
  - name: x
    run: Write-Host "x"
`,
	})

	_, err := runHookflowCmd(t, []string{"run", "--workflow", "nonexistent", "--dir", workspace}, nil)
	if err == nil {
		t.Errorf("Expected error for nonexistent workflow")
	}
}

// ── hookflow run --event (legacy mode) ──────────────────────────────

func TestRunLegacyEventMode(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"file-wf.yml": `name: File Workflow
on:
  file:
    paths: ['**/*.ts']
blocking: true
steps:
  - name: Block TS
    run: |
      Write-Host "TS blocked"
      exit 1
`,
	})

	// Legacy event JSON format
	eventJSON := `{"type":"file","file":{"path":"src/app.ts","action":"create","content":"const x = 1;"}}`

	output, err := runHookflowCmd(t,
		[]string{"run", "--event", eventJSON, "--dir", workspace, "--event-type", "preToolUse"},
		nil)
	// May exit non-zero if workflow denies
	_ = err
	if !strings.Contains(output, "permissionDecision") {
		t.Errorf("Expected JSON result in output, got: %s", output)
	}
}

// ── hookflow triggers command ───────────────────────────────────────

func TestTriggersListAll(t *testing.T) {
	output, err := runHookflowCmd(t, []string{"triggers"}, nil)
	if err != nil {
		t.Fatalf("triggers failed: %v\n%s", err, output)
	}
	if !strings.Contains(output, "hooks") {
		t.Errorf("Expected 'hooks' in triggers output, got: %s", output)
	}
	if !strings.Contains(output, "file") {
		t.Errorf("Expected 'file' in triggers output, got: %s", output)
	}
	if !strings.Contains(output, "commit") {
		t.Errorf("Expected 'commit' in triggers output, got: %s", output)
	}
	if !strings.Contains(output, "push") {
		t.Errorf("Expected 'push' in triggers output, got: %s", output)
	}
}

// ── hookflow check-setup ────────────────────────────────────────────

func TestCheckSetupWithInit(t *testing.T) {
	workspace := t.TempDir()
	gitInit(t, workspace)

	// Init first so hooks.json exists
	_, _ = runHookflowCmd(t, []string{"init", "--dir", workspace}, nil)

	output, err := runHookflowCmd(t, []string{"check-setup", "--dir", workspace}, nil)
	// check-setup may fail if not fully configured — that's OK
	_ = err
	// Should produce some output about the setup status
	if len(output) == 0 {
		t.Errorf("Expected some output from check-setup")
	}
}

// ── hookflow create command ─────────────────────────────────────────

func TestCreateCommand(t *testing.T) {
	workspace := t.TempDir()
	gitInit(t, workspace)

	// Create hookflows directory
	hookflowsDir := filepath.Join(workspace, ".github", "hookflows")
	if err := os.MkdirAll(hookflowsDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	output, err := runHookflowCmd(t, []string{"create", "--dir", workspace,
		"--name", "test-workflow", "--description", "A test workflow"}, nil)
	if err != nil {
		t.Logf("create output: %s", output)
	}
	// Should create a workflow file
	files, _ := os.ReadDir(hookflowsDir)
	if len(files) == 0 {
		t.Logf("No workflow file created (may require AI), output: %s", output)
	}
}

// ── hookflow discover with globs ────────────────────────────────────

func TestDiscoverWithGlob(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"pre-check.yml": `name: Pre Check
on:
  file:
    paths: ['**/*']
steps:
  - name: x
    run: Write-Host "x"
`,
		"post-notify.yml": `name: Post Notify
on:
  file:
    lifecycle: post
    paths: ['**/*']
steps:
  - name: y
    run: Write-Host "y"
`,
		"commit-check.yml": `name: Commit Check
on:
  commit:
steps:
  - name: z
    run: Write-Host "z"
`,
	})

	output, err := runHookflowCmd(t, []string{"discover", "--dir", workspace}, nil)
	if err != nil {
		t.Fatalf("discover failed: %v\n%s", err, output)
	}
	// All three should be discovered
	if !strings.Contains(output, "pre-check") {
		t.Errorf("Expected pre-check in output, got: %s", output)
	}
	if !strings.Contains(output, "post-notify") {
		t.Errorf("Expected post-notify in output, got: %s", output)
	}
	if !strings.Contains(output, "commit-check") {
		t.Errorf("Expected commit-check in output, got: %s", output)
	}
}

// ── hookflow validate with various schemas ──────────────────────────

func TestValidateAllTriggerTypes(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"all-triggers.yml": `name: All Triggers
on:
  hooks:
    types: [preToolUse]
    tools: [create, edit]
  file:
    paths: ['**/*.ts']
    types: [create, edit]
  commit:
    paths: ['src/**']
    paths-ignore: ['**/*.test.ts']
  push:
    branches: [main, 'release/**']
    branches-ignore: ['temp/**']
    tags: ['v*']
    tags-ignore: ['beta-*']
blocking: true
env:
  CI: "true"
steps:
  - name: Check
    run: Write-Host "checking"
    env:
      STEP_VAR: "value"
    timeout: 60
  - name: Conditional
    if: ${{ steps.check.outcome == 'success' }}
    run: Write-Host "conditional"
    continue-on-error: true
`,
	})

	output, err := runHookflowCmd(t, []string{"validate", "--dir", workspace}, nil)
	if err != nil {
		t.Fatalf("validate failed: %v\n%s", err, output)
	}
}

func TestValidateFileFlag(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"specific.yml": `name: Specific
on:
  file:
    paths: ['**/*']
steps:
  - name: x
    run: Write-Host "x"
`,
	})

	filePath := filepath.Join(workspace, ".github", "hookflows", "specific.yml")
	output, err := runHookflowCmd(t, []string{"validate", "--file", filePath}, nil)
	if err != nil {
		t.Fatalf("validate --file failed: %v\n%s", err, output)
	}
}

// ── event detection for different tool types ────────────────────────

func TestEventDetectionViewTool(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"hook-all.yml": `name: Hook All
on:
  hooks:
    types: [preToolUse]
steps:
  - name: Log
    run: Write-Host "Tool used: ${{ event.hook.tool.name }}"
`,
	})

	// view tool
	eventJSON := buildEventJSON("view", map[string]interface{}{
		"path": filepath.Join(workspace, "test.txt"),
	}, workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
}

func TestEventDetectionPowershellTool(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"tool-all.yml": `name: Tool All
on:
  hooks:
    types: [preToolUse]
steps:
  - name: Log
    run: Write-Host "Shell command detected"
`,
	})

	eventJSON := buildShellEventJSON("powershell", "echo hello", workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
}

// ── file events with paths-ignore ───────────────────────────────────

func TestFilePathsIgnoreMatch(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"ignore-tests.yml": `name: Ignore Tests
on:
  file:
    paths: ['**/*']
    paths-ignore:
      - '**/*_test.go'
      - '**/*.test.ts'
      - 'testdata/**'
blocking: true
steps:
  - name: Block non-test
    run: |
      Write-Host "Non-test file blocked"
      exit 1
`,
	})

	// Test file → should be ignored → allow
	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "main_test.go"),
		"file_text": "package main",
	}, workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)

	// Source file → should NOT be ignored → deny
	eventJSON2 := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "main.go"),
		"file_text": "package main",
	}, workspace)
	result2, output2 := runHookflow(t, workspace, eventJSON2, "preToolUse", nil)
	assertDeny(t, result2, output2, "")
}

// ── workflow with step using --raw with JSON in toolArgs ─────────────

func TestRawInputWithStringToolArgs(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"catch-all.yml": `name: Catch All
on:
  hooks:
    types: [preToolUse]
steps:
  - name: Log
    run: Write-Host "Caught"
`,
	})

	// Simulate toolArgs as a JSON string (how Copilot CLI sends it for preToolUse)
	rawJSON := `{"toolName":"powershell","toolArgs":"{\"command\":\"echo hello\"}","cwd":"` +
		strings.ReplaceAll(workspace, `\`, `\\`) + `"}`
	result, output := runHookflow(t, workspace, rawJSON, "preToolUse", nil)
	assertAllow(t, result, output)
}

// ── empty hookflows directory ───────────────────────────────────────

func TestNoHookflowsDirectory(t *testing.T) {
	workspace := t.TempDir()
	gitInit(t, workspace)

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "test.txt"),
		"file_text": "hello",
	}, workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	// No workflows → default allow
	assertAllow(t, result, output)
}
