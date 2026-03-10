package e2e

import (
	"testing"
)

// TestBlockBadCommitMessage verifies that a commit with a non-conventional message
// is denied. (Ports e2e.yml Test 6a)
func TestBlockBadCommitMessage(t *testing.T) {
	workspace := setupWorkspace(t)

	eventJSON := buildShellEventJSON("bash",
		`git commit -m "quick fix"`, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertDeny(t, result, output, "")
}

// TestAllowGoodCommitMessage verifies that a commit with a conventional message
// is allowed. (Ports e2e.yml Test 6b)
func TestAllowGoodCommitMessage(t *testing.T) {
	workspace := setupWorkspace(t)

	eventJSON := buildShellEventJSON("bash",
		`git commit -m "fix: resolve login issue"`, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
}

// TestAllowFeatCommitMessage verifies the feat: prefix is accepted.
func TestAllowFeatCommitMessage(t *testing.T) {
	workspace := setupWorkspace(t)

	eventJSON := buildShellEventJSON("bash",
		`git commit -m "feat: add user authentication"`, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
}

// TestAllowDocsCommitMessage verifies the docs: prefix is accepted.
func TestAllowDocsCommitMessage(t *testing.T) {
	workspace := setupWorkspace(t)

	eventJSON := buildShellEventJSON("bash",
		`git commit -m "docs: update README"`, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
}

// TestBlockCommitNoPrefix verifies a commit without any conventional prefix is blocked.
func TestBlockCommitNoPrefix(t *testing.T) {
	workspace := setupWorkspace(t)

	eventJSON := buildShellEventJSON("bash",
		`git commit -m "updated the thing"`, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertDeny(t, result, output, "")
}
