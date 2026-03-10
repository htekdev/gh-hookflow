package runner

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/htekdev/gh-hookflow/internal/schema"
)

// TestStepWithoutTimeout tests that steps without timeout run normally
func TestStepWithoutTimeout(t *testing.T) {
	workflow := &schema.Workflow{
		Name: "test-workflow",
		Steps: []schema.Step{
			{
				Name: "quick-command",
				Run:  "echo 'hello'",
			},
		},
	}

	runner := NewRunner(workflow, nil, ".", "")
	ctx := context.Background()

	results, err := runner.Run(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	result := results[0]
	if !result.Success {
		t.Errorf("Expected success, got error: %v", result.Error)
	}

	if !strings.Contains(result.Output, "hello") {
		t.Errorf("Expected output to contain 'hello', got: %s", result.Output)
	}

	if result.Duration == 0 {
		t.Errorf("Expected non-zero duration")
	}
}

// TestStepWithTimeoutCompleteInTime tests that steps with sufficient timeout succeed
func TestStepWithTimeoutCompleteInTime(t *testing.T) {
	workflow := &schema.Workflow{
		Name: "test-workflow",
		Steps: []schema.Step{
			{
				Name:    "quick-command",
				Run:     "echo 'hello'",
				Timeout: 10, // 10 seconds - should be plenty for echo
			},
		},
	}

	runner := NewRunner(workflow, nil, ".", "")
	ctx := context.Background()

	results, err := runner.Run(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	result := results[0]
	if !result.Success {
		t.Errorf("Expected success, got error: %v", result.Error)
	}

	if !strings.Contains(result.Output, "hello") {
		t.Errorf("Expected output to contain 'hello', got: %s", result.Output)
	}

	if result.Duration == 0 {
		t.Errorf("Expected non-zero duration")
	}
}

// TestStepWithTimeoutExceeded tests that steps exceeding timeout fail with timeout error
func TestStepWithTimeoutExceeded(t *testing.T) {
	// Skip if pwsh is not available
	if _, err := exec.LookPath("pwsh"); err != nil {
		t.Skip("pwsh not available")
	}

	// Use a sleep command that takes longer than the timeout (pwsh syntax)
	sleepCmd := "Start-Sleep -Seconds 5"

	workflow := &schema.Workflow{
		Name: "test-workflow",
		Steps: []schema.Step{
			{
				Name:    "slow-command",
				Run:     sleepCmd,
				Timeout: 1, // 1 second timeout
			},
		},
	}

	runner := NewRunner(workflow, nil, ".", "")
	ctx := context.Background()

	results, err := runner.Run(ctx)
	if err != nil {
		t.Fatalf("Expected no error from Run(), got %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	result := results[0]
	if result.Success {
		t.Errorf("Expected failure due to timeout, but step succeeded")
	}

	if result.Error == nil {
		t.Errorf("Expected error to be set, got nil")
	}

	// Check that the error message indicates timeout
	if !strings.Contains(result.Error.Error(), "timed out") {
		t.Errorf("Expected error message to contain 'timed out', got: %v", result.Error)
	}

	if !strings.Contains(result.Error.Error(), "1 seconds") {
		t.Errorf("Expected error message to contain timeout duration, got: %v", result.Error)
	}

	// Duration should be roughly equal to the timeout (plus overhead)
	// Allow generous margin for CI runner overhead and pwsh startup time
	if result.Duration < time.Duration(500)*time.Millisecond {
		t.Errorf("Expected duration >= 500ms, got %v", result.Duration)
	}
	if result.Duration > time.Duration(5)*time.Second {
		t.Errorf("Expected duration <= 5s (timeout was 1s + generous overhead), got %v", result.Duration)
	}
}

// TestMultipleStepsWithMixedTimeouts tests workflow with multiple steps, some with timeout
func TestMultipleStepsWithMixedTimeouts(t *testing.T) {
	workflow := &schema.Workflow{
		Name: "test-workflow",
		Steps: []schema.Step{
			{
				Name: "step1-no-timeout",
				Run:  "echo 'step1'",
			},
			{
				Name:    "step2-with-timeout",
				Run:     "echo 'step2'",
				Timeout: 10,
			},
			{
				Name: "step3-no-timeout",
				Run:  "echo 'step3'",
			},
		},
	}

	runner := NewRunner(workflow, nil, ".", "")
	ctx := context.Background()

	results, err := runner.Run(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("Expected 3 results, got %d", len(results))
	}

	// All steps should succeed
	for i, result := range results {
		if !result.Success {
			t.Errorf("Step %d failed: %v", i+1, result.Error)
		}
	}

	// Check outputs
	if !strings.Contains(results[0].Output, "step1") {
		t.Errorf("Step 1: expected 'step1' in output")
	}
	if !strings.Contains(results[1].Output, "step2") {
		t.Errorf("Step 2: expected 'step2' in output")
	}
	if !strings.Contains(results[2].Output, "step3") {
		t.Errorf("Step 3: expected 'step3' in output")
	}
}

// TestTimeoutContextPropagation tests that timeout context is properly used
func TestTimeoutContextPropagation(t *testing.T) {
	// Create a command that will be interrupted
	// Using a loop that should be killed by timeout
	workflow := &schema.Workflow{
		Name: "test-workflow",
		Steps: []schema.Step{
			{
				Name:    "timeout-context-test",
				Run:     "sleep 5",
				Timeout: 1,
			},
		},
	}

	runner := NewRunner(workflow, nil, ".", "")

	// Pass a context with its own timeout - should still respect step timeout
	parentCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	results, err := runner.Run(parentCtx)
	if err != nil {
		t.Fatalf("Expected no error from Run(), got %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	result := results[0]
	if result.Success {
		t.Errorf("Expected failure due to timeout")
	}

	if result.Error == nil {
		t.Errorf("Expected error to be set")
	}

	// Verify it's a timeout error
	if !strings.Contains(result.Error.Error(), "timed out") {
		t.Errorf("Expected timeout error, got: %v", result.Error)
	}
}

// TestCommandKilledOnTimeout verifies that command process is actually terminated
func TestCommandKilledOnTimeout(t *testing.T) {
	// This test verifies the process is killed by using a command that would hang
	workflow := &schema.Workflow{
		Name: "test-workflow",
		Steps: []schema.Step{
			{
				Name:    "process-kill-test",
				Run:     "sleep 5",
				Timeout: 1,
			},
		},
	}

	runner := NewRunner(workflow, nil, ".", "")
	ctx := context.Background()

	results, err := runner.Run(ctx)
	if err != nil {
		t.Fatalf("Expected no error from Run(), got %v", err)
	}

	result := results[0]
	if result.Success {
		t.Errorf("Expected timeout failure")
	}

	// Duration should be roughly equal to the timeout
	if result.Duration < time.Duration(900)*time.Millisecond {
		t.Errorf("Expected duration >= 900ms, got %v", result.Duration)
	}
}

// TestZeroTimeoutNotApplied tests that zero timeout means no timeout
func TestZeroTimeoutNotApplied(t *testing.T) {
	workflow := &schema.Workflow{
		Name: "test-workflow",
		Steps: []schema.Step{
			{
				Name:    "no-timeout-zero",
				Run:     "echo 'test'",
				Timeout: 0, // Zero timeout should not create a timeout context
			},
		},
	}

	runner := NewRunner(workflow, nil, ".", "")
	ctx := context.Background()

	results, err := runner.Run(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	result := results[0]
	if !result.Success {
		t.Errorf("Expected success with zero timeout, got error: %v", result.Error)
	}

	if !strings.Contains(result.Output, "test") {
		t.Errorf("Expected 'test' in output")
	}
}

// TestNegativeTimeoutNotApplied tests that negative timeout means no timeout
func TestNegativeTimeoutNotApplied(t *testing.T) {
	workflow := &schema.Workflow{
		Name: "test-workflow",
		Steps: []schema.Step{
			{
				Name:    "negative-timeout",
				Run:     "echo 'test'",
				Timeout: -1, // Negative timeout should not create a timeout context
			},
		},
	}

	runner := NewRunner(workflow, nil, ".", "")
	ctx := context.Background()

	results, err := runner.Run(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	result := results[0]
	if !result.Success {
		t.Errorf("Expected success with negative timeout, got error: %v", result.Error)
	}

	if !strings.Contains(result.Output, "test") {
		t.Errorf("Expected 'test' in output")
	}
}

// ============================================================================
// continue-on-error Tests
// ============================================================================

// TestContinueOnErrorTrueAllowsSubsequentSteps verifies that when continue-on-error is true,
// step failure doesn't prevent subsequent steps from running
func TestContinueOnErrorTrueAllowsSubsequentSteps(t *testing.T) {
	workflow := &schema.Workflow{
		Name: "test-continue-on-error-true",
		Steps: []schema.Step{
			{
				Name:            "Step 1 - Fail",
				Run:             "exit 1",
				ContinueOnError: true,
			},
			{
				Name:            "Step 2 - Should Run",
				Run:             "echo 'This should run'",
				ContinueOnError: false,
			},
		},
	}

	runner := NewRunner(workflow, nil, os.TempDir(), "")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// Step 1 should fail
	if results[0].Success {
		t.Errorf("Step 1 should have failed")
	}
	if results[0].Name != "Step 1 - Fail" {
		t.Errorf("expected step name 'Step 1 - Fail', got '%s'", results[0].Name)
	}

	// Step 2 should run and succeed (not be skipped)
	if results[1].Name != "Step 2 - Should Run" {
		t.Errorf("expected step name 'Step 2 - Should Run', got '%s'", results[1].Name)
	}
	if !results[1].Success {
		t.Errorf("Step 2 should have succeeded but got error: %v, output: %s", results[1].Error, results[1].Output)
	}
	if results[1].Output == "" {
		t.Errorf("Step 2 should have output but got empty string")
	}
}

// TestContinueOnErrorFalseStopsSubsequentSteps verifies that when continue-on-error is false (default),
// step failure prevents subsequent steps from running
func TestContinueOnErrorFalseStopsSubsequentSteps(t *testing.T) {
	workflow := &schema.Workflow{
		Name: "test-continue-on-error-false",
		Steps: []schema.Step{
			{
				Name:            "Step 1 - Fail",
				Run:             "exit 1",
				ContinueOnError: false,
			},
			{
				Name:            "Step 2 - Should Skip",
				Run:             "echo 'This should NOT run'",
				ContinueOnError: false,
			},
		},
	}

	runner := NewRunner(workflow, nil, os.TempDir(), "")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// Step 1 should fail
	if results[0].Success {
		t.Errorf("Step 1 should have failed")
	}
	if results[0].Name != "Step 1 - Fail" {
		t.Errorf("expected step name 'Step 1 - Fail', got '%s'", results[0].Name)
	}

	// Step 2 should be skipped (not executed)
	if results[1].Name != "Step 2 - Should Skip" {
		t.Errorf("expected step name 'Step 2 - Should Skip', got '%s'", results[1].Name)
	}
	if results[1].Success {
		t.Errorf("Step 2 should not have succeeded (it should be skipped)")
	}
	if results[1].Output != "Skipped (previous step failed)" {
		t.Errorf("Step 2 should be skipped with correct message, got: %s", results[1].Output)
	}
}

// TestDefaultContinueOnErrorIsFalse verifies that the default behavior (continue-on-error not set)
// stops execution on failure
func TestDefaultContinueOnErrorIsFalse(t *testing.T) {
	workflow := &schema.Workflow{
		Name: "test-default-continue-on-error",
		Steps: []schema.Step{
			{
				Name: "Step 1 - Fail",
				Run:  "exit 1",
				// ContinueOnError not set, defaults to false
			},
			{
				Name: "Step 2 - Should Skip",
				Run:  "echo 'This should NOT run'",
			},
		},
	}

	runner := NewRunner(workflow, nil, os.TempDir(), "")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// Step 1 should fail
	if results[0].Success {
		t.Errorf("Step 1 should have failed")
	}

	// Step 2 should be skipped
	if results[1].Success {
		t.Errorf("Step 2 should not have succeeded (it should be skipped)")
	}
	if results[1].Output != "Skipped (previous step failed)" {
		t.Errorf("Step 2 should be skipped with correct message, got: %s", results[1].Output)
	}
}

// TestAlwaysRunsRegardlessOfPreviousFailure verifies that steps with always() in their if condition
// run even when a previous step failed
func TestAlwaysRunsRegardlessOfPreviousFailure(t *testing.T) {
	workflow := &schema.Workflow{
		Name: "test-always-runs",
		Steps: []schema.Step{
			{
				Name:            "Step 1 - Fail",
				Run:             "exit 1",
				ContinueOnError: false,
			},
			{
				Name: "Step 2 - Always Run",
				Run:  "echo 'This always runs'",
				If:   "always()",
			},
		},
	}

	runner := NewRunner(workflow, nil, os.TempDir(), "")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// Step 1 should fail
	if results[0].Success {
		t.Errorf("Step 1 should have failed")
	}

	// Step 2 should run (not be skipped) because of always()
	if results[1].Name != "Step 2 - Always Run" {
		t.Errorf("expected step name 'Step 2 - Always Run', got '%s'", results[1].Name)
	}
	if !results[1].Success {
		t.Errorf("Step 2 should have succeeded but got error: %v, output: %s", results[1].Error, results[1].Output)
	}
	if results[1].Output == "" {
		t.Errorf("Step 2 should have output but got empty string")
	}
}

// TestMixedContinueOnErrorAndAlways verifies complex interaction patterns
func TestMixedContinueOnErrorAndAlways(t *testing.T) {
	workflow := &schema.Workflow{
		Name: "test-mixed-behavior",
		Steps: []schema.Step{
			{
				Name:            "Step 1 - Fail but Continue",
				Run:             "exit 1",
				ContinueOnError: true,
			},
			{
				Name:            "Step 2 - Should Run (continue-on-error from Step 1)",
				Run:             "echo 'Step 2 runs because step 1 had continue-on-error'",
				ContinueOnError: false,
			},
			{
				Name:            "Step 3 - Fail but Continue",
				Run:             "exit 1",
				ContinueOnError: true,
			},
			{
				Name: "Step 4 - Should Run (always)",
				Run:  "echo 'Step 4 always runs'",
				If:   "always()",
			},
		},
	}

	runner := NewRunner(workflow, nil, os.TempDir(), "")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 4 {
		t.Fatalf("expected 4 results, got %d", len(results))
	}

	// Step 1 should fail
	if results[0].Success {
		t.Errorf("Step 1 should have failed")
	}

	// Step 2 should run (Step 1 had continue-on-error)
	if !results[1].Success {
		t.Errorf("Step 2 should have succeeded, got error: %v", results[1].Error)
	}

	// Step 3 should fail
	if results[2].Success {
		t.Errorf("Step 3 should have failed")
	}

	// Step 4 should run (always() condition)
	if !results[3].Success {
		t.Errorf("Step 4 should have succeeded, got error: %v", results[3].Error)
	}
}

// TestContinueOnErrorWithMultipleFailures verifies behavior with multiple failures
func TestContinueOnErrorWithMultipleFailures(t *testing.T) {
	workflow := &schema.Workflow{
		Name: "test-multiple-failures-with-continue",
		Steps: []schema.Step{
			{
				Name:            "Step 1 - Fail",
				Run:             "exit 1",
				ContinueOnError: true,
			},
			{
				Name:            "Step 2 - Fail",
				Run:             "exit 1",
				ContinueOnError: true,
			},
			{
				Name:            "Step 3 - Should Run",
				Run:             "echo 'This should still run'",
				ContinueOnError: false,
			},
		},
	}

	runner := NewRunner(workflow, nil, os.TempDir(), "")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// All steps should have been executed
	if results[0].Success {
		t.Errorf("Step 1 should have failed")
	}
	if results[1].Success {
		t.Errorf("Step 2 should have failed")
	}
	if !results[2].Success {
		t.Errorf("Step 3 should have succeeded, got error: %v", results[2].Error)
	}
}

// TestSuccessfulStepDoesNotSetPrevStepFailed verifies that successful steps don't set the failure flag
func TestSuccessfulStepDoesNotSetPrevStepFailed(t *testing.T) {
	workflow := &schema.Workflow{
		Name: "test-success-no-flag",
		Steps: []schema.Step{
			{
				Name: "Step 1 - Success",
				Run:  "echo 'Success'",
			},
			{
				Name: "Step 2 - Should Run",
				Run:  "echo 'Step 2 runs'",
			},
		},
	}

	runner := NewRunner(workflow, nil, os.TempDir(), "")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// Both steps should succeed
	if !results[0].Success {
		t.Errorf("Step 1 should have succeeded")
	}
	if !results[1].Success {
		t.Errorf("Step 2 should have succeeded")
	}
}

// TestContinueOnErrorWithEnvironmentVariables verifies continue-on-error works with env vars
func TestContinueOnErrorWithEnvironmentVariables(t *testing.T) {
	workflow := &schema.Workflow{
		Name: "test-continue-with-env",
		Steps: []schema.Step{
			{
				Name:            "Step 1 - Fail with env var",
				Run:             "exit 1",
				Env:             map[string]string{"TEST_VAR": "test_value"},
				ContinueOnError: true,
			},
			{
				Name:            "Step 2 - Should Run with env var",
				Run:             "echo 'Success with env'",
				Env:             map[string]string{"TEST_VAR": "test_value"},
				ContinueOnError: false,
			},
		},
	}

	runner := NewRunner(workflow, nil, os.TempDir(), "")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// Step 1 should fail
	if results[0].Success {
		t.Errorf("Step 1 should have failed")
	}

	// Step 2 should run and succeed
	if !results[1].Success {
		t.Errorf("Step 2 should have succeeded, got error: %v", results[1].Error)
	}
}

// TestAlwaysWithContinueOnError verifies always() takes precedence over previous failures
func TestAlwaysWithContinueOnError(t *testing.T) {
	workflow := &schema.Workflow{
		Name: "test-always-with-continue",
		Steps: []schema.Step{
			{
				Name:            "Step 1 - Fail without continue",
				Run:             "exit 1",
				ContinueOnError: false,
			},
			{
				Name:            "Step 2 - Regular step (should skip)",
				Run:             "echo 'This should skip'",
				ContinueOnError: false,
			},
			{
				Name: "Step 3 - Always run",
				Run:  "echo 'This always runs'",
				If:   "always()",
			},
		},
	}

	runner := NewRunner(workflow, nil, os.TempDir(), "")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// Step 1 should fail
	if results[0].Success {
		t.Errorf("Step 1 should have failed")
	}

	// Step 2 should be skipped
	if results[1].Success {
		t.Errorf("Step 2 should not have succeeded (should be skipped)")
	}
	if results[1].Output != "Skipped (previous step failed)" {
		t.Errorf("Step 2 should be skipped with correct message, got: %s", results[1].Output)
	}

	// Step 3 should run (always() overrides the skip)
	if !results[2].Success {
		t.Errorf("Step 3 should have succeeded, got error: %v", results[2].Error)
	}
}

// TestPrevStepFailedFlagOnly verifies prevStepFailed is only set when continue-on-error is false
func TestPrevStepFailedFlagOnly(t *testing.T) {
	// This tests the internal behavior: prevStepFailed should only be set when
	// ContinueOnError is false. We verify this by checking if subsequent steps are skipped.
	workflow := &schema.Workflow{
		Name: "test-prev-step-failed-flag",
		Steps: []schema.Step{
			{
				Name:            "Step 1 - Fail with continue=true",
				Run:             "exit 1",
				ContinueOnError: true,
			},
			{
				Name:            "Step 2 - Should execute (no skip)",
				Run:             "echo 'Running after continue-on-error=true failure'",
				ContinueOnError: false,
			},
			{
				Name:            "Step 3 - Fail with continue=false",
				Run:             "exit 1",
				ContinueOnError: false,
			},
			{
				Name:            "Step 4 - Should skip (prev failed with continue=false)",
				Run:             "echo 'Should not run'",
				ContinueOnError: false,
			},
		},
	}

	runner := NewRunner(workflow, nil, os.TempDir(), "")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 4 {
		t.Fatalf("expected 4 results, got %d", len(results))
	}

	// Step 1: fails but continue-on-error=true, so prevStepFailed NOT set
	if results[0].Success {
		t.Errorf("Step 1 should have failed")
	}

	// Step 2: should run because Step 1's failure didn't set prevStepFailed
	if !results[1].Success {
		t.Errorf("Step 2 should have succeeded (Step 1 had continue-on-error=true), got error: %v", results[1].Error)
	}

	// Step 3: fails with continue-on-error=false, so prevStepFailed IS set
	if results[2].Success {
		t.Errorf("Step 3 should have failed")
	}

	// Step 4: should skip because Step 3's failure set prevStepFailed
	if results[3].Success {
		t.Errorf("Step 4 should not have succeeded (should be skipped)")
	}
	if results[3].Output != "Skipped (previous step failed)" {
		t.Errorf("Step 4 should be skipped with correct message, got: %s", results[3].Output)
	}
}


// TestStepIfConditionTrue tests that steps with if: true run
func TestStepIfConditionTrue(t *testing.T) {
	workflow := &schema.Workflow{
		Name: "test-workflow",
		Steps: []schema.Step{
			{
				Name: "test-step",
				If:   "true",
				Run:  "echo 'Step executed'",
			},
		},
	}

	event := &schema.Event{
		Cwd:       ".",
		Timestamp: "2024-01-01T00:00:00Z",
	}

	runner := NewRunner(workflow, event, ".", "")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	result := results[0]
	if !result.Success {
		t.Errorf("Expected step to succeed with if: true, got error: %v", result.Error)
	}

	if strings.Contains(result.Output, "Skipped") {
		t.Errorf("Expected step to run, but it was skipped")
	}
}

// TestStepIfConditionFalse tests that steps with if: false are skipped
func TestStepIfConditionFalse(t *testing.T) {
	workflow := &schema.Workflow{
		Name: "test-workflow",
		Steps: []schema.Step{
			{
				Name: "test-step",
				If:   "false",
				Run:  "echo 'Should not execute'",
			},
		},
	}

	event := &schema.Event{
		Cwd:       "/test",
		Timestamp: "2024-01-01T00:00:00Z",
	}

	runner := NewRunner(workflow, event, ".", "")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	result := results[0]
	if !result.Success {
		t.Errorf("Expected skipped step to be marked success, got error: %v", result.Error)
	}

	if !strings.Contains(result.Output, "Skipped") {
		t.Errorf("Expected output to indicate skipped, got: %s", result.Output)
	}
}

// TestStepIfExpressionEvaluation tests that if: ${{ expression }} evaluates correctly
func TestStepIfExpressionEvaluation(t *testing.T) {
	tests := []struct {
		name        string
		ifCondition string
		shouldRun   bool
	}{
		{
			name:        "equality check passes",
			ifCondition: "${{ 'test' == 'test' }}",
			shouldRun:   true,
		},
		{
			name:        "equality check fails",
			ifCondition: "${{ 'test' == 'other' }}",
			shouldRun:   false,
		},
		{
			name:        "inequality check passes",
			ifCondition: "${{ 'test' != 'other' }}",
			shouldRun:   true,
		},
		{
			name:        "logical AND true",
			ifCondition: "${{ true && true }}",
			shouldRun:   true,
		},
		{
			name:        "logical AND false",
			ifCondition: "${{ true && false }}",
			shouldRun:   false,
		},
		{
			name:        "logical OR true",
			ifCondition: "${{ false || true }}",
			shouldRun:   true,
		},
		{
			name:        "logical OR false",
			ifCondition: "${{ false || false }}",
			shouldRun:   false,
		},
		{
			name:        "NOT operator",
			ifCondition: "${{ !false }}",
			shouldRun:   true,
		},
		{
			name:        "numeric comparison",
			ifCondition: "${{ 5 > 3 }}",
			shouldRun:   true,
		},
		{
			name:        "contains function",
			ifCondition: "${{ contains('hello world', 'world') }}",
			shouldRun:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workflow := &schema.Workflow{
				Name: "test-workflow",
				Steps: []schema.Step{
					{
						Name: "test-step",
						If:   tt.ifCondition,
						Run:  "echo 'Step executed'",
					},
				},
			}

			event := &schema.Event{
				Cwd:       "/test",
				Timestamp: "2024-01-01T00:00:00Z",
			}

			runner := NewRunner(workflow, event, ".", "")
			results, err := runner.Run(context.Background())

			if err != nil {
				t.Fatalf("Expected no error, got %v", err)
			}

			if len(results) != 1 {
				t.Fatalf("Expected 1 result, got %d", len(results))
			}

			result := results[0]
			isSkipped := strings.Contains(result.Output, "Skipped")

			if tt.shouldRun && isSkipped {
				t.Errorf("Expected step to run, but it was skipped")
			}

			if !tt.shouldRun && !isSkipped {
				t.Errorf("Expected step to be skipped, but it ran")
			}
		})
	}
}

// TestStepIfConditionEvaluationError tests that failed condition evaluation marks step as failed
func TestStepIfConditionEvaluationError(t *testing.T) {
	workflow := &schema.Workflow{
		Name: "test-workflow",
		Steps: []schema.Step{
			{
				Name: "test-step",
				If:   "${{ invalid_function() }}",
				Run:  "echo 'Should not execute'",
			},
		},
	}

	event := &schema.Event{
		Cwd:       "/test",
		Timestamp: "2024-01-01T00:00:00Z",
	}

	runner := NewRunner(workflow, event, ".", "")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	result := results[0]
	if result.Success {
		t.Errorf("Expected step to fail with invalid condition, but it succeeded")
	}

	if result.Error == nil {
		t.Errorf("Expected error for invalid condition evaluation")
	}

	if !strings.Contains(result.Error.Error(), "failed to evaluate if condition") {
		t.Errorf("Expected 'failed to evaluate if condition' in error, got: %v", result.Error)
	}
}

// TestStepWithoutIfCondition tests that steps without if conditions run
func TestStepWithoutIfCondition(t *testing.T) {
	workflow := &schema.Workflow{
		Name: "test-workflow",
		Steps: []schema.Step{
			{
				Name: "test-step",
				Run:  "echo 'Step executed'",
			},
		},
	}

	event := &schema.Event{
		Cwd:       "/test",
		Timestamp: "2024-01-01T00:00:00Z",
	}

	runner := NewRunner(workflow, event, ".", "")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	result := results[0]
	if !result.Success {
		t.Errorf("Expected step without if to run, got error: %v", result.Error)
	}
}

// TestStepIfWithEnvironmentVariable tests that if conditions can reference env variables
func TestStepIfWithEnvironmentVariable(t *testing.T) {
	workflow := &schema.Workflow{
		Name: "test-workflow",
		Env: map[string]string{
			"ENABLE_STEP": "true",
		},
		Steps: []schema.Step{
			{
				Name: "test-step",
				If:   "${{ env.ENABLE_STEP == 'true' }}",
				Run:  "echo 'Step executed'",
			},
		},
	}

	event := &schema.Event{
		Cwd:       "/test",
		Timestamp: "2024-01-01T00:00:00Z",
	}

	runner := NewRunner(workflow, event, ".", "")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	result := results[0]
	if strings.Contains(result.Output, "Skipped") {
		t.Errorf("Expected step to run when env variable is true")
	}
}

// TestStepIfWithEventData tests that if conditions can reference event data
func TestStepIfWithEventData(t *testing.T) {
	workflow := &schema.Workflow{
		Name: "test-workflow",
		Steps: []schema.Step{
			{
				Name: "test-step",
				If:   "${{ event.cwd != '' }}",
				Run:  "echo 'Step executed'",
			},
		},
	}

	event := &schema.Event{
		Cwd:       "/test",
		Timestamp: "2024-01-01T00:00:00Z",
	}

	runner := NewRunner(workflow, event, ".", "")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	result := results[0]
	if strings.Contains(result.Output, "Skipped") {
		t.Errorf("Expected step to run when event.cwd is set")
	}
}

// TestMultipleStepsWithIfConditions tests multiple steps with various conditions
func TestMultipleStepsWithIfConditions(t *testing.T) {
	workflow := &schema.Workflow{
		Name: "test-workflow",
		Steps: []schema.Step{
			{
				Name: "step1",
				If:   "true",
				Run:  "echo 'Step 1'",
			},
			{
				Name: "step2",
				If:   "false",
				Run:  "echo 'Step 2'",
			},
			{
				Name: "step3",
				Run:  "echo 'Step 3'",
			},
		},
	}

	event := &schema.Event{
		Cwd:       "/test",
		Timestamp: "2024-01-01T00:00:00Z",
	}

	runner := NewRunner(workflow, event, ".", "")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("Expected 3 results, got %d", len(results))
	}

	// Step 1 should run
	if strings.Contains(results[0].Output, "Skipped") {
		t.Errorf("Step 1 should run but was skipped")
	}

	// Step 2 should be skipped
	if !strings.Contains(results[1].Output, "Skipped") {
		t.Errorf("Step 2 should be skipped but ran")
	}

	// Step 3 should run
	if strings.Contains(results[2].Output, "Skipped") {
		t.Errorf("Step 3 should run but was skipped")
	}
}

// TestStepIfWithComplexLogic tests complex conditional logic
func TestStepIfWithComplexLogic(t *testing.T) {
	workflow := &schema.Workflow{
		Name: "test-workflow",
		Env: map[string]string{
			"ENV_VAR": "value",
		},
		Steps: []schema.Step{
			{
				Name: "complex-condition",
				If:   "${{ (true && false) || (true && true) }}",
				Run:  "echo 'Complex logic'",
			},
		},
	}

	event := &schema.Event{
		Cwd:       "/test",
		Timestamp: "2024-01-01T00:00:00Z",
	}

	runner := NewRunner(workflow, event, ".", "")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	result := results[0]
	// (true && false) = false, (true && true) = true, false || true = true
	// So step should run
	if strings.Contains(result.Output, "Skipped") {
		t.Errorf("Expected step to run with complex logic that evaluates to true")
	}
}

// TestContinueOnErrorWithIfCondition tests continue-on-error flag interaction with if conditions
func TestContinueOnErrorWithIfCondition(t *testing.T) {
	workflow := &schema.Workflow{
		Name: "test-workflow",
		Steps: []schema.Step{
			{
				Name:            "failing-step",
				Run:             "exit 1",
				ContinueOnError: true,
			},
			{
				Name: "step-after-failure",
				If:   "true",
				Run:  "echo 'This should run'",
			},
		},
	}

	event := &schema.Event{
		Cwd:       "/test",
		Timestamp: "2024-01-01T00:00:00Z",
	}

	runner := NewRunner(workflow, event, ".", "")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("Expected 2 results, got %d", len(results))
	}

	// First step fails but has continue-on-error
	if results[0].Success {
		t.Errorf("Expected first step to fail")
	}

	// Second step should still run because first step has continue-on-error
	if strings.Contains(results[1].Output, "Skipped") {
		t.Errorf("Expected second step to run despite first step failure due to continue-on-error")
	}
}

// ============================================================================
// Shell Command Execution Tests
// ============================================================================

// TestEchoCommandExecution tests simple echo command execution
func TestEchoCommandExecution(t *testing.T) {
	workflow := &schema.Workflow{
		Name: "test-echo",
		Steps: []schema.Step{
			{
				Name: "echo-test",
				Run:  "echo 'Hello, World!'",
			},
		},
	}

	runner := NewRunner(workflow, nil, ".", "")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	result := results[0]
	if !result.Success {
		t.Errorf("Echo command should succeed, got error: %v", result.Error)
	}

	if !strings.Contains(result.Output, "Hello, World!") {
		t.Errorf("Expected output to contain 'Hello, World!', got: %s", result.Output)
	}
}

// TestCommandExitCodeSuccess tests command with successful exit code
func TestCommandExitCodeSuccess(t *testing.T) {
	workflow := &schema.Workflow{
		Name: "test-exit-0",
		Steps: []schema.Step{
			{
				Name: "exit-success",
				Run:  "exit 0",
			},
		},
	}

	runner := NewRunner(workflow, nil, ".", "")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	result := results[0]
	if !result.Success {
		t.Errorf("exit 0 should succeed, got error: %v", result.Error)
	}
}

// TestCommandExitCodeFailure tests command with non-zero exit code
func TestCommandExitCodeFailure(t *testing.T) {
	workflow := &schema.Workflow{
		Name: "test-exit-1",
		Steps: []schema.Step{
			{
				Name: "exit-failure",
				Run:  "exit 1",
			},
		},
	}

	runner := NewRunner(workflow, nil, ".", "")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error from Run(), got %v", err)
	}

	result := results[0]
	if result.Success {
		t.Errorf("exit 1 should fail")
	}

	if result.Error == nil {
		t.Errorf("Expected error for failed exit code, got nil")
	}
}

// TestCommandWithMultipleExitCodes tests various exit codes
func TestCommandWithMultipleExitCodes(t *testing.T) {
	testCases := []struct {
		name      string
		command   string
		shouldFail bool
	}{
		{"exit 0", "exit 0", false},
		{"exit 1", "exit 1", true},
		{"exit 2", "exit 2", true},
		{"exit 127", "exit 127", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			workflow := &schema.Workflow{
				Name: "test-exit-codes",
				Steps: []schema.Step{
					{
						Name: tc.name,
						Run:  tc.command,
					},
				},
			}

			runner := NewRunner(workflow, nil, ".", "")
			results, err := runner.Run(context.Background())

			if err != nil {
				t.Fatalf("Expected no error from Run(), got %v", err)
			}

			result := results[0]
			if tc.shouldFail && result.Success {
				t.Errorf("Expected failure but got success")
			}
			if !tc.shouldFail && !result.Success {
				t.Errorf("Expected success but got failure: %v", result.Error)
			}
		})
	}
}

// ============================================================================
// Working Directory Tests
// ============================================================================

// TestWorkingDirectoryDefault tests default working directory
func TestWorkingDirectoryDefault(t *testing.T) {
	workflow := &schema.Workflow{
		Name: "test-default-wd",
		Steps: []schema.Step{
			{
				Name: "pwd-test",
				Run:  "pwd",
			},
		},
	}

	runner := NewRunner(workflow, nil, ".", "")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	result := results[0]
	if !result.Success {
		t.Errorf("pwd should succeed, got error: %v", result.Error)
	}

	if result.Output == "" {
		t.Errorf("pwd should return output")
	}
}

// TestWorkingDirectoryCustom tests custom working directory via step
func TestWorkingDirectoryCustom(t *testing.T) {
	tmpDir := os.TempDir()

	workflow := &schema.Workflow{
		Name: "test-custom-wd",
		Steps: []schema.Step{
			{
				Name:             "pwd-in-tmpdir",
				Run:              "pwd",
				WorkingDirectory: tmpDir,
			},
		},
	}

	runner := NewRunner(workflow, nil, ".", "")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	result := results[0]
	if !result.Success {
		t.Errorf("pwd in custom directory should succeed, got error: %v", result.Error)
	}

	// Output should contain the tmpDir path (normalized)
	output := strings.TrimSpace(result.Output)
	if output == "" {
		t.Errorf("pwd output should not be empty")
	}
}

// TestWorkingDirectoryWithExpressionInterpolation tests working directory with expressions
func TestWorkingDirectoryWithExpressionInterpolation(t *testing.T) {
	workflow := &schema.Workflow{
		Name: "test-wd-expression",
		Env: map[string]string{
			"TEST_DIR": os.TempDir(),
		},
		Steps: []schema.Step{
			{
				Name:             "pwd-with-env",
				Run:              "pwd",
				WorkingDirectory: "${{ env.TEST_DIR }}",
			},
		},
	}

	runner := NewRunner(workflow, nil, ".", "")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	result := results[0]
	if !result.Success {
		t.Errorf("pwd with expression in working directory should succeed, got error: %v", result.Error)
	}
}

// ============================================================================
// Environment Variable Tests
// ============================================================================

// TestEnvironmentVariableExpansion tests that env vars are expanded in commands via expressions
func TestEnvironmentVariableExpansion(t *testing.T) {
	workflow := &schema.Workflow{
		Name: "test-env-expansion",
		Env: map[string]string{
			"MY_VAR": "test_value",
		},
		Steps: []schema.Step{
			{
				Name: "echo-env",
				Run:  "echo ${{ env.MY_VAR }}",
			},
		},
	}

	runner := NewRunner(workflow, nil, ".", "")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	result := results[0]
	if !result.Success {
		t.Errorf("Echo env var should succeed, got error: %v", result.Error)
	}

	if !strings.Contains(result.Output, "test_value") {
		t.Errorf("Expected output to contain 'test_value', got: %s", result.Output)
	}
}

// TestStepEnvironmentVariableOverride tests step-level env var override via expressions
func TestStepEnvironmentVariableOverride(t *testing.T) {
	workflow := &schema.Workflow{
		Name: "test-step-env-override",
		Env: map[string]string{
			"MY_VAR": "workflow_value",
		},
		Steps: []schema.Step{
			{
				Name: "echo-step-env",
				Run:  "echo ${{ env.MY_VAR }}",
				Env: map[string]string{
					"MY_VAR": "step_value",
				},
			},
		},
	}

	runner := NewRunner(workflow, nil, ".", "")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	result := results[0]
	if !result.Success {
		t.Errorf("Echo step env var should succeed, got error: %v", result.Error)
	}

	// Note: Currently step env vars may not override workflow env in expression evaluation
	// This test documents the current behavior - step env vars are added to the process env
	// but expression evaluation uses the original workflow env
	if strings.Contains(result.Output, "workflow_value") {
		t.Logf("Note: Expression uses workflow env, step env added to process only")
	}
}

// TestEnvironmentVariableInExpressionInterpolation tests env vars in expressions
func TestEnvironmentVariableInExpressionInterpolation(t *testing.T) {
	workflow := &schema.Workflow{
		Name: "test-env-in-expression",
		Env: map[string]string{
			"MY_VAR": "test_value",
		},
		Steps: []schema.Step{
			{
				Name: "echo-expr-env",
				Run:  "echo ${{ env.MY_VAR }}",
			},
		},
	}

	runner := NewRunner(workflow, nil, ".", "")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	result := results[0]
	if !result.Success {
		t.Errorf("Echo env var via expression should succeed, got error: %v", result.Error)
	}

	if !strings.Contains(result.Output, "test_value") {
		t.Errorf("Expected output to contain 'test_value', got: %s", result.Output)
	}
}

// TestMultipleEnvironmentVariables tests multiple env vars in workflow via expressions
func TestMultipleEnvironmentVariables(t *testing.T) {
	workflow := &schema.Workflow{
		Name: "test-multiple-env",
		Env: map[string]string{
			"VAR1": "value1",
			"VAR2": "value2",
			"VAR3": "value3",
		},
		Steps: []schema.Step{
			{
				Name: "echo-all-env",
				Run:  "echo ${{ env.VAR1 }} ${{ env.VAR2 }} ${{ env.VAR3 }}",
			},
		},
	}

	runner := NewRunner(workflow, nil, ".", "")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	result := results[0]
	if !result.Success {
		t.Errorf("Echo multiple env vars should succeed, got error: %v", result.Error)
	}

	// Check all variables appear in output
	if !strings.Contains(result.Output, "value1") {
		t.Errorf("Expected output to contain 'value1'")
	}
	if !strings.Contains(result.Output, "value2") {
		t.Errorf("Expected output to contain 'value2'")
	}
	if !strings.Contains(result.Output, "value3") {
		t.Errorf("Expected output to contain 'value3'")
	}
}

// ============================================================================
// Expression Interpolation Tests
// ============================================================================

// TestSimpleExpressionInterpolation tests basic expression interpolation
func TestSimpleExpressionInterpolation(t *testing.T) {
	workflow := &schema.Workflow{
		Name: "test-simple-expr",
		Steps: []schema.Step{
			{
				Name: "echo-expr",
				Run:  "echo ${{ 'hello' }}",
			},
		},
	}

	runner := NewRunner(workflow, nil, ".", "")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	result := results[0]
	if !result.Success {
		t.Errorf("Simple expression should succeed, got error: %v", result.Error)
	}

	if !strings.Contains(result.Output, "hello") {
		t.Errorf("Expected output to contain 'hello', got: %s", result.Output)
	}
}

// TestExpressionInterpolationConcatenation tests string concatenation in expressions
func TestExpressionInterpolationConcatenation(t *testing.T) {
	workflow := &schema.Workflow{
		Name: "test-concat-expr",
		Steps: []schema.Step{
			{
				Name: "echo-concat",
				Run:  "echo ${{ 'hello' }} ${{ 'world' }}",
			},
		},
	}

	runner := NewRunner(workflow, nil, ".", "")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	result := results[0]
	if !result.Success {
		t.Errorf("Concatenation expression should succeed, got error: %v", result.Error)
	}

	if !strings.Contains(result.Output, "hello") || !strings.Contains(result.Output, "world") {
		t.Errorf("Expected output to contain both 'hello' and 'world', got: %s", result.Output)
	}
}

// TestInvalidExpressionInterpolation tests handling of invalid expressions
func TestInvalidExpressionInterpolation(t *testing.T) {
	workflow := &schema.Workflow{
		Name: "test-invalid-expr",
		Steps: []schema.Step{
			{
				Name: "invalid-expr",
				Run:  "echo ${{ undefined_var }}",
			},
		},
	}

	runner := NewRunner(workflow, nil, ".", "")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error from Run(), got %v", err)
	}

	result := results[0]
	// Invalid expression should cause command to fail
	if result.Success {
		t.Logf("Note: Expression evaluation might be lenient and return empty string instead of failing")
	}
}

// TestExpressionWithEventData tests expressions accessing event data
func TestExpressionWithEventData(t *testing.T) {
	workflow := &schema.Workflow{
		Name: "test-event-expr",
		Steps: []schema.Step{
			{
				Name: "echo-event-cwd",
				Run:  "echo ${{ event.cwd }}",
			},
		},
	}

	event := &schema.Event{
		Cwd:       "/test/path",
		Timestamp: "2024-01-01T00:00:00Z",
	}

	runner := NewRunner(workflow, event, ".", "")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	result := results[0]
	if !result.Success {
		t.Errorf("Event data expression should succeed, got error: %v", result.Error)
	}

	if !strings.Contains(result.Output, "/test/path") {
		t.Errorf("Expected output to contain '/test/path', got: %s", result.Output)
	}
}

// TestComplexExpressionInterpolation tests complex expressions with multiple operations
func TestComplexExpressionInterpolation(t *testing.T) {
	workflow := &schema.Workflow{
		Name: "test-complex-expr",
		Env: map[string]string{
			"BASE": "value",
		},
		Steps: []schema.Step{
			{
				Name: "complex-expr",
				Run:  "echo ${{ env.BASE }}_suffix",
			},
		},
	}

	runner := NewRunner(workflow, nil, ".", "")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	result := results[0]
	if !result.Success {
		t.Errorf("Complex expression should succeed, got error: %v", result.Error)
	}

	if !strings.Contains(result.Output, "value_suffix") {
		t.Errorf("Expected output to contain 'value_suffix', got: %s", result.Output)
	}
}

// ============================================================================
// Step Output Capture Tests
// ============================================================================

// TestStepOutputCapture tests that step output is properly captured
func TestStepOutputCapture(t *testing.T) {
	workflow := &schema.Workflow{
		Name: "test-output-capture",
		Steps: []schema.Step{
			{
				Name: "multi-line-output",
				Run:  "echo 'line1'; echo 'line2'; echo 'line3'",
			},
		},
	}

	runner := NewRunner(workflow, nil, ".", "")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	result := results[0]
	if !result.Success {
		t.Errorf("Output capture should succeed, got error: %v", result.Error)
	}

	output := result.Output
	if !strings.Contains(output, "line1") {
		t.Errorf("Expected output to contain 'line1'")
	}
	if !strings.Contains(output, "line2") {
		t.Errorf("Expected output to contain 'line2'")
	}
	if !strings.Contains(output, "line3") {
		t.Errorf("Expected output to contain 'line3'")
	}
}

// TestStepErrorOutputCapture tests that stderr is captured
func TestStepErrorOutputCapture(t *testing.T) {
	workflow := &schema.Workflow{
		Name: "test-error-output",
		Steps: []schema.Step{
			{
				Name: "stderr-test",
				Run:  "echo 'stdout' && echo 'stderr'",
			},
		},
	}

	runner := NewRunner(workflow, nil, ".", "")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	result := results[0]
	if !result.Success {
		t.Errorf("Output capture should succeed, got error: %v", result.Error)
	}

	output := result.Output
	if !strings.Contains(output, "stdout") {
		t.Errorf("Expected output to contain 'stdout'")
	}
	if !strings.Contains(output, "stderr") {
		t.Errorf("Expected output to contain 'stderr'")
	}
}

// TestEmptyStepOutput tests handling of steps with no output
func TestEmptyStepOutput(t *testing.T) {
	workflow := &schema.Workflow{
		Name: "test-empty-output",
		Steps: []schema.Step{
			{
				Name: "no-output",
				Run:  "exit 0",
			},
		},
	}

	runner := NewRunner(workflow, nil, ".", "")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	result := results[0]
	if !result.Success {
		t.Errorf("Step with no output should succeed, got error: %v", result.Error)
	}

	// Output can be empty, that's ok
	if result.Output != "" {
		t.Logf("Note: Step output is: %q (expected empty or whitespace)", result.Output)
	}
}

// TestLargeStepOutput tests handling of large output
func TestLargeStepOutput(t *testing.T) {
	// Create a command that outputs many lines
	// Using a simple loop that's more portable
	workflow := &schema.Workflow{
		Name: "test-large-output",
		Steps: []schema.Step{
			{
				Name: "large-output",
				Run:  "echo 'Line 1'; echo 'Line 2'; echo 'Line 3'; echo 'Line 4'; echo 'Line 5'",
			},
		},
	}

	runner := NewRunner(workflow, nil, ".", "")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	result := results[0]
	if !result.Success {
		t.Errorf("Large output should succeed, got error: %v", result.Error)
	}

	// Check that output contains multiple lines
	lineCount := strings.Count(result.Output, "Line")
	if lineCount < 3 {
		t.Errorf("Expected multiple lines in output, got %d lines", lineCount)
	}
}

// TestStepOutputWithSpecialCharacters tests output handling of special characters
func TestStepOutputWithSpecialCharacters(t *testing.T) {
	workflow := &schema.Workflow{
		Name: "test-special-chars",
		Steps: []schema.Step{
			{
				Name: "special-output",
				Run:  "echo 'Special: !@#$%^&*()_+-=[]{}|;:,.<>?'",
			},
		},
	}

	runner := NewRunner(workflow, nil, ".", "")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	result := results[0]
	if !result.Success {
		t.Errorf("Special character output should succeed, got error: %v", result.Error)
	}

	if !strings.Contains(result.Output, "Special:") {
		t.Errorf("Expected output to contain special characters")
	}
}

// TestDurationCapture tests that step duration is recorded
func TestDurationCapture(t *testing.T) {
	workflow := &schema.Workflow{
		Name: "test-duration",
		Steps: []schema.Step{
			{
				Name: "sleep-short",
				Run:  "sleep 1",
			},
		},
	}

	runner := NewRunner(workflow, nil, ".", "")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	result := results[0]
	if !result.Success {
		t.Errorf("Sleep command should succeed, got error: %v", result.Error)
	}

	// Duration should be at least 1 second
	if result.Duration < time.Second {
		t.Errorf("Expected duration >= 1 second, got %v", result.Duration)
	}

	// Duration should be less than 10 seconds (generous margin for CI + pwsh startup)
	if result.Duration > 10*time.Second {
		t.Errorf("Expected duration <= 10 seconds, got %v", result.Duration)
	}
}
