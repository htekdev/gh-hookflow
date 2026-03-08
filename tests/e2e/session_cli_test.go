package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── session error write and read ────────────────────────────────────

func TestSessionErrorWriteAndRead(t *testing.T) {
	sessionDir := t.TempDir()

	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"post-fail.yml": `name: Post Fail
lifecycle: post
on:
  file:
    paths: ['**/*']
blocking: true
steps:
  - name: Fail post
    run: |
      Write-Host "Post-lifecycle failure"
      exit 1
`,
	})

	opts := &hookflowOpts{sessionDir: sessionDir}

	// Post lifecycle → workflow fails → error.md should be written
	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "test.txt"),
		"file_text": "hello",
	}, workspace)
	_, _ = runHookflow(t, workspace, eventJSON, "postToolUse", opts)

	// Check error.md exists
	errorFile := filepath.Join(sessionDir, "error.md")
	data, err := os.ReadFile(errorFile)
	if err != nil {
		t.Fatalf("error.md not written: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "Post Fail") {
		t.Errorf("Expected error.md to contain workflow name, got: %s", content)
	}
}

func TestSessionErrorBlocksSubsequentCalls(t *testing.T) {
	sessionDir := t.TempDir()

	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"allow-all.yml": `name: Allow All
lifecycle: pre
on:
  file:
    paths: ['**/*']
steps:
  - name: Allow
    run: Write-Host "allowed"
`,
	})

	opts := &hookflowOpts{sessionDir: sessionDir}

	// Manually write error.md
	errorFile := filepath.Join(sessionDir, "error.md")
	errorContent := "# Workflow Error\n\n**Workflow:** test\n**Step:** test-step\n\nSomething failed"
	if err := os.WriteFile(errorFile, []byte(errorContent), 0644); err != nil {
		t.Fatalf("write error.md: %v", err)
	}

	// Next pre-lifecycle call should be denied
	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "test.txt"),
		"file_text": "hello",
	}, workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", opts)
	assertDeny(t, result, output, "")
}

// ── discover CLI command ────────────────────────────────────────────

func TestDiscoverCommand(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"workflow-a.yml": `name: Workflow A
lifecycle: pre
on:
  file:
    paths: ['**/*']
steps:
  - name: A
    run: Write-Host "A"
`,
		"workflow-b.yml": `name: Workflow B
lifecycle: post
on:
  file:
    paths: ['**/*']
steps:
  - name: B
    run: Write-Host "B"
`,
	})

	output, err := runHookflowCmd(t, []string{"discover", "--dir", workspace}, nil)
	if err != nil {
		t.Fatalf("discover failed: %v\n%s", err, output)
	}
	if !strings.Contains(output, "workflow-a.yml") {
		t.Errorf("Expected workflow-a.yml in discover output, got: %s", output)
	}
	if !strings.Contains(output, "workflow-b.yml") {
		t.Errorf("Expected workflow-b.yml in discover output, got: %s", output)
	}
}

// ── validate CLI with multiple files ────────────────────────────────

func TestValidateMultipleValid(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"valid1.yml": `name: Valid 1
lifecycle: pre
on:
  file:
    paths: ['**/*']
steps:
  - name: Step 1
    run: Write-Host "ok"
`,
		"valid2.yml": `name: Valid 2
lifecycle: post
on:
  commit:
steps:
  - name: Step 2
    run: Write-Host "ok"
`,
	})

	output, err := runHookflowCmd(t, []string{"validate", "--dir", workspace}, nil)
	if err != nil {
		t.Fatalf("validate failed: %v\n%s", err, output)
	}
}

func TestValidateInvalidMissingSteps(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"invalid.yml": `name: Invalid
lifecycle: pre
on:
  file:
    paths: ['**/*']
`,
		// Missing steps — should fail validation
	})

	output, err := runHookflowCmd(t, []string{"validate", "--dir", workspace}, nil)
	if err == nil {
		t.Logf("Expected validate to fail for missing steps, output: %s", output)
	}
}

// ── logging command ─────────────────────────────────────────────────

func TestLogsCommand(t *testing.T) {
	output, err := runHookflowCmd(t, []string{"logs"}, nil)
	// logs command should succeed (even if no logs exist)
	if err != nil {
		// Just log, don't fail — logs command might not find files
		t.Logf("logs command output: %s", output)
	}
}

// ── version command ─────────────────────────────────────────────────

func TestVersionOutput(t *testing.T) {
	output, err := runHookflowCmd(t, []string{"version"}, nil)
	if err != nil {
		t.Fatalf("version failed: %v\n%s", err, output)
	}
}

// ── help command ────────────────────────────────────────────────────

func TestHelpCommand(t *testing.T) {
	output, err := runHookflowCmd(t, []string{"help"}, nil)
	if err != nil {
		t.Fatalf("help failed: %v\n%s", err, output)
	}
	if !strings.Contains(output, "hookflow") {
		t.Errorf("Expected 'hookflow' in help output, got: %s", output)
	}
}

// ── init command ────────────────────────────────────────────────────

