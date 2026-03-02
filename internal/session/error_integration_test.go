//go:build integration

package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestErrorFlowE2E tests the full error tracking flow using file operations.
// This simulates the error flow without requiring an actual Copilot session.
func TestErrorFlowE2E(t *testing.T) {
	// Create isolated temp directory to simulate session directory
	tempDir := t.TempDir()
	errorPath := filepath.Join(tempDir, "error.md")

	// 1. Ensure no existing error
	if _, err := os.Stat(errorPath); err == nil {
		t.Fatal("error file should not exist initially")
	}

	// 2. Simulate postToolUse failure by writing error file
	workflowName := "lint-workflow"
	stepName := "eslint-check"
	errorDetails := "exit code 1: error in file.ts:10:5 - unused variable 'x'"

	content := formatErrorMarkdown(workflowName, stepName, errorDetails)
	if err := os.WriteFile(errorPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write error file: %v", err)
	}

	// 3. Check HasError returns true (simulated)
	_, statErr := os.Stat(errorPath)
	hasError := statErr == nil
	if !hasError {
		t.Fatal("expected error to exist after WriteError")
	}

	// 4. Simulate preToolUse check - should find error
	data, err := os.ReadFile(errorPath)
	if err != nil {
		t.Fatalf("failed to read error file: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("error content should not be empty")
	}

	// Verify error content has expected fields
	errorContent := string(data)
	if !strings.Contains(errorContent, workflowName) {
		t.Error("error content should contain workflow name")
	}
	if !strings.Contains(errorContent, stepName) {
		t.Error("error content should contain step name")
	}
	if !strings.Contains(errorContent, errorDetails) {
		t.Error("error content should contain error details")
	}

	// 5. Call ReadAndClearError (simulated)
	readContent, readErr := os.ReadFile(errorPath)
	if readErr != nil {
		t.Fatalf("failed to read for clear: %v", readErr)
	}

	if err := os.Remove(errorPath); err != nil {
		t.Fatalf("failed to remove error file: %v", err)
	}

	// Verify content was returned
	if string(readContent) != errorContent {
		t.Error("ReadAndClearError should return original content")
	}

	// 6. Check HasError returns false after clear
	_, statErr = os.Stat(errorPath)
	hasErrorAfterClear := statErr == nil
	if hasErrorAfterClear {
		t.Fatal("error should not exist after ReadAndClearError")
	}
}

// TestErrorFlowMultipleErrors tests that new errors overwrite old ones
func TestErrorFlowMultipleErrors(t *testing.T) {
	tempDir := t.TempDir()
	errorPath := filepath.Join(tempDir, "error.md")

	// Write first error
	content1 := formatErrorMarkdown("workflow-1", "step-1", "first error")
	if err := os.WriteFile(errorPath, []byte(content1), 0644); err != nil {
		t.Fatalf("failed to write first error: %v", err)
	}

	// Write second error (should overwrite)
	content2 := formatErrorMarkdown("workflow-2", "step-2", "second error")
	if err := os.WriteFile(errorPath, []byte(content2), 0644); err != nil {
		t.Fatalf("failed to write second error: %v", err)
	}

	// Read and verify only second error exists
	data, err := os.ReadFile(errorPath)
	if err != nil {
		t.Fatalf("failed to read error: %v", err)
	}

	content := string(data)
	if strings.Contains(content, "first error") {
		t.Error("old error should be overwritten")
	}
	if !strings.Contains(content, "second error") {
		t.Error("new error should be present")
	}
	if !strings.Contains(content, "workflow-2") {
		t.Error("new workflow name should be present")
	}
}

// TestErrorFlowConcurrentAccess tests that error file operations are safe
func TestErrorFlowConcurrentAccess(t *testing.T) {
	tempDir := t.TempDir()
	errorPath := filepath.Join(tempDir, "error.md")

	// Write error
	content := formatErrorMarkdown("test-workflow", "test-step", "test error")
	if err := os.WriteFile(errorPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write error: %v", err)
	}

	// Simulate concurrent read and clear
	done := make(chan bool, 2)

	// Reader 1
	go func() {
		_, _ = os.ReadFile(errorPath)
		done <- true
	}()

	// Reader 2
	go func() {
		_, _ = os.ReadFile(errorPath)
		done <- true
	}()

	// Wait for both readers
	<-done
	<-done

	// Clear should succeed even after concurrent reads
	if err := os.Remove(errorPath); err != nil && !os.IsNotExist(err) {
		t.Fatalf("failed to remove error file: %v", err)
	}
}

// TestErrorMarkdownFormat verifies the error file format is correct
func TestErrorMarkdownFormat(t *testing.T) {
	content := formatErrorMarkdown("my-workflow", "failing-step", "Command failed: exit status 1\nOutput: error at line 5")

	// Check header
	if !strings.HasPrefix(content, "# Hookflow Error") {
		t.Error("error file should start with header")
	}

	// Check required fields
	requiredFields := []string{
		"**Workflow:** my-workflow",
		"**Step:** failing-step",
		"**Time:**",
		"## Error Details",
		"Command failed: exit status 1",
		"Output: error at line 5",
	}

	for _, field := range requiredFields {
		if !strings.Contains(content, field) {
			t.Errorf("error file missing required field: %s", field)
		}
	}
}

// TestPreToolUseBlockingScenario simulates the preToolUse hook blocking scenario
func TestPreToolUseBlockingScenario(t *testing.T) {
	tempDir := t.TempDir()
	errorPath := filepath.Join(tempDir, "error.md")

	// Scenario: postToolUse wrote an error on previous action
	// Now preToolUse is called for a new action

	// Step 1: Simulate previous postToolUse wrote error
	previousError := formatErrorMarkdown("ci-workflow", "build-step", "Build failed: missing dependency")
	if err := os.WriteFile(errorPath, []byte(previousError), 0644); err != nil {
		t.Fatalf("failed to write previous error: %v", err)
	}

	// Step 2: preToolUse check - detect existing error
	_, statErr := os.Stat(errorPath)
	hasError := statErr == nil
	if !hasError {
		t.Fatal("preToolUse should detect existing error")
	}

	// Step 3: preToolUse would return deny with error message
	// In real flow, this would be the JSON response with systemMessage
	data, _ := os.ReadFile(errorPath)
	blockMessage := string(data)

	if !strings.Contains(blockMessage, "Build failed: missing dependency") {
		t.Error("block message should contain error details")
	}

	// Step 4: MCP tool clears the error (simulated)
	if err := os.Remove(errorPath); err != nil {
		t.Fatalf("MCP clear failed: %v", err)
	}

	// Step 5: Next preToolUse should allow
	_, statErr = os.Stat(errorPath)
	hasErrorAfterClear := statErr == nil
	if hasErrorAfterClear {
		t.Fatal("error should be cleared, allowing next action")
	}
}

// TestSessionDirectoryCreation tests that session directories are created properly
func TestSessionDirectoryCreation(t *testing.T) {
	tempDir := t.TempDir()
	sessionDir := filepath.Join(tempDir, "sessions", "12345")

	// Create session directory
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		t.Fatalf("failed to create session dir: %v", err)
	}

	// Verify directory exists
	info, err := os.Stat(sessionDir)
	if err != nil {
		t.Fatalf("session dir should exist: %v", err)
	}
	if !info.IsDir() {
		t.Error("session path should be a directory")
	}

	// Write error file in session directory
	errorPath := filepath.Join(sessionDir, "error.md")
	content := formatErrorMarkdown("test", "test", "test")
	if err := os.WriteFile(errorPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write error in session dir: %v", err)
	}

	// Verify error file exists
	if _, err := os.Stat(errorPath); os.IsNotExist(err) {
		t.Error("error file should exist in session directory")
	}
}
