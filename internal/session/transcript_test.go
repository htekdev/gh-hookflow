package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
)

func TestAppendEntry_CreatesFileAndWritesEntry(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOOKFLOW_SESSION_DIR", dir)

	entry := TranscriptEntry{
		Timestamp: 1704614600000,
		Lifecycle: "pre",
		EventType: "preToolUse",
		ToolName:  "bash",
		ToolArgs:  map[string]interface{}{"command": "go test ./..."},
	}

	if err := AppendEntry(entry); err != nil {
		t.Fatalf("AppendEntry failed: %v", err)
	}

	// Verify file exists
	tp := filepath.Join(dir, transcriptFileName)
	data, err := os.ReadFile(tp)
	if err != nil {
		t.Fatalf("failed to read transcript file: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}

	var parsed TranscriptEntry
	if err := json.Unmarshal([]byte(lines[0]), &parsed); err != nil {
		t.Fatalf("failed to parse entry: %v", err)
	}

	if parsed.Seq != 1 {
		t.Errorf("expected seq=1, got %d", parsed.Seq)
	}
	if parsed.ToolName != "bash" {
		t.Errorf("expected toolName=bash, got %s", parsed.ToolName)
	}
	if parsed.Lifecycle != "pre" {
		t.Errorf("expected lifecycle=pre, got %s", parsed.Lifecycle)
	}
}

func TestAppendEntry_SequenceIncrements(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOOKFLOW_SESSION_DIR", dir)

	for i := 0; i < 5; i++ {
		entry := TranscriptEntry{
			Timestamp: int64(1704614600000 + i),
			Lifecycle: "pre",
			EventType: "preToolUse",
			ToolName:  "edit",
		}
		if err := AppendEntry(entry); err != nil {
			t.Fatalf("AppendEntry %d failed: %v", i, err)
		}
	}

	entries, err := ReadTranscript()
	if err != nil {
		t.Fatalf("ReadTranscript failed: %v", err)
	}

	if len(entries) != 5 {
		t.Fatalf("expected 5 entries, got %d", len(entries))
	}

	for i, e := range entries {
		expectedSeq := int64(i + 1)
		if e.Seq != expectedSeq {
			t.Errorf("entry %d: expected seq=%d, got %d", i, expectedSeq, e.Seq)
		}
	}
}

func TestAppendEntry_PostLifecycleWithResult(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOOKFLOW_SESSION_DIR", dir)

	entry := TranscriptEntry{
		Timestamp: 1704614700000,
		Lifecycle: "post",
		EventType: "postToolUse",
		ToolName:  "bash",
		ToolArgs:  map[string]interface{}{"command": "go test ./..."},
		ToolResult: map[string]interface{}{
			"resultType":     "success",
			"textResultForLlm": "All tests passed",
		},
		Decision: "allow",
	}

	if err := AppendEntry(entry); err != nil {
		t.Fatalf("AppendEntry failed: %v", err)
	}

	entries, err := ReadTranscript()
	if err != nil {
		t.Fatalf("ReadTranscript failed: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	result, ok := entries[0].ToolResult["resultType"]
	if !ok {
		t.Fatal("expected toolResult.resultType")
	}
	if result != "success" {
		t.Errorf("expected resultType=success, got %v", result)
	}
}

func TestReadTranscript_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOOKFLOW_SESSION_DIR", dir)

	entries, err := ReadTranscript()
	if err != nil {
		t.Fatalf("ReadTranscript on empty session should not error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestReadTranscript_MalformedLines(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOOKFLOW_SESSION_DIR", dir)

	tp := filepath.Join(dir, transcriptFileName)
	content := `{"timestamp":1,"lifecycle":"pre","toolName":"bash","seq":1}
not valid json
{"timestamp":2,"lifecycle":"post","toolName":"bash","seq":2}
`
	if err := os.WriteFile(tp, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	entries, err := ReadTranscript()
	if err != nil {
		t.Fatalf("ReadTranscript failed: %v", err)
	}

	// Malformed line should be skipped
	if len(entries) != 2 {
		t.Errorf("expected 2 valid entries (skipping malformed), got %d", len(entries))
	}
}

func TestReadTranscriptFromDir(t *testing.T) {
	dir := t.TempDir()

	tp := filepath.Join(dir, transcriptFileName)
	content := `{"timestamp":1,"lifecycle":"pre","toolName":"edit","seq":1}
{"timestamp":2,"lifecycle":"pre","toolName":"bash","seq":2}
`
	if err := os.WriteFile(tp, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	entries, err := ReadTranscriptFromDir(dir)
	if err != nil {
		t.Fatalf("ReadTranscriptFromDir failed: %v", err)
	}

	if len(entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(entries))
	}
}

func TestEnforceCap_TruncatesOldest(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOOKFLOW_SESSION_DIR", dir)
	t.Setenv(maxEntriesEnvVar, "10")

	// Write 10 entries (at the cap)
	for i := 0; i < 10; i++ {
		entry := TranscriptEntry{
			Timestamp: int64(i),
			Lifecycle: "pre",
			EventType: "preToolUse",
			ToolName:  "edit",
		}
		if err := AppendEntry(entry); err != nil {
			t.Fatalf("AppendEntry %d failed: %v", i, err)
		}
	}

	// At this point we should have exactly 10 entries
	entries, err := ReadTranscript()
	if err != nil {
		t.Fatalf("ReadTranscript failed: %v", err)
	}
	if len(entries) != 10 {
		t.Fatalf("expected 10 entries before cap, got %d", len(entries))
	}

	// Add one more to trigger truncation
	entry := TranscriptEntry{
		Timestamp: 100,
		Lifecycle: "pre",
		EventType: "preToolUse",
		ToolName:  "bash",
	}
	if err := AppendEntry(entry); err != nil {
		t.Fatalf("AppendEntry (trigger cap) failed: %v", err)
	}

	entries, err = ReadTranscript()
	if err != nil {
		t.Fatalf("ReadTranscript after cap failed: %v", err)
	}

	// Cap=10, truncate keeps newest half (5) + the new entry = 6
	if len(entries) != 6 {
		t.Errorf("expected 6 entries after cap (5 kept + 1 new), got %d", len(entries))
	}

	// Verify the oldest entries were removed — first kept should be seq 6
	// (entries 0-4 were dropped, entries 5-9 kept, plus new entry)
	if entries[0].Seq < 6 {
		t.Errorf("expected oldest kept entry seq >= 6, got %d", entries[0].Seq)
	}

	// Last entry should be the newly appended one
	last := entries[len(entries)-1]
	if last.ToolName != "bash" {
		t.Errorf("expected last entry toolName=bash, got %s", last.ToolName)
	}
}

func TestEnforceCap_CustomEnvVar(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOOKFLOW_SESSION_DIR", dir)
	t.Setenv(maxEntriesEnvVar, "4")

	for i := 0; i < 5; i++ {
		entry := TranscriptEntry{
			Timestamp: int64(i),
			Lifecycle: "pre",
			ToolName:  "edit",
		}
		if err := AppendEntry(entry); err != nil {
			t.Fatalf("AppendEntry %d failed: %v", i, err)
		}
	}

	entries, err := ReadTranscript()
	if err != nil {
		t.Fatalf("ReadTranscript failed: %v", err)
	}

	// Cap=4, truncation triggered on 5th write: keeps newest 2 + appends 1 = 3
	if len(entries) != 3 {
		t.Errorf("expected 3 entries with cap=4, got %d", len(entries))
	}
}

func TestFilterByRegex_MatchesToolName(t *testing.T) {
	lines := []string{
		`{"timestamp":1,"lifecycle":"pre","toolName":"bash","toolArgs":{"command":"go test ./..."},"seq":1}`,
		`{"timestamp":2,"lifecycle":"pre","toolName":"edit","toolArgs":{"path":"main.go"},"seq":2}`,
		`{"timestamp":3,"lifecycle":"pre","toolName":"bash","toolArgs":{"command":"git commit -m 'fix'"},"seq":3}`,
		`{"timestamp":4,"lifecycle":"pre","toolName":"edit","toolArgs":{"path":"test.go"},"seq":4}`,
	}

	matched, err := FilterByRegex(lines, `"toolName":"bash"`)
	if err != nil {
		t.Fatalf("FilterByRegex failed: %v", err)
	}
	if len(matched) != 2 {
		t.Errorf("expected 2 bash entries, got %d", len(matched))
	}
}

func TestFilterByRegex_MatchesCommandContent(t *testing.T) {
	lines := []string{
		`{"timestamp":1,"toolName":"powershell","toolArgs":{"command":"go test ./..."},"seq":1}`,
		`{"timestamp":2,"toolName":"powershell","toolArgs":{"command":"git commit -m 'fix bug'"},"seq":2}`,
		`{"timestamp":3,"toolName":"powershell","toolArgs":{"command":"go build ./..."},"seq":3}`,
	}

	matched, err := FilterByRegex(lines, `go test|npm test|pytest`)
	if err != nil {
		t.Fatalf("FilterByRegex failed: %v", err)
	}
	if len(matched) != 1 {
		t.Errorf("expected 1 test entry, got %d", len(matched))
	}
}

func TestFilterByRegex_MatchesFilePath(t *testing.T) {
	lines := []string{
		`{"timestamp":1,"toolName":"edit","toolArgs":{"path":"src/main.go"},"seq":1}`,
		`{"timestamp":2,"toolName":"edit","toolArgs":{"path":"src/utils.ts"},"seq":2}`,
		`{"timestamp":3,"toolName":"create","toolArgs":{"path":"test/main_test.go"},"seq":3}`,
	}

	matched, err := FilterByRegex(lines, `\.go"`)
	if err != nil {
		t.Fatalf("FilterByRegex failed: %v", err)
	}
	if len(matched) != 2 {
		t.Errorf("expected 2 Go file entries, got %d", len(matched))
	}
}

func TestFilterByRegex_InvalidPattern(t *testing.T) {
	_, err := FilterByRegex(nil, `[invalid`)
	if err == nil {
		t.Error("expected error for invalid regex")
	}
}

func TestFilterSinceLastMatch(t *testing.T) {
	lines := []string{
		`{"timestamp":1,"toolName":"edit","seq":1}`,
		`{"timestamp":2,"toolName":"powershell","toolArgs":{"command":"git commit -m 'first'"},"seq":2}`,
		`{"timestamp":3,"toolName":"edit","seq":3}`,
		`{"timestamp":4,"toolName":"powershell","toolArgs":{"command":"go test ./..."},"seq":4}`,
		`{"timestamp":5,"toolName":"edit","seq":5}`,
	}

	result, err := FilterSinceLastMatch(lines, `git commit`)
	if err != nil {
		t.Fatalf("FilterSinceLastMatch failed: %v", err)
	}

	if len(result) != 3 {
		t.Errorf("expected 3 entries after last 'git commit', got %d", len(result))
	}
}

func TestFilterSinceLastMatch_NoMatch(t *testing.T) {
	lines := []string{
		`{"timestamp":1,"toolName":"edit","seq":1}`,
		`{"timestamp":2,"toolName":"bash","seq":2}`,
	}

	result, err := FilterSinceLastMatch(lines, `git commit`)
	if err != nil {
		t.Fatalf("FilterSinceLastMatch failed: %v", err)
	}

	// No match means return all lines
	if len(result) != 2 {
		t.Errorf("expected all 2 lines when no match, got %d", len(result))
	}
}

func TestFilterSinceLastMatch_MatchIsLast(t *testing.T) {
	lines := []string{
		`{"timestamp":1,"toolName":"edit","seq":1}`,
		`{"timestamp":2,"toolName":"powershell","toolArgs":{"command":"git commit"},"seq":2}`,
	}

	result, err := FilterSinceLastMatch(lines, `git commit`)
	if err != nil {
		t.Fatalf("FilterSinceLastMatch failed: %v", err)
	}

	// Match is the last line, nothing after it
	if len(result) != 0 {
		t.Errorf("expected 0 entries after last-line match, got %d", len(result))
	}
}

func TestCountMatches(t *testing.T) {
	lines := []string{
		`{"timestamp":1,"toolName":"powershell","toolArgs":{"command":"go test ./..."},"seq":1}`,
		`{"timestamp":2,"toolName":"edit","seq":2}`,
		`{"timestamp":3,"toolName":"powershell","toolArgs":{"command":"npm test"},"seq":3}`,
		`{"timestamp":4,"toolName":"edit","seq":4}`,
	}

	count, err := CountMatches(lines, `go test|npm test`)
	if err != nil {
		t.Fatalf("CountMatches failed: %v", err)
	}
	if count != 2 {
		t.Errorf("expected count=2, got %d", count)
	}
}

func TestCountMatches_NoMatch(t *testing.T) {
	lines := []string{
		`{"timestamp":1,"toolName":"edit","seq":1}`,
	}

	count, err := CountMatches(lines, `git push`)
	if err != nil {
		t.Fatalf("CountMatches failed: %v", err)
	}
	if count != 0 {
		t.Errorf("expected count=0, got %d", count)
	}
}

func TestLastMatch(t *testing.T) {
	lines := []string{
		`{"timestamp":1,"toolName":"edit","toolArgs":{"path":"a.go"},"seq":1}`,
		`{"timestamp":2,"toolName":"bash","seq":2}`,
		`{"timestamp":3,"toolName":"edit","toolArgs":{"path":"b.go"},"seq":3}`,
		`{"timestamp":4,"toolName":"bash","seq":4}`,
	}

	result, err := LastMatch(lines, `"toolName":"edit"`)
	if err != nil {
		t.Fatalf("LastMatch failed: %v", err)
	}
	if result == "" {
		t.Fatal("expected a match")
	}
	if !strings.Contains(result, "b.go") {
		t.Errorf("expected last edit to be b.go, got: %s", result)
	}
}

func TestLastMatch_NoMatch(t *testing.T) {
	lines := []string{
		`{"timestamp":1,"toolName":"edit","seq":1}`,
	}

	result, err := LastMatch(lines, `git push`)
	if err != nil {
		t.Fatalf("LastMatch failed: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty result, got: %s", result)
	}
}

func TestReadTranscriptRaw(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOOKFLOW_SESSION_DIR", dir)

	tp := filepath.Join(dir, transcriptFileName)
	content := `{"timestamp":1,"toolName":"bash","seq":1}
{"timestamp":2,"toolName":"edit","seq":2}
`
	if err := os.WriteFile(tp, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	lines, err := ReadTranscriptRaw()
	if err != nil {
		t.Fatalf("ReadTranscriptRaw failed: %v", err)
	}
	if len(lines) != 2 {
		t.Errorf("expected 2 raw lines, got %d", len(lines))
	}
}

func TestReadTranscriptRaw_NoFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOOKFLOW_SESSION_DIR", dir)

	lines, err := ReadTranscriptRaw()
	if err != nil {
		t.Fatalf("ReadTranscriptRaw should not error on missing file: %v", err)
	}
	if len(lines) != 0 {
		t.Errorf("expected 0 lines, got %d", len(lines))
	}
}

func TestReadTranscriptRawFromDir(t *testing.T) {
	dir := t.TempDir()
	tp := filepath.Join(dir, transcriptFileName)
	content := `{"line":1}
{"line":2}
{"line":3}
`
	if err := os.WriteFile(tp, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	lines, err := ReadTranscriptRawFromDir(dir)
	if err != nil {
		t.Fatalf("ReadTranscriptRawFromDir failed: %v", err)
	}
	if len(lines) != 3 {
		t.Errorf("expected 3 lines, got %d", len(lines))
	}
}

func TestConcurrentAppend(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOOKFLOW_SESSION_DIR", dir)

	var wg sync.WaitGroup
	const goroutines = 10

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			entry := TranscriptEntry{
				Timestamp: int64(idx),
				Lifecycle: "pre",
				ToolName:  "bash",
			}
			if err := AppendEntry(entry); err != nil {
				t.Errorf("concurrent AppendEntry %d failed: %v", idx, err)
			}
		}(i)
	}
	wg.Wait()

	entries, err := ReadTranscript()
	if err != nil {
		t.Fatalf("ReadTranscript failed: %v", err)
	}

	if len(entries) != goroutines {
		t.Errorf("expected %d entries from concurrent writes, got %d", goroutines, len(entries))
	}

	// Verify all sequences are unique
	seqs := make(map[int64]bool)
	for _, e := range entries {
		if seqs[e.Seq] {
			t.Errorf("duplicate sequence number: %d", e.Seq)
		}
		seqs[e.Seq] = true
	}
}

func TestMaxEntries_DefaultAndCustom(t *testing.T) {
	// Default
	t.Setenv(maxEntriesEnvVar, "")
	if got := maxEntries(); got != defaultMaxEntries {
		t.Errorf("expected default %d, got %d", defaultMaxEntries, got)
	}

	// Custom
	t.Setenv(maxEntriesEnvVar, "50")
	if got := maxEntries(); got != 50 {
		t.Errorf("expected 50, got %d", got)
	}

	// Invalid (should fall back to default)
	t.Setenv(maxEntriesEnvVar, "notanumber")
	if got := maxEntries(); got != defaultMaxEntries {
		t.Errorf("expected default %d for invalid env, got %d", defaultMaxEntries, got)
	}

	// Zero (should fall back to default)
	t.Setenv(maxEntriesEnvVar, "0")
	if got := maxEntries(); got != defaultMaxEntries {
		t.Errorf("expected default %d for zero env, got %d", defaultMaxEntries, got)
	}
}

func TestFilterByRegex_EmptyLines(t *testing.T) {
	matched, err := FilterByRegex(nil, `test`)
	if err != nil {
		t.Fatalf("FilterByRegex on nil failed: %v", err)
	}
	if len(matched) != 0 {
		t.Errorf("expected 0 matches on nil, got %d", len(matched))
	}
}

func TestFilterSinceLastMatch_EmptyLines(t *testing.T) {
	result, err := FilterSinceLastMatch(nil, `test`)
	if err != nil {
		t.Fatalf("FilterSinceLastMatch on nil failed: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected 0 entries on nil, got %d", len(result))
	}
}

func TestCountMatches_EmptyLines(t *testing.T) {
	count, err := CountMatches(nil, `test`)
	if err != nil {
		t.Fatalf("CountMatches on nil failed: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0, got %d", count)
	}
}

func TestLastMatch_EmptyLines(t *testing.T) {
	result, err := LastMatch(nil, `test`)
	if err != nil {
		t.Fatalf("LastMatch on nil failed: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty, got %s", result)
	}
}

func TestAppendEntry_WithDecisionAndWorkflows(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOOKFLOW_SESSION_DIR", dir)

	entry := TranscriptEntry{
		Timestamp:        1704614600000,
		Lifecycle:        "pre",
		EventType:        "preToolUse",
		ToolName:         "create",
		ToolArgs:         map[string]interface{}{"path": ".env"},
		Decision:         "deny",
		WorkflowsMatched: []string{"block-sensitive-files"},
	}

	if err := AppendEntry(entry); err != nil {
		t.Fatalf("AppendEntry failed: %v", err)
	}

	entries, err := ReadTranscript()
	if err != nil {
		t.Fatalf("ReadTranscript failed: %v", err)
	}

	if entries[0].Decision != "deny" {
		t.Errorf("expected decision=deny, got %s", entries[0].Decision)
	}
	if len(entries[0].WorkflowsMatched) != 1 || entries[0].WorkflowsMatched[0] != "block-sensitive-files" {
		t.Errorf("expected workflowsMatched=[block-sensitive-files], got %v", entries[0].WorkflowsMatched)
	}
}

// Suppress unused import warnings for strconv — it's used in the main file
var _ = strconv.Atoi
