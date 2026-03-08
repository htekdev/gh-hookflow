package e2e

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ── transcript recording ────────────────────────────────────────────

func TestTranscriptRecordsEntries(t *testing.T) {
	sessionDir := t.TempDir()

	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"allow-all.yml": `name: Allow All
lifecycle: pre
on:
  file:
    paths: ['**/*']
steps:
  - name: Allow
    run: echo "allowed"
`,
	})

	opts := &hookflowOpts{sessionDir: sessionDir}

	// Invoke hookflow twice to generate transcript entries
	eventJSON1 := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "file1.txt"),
		"file_text": "first",
	}, workspace)
	_, _ = runHookflow(t, workspace, eventJSON1, "preToolUse", opts)

	eventJSON2 := buildEventJSON("edit", map[string]interface{}{
		"path":    filepath.Join(workspace, "file2.txt"),
		"old_str": "old",
		"new_str": "new",
	}, workspace)
	_, _ = runHookflow(t, workspace, eventJSON2, "preToolUse", opts)

	// Check transcript file exists
	transcriptPath := filepath.Join(sessionDir, "transcript.jsonl")
	data, err := os.ReadFile(transcriptPath)
	if err != nil {
		t.Fatalf("transcript.jsonl not found: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 transcript entries, got %d", len(lines))
	}

	// Verify entries are valid JSON with expected fields
	for i, line := range lines {
		var entry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Errorf("line %d is not valid JSON: %v", i, err)
			continue
		}
		if _, ok := entry["timestamp"]; !ok {
			t.Errorf("line %d missing timestamp", i)
		}
		if _, ok := entry["lifecycle"]; !ok {
			t.Errorf("line %d missing lifecycle", i)
		}
		if _, ok := entry["seq"]; !ok {
			t.Errorf("line %d missing seq", i)
		}
	}
}

// ── transcript_count in workflow condition ───────────────────────────

func TestTranscriptCountExpression(t *testing.T) {
	sessionDir := t.TempDir()

	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"count-check.yml": `name: Transcript Count Check
lifecycle: pre
on:
  file:
    paths: ['**/*']
blocking: true
steps:
  - name: Block after 2 invocations
    if: ${{ transcript_count('toolName') > 2 }}
    run: |
      echo "Too many invocations"
      exit 1
`,
	})

	opts := &hookflowOpts{sessionDir: sessionDir}

	// First invocation: transcript writes entry (count=1), 1 > 2 is false → allow
	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "file1.txt"),
		"file_text": "first",
	}, workspace)
	result1, output1 := runHookflow(t, workspace, eventJSON, "preToolUse", opts)
	assertAllow(t, result1, output1)

	// Second invocation: count=2, 2 > 2 is false → allow
	eventJSON2 := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "file2.txt"),
		"file_text": "second",
	}, workspace)
	result2, output2 := runHookflow(t, workspace, eventJSON2, "preToolUse", opts)
	assertAllow(t, result2, output2)

	// Third invocation: count=3, 3 > 2 is true → deny
	eventJSON3 := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "file3.txt"),
		"file_text": "third",
	}, workspace)
	result3, output3 := runHookflow(t, workspace, eventJSON3, "preToolUse", opts)
	assertDeny(t, result3, output3, "")
}

// ── transcript_last in workflow ─────────────────────────────────────

func TestTranscriptLastExpression(t *testing.T) {
	sessionDir := t.TempDir()

	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"last-check.yml": `name: Transcript Last
lifecycle: pre
on:
  file:
    paths: ['**/*']
steps:
  - name: Show last
    run: |
      echo "Last entry: ${{ transcript_last('lifecycle') }}"
`,
	})

	opts := &hookflowOpts{sessionDir: sessionDir}

	// First invocation
	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "test.txt"),
		"file_text": "test",
	}, workspace)
	_, _ = runHookflow(t, workspace, eventJSON, "preToolUse", opts)

	// Second invocation — transcript_last should return data
	eventJSON2 := buildEventJSON("edit", map[string]interface{}{
		"path":    filepath.Join(workspace, "test.txt"),
		"old_str": "test",
		"new_str": "updated",
	}, workspace)
	result, output := runHookflow(t, workspace, eventJSON2, "preToolUse", opts)
	assertAllow(t, result, output)
}

// ── transcript_since in workflow ────────────────────────────────────

func TestTranscriptSinceExpression(t *testing.T) {
	sessionDir := t.TempDir()

	// Pre-populate transcript with some entries
	transcriptFile := filepath.Join(sessionDir, "transcript.jsonl")
	seqFile := filepath.Join(sessionDir, "transcript.seq")
	entries := []string{
		`{"timestamp":1000,"lifecycle":"pre","eventType":"file","toolName":"create","seq":1}`,
		`{"timestamp":2000,"lifecycle":"pre","eventType":"file","toolName":"edit","seq":2}`,
		`{"timestamp":3000,"lifecycle":"pre","eventType":"file","toolName":"create","seq":3}`,
	}
	_ = os.WriteFile(transcriptFile, []byte(strings.Join(entries, "\n")+"\n"), 0644)
	_ = os.WriteFile(seqFile, []byte("3"), 0644)

	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"since-check.yml": `name: Transcript Since
lifecycle: pre
on:
  file:
    paths: ['**/*']
steps:
  - name: Since last edit
    run: |
      echo "Since last edit: ${{ transcript_since('toolName.*edit') }}"
`,
	})

	opts := &hookflowOpts{sessionDir: sessionDir}

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "test.txt"),
		"file_text": "test",
	}, workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", opts)
	assertAllow(t, result, output)
}

// ── transcript() full dump ──────────────────────────────────────────

func TestTranscriptFullDump(t *testing.T) {
	sessionDir := t.TempDir()

	// Pre-populate transcript
	transcriptFile := filepath.Join(sessionDir, "transcript.jsonl")
	seqFile := filepath.Join(sessionDir, "transcript.seq")
	now := time.Now().Unix()
	entries := []string{
		`{"timestamp":` + itoa(now-10) + `,"lifecycle":"pre","eventType":"file","toolName":"create","seq":1}`,
		`{"timestamp":` + itoa(now-5) + `,"lifecycle":"pre","eventType":"file","toolName":"edit","seq":2}`,
	}
	_ = os.WriteFile(transcriptFile, []byte(strings.Join(entries, "\n")+"\n"), 0644)
	_ = os.WriteFile(seqFile, []byte("2"), 0644)

	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"transcript-all.yml": `name: Full Transcript
lifecycle: pre
on:
  file:
    paths: ['**/*']
steps:
  - name: Dump transcript
    run: |
      echo "Full transcript: ${{ transcript() }}"
  - name: Filtered transcript
    run: |
      echo "Create only: ${{ transcript('create') }}"
`,
	})

	opts := &hookflowOpts{sessionDir: sessionDir}

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "test.txt"),
		"file_text": "test",
	}, workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", opts)
	assertAllow(t, result, output)
}

func itoa(n int64) string {
	return fmt.Sprintf("%d", n)
}
