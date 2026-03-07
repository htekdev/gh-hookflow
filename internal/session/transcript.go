package session

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

const (
	transcriptFileName    = "transcript.jsonl"
	seqFileName           = "transcript.seq"
	defaultMaxEntries     = 1000
	maxEntriesEnvVar      = "HOOKFLOW_TRANSCRIPT_MAX_ENTRIES"
)

// TranscriptEntry represents a single entry in the session transcript.
// Each hook invocation (pre or post) produces one entry.
type TranscriptEntry struct {
	Timestamp        int64                  `json:"timestamp"`
	Lifecycle        string                 `json:"lifecycle"`
	EventType        string                 `json:"eventType"`
	ToolName         string                 `json:"toolName,omitempty"`
	ToolArgs         map[string]interface{} `json:"toolArgs,omitempty"`
	ToolResult       map[string]interface{} `json:"toolResult,omitempty"`
	Decision         string                 `json:"decision,omitempty"`
	WorkflowsMatched []string               `json:"workflowsMatched,omitempty"`
	Seq              int64                  `json:"seq"`
}

// transcriptMu protects concurrent transcript file writes within the same process.
var transcriptMu sync.Mutex

// transcriptPath returns the path to the transcript JSONL file for the current session.
func transcriptPath() (string, error) {
	dir, err := GetSessionDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, transcriptFileName), nil
}

// seqPath returns the path to the sequence counter file for the current session.
func seqPath() (string, error) {
	dir, err := GetSessionDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, seqFileName), nil
}

// nextSeq reads, increments, and persists the sequence counter.
func nextSeq() (int64, error) {
	sp, err := seqPath()
	if err != nil {
		return 0, err
	}

	var seq int64
	data, err := os.ReadFile(sp)
	if err == nil {
		seq, _ = strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	}

	seq++
	if err := os.WriteFile(sp, []byte(strconv.FormatInt(seq, 10)), 0644); err != nil {
		return 0, fmt.Errorf("failed to write sequence file: %w", err)
	}
	return seq, nil
}

// maxEntries returns the configured maximum transcript entries.
func maxEntries() int {
	if v := os.Getenv(maxEntriesEnvVar); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return defaultMaxEntries
}

// AppendEntry appends a transcript entry to the session transcript file.
// It handles sequence numbering, size capping, and file creation automatically.
func AppendEntry(entry TranscriptEntry) error {
	transcriptMu.Lock()
	defer transcriptMu.Unlock()

	if err := EnsureSessionDir(); err != nil {
		return fmt.Errorf("failed to ensure session directory: %w", err)
	}

	seq, err := nextSeq()
	if err != nil {
		return fmt.Errorf("failed to get next sequence: %w", err)
	}
	entry.Seq = seq

	tp, err := transcriptPath()
	if err != nil {
		return err
	}

	// Enforce size cap before appending
	if err := enforceCapLocked(tp); err != nil {
		return fmt.Errorf("failed to enforce transcript cap: %w", err)
	}

	line, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal transcript entry: %w", err)
	}

	f, err := os.OpenFile(tp, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open transcript file: %w", err)
	}
	defer func() { _ = f.Close() }()

	if _, err := f.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("failed to write transcript entry: %w", err)
	}

	return nil
}

// enforceCapLocked truncates the transcript to the newest half if it exceeds the cap.
// Must be called with transcriptMu held.
func enforceCapLocked(tp string) error {
	cap := maxEntries()

	lines, err := readLines(tp)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	if len(lines) < cap {
		return nil
	}

	// Keep the newest half
	keep := cap / 2
	lines = lines[len(lines)-keep:]

	content := strings.Join(lines, "\n") + "\n"
	return os.WriteFile(tp, []byte(content), 0644)
}

// readLines reads all non-empty lines from a file.
func readLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	var lines []string
	scanner := bufio.NewScanner(f)
	// Increase scanner buffer for large JSONL lines
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines, scanner.Err()
}

// ReadTranscript reads all transcript entries from the session transcript file.
func ReadTranscript() ([]TranscriptEntry, error) {
	tp, err := transcriptPath()
	if err != nil {
		return nil, err
	}
	return readEntriesFromFile(tp)
}

// ReadTranscriptFromDir reads all transcript entries from a specific session directory.
func ReadTranscriptFromDir(sessionDir string) ([]TranscriptEntry, error) {
	tp := filepath.Join(sessionDir, transcriptFileName)
	return readEntriesFromFile(tp)
}

// readEntriesFromFile reads and parses all entries from a JSONL file.
func readEntriesFromFile(path string) ([]TranscriptEntry, error) {
	lines, err := readLines(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read transcript: %w", err)
	}

	entries := make([]TranscriptEntry, 0, len(lines))
	for _, line := range lines {
		var entry TranscriptEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue // skip malformed lines
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// ReadTranscriptRaw reads the raw JSONL lines from the session transcript.
// Returns the lines as-is for regex matching without parsing.
func ReadTranscriptRaw() ([]string, error) {
	tp, err := transcriptPath()
	if err != nil {
		return nil, err
	}
	return readRawLines(tp)
}

// ReadTranscriptRawFromDir reads raw JSONL lines from a specific session directory.
func ReadTranscriptRawFromDir(sessionDir string) ([]string, error) {
	tp := filepath.Join(sessionDir, transcriptFileName)
	return readRawLines(tp)
}

// readRawLines reads non-empty lines from a file, returning them unparsed.
func readRawLines(path string) ([]string, error) {
	lines, err := readLines(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return lines, nil
}

// FilterByRegex returns raw JSONL lines matching the given regex pattern.
func FilterByRegex(lines []string, pattern string) ([]string, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex pattern %q: %w", pattern, err)
	}

	var matched []string
	for _, line := range lines {
		if re.MatchString(line) {
			matched = append(matched, line)
		}
	}
	return matched, nil
}

// FilterSinceLastMatch returns all lines after the last line matching the regex.
// If no line matches, returns all lines (nothing to anchor to).
func FilterSinceLastMatch(lines []string, pattern string) ([]string, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex pattern %q: %w", pattern, err)
	}

	lastIdx := -1
	for i, line := range lines {
		if re.MatchString(line) {
			lastIdx = i
		}
	}

	if lastIdx == -1 {
		return lines, nil
	}

	if lastIdx+1 >= len(lines) {
		return nil, nil
	}
	return lines[lastIdx+1:], nil
}

// CountMatches returns the number of lines matching the regex pattern.
func CountMatches(lines []string, pattern string) (int, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return 0, fmt.Errorf("invalid regex pattern %q: %w", pattern, err)
	}

	count := 0
	for _, line := range lines {
		if re.MatchString(line) {
			count++
		}
	}
	return count, nil
}

// LastMatch returns the last line matching the regex, or empty string if none.
func LastMatch(lines []string, pattern string) (string, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", fmt.Errorf("invalid regex pattern %q: %w", pattern, err)
	}

	for i := len(lines) - 1; i >= 0; i-- {
		if re.MatchString(lines[i]) {
			return lines[i], nil
		}
	}
	return "", nil
}
