package e2e

import (
	"testing"
)

// TestBlockGitPushPrimitiveGuard verifies that direct git push commands are
// blocked by the primitive guard, regardless of tool name. (Ports e2e.yml Test 15a)
func TestBlockGitPushPrimitiveGuard(t *testing.T) {
	workspace := setupWorkspace(t)

	eventJSON := buildShellEventJSON("unknown_shell_tool",
		"git push origin main", workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertDeny(t, result, output, "hookflow git-push")
}

// TestBlockMultiGitCommands verifies that multiple git commands in a single tool
// call are blocked by the primitive guard. (Ports e2e.yml Test 15b)
func TestBlockMultiGitCommands(t *testing.T) {
	workspace := setupWorkspace(t)

	eventJSON := buildShellEventJSON("bash",
		`git add . && git commit -m "fix"`, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertDeny(t, result, output, "Multiple git commands")
}

// TestPostLifecycleSkipsPrimitiveGuards verifies that post-lifecycle events
// are not blocked by primitive guards. (Ports e2e.yml Test 15c)
func TestPostLifecycleSkipsPrimitiveGuards(t *testing.T) {
	workspace := setupWorkspace(t)

	eventJSON := buildShellEventJSON("bash",
		"git push origin main", workspace)

	result, output := runHookflow(t, workspace, eventJSON, "postToolUse", nil)

	// Post lifecycle should NOT be blocked by primitive guards
	assertAllow(t, result, output)
}

// TestBlockGitPushWithFlags verifies git push with various flags is blocked.
func TestBlockGitPushWithFlags(t *testing.T) {
	workspace := setupWorkspace(t)

	commands := []string{
		"git push --force origin main",
		"git push -u origin feature",
		"git push --set-upstream origin branch",
	}

	for _, cmd := range commands {
		t.Run(cmd, func(t *testing.T) {
			eventJSON := buildShellEventJSON("powershell", cmd, workspace)
			result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
			assertDeny(t, result, output, "hookflow git-push")
		})
	}
}

// TestAllowSingleGitCommand verifies that a single (non-push) git command is allowed.
func TestAllowSingleGitCommand(t *testing.T) {
	workspace := setupWorkspace(t)

	eventJSON := buildShellEventJSON("bash", "git status", workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
}
