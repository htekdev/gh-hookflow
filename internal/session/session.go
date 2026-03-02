// Package session provides Copilot session identification and directory management.
// Sessions are identified by finding the "copilot" process in the process tree.
// Session-specific data is stored in ~/.hookflow/sessions/{copilot-pid}/
package session

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// ErrCopilotNotFound is returned when the copilot process cannot be found in the process tree
var ErrCopilotNotFound = fmt.Errorf("copilot process not found in process tree")

// sessionsDir returns the base sessions directory
func sessionsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(home, ".hookflow", "sessions"), nil
}

// GetSessionDir returns the session directory for the current Copilot session.
// Returns ~/.hookflow/sessions/{copilot-pid}/
// If HOOKFLOW_SESSION_DIR is set, uses that directly (for testing).
func GetSessionDir() (string, error) {
	if dir := os.Getenv("HOOKFLOW_SESSION_DIR"); dir != "" {
		return dir, nil
	}

	pid, err := GetCopilotPID()
	if err != nil {
		return "", err
	}

	base, err := sessionsDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(base, strconv.Itoa(pid)), nil
}

// GetErrorFilePath returns the path to the error.md file for the current session.
func GetErrorFilePath() (string, error) {
	dir, err := GetSessionDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "error.md"), nil
}

// EnsureSessionDir creates the session directory if it doesn't exist.
func EnsureSessionDir() error {
	dir, err := GetSessionDir()
	if err != nil {
		return err
	}
	return os.MkdirAll(dir, 0755)
}

// CleanupStaleSessions removes session directories for PIDs that no longer exist.
// This is called periodically to prevent disk bloat.
func CleanupStaleSessions() error {
	base, err := sessionsDir()
	if err != nil {
		return err
	}

	entries, err := os.ReadDir(base)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No sessions directory yet
		}
		return fmt.Errorf("failed to read sessions directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue // Not a PID directory
		}

		if !processExists(pid) {
			sessionDir := filepath.Join(base, entry.Name())
			_ = os.RemoveAll(sessionDir)
		}
	}

	return nil
}

// processExists checks if a process with the given PID exists
func processExists(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Unix, FindProcess always succeeds; we need to send signal 0 to check
	// On Windows, FindProcess fails if the process doesn't exist
	// The platform-specific implementation handles the difference
	return processExistsCheck(process, pid)
}
