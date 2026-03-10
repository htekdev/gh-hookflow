package e2e

import (
	"testing"
)

// TestStepTimeoutKillsLongRunning verifies that a step with a short timeout
// is killed and the workflow denies. (Ports e2e.yml Test 12)
func TestStepTimeoutKillsLongRunning(t *testing.T) {
	workspace := setupWorkspace(t)

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      "slow/data.txt",
		"file_text": "data",
	}, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertDeny(t, result, output, "")
}

// TestStepTimeoutDoesNotAffectFastSteps verifies that steps completing within
// their timeout are not killed.
func TestStepTimeoutDoesNotAffectFastSteps(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"fast-timeout.yml": `name: Fast timeout
description: Step finishes well within timeout
on:
  file:
    paths: ["timeout-ok/**"]
    types: [create]
blocking: true
steps:
  - name: fast-step
    timeout: 60
    run: Write-Host "Done quickly"
`,
	})

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      "timeout-ok/data.txt",
		"file_text": "data",
	}, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
}
