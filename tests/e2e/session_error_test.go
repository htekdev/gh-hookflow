package e2e

import (
	"os"
	"path/filepath"
	"testing"
)

// TestPostErrorRecordsSessionError verifies that a blocking post-lifecycle workflow
// failure creates a session error file. (Ports e2e.yml Test 8a)
func TestPostErrorRecordsSessionError(t *testing.T) {
	workspace := setupWorkspace(t)

	sessionDir, err := os.MkdirTemp("", "hookflow-e2e-session-8a-*")
	if err != nil {
		t.Fatalf("Failed to create session dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(sessionDir) })

	eventJSON := buildEventJSON("edit", map[string]interface{}{
		"path":    "app.config.json",
		"old_str": "old",
		"new_str": "new",
	}, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "postToolUse", &hookflowOpts{
		sessionDir: sessionDir,
	})

	// Post workflow with blocking + failed step should deny
	assertDeny(t, result, output, "")

	// Verify error file was created in session dir
	errorFile := filepath.Join(sessionDir, "error.md")
	if _, err := os.Stat(errorFile); os.IsNotExist(err) {
		t.Errorf("Session error file was NOT created at %s", errorFile)
	}
}

// TestSessionErrorBlocksNextToolCall verifies that a pending session error blocks
// the next preToolUse call. (Ports e2e.yml Test 8b)
func TestSessionErrorBlocksNextToolCall(t *testing.T) {
	workspace := setupWorkspace(t)

	sessionDir, err := os.MkdirTemp("", "hookflow-e2e-session-8b-*")
	if err != nil {
		t.Fatalf("Failed to create session dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(sessionDir) })

	// First: trigger a post-error to create error.md
	postEventJSON := buildEventJSON("edit", map[string]interface{}{
		"path":    "app.config.json",
		"old_str": "old",
		"new_str": "new",
	}, workspace)

	_, _ = runHookflow(t, workspace, postEventJSON, "postToolUse", &hookflowOpts{
		sessionDir: sessionDir,
	})

	// Verify error.md exists before proceeding
	errorFile := filepath.Join(sessionDir, "error.md")
	if _, err := os.Stat(errorFile); os.IsNotExist(err) {
		t.Fatalf("Setup failed: error.md not created")
	}

	// Second: next preToolUse should be blocked by the session error
	preEventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      "hello.txt",
		"file_text": "hello",
	}, workspace)

	result, output := runHookflow(t, workspace, preEventJSON, "preToolUse", &hookflowOpts{
		sessionDir: sessionDir,
	})

	assertDeny(t, result, output, "previous tool operation failed")
}

// TestClearedErrorAllowsNextToolCall verifies that after clearing a session error,
// the next preToolUse is allowed. (Ports e2e.yml Test 8c)
func TestClearedErrorAllowsNextToolCall(t *testing.T) {
	workspace := setupWorkspace(t)

	sessionDir, err := os.MkdirTemp("", "hookflow-e2e-session-8c-*")
	if err != nil {
		t.Fatalf("Failed to create session dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(sessionDir) })

	// First: trigger a post-error
	postEventJSON := buildEventJSON("edit", map[string]interface{}{
		"path":    "app.config.json",
		"old_str": "old",
		"new_str": "new",
	}, workspace)
	_, _ = runHookflow(t, workspace, postEventJSON, "postToolUse", &hookflowOpts{
		sessionDir: sessionDir,
	})

	// Clear the session error by removing the directory contents
	errorFile := filepath.Join(sessionDir, "error.md")
	_ = os.Remove(errorFile)

	// Next preToolUse for a safe file should be allowed
	preEventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      "hello.txt",
		"file_text": "hello",
	}, workspace)

	result, output := runHookflow(t, workspace, preEventJSON, "preToolUse", &hookflowOpts{
		sessionDir: sessionDir,
	})

	assertAllow(t, result, output)
}
