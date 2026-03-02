package schema

import (
	"os"
	"path/filepath"
	"testing"
)

// ============================================================================
// LoadWorkflow Tests
// ============================================================================

func TestLoadWorkflow_ValidSimple(t *testing.T) {
	workflow, err := LoadWorkflow("../../testdata/workflows/valid/simple.yml")
	if err != nil {
		t.Fatalf("Failed to load valid workflow: %v", err)
	}
	if workflow.Name != "Lint JavaScript Files" {
		t.Errorf("Expected name 'Lint JavaScript Files', got '%s'", workflow.Name)
	}
	if workflow.Description != "Run ESLint on JS file edits" {
		t.Errorf("Expected description 'Run ESLint on JS file edits', got '%s'", workflow.Description)
	}
	if len(workflow.Steps) != 2 {
		t.Errorf("Expected 2 steps, got %d", len(workflow.Steps))
	}
}

func TestLoadWorkflow_ValidMinimal(t *testing.T) {
	workflow, err := LoadWorkflow("../../testdata/workflows/valid/minimal.yml")
	if err != nil {
		t.Fatalf("Failed to load valid workflow: %v", err)
	}
	if workflow.Name != "Minimal Valid" {
		t.Errorf("Expected name 'Minimal Valid', got '%s'", workflow.Name)
	}
	if len(workflow.Steps) != 1 {
		t.Errorf("Expected 1 step, got %d", len(workflow.Steps))
	}
}

func TestLoadWorkflow_ValidComplexFull(t *testing.T) {
	workflow, err := LoadWorkflow("../../testdata/workflows/valid/complex-full.yml")
	if err != nil {
		t.Fatalf("Failed to load valid workflow: %v", err)
	}
	if workflow.Name != "With All Valid Options" {
		t.Errorf("Expected name 'With All Valid Options', got '%s'", workflow.Name)
	}

	// Check blocking
	if workflow.Blocking == nil || *workflow.Blocking != false {
		t.Errorf("Expected blocking to be false")
	}
	if workflow.IsBlocking() {
		t.Errorf("Expected IsBlocking() to return false")
	}

	// Check concurrency
	if workflow.Concurrency == nil {
		t.Fatal("Expected concurrency to be set")
	}
	if workflow.Concurrency.MaxParallel != 3 {
		t.Errorf("Expected max-parallel 3, got %d", workflow.Concurrency.MaxParallel)
	}

	// Check env
	if len(workflow.Env) != 2 {
		t.Errorf("Expected 2 env vars, got %d", len(workflow.Env))
	}

	// Check steps
	if len(workflow.Steps) != 4 {
		t.Errorf("Expected 4 steps, got %d", len(workflow.Steps))
	}
}

func TestLoadWorkflow_FileNotFound(t *testing.T) {
	_, err := LoadWorkflow("../../testdata/workflows/nonexistent.yml")
	if err == nil {
		t.Error("Expected error for non-existent file, got nil")
	}
}

func TestLoadWorkflow_InvalidYAML(t *testing.T) {
	_, err := LoadWorkflow("../../testdata/workflows/invalid/bad-syntax.yml")
	if err == nil {
		t.Error("Expected error for invalid YAML, got nil")
	}
}

