package session

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMarkRepoHooksActive(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOOKFLOW_SESSION_DIR", dir)

	// Initially not active
	active, err := IsRepoHooksActive()
	if err != nil {
		t.Fatalf("IsRepoHooksActive() error: %v", err)
	}
	if active {
		t.Fatal("expected repo hooks not active initially")
	}

	// Mark as active
	if err := MarkRepoHooksActive(); err != nil {
		t.Fatalf("MarkRepoHooksActive() error: %v", err)
	}

	// Now should be active
	active, err = IsRepoHooksActive()
	if err != nil {
		t.Fatalf("IsRepoHooksActive() error: %v", err)
	}
	if !active {
		t.Fatal("expected repo hooks active after marking")
	}

	// Verify file exists
	markerPath := filepath.Join(dir, repoHooksActiveFileName)
	if _, err := os.Stat(markerPath); os.IsNotExist(err) {
		t.Fatal("marker file should exist on disk")
	}
}

func TestMarkRepoHooksActive_Idempotent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOOKFLOW_SESSION_DIR", dir)

	// Mark twice — should not error
	if err := MarkRepoHooksActive(); err != nil {
		t.Fatalf("first MarkRepoHooksActive() error: %v", err)
	}
	if err := MarkRepoHooksActive(); err != nil {
		t.Fatalf("second MarkRepoHooksActive() error: %v", err)
	}

	active, err := IsRepoHooksActive()
	if err != nil {
		t.Fatalf("IsRepoHooksActive() error: %v", err)
	}
	if !active {
		t.Fatal("expected repo hooks active")
	}
}

func TestIsRepoHooksActive_NoSessionDir(t *testing.T) {
	dir := t.TempDir()
	nonexistent := dir + string(os.PathSeparator) + "nonexistent"
	t.Setenv("HOOKFLOW_SESSION_DIR", nonexistent)

	active, err := IsRepoHooksActive()
	if err != nil {
		t.Fatalf("IsRepoHooksActive() should not error for nonexistent dir: %v", err)
	}
	if active {
		t.Fatal("expected not active for nonexistent dir")
	}
}

func TestClearRepoHooksActive(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOOKFLOW_SESSION_DIR", dir)

	// Mark as active
	if err := MarkRepoHooksActive(); err != nil {
		t.Fatalf("MarkRepoHooksActive() error: %v", err)
	}

	active, _ := IsRepoHooksActive()
	if !active {
		t.Fatal("expected repo hooks active after marking")
	}

	// Clear the marker
	if err := ClearRepoHooksActive(); err != nil {
		t.Fatalf("ClearRepoHooksActive() error: %v", err)
	}

	// Should no longer be active
	active, err := IsRepoHooksActive()
	if err != nil {
		t.Fatalf("IsRepoHooksActive() error after clear: %v", err)
	}
	if active {
		t.Fatal("expected repo hooks not active after clearing")
	}
}

func TestClearRepoHooksActive_NoMarker(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOOKFLOW_SESSION_DIR", dir)

	// Clear when no marker exists should not error
	if err := ClearRepoHooksActive(); err != nil {
		t.Fatalf("ClearRepoHooksActive() should not error when no marker: %v", err)
	}
}
