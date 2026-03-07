package e2e

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// HookflowResult represents the JSON output from hookflow run --raw.
type HookflowResult struct {
	PermissionDecision       string `json:"permissionDecision"`
	PermissionDecisionReason string `json:"permissionDecisionReason"`
	LogFile                  string `json:"logFile,omitempty"`
	StepOutputs              string `json:"stepOutputs,omitempty"`
}

// hookflowOpts configures how runHookflow executes the binary.
type hookflowOpts struct {
	global     bool   // pass --global flag
	sessionDir string // override HOOKFLOW_SESSION_DIR
	dir        string // override --dir flag (defaults to workspace)
}

// runHookflow executes the coverage-instrumented hookflow binary with the given
// event JSON and event type (preToolUse or postToolUse). It writes coverage data
// to a per-test subdirectory under globalCoverDir.
//
// Returns the parsed result and the raw combined output.
func runHookflow(t *testing.T, workspace, eventJSON, eventType string, opts *hookflowOpts) (*HookflowResult, string) {
	t.Helper()

	// Create per-test coverage subdirectory
	safeName := strings.ReplaceAll(t.Name(), "/", "_")
	safeName = strings.ReplaceAll(safeName, "\\", "_")
	coverSubDir := filepath.Join(globalCoverDir, safeName)
	if err := os.MkdirAll(coverSubDir, 0755); err != nil {
		t.Fatalf("Failed to create coverage subdir: %v", err)
	}

	// Create per-test session directory if not overridden
	sessionDir := ""
	if opts != nil && opts.sessionDir != "" {
		sessionDir = opts.sessionDir
	} else {
		sd, err := os.MkdirTemp("", "hookflow-e2e-session-*")
		if err != nil {
			t.Fatalf("Failed to create session dir: %v", err)
		}
		sessionDir = sd
		t.Cleanup(func() { _ = os.RemoveAll(sd) })
	}

	// Build command args
	dir := workspace
	if opts != nil && opts.dir != "" {
		dir = opts.dir
	}

	args := []string{"run", "--raw", "--event-type", eventType,
		"-e", eventJSON, "--dir", dir}
	if opts != nil && opts.global {
		args = append(args, "--global")
	}

	cmd := exec.Command(binaryPath, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GOCOVERDIR="+coverSubDir,
		"HOOKFLOW_SESSION_DIR="+sessionDir,
	)

	out, err := cmd.CombinedOutput()
	output := string(out)

	// hookflow may exit non-zero on deny — that's expected, not an error
	if err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			t.Fatalf("Failed to execute hookflow: %v\nOutput: %s", err, output)
		}
	}

	// Parse the JSON result from output (find the last JSON object in output)
	result := parseResult(t, output)
	return result, output
}

// parseResult extracts the HookflowResult JSON from hookflow output.
// The output may contain log lines before the JSON, so we find the last
// line that looks like JSON.
func parseResult(t *testing.T, output string) *HookflowResult {
	t.Helper()

	// Try to find JSON in the output by looking for permissionDecision
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if strings.Contains(line, "permissionDecision") {
			var result HookflowResult
			if err := json.Unmarshal([]byte(line), &result); err == nil {
				return &result
			}
			// Try multiline JSON: concatenate from this line onward
			block := strings.Join(lines[i:], "\n")
			if err := json.Unmarshal([]byte(block), &result); err == nil {
				return &result
			}
		}
	}

	// Try parsing the entire output as JSON (pretty-printed)
	var result HookflowResult
	if err := json.Unmarshal([]byte(output), &result); err == nil {
		return &result
	}

	t.Fatalf("Failed to parse hookflow result from output:\n%s", output)
	return nil
}

// assertDeny verifies the result is a deny decision. If reason is non-empty,
// also checks that the deny reason contains the expected substring.
func assertDeny(t *testing.T, result *HookflowResult, output string, reason string) {
	t.Helper()
	if result.PermissionDecision != "deny" {
		t.Errorf("Expected deny, got %q\nOutput:\n%s", result.PermissionDecision, output)
		return
	}
	if reason != "" && !strings.Contains(output, reason) {
		t.Errorf("Expected deny reason to contain %q\nGot: %s\nFull output:\n%s",
			reason, result.PermissionDecisionReason, output)
	}
}

// assertAllow verifies the result is an allow decision.
func assertAllow(t *testing.T, result *HookflowResult, output string) {
	t.Helper()
	if result.PermissionDecision != "allow" {
		t.Errorf("Expected allow, got %q\nOutput:\n%s", result.PermissionDecision, output)
	}
}

