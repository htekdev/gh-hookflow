package logging

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestLogLevelString(t *testing.T) {
	tests := []struct {
		level    Level
		expected string
	}{
		{LevelDebug, "DEBUG"},
		{LevelInfo, "INFO"},
		{LevelWarn, "WARN"},
		{LevelError, "ERROR"},
	}

	for _, tt := range tests {
		if got := tt.level.String(); got != tt.expected {
			t.Errorf("Level(%d).String() = %q, want %q", tt.level, got, tt.expected)
		}
	}
}

func TestLogDir(t *testing.T) {
	dir := logDir()
	if dir == "" {
		t.Error("logDir() returned empty string")
	}

	// Should end with .hookflow/logs or hookflow/logs
	if !strings.Contains(dir, "hookflow") || !strings.HasSuffix(dir, "logs") {
		t.Errorf("logDir() = %q, expected path containing hookflow/logs", dir)
	}

	// Test the exported version too
	exportedDir := LogDir()
	if exportedDir != dir {
		t.Errorf("LogDir() = %q, want %q (same as logDir())", exportedDir, dir)
	}
}

func TestInitAndLog(t *testing.T) {
	// Reset the singleton for testing
	defaultLogger = nil
	once = sync.Once{}

	// Use temp directory for test
	tmpDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", tmpDir)
	defer func() { _ = os.Setenv("HOME", originalHome) }()

	// Initialize
	err := Init()
	if err != nil {
		t.Fatalf("Init() failed: %v", err)
	}
	defer Close()

	// Log some messages
	Info("test info message")
	Warn("test warn message")
	Error("test error message")

	// Enable debug and log debug message
	EnableDebug()
	Debug("test debug message")

	// Check log file exists
	logPath := LogPath()
	if logPath == "" {
		t.Fatal("LogPath() returned empty string after Init()")
	}

	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Errorf("Log file does not exist: %s", logPath)
	}

	// Read log file and verify content
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	logContent := string(content)
	expectedMessages := []string{
		"INFO",
		"test info message",
		"WARN",
		"test warn message",
		"ERROR",
		"test error message",
		"DEBUG",
		"test debug message",
	}

	for _, msg := range expectedMessages {
		if !strings.Contains(logContent, msg) {
			t.Errorf("Log file missing expected content: %q", msg)
		}
	}
}

func TestContextLogger(t *testing.T) {
	// Reset the singleton
	defaultLogger = nil
	once = sync.Once{}

	tmpDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", tmpDir)
	defer func() { _ = os.Setenv("HOME", originalHome) }()

	err := Init()
	if err != nil {
		t.Fatalf("Init() failed: %v", err)
	}
	defer Close()

	EnableDebug()

	// Use context logger
	ctx := Context("matcher")
	ctx.Debug("testing pattern %s", "*.json")
	ctx.Info("matched workflow %s", "lint.yml")
	ctx.Warn("skipping invalid workflow %s", "broken.yml")
	ctx.Error("workflow failed: %s", "fatal error")

	// Verify context prefix in logs
	content, _ := os.ReadFile(LogPath())
	logContent := string(content)

	if !strings.Contains(logContent, "[matcher]") {
		t.Error("Log file missing context prefix [matcher]")
	}
}

func TestStartOperation(t *testing.T) {
	// Reset the singleton
	defaultLogger = nil
	once = sync.Once{}

	tmpDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", tmpDir)
	defer func() { _ = os.Setenv("HOME", originalHome) }()

	err := Init()
	if err != nil {
		t.Fatalf("Init() failed: %v", err)
	}
	defer Close()

	EnableDebug()

	// Test successful operation
	done := StartOperation("workflow-match", "dir=/workspace")
	time.Sleep(10 * time.Millisecond)
	done(nil)

	// Test failed operation
	done2 := StartOperation("step-run", "step=lint")
	done2(os.ErrNotExist)

	// Verify log content
	content, _ := os.ReadFile(LogPath())
	logContent := string(content)

	if !strings.Contains(logContent, "START workflow-match") {
		t.Error("Missing START entry for workflow-match")
	}
	if !strings.Contains(logContent, "DONE workflow-match") {
		t.Error("Missing DONE entry for workflow-match")
	}
	if !strings.Contains(logContent, "FAIL step-run") {
		t.Error("Missing FAIL entry for step-run")
	}
}

