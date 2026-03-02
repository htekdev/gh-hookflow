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

func TestProcessExists_ZeroPID(t *testing.T) {
	// PID 0 should not panic; result is OS-dependent
	_ = processExists(0)
}

func TestProcessExists_NegativePID(t *testing.T) {
	if processExists(-1) {
		t.Error("processExists(-1) = true, want false")
	}
}

func TestProcessExists_CurrentProcess(t *testing.T) {
	if !processExists(os.Getpid()) {
		t.Error("processExists(os.Getpid()) should be true")
	}
}

func TestCleanupStaleSessions_NoSessionsDir(t *testing.T) {
	// If sessions dir doesn't exist yet, cleanup should succeed silently
	base, err := sessionsDir()
	if err != nil {
		t.Fatalf("sessionsDir() failed: %v", err)
	}

	// Only test if the sessions dir doesn't already exist
	if _, err := os.Stat(base); os.IsNotExist(err) {
		err = CleanupStaleSessions()
		if err != nil {
			t.Fatalf("CleanupStaleSessions() should succeed when no sessions dir: %v", err)
		}
	} else {
		t.Log("Sessions dir already exists, skipping no-dir test")
	}
}

func TestCleanupStaleSessions_NonPIDEntries(t *testing.T) {
	base, err := sessionsDir()
	if err != nil {
		t.Fatalf("sessionsDir() failed: %v", err)
	}

	// Create directories with non-numeric names
	nonPIDDir := filepath.Join(base, "not-a-pid")
	if err := os.MkdirAll(nonPIDDir, 0755); err != nil {
		t.Fatalf("Failed to create non-PID dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(nonPIDDir) }()

	err = CleanupStaleSessions()
	if err != nil {
		t.Fatalf("CleanupStaleSessions() failed: %v", err)
	}

	// Non-PID directory should still exist
	if _, err := os.Stat(nonPIDDir); os.IsNotExist(err) {
		t.Error("Non-PID directory should not be removed by cleanup")
	}
}

func TestCleanupStaleSessions_ActiveProcess(t *testing.T) {
	base, err := sessionsDir()
	if err != nil {
		t.Fatalf("sessionsDir() failed: %v", err)
	}

	// Create directory with current process PID (should not be cleaned up)
	activePID := strconv.Itoa(os.Getpid())
	activeDir := filepath.Join(base, activePID)
	if err := os.MkdirAll(activeDir, 0755); err != nil {
		t.Fatalf("Failed to create active PID dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(activeDir) }()

	err = CleanupStaleSessions()
	if err != nil {
		t.Fatalf("CleanupStaleSessions() failed: %v", err)
	}

	// Active process directory should still exist
	if _, err := os.Stat(activeDir); os.IsNotExist(err) {
		t.Error("Active process directory should not be removed")
	}
}

func TestCleanupStaleSessions_FileEntries(t *testing.T) {
	base, err := sessionsDir()
	if err != nil {
		t.Fatalf("sessionsDir() failed: %v", err)
	}

	if err := os.MkdirAll(base, 0755); err != nil {
		t.Fatalf("Failed to create base dir: %v", err)
	}

	// Create a regular file (not a directory) in sessions dir
	filePath := filepath.Join(base, "999999998")
	if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}
	defer func() { _ = os.Remove(filePath) }()

	err = CleanupStaleSessions()
	if err != nil {
		t.Fatalf("CleanupStaleSessions() failed: %v", err)
	}

	// File should still exist (skipped because not a directory)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Error("File entries should not be removed by cleanup")
	}
}

func TestCleanupStaleSessions_MixedEntries(t *testing.T) {
	base, err := sessionsDir()
	if err != nil {
		t.Fatalf("sessionsDir() failed: %v", err)
	}

	if err := os.MkdirAll(base, 0755); err != nil {
		t.Fatalf("Failed to create base dir: %v", err)
	}

	// Stale PID directory (should be removed)
	staleDir := filepath.Join(base, "999999997")
	if err := os.MkdirAll(staleDir, 0755); err != nil {
		t.Fatalf("Failed to create stale dir: %v", err)
	}

	// Active PID directory (should NOT be removed)
	activePID := strconv.Itoa(os.Getpid())
	activeDir := filepath.Join(base, activePID)
	if err := os.MkdirAll(activeDir, 0755); err != nil {
		t.Fatalf("Failed to create active dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(activeDir) }()

	// Non-PID directory (should NOT be removed)
	nonPIDDir := filepath.Join(base, "readme")
	if err := os.MkdirAll(nonPIDDir, 0755); err != nil {
		t.Fatalf("Failed to create non-PID dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(nonPIDDir) }()

	err = CleanupStaleSessions()
	if err != nil {
		t.Fatalf("CleanupStaleSessions() failed: %v", err)
	}

	// Stale dir should be gone
	if _, err := os.Stat(staleDir); !os.IsNotExist(err) {
		t.Error("Stale session directory should have been removed")
		_ = os.RemoveAll(staleDir)
	}

	// Active dir should remain
	if _, err := os.Stat(activeDir); os.IsNotExist(err) {
		t.Error("Active process directory should not be removed")
	}

	// Non-PID dir should remain
	if _, err := os.Stat(nonPIDDir); os.IsNotExist(err) {
		t.Error("Non-PID directory should not be removed")
	}
}

func TestGetSessionDir_ReturnsPath(t *testing.T) {
	pid, err := GetCopilotPID()
	if err != nil {
		t.Skip("Not running under Copilot")
	}

	dir, err := GetSessionDir()
	if err != nil {
		t.Fatalf("GetSessionDir() failed: %v", err)
	}

	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".hookflow", "sessions", strconv.Itoa(pid))
	if dir != expected {
		t.Errorf("GetSessionDir() = %q, want %q", dir, expected)
	}
}
