package mcp

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/htekdev/gh-hookflow/internal/session"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// --- handleGetError tests ---

func TestHandleGetError_NoPendingErrors(t *testing.T) {
	// Set up a temp session dir with no error file
	tmpDir := t.TempDir()
	t.Setenv("HOOKFLOW_SESSION_DIR", tmpDir)

	result, output, err := handleGetError(context.Background(), nil, getErrorInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil && result.IsError {
		t.Error("expected no error result")
	}
	if output.Message != "No pending errors" {
		t.Errorf("Message = %q, want %q", output.Message, "No pending errors")
	}
}

func TestHandleGetError_WithPendingError(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOOKFLOW_SESSION_DIR", tmpDir)

	// Write an error file
	errContent := "## Workflow Failed\nTest error content"
	if err := os.WriteFile(filepath.Join(tmpDir, "error.md"), []byte(errContent), 0644); err != nil {
		t.Fatalf("failed to write error file: %v", err)
	}

	result, output, err := handleGetError(context.Background(), nil, getErrorInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil && result.IsError {
		t.Error("expected successful result, not error")
	}
	if output.Message != errContent {
		t.Errorf("Message = %q, want %q", output.Message, errContent)
	}

	// Verify error file was cleared
	if _, err := os.Stat(filepath.Join(tmpDir, "error.md")); !os.IsNotExist(err) {
		t.Error("error file should have been deleted after read")
	}
}

func TestHandleGetError_ReadAndClearIsIdempotent(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOOKFLOW_SESSION_DIR", tmpDir)

	// Write an error
	if err := os.WriteFile(filepath.Join(tmpDir, "error.md"), []byte("error1"), 0644); err != nil {
		t.Fatalf("failed to write error file: %v", err)
	}

	// First call returns and clears
	_, output1, _ := handleGetError(context.Background(), nil, getErrorInput{})
	if output1.Message != "error1" {
		t.Errorf("first call: Message = %q, want %q", output1.Message, "error1")
	}

	// Second call returns no errors
	_, output2, _ := handleGetError(context.Background(), nil, getErrorInput{})
	if output2.Message != "No pending errors" {
		t.Errorf("second call: Message = %q, want %q", output2.Message, "No pending errors")
	}
}

// --- Server construction tests ---

func TestServerCreation(t *testing.T) {
	// Verify that building the server doesn't panic
	server := mcpsdk.NewServer(
		&mcpsdk.Implementation{
			Name:    "hookflow",
			Version: "1.0.0",
		},
		nil,
	)

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "hookflow_get_error",
		Description: "Get and clear the current hookflow error.",
	}, handleGetError)

	if server == nil {
		t.Fatal("expected non-nil server")
	}
}

// --- getErrorOutput JSON tests ---

func TestGetErrorOutput_Serialization(t *testing.T) {
	output := getErrorOutput{Message: "test error message"}
	if output.Message != "test error message" {
		t.Errorf("Message = %q, want %q", output.Message, "test error message")
	}
}

func TestGetErrorOutput_EmptyMessage(t *testing.T) {
	output := getErrorOutput{}
	if output.Message != "" {
		t.Errorf("Message = %q, want empty", output.Message)
	}
}

// --- Integration with session package ---

func TestHandleGetError_SessionIntegration(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOOKFLOW_SESSION_DIR", tmpDir)

	// Use session package to write error
	err := session.WriteError("TestWorkflow", "step failed", "exit code 1")
	if err != nil {
		t.Fatalf("failed to write error: %v", err)
	}

	// Read via handler
	_, output, err := handleGetError(context.Background(), nil, getErrorInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output.Message == "No pending errors" {
		t.Error("expected error content, got 'No pending errors'")
	}
	if output.Message == "" {
		t.Error("expected non-empty error message")
	}
}
