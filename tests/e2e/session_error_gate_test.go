package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestErrorGateAllowsViewOfErrorFile tests the session error gate exemption:
// when an error.md exists, "view" of the error file is allowed through.
// Targets: isSessionErrorFileRead (main.go:1189), extractToolArgsPath (main.go:1210)
func TestErrorGateAllowsViewOfErrorFile(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"dummy.yml": `name: Dummy
lifecycle: pre
on:
  file:
    paths: ["**/*"]
steps:
  - name: allow
    run: Write-Host "ok"
`,
	})

	// Create session dir with error.md
	sessionDir, err := os.MkdirTemp("", "hookflow-e2e-errgate-*")
	if err != nil {
		t.Fatalf("Failed to create session dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(sessionDir) })

	errorFilePath := filepath.Join(sessionDir, "error.md")
	errContent := "# Error\nWorkflow: test\nStep: check\nDetails: something failed\n"
	if err := os.WriteFile(errorFilePath, []byte(errContent), 0644); err != nil {
		t.Fatalf("Failed to write error.md: %v", err)
	}

	// Send a "view" of the error file — should be allowed through (exemption)
	eventJSON := buildEventJSON("view", map[string]interface{}{
		"path": errorFilePath,
	}, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", &hookflowOpts{
		sessionDir: sessionDir,
	})

	// The view exemption should allow this through immediately
	assertAllow(t, result, output)
}

// TestErrorGatePostClearsErrorFile tests that postToolUse for view of error file
// triggers ReadAndClearError, deleting the error.md.
// Targets: ReadAndClearError (session/errors.go:51)
func TestErrorGatePostClearsErrorFile(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"dummy.yml": `name: Dummy
lifecycle: post
on:
  file:
    paths: ["**/*"]
steps:
  - name: allow
    run: Write-Host "ok"
`,
	})

	// Create session dir with error.md
	sessionDir, err := os.MkdirTemp("", "hookflow-e2e-errpost-*")
	if err != nil {
		t.Fatalf("Failed to create session dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(sessionDir) })

	errorFilePath := filepath.Join(sessionDir, "error.md")
	errContent := "# Error\nWorkflow: test\nStep: check\nDetails: something failed\n"
	if err := os.WriteFile(errorFilePath, []byte(errContent), 0644); err != nil {
		t.Fatalf("Failed to write error.md: %v", err)
	}

	// Send postToolUse for view of error file — should clear error.md
	eventJSON := buildEventJSON("view", map[string]interface{}{
		"path": errorFilePath,
	}, workspace)

	_, _ = runHookflow(t, workspace, eventJSON, "postToolUse", &hookflowOpts{
		sessionDir: sessionDir,
	})

	// Verify error.md was deleted
	if _, err := os.Stat(errorFilePath); !os.IsNotExist(err) {
		t.Errorf("Expected error.md to be deleted after postToolUse view, but it still exists")
	}
}

// TestErrorGateNormalAfterClearance tests that after error.md is cleared,
// subsequent preToolUse events proceed normally.
// Targets: HasError (session/errors.go:33) returning false
func TestErrorGateNormalAfterClearance(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"dummy.yml": `name: Dummy
lifecycle: pre
on:
  file:
    paths: ["**/*"]
steps:
  - name: allow
    run: Write-Host "ok"
`,
	})

	// Create session dir WITHOUT error.md (simulating post-clearance state)
	sessionDir, err := os.MkdirTemp("", "hookflow-e2e-cleared-*")
	if err != nil {
		t.Fatalf("Failed to create session dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(sessionDir) })

	// Normal preToolUse should proceed without error gate blocking
	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "test.txt"),
		"file_text": "hello",
	}, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", &hookflowOpts{
		sessionDir: sessionDir,
	})
	assertAllow(t, result, output)
}

// TestErrorGateSkipsPostLifecycle tests that the error gate only fires for
// pre-lifecycle events. Post-lifecycle events are not blocked by error.md.
// Targets: lifecycle check in session error gate (main.go ~line 680)
func TestErrorGateSkipsPostLifecycle(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"post-dummy.yml": `name: Post Dummy
lifecycle: post
on:
  file:
    paths: ["**/*"]
steps:
  - name: notify
    run: Write-Host "post lifecycle"
`,
	})

	// Create session dir WITH error.md
	sessionDir, err := os.MkdirTemp("", "hookflow-e2e-postskip-*")
	if err != nil {
		t.Fatalf("Failed to create session dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(sessionDir) })

	errorFilePath := filepath.Join(sessionDir, "error.md")
	if err := os.WriteFile(errorFilePath, []byte("# Error\ntest error"), 0644); err != nil {
		t.Fatalf("Failed to write error.md: %v", err)
	}

	// Send a postToolUse event for a non-view tool — should NOT be blocked
	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "test.txt"),
		"file_text": "hello",
	}, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "postToolUse", &hookflowOpts{
		sessionDir: sessionDir,
	})

	// Post lifecycle is not blocked by error gate
	assertAllow(t, result, output)
}

// TestErrorGateNonViewDenied tests that non-view tool calls are denied
// when error.md exists (the standard error gate behavior).
// Targets: HasError true path, deny message construction (main.go ~line 688)
func TestErrorGateNonViewDenied(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"dummy.yml": `name: Dummy
lifecycle: pre
on:
  file:
    paths: ["**/*"]
steps:
  - name: allow
    run: Write-Host "ok"
`,
	})

	// Create session dir with error.md
	sessionDir, err := os.MkdirTemp("", "hookflow-e2e-nonview-*")
	if err != nil {
		t.Fatalf("Failed to create session dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(sessionDir) })

	errorFilePath := filepath.Join(sessionDir, "error.md")
	if err := os.WriteFile(errorFilePath, []byte("# Error\ntest error"), 0644); err != nil {
		t.Fatalf("Failed to write error.md: %v", err)
	}

	// Send a preToolUse with non-view tool (edit) — should be DENIED
	eventJSON := buildEventJSON("edit", map[string]interface{}{
		"path":    filepath.Join(workspace, "test.txt"),
		"old_str": "old",
		"new_str": "new",
	}, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", &hookflowOpts{
		sessionDir: sessionDir,
	})

	assertDeny(t, result, output, "error file")
}

