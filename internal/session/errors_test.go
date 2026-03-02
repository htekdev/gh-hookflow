package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteError_CreatesFile(t *testing.T) {
	// Create a temp directory to simulate session dir
	tempDir := t.TempDir()
	errorPath := filepath.Join(tempDir, "error.md")

	// Write error file directly (bypass session detection for testing)
	content := formatErrorMarkdown("lint-workflow", "run-linter", "exit code 1: file.go:10 unused variable")
	if err := os.WriteFile(errorPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write error file: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(errorPath); os.IsNotExist(err) {
		t.Fatal("error file was not created")
	}

	// Verify content format
	data, err := os.ReadFile(errorPath)
	if err != nil {
		t.Fatalf("failed to read error file: %v", err)
	}

	content = string(data)
	if !strings.Contains(content, "# Hookflow Error") {
		t.Error("missing header in error file")
	}
	if !strings.Contains(content, "**Workflow:** lint-workflow") {
		t.Error("missing workflow name in error file")
	}
	if !strings.Contains(content, "**Step:** run-linter") {
		t.Error("missing step name in error file")
	}
	if !strings.Contains(content, "exit code 1: file.go:10 unused variable") {
		t.Error("missing error details in error file")
	}
}

func TestHasError_WhenNoError(t *testing.T) {
	tempDir := t.TempDir()
	errorPath := filepath.Join(tempDir, "error.md")

	// Check file does not exist
	_, err := os.Stat(errorPath)
	if !os.IsNotExist(err) {
		t.Fatal("expected error file to not exist")
	}
}

func TestHasError_WhenErrorExists(t *testing.T) {
	tempDir := t.TempDir()
	errorPath := filepath.Join(tempDir, "error.md")

	// Create error file
	if err := os.WriteFile(errorPath, []byte("test error"), 0644); err != nil {
		t.Fatalf("failed to create error file: %v", err)
	}

	// Check file exists
	_, err := os.Stat(errorPath)
	if err != nil {
		t.Fatalf("expected error file to exist: %v", err)
	}
}

func TestReadAndClearError_ReturnsAndDeletes(t *testing.T) {
	tempDir := t.TempDir()
	errorPath := filepath.Join(tempDir, "error.md")
	expectedContent := "# Hookflow Error\n\ntest error details"

	// Create error file
	if err := os.WriteFile(errorPath, []byte(expectedContent), 0644); err != nil {
		t.Fatalf("failed to create error file: %v", err)
	}

	// Read the file
	content, err := os.ReadFile(errorPath)
	if err != nil {
		t.Fatalf("failed to read error file: %v", err)
	}

	if string(content) != expectedContent {
		t.Errorf("content mismatch: got %q, want %q", string(content), expectedContent)
	}

	// Delete the file
	if err := os.Remove(errorPath); err != nil {
		t.Fatalf("failed to remove error file: %v", err)
	}

	// Verify file is deleted
	if _, err := os.Stat(errorPath); !os.IsNotExist(err) {
		t.Fatal("error file should have been deleted")
	}
}

func TestReadAndClearError_WhenNoError(t *testing.T) {
	tempDir := t.TempDir()
	errorPath := filepath.Join(tempDir, "error.md")

	// Try to read non-existent file
	_, err := os.ReadFile(errorPath)
	if !os.IsNotExist(err) {
		t.Fatal("expected file to not exist")
	}
}

func TestFormatErrorMarkdown(t *testing.T) {
	content := formatErrorMarkdown("test-workflow", "test-step", "something went wrong")

	if !strings.Contains(content, "# Hookflow Error") {
		t.Error("missing header")
	}
	if !strings.Contains(content, "**Workflow:** test-workflow") {
		t.Error("missing workflow name")
	}
	if !strings.Contains(content, "**Step:** test-step") {
		t.Error("missing step name")
	}
	if !strings.Contains(content, "**Time:**") {
		t.Error("missing timestamp")
	}
	if !strings.Contains(content, "## Error Details") {
		t.Error("missing error details section")
	}
	if !strings.Contains(content, "something went wrong") {
		t.Error("missing error message")
	}
}

func TestFormatErrorMarkdown_EmptyFields(t *testing.T) {
	content := formatErrorMarkdown("", "", "")
	if !strings.Contains(content, "# Hookflow Error") {
		t.Error("missing header with empty fields")
	}
	if !strings.Contains(content, "**Workflow:** ") {
		t.Error("missing workflow field")
	}
	if !strings.Contains(content, "**Step:** ") {
		t.Error("missing step field")
	}
}

func TestFormatErrorMarkdown_SpecialChars(t *testing.T) {
	content := formatErrorMarkdown("work**flow", "step`name", "error with <html> & \"quotes\"")
	if !strings.Contains(content, "work**flow") {
		t.Error("special chars in workflow name not preserved")
	}
	if !strings.Contains(content, "step`name") {
		t.Error("special chars in step name not preserved")
	}
	if !strings.Contains(content, "<html> & \"quotes\"") {
		t.Error("special chars in error details not preserved")
	}
}

func TestFormatErrorMarkdown_MultilineError(t *testing.T) {
	multiline := "line1\nline2\nline3\n\ttabbed line"
	content := formatErrorMarkdown("wf", "st", multiline)
	if !strings.Contains(content, "line1\nline2\nline3") {
		t.Error("multiline error details not preserved")
	}
	if !strings.Contains(content, "\ttabbed line") {
		t.Error("tabbed line not preserved")
	}
}

func TestFormatErrorMarkdown_LargeContent(t *testing.T) {
	large := strings.Repeat("x", 100_000)
	content := formatErrorMarkdown("wf", "st", large)
	if !strings.Contains(content, large) {
		t.Error("large content not preserved")
	}
	if len(content) < 100_000 {
		t.Errorf("content too short: %d", len(content))
	}
}

func TestWriteError_Functional(t *testing.T) {
	_, err := GetCopilotPID()
	if err != nil {
		t.Skip("Not running under Copilot")
	}

	err = WriteError("func-test-workflow", "func-test-step", "functional test error")
	if err != nil {
		t.Fatalf("WriteError() failed: %v", err)
	}

	// Clean up via ReadAndClearError
	content, err := ReadAndClearError()
	if err != nil {
		t.Fatalf("ReadAndClearError() failed: %v", err)
	}
	if !strings.Contains(content, "func-test-workflow") {
		t.Error("missing workflow name in written error")
	}
	if !strings.Contains(content, "func-test-step") {
		t.Error("missing step name in written error")
	}
	if !strings.Contains(content, "functional test error") {
		t.Error("missing error details in written error")
	}
}

func TestHasError_Functional(t *testing.T) {
	_, err := GetCopilotPID()
	if err != nil {
		t.Skip("Not running under Copilot")
	}

	// Ensure no pre-existing error
	_, _ = ReadAndClearError()

	// No error should exist
	has, err := HasError()
	if err != nil {
		t.Fatalf("HasError() failed: %v", err)
	}
	if has {
		t.Error("HasError() = true, want false when no error exists")
	}

	// Write an error
	err = WriteError("has-test-wf", "has-test-step", "has test error")
	if err != nil {
		t.Fatalf("WriteError() failed: %v", err)
	}

	// Error should exist
	has, err = HasError()
	if err != nil {
		t.Fatalf("HasError() failed: %v", err)
	}
	if !has {
		t.Error("HasError() = false, want true after WriteError")
	}

	// Clean up
	_, _ = ReadAndClearError()
}

func TestReadAndClearError_Functional_NoError(t *testing.T) {
	_, err := GetCopilotPID()
	if err != nil {
		t.Skip("Not running under Copilot")
	}

	// Ensure no pre-existing error
	_, _ = ReadAndClearError()

	// Reading when no error exists should return empty string
	content, err := ReadAndClearError()
	if err != nil {
		t.Fatalf("ReadAndClearError() failed: %v", err)
	}
	if content != "" {
		t.Errorf("ReadAndClearError() = %q, want empty string", content)
	}
}

func TestReadAndClearError_Functional_ClearsFile(t *testing.T) {
	_, err := GetCopilotPID()
	if err != nil {
		t.Skip("Not running under Copilot")
	}

	// Write an error
	err = WriteError("clear-test-wf", "clear-test-step", "clear test error")
	if err != nil {
		t.Fatalf("WriteError() failed: %v", err)
	}

	// Read and clear
	content, err := ReadAndClearError()
	if err != nil {
		t.Fatalf("ReadAndClearError() failed: %v", err)
	}
	if !strings.Contains(content, "clear test error") {
		t.Error("ReadAndClearError() missing error details")
	}

	// File should be gone
	has, err := HasError()
	if err != nil {
		t.Fatalf("HasError() failed: %v", err)
	}
	if has {
		t.Error("HasError() = true after ReadAndClearError")
	}
}

func TestWriteError_OverwritesPrevious(t *testing.T) {
	_, err := GetCopilotPID()
	if err != nil {
		t.Skip("Not running under Copilot")
	}

	// Write first error
	err = WriteError("wf1", "step1", "first error")
	if err != nil {
		t.Fatalf("WriteError() first failed: %v", err)
	}

	// Write second error (overwrites)
	err = WriteError("wf2", "step2", "second error")
	if err != nil {
		t.Fatalf("WriteError() second failed: %v", err)
	}

	content, err := ReadAndClearError()
	if err != nil {
		t.Fatalf("ReadAndClearError() failed: %v", err)
	}
	if strings.Contains(content, "first error") {
		t.Error("first error should be overwritten")
	}
	if !strings.Contains(content, "second error") {
		t.Error("second error should be present")
	}
}

func TestWriteError_LargeContent(t *testing.T) {
	_, err := GetCopilotPID()
	if err != nil {
		t.Skip("Not running under Copilot")
	}

	large := strings.Repeat("error line\n", 10_000)
	err = WriteError("large-wf", "large-step", large)
	if err != nil {
		t.Fatalf("WriteError() with large content failed: %v", err)
	}

	content, err := ReadAndClearError()
	if err != nil {
		t.Fatalf("ReadAndClearError() failed: %v", err)
	}
	if !strings.Contains(content, "error line") {
		t.Error("large content not preserved")
	}
}