func TestCleanOldLogs(t *testing.T) {
	tmpDir := t.TempDir()

	// Create some test log files
	oldFile := filepath.Join(tmpDir, "hookflow-2020-01-01.log")
	newFile := filepath.Join(tmpDir, "hookflow-2099-01-01.log")
	otherFile := filepath.Join(tmpDir, "other.txt")

	_ = os.WriteFile(oldFile, []byte("old"), 0644)
	_ = os.WriteFile(newFile, []byte("new"), 0644)
	_ = os.WriteFile(otherFile, []byte("other"), 0644)

	// Set old modification time
	oldTime := time.Now().AddDate(0, 0, -30)
	_ = os.Chtimes(oldFile, oldTime, oldTime)

	// Run cleanup
	cleanOldLogs(tmpDir, 7)

	// Old log should be deleted
	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Error("Old log file should have been deleted")
	}

	// New log should still exist
	if _, err := os.Stat(newFile); os.IsNotExist(err) {
		t.Error("New log file should not have been deleted")
	}

	// Non-log file should still exist
	if _, err := os.Stat(otherFile); os.IsNotExist(err) {
		t.Error("Non-log file should not have been deleted")
	}
}

func TestLogLevelFiltering(t *testing.T) {
	// Reset the singleton
	defaultLogger = nil
	once = sync.Once{}

	tmpDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", tmpDir)
	defer func() { _ = os.Setenv("HOME", originalHome) }()

	err := Init()
	if err != nil {
		t.Fatalf("Init() failed: %v", err)
	}
	defer Close()

	// Default level is INFO, so DEBUG should be filtered
	Debug("should be filtered")
	Info("should appear")

	content, _ := os.ReadFile(LogPath())
	logContent := string(content)

	if strings.Contains(logContent, "should be filtered") {
		t.Error("Debug message should be filtered at INFO level")
	}
	if !strings.Contains(logContent, "should appear") {
		t.Error("Info message should appear")
	}
}

func TestTee(t *testing.T) {
	// Reset the singleton
	defaultLogger = nil
	once = sync.Once{}

	tmpDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", tmpDir)
	defer func() { _ = os.Setenv("HOME", originalHome) }()

	err := Init()
	if err != nil {
		t.Fatalf("Init() failed: %v", err)
	}
	defer Close()

	// Tee should return a multi-writer
	var buf strings.Builder
	writer := Tee(&buf)
	if writer == nil {
		t.Fatal("Tee returned nil")
	}

	// Writing to tee should write to our buffer
	_, err = writer.Write([]byte("test output"))
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if !strings.Contains(buf.String(), "test output") {
		t.Error("Tee did not write to provided writer")
	}
}

func TestTeeWithoutInit(t *testing.T) {
	// Reset the singleton
	defaultLogger = nil
	once = sync.Once{}

	// Tee without init should return the original writer
	var buf strings.Builder
	writer := Tee(&buf)
	if writer != &buf {
		t.Error("Tee without init should return the original writer")
	}
}

func TestContextLoggerWarnAndError(t *testing.T) {
	// Reset the singleton
	defaultLogger = nil
	once = sync.Once{}

	tmpDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", tmpDir)
	defer func() { _ = os.Setenv("HOME", originalHome) }()

	err := Init()
	if err != nil {
		t.Fatalf("Init() failed: %v", err)
	}
	defer Close()

	ctx := Context("test-ctx")
	ctx.Warn("warning: %s", "test warning")
	ctx.Error("error: %s", "test error")

	content, _ := os.ReadFile(LogPath())
	logContent := string(content)

	if !strings.Contains(logContent, "WARN") {
		t.Error("Log should contain WARN level entry")
	}
	if !strings.Contains(logContent, "ERROR") {
		t.Error("Log should contain ERROR level entry")
	}
	if !strings.Contains(logContent, "[test-ctx]") {
		t.Error("Log should contain context prefix")
	}
}
