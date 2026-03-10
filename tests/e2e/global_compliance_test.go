package e2e

import (
	"os"
	"path/filepath"
	"testing"
)

// TestGlobalProcessesWithoutMarker verifies that global mode processes events
// when no repo-hooks-active marker exists. (Ports e2e.yml Test 16a)
func TestGlobalProcessesWithoutMarker(t *testing.T) {
	workspace := setupWorkspace(t)

	sessionDir, err := os.MkdirTemp("", "hookflow-e2e-session-16a-*")
	if err != nil {
		t.Fatalf("Failed to create session dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(sessionDir) })

	// No repo-hooks-active marker → global should process
	// .env should be blocked by block-sensitive-files workflow
	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      ".env",
		"file_text": "SECRET=abc",
	}, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", &hookflowOpts{
		global:     true,
		sessionDir: sessionDir,
	})
	assertDeny(t, result, output, "")
}

// TestGlobalSkipsWithMarker verifies that global mode skips processing when the
// repo-hooks-active marker exists. (Ports e2e.yml Test 16b)
func TestGlobalSkipsWithMarker(t *testing.T) {
	workspace := setupWorkspace(t)

	// Global compliance check requires hooks.json to exist when hookflows are present
	hooksDir := filepath.Join(workspace, ".github", "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		t.Fatalf("Failed to create hooks dir: %v", err)
	}
	hooksJSON := `{"version":1,"hooks":{"preToolUse":[{"bash":"gh hookflow run --raw --event-type preToolUse"}]}}`
	if err := os.WriteFile(filepath.Join(hooksDir, "hooks.json"), []byte(hooksJSON), 0644); err != nil {
		t.Fatalf("Failed to write hooks.json: %v", err)
	}

	sessionDir, err := os.MkdirTemp("", "hookflow-e2e-session-16b-*")
	if err != nil {
		t.Fatalf("Failed to create session dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(sessionDir) })

	// Create repo-hooks-active marker
	markerFile := filepath.Join(sessionDir, "repo-hooks-active")
	if err := os.WriteFile(markerFile, []byte(""), 0644); err != nil {
		t.Fatalf("Failed to create marker: %v", err)
	}

	// .env would normally be blocked, but global should skip
	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      ".env",
		"file_text": "SECRET=abc",
	}, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", &hookflowOpts{
		global:     true,
		sessionDir: sessionDir,
	})
	assertAllow(t, result, output)
}

// TestRepoHooksCreateMarker verifies that running in repo mode (non-global) creates
// the repo-hooks-active marker. (Ports e2e.yml Test 16c)
func TestRepoHooksCreateMarker(t *testing.T) {
	workspace := setupWorkspace(t)

	sessionDir, err := os.MkdirTemp("", "hookflow-e2e-session-16c-*")
	if err != nil {
		t.Fatalf("Failed to create session dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(sessionDir) })

	// Run in repo mode (no --global)
	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      "test.txt",
		"file_text": "hello",
	}, workspace)

	_, _ = runHookflow(t, workspace, eventJSON, "preToolUse", &hookflowOpts{
		sessionDir: sessionDir,
	})

	// Verify repo-hooks-active marker was created
	markerFile := filepath.Join(sessionDir, "repo-hooks-active")
	if _, err := os.Stat(markerFile); os.IsNotExist(err) {
		t.Errorf("repo-hooks-active marker was not created")
	}
}

// TestGlobalComplianceDenyWithoutHooksJSON verifies that global mode denies when
// hookflows exist but hooks.json doesn't. (Ports e2e.yml Test 17a)
func TestGlobalComplianceDenyWithoutHooksJSON(t *testing.T) {
	// Create workspace with hookflows but NO hooks.json
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"test.yml": `name: test-workflow
on:
  file:
    paths: ["**"]
steps:
  - run: echo ok
`,
	})

	sessionDir, err := os.MkdirTemp("", "hookflow-e2e-session-17a-*")
	if err != nil {
		t.Fatalf("Failed to create session dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(sessionDir) })

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      "test.txt",
		"file_text": "hello",
	}, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", &hookflowOpts{
		global:     true,
		sessionDir: sessionDir,
		dir:        workspace,
	})

	assertDeny(t, result, output, "gh hookflow init")
}

