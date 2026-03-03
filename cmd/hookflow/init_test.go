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

	// Verify ~/.copilot/hooks.json exists with hookflow
	hooksPath := filepath.Join(copilotDir, "hooks.json")
	hooksData, err := os.ReadFile(hooksPath)
	if err != nil {
		t.Fatalf("hooks.json not created: %v", err)
	}

	var hooksConfig map[string]interface{}
	if err := json.Unmarshal(hooksData, &hooksConfig); err != nil {
		t.Fatalf("hooks.json is invalid JSON: %v", err)
	}

	// Check hooks structure
	hooks, ok := hooksConfig["hooks"].(map[string]interface{})
	if !ok {
		t.Fatal("hooks.json missing 'hooks' field")
	}

	// Check preToolUse hook exists
	preToolUse, ok := hooks["preToolUse"].([]interface{})
	if !ok || len(preToolUse) == 0 {
		t.Fatal("hooks.json missing preToolUse hooks")
	}

	// Check postToolUse hook exists
	postToolUse, ok := hooks["postToolUse"].([]interface{})
	if !ok || len(postToolUse) == 0 {
		t.Fatal("hooks.json missing postToolUse hooks")
	}

	// Verify hookflow command is in hooks
	hooksStr := string(hooksData)
	if !strings.Contains(hooksStr, "hookflow run") {
		t.Error("hooks.json should contain 'hookflow run' command")
	}

	// Verify ~/.copilot/mcp-config.json exists with hookflow
	mcpPath := filepath.Join(copilotDir, "mcp-config.json")
	mcpData, err := os.ReadFile(mcpPath)
	if err != nil {
		t.Fatalf("mcp-config.json not created: %v", err)
	}

	var mcpConfig map[string]interface{}
	if err := json.Unmarshal(mcpData, &mcpConfig); err != nil {
		t.Fatalf("mcp-config.json is invalid JSON: %v", err)
	}

	// Check mcpServers structure
	mcpServers, ok := mcpConfig["mcpServers"].(map[string]interface{})
	if !ok {
		t.Fatal("mcp-config.json missing 'mcpServers' field")
	}

	// Check hookflow server exists
	if _, ok := mcpServers["hookflow"]; !ok {
		t.Fatal("mcp-config.json missing 'hookflow' server")
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

// TestInitMerge tests that init preserves existing configuration entries
func TestInitMerge(t *testing.T) {
	tempHome := t.TempDir()
	copilotDir := filepath.Join(tempHome, ".copilot")

	// Create copilot dir
	if err := os.MkdirAll(copilotDir, 0755); err != nil {
		t.Fatalf("failed to create copilot dir: %v", err)
	}

	// Create existing mcp-config.json with another server
	existingMCP := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			"other-server": map[string]interface{}{
				"type":    "stdio",
				"command": "other-tool",
				"args":    []string{"serve"},
			},
		},
	}

	existingMCPBytes, _ := json.MarshalIndent(existingMCP, "", "  ")
	mcpPath := filepath.Join(copilotDir, "mcp-config.json")
	if err := os.WriteFile(mcpPath, existingMCPBytes, 0644); err != nil {
		t.Fatalf("failed to create existing mcp-config.json: %v", err)
	}

	// Create existing hooks.json with another hook
	existingHooks := map[string]interface{}{
		"version": 1,
		"hooks": map[string]interface{}{
			"preToolUse": []interface{}{
				map[string]interface{}{
					"type":    "command",
					"bash":    "echo 'existing hook'",
					"comment": "existing-hook",
				},
			},
		},
	}

	existingHooksBytes, _ := json.MarshalIndent(existingHooks, "", "  ")
	hooksPath := filepath.Join(copilotDir, "hooks.json")
	if err := os.WriteFile(hooksPath, existingHooksBytes, 0644); err != nil {
		t.Fatalf("failed to create existing hooks.json: %v", err)
	}

	// Run init
	if err := runGlobalInit(copilotDir, false); err != nil {
		t.Fatalf("runGlobalInit failed: %v", err)
	}

	// Verify other-server is preserved in mcp-config.json
	mcpData, err := os.ReadFile(mcpPath)
	if err != nil {
		t.Fatalf("failed to read mcp-config.json: %v", err)
	}

	var mcpConfig map[string]interface{}
	if err := json.Unmarshal(mcpData, &mcpConfig); err != nil {
		t.Fatalf("mcp-config.json is invalid JSON: %v", err)
	}

	mcpServers, ok := mcpConfig["mcpServers"].(map[string]interface{})
	if !ok {
		t.Fatal("mcp-config.json missing 'mcpServers' field")
	}

	// Check other-server is preserved
	if _, ok := mcpServers["other-server"]; !ok {
		t.Error("existing 'other-server' should be preserved in mcp-config.json")
	}

	// Check hookflow server is added
	if _, ok := mcpServers["hookflow"]; !ok {
		t.Error("'hookflow' server should be added to mcp-config.json")
	}

	// Verify hooks.json has both existing and hookflow hooks
	hooksData, err := os.ReadFile(hooksPath)
	if err != nil {
		t.Fatalf("failed to read hooks.json: %v", err)
	}

	var hooksConfig map[string]interface{}
	if err := json.Unmarshal(hooksData, &hooksConfig); err != nil {
		t.Fatalf("hooks.json is invalid JSON: %v", err)
	}

	hooks, ok := hooksConfig["hooks"].(map[string]interface{})
	if !ok {
		t.Fatal("hooks.json missing 'hooks' field")
	}

	preToolUse, ok := hooks["preToolUse"].([]interface{})
	if !ok {
		t.Fatal("hooks.json missing 'preToolUse' field")
	}

	// Check both hooks exist (existing + hookflow)
	hooksStr := string(hooksData)
	if !strings.Contains(hooksStr, "existing hook") {
		t.Error("existing hook should be preserved in hooks.json")
	}
	if !strings.Contains(hooksStr, "hookflow run") {
		t.Error("hookflow hook should be added to hooks.json")
	}

	// Count hooks - should have 2
	if len(preToolUse) < 2 {
		t.Errorf("expected at least 2 preToolUse hooks, got %d", len(preToolUse))
	}
}

