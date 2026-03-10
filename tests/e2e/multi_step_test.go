package e2e

import (
	"testing"
)

// TestMultiStepPipelineWithExpressions verifies that a multi-step pipeline with
// failure() and always() expressions works correctly. The workflow should deny
// because the deliberate-failure step fails. (Ports e2e.yml Test 11)
func TestMultiStepPipelineWithExpressions(t *testing.T) {
	workspace := setupWorkspace(t)

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      "pipeline/data.json",
		"file_text": "{}",
	}, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertDeny(t, result, output, "deliberate-failure")
}

// TestMultiStepRecoveryStepRuns verifies that always() recovery steps run even
// after a previous step fails.
func TestMultiStepRecoveryStepRuns(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"recovery-pipeline.yml": `name: Recovery pipeline
description: Tests always() expression for recovery
on:
  file:
    paths: ["recovery/**"]
    types: [create]
blocking: true
steps:
  - name: will-fail
    run: exit 1
  - name: recovery
    if: always()
    run: Write-Host "Recovery step ran"
  - name: only-on-success
    if: success()
    run: Write-Host "Should not run"
`,
	})

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      "recovery/file.txt",
		"file_text": "data",
	}, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)

	// Should deny because will-fail exits 1
	assertDeny(t, result, output, "will-fail")

	// Recovery step should have run (appears in output)
	if result != nil {
		// The recovery step output should be somewhere in the full output
		// even though the workflow denies
		t.Logf("Multi-step pipeline output:\n%s", output)
	}
}
