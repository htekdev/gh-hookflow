package session

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestSessionsDir(t *testing.T) {
	dir, err := sessionsDir()
	if err != nil {
		t.Fatalf("sessionsDir() failed: %v", err)
	}

	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".hookflow", "sessions")
	if dir != expected {
		t.Errorf("sessionsDir() = %q, want %q", dir, expected)
	}
}

func TestGetSessionDir_NoCopilot(t *testing.T) {
	// When not running under Copilot, GetSessionDir should return an error
	_, err := GetSessionDir()
	if err == nil {
		// If it succeeds, there's actually a copilot process in the tree
		// This is fine in CI or when running under Copilot
		t.Log("GetSessionDir succeeded - likely running under Copilot")
		return
	}

	if err != ErrCopilotNotFound {
		t.Errorf("GetSessionDir() error = %v, want ErrCopilotNotFound", err)
	}
}

func TestGetErrorFilePath(t *testing.T) {
	// Skip if not running under Copilot
	pid, err := GetCopilotPID()
	if err != nil {
		t.Skip("Not running under Copilot")
	}

	errorPath, err := GetErrorFilePath()
	if err != nil {
		t.Fatalf("GetErrorFilePath() failed: %v", err)
	}

	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".hookflow", "sessions", strconv.Itoa(pid), "error.md")
	if errorPath != expected {
		t.Errorf("GetErrorFilePath() = %q, want %q", errorPath, expected)
	}
}

func TestEnsureSessionDir(t *testing.T) {
	// Skip if not running under Copilot
	_, err := GetCopilotPID()
	if err != nil {
		t.Skip("Not running under Copilot")
	}

	err = EnsureSessionDir()
	if err != nil {
		t.Fatalf("EnsureSessionDir() failed: %v", err)
	}

	dir, _ := GetSessionDir()
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("Session directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Errorf("Session path is not a directory")
	}

	// Cleanup
	_ = os.RemoveAll(dir)
}

func TestProcessExists(t *testing.T) {
	// Current process should exist
	if !processExists(os.Getpid()) {
		t.Error("processExists(os.Getpid()) = false, want true")
	}

	// Non-existent PID should not exist (use a very high unlikely PID)
	// Note: This test might be flaky on systems with many processes
	if processExists(999999999) {
		t.Error("processExists(999999999) = true, want false")
	}
}

func TestCleanupStaleSessions(t *testing.T) {
	// Create a temporary sessions directory structure
	base, err := sessionsDir()
	if err != nil {
		t.Fatalf("sessionsDir() failed: %v", err)
	}

	// Create a fake stale session directory with a PID that definitely doesn't exist
	stalePID := "999999999"
	staleDir := filepath.Join(base, stalePID)
	if err := os.MkdirAll(staleDir, 0755); err != nil {
		t.Fatalf("Failed to create stale session dir: %v", err)
	}

	// Create a test file in the stale directory
	testFile := filepath.Join(staleDir, "error.md")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Run cleanup
	err = CleanupStaleSessions()
	if err != nil {
		t.Fatalf("CleanupStaleSessions() failed: %v", err)
	}

	// Stale directory should be removed
	if _, err := os.Stat(staleDir); !os.IsNotExist(err) {
		t.Errorf("Stale session directory should have been removed")
		_ = os.RemoveAll(staleDir) // Cleanup
	}
}

func TestGetCopilotPID_Integration(t *testing.T) {
	pid, err := GetCopilotPID()
	if err == ErrCopilotNotFound {
		t.Log("Not running under Copilot - expected in standalone test runs")
		return
	}
	if err != nil {
		t.Fatalf("GetCopilotPID() failed with unexpected error: %v", err)
	}

	// If we found a PID, it should be valid
	if pid <= 0 {
		t.Errorf("GetCopilotPID() = %d, want positive PID", pid)
	}
	t.Logf("Found Copilot PID: %d", pid)
}
