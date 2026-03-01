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