func TestLoadWorkflow_AllTriggers(t *testing.T) {
	workflow, err := LoadWorkflow("../../testdata/workflows/valid/all-triggers.yml")
	if err != nil {
		t.Fatalf("Failed to load workflow: %v", err)
	}

	// Check hooks trigger
	if workflow.On.Hooks == nil {
		t.Error("Expected hooks trigger to be set")
	} else {
		if len(workflow.On.Hooks.Types) != 2 {
			t.Errorf("Expected 2 hook types, got %d", len(workflow.On.Hooks.Types))
		}
		if len(workflow.On.Hooks.Tools) != 2 {
			t.Errorf("Expected 2 hook tools, got %d", len(workflow.On.Hooks.Tools))
		}
	}

	// Check tool trigger
	if workflow.On.Tool == nil {
		t.Error("Expected tool trigger to be set")
	} else {
		if workflow.On.Tool.Name != "edit" {
			t.Errorf("Expected tool name 'edit', got '%s'", workflow.On.Tool.Name)
		}
	}

	// Check tools trigger
	if len(workflow.On.Tools) != 2 {
		t.Errorf("Expected 2 tools triggers, got %d", len(workflow.On.Tools))
	}

	// Check file trigger
	if workflow.On.File == nil {
		t.Error("Expected file trigger to be set")
	}

	// Check commit trigger
	if workflow.On.Commit == nil {
		t.Error("Expected commit trigger to be set")
	}

	// Check push trigger
	if workflow.On.Push == nil {
		t.Error("Expected push trigger to be set")
	}
}

// ============================================================================
// Empty Trigger Tests (YAML with just "commit:" no properties)
// ============================================================================

func TestLoadWorkflow_EmptyCommitTrigger(t *testing.T) {
	workflow, err := LoadWorkflow("../../testdata/workflows/valid/empty-commit-trigger.yml")
	if err != nil {
		t.Fatalf("Failed to load workflow with empty commit trigger: %v", err)
	}
	if workflow.Name != "Empty Commit Trigger Test" {
		t.Errorf("Expected name 'Empty Commit Trigger Test', got '%s'", workflow.Name)
	}

	// The key test: commit trigger should NOT be nil even though YAML has just "commit:"
	if workflow.On.Commit == nil {
		t.Error("Expected commit trigger to be non-nil for 'on: commit:' syntax")
	}

	// Other triggers should still be nil
	if workflow.On.Hooks != nil {
		t.Error("Expected hooks trigger to be nil")
	}
	if workflow.On.Push != nil {
		t.Error("Expected push trigger to be nil")
	}
}

// ============================================================================
// IsBlocking Tests
// ============================================================================

func TestWorkflow_IsBlocking_Default(t *testing.T) {
	workflow := &Workflow{}
	if !workflow.IsBlocking() {
		t.Error("Expected IsBlocking() to return true when Blocking is nil")
	}
}

func TestWorkflow_IsBlocking_ExplicitTrue(t *testing.T) {
	blocking := true
	workflow := &Workflow{Blocking: &blocking}
	if !workflow.IsBlocking() {
		t.Error("Expected IsBlocking() to return true")
	}
}

func TestWorkflow_IsBlocking_ExplicitFalse(t *testing.T) {
	blocking := false
	workflow := &Workflow{Blocking: &blocking}
	if workflow.IsBlocking() {
		t.Error("Expected IsBlocking() to return false")
	}
}

// ============================================================================
// LoadAndValidateWorkflow Tests
// ============================================================================

func TestLoadAndValidateWorkflow_Valid(t *testing.T) {
	workflow, err := LoadAndValidateWorkflow("../../testdata/workflows/valid/simple.yml")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if workflow == nil {
		t.Fatal("Expected non-nil workflow")
	}
	if workflow.Name != "Lint JavaScript Files" {
		t.Errorf("Expected name 'Lint JavaScript Files', got '%s'", workflow.Name)
	}
}

func TestLoadAndValidateWorkflow_FileNotFound(t *testing.T) {
	_, err := LoadAndValidateWorkflow("../../testdata/workflows/nonexistent.yml")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}

func TestLoadAndValidateWorkflow_InvalidSchema(t *testing.T) {
	_, err := LoadAndValidateWorkflow("../../testdata/workflows/invalid/missing-required.yml")
	if err == nil {
		t.Error("Expected error for invalid schema")
	}
}

func TestLoadAndValidateWorkflow_BadSyntax(t *testing.T) {
	_, err := LoadAndValidateWorkflow("../../testdata/workflows/invalid/bad-syntax.yml")
	if err == nil {
		t.Error("Expected error for bad YAML syntax")
	}
}

