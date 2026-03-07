package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDiscoverFindsAllWorkflows verifies the discover command finds all workflow files.
func TestDiscoverFindsAllWorkflows(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"workflow-a.yml": `name: Workflow A
on:
  file:
    paths: ['**/*']
steps:
  - name: Check
    run: echo ok
`,
		"workflow-b.yaml": `name: Workflow B
on:
  file:
    paths: ['**/*']
steps:
  - name: Check
    run: echo ok
`,
		"workflow-c.yml": `name: Workflow C
on:
  file:
    paths: ['**/*']
steps:
  - name: Check
    run: echo ok
`,
	})

	output, err := runHookflowCmd(t,
		[]string{"discover", "--dir", workspace},
		nil,
	)
	if err != nil {
		t.Fatalf("discover failed: %v\nOutput: %s", err, output)
	}

	if !strings.Contains(output, "workflow-a") {
		t.Errorf("expected to find workflow-a, got: %s", output)
	}
	if !strings.Contains(output, "workflow-b") {
		t.Errorf("expected to find workflow-b, got: %s", output)
	}
	if !strings.Contains(output, "workflow-c") {
		t.Errorf("expected to find workflow-c, got: %s", output)
	}
	if !strings.Contains(output, "3") {
		t.Errorf("expected output to mention 3 workflows, got: %s", output)
	}
}

// TestDiscoverSkipsNonYaml verifies non-YAML files in hookflows/ are ignored.
func TestDiscoverSkipsNonYaml(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"valid-workflow.yml": `name: Valid
on:
  file:
    paths: ['**/*']
steps:
  - name: Check
    run: echo ok
`,
	})

	// Add non-YAML files
	hookflowsDir := filepath.Join(workspace, ".github", "hookflows")
	_ = os.WriteFile(filepath.Join(hookflowsDir, "README.md"), []byte("# Notes"), 0644)
	_ = os.WriteFile(filepath.Join(hookflowsDir, "notes.txt"), []byte("some notes"), 0644)

	output, err := runHookflowCmd(t,
		[]string{"discover", "--dir", workspace},
		nil,
	)
	if err != nil {
		t.Fatalf("discover failed: %v\nOutput: %s", err, output)
	}

	if !strings.Contains(output, "valid-workflow") {
		t.Errorf("expected to find valid-workflow, got: %s", output)
	}
	// Should find exactly 1 workflow (the .yml file), not the .md or .txt
	if !strings.Contains(output, "1") {
		t.Errorf("expected output to show 1 workflow, got: %s", output)
	}
}

// TestDiscoverEmptyDir verifies behavior when no workflows exist.
func TestDiscoverEmptyDir(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{})

	output, err := runHookflowCmd(t,
		[]string{"discover", "--dir", workspace},
		nil,
	)
	if err != nil {
		t.Fatalf("discover failed: %v\nOutput: %s", err, output)
	}

	if !strings.Contains(strings.ToLower(output), "no workflow") && !strings.Contains(output, "0") {
		t.Errorf("expected 'no workflows' message, got: %s", output)
	}
}

// TestDiscoverNoHookflowsDir verifies behavior when .github/hookflows/ doesn't exist.
func TestDiscoverNoHookflowsDir(t *testing.T) {
	workspace, err := os.MkdirTemp("", "hookflow-e2e-empty-*")
	if err != nil {
		t.Fatalf("Failed to create workspace: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(workspace) })
	gitInit(t, workspace)

	output, err := runHookflowCmd(t,
		[]string{"discover", "--dir", workspace},
		nil,
	)
	if err != nil {
		t.Fatalf("discover failed: %v\nOutput: %s", err, output)
	}

	if !strings.Contains(strings.ToLower(output), "no workflow") && !strings.Contains(output, "0") {
		t.Errorf("expected 'no workflows' message, got: %s", output)
	}
}