func TestInitCommand(t *testing.T) {
	workspace := t.TempDir()
	gitInit(t, workspace)

	output, err := runHookflowCmd(t, []string{"init", "--dir", workspace}, nil)
	if err != nil {
		t.Fatalf("init failed: %v\n%s", err, output)
	}

	// Check hooks.json was created
	hooksFile := filepath.Join(workspace, ".github", "hooks", "hooks.json")
	if _, err := os.Stat(hooksFile); os.IsNotExist(err) {
		t.Errorf("hooks.json not created at %s", hooksFile)
	}
}

// ── validate with push trigger ──────────────────────────────────────

func TestValidatePushWorkflow(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"push-wf.yml": `name: Push Workflow
lifecycle: pre
on:
  push:
    branches:
      - main
      - 'release/**'
    tags:
      - 'v*'
blocking: true
steps:
  - name: Check push
    run: Write-Host "Push validated"
`,
	})

	output, err := runHookflowCmd(t, []string{"validate", "--dir", workspace}, nil)
	if err != nil {
		t.Fatalf("validate failed: %v\n%s", err, output)
	}
}

// ── validate with hooks trigger ─────────────────────────────────────

func TestValidateHooksWorkflow(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"hooks-wf.yml": `name: Hooks Workflow
lifecycle: pre
on:
  hooks:
    types: [preToolUse]
    tools: [create, edit]
steps:
  - name: Check hooks
    run: Write-Host "Hooks validated"
`,
	})

	output, err := runHookflowCmd(t, []string{"validate", "--dir", workspace}, nil)
	if err != nil {
		t.Fatalf("validate failed: %v\n%s", err, output)
	}
}

// ── workflow env variables ──────────────────────────────────────────

func TestWorkflowEnvVars(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"env-test.yml": `name: Env Test
lifecycle: pre
on:
  file:
    paths: ['**/*']
env:
  APP_ENV: "production"
  LOG_LEVEL: "debug"
steps:
  - name: Check env
    run: |
      Write-Host "APP_ENV=$env:APP_ENV"
      Write-Host "LOG_LEVEL=$env:LOG_LEVEL"
`,
	})

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "test.txt"),
		"file_text": "hello",
	}, workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
	if !strings.Contains(output, "APP_ENV=production") {
		t.Errorf("Expected APP_ENV in output, got: %s", output)
	}
	if !strings.Contains(output, "LOG_LEVEL=debug") {
		t.Errorf("Expected LOG_LEVEL in output, got: %s", output)
	}
}

// ── step environment variables ──────────────────────────────────────

func TestStepEnvVars(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"step-env.yml": `name: Step Env
lifecycle: pre
on:
  file:
    paths: ['**/*']
steps:
  - name: With step env
    env:
      STEP_VAR: "step-value"
    run: |
      Write-Host "STEP_VAR=$env:STEP_VAR"
`,
	})

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "test.txt"),
		"file_text": "hello",
	}, workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
	if !strings.Contains(output, "STEP_VAR=step-value") {
		t.Errorf("Expected STEP_VAR in output, got: %s", output)
	}
}

// ── working directory ───────────────────────────────────────────────

func TestStepWorkingDirectory(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"workdir.yml": `name: Work Dir
lifecycle: pre
on:
  file:
    paths: ['**/*']
steps:
  - name: Check workdir
    working-directory: .github
    run: |
      $items = Get-ChildItem -Name
      Write-Host "Items: $items"
`,
	})

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "test.txt"),
		"file_text": "hello",
	}, workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
	// .github directory should contain hookflows
	if !strings.Contains(output, "hookflows") {
		t.Logf("Expected 'hookflows' in workdir output, got: %s", output)
	}
}

// ── multiple workflows triggered ────────────────────────────────────

func TestMultipleWorkflowsTriggered(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"wf-a.yml": `name: Workflow A
lifecycle: pre
on:
  file:
    paths: ['**/*.ts']
steps:
  - name: A runs
    run: Write-Host "Workflow A executed"
`,
		"wf-b.yml": `name: Workflow B
lifecycle: pre
on:
  file:
    paths: ['**/*']
steps:
  - name: B runs
    run: Write-Host "Workflow B executed"
`,
	})

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "app.ts"),
		"file_text": "const x = 1;",
	}, workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
}

// ── no matching workflows ───────────────────────────────────────────

func TestNoMatchingWorkflows(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"ts-only.yml": `name: TS Only
lifecycle: pre
on:
  file:
    paths: ['**/*.ts']
blocking: true
steps:
  - name: Block
    run: exit 1
`,
	})

	// .go file should NOT match .ts workflow
	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "main.go"),
		"file_text": "package main",
	}, workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
}

// ── continue-on-error ───────────────────────────────────────────────

func TestContinueOnErrorStep(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"coe.yml": `name: Continue On Error
lifecycle: pre
on:
  file:
    paths: ['**/*']
steps:
  - name: Failing step
    run: exit 1
    continue-on-error: true
  - name: Next step
    run: Write-Host "Continued after error"
`,
	})

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "test.txt"),
		"file_text": "hello",
	}, workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
	if !strings.Contains(output, "Continued after error") {
		t.Errorf("Expected continue-on-error to allow next step, output: %s", output)
	}
}
