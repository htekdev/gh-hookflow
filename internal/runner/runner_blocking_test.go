package runner

import (
	"context"
	"os"
	"testing"

	"github.com/htekdev/gh-hookflow/internal/schema"
)

// TestRunWithBlockingAllowOnSuccess tests that a successful workflow returns allow
func TestRunWithBlockingAllowOnSuccess(t *testing.T) {
	workflow := &schema.Workflow{
		Name:     "test-allow",
		Blocking: ptrBool(true),
		Steps: []schema.Step{
			{
				Name: "success-step",
				Run:  "echo 'success'",
			},
		},
	}

	runner := NewRunner(workflow, nil, ".", "")
	ctx := context.Background()
	result := runner.RunWithBlocking(ctx)

	if result.PermissionDecision != "allow" {
		t.Errorf("Expected allow, got %s", result.PermissionDecision)
	}
}

// TestRunWithBlockingDenyOnFailure tests that a blocking workflow denies on failure
func TestRunWithBlockingDenyOnFailure(t *testing.T) {
	workflow := &schema.Workflow{
		Name:     "test-deny",
		Blocking: ptrBool(true),
		Steps: []schema.Step{
			{
				Name: "fail-step",
				Run:  "exit 1",
			},
		},
	}

	runner := NewRunner(workflow, nil, ".", "")
	ctx := context.Background()
	result := runner.RunWithBlocking(ctx)

	if result.PermissionDecision != "deny" {
		t.Errorf("Expected deny, got %s", result.PermissionDecision)
	}
	if result.PermissionDecisionReason == "" {
		t.Error("Expected reason for denial")
	}
}

// TestRunWithBlockingFalseAllowsOnFailure tests that non-blocking mode allows even with failures
func TestRunWithBlockingFalseAllowsOnFailure(t *testing.T) {
	workflow := &schema.Workflow{
		Name:     "test-non-blocking",
		Blocking: ptrBool(false),
		Steps: []schema.Step{
			{
				Name: "fail-step",
				Run:  "exit 1",
			},
		},
	}

	runner := NewRunner(workflow, nil, ".", "")
	ctx := context.Background()
	result := runner.RunWithBlocking(ctx)

	if result.PermissionDecision != "allow" {
		t.Errorf("Expected allow in non-blocking mode, got %s", result.PermissionDecision)
	}
}

// TestRunWithBlockingDefaultTrue tests that blocking defaults to true
func TestRunWithBlockingDefaultTrue(t *testing.T) {
	workflow := &schema.Workflow{
		Name: "test-default",
		// Blocking not specified, should default to true
		Steps: []schema.Step{
			{
				Name: "fail-step",
				Run:  "exit 1",
			},
		},
	}

	runner := NewRunner(workflow, nil, ".", "")
	ctx := context.Background()
	result := runner.RunWithBlocking(ctx)

	if result.PermissionDecision != "deny" {
		t.Errorf("Expected deny (default blocking=true), got %s", result.PermissionDecision)
	}
}

// TestRunWithBlockingMultipleStepFailures tests denial reason includes all failed steps
func TestRunWithBlockingMultipleStepFailures(t *testing.T) {
	workflow := &schema.Workflow{
		Name:     "test-multiple",
		Blocking: ptrBool(true),
		Steps: []schema.Step{
			{
				Name: "fail-step-1",
				Run:  "exit 1",
			},
			{
				Name:           "fail-step-2",
				Run:            "exit 1",
				ContinueOnError: true,
			},
		},
	}

	runner := NewRunner(workflow, nil, ".", "")
	ctx := context.Background()
	result := runner.RunWithBlocking(ctx)

	if result.PermissionDecision != "deny" {
		t.Errorf("Expected deny, got %s", result.PermissionDecision)
	}
	if !contains(result.PermissionDecisionReason, "fail-step-1") {
		t.Errorf("Expected reason to mention fail-step-1, got: %s", result.PermissionDecisionReason)
	}
}

// Helper function to create a bool pointer
func ptrBool(b bool) *bool {
	return &b
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	for i := 0; i < len(s)-len(substr)+1; i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestRunWithBlockingCreatesLogFile tests that denial creates a log file with step outputs
func TestRunWithBlockingCreatesLogFile(t *testing.T) {
	workflow := &schema.Workflow{
		Name:        "test-logs",
		Description: "Test workflow for log output",
		Blocking:    ptrBool(true),
		Steps: []schema.Step{
			{
				Name: "echo-step",
				Run:  "echo 'test output'",
			},
			{
				Name: "fail-step",
				Run:  "echo 'failure message' && exit 1",
			},
		},
	}

	runner := NewRunner(workflow, nil, ".", "")
	ctx := context.Background()
	result := runner.RunWithBlocking(ctx)

	if result.PermissionDecision != "deny" {
		t.Errorf("Expected deny, got %s", result.PermissionDecision)
	}

	// Check that log file was created
	if result.LogFile == "" {
		t.Error("Expected LogFile to be set")
	}

	// Check that reason mentions the log file
	if !contains(result.PermissionDecisionReason, result.LogFile) {
		t.Errorf("Expected reason to mention log file path, got: %s", result.PermissionDecisionReason)
	}

	// Verify log file exists and has content
	if result.LogFile != "" {
		content, err := os.ReadFile(result.LogFile)
		if err != nil {
			t.Errorf("Failed to read log file: %v", err)
		}
		logContent := string(content)

		// Should contain workflow name
		if !contains(logContent, "test-logs") {
			t.Error("Log should contain workflow name")
		}

		// Should contain step names
		if !contains(logContent, "echo-step") {
			t.Error("Log should contain echo-step")
		}
		if !contains(logContent, "fail-step") {
			t.Error("Log should contain fail-step")
		}

		// Should contain output
		if !contains(logContent, "test output") {
			t.Error("Log should contain step output")
		}

		// Clean up
		_ = os.Remove(result.LogFile)
	}
}