// TestErrorGateViewWrongPathDenied tests that viewing a file OTHER than
// the error file is still denied when error.md exists.
// Targets: isSessionErrorFileRead path comparison (main.go:1189)
func TestErrorGateViewWrongPathDenied(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"dummy.yml": `name: Dummy
lifecycle: pre
on:
  file:
    paths: ["**/*"]
steps:
  - name: allow
    run: Write-Host "ok"
`,
	})

	// Create session dir with error.md
	sessionDir, err := os.MkdirTemp("", "hookflow-e2e-wrongview-*")
	if err != nil {
		t.Fatalf("Failed to create session dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(sessionDir) })

	errorFilePath := filepath.Join(sessionDir, "error.md")
	if err := os.WriteFile(errorFilePath, []byte("# Error\ntest error"), 0644); err != nil {
		t.Fatalf("Failed to write error.md: %v", err)
	}

	// Send view of a DIFFERENT file — should still be denied
	eventJSON := buildEventJSON("view", map[string]interface{}{
		"path": filepath.Join(workspace, "some-other-file.txt"),
	}, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", &hookflowOpts{
		sessionDir: sessionDir,
	})

	assertDeny(t, result, output, "error file")
}

// TestErrorGateFullCycle tests the complete error gate lifecycle:
// 1. Error exists → deny
// 2. View error file → allowed (exemption)
// 3. Post view error file → clears error
// 4. Next preToolUse → normal processing
func TestErrorGateFullCycle(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"dummy.yml": `name: Dummy
lifecycle: pre
on:
  file:
    paths: ["**/*"]
steps:
  - name: allow
    run: Write-Host "ok"
`,
	})

	// Create persistent session dir
	sessionDir, err := os.MkdirTemp("", "hookflow-e2e-fullcycle-*")
	if err != nil {
		t.Fatalf("Failed to create session dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(sessionDir) })

	errorFilePath := filepath.Join(sessionDir, "error.md")

	// Step 1: Write error and verify denial
	if err := os.WriteFile(errorFilePath, []byte("# Error\nStep: lint\nDetails: failed"), 0644); err != nil {
		t.Fatalf("Failed to write error.md: %v", err)
	}

	editEvent := buildEventJSON("edit", map[string]interface{}{
		"path":    filepath.Join(workspace, "test.txt"),
		"old_str": "old",
		"new_str": "new",
	}, workspace)

	result1, output1 := runHookflow(t, workspace, editEvent, "preToolUse", &hookflowOpts{
		sessionDir: sessionDir,
	})
	assertDeny(t, result1, output1, "error file")

	// Step 2: View error file — exemption allows
	viewEvent := buildEventJSON("view", map[string]interface{}{
		"path": errorFilePath,
	}, workspace)

	result2, output2 := runHookflow(t, workspace, viewEvent, "preToolUse", &hookflowOpts{
		sessionDir: sessionDir,
	})
	assertAllow(t, result2, output2)

	// Step 3: Post-lifecycle view clears error
	_, _ = runHookflow(t, workspace, viewEvent, "postToolUse", &hookflowOpts{
		sessionDir: sessionDir,
	})

	if _, err := os.Stat(errorFilePath); !os.IsNotExist(err) {
		t.Fatalf("error.md should be deleted after postToolUse view")
	}

	// Step 4: Normal preToolUse succeeds
	result4, output4 := runHookflow(t, workspace, editEvent, "preToolUse", &hookflowOpts{
		sessionDir: sessionDir,
	})
	assertAllow(t, result4, output4)
}

// TestExtractToolArgsPathStringFormat tests extractToolArgsPath with string-encoded
// toolArgs (the format used by Copilot CLI preToolUse events).
// Targets: extractToolArgsPath string unwrap path (main.go:1224)
func TestExtractToolArgsPathStringFormat(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"dummy.yml": `name: Dummy
lifecycle: pre
on:
  file:
    paths: ["**/*"]
steps:
  - name: allow
    run: Write-Host "ok"
`,
	})

	// Create session dir with error.md
	sessionDir, err := os.MkdirTemp("", "hookflow-e2e-strformat-*")
	if err != nil {
		t.Fatalf("Failed to create session dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(sessionDir) })

	errorFilePath := filepath.Join(sessionDir, "error.md")
	if err := os.WriteFile(errorFilePath, []byte("# Error\ntest"), 0644); err != nil {
		t.Fatalf("Failed to write error.md: %v", err)
	}

	// Build event with STRING toolArgs (Copilot CLI preToolUse format)
	toolArgsInner := map[string]interface{}{"path": errorFilePath}
	argsJSON, _ := json.Marshal(toolArgsInner)
	event := map[string]interface{}{
		"toolName": "view",
		"toolArgs": string(argsJSON), // String-encoded, not object
		"cwd":      workspace,
	}
	eventBytes, _ := json.Marshal(event)
	eventJSON := string(eventBytes)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", &hookflowOpts{
		sessionDir: sessionDir,
	})

	// String-format toolArgs should still be parsed and matched
	assertAllow(t, result, output)
}
