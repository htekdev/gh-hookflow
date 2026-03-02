package ai

import (
	"context"
	"strings"
	"testing"
	"time"
)

// --- NewClient tests ---

func TestNewClient(t *testing.T) {
	c := NewClient()
	if c == nil {
		t.Fatal("NewClient returned nil")
	}
	if c.started {
		t.Error("new client should not be started")
	}
	if c.client != nil {
		t.Error("new client should have nil underlying client")
	}
}

// --- Stop tests ---

func TestStop_NotStarted(t *testing.T) {
	c := NewClient()
	err := c.Stop()
	if err != nil {
		t.Errorf("Stop on unstarted client should return nil, got: %v", err)
	}
}

func TestStop_StartedButNilClient(t *testing.T) {
	c := NewClient()
	// Simulate started state without real SDK client
	c.started = true
	// Stop returns early when client is nil (guard: !started || client == nil)
	err := c.Stop()
	if err != nil {
		t.Errorf("Stop should return nil when client is nil, got: %v", err)
	}
}

// --- GenerateWorkflow tests (no SDK) ---

func TestGenerateWorkflow_NotStarted(t *testing.T) {
	c := NewClient()
	result, err := c.GenerateWorkflow(context.Background(), "test prompt")
	if err == nil {
		t.Fatal("expected error for unstarted client")
	}
	if !strings.Contains(err.Error(), "client not started") {
		t.Errorf("expected 'client not started' error, got: %v", err)
	}
	if result != nil {
		t.Error("expected nil result for unstarted client")
	}
}

// --- extractYAML tests ---

func TestExtractYAML_YAMLCodeBlock(t *testing.T) {
	input := "Here is the workflow:\n```yaml\nname: test\non:\n  file:\n    paths:\n      - '*.go'\n```\nDone."
	got := extractYAML(input)
	if !strings.HasPrefix(got, "name: test") {
		t.Errorf("expected YAML starting with 'name: test', got: %q", got)
	}
	if strings.Contains(got, "```") {
		t.Errorf("extracted YAML should not contain backticks, got: %q", got)
	}
}

func TestExtractYAML_GenericCodeBlock(t *testing.T) {
	input := "Result:\n```\nname: my-workflow\nsteps:\n  - run: echo hi\n```"
	got := extractYAML(input)
	if !strings.HasPrefix(got, "name: my-workflow") {
		t.Errorf("expected YAML starting with 'name: my-workflow', got: %q", got)
	}
}

func TestExtractYAML_DocumentSeparator(t *testing.T) {
	input := "Here is the workflow:\n---\nname: separator-test\nsteps:\n  - run: echo hi"
	got := extractYAML(input)
	if !strings.HasPrefix(got, "---") {
		t.Errorf("expected YAML starting with '---', got: %q", got)
	}
	if !strings.Contains(got, "separator-test") {
		t.Errorf("expected YAML to contain 'separator-test', got: %q", got)
	}
}

func TestExtractYAML_NameStart(t *testing.T) {
	input := "name: direct-name\nsteps:\n  - run: echo"
	got := extractYAML(input)
	if !strings.HasPrefix(got, "name: direct-name") {
		t.Errorf("expected YAML starting with 'name: direct-name', got: %q", got)
	}
}

func TestExtractYAML_Empty(t *testing.T) {
	got := extractYAML("No YAML here, just plain text.")
	if got != "" {
		t.Errorf("expected empty string for non-YAML input, got: %q", got)
	}
}

func TestExtractYAML_EmptyString(t *testing.T) {
	got := extractYAML("")
	if got != "" {
		t.Errorf("expected empty string for empty input, got: %q", got)
	}
}

func TestExtractYAML_YAMLBlockPriority(t *testing.T) {
	// yaml code block should be preferred over --- separator
	input := "---\nignore this\n```yaml\nname: preferred\n```"
	got := extractYAML(input)
	if !strings.HasPrefix(got, "name: preferred") {
		t.Errorf("yaml code block should have priority, got: %q", got)
	}
}

func TestExtractYAML_MultilineContent(t *testing.T) {
	input := "```yaml\nname: multi\ndescription: |\n  This is a\n  multiline description\nsteps:\n  - name: step1\n    run: echo hello\n```"
	got := extractYAML(input)
	if !strings.Contains(got, "multiline description") {
		t.Errorf("should preserve multiline content, got: %q", got)
	}
}

// --- extractWorkflowName tests ---

func TestExtractWorkflowName_Simple(t *testing.T) {
	got := extractWorkflowName("name: my-workflow\nsteps:\n  - run: echo")
	if got != "my-workflow" {
		t.Errorf("expected 'my-workflow', got: %q", got)
	}
}

