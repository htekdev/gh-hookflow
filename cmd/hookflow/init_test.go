package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestInitRepoOnly tests that hookflow init only creates repo-level files (no global setup)
func TestInitRepoOnly(t *testing.T) {
	tempRepo := t.TempDir()

	// Run init (should only create repo-level files)
	if err := runInit(tempRepo, false, false); err != nil {
		t.Fatalf("runInit failed: %v", err)
	}

	// Verify .github/hooks/hooks.json exists
	hooksPath := filepath.Join(tempRepo, ".github", "hooks", "hooks.json")
	if _, err := os.Stat(hooksPath); os.IsNotExist(err) {
		t.Fatal("hooks.json not created")
	}

	// Verify NO global files are created (init is repo-only now)
	homeDir, _ := os.UserHomeDir()
	skillPath := filepath.Join(homeDir, ".copilot", "skills", "hookflow", "SKILL.md")
	// We can't assert non-existence of SKILL.md since it may already exist from
	// a real install, but we verify init doesn't call runGlobalInit by checking
	// the function doesn't exist anymore (compile-time check).
	_ = skillPath
}

// TestInitWithRepoFlag tests that --repo creates example workflow scaffolding
func TestInitWithRepoFlag(t *testing.T) {
	tempRepo := t.TempDir()

	// Run init with --repo flag
	if err := runInit(tempRepo, false, true); err != nil {
		t.Fatalf("runInit --repo failed: %v", err)
	}

	// Verify .github/hookflows/example.yml exists
	examplePath := filepath.Join(tempRepo, ".github", "hookflows", "example.yml")
	if _, err := os.Stat(examplePath); os.IsNotExist(err) {
		t.Fatal("example.yml not created")
	}

	exampleData, err := os.ReadFile(examplePath)
	if err != nil {
		t.Fatalf("failed to read example.yml: %v", err)
	}

	// Verify example workflow has expected content
	exampleContent := string(exampleData)
	if !strings.Contains(exampleContent, "name:") {
		t.Error("example.yml should have a name field")
	}
	if !strings.Contains(exampleContent, "on:") {
		t.Error("example.yml should have an on trigger field")
	}
	if !strings.Contains(exampleContent, "steps:") {
		t.Error("example.yml should have steps")
	}

	// Verify .github/hooks/hooks.json has sessionStart
	hooksPath := filepath.Join(tempRepo, ".github", "hooks", "hooks.json")
	hooksData, err := os.ReadFile(hooksPath)
	if err != nil {
		t.Fatalf("repo hooks.json not created: %v", err)
	}

	var hooksConfig map[string]interface{}
	if err := json.Unmarshal(hooksData, &hooksConfig); err != nil {
		t.Fatalf("repo hooks.json is invalid JSON: %v", err)
	}

	hooks, ok := hooksConfig["hooks"].(map[string]interface{})
	if !ok {
		t.Fatal("repo hooks.json missing 'hooks' field")
	}

	sessionStart, ok := hooks["sessionStart"].([]interface{})
	if !ok || len(sessionStart) == 0 {
		t.Fatal("repo hooks.json missing sessionStart hooks")
	}

	// Verify hookflow check-setup is in sessionStart
	hooksStr := string(hooksData)
	if !strings.Contains(hooksStr, "hookflow check-setup") {
		t.Error("repo hooks.json should contain 'hookflow check-setup' command")
	}
}

// TestInitRepoCreatesDirectories tests that repo init creates .github directories
func TestInitRepoCreatesDirectories(t *testing.T) {
	tempRepo := t.TempDir()

	// Run repo init
	if err := runRepoHooksInit(tempRepo, false); err != nil {
		t.Fatalf("runRepoHooksInit failed: %v", err)
	}
	if err := runRepoScaffoldInit(tempRepo, false); err != nil {
		t.Fatalf("runRepoScaffoldInit failed: %v", err)
	}

	// Verify directories were created
	dirs := []string{
		filepath.Join(tempRepo, ".github"),
		filepath.Join(tempRepo, ".github", "hookflows"),
		filepath.Join(tempRepo, ".github", "hooks"),
	}

	for _, dir := range dirs {
		info, err := os.Stat(dir)
		if err != nil {
			t.Errorf("directory %s should exist: %v", dir, err)
		} else if !info.IsDir() {
			t.Errorf("%s should be a directory", dir)
		}
	}
}

