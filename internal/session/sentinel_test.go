package session

import (
	"os"
	"testing"
)

func TestToggleSentinel_CreateAndDelete(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOOKFLOW_SESSION_DIR", dir)

	// Initially no sentinel
	has, err := HasSentinel()
	if err != nil {
		t.Fatalf("HasSentinel() error: %v", err)
	}
	if has {
		t.Fatal("expected no sentinel initially")
	}

	// First toggle creates it
	created, err := ToggleSentinel()
	if err != nil {
		t.Fatalf("ToggleSentinel() error: %v", err)
	}
	if !created {
		t.Fatal("expected sentinel to be created")
	}

	has, err = HasSentinel()
	if err != nil {
		t.Fatalf("HasSentinel() error: %v", err)
	}
	if !has {
		t.Fatal("expected sentinel to exist after creation")
	}

	// Second toggle deletes it
	created, err = ToggleSentinel()
	if err != nil {
		t.Fatalf("ToggleSentinel() error: %v", err)
	}
	if created {
		t.Fatal("expected sentinel to be deleted")
	}

	has, err = HasSentinel()
	if err != nil {
		t.Fatalf("HasSentinel() error: %v", err)
	}
	if has {
		t.Fatal("expected no sentinel after deletion")
	}
}

func TestToggleSentinel_DoubleToggle(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOOKFLOW_SESSION_DIR", dir)

	// Toggle on
	created, _ := ToggleSentinel()
	if !created {
		t.Fatal("first toggle should create")
	}

	// Toggle off
	created, _ = ToggleSentinel()
	if created {
		t.Fatal("second toggle should delete")
	}

	// Toggle on again
	created, _ = ToggleSentinel()
	if !created {
		t.Fatal("third toggle should create again")
	}
}

func TestHasSentinel_NoSessionDir(t *testing.T) {
	dir := t.TempDir()
	nonexistent := dir + string(os.PathSeparator) + "nonexistent"
	t.Setenv("HOOKFLOW_SESSION_DIR", nonexistent)

	has, err := HasSentinel()
	if err != nil {
		t.Fatalf("HasSentinel() should not error for nonexistent dir: %v", err)
	}
	if has {
		t.Fatal("expected no sentinel for nonexistent dir")
	}
}