// TestGlobalComplianceExemptsHookflowInit verifies that the hookflow init command
// is allowed through the compliance check. (Ports e2e.yml Test 17b)
func TestGlobalComplianceExemptsHookflowInit(t *testing.T) {
	// Create workspace with hookflows but NO hooks.json
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"test.yml": `name: test-workflow
on:
  hooks:
    types: [preToolUse]
steps:
  - run: echo ok
`,
	})

	sessionDir, err := os.MkdirTemp("", "hookflow-e2e-session-17b-*")
	if err != nil {
		t.Fatalf("Failed to create session dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(sessionDir) })

	eventJSON := buildShellEventJSON("powershell",
		"gh hookflow init", workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", &hookflowOpts{
		global:     true,
		sessionDir: sessionDir,
		dir:        workspace,
	})

	assertAllow(t, result, output)
}

// TestNoHookflowsGlobalAllows verifies that global mode allows when there are
// no hookflows and no hooks.json. (Ports e2e.yml Test 18a)
func TestNoHookflowsGlobalAllows(t *testing.T) {
	// Create empty workspace (no hookflows)
	workspace, err := os.MkdirTemp("", "hookflow-e2e-empty-*")
	if err != nil {
		t.Fatalf("Failed to create workspace: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(workspace) })
	gitInit(t, workspace)

	sessionDir, err := os.MkdirTemp("", "hookflow-e2e-session-18a-*")
	if err != nil {
		t.Fatalf("Failed to create session dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(sessionDir) })

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      "test.txt",
		"file_text": "hello",
	}, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", &hookflowOpts{
		global:     true,
		sessionDir: sessionDir,
		dir:        workspace,
	})

	assertAllow(t, result, output)
}

// TestComplianceInitRecoveryFlow verifies the full compliance recovery flow:
// deny → allow init → after init works. (Ports e2e.yml Test 19)
func TestComplianceInitRecoveryFlow(t *testing.T) {
	// Create workspace with hookflows but NO hooks.json
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"block-env.yml": `name: Block Env Files
on:
  file:
    paths: ["**/.env*"]
    types: [create, edit]
blocking: true
steps:
  - name: Deny
    run: |
      echo "Blocked"
      exit 1
`,
	})

	sessionDir, err := os.MkdirTemp("", "hookflow-e2e-session-19-*")
	if err != nil {
		t.Fatalf("Failed to create session dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(sessionDir) })

	// Step 1: Normal create should be DENIED (no hooks.json)
	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      "test.txt",
		"file_text": "hello",
	}, workspace)

	result1, output1 := runHookflow(t, workspace, eventJSON, "preToolUse", &hookflowOpts{
		global:     true,
		sessionDir: sessionDir,
		dir:        workspace,
	})
	assertDeny(t, result1, output1, "hookflow init")

	// Step 2: hookflow init command should be ALLOWED through
	initEventJSON := buildShellEventJSON("powershell",
		"gh hookflow init", workspace)

	result2, output2 := runHookflow(t, workspace, initEventJSON, "preToolUse", &hookflowOpts{
		global:     true,
		sessionDir: sessionDir,
		dir:        workspace,
	})
	assertAllow(t, result2, output2)

	// Step 3: Create hooks.json manually (simulating init)
	hooksDir := filepath.Join(workspace, ".github", "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		t.Fatalf("Failed to create hooks dir: %v", err)
	}
	hooksJSON := `{"version":1,"hooks":{"preToolUse":[{"bash":"gh hookflow run --raw --event-type preToolUse"}]}}`
	if err := os.WriteFile(filepath.Join(hooksDir, "hooks.json"), []byte(hooksJSON), 0644); err != nil {
		t.Fatalf("Failed to create hooks.json: %v", err)
	}

	// Step 4: Normal create should now be ALLOWED
	result4, output4 := runHookflow(t, workspace, eventJSON, "preToolUse", &hookflowOpts{
		global:     true,
		sessionDir: sessionDir,
		dir:        workspace,
	})
	assertAllow(t, result4, output4)
}
