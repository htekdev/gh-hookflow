package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestInitGlobal tests that hookflow init creates global configuration files
func TestInitGlobal(t *testing.T) {
	// Create temp home directory
	tempHome := t.TempDir()
	copilotDir := filepath.Join(tempHome, ".copilot")

	// Run global init
	if err := runGlobalInit(copilotDir, false); err != nil {
		t.Fatalf("runGlobalInit failed: %v", err)
	}

	// Verify ~/.copilot/skills/hookflow/SKILL.md exists
	skillPath := filepath.Join(copilotDir, "skills", "hookflow", "SKILL.md")
	if _, err := os.Stat(skillPath); os.IsNotExist(err) {
		t.Fatal("SKILL.md not created")
	}

	skillData, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("failed to read SKILL.md: %v", err)
	}

	if !strings.Contains(string(skillData), "hookflow") {
		t.Error("SKILL.md should contain 'hookflow'")
	}

	// Verify hooks.json is NOT created (global hooks moved to plugin)
	hooksPath := filepath.Join(copilotDir, "hooks.json")
	if _, err := os.Stat(hooksPath); err == nil {
		t.Error("global hooks.json should NOT be created (moved to plugin)")
	}

	// Verify mcp-config.json is NOT created (removed from init)
	mcpPath := filepath.Join(copilotDir, "mcp-config.json")
	if _, err := os.Stat(mcpPath); err == nil {
		t.Error("mcp-config.json should NOT be created (removed from init)")
	}
}

// TestInitRepo tests that hookflow init --repo creates per-repo configuration
func TestInitRepo(t *testing.T) {
	// Create temp directories
	tempHome := t.TempDir()
	tempRepo := t.TempDir()
	copilotDir := filepath.Join(tempHome, ".copilot")

	// Run global init first
	if err := runGlobalInit(copilotDir, false); err != nil {
		t.Fatalf("runGlobalInit failed: %v", err)
	}

	// Run repo hooks init and scaffold init
	if err := runRepoHooksInit(tempRepo, false); err != nil {
		t.Fatalf("runRepoHooksInit failed: %v", err)
	}
	if err := runRepoScaffoldInit(tempRepo, false); err != nil {
		t.Fatalf("runRepoScaffoldInit failed: %v", err)
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

// TestInitMerge tests that init preserves existing skill configuration
func TestInitMerge(t *testing.T) {
	tempHome := t.TempDir()
	copilotDir := filepath.Join(tempHome, ".copilot")

	// Create copilot dir and existing skill
	skillDir := filepath.Join(copilotDir, "skills", "hookflow")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("failed to create skill dir: %v", err)
	}

	// Create existing SKILL.md - should be preserved without force
	skillPath := filepath.Join(skillDir, "SKILL.md")
	customContent := "# Custom SKILL.md\nThis should be preserved"
	if err := os.WriteFile(skillPath, []byte(customContent), 0644); err != nil {
		t.Fatalf("failed to create custom SKILL.md: %v", err)
	}

	// Run init without force
	if err := runGlobalInit(copilotDir, false); err != nil {
		t.Fatalf("runGlobalInit failed: %v", err)
	}

	// Verify custom SKILL.md is preserved
	skillData, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("failed to read SKILL.md: %v", err)
	}

	if string(skillData) != customContent {
		t.Error("custom SKILL.md should be preserved without --force")
	}
}

// TestInitForce tests that --force overwrites existing skill
func TestInitForce(t *testing.T) {
	tempHome := t.TempDir()
	copilotDir := filepath.Join(tempHome, ".copilot")

	// Create copilot dir and existing skill
	skillDir := filepath.Join(copilotDir, "skills", "hookflow")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("failed to create skill dir: %v", err)
	}

	// Create existing SKILL.md with old content
	skillPath := filepath.Join(skillDir, "SKILL.md")
	oldContent := "# Old SKILL.md\nThis should be overwritten with --force"
	if err := os.WriteFile(skillPath, []byte(oldContent), 0644); err != nil {
		t.Fatalf("failed to create old SKILL.md: %v", err)
	}

	// Run init with force
	if err := runGlobalInit(copilotDir, true); err != nil {
		t.Fatalf("runGlobalInit with force failed: %v", err)
	}

	// Verify SKILL.md was overwritten
	skillData, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("failed to read SKILL.md: %v", err)
	}

	if strings.Contains(string(skillData), "Old SKILL.md") {
		t.Error("old SKILL.md content should be replaced with --force")
	}
	if !strings.Contains(string(skillData), "hookflow") {
		t.Error("new SKILL.md should contain 'hookflow'")
	}
}

// TestInitSkipsExistingSkill tests that init skips SKILL.md when it already exists (no force)
func TestInitSkipsExistingSkill(t *testing.T) {
	tempHome := t.TempDir()
	copilotDir := filepath.Join(tempHome, ".copilot")
	skillDir := filepath.Join(copilotDir, "skills", "hookflow")

	// Create directories
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("failed to create skill dir: %v", err)
	}

	// Create existing SKILL.md with custom content
	skillPath := filepath.Join(skillDir, "SKILL.md")
	customContent := "# Custom SKILL.md\nThis should be preserved"
	if err := os.WriteFile(skillPath, []byte(customContent), 0644); err != nil {
		t.Fatalf("failed to create custom SKILL.md: %v", err)
	}

	// Run init without force
	if err := runGlobalInit(copilotDir, false); err != nil {
		t.Fatalf("runGlobalInit failed: %v", err)
	}

	// Verify custom SKILL.md is preserved
	skillData, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("failed to read SKILL.md: %v", err)
	}

	if string(skillData) != customContent {
		t.Error("custom SKILL.md should be preserved without --force")
	}
}

// TestInitCreatesDirectories tests that init creates necessary directories
func TestInitCreatesDirectories(t *testing.T) {
	tempHome := t.TempDir()
	copilotDir := filepath.Join(tempHome, ".copilot")

	// Verify directory doesn't exist
	if _, err := os.Stat(copilotDir); !os.IsNotExist(err) {
		t.Fatal("copilot dir should not exist initially")
	}

	// Run init
	if err := runGlobalInit(copilotDir, false); err != nil {
		t.Fatalf("runGlobalInit failed: %v", err)
	}

	// Verify directories were created
	dirs := []string{
		copilotDir,
		filepath.Join(copilotDir, "skills"),
		filepath.Join(copilotDir, "skills", "hookflow"),
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

// TestInitDoesNotCreateGlobalHooksOrMCP tests that init no longer creates global hooks.json or mcp-config.json
func TestInitDoesNotCreateGlobalHooksOrMCP(t *testing.T) {
	tempHome := t.TempDir()
	copilotDir := filepath.Join(tempHome, ".copilot")

	// Run init
	if err := runGlobalInit(copilotDir, false); err != nil {
		t.Fatalf("runGlobalInit failed: %v", err)
	}

	// Verify hooks.json does NOT exist
	hooksPath := filepath.Join(copilotDir, "hooks.json")
	if _, err := os.Stat(hooksPath); err == nil {
		t.Error("global hooks.json should NOT be created - hooks are delivered via plugin")
	}

	// Verify mcp-config.json does NOT exist
	mcpPath := filepath.Join(copilotDir, "mcp-config.json")
	if _, err := os.Stat(mcpPath); err == nil {
		t.Error("mcp-config.json should NOT be created - removed from init")
	}

	// Verify SKILL.md still IS created
	skillPath := filepath.Join(copilotDir, "skills", "hookflow", "SKILL.md")
	if _, err := os.Stat(skillPath); os.IsNotExist(err) {
		t.Error("SKILL.md should still be created by init")
	}
}
