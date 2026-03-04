package session

import (
	"os"
	"path/filepath"
)

const repoHooksActiveFileName = "repo-hooks-active"

// repoHooksActivePath returns the path to the marker file for the current session.
func repoHooksActivePath() (string, error) {
	dir, err := GetSessionDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, repoHooksActiveFileName), nil
}

// MarkRepoHooksActive creates the marker file indicating repo hooks are handling hookflow.
func MarkRepoHooksActive() error {
	path, err := repoHooksActivePath()
	if err != nil {
		return err
	}
	if err := EnsureSessionDir(); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(""), 0644)
}

// ClearRepoHooksActive removes the marker file, used when the marker is stale
// (e.g., hooks.json was deleted mid-session).
func ClearRepoHooksActive() error {
	path, err := repoHooksActivePath()
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// IsRepoHooksActive returns true if the marker file exists for the current session.
func IsRepoHooksActive() (bool, error) {
	path, err := repoHooksActivePath()
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