func TestLoadAndValidateWorkflow_EmptyOn(t *testing.T) {
	_, err := LoadAndValidateWorkflow("../../testdata/workflows/invalid/empty-on.yml")
	if err == nil {
		t.Error("Expected error for empty on config")
	}
}

func TestLoadAndValidateWorkflow_ValidationFailNoErrors(t *testing.T) {
	// Test with empty name - schema validates to invalid with errors
	_, err := LoadAndValidateWorkflow("../../testdata/workflows/invalid/empty-name.yml")
	if err == nil {
		t.Error("Expected error for empty name")
	}
}

// ============================================================================
// Timeout Validation Tests
// ============================================================================

func TestValidateWorkflow_InvalidTimeoutNegative(t *testing.T) {
	result := ValidateWorkflow("../../testdata/workflows/invalid/invalid-timeout.yml")
	if result.Valid {
		t.Error("Expected invalid workflow for negative timeout")
	}
	assertHasValidationError(t, result)
}

func TestValidateWorkflow_ZeroTimeout(t *testing.T) {
	result := ValidateWorkflow("../../testdata/workflows/invalid/zero-timeout.yml")
	if result.Valid {
		t.Error("Expected invalid workflow for zero timeout")
	}
	assertHasValidationError(t, result)
}

func TestValidateWorkflow_StringTimeout(t *testing.T) {
	result := ValidateWorkflow("../../testdata/workflows/invalid/string-timeout.yml")
	if result.Valid {
		t.Error("Expected invalid workflow for string timeout")
	}
	assertHasValidationError(t, result)
}

// ============================================================================
// Shell Validation Tests
// ============================================================================

func TestValidateWorkflow_InvalidShell(t *testing.T) {
	result := ValidateWorkflow("../../testdata/workflows/invalid/invalid-shell.yml")
	if result.Valid {
		t.Error("Expected invalid workflow for invalid shell")
	}
	assertHasValidationError(t, result)
}

func TestValidateWorkflow_ValidShells(t *testing.T) {
	// Test that valid shells pass - use simple.yml which has shell: pwsh
	result := ValidateWorkflow("../../testdata/workflows/valid/simple.yml")
	if !result.Valid {
		t.Errorf("Expected valid workflow with pwsh shell, got errors: %v", result.Errors)
	}
}

// ============================================================================
// Trigger Validation Tests
// ============================================================================

func TestValidateWorkflow_EmptyOn(t *testing.T) {
	result := ValidateWorkflow("../../testdata/workflows/invalid/empty-on.yml")
	if result.Valid {
		t.Error("Expected invalid workflow for empty 'on' config")
	}
	assertHasValidationError(t, result)
}

func TestValidateWorkflow_InvalidToolTrigger(t *testing.T) {
	result := ValidateWorkflow("../../testdata/workflows/invalid/invalid-tool-trigger.yml")
	if result.Valid {
		t.Error("Expected invalid workflow for tool trigger missing name")
	}
	assertHasValidationError(t, result)
}

func TestValidateWorkflow_InvalidFileType(t *testing.T) {
	result := ValidateWorkflow("../../testdata/workflows/invalid/invalid-file-type.yml")
	if result.Valid {
		t.Error("Expected invalid workflow for invalid file type")
	}
	assertHasValidationError(t, result)
}

func TestValidateWorkflow_EmptyToolsArray(t *testing.T) {
	result := ValidateWorkflow("../../testdata/workflows/invalid/empty-tools-array.yml")
	if result.Valid {
		t.Error("Expected invalid workflow for empty tools array")
	}
	assertHasValidationError(t, result)
}

// ============================================================================
// Env Variable Validation Tests
// ============================================================================

func TestValidateWorkflow_InvalidEnvType(t *testing.T) {
	result := ValidateWorkflow("../../testdata/workflows/invalid/invalid-env-type.yml")
	if result.Valid {
		t.Error("Expected invalid workflow for non-string env value")
	}
	assertHasValidationError(t, result)
}

