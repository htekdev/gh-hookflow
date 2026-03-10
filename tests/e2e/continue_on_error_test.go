package e2e

import (
	"testing"
)

// TestContinueOnErrorAllows verifies that a workflow with continue-on-error: true
// on a failing step still allows the operation. (Ports e2e.yml Test 9)
func TestContinueOnErrorAllows(t *testing.T) {
	workspace := setupWorkspace(t)

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      "docs/readme.md",
		"file_text": "# Hello",
	}, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
}

// TestContinueOnErrorDoesNotMaskSubsequentFailure verifies that continue-on-error
// only applies to the specific step, not the entire workflow.
func TestContinueOnErrorDoesNotMaskSubsequentFailure(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"continue-then-fail.yml": `name: Continue then fail
description: First step continues on error, second step fails
on:
  file:
    paths: ["test-continue/**"]
    types: [create]
blocking: true
steps:
  - name: soft-fail
    continue-on-error: true
    run: exit 1
  - name: hard-fail
    run: |
      Write-Host "This step always fails"
      exit 1
`,
	})

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      "test-continue/data.txt",
		"file_text": "data",
	}, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertDeny(t, result, output, "hard-fail")
}
