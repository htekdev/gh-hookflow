package session

import (
	"os"
	"path/filepath"
)

const sentinelFileName = "global-only"

// sentinelPath returns the path to the sentinel marker file for the current session.
func sentinelPath() (string, error) {
	dir, err := GetSessionDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, sentinelFileName), nil
}

// ToggleSentinel creates the sentinel file if it doesn't exist, or deletes it if it does.
// Returns true if the sentinel now exists (was created), false if it was deleted.
func ToggleSentinel() (bool, error) {
	path, err := sentinelPath()
	if err != nil {
		return false, err
	}

	if _, err := os.Stat(path); err == nil {
		// File exists — delete it
		if err := os.Remove(path); err != nil {
			return false, err
		}
		return false, nil
	}

	// File doesn't exist — create it
	if err := EnsureSessionDir(); err != nil {
		return false, err
	}
	if err := os.WriteFile(path, []byte(""), 0644); err != nil {
		return false, err
	}
	return true, nil
}

// HasSentinel returns true if the sentinel file exists for the current session.
func HasSentinel() (bool, error) {
	path, err := sentinelPath()
	if err != nil {
		return false, err
	}

	_, err = os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
