package e2e

import (
	"testing"
)

// TestPostEditNonBlocking verifies that post-lifecycle hooks run but don't block
// the tool operation. (Ports e2e.yml Test 5)
func TestPostEditNonBlocking(t *testing.T) {
	workspace := setupWorkspace(t)

	eventJSON := buildEventJSON("edit", map[string]interface{}{
		"path":    "main.go",
		"old_str": "old",
		"new_str": "new",
	}, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "postToolUse", nil)

	// Post-lifecycle hooks should never deny
	if result.PermissionDecision == "deny" {
		t.Errorf("Post-edit notification should not deny\nOutput:\n%s", output)
	}
}

// TestPostCreateNonBlocking verifies post-lifecycle create events don't block.
func TestPostCreateNonBlocking(t *testing.T) {
	workspace := setupWorkspace(t)

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      "utils.go",
		"file_text": "package utils",
	}, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "postToolUse", nil)

	if result.PermissionDecision == "deny" {
		t.Errorf("Post-create should not deny\nOutput:\n%s", output)
	}
}