func TestValidateWorkflow_InvalidStepEnvType(t *testing.T) {
	result := ValidateWorkflow("../../testdata/workflows/invalid/invalid-step-env.yml")
	if result.Valid {
		t.Error("Expected invalid workflow for non-string step env value")
	}
	assertHasValidationError(t, result)
}

// ============================================================================
// Concurrency Validation Tests
// ============================================================================

func TestValidateWorkflow_InvalidConcurrencyMissingGroup(t *testing.T) {
	result := ValidateWorkflow("../../testdata/workflows/invalid/invalid-concurrency.yml")
	if result.Valid {
		t.Error("Expected invalid workflow for concurrency missing group")
	}
	assertHasValidationError(t, result)
}

func TestValidateWorkflow_InvalidMaxParallel(t *testing.T) {
	result := ValidateWorkflow("../../testdata/workflows/invalid/invalid-max-parallel.yml")
	if result.Valid {
		t.Error("Expected invalid workflow for max-parallel < 1")
	}
	assertHasValidationError(t, result)
}

func TestValidateWorkflow_EmptyConcurrencyGroup(t *testing.T) {
	result := ValidateWorkflow("../../testdata/workflows/invalid/empty-concurrency-group.yml")
	if result.Valid {
		t.Error("Expected invalid workflow for empty concurrency group")
	}
	assertHasValidationError(t, result)
}

// ============================================================================
// Steps Validation Tests
// ============================================================================

func TestValidateWorkflow_EmptySteps(t *testing.T) {
	result := ValidateWorkflow("../../testdata/workflows/invalid/empty-steps.yml")
	if result.Valid {
		t.Error("Expected invalid workflow for empty steps array")
	}
	assertHasValidationError(t, result)
}

func TestValidateWorkflow_StepMissingAction(t *testing.T) {
	result := ValidateWorkflow("../../testdata/workflows/invalid/step-missing-action.yml")
	if result.Valid {
		t.Error("Expected invalid workflow for step missing run/uses")
	}
	assertHasValidationError(t, result)
}

// ============================================================================
// Additional Property Validation Tests
// ============================================================================

func TestValidateWorkflow_ExtraProperty(t *testing.T) {
	result := ValidateWorkflow("../../testdata/workflows/invalid/extra-property.yml")
	if result.Valid {
		t.Error("Expected invalid workflow for extra property")
	}
	assertHasValidationError(t, result)
}

func TestValidateWorkflow_EmptyName(t *testing.T) {
	result := ValidateWorkflow("../../testdata/workflows/invalid/empty-name.yml")
	if result.Valid {
		t.Error("Expected invalid workflow for empty name")
	}
	assertHasValidationError(t, result)
}

// ============================================================================
// Valid Workflows Tests
// ============================================================================

func TestValidateWorkflow_ValidComplexFull(t *testing.T) {
	result := ValidateWorkflow("../../testdata/workflows/valid/complex-full.yml")
	if !result.Valid {
		t.Errorf("Expected valid workflow, got errors: %v", result.Errors)
	}
}

func TestValidateWorkflow_ValidMinimal(t *testing.T) {
	result := ValidateWorkflow("../../testdata/workflows/valid/minimal.yml")
	if !result.Valid {
		t.Errorf("Expected valid workflow, got errors: %v", result.Errors)
	}
}

// ============================================================================
// YAML-specific Scenarios Tests
// ============================================================================