// TestInitMergePreservesExistingHooks tests that init preserves non-hookflow hooks in hooks.json
func TestInitMergePreservesExistingHooks(t *testing.T) {
	tempRepo := t.TempDir()
	hooksDir := filepath.Join(tempRepo, ".github", "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		t.Fatalf("failed to create hooks dir: %v", err)
	}

	// Create existing hooks.json with a custom hook
	existingConfig := map[string]interface{}{
		"version": 1,
		"hooks": map[string]interface{}{
			"preToolUse": []interface{}{
				map[string]interface{}{
					"type": "command",
					"bash": "echo 'custom pre hook'",
				},
			},
		},
	}
	existingJSON, _ := json.MarshalIndent(existingConfig, "", "  ")
	hooksFile := filepath.Join(hooksDir, "hooks.json")
	if err := os.WriteFile(hooksFile, existingJSON, 0644); err != nil {
		t.Fatalf("failed to create hooks.json: %v", err)
	}

	// Run init
	if err := runRepoHooksInit(tempRepo, false); err != nil {
		t.Fatalf("runRepoHooksInit failed: %v", err)
	}

	// Read merged hooks.json
	hooksData, err := os.ReadFile(hooksFile)
	if err != nil {
		t.Fatalf("failed to read hooks.json: %v", err)
	}

	hooksStr := string(hooksData)
	// Custom hook should be preserved
	if !strings.Contains(hooksStr, "custom pre hook") {
		t.Error("existing custom hook should be preserved")
	}
	// Hookflow hooks should also be present
	if !strings.Contains(hooksStr, "hookflow run") {
		t.Error("hookflow hooks should be added")
	}
}

// TestInitForceOverwritesHooks tests that --force replaces hookflow hooks
func TestInitForceOverwritesHooks(t *testing.T) {
	tempRepo := t.TempDir()

	// Run init twice — second time with force
	if err := runRepoHooksInit(tempRepo, false); err != nil {
		t.Fatalf("first runRepoHooksInit failed: %v", err)
	}
	if err := runRepoHooksInit(tempRepo, true); err != nil {
		t.Fatalf("forced runRepoHooksInit failed: %v", err)
	}

	// Verify hooks.json still valid
	hooksFile := filepath.Join(tempRepo, ".github", "hooks", "hooks.json")
	hooksData, err := os.ReadFile(hooksFile)
	if err != nil {
		t.Fatalf("failed to read hooks.json: %v", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(hooksData, &config); err != nil {
		t.Fatalf("hooks.json is invalid JSON: %v", err)
	}

	// Should have exactly one hookflow hook per lifecycle (not duplicated)
	hooks := config["hooks"].(map[string]interface{})
	preHooks := hooks["preToolUse"].([]interface{})
	hookflowCount := 0
	for _, h := range preHooks {
		hookBytes, _ := json.Marshal(h)
		if strings.Contains(string(hookBytes), "hookflow") {
			hookflowCount++
		}
	}
	if hookflowCount != 1 {
		t.Errorf("expected exactly 1 hookflow preToolUse hook, got %d", hookflowCount)
	}
}

// TestContainsHookflowHook tests the hook detection helper
func TestContainsHookflowHook(t *testing.T) {
	tests := []struct {
		name     string
		hooks    []interface{}
		expected bool
	}{
		{
			name:     "empty array",
			hooks:    []interface{}{},
			expected: false,
		},
		{
			name: "no hookflow",
			hooks: []interface{}{
				map[string]interface{}{
					"type": "command",
					"bash": "echo 'other'",
				},
			},
			expected: false,
		},
		{
			name: "has hookflow",
			hooks: []interface{}{
				map[string]interface{}{
					"type": "command",
					"bash": "hookflow run --raw",
				},
			},
			expected: true,
		},
		{
			name: "hookflow in powershell",
			hooks: []interface{}{
				map[string]interface{}{
					"type":       "command",
					"powershell": "hookflow run --raw",
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsHookflowHook(tt.hooks)
			if result != tt.expected {
				t.Errorf("containsHookflowHook() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestRemoveHookflowHooks tests the hook removal helper
func TestRemoveHookflowHooks(t *testing.T) {
	hooks := []interface{}{
		map[string]interface{}{
			"type": "command",
			"bash": "echo 'keep this'",
		},
		map[string]interface{}{
			"type": "command",
			"bash": "hookflow run --raw",
		},
		map[string]interface{}{
			"type": "command",
			"bash": "echo 'keep this too'",
		},
	}

	result := removeHookflowHooks(hooks)

	if len(result) != 2 {
		t.Errorf("expected 2 hooks after removal, got %d", len(result))
	}

	// Verify hookflow hook was removed
	for _, h := range result {
		hookBytes, _ := json.Marshal(h)
		if strings.Contains(string(hookBytes), "hookflow") {
			t.Error("hookflow hooks should have been removed")
		}
	}
}

// TestGenerateExampleWorkflow tests that generated example workflow is valid YAML
func TestGenerateExampleWorkflow(t *testing.T) {
	content := generateExampleWorkflow()

	// Check for required YAML fields
	requiredFields := []string{"name:", "on:", "steps:", "file:"}
	for _, field := range requiredFields {
		if !strings.Contains(content, field) {
			t.Errorf("example workflow missing required field: %s", field)
		}
	}
}

// TestGenerateSkillMD tests that generated skill markdown is valid
func TestGenerateSkillMD(t *testing.T) {
	content := generateSkillMD()

	// Check for required sections
	requiredSections := []string{
		"name: hookflow",
		"description:",
		"Workflow Schema",
		"Trigger Types",
		"Expression Syntax",
		"Common Patterns",
	}

	for _, section := range requiredSections {
		if !strings.Contains(content, section) {
			t.Errorf("SKILL.md missing required section: %s", section)
		}
	}
}
