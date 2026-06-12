package logging

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.level.String(), "Level.String() should return correct string representation")
		})
	}
}

func TestLogDir(t *testing.T) {
	dir := logDir()
	require.NotEmpty(t, dir, "logDir() should return a non-empty path")

	// Should contain hookflow/logs path component
	assert.Contains(t, dir, "hookflow", "logDir() path should contain 'hookflow'")
	assert.Regexp(t, `logs$`, dir, "logDir() path should end with 'logs'")

	// Test the exported version too
	exportedDir := LogDir()
	assert.Equal(t, dir, exportedDir, "LogDir() should return the same path as logDir()")
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
	require.NoError(t, Init(), "Init() should succeed")
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
	require.NotEmpty(t, logPath, "LogPath() should return a non-empty path after Init()")
	assert.FileExists(t, logPath, "log file should exist after Init()")

	// Read log file and verify content
	content, err := os.ReadFile(logPath)
	require.NoError(t, err, "should be able to read the log file")

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
		assert.Contains(t, logContent, msg, "log file should contain expected message %q", msg)
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

	require.NoError(t, Init(), "Init() should succeed")
	defer Close()

	EnableDebug()

	// Use context logger
	ctx := Context("matcher")
	ctx.Debug("testing pattern %s", "*.json")
	ctx.Info("matched workflow %s", "lint.yml")
	ctx.Warn("skipping invalid workflow %s", "broken.yml")
	ctx.Error("workflow failed: %s", "fatal error")

	// Verify context prefix in logs
	content, err := os.ReadFile(LogPath())
	require.NoError(t, err, "should be able to read the log file")
	assert.Contains(t, string(content), "[matcher]", "log file should contain context prefix [matcher]")
}

func TestStartOperation(t *testing.T) {
	// Reset the singleton
	defaultLogger = nil
	once = sync.Once{}

	tmpDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", tmpDir)
	defer func() { _ = os.Setenv("HOME", originalHome) }()

	require.NoError(t, Init(), "Init() should succeed")
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
	content, err := os.ReadFile(LogPath())
	require.NoError(t, err, "should be able to read the log file")
	logContent := string(content)

	assert.Contains(t, logContent, "START workflow-match", "log should contain START entry for workflow-match")
	assert.Contains(t, logContent, "DONE workflow-match", "log should contain DONE entry for workflow-match")
	assert.Contains(t, logContent, "FAIL step-run", "log should contain FAIL entry for step-run")
}

func TestCleanOldLogs(t *testing.T) {
	tmpDir := t.TempDir()

	// Create some test log files
	oldFile := filepath.Join(tmpDir, "hookflow-2020-01-01.log")
	newFile := filepath.Join(tmpDir, "hookflow-2099-01-01.log")
	otherFile := filepath.Join(tmpDir, "other.txt")

	require.NoError(t, os.WriteFile(oldFile, []byte("old"), 0644), "should create old log file for test setup")
	require.NoError(t, os.WriteFile(newFile, []byte("new"), 0644), "should create new log file for test setup")
	require.NoError(t, os.WriteFile(otherFile, []byte("other"), 0644), "should create other file for test setup")

	// Set old modification time
	oldTime := time.Now().AddDate(0, 0, -30)
	require.NoError(t, os.Chtimes(oldFile, oldTime, oldTime), "should set old modification time for test setup")

	// Run cleanup
	cleanOldLogs(tmpDir, 7)

	// Old log should be deleted
	assert.NoFileExists(t, oldFile, "old log file should have been deleted after cleanup")

	// New log should still exist
	assert.FileExists(t, newFile, "new log file should not have been deleted by cleanup")

	// Non-log file should still exist
	assert.FileExists(t, otherFile, "non-log file should not have been deleted by cleanup")
}

func TestLogLevelFiltering(t *testing.T) {
	// Reset the singleton
	defaultLogger = nil
	once = sync.Once{}

	tmpDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", tmpDir)
	defer func() { _ = os.Setenv("HOME", originalHome) }()

	require.NoError(t, Init(), "Init() should succeed")
	defer Close()

	// Default level is INFO, so DEBUG should be filtered
	Debug("should be filtered")
	Info("should appear")

	content, err := os.ReadFile(LogPath())
	require.NoError(t, err, "should be able to read the log file")
	logContent := string(content)

	assert.NotContains(t, logContent, "should be filtered", "debug message should be filtered at INFO level")
	assert.Contains(t, logContent, "should appear", "info message should appear in log")
}

func TestTee(t *testing.T) {
	// Reset the singleton
	defaultLogger = nil
	once = sync.Once{}

	tmpDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", tmpDir)
	defer func() { _ = os.Setenv("HOME", originalHome) }()

	require.NoError(t, Init(), "Init() should succeed")
	defer Close()

	// Tee should return a multi-writer
	var buf strings.Builder
	writer := Tee(&buf)
	require.NotNil(t, writer, "Tee should return a non-nil writer")

	// Writing to tee should write to our buffer
	_, err := writer.Write([]byte("test output"))
	require.NoError(t, err, "Write to Tee writer should succeed")
	assert.Contains(t, buf.String(), "test output", "Tee should forward writes to the provided writer")
}

func TestTeeWithoutInit(t *testing.T) {
	// Reset the singleton
	defaultLogger = nil
	once = sync.Once{}

	// Tee without init should return the original writer
	var buf strings.Builder
	writer := Tee(&buf)
	assert.Equal(t, &buf, writer, "Tee without init should return the original writer unchanged")
}

func TestContextLoggerWarnAndError(t *testing.T) {
	// Reset the singleton
	defaultLogger = nil
	once = sync.Once{}

	tmpDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", tmpDir)
	defer func() { _ = os.Setenv("HOME", originalHome) }()

	require.NoError(t, Init(), "Init() should succeed")
	defer Close()

	ctx := Context("test-ctx")
	ctx.Warn("warning: %s", "test warning")
	ctx.Error("error: %s", "test error")

	content, err := os.ReadFile(LogPath())
	require.NoError(t, err, "should be able to read the log file")
	logContent := string(content)

	assert.Contains(t, logContent, "WARN", "log should contain WARN level entry")
	assert.Contains(t, logContent, "ERROR", "log should contain ERROR level entry")
	assert.Contains(t, logContent, "[test-ctx]", "log should contain context prefix [test-ctx]")
}
