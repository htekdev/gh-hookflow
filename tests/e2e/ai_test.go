package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCreateWorkflowDryRun tests the AI create command in dry-run mode.
// With the e2etest build tag, the fake AI client returns a canned workflow.
func TestCreateWorkflowDryRun(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{})

	output, err := runHookflowCmd(t,
		[]string{"create", "block edits to env files", "--dry-run", "--dir", workspace},
		nil,
	)
	if err != nil {
		t.Fatalf("create --dry-run failed: %v\nOutput: %s", err, output)
	}

	if !strings.Contains(output, "Workflow generated") {
		t.Errorf("expected success message, got: %s", output)
	}
	if !strings.Contains(output, "dry-run") {
		t.Errorf("expected dry-run notice, got: %s", output)
	}
}

// TestCreateWorkflowSavesToDisk tests that create saves the workflow file.
func TestCreateWorkflowSavesToDisk(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{})

	output, err := runHookflowCmd(t,
		[]string{"create", "validate config files", "--dir", workspace, "--output", "test-generated.yml"},
		nil,
	)
	if err != nil {
		t.Fatalf("create failed: %v\nOutput: %s", err, output)
	}

	savedPath := filepath.Join(workspace, ".github", "hookflows", "test-generated.yml")
	if _, err := os.Stat(savedPath); os.IsNotExist(err) {
		t.Errorf("expected workflow file to be saved at %s", savedPath)
	}

	data, err := os.ReadFile(savedPath)
	if err != nil {
		t.Fatalf("Failed to read saved workflow: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "name:") {
		t.Errorf("expected saved file to contain 'name:', got: %s", content)
	}
}

// TestCreateWorkflowWithFakeResponse tests that a custom fake AI response is used.
func TestCreateWorkflowWithFakeResponse(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{})

	fakeYAML := `name: Custom Generated
description: A custom test workflow
on:
  file:
    paths:
      - '**/*.json'
    types:
      - edit
blocking: true
steps:
  - name: Validate JSON
    run: |
      echo "Validating JSON"
      exit 0
`
	output, err := runHookflowCmd(t,
		[]string{"create", "validate json", "--dry-run", "--dir", workspace},
		[]string{"HOOKFLOW_FAKE_AI_RESPONSE=" + fakeYAML},
	)
	if err != nil {
		t.Fatalf("create failed: %v\nOutput: %s", err, output)
	}

	if !strings.Contains(output, "Custom Generated") {
		t.Errorf("expected fake AI response to be used, got: %s", output)
	}
}

// TestCreateWorkflowAIError tests error handling when the AI client fails.
func TestCreateWorkflowAIError(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{})

	output, err := runHookflowCmd(t,
		[]string{"create", "something", "--dir", workspace},
		[]string{
			"HOOKFLOW_FAKE_AI_ERROR=1",
			"HOOKFLOW_FAKE_AI_ERROR_MSG=service temporarily unavailable",
		},
	)
	if err == nil {
		t.Fatalf("expected create to fail with AI error, got output: %s", output)
	}

	if !strings.Contains(output, "service temporarily unavailable") {
		t.Errorf("expected error message in output, got: %s", output)
	}
}
