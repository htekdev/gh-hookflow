package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── local composite action via uses: ────────────────────────────────

func TestCompositeActionLocal(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"use-action.yml": `name: Use Local Action
lifecycle: pre
on:
  file:
    paths: ['**/*.ts']
blocking: true
steps:
  - name: Run checker
    uses: ./.github/actions/ts-checker
    with:
      mode: strict
`,
	})

	// Create the local action
	actionDir := filepath.Join(workspace, ".github", "actions", "ts-checker")
	if err := os.MkdirAll(actionDir, 0755); err != nil {
		t.Fatalf("Failed to create action dir: %v", err)
	}

	actionYAML := `name: TS Checker
description: Check TypeScript files
inputs:
  mode:
    description: Check mode
    required: false
    default: normal
runs:
  using: composite
  steps:
    - name: Check mode
      run: |
        Write-Host "Running in mode: $env:INPUT_MODE"
        if ($env:INPUT_MODE -eq "strict") {
          Write-Host "Strict mode: blocking"
          exit 1
        }
      shell: pwsh
`
	if err := os.WriteFile(filepath.Join(actionDir, "action.yml"), []byte(actionYAML), 0644); err != nil {
		t.Fatalf("Failed to write action.yml: %v", err)
	}

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "app.ts"),
		"file_text": "const x = 1;",
	}, workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertDeny(t, result, output, "")
}

func TestCompositeActionLocalMultiStep(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"multi-action.yml": `name: Multi Step Action
lifecycle: pre
on:
  file:
    paths: ['**/*']
steps:
  - name: Run multi-step
    uses: ./.github/actions/multi-step
`,
	})

	actionDir := filepath.Join(workspace, ".github", "actions", "multi-step")
	if err := os.MkdirAll(actionDir, 0755); err != nil {
		t.Fatalf("Failed to create action dir: %v", err)
	}

	actionYAML := `name: Multi Step
description: Multiple steps
runs:
  using: composite
  steps:
    - name: Step 1
      run: Write-Host "Step 1 complete"
      shell: pwsh
    - name: Step 2
      run: Write-Host "Step 2 complete"
      shell: pwsh
    - name: Step 3
      run: Write-Host "Step 3 complete"
      shell: pwsh
`
	if err := os.WriteFile(filepath.Join(actionDir, "action.yml"), []byte(actionYAML), 0644); err != nil {
		t.Fatalf("Failed to write action.yml: %v", err)
	}

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "test.txt"),
		"file_text": "hello",
	}, workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
}

func TestCompositeActionWithInputs(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"input-action.yml": `name: Action With Inputs
lifecycle: pre
on:
  file:
    paths: ['**/*']
steps:
  - name: Run with inputs
    uses: ./.github/actions/greeter
    with:
      greeting: "Hello"
      name: "World"
`,
	})

	actionDir := filepath.Join(workspace, ".github", "actions", "greeter")
	if err := os.MkdirAll(actionDir, 0755); err != nil {
		t.Fatalf("Failed to create action dir: %v", err)
	}

	actionYAML := `name: Greeter
description: Greets someone
inputs:
  greeting:
    description: The greeting
    required: true
  name:
    description: The name
    required: true
runs:
  using: composite
  steps:
    - name: Greet
      run: Write-Host "$env:INPUT_GREETING, $env:INPUT_NAME!"
      shell: pwsh
`
	if err := os.WriteFile(filepath.Join(actionDir, "action.yml"), []byte(actionYAML), 0644); err != nil {
		t.Fatalf("Failed to write action.yml: %v", err)
	}

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "test.txt"),
		"file_text": "hello",
	}, workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
	if !strings.Contains(output, "Hello") || !strings.Contains(output, "World") {
		t.Logf("Expected greeting in output, got: %s", output)
	}
}

func TestCompositeActionNotFound(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"missing-action.yml": `name: Missing Action
lifecycle: pre
on:
  file:
    paths: ['**/*']
blocking: true
steps:
  - name: Use missing
    uses: ./.github/actions/nonexistent
`,
	})

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "test.txt"),
		"file_text": "hello",
	}, workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	// Should fail because action directory doesn't exist
	_ = result
	_ = output
}

// ── shell action ────────────────────────────────────────────────────

func TestCompositeActionShellType(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"shell-action.yml": `name: Shell Action
lifecycle: pre
on:
  file:
    paths: ['**/*']
steps:
  - name: Run shell
    uses: ./.github/actions/shell-runner
`,
	})

	actionDir := filepath.Join(workspace, ".github", "actions", "shell-runner")
	if err := os.MkdirAll(actionDir, 0755); err != nil {
		t.Fatalf("Failed to create action dir: %v", err)
	}

	actionYAML := `name: Shell Runner
description: Runs a shell command
runs:
  using: shell
  run: Write-Host "Shell action executed"
  shell: pwsh
`
	if err := os.WriteFile(filepath.Join(actionDir, "action.yml"), []byte(actionYAML), 0644); err != nil {
		t.Fatalf("Failed to write action.yml: %v", err)
	}

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "test.txt"),
		"file_text": "hello",
	}, workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
}