// setupWorkspace creates a temporary workspace directory with the standard set
// of E2E test hookflows from testdata/e2e/hookflows/. It also initializes a
// git repo so hookflow can detect the repo root.
func setupWorkspace(t *testing.T) string {
	t.Helper()

	workspace, err := os.MkdirTemp("", "hookflow-e2e-workspace-*")
	if err != nil {
		t.Fatalf("Failed to create workspace: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(workspace) })

	// Create hookflows directory
	hookflowsDir := filepath.Join(workspace, ".github", "hookflows")
	if err := os.MkdirAll(hookflowsDir, 0755); err != nil {
		t.Fatalf("Failed to create hookflows dir: %v", err)
	}

	// Copy test hookflows from testdata
	srcDir := filepath.Join(repoRoot, "testdata", "e2e", "hookflows")
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		t.Fatalf("Failed to read testdata hookflows: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(srcDir, entry.Name()))
		if err != nil {
			t.Fatalf("Failed to read %s: %v", entry.Name(), err)
		}
		if err := os.WriteFile(filepath.Join(hookflowsDir, entry.Name()), data, 0644); err != nil {
			t.Fatalf("Failed to write %s: %v", entry.Name(), err)
		}
	}

	// Initialize git repo (hookflow requires it)
	gitInit(t, workspace)

	return workspace
}

// setupWorkspaceWithHookflows creates a workspace with custom hookflow definitions.
// hookflows is a map of filename → YAML content.
func setupWorkspaceWithHookflows(t *testing.T, hookflows map[string]string) string {
	t.Helper()

	workspace, err := os.MkdirTemp("", "hookflow-e2e-workspace-*")
	if err != nil {
		t.Fatalf("Failed to create workspace: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(workspace) })

	hookflowsDir := filepath.Join(workspace, ".github", "hookflows")
	if err := os.MkdirAll(hookflowsDir, 0755); err != nil {
		t.Fatalf("Failed to create hookflows dir: %v", err)
	}

	for name, content := range hookflows {
		if err := os.WriteFile(filepath.Join(hookflowsDir, name), []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write hookflow %s: %v", name, err)
		}
	}

	gitInit(t, workspace)

	return workspace
}

// gitInit initializes a git repository in the given directory with an initial commit.
func gitInit(t *testing.T, dir string) {
	t.Helper()

	cmds := []struct {
		args []string
	}{
		{[]string{"git", "init"}},
		{[]string{"git", "config", "user.email", "e2e-test@hookflow.dev"}},
		{[]string{"git", "config", "user.name", "E2E Test"}},
	}

	for _, c := range cmds {
		cmd := exec.Command(c.args[0], c.args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git command %v failed: %v\n%s", c.args, err, out)
		}
	}

	// Create and commit an initial file
	initialFile := filepath.Join(dir, "initial.txt")
	if err := os.WriteFile(initialFile, []byte("initial"), 0644); err != nil {
		t.Fatalf("Failed to write initial file: %v", err)
	}

	addCmd := exec.Command("git", "add", ".")
	addCmd.Dir = dir
	if out, err := addCmd.CombinedOutput(); err != nil {
		t.Fatalf("git add failed: %v\n%s", err, out)
	}

	commitCmd := exec.Command("git", "commit", "-m", "chore: initial commit")
	commitCmd.Dir = dir
	if out, err := commitCmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit failed: %v\n%s", err, out)
	}
}

// buildEventJSON constructs a hook input JSON string from the given parameters.
func buildEventJSON(toolName string, toolArgs map[string]interface{}, cwd string) string {
	argsJSON, err := json.Marshal(toolArgs)
	if err != nil {
		panic(fmt.Sprintf("Failed to marshal toolArgs: %v", err))
	}
	event := map[string]interface{}{
		"toolName": toolName,
		"toolArgs": json.RawMessage(argsJSON),
		"cwd":      cwd,
	}
	data, err := json.Marshal(event)
	if err != nil {
		panic(fmt.Sprintf("Failed to marshal event: %v", err))
	}
	return string(data)
}

// buildShellEventJSON constructs a hook input JSON for a shell command (toolArgs
// is serialized as a JSON string, matching the Copilot CLI format for powershell/bash).
func buildShellEventJSON(toolName, command, cwd string) string {
	toolArgs := map[string]interface{}{
		"command": command,
	}
	argsJSON, _ := json.Marshal(toolArgs)
	event := map[string]interface{}{
		"toolName": toolName,
		"toolArgs": string(argsJSON),
		"cwd":      cwd,
	}
	data, _ := json.Marshal(event)
	return string(data)
}

// runHookflowCmd executes the coverage-instrumented hookflow binary with
// arbitrary subcommand args (not run --raw). Used for testing git-push, create,
// discover, and other CLI subcommands.
func runHookflowCmd(t *testing.T, args []string, env []string) (string, error) {
	t.Helper()

	safeName := strings.ReplaceAll(t.Name(), "/", "_")
	safeName = strings.ReplaceAll(safeName, "\\", "_")
	coverSubDir := filepath.Join(globalCoverDir, safeName)
	if err := os.MkdirAll(coverSubDir, 0755); err != nil {
		t.Fatalf("Failed to create coverage subdir: %v", err)
	}

	cmd := exec.Command(binaryPath, args...)
	cmd.Env = append(os.Environ(), "GOCOVERDIR="+coverSubDir)
	cmd.Env = append(cmd.Env, env...)

	out, err := cmd.CombinedOutput()
	return string(out), err
}