func TestValidateWorkflow_MalformedYAML_UnclosedBracket(t *testing.T) {
	// Create temp file with unclosed bracket
	tmpDir, err := os.MkdirTemp("", "yaml-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	content := `name: Test
on:
  hooks:
    types: [preToolUse
steps:
  - run: echo
`
	tmpFile := filepath.Join(tmpDir, "unclosed.yml")
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	result := ValidateWorkflow(tmpFile)
	if result.Valid {
		t.Error("Expected invalid for unclosed bracket YAML")
	}
}

func TestValidateWorkflow_MalformedYAML_BadIndentation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "yaml-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	content := `name: Test
on:
hooks:  # wrong indentation
    types:
      - preToolUse
steps:
  - run: echo
`
	tmpFile := filepath.Join(tmpDir, "bad-indent.yml")
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	result := ValidateWorkflow(tmpFile)
	if result.Valid {
		t.Error("Expected invalid for bad indentation YAML")
	}
}

func TestValidateWorkflow_MalformedYAML_DuplicateKeys(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "yaml-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	content := `name: Test
name: Duplicate
on:
  hooks:
    types:
      - preToolUse
steps:
  - run: echo
`
	tmpFile := filepath.Join(tmpDir, "dup-keys.yml")
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	// gopkg.in/yaml.v3 detects duplicate keys as errors
	result := ValidateWorkflow(tmpFile)
	if result.Valid {
		t.Error("Expected invalid for duplicate keys in YAML")
	}
	assertHasValidationError(t, result)
}

func TestValidateWorkflow_MalformedYAML_TabsInsteadOfSpaces(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "yaml-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	content := "name: Test\non:\n\thooks:\n\t\ttypes:\n\t\t\t- preToolUse\nsteps:\n\t- run: echo\n"
	tmpFile := filepath.Join(tmpDir, "tabs.yml")
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	result := ValidateWorkflow(tmpFile)
	if result.Valid {
		t.Error("Expected invalid for YAML with tabs")
	}
}

func TestValidateWorkflow_MalformedYAML_EmptyFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "yaml-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	tmpFile := filepath.Join(tmpDir, "empty.yml")
	if err := os.WriteFile(tmpFile, []byte(""), 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	result := ValidateWorkflow(tmpFile)
	if result.Valid {
		t.Error("Expected invalid for empty YAML file")
	}
}

func TestValidateWorkflow_MalformedYAML_OnlyComments(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "yaml-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	content := `# This is only comments
# No actual content
`
	tmpFile := filepath.Join(tmpDir, "comments-only.yml")
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	result := ValidateWorkflow(tmpFile)
	if result.Valid {
		t.Error("Expected invalid for comments-only YAML file")
	}
}

// ============================================================================
// File Permission Error Tests
// ============================================================================

func TestValidateWorkflow_ReadPermissionError(t *testing.T) {
	// Skip on Windows as permission handling is different
	if os.Getenv("OS") == "Windows_NT" {
		t.Skip("Skipping permission test on Windows")
	}

	tmpDir, err := os.MkdirTemp("", "perm-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	tmpFile := filepath.Join(tmpDir, "no-read.yml")
	if err := os.WriteFile(tmpFile, []byte("name: test"), 0000); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	result := ValidateWorkflow(tmpFile)
	if result.Valid {
		t.Error("Expected invalid for file with no read permission")
	}
	assertHasValidationError(t, result)
}

// ============================================================================
// ValidateWorkflowsInDir Additional Tests
// ============================================================================

func TestValidateWorkflowsInDir_MixedValidInvalid(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mixed-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	workflowDir := filepath.Join(tmpDir, ".github", "hookflows")
	if err := os.MkdirAll(workflowDir, 0755); err != nil {
		t.Fatalf("Failed to create workflow dir: %v", err)
	}

	// Copy a valid workflow
	validContent, _ := os.ReadFile("../../testdata/workflows/valid/simple.yml")
	if err := os.WriteFile(filepath.Join(workflowDir, "valid.yml"), validContent, 0644); err != nil {
		t.Fatalf("Failed to write valid workflow: %v", err)
	}

	// Copy an invalid workflow
	invalidContent, _ := os.ReadFile("../../testdata/workflows/invalid/missing-required.yml")
	if err := os.WriteFile(filepath.Join(workflowDir, "invalid.yml"), invalidContent, 0644); err != nil {
		t.Fatalf("Failed to write invalid workflow: %v", err)
	}

	result := ValidateWorkflowsInDir(tmpDir)
	if result.Valid {
		t.Error("Expected invalid result when directory contains invalid workflow")
	}
	if len(result.Errors) == 0 {
		t.Error("Expected at least one error")
	}
}

func TestValidateWorkflowsInDir_NestedWorkflows(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "nested-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	workflowDir := filepath.Join(tmpDir, ".github", "hookflows")
	nestedDir := filepath.Join(workflowDir, "nested")
	if err := os.MkdirAll(nestedDir, 0755); err != nil {
		t.Fatalf("Failed to create nested dir: %v", err)
	}

	validContent, _ := os.ReadFile("../../testdata/workflows/valid/simple.yml")
	if err := os.WriteFile(filepath.Join(nestedDir, "nested.yml"), validContent, 0644); err != nil {
		t.Fatalf("Failed to write nested workflow: %v", err)
	}

	result := ValidateWorkflowsInDir(tmpDir)
	if !result.Valid {
		t.Errorf("Expected valid for nested workflows: %v", result.Errors)
	}
}

func TestValidateWorkflowsInDir_IgnoresNonYAML(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "non-yaml-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	workflowDir := filepath.Join(tmpDir, ".github", "hookflows")
	if err := os.MkdirAll(workflowDir, 0755); err != nil {
		t.Fatalf("Failed to create workflow dir: %v", err)
	}

	// Write non-YAML files
	if err := os.WriteFile(filepath.Join(workflowDir, "README.md"), []byte("# Readme"), 0644); err != nil {
		t.Fatalf("Failed to write readme: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workflowDir, "config.json"), []byte("{}"), 0644); err != nil {
		t.Fatalf("Failed to write json: %v", err)
	}

	result := ValidateWorkflowsInDir(tmpDir)
	if !result.Valid {
		t.Errorf("Expected valid when only non-YAML files exist: %v", result.Errors)
	}
}

func TestValidateWorkflowsInDir_YAMLExtensions(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "ext-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	workflowDir := filepath.Join(tmpDir, ".github", "hookflows")
	if err := os.MkdirAll(workflowDir, 0755); err != nil {
		t.Fatalf("Failed to create workflow dir: %v", err)
	}

	validContent, _ := os.ReadFile("../../testdata/workflows/valid/simple.yml")

	// Test .yml extension
	if err := os.WriteFile(filepath.Join(workflowDir, "test.yml"), validContent, 0644); err != nil {
		t.Fatalf("Failed to write .yml file: %v", err)
	}

	// Test .yaml extension
	if err := os.WriteFile(filepath.Join(workflowDir, "test2.yaml"), validContent, 0644); err != nil {
		t.Fatalf("Failed to write .yaml file: %v", err)
	}

	// Test .YML extension (uppercase)
	if err := os.WriteFile(filepath.Join(workflowDir, "test3.YML"), validContent, 0644); err != nil {
		t.Fatalf("Failed to write .YML file: %v", err)
	}

	result := ValidateWorkflowsInDir(tmpDir)
	if !result.Valid {
		t.Errorf("Expected valid for various YAML extensions: %v", result.Errors)
	}
}

// ============================================================================
// Result Types Tests
// ============================================================================

func TestNewAllowResult(t *testing.T) {
	result := NewAllowResult()
	if result.PermissionDecision != "allow" {
		t.Errorf("Expected 'allow', got '%s'", result.PermissionDecision)
	}
	if result.PermissionDecisionReason != "" {
		t.Errorf("Expected empty reason, got '%s'", result.PermissionDecisionReason)
	}
}

func TestNewDenyResult(t *testing.T) {
	result := NewDenyResult("security violation")
	if result.PermissionDecision != "deny" {
		t.Errorf("Expected 'deny', got '%s'", result.PermissionDecision)
	}
	if result.PermissionDecisionReason != "security violation" {
		t.Errorf("Expected 'security violation', got '%s'", result.PermissionDecisionReason)
	}
}

func TestNewDenyResult_EmptyReason(t *testing.T) {
	result := NewDenyResult("")
	if result.PermissionDecision != "deny" {
		t.Errorf("Expected 'deny', got '%s'", result.PermissionDecision)
	}
	if result.PermissionDecisionReason != "" {
		t.Errorf("Expected empty reason, got '%s'", result.PermissionDecisionReason)
	}
}

// ============================================================================
// Empty Trigger Tests - Push, Hooks, File
// ============================================================================

func TestLoadWorkflow_EmptyPushTrigger(t *testing.T) {
	workflow, err := LoadWorkflow("../../testdata/workflows/valid/empty-push-trigger.yml")
	if err != nil {
		t.Fatalf("Failed to load workflow: %v", err)
	}
	if workflow.On.Push == nil {
		t.Error("Expected push trigger to be non-nil for bare 'push:' syntax")
	}
	if workflow.On.Commit != nil {
		t.Error("Expected commit trigger to be nil")
	}
}

func TestLoadWorkflow_EmptyHooksTrigger(t *testing.T) {
	workflow, err := LoadWorkflow("../../testdata/workflows/valid/empty-hooks-trigger.yml")
	if err != nil {
		t.Fatalf("Failed to load workflow: %v", err)
	}
	if workflow.On.Hooks == nil {
		t.Error("Expected hooks trigger to be non-nil for bare 'hooks:' syntax")
	}
}

func TestLoadWorkflow_EmptyFileTrigger(t *testing.T) {
	workflow, err := LoadWorkflow("../../testdata/workflows/valid/empty-file-trigger.yml")
	if err != nil {
		t.Fatalf("Failed to load workflow: %v", err)
	}
	if workflow.On.File == nil {
		t.Error("Expected file trigger to be non-nil for bare 'file:' syntax")
	}
}

// ============================================================================
// LoadEvent Test
// ============================================================================

func TestLoadEvent(t *testing.T) {
	// Currently LoadEvent returns nil, nil - just ensure it doesn't error
	event, err := LoadEvent(`{"type": "test"}`)
	if err != nil {
		t.Errorf("LoadEvent should not return error: %v", err)
	}
	if event != nil {
		t.Error("LoadEvent currently returns nil, expected nil")
	}
}

// ============================================================================
// Validation Error Details Tests
// ============================================================================

func TestValidationError_WithDetails(t *testing.T) {
	result := ValidateWorkflow("../../testdata/workflows/invalid/missing-required.yml")
	if result.Valid {
		t.Fatal("Expected invalid result")
	}
	if len(result.Errors) == 0 {
		t.Fatal("Expected at least one error")
	}

	// Check that we have details for schema validation errors
	err := result.Errors[0]
	if err.File == "" {
		t.Error("Expected file path in error")
	}
	if err.Message == "" {
		t.Error("Expected message in error")
	}
	// Details should contain specific validation failures
	if len(err.Details) == 0 {
		t.Error("Expected details in validation error")
	}
}

func TestValidationResult_MultipleErrors(t *testing.T) {
	// Create a temp workflow with multiple issues
	tmpDir, err := os.MkdirTemp("", "multi-error")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Workflow with multiple validation issues
	content := `name: ""
on: {}
steps: []
`
	tmpFile := filepath.Join(tmpDir, "multi-error.yml")
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	result := ValidateWorkflow(tmpFile)
	if result.Valid {
		t.Error("Expected invalid result")
	}
	if len(result.Errors) == 0 {
		t.Error("Expected validation errors")
	}
	// Check that multiple issues are captured in details
	if len(result.Errors) > 0 && len(result.Errors[0].Details) == 0 {
		t.Error("Expected multiple validation details")
	}
}

// ============================================================================
// Helper Functions
// ============================================================================

func assertHasValidationError(t *testing.T, result *ValidationResult) {
	t.Helper()
	if len(result.Errors) == 0 {
		t.Error("Expected at least one validation error, got none")
	}
}

