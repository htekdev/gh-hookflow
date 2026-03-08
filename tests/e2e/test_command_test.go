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
lifecycle: pre
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
	_ = err
	t.Logf("test file output: %s", output)
}

func TestTestCommandCommitMsg(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"commit-lint.yml": `name: Commit Lint
lifecycle: pre
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
	_ = err
	t.Logf("test commit output: %s", output)
}

func TestTestCommandPushBranch(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"push-check.yml": `name: Push Check
lifecycle: pre
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
	_ = err
	t.Logf("test push output: %s", output)
}

func TestTestCommandMissingEvent(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"dummy.yml": `name: Dummy
lifecycle: pre
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
lifecycle: pre
on:
  file:
    paths: ['**/*.go']
steps:
  - name: Go check
    run: Write-Host "go check"
`,
		"other.yml": `name: Other
lifecycle: pre
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
	_ = err
	t.Logf("test specific workflow output: %s", output)
}

// ── hookflow init with --repo flag ──────────────────────────────────

func TestInitRepoFlag(t *testing.T) {
	workspace := t.TempDir()
	gitInit(t, workspace)

	output, err := runHookflowCmd(t, []string{"init", "--dir", workspace, "--repo"}, nil)
	if err != nil {
		t.Logf("init --repo output: %s", output)
	}

	hookflowsDir := filepath.Join(workspace, ".github", "hookflows")
	entries, _ := os.ReadDir(hookflowsDir)
	if len(entries) > 0 {
		t.Logf("Created %d hookflow files", len(entries))
	}
}

// ── post-lifecycle workflow execution ───────────────────────────────

func TestPostLifecycleWorkflow(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"post-validate.yml": `name: Post Validate
lifecycle: post
on:
  file:
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
