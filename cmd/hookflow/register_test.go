package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRegisterCreatesPersonalHooksAndSkill tests the happy path: register creates both hooks and skill
func TestRegisterCreatesPersonalHooksAndSkill(t *testing.T) {
	homeDir := t.TempDir()
	origHome := overrideHomeDir(t, homeDir)
	defer restoreHomeDir(origHome)

	if err := runRegister(false, false, false); err != nil {
		t.Fatalf("runRegister failed: %v", err)
	}

	// Verify hooks.json exists
	hooksPath := filepath.Join(homeDir, ".copilot", "hooks", "hooks.json")
	assertFileExists(t, hooksPath)

	// Verify hooks.json content
	hooksData, err := os.ReadFile(hooksPath)
	if err != nil {
		t.Fatalf("failed to read hooks.json: %v", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(hooksData, &config); err != nil {
		t.Fatalf("hooks.json is invalid JSON: %v", err)
	}

	hooks := config["hooks"].(map[string]interface{})

	// Verify all three hook types exist
	for _, hookType := range []string{"preToolUse", "postToolUse", "sessionStart"} {
		arr, ok := hooks[hookType].([]interface{})
		if !ok || len(arr) == 0 {
			t.Errorf("hooks.json missing %s hooks", hookType)
		}
	}

	// Verify --global flag in preToolUse and postToolUse
	hooksStr := string(hooksData)
	if !strings.Contains(hooksStr, "--global") {
		t.Error("personal hooks should contain --global flag")
	}

	// Verify SKILL.md exists
	skillPath := filepath.Join(homeDir, ".copilot", "skills", "hookflow", "SKILL.md")
	assertFileExists(t, skillPath)

	// Verify SKILL.md content
	skillData, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("failed to read SKILL.md: %v", err)
	}
	skillContent := string(skillData)

	requiredSections := []string{
		"name: hookflow",
		"description:",
		"Hookify Rules (Recommended)",
		"YAML Workflows (Advanced)",
		"Trigger Types",
		"Expression Syntax",
		"Transcript Functions",
		"Step Outcome Functions",
		"hookflow register",
	}

	for _, section := range requiredSections {
		if !strings.Contains(skillContent, section) {
			t.Errorf("SKILL.md missing required section: %s", section)
		}
	}
}

// TestRegisterIdempotent tests that running register twice doesn't duplicate hooks
func TestRegisterIdempotent(t *testing.T) {
	homeDir := t.TempDir()
	origHome := overrideHomeDir(t, homeDir)
	defer restoreHomeDir(origHome)

	// Register twice
	if err := runRegister(false, false, false); err != nil {
		t.Fatalf("first runRegister failed: %v", err)
	}
	if err := runRegister(false, false, false); err != nil {
		t.Fatalf("second runRegister failed: %v", err)
	}

	// Read hooks.json
	hooksPath := filepath.Join(homeDir, ".copilot", "hooks", "hooks.json")
	hooksData, err := os.ReadFile(hooksPath)
	if err != nil {
		t.Fatalf("failed to read hooks.json: %v", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(hooksData, &config); err != nil {
		t.Fatalf("hooks.json is invalid JSON: %v", err)
	}

	hooks := config["hooks"].(map[string]interface{})

	// Verify exactly one hookflow hook per lifecycle
	for _, hookType := range []string{"preToolUse", "postToolUse", "sessionStart"} {
		arr := hooks[hookType].([]interface{})
		hookflowCount := 0
		for _, h := range arr {
			hookBytes, _ := json.Marshal(h)
			if strings.Contains(string(hookBytes), "hookflow") {
				hookflowCount++
			}
		}
		if hookflowCount != 1 {
			t.Errorf("expected exactly 1 hookflow %s hook, got %d", hookType, hookflowCount)
		}
	}
}

// TestRegisterPreservesExistingHooks tests that non-hookflow hooks are preserved
func TestRegisterPreservesExistingHooks(t *testing.T) {
	homeDir := t.TempDir()
	origHome := overrideHomeDir(t, homeDir)
	defer restoreHomeDir(origHome)

	// Create existing hooks.json with custom hooks
	hooksDir := filepath.Join(homeDir, ".copilot", "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		t.Fatalf("failed to create hooks dir: %v", err)
	}

	existingConfig := map[string]interface{}{
		"version": 1,
		"hooks": map[string]interface{}{
			"preToolUse": []interface{}{
				map[string]interface{}{
					"type": "command",
					"bash": "echo 'my custom pre hook'",
				},
			},
			"postToolUse": []interface{}{
				map[string]interface{}{
					"type": "command",
					"bash": "echo 'my custom post hook'",
				},
			},
		},
	}
	existingJSON, _ := json.MarshalIndent(existingConfig, "", "  ")
	hooksFile := filepath.Join(hooksDir, "hooks.json")
	if err := os.WriteFile(hooksFile, existingJSON, 0644); err != nil {
		t.Fatalf("failed to create hooks.json: %v", err)
	}

	// Register hookflow
	if err := runRegister(false, false, false); err != nil {
		t.Fatalf("runRegister failed: %v", err)
	}

	// Read merged hooks.json
	hooksData, err := os.ReadFile(hooksFile)
	if err != nil {
		t.Fatalf("failed to read hooks.json: %v", err)
	}

	hooksStr := string(hooksData)

	// Custom hooks should be preserved
	if !strings.Contains(hooksStr, "my custom pre hook") {
		t.Error("existing custom preToolUse hook should be preserved")
	}
	if !strings.Contains(hooksStr, "my custom post hook") {
		t.Error("existing custom postToolUse hook should be preserved")
	}

	// Hookflow hooks should also be present
	if !strings.Contains(hooksStr, "hookflow run") {
		t.Error("hookflow hooks should be added")
	}
}

// TestRegisterReplacesOldHookflowHooks tests that outdated hookflow hooks are replaced
func TestRegisterReplacesOldHookflowHooks(t *testing.T) {
	homeDir := t.TempDir()
	origHome := overrideHomeDir(t, homeDir)
	defer restoreHomeDir(origHome)

	// Create existing hooks.json with OLD hookflow hooks (e.g., without --global)
	hooksDir := filepath.Join(homeDir, ".copilot", "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		t.Fatalf("failed to create hooks dir: %v", err)
	}

	oldConfig := map[string]interface{}{
		"version": 1,
		"hooks": map[string]interface{}{
			"preToolUse": []interface{}{
				map[string]interface{}{
					"type": "command",
					"bash": "gh hookflow run --raw --event-type preToolUse",
				},
			},
			"postToolUse": []interface{}{
				map[string]interface{}{
					"type": "command",
					"bash": "gh hookflow run --raw --event-type postToolUse",
				},
			},
		},
	}
	oldJSON, _ := json.MarshalIndent(oldConfig, "", "  ")
	hooksFile := filepath.Join(hooksDir, "hooks.json")
	if err := os.WriteFile(hooksFile, oldJSON, 0644); err != nil {
		t.Fatalf("failed to create hooks.json: %v", err)
	}

	// Register (should replace old hooks with new ones that have --global)
	if err := runRegister(false, false, false); err != nil {
		t.Fatalf("runRegister failed: %v", err)
	}

	// Read updated hooks.json
	hooksData, err := os.ReadFile(hooksFile)
	if err != nil {
		t.Fatalf("failed to read hooks.json: %v", err)
	}

	hooksStr := string(hooksData)

	// New hooks should have --global flag
	if !strings.Contains(hooksStr, "--global") {
		t.Error("updated hooks should contain --global flag")
	}

	// Verify no duplicate hookflow hooks
	var config map[string]interface{}
	if err := json.Unmarshal(hooksData, &config); err != nil {
		t.Fatalf("hooks.json is invalid JSON: %v", err)
	}
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
		t.Errorf("expected exactly 1 hookflow preToolUse hook after replacement, got %d", hookflowCount)
	}
}

// TestUnregisterRemovesHookflowHooksPreservesOthers tests that unregister only removes hookflow hooks
func TestUnregisterRemovesHookflowHooksPreservesOthers(t *testing.T) {
	homeDir := t.TempDir()
	origHome := overrideHomeDir(t, homeDir)
	defer restoreHomeDir(origHome)

	// Create hooks.json with both hookflow and custom hooks
	hooksDir := filepath.Join(homeDir, ".copilot", "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		t.Fatalf("failed to create hooks dir: %v", err)
	}

	config := map[string]interface{}{
		"version": 1,
		"hooks": map[string]interface{}{
			"preToolUse": []interface{}{
				map[string]interface{}{
					"type": "command",
					"bash": "echo 'custom hook'",
				},
				map[string]interface{}{
					"type":       "command",
					"bash":       "gh hookflow run --raw --event-type preToolUse --global",
					"powershell": "gh hookflow run --raw --event-type preToolUse --global",
				},
			},
			"postToolUse": []interface{}{
				map[string]interface{}{
					"type":       "command",
					"bash":       "gh hookflow run --raw --event-type postToolUse --global",
					"powershell": "gh hookflow run --raw --event-type postToolUse --global",
				},
			},
			"sessionStart": []interface{}{
				map[string]interface{}{
					"type": "command",
					"bash": "gh hookflow check-setup",
				},
			},
		},
	}
	configJSON, _ := json.MarshalIndent(config, "", "  ")
	hooksFile := filepath.Join(hooksDir, "hooks.json")
	if err := os.WriteFile(hooksFile, configJSON, 0644); err != nil {
		t.Fatalf("failed to create hooks.json: %v", err)
	}

	// Also create skill
	skillDir := filepath.Join(homeDir, ".copilot", "skills", "hookflow")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("failed to create skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create SKILL.md: %v", err)
	}

	// Unregister
	if err := runRegister(true, false, false); err != nil {
		t.Fatalf("runRegister --unregister failed: %v", err)
	}

	// Verify hooks.json still exists (has custom hook)
	hooksData, err := os.ReadFile(hooksFile)
	if err != nil {
		t.Fatalf("failed to read hooks.json: %v", err)
	}

	hooksStr := string(hooksData)

	// Custom hook should be preserved
	if !strings.Contains(hooksStr, "custom hook") {
		t.Error("custom hook should be preserved after unregister")
	}

	// Hookflow hooks should be removed
	if strings.Contains(hooksStr, "hookflow run") {
		t.Error("hookflow run hooks should be removed after unregister")
	}
	if strings.Contains(hooksStr, "hookflow check-setup") {
		t.Error("hookflow check-setup hook should be removed after unregister")
	}

	// Skill directory should be removed
	skillPath := filepath.Join(homeDir, ".copilot", "skills", "hookflow")
	if _, err := os.Stat(skillPath); !os.IsNotExist(err) {
		t.Error("skill directory should be removed after unregister")
	}
}

// TestUnregisterRemovesHooksFileWhenEmpty tests that hooks.json is deleted when no hooks remain
func TestUnregisterRemovesHooksFileWhenEmpty(t *testing.T) {
	homeDir := t.TempDir()
	origHome := overrideHomeDir(t, homeDir)
	defer restoreHomeDir(origHome)

	// Register first
	if err := runRegister(false, false, false); err != nil {
		t.Fatalf("runRegister failed: %v", err)
	}

	// Unregister
	if err := runRegister(true, false, false); err != nil {
		t.Fatalf("runRegister --unregister failed: %v", err)
	}

	// hooks.json should be removed since no hooks remain
	hooksFile := filepath.Join(homeDir, ".copilot", "hooks", "hooks.json")
	if _, err := os.Stat(hooksFile); !os.IsNotExist(err) {
		t.Error("hooks.json should be removed when no hooks remain")
	}
}

// TestRegisterHooksOnly tests the --hooks-only flag
func TestRegisterHooksOnly(t *testing.T) {
	homeDir := t.TempDir()
	origHome := overrideHomeDir(t, homeDir)
	defer restoreHomeDir(origHome)

	if err := runRegister(false, true, false); err != nil {
		t.Fatalf("runRegister --hooks-only failed: %v", err)
	}

	// Hooks should exist
	hooksPath := filepath.Join(homeDir, ".copilot", "hooks", "hooks.json")
	assertFileExists(t, hooksPath)

	// Skill should NOT exist
	skillPath := filepath.Join(homeDir, ".copilot", "skills", "hookflow", "SKILL.md")
	if _, err := os.Stat(skillPath); !os.IsNotExist(err) {
		t.Error("SKILL.md should not be created with --hooks-only")
	}
}

// TestRegisterSkillOnly tests the --skill-only flag
func TestRegisterSkillOnly(t *testing.T) {
	homeDir := t.TempDir()
	origHome := overrideHomeDir(t, homeDir)
	defer restoreHomeDir(origHome)

	if err := runRegister(false, false, true); err != nil {
		t.Fatalf("runRegister --skill-only failed: %v", err)
	}

	// Skill should exist
	skillPath := filepath.Join(homeDir, ".copilot", "skills", "hookflow", "SKILL.md")
	assertFileExists(t, skillPath)

	// Hooks should NOT exist
	hooksPath := filepath.Join(homeDir, ".copilot", "hooks", "hooks.json")
	if _, err := os.Stat(hooksPath); !os.IsNotExist(err) {
		t.Error("hooks.json should not be created with --skill-only")
	}
}

// TestRegisterHandlesInvalidExistingJSON tests that invalid existing hooks.json is backed up
func TestRegisterHandlesInvalidExistingJSON(t *testing.T) {
	homeDir := t.TempDir()
	origHome := overrideHomeDir(t, homeDir)
	defer restoreHomeDir(origHome)

	// Create invalid hooks.json
	hooksDir := filepath.Join(homeDir, ".copilot", "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		t.Fatalf("failed to create hooks dir: %v", err)
	}

	hooksFile := filepath.Join(hooksDir, "hooks.json")
	if err := os.WriteFile(hooksFile, []byte("not valid json{{{"), 0644); err != nil {
		t.Fatalf("failed to create invalid hooks.json: %v", err)
	}

	// Register should succeed (backs up invalid file)
	if err := runRegister(false, true, false); err != nil {
		t.Fatalf("runRegister with invalid existing JSON failed: %v", err)
	}

	// Backup should exist
	bakFile := hooksFile + ".bak"
	assertFileExists(t, bakFile)

	// New hooks.json should be valid
	hooksData, err := os.ReadFile(hooksFile)
	if err != nil {
		t.Fatalf("failed to read hooks.json: %v", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(hooksData, &config); err != nil {
		t.Fatalf("new hooks.json should be valid JSON: %v", err)
	}
}

// TestUnregisterNoExistingFiles tests that unregister handles missing files gracefully
func TestUnregisterNoExistingFiles(t *testing.T) {
	homeDir := t.TempDir()
	origHome := overrideHomeDir(t, homeDir)
	defer restoreHomeDir(origHome)

	// Unregister when nothing exists should not error
	if err := runRegister(true, false, false); err != nil {
		t.Fatalf("runRegister --unregister with no files should not error: %v", err)
	}
}

// TestRegisterSessionStartHookContent tests that sessionStart hook references hookflow register
func TestRegisterSessionStartHookContent(t *testing.T) {
	homeDir := t.TempDir()
	origHome := overrideHomeDir(t, homeDir)
	defer restoreHomeDir(origHome)

	if err := runRegister(false, true, false); err != nil {
		t.Fatalf("runRegister failed: %v", err)
	}

	hooksPath := filepath.Join(homeDir, ".copilot", "hooks", "hooks.json")
	hooksData, err := os.ReadFile(hooksPath)
	if err != nil {
		t.Fatalf("failed to read hooks.json: %v", err)
	}

	hooksStr := string(hooksData)

	// sessionStart should reference "hookflow register" not plugin install
	if !strings.Contains(hooksStr, "hookflow register") {
		t.Error("sessionStart hook should reference 'hookflow register'")
	}
	if strings.Contains(hooksStr, "copilot plugin install") {
		t.Error("sessionStart hook should NOT reference 'copilot plugin install'")
	}
}

// TestRegisterPersonalHooksHaveGlobalFlag tests that personal hooks use --global flag
func TestRegisterPersonalHooksHaveGlobalFlag(t *testing.T) {
	homeDir := t.TempDir()
	origHome := overrideHomeDir(t, homeDir)
	defer restoreHomeDir(origHome)

	if err := runRegister(false, true, false); err != nil {
		t.Fatalf("runRegister failed: %v", err)
	}

	hooksPath := filepath.Join(homeDir, ".copilot", "hooks", "hooks.json")
	hooksData, err := os.ReadFile(hooksPath)
	if err != nil {
		t.Fatalf("failed to read hooks.json: %v", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(hooksData, &config); err != nil {
		t.Fatalf("hooks.json is invalid JSON: %v", err)
	}

	hooks := config["hooks"].(map[string]interface{})

	// Check preToolUse has --global in bash and powershell
	preHooks := hooks["preToolUse"].([]interface{})
	found := false
	for _, h := range preHooks {
		hookMap, ok := h.(map[string]interface{})
		if !ok {
			continue
		}
		bashCmd, _ := hookMap["bash"].(string)
		psCmd, _ := hookMap["powershell"].(string)
		if strings.Contains(bashCmd, "--global") && strings.Contains(psCmd, "--global") {
			found = true
			break
		}
	}
	if !found {
		t.Error("preToolUse hook should have --global flag in both bash and powershell commands")
	}

	// Check postToolUse has --global
	postHooks := hooks["postToolUse"].([]interface{})
	found = false
	for _, h := range postHooks {
		hookMap, ok := h.(map[string]interface{})
		if !ok {
			continue
		}
		bashCmd, _ := hookMap["bash"].(string)
		psCmd, _ := hookMap["powershell"].(string)
		if strings.Contains(bashCmd, "--global") && strings.Contains(psCmd, "--global") {
			found = true
			break
		}
	}
	if !found {
		t.Error("postToolUse hook should have --global flag in both bash and powershell commands")
	}
}

// TestGenerateRegisterSkillMD tests that the generated skill markdown has all required sections
func TestGenerateRegisterSkillMD(t *testing.T) {
	content := generateRegisterSkillMD()

	requiredSections := []string{
		// Frontmatter
		"name: hookflow",
		"description:",
		// Top-level structure
		"Quick Reference",
		"Hookify Rules (Recommended)",
		"YAML Workflows (Advanced)",
		"Troubleshooting",
		// Hookify sections
		"Hookify Schema",
		"Frontmatter Fields",
		"Event Types",
		"Pattern vs Conditions",
		"Condition Fields",
		"Condition Operators",
		"Tool Matcher",
		"Disabling Rules",
		"Message Body",
		"Hookify Examples",
		"Block Dangerous Commands",
		"Block Sensitive File Types",
		"Require Tests Before Commit",
		"Warn on console.log",
		"Block Recursive Delete",
		"Post-Lifecycle Advisory",
		"Block Hardcoded Secrets",
		// YAML sections
		"YAML Workflow Schema",
		"Required Fields",
		"Optional Fields",
		"Trigger Types",
		"File Trigger",
		"Tool Trigger",
		"Multiple Tool Triggers",
		"Commit Trigger",
		"Push Trigger",
		"Expression Syntax",
		"String Functions",
		"Step Outcome Functions",
		"Transcript Functions",
		"Step Configuration",
		"Reusable Actions",
		"YAML Workflow Examples",
		"Validate JSON",
		"Conventional Commits",
		"Protect Hookflow",
		// Commands
		"hookflow register",
		"hookflow init",
		"hookflow validate",
		// Key features
		"concurrency",
		"continue-on-error",
		"transcript_count",
		"transcript_since",
		"transcript_last",
		"success()",
		"failure()",
		"always()",
		"git-push",
	}

	for _, section := range requiredSections {
		if !strings.Contains(content, section) {
			t.Errorf("SKILL.md missing required content: %s", section)
		}
	}
}

// TestUnregisterHooksOnlyPreservesSkill tests that --hooks-only unregister keeps skill
func TestUnregisterHooksOnlyPreservesSkill(t *testing.T) {
	homeDir := t.TempDir()
	origHome := overrideHomeDir(t, homeDir)
	defer restoreHomeDir(origHome)

	// Register both
	if err := runRegister(false, false, false); err != nil {
		t.Fatalf("runRegister failed: %v", err)
	}

	// Unregister hooks only
	if err := runRegister(true, true, false); err != nil {
		t.Fatalf("runRegister --unregister --hooks-only failed: %v", err)
	}

	// Hooks should be gone
	hooksFile := filepath.Join(homeDir, ".copilot", "hooks", "hooks.json")
	if _, err := os.Stat(hooksFile); !os.IsNotExist(err) {
		t.Error("hooks.json should be removed after --hooks-only unregister")
	}

	// Skill should still exist
	skillPath := filepath.Join(homeDir, ".copilot", "skills", "hookflow", "SKILL.md")
	assertFileExists(t, skillPath)
}

// TestUnregisterSkillOnlyPreservesHooks tests that --skill-only unregister keeps hooks
func TestUnregisterSkillOnlyPreservesHooks(t *testing.T) {
	homeDir := t.TempDir()
	origHome := overrideHomeDir(t, homeDir)
	defer restoreHomeDir(origHome)

	// Register both
	if err := runRegister(false, false, false); err != nil {
		t.Fatalf("runRegister failed: %v", err)
	}

	// Unregister skill only
	if err := runRegister(true, false, true); err != nil {
		t.Fatalf("runRegister --unregister --skill-only failed: %v", err)
	}

	// Hooks should still exist
	hooksFile := filepath.Join(homeDir, ".copilot", "hooks", "hooks.json")
	assertFileExists(t, hooksFile)

	// Skill should be gone
	skillPath := filepath.Join(homeDir, ".copilot", "skills", "hookflow", "SKILL.md")
	if _, err := os.Stat(skillPath); !os.IsNotExist(err) {
		t.Error("SKILL.md should be removed after --skill-only unregister")
	}
}

// TestMergePersonalHooksJSONVersionField tests that version field is preserved
func TestMergePersonalHooksJSONVersionField(t *testing.T) {
	homeDir := t.TempDir()
	origHome := overrideHomeDir(t, homeDir)
	defer restoreHomeDir(origHome)

	if err := runRegister(false, true, false); err != nil {
		t.Fatalf("runRegister failed: %v", err)
	}

	hooksPath := filepath.Join(homeDir, ".copilot", "hooks", "hooks.json")
	hooksData, err := os.ReadFile(hooksPath)
	if err != nil {
		t.Fatalf("failed to read hooks.json: %v", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(hooksData, &config); err != nil {
		t.Fatalf("hooks.json is invalid JSON: %v", err)
	}

	version, ok := config["version"]
	if !ok {
		t.Error("hooks.json should have version field")
	}
	if v, ok := version.(float64); !ok || v != 1 {
		t.Errorf("hooks.json version should be 1, got %v", version)
	}
}

// TestRegisterHooksTimeout tests that hooks have proper timeout values
func TestRegisterHooksTimeout(t *testing.T) {
	homeDir := t.TempDir()
	origHome := overrideHomeDir(t, homeDir)
	defer restoreHomeDir(origHome)

	if err := runRegister(false, true, false); err != nil {
		t.Fatalf("runRegister failed: %v", err)
	}

	hooksPath := filepath.Join(homeDir, ".copilot", "hooks", "hooks.json")
	hooksData, err := os.ReadFile(hooksPath)
	if err != nil {
		t.Fatalf("failed to read hooks.json: %v", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(hooksData, &config); err != nil {
		t.Fatalf("hooks.json is invalid JSON: %v", err)
	}

	hooks := config["hooks"].(map[string]interface{})

	for _, hookType := range []string{"preToolUse", "postToolUse", "sessionStart"} {
		arr := hooks[hookType].([]interface{})
		for _, h := range arr {
			hookMap := h.(map[string]interface{})
			timeout, ok := hookMap["timeoutSec"]
			if !ok {
				t.Errorf("%s hook missing timeoutSec", hookType)
			}
			if v, ok := timeout.(float64); !ok || v != 1800 {
				t.Errorf("%s hook timeoutSec should be 1800, got %v", hookType, timeout)
			}
		}
	}
}

// --- Test Helpers ---

// overrideHomeDir temporarily overrides HOME/USERPROFILE for testing.
// Returns the original value to restore later.
func overrideHomeDir(t *testing.T, newHome string) string {
	t.Helper()
	origHome := os.Getenv("HOME")
	origUserProfile := os.Getenv("USERPROFILE")

	t.Setenv("HOME", newHome)
	t.Setenv("USERPROFILE", newHome)

	// Return original for manual restore if needed
	if origHome != "" {
		return origHome
	}
	return origUserProfile
}

// restoreHomeDir is a no-op since t.Setenv automatically restores.
// Kept for readability in defer statements.
func restoreHomeDir(_ string) {}

// assertFileExists checks that a file exists at the given path
func assertFileExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("expected file to exist: %s", path)
	}
}