// TestInitForce tests that --force overwrites existing hookflow entries
func TestInitForce(t *testing.T) {
	tempHome := t.TempDir()
	copilotDir := filepath.Join(tempHome, ".copilot")

	// Create copilot dir
	if err := os.MkdirAll(copilotDir, 0755); err != nil {
		t.Fatalf("failed to create copilot dir: %v", err)
	}

	// Create existing mcp-config.json with old hookflow config
	existingMCP := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			"hookflow": map[string]interface{}{
				"type":    "stdio",
				"command": "old-hookflow",
				"args":    []string{"old-serve"},
			},
		},
	}

	existingMCPBytes, _ := json.MarshalIndent(existingMCP, "", "  ")
	mcpPath := filepath.Join(copilotDir, "mcp-config.json")
	if err := os.WriteFile(mcpPath, existingMCPBytes, 0644); err != nil {
		t.Fatalf("failed to create existing mcp-config.json: %v", err)
	}

	// Run init with force
	if err := runGlobalInit(copilotDir, true); err != nil {
		t.Fatalf("runGlobalInit with force failed: %v", err)
	}

	// Verify hookflow config was updated
	mcpData, err := os.ReadFile(mcpPath)
	if err != nil {
		t.Fatalf("failed to read mcp-config.json: %v", err)
	}

	if strings.Contains(string(mcpData), "old-hookflow") {
		t.Error("old hookflow config should be replaced with --force")
	}
	if !strings.Contains(string(mcpData), "mcp") {
		t.Error("new hookflow config should include 'mcp' args")
	}
}

// TestInitSkipsExisting tests that init skips files when they already exist (no force)
func TestInitSkipsExisting(t *testing.T) {
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

// TestInitInvalidJSONBackup tests that invalid JSON files are backed up
func TestInitInvalidJSONBackup(t *testing.T) {
	tempHome := t.TempDir()
	copilotDir := filepath.Join(tempHome, ".copilot")

	// Create copilot dir
	if err := os.MkdirAll(copilotDir, 0755); err != nil {
		t.Fatalf("failed to create copilot dir: %v", err)
	}

	// Create invalid mcp-config.json
	mcpPath := filepath.Join(copilotDir, "mcp-config.json")
	if err := os.WriteFile(mcpPath, []byte("invalid json {{{"), 0644); err != nil {
		t.Fatalf("failed to create invalid mcp-config.json: %v", err)
	}

	// Run init
	if err := runGlobalInit(copilotDir, false); err != nil {
		t.Fatalf("runGlobalInit failed: %v", err)
	}

	// Verify backup was created
	backupPath := mcpPath + ".bak"
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		t.Error("invalid JSON file should have been backed up")
	}

	// Verify new valid JSON was created
	mcpData, err := os.ReadFile(mcpPath)
	if err != nil {
		t.Fatalf("failed to read new mcp-config.json: %v", err)
	}

	var mcpConfig map[string]interface{}
	if err := json.Unmarshal(mcpData, &mcpConfig); err != nil {
		t.Error("new mcp-config.json should be valid JSON")
	}
}
