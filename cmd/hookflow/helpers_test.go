package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Tests for pure helper functions across multiple cmd files

func TestGenerateFileName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"My Workflow", "my-workflow"},
		{"test_name", "test-name"},
		{"Hello World!", "hello-world"},
		{"  spaces  ", "spaces"},
		{"UPPER_CASE", "upper-case"},
		{"multi---hyphens", "multi-hyphens"},
		{"special@#$chars", "specialchars"},
		{"", "generated-workflow"},
		{"---", "generated-workflow"},
		{"valid-name", "valid-name"},
	}
	for _, tt := range tests {
		got := generateFileName(tt.input)
		if got != tt.want {
			t.Errorf("generateFileName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestBuildShiftLeftPrompt(t *testing.T) {
	workflows := []string{"name: CI\non: push", "name: Lint\non: pull_request"}
	result := buildShiftLeftPrompt(workflows)

	if !strings.Contains(result, "CI Workflows to Analyze") {
		t.Error("expected prompt to contain analysis header")
	}
	if !strings.Contains(result, "name: CI") {
		t.Error("expected prompt to contain first workflow")
	}
	if !strings.Contains(result, "name: Lint") {
		t.Error("expected prompt to contain second workflow")
	}
	if !strings.Contains(result, "shift these checks LEFT") {
		t.Error("expected prompt to mention shifting left")
	}
}

func TestBuildShiftRightPrompt(t *testing.T) {
	workflows := []string{"name: Protect Files\non:\n  file:\n    paths: ['*.go']"}
	result := buildShiftRightPrompt(workflows)

	if !strings.Contains(result, "Agent Workflows to Analyze") {
		t.Error("expected prompt to contain analysis header")
	}
	if !strings.Contains(result, "name: Protect Files") {
		t.Error("expected prompt to contain workflow content")
	}
	if !strings.Contains(result, "pull_request") {
		t.Error("expected prompt to mention pull_request")
	}
}

func TestBuildMockEvent(t *testing.T) {
	tests := []struct {
		name      string
		eventType string
		opts      testEventOptions
	}{
		{
			name:      "commit event",
			eventType: "commit",
			opts:      testEventOptions{Message: "test msg"},
		},
		{
			name:      "commit with path",
			eventType: "commit",
			opts:      testEventOptions{Path: "src/main.go"},
		},
		{
			name:      "push event",
			eventType: "push",
			opts:      testEventOptions{Branch: "main"},
		},
		{
			name:      "file event default",
			eventType: "file",
			opts:      testEventOptions{},
		},
		{
			name:      "file event with action",
			eventType: "file",
			opts:      testEventOptions{Path: "test.go", Action: "create"},
		},
		{
			name:      "hook event",
			eventType: "hook",
			opts:      testEventOptions{Path: "src/app.ts"},
		},
		{
			name:      "tool event",
			eventType: "tool",
			opts:      testEventOptions{Path: "src/app.ts"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evt := buildMockEvent(tt.eventType, tt.opts)
			if evt == nil {
				t.Fatal("expected non-nil event")
			}
			switch tt.eventType {
			case "commit":
				if evt.Commit == nil {
					t.Error("expected Commit to be set")
				}
			case "push":
				if evt.Push == nil {
					t.Error("expected Push to be set")
				}
			case "file":
				if evt.File == nil {
					t.Error("expected File to be set")
				}
			case "hook", "tool":
				if evt.Hook == nil {
					t.Error("expected Hook to be set")
				}
			}
		})
	}
}

func TestHasHookflowHooks(t *testing.T) {
	tmpDir := t.TempDir()

	// Test missing file
	if hasHookflowHooks(filepath.Join(tmpDir, "nonexistent.json")) {
		t.Error("expected false for nonexistent file")
	}

	// Test invalid JSON
	badFile := filepath.Join(tmpDir, "bad.json")
	_ = os.WriteFile(badFile, []byte("not json"), 0644)
	if hasHookflowHooks(badFile) {
		t.Error("expected false for invalid JSON")
	}

	// Test hooks without hookflow
	noHookflow := filepath.Join(tmpDir, "no-hookflow.json")
	data, _ := json.Marshal(map[string]interface{}{
		"hooks": map[string]interface{}{
			"preToolUse":  []string{},
			"postToolUse": []string{},
		},
	})
	_ = os.WriteFile(noHookflow, data, 0644)
	if hasHookflowHooks(noHookflow) {
		t.Error("expected false when hookflow not in hooks")
	}

	// Test hooks with hookflow
	withHookflow := filepath.Join(tmpDir, "with-hookflow.json")
	_ = os.WriteFile(withHookflow, []byte(`{
		"hooks": {
			"preToolUse": [{"command": "hookflow run --raw"}],
			"postToolUse": []
		}
	}`), 0644)
	if !hasHookflowHooks(withHookflow) {
		t.Error("expected true when hookflow in hooks")
	}
}

func TestHasHookflowMCP(t *testing.T) {
	tmpDir := t.TempDir()

	// Test missing file
	if hasHookflowMCP(filepath.Join(tmpDir, "nonexistent.json")) {
		t.Error("expected false for nonexistent file")
	}

	// Test invalid JSON
	badFile := filepath.Join(tmpDir, "bad.json")
	_ = os.WriteFile(badFile, []byte("not json"), 0644)
	if hasHookflowMCP(badFile) {
		t.Error("expected false for invalid JSON")
	}

	// Test without hookflow MCP
	noMCP := filepath.Join(tmpDir, "no-mcp.json")
	_ = os.WriteFile(noMCP, []byte(`{"mcpServers": {"other": {}}}`), 0644)
	if hasHookflowMCP(noMCP) {
		t.Error("expected false when hookflow not in MCP servers")
	}

	// Test with hookflow MCP
	withMCP := filepath.Join(tmpDir, "with-mcp.json")
	_ = os.WriteFile(withMCP, []byte(`{"mcpServers": {"hookflow": {"command": "hookflow"}}}`), 0644)
	if !hasHookflowMCP(withMCP) {
		t.Error("expected true when hookflow in MCP servers")
	}
}
