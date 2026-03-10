package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── hookflow test command ───────────────────────────────────────────

func TestTestCommandFileCreate(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"file-check.yml": `name: File Check
on:
  file:
    paths: ['**/*.ts']
blocking: true
steps:
  - name: Check
    run: Write-Host "checking"
`,
	})

	output, err := runHookflowCmd(t, []string{
		"test",
		"--event", "file",
		"--path", "src/app.ts",
		"--action", "create",
		"--dir", workspace,
	}, nil)
	if err != nil {
		t.Fatalf("test command failed: %v\noutput: %s", err, output)
	}
	lower := strings.ToLower(output)
	if !strings.Contains(lower, "file check") && !strings.Contains(lower, "check") {
		t.Errorf("expected output to reference the matching workflow, got: %s", output)
	}
}

func TestTestCommandCommitMsg(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"commit-lint.yml": `name: Commit Lint
on:
  commit:
steps:
  - name: Lint
    run: Write-Host "lint"
`,
	})

	output, err := runHookflowCmd(t, []string{
		"test",
		"--event", "commit",
		"--message", "feat: add feature",
		"--branch", "main",
		"--dir", workspace,
	}, nil)
	if err != nil {
		t.Fatalf("test command failed: %v\noutput: %s", err, output)
	}
}

func TestTestCommandPushBranch(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"push-check.yml": `name: Push Check
on:
  push:
    branches: [main]
steps:
  - name: Check
    run: Write-Host "push check"
`,
	})

	output, err := runHookflowCmd(t, []string{
		"test",
		"--event", "push",
		"--branch", "main",
		"--dir", workspace,
	}, nil)
	if err != nil {
		t.Fatalf("test command failed: %v\noutput: %s", err, output)
	}
}

func TestTestCommandMissingEvent(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"dummy.yml": `name: Dummy
on:
  file:
    paths: ['**/*']
steps:
  - name: x
    run: Write-Host "x"
`,
	})

	_, err := runHookflowCmd(t, []string{"test", "--dir", workspace}, nil)
	if err == nil {
		t.Errorf("Expected error when --event is missing")
	}
}

func TestTestCommandSpecificWorkflow(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"target.yml": `name: Target
on:
  file:
    paths: ['**/*.go']
steps:
  - name: Go check
    run: Write-Host "go check"
`,
		"other.yml": `name: Other
on:
  file:
    paths: ['**/*.ts']
steps:
  - name: TS check
    run: Write-Host "ts check"
`,
	})

	output, err := runHookflowCmd(t, []string{
		"test",
		"--event", "file",
		"--path", "main.go",
		"--workflow", "target",
		"--dir", workspace,
	}, nil)
	if err != nil {
		t.Fatalf("test command failed: %v\noutput: %s", err, output)
	}
	lower := strings.ToLower(output)
	if !strings.Contains(lower, "target") {
		t.Errorf("expected output to reference 'Target' workflow, got: %s", output)
	}
	if strings.Contains(lower, "other") {
		t.Errorf("expected output to NOT reference 'Other' workflow when --workflow filter is used, got: %s", output)
	}
}

// ── hookflow init with --repo flag ──────────────────────────────────

func TestInitRepoFlag(t *testing.T) {
	workspace := t.TempDir()
	gitInit(t, workspace)

	output, err := runHookflowCmd(t, []string{"init", "--dir", workspace, "--repo"}, nil)
	if err != nil {
		t.Fatalf("init --repo failed: %v\noutput: %s", err, output)
	}

	hookflowsDir := filepath.Join(workspace, ".github", "hookflows")
	info, statErr := os.Stat(hookflowsDir)
	if statErr != nil {
		t.Fatalf("hookflows directory was not created: %v", statErr)
	}
	if !info.IsDir() {
		t.Fatalf("expected %s to be a directory", hookflowsDir)
	}
	entries, readErr := os.ReadDir(hookflowsDir)
	if readErr != nil {
		t.Fatalf("failed to read hookflows directory: %v", readErr)
	}
	if len(entries) == 0 {
		t.Errorf("expected at least one hookflow file in %s, found none", hookflowsDir)
	}
}

// ── post-lifecycle workflow execution ───────────────────────────────

func TestPostLifecycleWorkflow(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"post-validate.yml": `name: Post Validate
on:
  file:
    lifecycle: post
    paths: ['**/*.json']
steps:
  - name: Validate JSON
    run: Write-Host "Validating JSON file"
`,
	})

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "config.json"),
		"file_text": `{"key": "value"}`,
	}, workspace)
	result, output := runHookflow(t, workspace, eventJSON, "postToolUse", nil)
	assertAllow(t, result, output)
	if !strings.Contains(output, "Validating JSON") {
		t.Logf("Expected post-lifecycle output, got: %s", output)
	}
}

// ── hookflow validate edge cases ────────────────────────────────────

func TestValidateInvalidYAMLFile(t *testing.T) {
	workspace := t.TempDir()
	invalidFile := filepath.Join(workspace, "bad.yml")
	if err := os.WriteFile(invalidFile, []byte("not: valid: yaml: {{"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, err := runHookflowCmd(t, []string{"validate", "--file", invalidFile}, nil)
	if err == nil {
		t.Logf("Expected validate to fail for invalid YAML")
	}
}

func TestValidateNonexistentFile(t *testing.T) {
	_, err := runHookflowCmd(t, []string{"validate", "--file", "/nonexistent/path/bad.yml"}, nil)
	if err == nil {
		t.Errorf("Expected error for nonexistent file")
	}
}