func TestExtractWorkflowName_Quoted(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`name: "quoted-name"`, "quoted-name"},
		{`name: 'single-quoted'`, "single-quoted"},
		{`name: "mixed'quotes"`, "mixed'quotes"},
	}
	for _, tt := range tests {
		got := extractWorkflowName(tt.input)
		if got != tt.expected {
			t.Errorf("extractWorkflowName(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestExtractWorkflowName_WithPrefix(t *testing.T) {
	yaml := "---\ndescription: something\nname: after-prefix\nsteps: []"
	got := extractWorkflowName(yaml)
	if got != "after-prefix" {
		t.Errorf("expected 'after-prefix', got: %q", got)
	}
}

func TestExtractWorkflowName_NoName(t *testing.T) {
	got := extractWorkflowName("steps:\n  - run: echo hi")
	if got != "generated-workflow" {
		t.Errorf("expected default 'generated-workflow', got: %q", got)
	}
}

func TestExtractWorkflowName_Empty(t *testing.T) {
	got := extractWorkflowName("")
	if got != "generated-workflow" {
		t.Errorf("expected default 'generated-workflow', got: %q", got)
	}
}

func TestExtractWorkflowName_WithSpaces(t *testing.T) {
	got := extractWorkflowName("name:   spaced-name  \nsteps: []")
	if got != "spaced-name" {
		t.Errorf("expected 'spaced-name', got: %q", got)
	}
}

// --- buildWorkflowPrompt tests ---

func TestBuildWorkflowPrompt_ContainsUserPrompt(t *testing.T) {
	prompt := buildWorkflowPrompt("block edits to .env files")
	if !strings.Contains(prompt, "block edits to .env files") {
		t.Error("prompt should contain user's request")
	}
}

func TestBuildWorkflowPrompt_ContainsSchemaInfo(t *testing.T) {
	prompt := buildWorkflowPrompt("test")
	checks := []string{
		"hookflow workflow",
		"name:",
		"on:",
		"steps:",
		"${{",
		"file:",
		"commit:",
		"tool:",
	}
	for _, check := range checks {
		if !strings.Contains(prompt, check) {
			t.Errorf("prompt should contain %q", check)
		}
	}
}

func TestBuildWorkflowPrompt_ContainsTriggerExamples(t *testing.T) {
	prompt := buildWorkflowPrompt("test")
	if !strings.Contains(prompt, "paths:") {
		t.Error("prompt should contain trigger examples with paths")
	}
	if !strings.Contains(prompt, "types:") {
		t.Error("prompt should contain trigger examples with types")
	}
}

// --- GenerateWorkflowResult type tests ---

func TestGenerateWorkflowResult_Fields(t *testing.T) {
	r := GenerateWorkflowResult{
		Name:        "test-workflow",
		Description: "A test workflow",
		YAML:        "name: test-workflow\nsteps: []",
	}
	if r.Name != "test-workflow" {
		t.Errorf("expected name 'test-workflow', got: %q", r.Name)
	}
	if r.Description != "A test workflow" {
		t.Errorf("expected description 'A test workflow', got: %q", r.Description)
	}
	if r.YAML != "name: test-workflow\nsteps: []" {
		t.Errorf("unexpected YAML: %q", r.YAML)
	}
}

// --- Concurrency tests ---

func TestClient_ConcurrentStop(t *testing.T) {
	c := NewClient()
	done := make(chan error, 10)
	for i := 0; i < 10; i++ {
		go func() {
			done <- c.Stop()
		}()
	}
	for i := 0; i < 10; i++ {
		if err := <-done; err != nil {
			t.Errorf("concurrent Stop returned error: %v", err)
		}
	}
}

// --- Start tests (requires Copilot CLI, skipped if unavailable) ---

func TestStart_RequiresCopilotCLI(t *testing.T) {
	// Start requires the copilot CLI to be available.
	// This test validates error handling when it's not.
	c := NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := c.Start(ctx)
	// We expect an error since copilot CLI likely isn't available in test env
	if err == nil {
		// If it somehow worked, clean up
		_ = c.Stop()
		t.Skip("copilot CLI is available, skipping error path test")
	}
	// Verify the error is wrapped properly
	if !strings.Contains(err.Error(), "failed to start Copilot client") {
		t.Errorf("expected wrapped error, got: %v", err)
	}
}

// --- Start idempotency test ---

func TestStart_IdempotentWhenStarted(t *testing.T) {
	c := NewClient()
	// Simulate already-started state
	c.started = true
	err := c.Start(context.Background())
	if err != nil {
		t.Errorf("Start on already-started client should return nil, got: %v", err)
	}
}
