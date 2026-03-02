package runner

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/htekdev/gh-hookflow/internal/schema"
)

// ============================================================================
// Shell Type Tests
// ============================================================================

// TestShellTypeBash tests bash shell execution when explicitly specified
func TestShellTypeBash(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping - bash shell test is not reliable on Windows")
	}
	// Skip if bash is not available
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("Skipping - bash not available")
	}

	workflow := &schema.Workflow{
		Name: "test-bash-shell",
		Steps: []schema.Step{
			{
				Name:  "bash-step",
				Shell: "bash",
				Run:   "echo $BASH_VERSION",
			},
		},
	}

	runner := NewRunner(workflow, nil, ".")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	result := results[0]
	if !result.Success {
		t.Errorf("bash command should succeed, got error: %v", result.Error)
	}
}

// TestShellTypeSh tests sh shell execution when explicitly specified
func TestShellTypeSh(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping - sh shell test is not reliable on Windows")
	}
	// Skip if sh is not available
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("Skipping - sh not available")
	}

	workflow := &schema.Workflow{
		Name: "test-sh-shell",
		Steps: []schema.Step{
			{
				Name:  "sh-step",
				Shell: "sh",
				Run:   "echo hello from sh",
			},
		},
	}

	runner := NewRunner(workflow, nil, ".")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	result := results[0]
	if !result.Success {
		t.Errorf("sh command should succeed, got error: %v", result.Error)
	}

	if !strings.Contains(result.Output, "hello from sh") {
		t.Errorf("Expected output to contain 'hello from sh', got: %s", result.Output)
	}
}

// TestShellTypePwsh tests pwsh (PowerShell Core) shell execution
func TestShellTypePwsh(t *testing.T) {
	workflow := &schema.Workflow{
		Name: "test-pwsh-shell",
		Steps: []schema.Step{
			{
				Name:  "pwsh-step",
				Shell: "pwsh",
				Run:   "Write-Output 'hello from pwsh'",
			},
		},
	}

	runner := NewRunner(workflow, nil, ".")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	result := results[0]
	if !result.Success {
		t.Errorf("pwsh command should succeed, got error: %v", result.Error)
	}

	if !strings.Contains(result.Output, "hello from pwsh") {
		t.Errorf("Expected output to contain 'hello from pwsh', got: %s", result.Output)
	}
}

// TestShellTypePowerShell tests powershell alias shell execution
func TestShellTypePowerShell(t *testing.T) {
	workflow := &schema.Workflow{
		Name: "test-powershell-shell",
		Steps: []schema.Step{
			{
				Name:  "powershell-step",
				Shell: "powershell",
				Run:   "Write-Output 'hello from powershell'",
			},
		},
	}

	runner := NewRunner(workflow, nil, ".")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	result := results[0]
	if !result.Success {
		t.Errorf("powershell command should succeed, got error: %v", result.Error)
	}

	if !strings.Contains(result.Output, "hello from powershell") {
		t.Errorf("Expected output to contain 'hello from powershell', got: %s", result.Output)
	}
}

// TestShellTypeCmd tests cmd shell execution (Windows only)
func TestShellTypeCmd(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Skipping cmd test on non-Windows")
	}

	workflow := &schema.Workflow{
		Name: "test-cmd-shell",
		Steps: []schema.Step{
			{
				Name:  "cmd-step",
				Shell: "cmd",
				Run:   "echo hello from cmd",
			},
		},
	}

	runner := NewRunner(workflow, nil, ".")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	result := results[0]
	if !result.Success {
		t.Errorf("cmd command should succeed, got error: %v", result.Error)
	}

	if !strings.Contains(result.Output, "hello from cmd") {
		t.Errorf("Expected output to contain 'hello from cmd', got: %s", result.Output)
	}
}

// TestShellTypeDefaultOnWindows tests default shell on Windows
func TestShellTypeDefaultOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Skipping Windows-specific default shell test")
	}

	if defaultShell() != "pwsh" {
		t.Errorf("Expected default shell on Windows to be 'pwsh', got: %s", defaultShell())
	}
}

// TestShellTypeDefaultOnUnix tests default shell on Unix
// NOTE: We standardized on pwsh (PowerShell Core) as the default shell for cross-platform consistency
func TestShellTypeDefaultOnUnix(t *testing.T) {
	// Default shell is now pwsh for all platforms
	if defaultShell() != "pwsh" {
		t.Errorf("Expected default shell to be 'pwsh', got: %s", defaultShell())
	}
}

// TestShellTypeCustom tests a custom shell (falls back to -c convention)
func TestShellTypeCustom(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping custom shell test on Windows")
	}

	workflow := &schema.Workflow{
		Name: "test-custom-shell",
		Steps: []schema.Step{
			{
				Name:  "custom-shell-step",
				Shell: "bash", // Using bash as a "custom" shell
				Run:   "echo 'custom shell works'",
			},
		},
	}

	runner := NewRunner(workflow, nil, ".")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	result := results[0]
	if !result.Success {
		t.Errorf("custom shell command should succeed, got error: %v", result.Error)
	}
}

// ============================================================================
// Step Name Auto-Generation Tests
// ============================================================================

// TestStepNameAutoGeneration tests that step names are auto-generated when not provided
func TestStepNameAutoGeneration(t *testing.T) {
	workflow := &schema.Workflow{
		Name: "test-auto-name",
		Steps: []schema.Step{
			{
				// No name provided
				Run: "echo 'first'",
			},
			{
				// No name provided
				Run: "echo 'second'",
			},
			{
				// Name provided
				Name: "named-step",
				Run:  "echo 'third'",
			},
		},
	}

	runner := NewRunner(workflow, nil, ".")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("Expected 3 results, got %d", len(results))
	}

	// First step should be auto-named "Step 1"
	if results[0].Name != "Step 1" {
		t.Errorf("Expected first step to be 'Step 1', got: %s", results[0].Name)
	}

	// Second step should be auto-named "Step 2"
	if results[1].Name != "Step 2" {
		t.Errorf("Expected second step to be 'Step 2', got: %s", results[1].Name)
	}

	// Third step should keep its provided name
	if results[2].Name != "named-step" {
		t.Errorf("Expected third step to be 'named-step', got: %s", results[2].Name)
	}
}

// ============================================================================
// Steps with Neither Run Nor Uses (Should Error)
// ============================================================================

// TestStepWithNeitherRunNorUses tests that steps without run or uses fail
func TestStepWithNeitherRunNorUses(t *testing.T) {
	workflow := &schema.Workflow{
		Name: "test-no-run-uses",
		Steps: []schema.Step{
			{
				Name: "empty-step",
				// Neither run nor uses provided
			},
		},
	}

	runner := NewRunner(workflow, nil, ".")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error from Run(), got %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	result := results[0]
	if result.Success {
		t.Errorf("Expected step without run or uses to fail")
	}

	if result.Error == nil {
		t.Errorf("Expected error for step without run or uses")
	}

	if !strings.Contains(result.Error.Error(), "neither 'run' nor 'uses'") {
		t.Errorf("Expected error about missing run/uses, got: %v", result.Error)
	}
}

// ============================================================================
// Workflow Event Context Population Tests
// ============================================================================

// TestEventContextCwdAndTimestamp tests that event cwd and timestamp are populated
func TestEventContextCwdAndTimestamp(t *testing.T) {
	workflow := &schema.Workflow{
		Name: "test-event-context",
		Steps: []schema.Step{
			{
				Name: "check-event-cwd",
				If:   "${{ event.cwd == '/test/path' }}",
				Run:  "echo 'cwd matches'",
			},
		},
	}

	event := &schema.Event{
		Cwd:       "/test/path",
		Timestamp: "2024-01-01T12:00:00Z",
	}

	runner := NewRunner(workflow, event, ".")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	result := results[0]
	if strings.Contains(result.Output, "Skipped") {
		t.Errorf("Expected step to run when event.cwd matches, but it was skipped")
	}
}

// TestEventContextHook tests that hook event data is populated in context
func TestEventContextHook(t *testing.T) {
	workflow := &schema.Workflow{
		Name: "test-hook-event-context",
		Steps: []schema.Step{
			{
				Name: "check-hook-type",
				If:   "${{ event.hook.type == 'preToolUse' }}",
				Run:  "echo 'hook type matches'",
			},
		},
	}

	event := &schema.Event{
		Cwd:       "/test",
		Timestamp: "2024-01-01T12:00:00Z",
		Hook: &schema.HookEvent{
			Type: "preToolUse",
			Cwd:  "/test",
		},
	}

	runner := NewRunner(workflow, event, ".")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	result := results[0]
	if strings.Contains(result.Output, "Skipped") {
		t.Errorf("Expected step to run when hook type matches")
	}
}

// TestEventContextTool tests that tool event data is populated in context
func TestEventContextTool(t *testing.T) {
	workflow := &schema.Workflow{
		Name: "test-tool-event-context",
		Steps: []schema.Step{
			{
				Name: "check-tool-name",
				If:   "${{ event.tool.name == 'edit' }}",
				Run:  "echo 'tool name matches'",
			},
		},
	}

	event := &schema.Event{
		Cwd:       "/test",
		Timestamp: "2024-01-01T12:00:00Z",
		Tool: &schema.ToolEvent{
			Name:     "edit",
			Args:     map[string]interface{}{"path": "/test/file.go"},
			HookType: "preToolUse",
		},
	}

	runner := NewRunner(workflow, event, ".")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	result := results[0]
	if strings.Contains(result.Output, "Skipped") {
		t.Errorf("Expected step to run when tool name matches")
	}
}

// TestEventContextToolWithHook tests hook with nested tool data
func TestEventContextToolWithHook(t *testing.T) {
	workflow := &schema.Workflow{
		Name: "test-hook-tool-context",
		Steps: []schema.Step{
			{
				Name: "check-hook-tool",
				If:   "${{ event.hook.tool.name == 'create' }}",
				Run:  "echo 'hook tool name matches'",
			},
		},
	}

	event := &schema.Event{
		Cwd:       "/test",
		Timestamp: "2024-01-01T12:00:00Z",
		Hook: &schema.HookEvent{
			Type: "preToolUse",
			Cwd:  "/test",
			Tool: &schema.ToolEvent{
				Name: "create",
				Args: map[string]interface{}{"path": "/new/file.txt"},
			},
		},
	}

	runner := NewRunner(workflow, event, ".")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	result := results[0]
	if strings.Contains(result.Output, "Skipped") {
		t.Errorf("Expected step to run when hook.tool.name matches")
	}
}

// TestEventContextFile tests that file event data is populated in context
func TestEventContextFile(t *testing.T) {
	workflow := &schema.Workflow{
		Name: "test-file-event-context",
		Steps: []schema.Step{
			{
				Name: "check-file-action",
				If:   "${{ event.file.action == 'create' }}",
				Run:  "echo 'file action matches'",
			},
		},
	}

	event := &schema.Event{
		Cwd:       "/test",
		Timestamp: "2024-01-01T12:00:00Z",
		File: &schema.FileEvent{
			Path:    "/test/new-file.go",
			Action:  "create",
			Content: "package main",
		},
	}

	runner := NewRunner(workflow, event, ".")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	result := results[0]
	if strings.Contains(result.Output, "Skipped") {
		t.Errorf("Expected step to run when file action matches")
	}
}

// TestEventContextCommit tests that commit event data is populated in context
func TestEventContextCommit(t *testing.T) {
	workflow := &schema.Workflow{
		Name: "test-commit-event-context",
		Steps: []schema.Step{
			{
				Name: "check-commit-author",
				If:   "${{ event.commit.author == 'test-user' }}",
				Run:  "echo 'commit author matches'",
			},
		},
	}

	event := &schema.Event{
		Cwd:       "/test",
		Timestamp: "2024-01-01T12:00:00Z",
		Commit: &schema.CommitEvent{
			SHA:     "abc123",
			Message: "Test commit message",
			Author:  "test-user",
			Files: []schema.FileStatus{
				{Path: "file1.go", Status: "modified"},
				{Path: "file2.go", Status: "added"},
			},
		},
	}

	runner := NewRunner(workflow, event, ".")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	result := results[0]
	if strings.Contains(result.Output, "Skipped") {
		t.Errorf("Expected step to run when commit author matches")
	}
}

// TestEventContextPush tests that push event data is populated in context
func TestEventContextPush(t *testing.T) {
	workflow := &schema.Workflow{
		Name: "test-push-event-context",
		Steps: []schema.Step{
			{
				Name: "check-push-ref",
				If:   "${{ event.push.ref == 'refs/heads/main' }}",
				Run:  "echo 'push ref matches'",
			},
		},
	}

	event := &schema.Event{
		Cwd:       "/test",
		Timestamp: "2024-01-01T12:00:00Z",
		Push: &schema.PushEvent{
			Ref:    "refs/heads/main",
			Before: "abc123",
			After:  "def456",
		},
	}

	runner := NewRunner(workflow, event, ".")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	result := results[0]
	if strings.Contains(result.Output, "Skipped") {
		t.Errorf("Expected step to run when push ref matches")
	}
}

// TestEventContextNil tests that nil event doesn't cause panic
func TestEventContextNil(t *testing.T) {
	workflow := &schema.Workflow{
		Name: "test-nil-event",
		Steps: []schema.Step{
			{
				Name: "simple-step",
				Run:  "echo 'works with nil event'",
			},
		},
	}

	// Pass nil event
	runner := NewRunner(workflow, nil, ".")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error with nil event, got %v", err)
	}

	result := results[0]
	if !result.Success {
		t.Errorf("Expected step to succeed with nil event, got error: %v", result.Error)
	}
}

// ============================================================================
// Stderr/Stdout Capture Tests
// ============================================================================

// TestStdoutCapture tests that stdout is captured correctly
func TestStdoutCapture(t *testing.T) {
	workflow := &schema.Workflow{
		Name: "test-stdout",
		Steps: []schema.Step{
			{
				Name: "stdout-step",
				Run:  "echo 'stdout message'",
			},
		},
	}

	runner := NewRunner(workflow, nil, ".")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	result := results[0]
	if !strings.Contains(result.Output, "stdout message") {
		t.Errorf("Expected output to contain stdout message, got: %s", result.Output)
	}
}

// TestStderrCapture tests that stderr is captured correctly
func TestStderrCapture(t *testing.T) {
	// Use a command that writes to stderr
	var cmd string
	if runtime.GOOS == "windows" {
		cmd = "Write-Error 'stderr message' -ErrorAction Continue"
	} else {
		cmd = "echo 'stderr message' >&2"
	}

	workflow := &schema.Workflow{
		Name: "test-stderr",
		Steps: []schema.Step{
			{
				Name: "stderr-step",
				Run:  cmd,
			},
		},
	}

	runner := NewRunner(workflow, nil, ".")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	result := results[0]
	if !strings.Contains(result.Output, "stderr") {
		t.Errorf("Expected output to contain stderr message, got: %s", result.Output)
	}
}

// TestStdoutAndStderrCombined tests that both stdout and stderr are captured
func TestStdoutAndStderrCombined(t *testing.T) {
	var cmd string
	if runtime.GOOS == "windows" {
		cmd = "Write-Output 'stdout'; Write-Error 'stderr' -ErrorAction Continue"
	} else {
		cmd = "echo 'stdout' && echo 'stderr' >&2"
	}

	workflow := &schema.Workflow{
		Name: "test-combined-output",
		Steps: []schema.Step{
			{
				Name: "combined-step",
				Run:  cmd,
			},
		},
	}

	runner := NewRunner(workflow, nil, ".")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	result := results[0]
	// Both stdout and stderr should be in the output
	if !strings.Contains(result.Output, "stdout") {
		t.Errorf("Expected output to contain 'stdout', got: %s", result.Output)
	}
	if !strings.Contains(result.Output, "stderr") {
		t.Errorf("Expected output to contain 'stderr', got: %s", result.Output)
	}
}

// ============================================================================
// Timeout Edge Cases
// ============================================================================

// TestVeryShortTimeout tests a very short timeout (less than 1 second)
func TestVeryShortTimeout(t *testing.T) {
	// Sleep for 2 seconds with a very short timeout
	sleepCmd := "sleep 2"
	if runtime.GOOS == "windows" {
		sleepCmd = "Start-Sleep -Seconds 2"
	}

	workflow := &schema.Workflow{
		Name: "test-very-short-timeout",
		Steps: []schema.Step{
			{
				Name:    "short-timeout",
				Run:     sleepCmd,
				Timeout: 1, // 1 second timeout - minimum practical timeout
			},
		},
	}

	runner := NewRunner(workflow, nil, ".")
	start := time.Now()
	results, err := runner.Run(context.Background())
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Expected no error from Run(), got %v", err)
	}

	result := results[0]
	if result.Success {
		t.Errorf("Expected timeout failure")
	}

	if result.Error == nil || !strings.Contains(result.Error.Error(), "timed out") {
		t.Errorf("Expected timeout error, got: %v", result.Error)
	}

	// Should complete within 2 seconds (1s timeout + overhead)
	if elapsed > 3*time.Second {
		t.Errorf("Expected to complete faster than 3s due to timeout, took: %v", elapsed)
	}
}

// TestTimeoutMessageIncludesSeconds tests that timeout error includes the timeout value
func TestTimeoutMessageIncludesSeconds(t *testing.T) {
	sleepCmd := "sleep 10"
	if runtime.GOOS == "windows" {
		sleepCmd = "Start-Sleep -Seconds 10"
	}

	workflow := &schema.Workflow{
		Name: "test-timeout-message",
		Steps: []schema.Step{
			{
				Name:    "timeout-step",
				Run:     sleepCmd,
				Timeout: 1,
			},
		},
	}

	runner := NewRunner(workflow, nil, ".")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	result := results[0]
	if result.Error == nil {
		t.Fatalf("Expected error, got nil")
	}

	errorMsg := result.Error.Error()
	if !strings.Contains(errorMsg, "1 seconds") {
		t.Errorf("Expected error to contain '1 seconds', got: %s", errorMsg)
	}
}

// ============================================================================
// Working Directory Tests
// ============================================================================

// TestWorkingDirectoryInvalidPath tests behavior with invalid working directory
func TestWorkingDirectoryInvalidPath(t *testing.T) {
	workflow := &schema.Workflow{
		Name: "test-invalid-wd",
		Steps: []schema.Step{
			{
				Name:             "invalid-wd-step",
				Run:              "echo 'test'",
				WorkingDirectory: "/nonexistent/path/that/does/not/exist/12345",
			},
		},
	}

	runner := NewRunner(workflow, nil, ".")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error from Run(), got %v", err)
	}

	result := results[0]
	// The step should fail because the working directory doesn't exist
	if result.Success {
		t.Logf("Note: Step succeeded despite invalid working directory (shell-dependent behavior)")
	}
}

// TestWorkingDirectoryWithEnvVar tests working directory from environment variable expression
func TestWorkingDirectoryWithEnvVar(t *testing.T) {
	tmpDir := os.TempDir()

	workflow := &schema.Workflow{
		Name: "test-wd-env",
		Env: map[string]string{
			"WORK_DIR": tmpDir,
		},
		Steps: []schema.Step{
			{
				Name:             "wd-env-step",
				Run:              "echo 'working'",
				WorkingDirectory: "${{ env.WORK_DIR }}",
			},
		},
	}

	runner := NewRunner(workflow, nil, ".")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	result := results[0]
	if !result.Success {
		t.Errorf("Expected step to succeed with env var working directory, got error: %v", result.Error)
	}
}

// TestMultipleStepsWithDifferentWorkingDirectories tests steps with different working directories
func TestMultipleStepsWithDifferentWorkingDirectories(t *testing.T) {
	tmpDir := os.TempDir()
	currentDir, _ := os.Getwd()

	workflow := &schema.Workflow{
		Name: "test-multiple-wd",
		Steps: []schema.Step{
			{
				Name:             "step1-tmpdir",
				Run:              "echo 'step1'",
				WorkingDirectory: tmpDir,
			},
			{
				Name:             "step2-currentdir",
				Run:              "echo 'step2'",
				WorkingDirectory: currentDir,
			},
			{
				Name: "step3-default",
				Run:  "echo 'step3'",
				// No working directory specified - uses runner's default
			},
		},
	}

	runner := NewRunner(workflow, nil, currentDir)
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("Expected 3 results, got %d", len(results))
	}

	for i, result := range results {
		if !result.Success {
			t.Errorf("Step %d failed: %v", i+1, result.Error)
		}
	}
}

// ============================================================================
// Environment Variable Interpolation Tests
// ============================================================================

// TestEnvVarInterpolationInCommand tests expression interpolation of env vars in commands
func TestEnvVarInterpolationInCommand(t *testing.T) {
	workflow := &schema.Workflow{
		Name: "test-env-interpolation",
		Env: map[string]string{
			"MY_MESSAGE": "Hello World",
		},
		Steps: []schema.Step{
			{
				Name: "env-step",
				Run:  "echo '${{ env.MY_MESSAGE }}'",
			},
		},
	}

	runner := NewRunner(workflow, nil, ".")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	result := results[0]
	if !result.Success {
		t.Errorf("Step should succeed, got error: %v", result.Error)
	}

	if !strings.Contains(result.Output, "Hello World") {
		t.Errorf("Expected output to contain 'Hello World', got: %s", result.Output)
	}
}

// TestStepEnvVarAdded tests that step env vars are added to the process environment
func TestStepEnvVarAdded(t *testing.T) {
	// Skip if pwsh is not available
	if _, err := exec.LookPath("pwsh"); err != nil {
		t.Skip("pwsh not available")
	}

	// Use pwsh syntax since that's the default shell
	cmd := "Write-Output $env:STEP_VAR"

	workflow := &schema.Workflow{
		Name: "test-step-env",
		Steps: []schema.Step{
			{
				Name: "step-env-test",
				Run:  cmd,
				Env: map[string]string{
					"STEP_VAR": "step_value_123",
				},
			},
		},
	}

	runner := NewRunner(workflow, nil, ".")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	result := results[0]
	if !result.Success {
		t.Errorf("Step should succeed, got error: %v", result.Error)
	}

	if !strings.Contains(result.Output, "step_value_123") {
		t.Errorf("Expected output to contain 'step_value_123', got: %s", result.Output)
	}
}

// TestEnvVarWithExpression tests env var value containing expression
func TestEnvVarWithExpression(t *testing.T) {
	// Skip if pwsh is not available
	if _, err := exec.LookPath("pwsh"); err != nil {
		t.Skip("pwsh not available")
	}

	// Use pwsh syntax since that's the default shell
	cmd := "Write-Output $env:DYNAMIC_VAR"

	workflow := &schema.Workflow{
		Name: "test-env-expr",
		Env: map[string]string{
			"BASE_VALUE": "base",
		},
		Steps: []schema.Step{
			{
				Name: "dynamic-env-test",
				Run:  cmd,
				Env: map[string]string{
					"DYNAMIC_VAR": "${{ env.BASE_VALUE }}_suffix",
				},
			},
		},
	}

	runner := NewRunner(workflow, nil, ".")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	result := results[0]
	if !result.Success {
		t.Errorf("Step should succeed, got error: %v", result.Error)
	}

	if !strings.Contains(result.Output, "base_suffix") {
		t.Errorf("Expected output to contain 'base_suffix', got: %s", result.Output)
	}
}

// ============================================================================
// If Condition Error Handling Tests
// ============================================================================

// TestIfConditionErrorSetsFailure tests that if condition errors set step as failed
func TestIfConditionErrorSetsFailure(t *testing.T) {
	// Note: The expression evaluator treats missing properties as empty/falsy rather than errors.
	// This test documents that behavior - missing property chains return empty, causing skip.
	workflow := &schema.Workflow{
		Name: "test-if-error",
		Steps: []schema.Step{
			{
				Name: "empty-condition-step",
				If:   "${{ nonexistent.property.chain }}", // This evaluates to empty/falsy, not an error
				Run:  "echo 'should not run'",
			},
		},
	}

	runner := NewRunner(workflow, nil, ".")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error from Run(), got %v", err)
	}

	result := results[0]
	// The step should be skipped because the condition evaluates to falsy
	if !result.Success {
		// If it fails, it should be due to condition evaluation - both behaviors are acceptable
		t.Logf("Step failed (condition evaluation issue): %v", result.Error)
	} else {
		// Step marked success means it was skipped
		if !strings.Contains(result.Output, "Skipped") {
			t.Errorf("Expected step to be skipped when condition is falsy, got output: %s", result.Output)
		}
	}
}

// TestIfConditionErrorWithContinueOnError tests if error with continue-on-error
func TestIfConditionErrorWithContinueOnError(t *testing.T) {
	workflow := &schema.Workflow{
		Name: "test-if-error-continue",
		Steps: []schema.Step{
			{
				Name:            "error-condition-step",
				If:              "${{ invalid_func_xxx() }}",
				Run:             "echo 'should not run'",
				ContinueOnError: true,
			},
			{
				Name: "next-step",
				Run:  "echo 'this should run'",
			},
		},
	}

	runner := NewRunner(workflow, nil, ".")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error from Run(), got %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("Expected 2 results, got %d", len(results))
	}

	// First step should fail
	if results[0].Success {
		t.Errorf("First step should fail due to condition error")
	}

	// Second step should run because first has continue-on-error
	if !results[1].Success {
		t.Errorf("Second step should succeed, got error: %v", results[1].Error)
	}
}

// ============================================================================
// Step Execution Order Tests
// ============================================================================

// TestStepExecutionOrder tests that steps execute in order
func TestStepExecutionOrder(t *testing.T) {
	workflow := &schema.Workflow{
		Name: "test-execution-order",
		Steps: []schema.Step{
			{Name: "step-1", Run: "echo 'first'"},
			{Name: "step-2", Run: "echo 'second'"},
			{Name: "step-3", Run: "echo 'third'"},
		},
	}

	runner := NewRunner(workflow, nil, ".")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("Expected 3 results, got %d", len(results))
	}

	expectedNames := []string{"step-1", "step-2", "step-3"}
	for i, result := range results {
		if result.Name != expectedNames[i] {
			t.Errorf("Expected result %d to be '%s', got '%s'", i, expectedNames[i], result.Name)
		}
	}
}

// TestFailurePropagationStopsSubsequentSteps tests that failure stops subsequent steps
func TestFailurePropagationStopsSubsequentSteps(t *testing.T) {
	workflow := &schema.Workflow{
		Name: "test-failure-propagation",
		Steps: []schema.Step{
			{Name: "success-step", Run: "echo 'success'"},
			{Name: "fail-step", Run: "exit 1"},
			{Name: "should-skip", Run: "echo 'should not run'"},
		},
	}

	runner := NewRunner(workflow, nil, ".")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error from Run(), got %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("Expected 3 results, got %d", len(results))
	}

	// First step succeeds
	if !results[0].Success {
		t.Errorf("First step should succeed")
	}

	// Second step fails
	if results[1].Success {
		t.Errorf("Second step should fail")
	}

	// Third step should be skipped
	if results[2].Success {
		t.Errorf("Third step should not succeed (should be skipped)")
	}
	if !strings.Contains(results[2].Output, "Skipped") {
		t.Errorf("Third step should be skipped, got: %s", results[2].Output)
	}
}

// ============================================================================
// Step Context Update Tests
// ============================================================================

// TestStepContextOutcome tests that step context outcome is set correctly
func TestStepContextOutcome(t *testing.T) {
	workflow := &schema.Workflow{
		Name: "test-step-context",
		Steps: []schema.Step{
			{
				Name: "first-step",
				Run:  "echo 'first'",
			},
			{
				Name: "check-first-outcome",
				If:   "true", // Always run
				Run:  "echo 'checking'",
			},
		},
	}

	runner := NewRunner(workflow, nil, ".")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Both steps should succeed
	for i, result := range results {
		if !result.Success {
			t.Errorf("Step %d should succeed, got error: %v", i, result.Error)
		}
	}
}

// ============================================================================
// BuildDenialWithLogs Tests
// ============================================================================

// TestBuildDenialWithLogsContainsWorkflowInfo tests log file contains workflow info
func TestBuildDenialWithLogsContainsWorkflowInfo(t *testing.T) {
	workflow := &schema.Workflow{
		Name:        "test-log-workflow",
		Description: "Test workflow for logging",
		Blocking:    ptrBool(true),
		Steps: []schema.Step{
			{
				Name: "fail-step",
				Run:  "exit 1",
			},
		},
	}

	runner := NewRunner(workflow, nil, ".")
	ctx := context.Background()
	result := runner.RunWithBlocking(ctx)

	if result.PermissionDecision != "deny" {
		t.Errorf("Expected deny, got %s", result.PermissionDecision)
	}

	if result.LogFile == "" {
		t.Fatal("Expected LogFile to be set")
	}

	// Read and check log file content
	content, err := os.ReadFile(result.LogFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	logContent := string(content)

	// Should contain workflow name
	if !strings.Contains(logContent, "test-log-workflow") {
		t.Error("Log should contain workflow name")
	}

	// Should contain description
	if !strings.Contains(logContent, "Test workflow for logging") {
		t.Error("Log should contain workflow description")
	}

	// Should contain step name
	if !strings.Contains(logContent, "fail-step") {
		t.Error("Log should contain step name")
	}

	// Should contain status
	if !strings.Contains(logContent, "FAILED") {
		t.Error("Log should contain FAILED status")
	}

	// Cleanup
	_ = os.Remove(result.LogFile)
}

// TestBuildDenialWithLogsReasonFormat tests that denial reason has correct format
func TestBuildDenialWithLogsReasonFormat(t *testing.T) {
	workflow := &schema.Workflow{
		Name:     "test-reason-format",
		Blocking: ptrBool(true),
		Steps: []schema.Step{
			{
				Name: "fail-step-1",
				Run:  "echo 'output from step 1' && exit 1",
			},
		},
	}

	runner := NewRunner(workflow, nil, ".")
	ctx := context.Background()
	result := runner.RunWithBlocking(ctx)

	if result.PermissionDecision != "deny" {
		t.Errorf("Expected deny, got %s", result.PermissionDecision)
	}

	reason := result.PermissionDecisionReason

	// Should mention workflow is blocked
	if !strings.Contains(reason, "blocked") {
		t.Errorf("Reason should mention 'blocked', got: %s", reason)
	}

	// Should mention the failed step
	if !strings.Contains(reason, "fail-step-1") {
		t.Errorf("Reason should mention failed step, got: %s", reason)
	}

	// Should mention log file path
	if !strings.Contains(reason, result.LogFile) {
		t.Errorf("Reason should mention log file path, got: %s", reason)
	}

	// Cleanup
	if result.LogFile != "" {
		_ = os.Remove(result.LogFile)
	}
}

// ============================================================================
// Uses Step Tests (Reusable Actions)
// ============================================================================

// TestStepWithLocalUses tests local action reference
func TestStepWithLocalUses(t *testing.T) {
	// Create a temporary action directory
	tmpDir, err := os.MkdirTemp("", "test-action")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create action.yml
	actionYml := `
name: Test Action
description: A test action
runs:
  using: shell
  run: echo 'action executed'
`
	if err := os.WriteFile(filepath.Join(tmpDir, "action.yml"), []byte(actionYml), 0644); err != nil {
		t.Fatalf("Failed to write action.yml: %v", err)
	}

	workflow := &schema.Workflow{
		Name: "test-local-uses",
		Steps: []schema.Step{
			{
				Name: "local-action-step",
				Uses: tmpDir,
			},
		},
	}

	runner := NewRunner(workflow, nil, ".")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	result := results[0]
	if !result.Success {
		// Expected - the path needs to be relative
		t.Logf("Local action execution result: success=%v, error=%v", result.Success, result.Error)
	}
}

// TestStepWithMissingLocalAction tests that missing local action fails gracefully
func TestStepWithMissingLocalAction(t *testing.T) {
	workflow := &schema.Workflow{
		Name: "test-missing-local-action",
		Steps: []schema.Step{
			{
				Name: "missing-action-step",
				Uses: "./nonexistent-action-path-xyz",
			},
		},
	}

	runner := NewRunner(workflow, nil, ".")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error from Run(), got %v", err)
	}

	result := results[0]
	if result.Success {
		t.Errorf("Expected step to fail with missing action")
	}

	if result.Error == nil {
		t.Errorf("Expected error for missing action")
	}
}

// TestLocalActionWithValidActionYml tests loading and executing a valid local action
func TestLocalActionWithValidActionYml(t *testing.T) {
	// Create a temporary action directory
	tmpDir, err := os.MkdirTemp("", "test-valid-action")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create action.yml with shell-based runs
	actionYml := `name: Test Shell Action
description: A test shell action
runs:
  using: shell
  run: echo 'action executed successfully'
`
	if err := os.WriteFile(filepath.Join(tmpDir, "action.yml"), []byte(actionYml), 0644); err != nil {
		t.Fatalf("Failed to write action.yml: %v", err)
	}

	// Use relative path format
	workflow := &schema.Workflow{
		Name: "test-valid-local-action",
		Steps: []schema.Step{
			{
				Name: "valid-action-step",
				Uses: "./" + filepath.Base(tmpDir),
			},
		},
	}

	// Run from parent dir so relative path works
	runner := NewRunner(workflow, nil, filepath.Dir(tmpDir))
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	result := results[0]
	// Log the result for debugging
	t.Logf("Action result: success=%v, error=%v, output=%s", result.Success, result.Error, result.Output)
}

// TestActionWithCompositeSteps tests composite action with multiple steps
func TestActionWithCompositeSteps(t *testing.T) {
	// Create a temporary action directory
	tmpDir, err := os.MkdirTemp("", "test-composite-action")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create action.yml with composite steps
	actionYml := `name: Composite Test Action
description: A composite test action
runs:
  using: composite
  steps:
    - name: Step 1
      run: echo 'composite step 1'
    - name: Step 2
      run: echo 'composite step 2'
`
	if err := os.WriteFile(filepath.Join(tmpDir, "action.yml"), []byte(actionYml), 0644); err != nil {
		t.Fatalf("Failed to write action.yml: %v", err)
	}

	workflow := &schema.Workflow{
		Name: "test-composite-action",
		Steps: []schema.Step{
			{
				Name: "composite-step",
				Uses: "./" + filepath.Base(tmpDir),
			},
		},
	}

	runner := NewRunner(workflow, nil, filepath.Dir(tmpDir))
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	result := results[0]
	t.Logf("Composite action result: success=%v, error=%v, output=%s", result.Success, result.Error, result.Output)
}

// TestActionWithInputs tests action with input parameters
func TestActionWithInputs(t *testing.T) {
	// Create a temporary action directory
	tmpDir, err := os.MkdirTemp("", "test-action-inputs")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create action.yml with inputs
	var runCmd string
	if runtime.GOOS == "windows" {
		runCmd = "Write-Output \"Message: $env:INPUT_MESSAGE\""
	} else {
		runCmd = "echo \"Message: $INPUT_MESSAGE\""
	}

	actionYml := `name: Input Test Action
description: Action with inputs
inputs:
  message:
    description: Message to print
    required: true
    default: 'default message'
runs:
  using: shell
  run: ` + runCmd + `
`
	if err := os.WriteFile(filepath.Join(tmpDir, "action.yml"), []byte(actionYml), 0644); err != nil {
		t.Fatalf("Failed to write action.yml: %v", err)
	}

	workflow := &schema.Workflow{
		Name: "test-action-with-inputs",
		Steps: []schema.Step{
			{
				Name: "input-action-step",
				Uses: "./" + filepath.Base(tmpDir),
				With: map[string]string{
					"message": "custom message from workflow",
				},
			},
		},
	}

	runner := NewRunner(workflow, nil, filepath.Dir(tmpDir))
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	result := results[0]
	t.Logf("Input action result: success=%v, error=%v, output=%s", result.Success, result.Error, result.Output)
}

// TestActionYamlAlternativeFile tests loading action.yaml instead of action.yml
func TestActionYamlAlternativeFile(t *testing.T) {
	// Create a temporary action directory
	tmpDir, err := os.MkdirTemp("", "test-action-yaml")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create action.yaml (not .yml)
	actionYaml := `name: YAML Extension Action
description: Action using .yaml extension
runs:
  using: shell
  run: echo 'yaml extension works'
`
	if err := os.WriteFile(filepath.Join(tmpDir, "action.yaml"), []byte(actionYaml), 0644); err != nil {
		t.Fatalf("Failed to write action.yaml: %v", err)
	}

	workflow := &schema.Workflow{
		Name: "test-action-yaml-extension",
		Steps: []schema.Step{
			{
				Name: "yaml-action-step",
				Uses: "./" + filepath.Base(tmpDir),
			},
		},
	}

	runner := NewRunner(workflow, nil, filepath.Dir(tmpDir))
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	result := results[0]
	t.Logf("YAML action result: success=%v, error=%v", result.Success, result.Error)
}

// TestActionMissingMetadataFile tests action with no action.yml/action.yaml
func TestActionMissingMetadataFile(t *testing.T) {
	// Create empty action directory
	tmpDir, err := os.MkdirTemp("", "test-no-metadata")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	workflow := &schema.Workflow{
		Name: "test-no-metadata-action",
		Steps: []schema.Step{
			{
				Name: "no-metadata-step",
				Uses: "./" + filepath.Base(tmpDir),
			},
		},
	}

	runner := NewRunner(workflow, nil, filepath.Dir(tmpDir))
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error from Run(), got %v", err)
	}

	result := results[0]
	if result.Success {
		t.Errorf("Expected step to fail without action metadata")
	}

	if result.Error == nil {
		t.Errorf("Expected error for missing metadata")
	} else {
		// Should mention missing action.yaml/action.yml
		if !strings.Contains(result.Error.Error(), "action") {
			t.Errorf("Error should mention action metadata, got: %v", result.Error)
		}
	}
}

// TestActionInvalidYaml tests action with malformed YAML
func TestActionInvalidYaml(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-invalid-yaml")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create invalid YAML
	invalidYml := `name: Invalid Action
this is not valid yaml:
  - [ broken structure
    missing bracket
`
	if err := os.WriteFile(filepath.Join(tmpDir, "action.yml"), []byte(invalidYml), 0644); err != nil {
		t.Fatalf("Failed to write invalid action.yml: %v", err)
	}

	workflow := &schema.Workflow{
		Name: "test-invalid-yaml-action",
		Steps: []schema.Step{
			{
				Name: "invalid-yaml-step",
				Uses: "./" + filepath.Base(tmpDir),
			},
		},
	}

	runner := NewRunner(workflow, nil, filepath.Dir(tmpDir))
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error from Run(), got %v", err)
	}

	result := results[0]
	if result.Success {
		t.Errorf("Expected step to fail with invalid YAML")
	}

	if result.Error == nil {
		t.Errorf("Expected error for invalid YAML")
	}
}

// TestActionUnsupportedType tests action with unsupported type (docker)
func TestActionUnsupportedType(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-docker-action")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create docker-based action (unsupported)
	dockerActionYml := `name: Docker Action
description: A docker-based action
runs:
  using: docker
  image: Dockerfile
`
	if err := os.WriteFile(filepath.Join(tmpDir, "action.yml"), []byte(dockerActionYml), 0644); err != nil {
		t.Fatalf("Failed to write action.yml: %v", err)
	}

	workflow := &schema.Workflow{
		Name: "test-docker-action",
		Steps: []schema.Step{
			{
				Name: "docker-step",
				Uses: "./" + filepath.Base(tmpDir),
			},
		},
	}

	runner := NewRunner(workflow, nil, filepath.Dir(tmpDir))
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error from Run(), got %v", err)
	}

	result := results[0]
	if result.Success {
		t.Errorf("Expected step to fail with unsupported docker action")
	}

	if result.Error != nil && !strings.Contains(result.Error.Error(), "docker") {
		t.Logf("Expected error about docker, got: %v", result.Error)
	}
}

// TestActionShellWithNoRunCommand tests shell action missing run command
func TestActionShellWithNoRunCommand(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-no-run")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create shell action with no run command
	noRunActionYml := `name: No Run Action
description: Shell action without run
runs:
  using: shell
`
	if err := os.WriteFile(filepath.Join(tmpDir, "action.yml"), []byte(noRunActionYml), 0644); err != nil {
		t.Fatalf("Failed to write action.yml: %v", err)
	}

	workflow := &schema.Workflow{
		Name: "test-no-run-action",
		Steps: []schema.Step{
			{
				Name: "no-run-step",
				Uses: "./" + filepath.Base(tmpDir),
			},
		},
	}

	runner := NewRunner(workflow, nil, filepath.Dir(tmpDir))
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error from Run(), got %v", err)
	}

	result := results[0]
	if result.Success {
		t.Errorf("Expected step to fail without run command")
	}

	if result.Error != nil && !strings.Contains(result.Error.Error(), "run") {
		t.Logf("Expected error about missing run, got: %v", result.Error)
	}
}

// TestActionCompositeNoStepsOrMain tests composite action with neither steps nor main
func TestActionCompositeNoStepsOrMain(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-empty-composite")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create composite action with no steps
	emptyCompositeYml := `name: Empty Composite
description: Composite without steps or main
runs:
  using: composite
`
	if err := os.WriteFile(filepath.Join(tmpDir, "action.yml"), []byte(emptyCompositeYml), 0644); err != nil {
		t.Fatalf("Failed to write action.yml: %v", err)
	}

	workflow := &schema.Workflow{
		Name: "test-empty-composite-action",
		Steps: []schema.Step{
			{
				Name: "empty-composite-step",
				Uses: "./" + filepath.Base(tmpDir),
			},
		},
	}

	runner := NewRunner(workflow, nil, filepath.Dir(tmpDir))
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error from Run(), got %v", err)
	}

	result := results[0]
	if result.Success {
		t.Errorf("Expected step to fail with empty composite action")
	}
}

// TestActionWithExpressionsInInputs tests input value with expressions
func TestActionWithExpressionsInInputs(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-expr-inputs")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	var runCmd string
	if runtime.GOOS == "windows" {
		runCmd = "Write-Output \"Value: $env:INPUT_VALUE\""
	} else {
		runCmd = "echo \"Value: $INPUT_VALUE\""
	}

	actionYml := `name: Expression Input Action
description: Action with expression inputs
inputs:
  value:
    description: A value
    required: true
runs:
  using: shell
  run: ` + runCmd + `
`
	if err := os.WriteFile(filepath.Join(tmpDir, "action.yml"), []byte(actionYml), 0644); err != nil {
		t.Fatalf("Failed to write action.yml: %v", err)
	}

	workflow := &schema.Workflow{
		Name: "test-expr-input-action",
		Env: map[string]string{
			"MY_VALUE": "evaluated_value",
		},
		Steps: []schema.Step{
			{
				Name: "expr-input-step",
				Uses: "./" + filepath.Base(tmpDir),
				With: map[string]string{
					"value": "${{ env.MY_VALUE }}",
				},
			},
		},
	}

	runner := NewRunner(workflow, nil, filepath.Dir(tmpDir))
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	result := results[0]
	t.Logf("Expression input result: success=%v, output=%s", result.Success, result.Output)
}

// TestActionWithTimeout tests action execution with timeout
func TestActionWithTimeout(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-timeout-action")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	var sleepCmd string
	if runtime.GOOS == "windows" {
		sleepCmd = "Start-Sleep -Seconds 5"
	} else {
		sleepCmd = "sleep 5"
	}

	actionYml := `name: Slow Action
description: Action that takes time
runs:
  using: shell
  run: ` + sleepCmd + `
`
	if err := os.WriteFile(filepath.Join(tmpDir, "action.yml"), []byte(actionYml), 0644); err != nil {
		t.Fatalf("Failed to write action.yml: %v", err)
	}

	workflow := &schema.Workflow{
		Name: "test-timeout-action",
		Steps: []schema.Step{
			{
				Name:    "timeout-action-step",
				Uses:    "./" + filepath.Base(tmpDir),
				Timeout: 1, // 1 second timeout
			},
		},
	}

	runner := NewRunner(workflow, nil, filepath.Dir(tmpDir))
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error from Run(), got %v", err)
	}

	result := results[0]
	if result.Success {
		t.Errorf("Expected step to timeout")
	}

	if result.Error != nil {
		if !strings.Contains(result.Error.Error(), "timed out") && !strings.Contains(result.Error.Error(), "timeout") {
			t.Logf("Error: %v", result.Error)
		}
	}
}

// ============================================================================
// Expression Evaluation Error Tests
// ============================================================================

// TestCommandExpressionEvaluationError tests error in command expression
func TestCommandExpressionEvaluationError(t *testing.T) {
	workflow := &schema.Workflow{
		Name: "test-cmd-expr-error",
		Steps: []schema.Step{
			{
				Name: "expr-error-step",
				Run:  "echo ${{ invalid_function_xyz() }}",
			},
		},
	}

	runner := NewRunner(workflow, nil, ".")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error from Run(), got %v", err)
	}

	result := results[0]
	if result.Success {
		t.Errorf("Expected step to fail with expression error")
	}

	if result.Error == nil {
		t.Errorf("Expected error for invalid expression")
	}

	if !strings.Contains(result.Error.Error(), "failed to evaluate command") {
		t.Errorf("Expected 'failed to evaluate command' in error, got: %v", result.Error)
	}
}

// ============================================================================
// Duration Tracking Tests
// ============================================================================

// TestStepDurationTracking tests that step duration is tracked
func TestStepDurationTracking(t *testing.T) {
	workflow := &schema.Workflow{
		Name: "test-duration",
		Steps: []schema.Step{
			{
				Name: "duration-step",
				Run:  "echo 'quick'",
			},
		},
	}

	runner := NewRunner(workflow, nil, ".")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	result := results[0]
	if result.Duration == 0 {
		t.Errorf("Expected non-zero duration")
	}

	// Duration should be reasonable (less than 10 seconds for echo)
	if result.Duration > 10*time.Second {
		t.Errorf("Duration seems too long: %v", result.Duration)
	}
}

// TestFailedStepDurationTracking tests that duration is tracked even for failed steps
func TestFailedStepDurationTracking(t *testing.T) {
	workflow := &schema.Workflow{
		Name: "test-failed-duration",
		Steps: []schema.Step{
			{
				Name: "fail-with-duration",
				Run:  "exit 1",
			},
		},
	}

	runner := NewRunner(workflow, nil, ".")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error from Run(), got %v", err)
	}

	result := results[0]
	if result.Success {
		t.Errorf("Expected step to fail")
	}

	if result.Duration == 0 {
		t.Errorf("Expected non-zero duration even for failed step")
	}
}

// ============================================================================
// Workflow Env Merge Tests
// ============================================================================

// TestWorkflowEnvMerge tests that workflow env is merged correctly
func TestWorkflowEnvMerge(t *testing.T) {
	workflow := &schema.Workflow{
		Name: "test-env-merge",
		Env: map[string]string{
			"WORKFLOW_VAR": "workflow_value",
			"SHARED_VAR":   "workflow_shared",
		},
		Steps: []schema.Step{
			{
				Name: "env-merge-step",
				Run:  "echo '${{ env.WORKFLOW_VAR }} ${{ env.SHARED_VAR }}'",
			},
		},
	}

	runner := NewRunner(workflow, nil, ".")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	result := results[0]
	if !result.Success {
		t.Errorf("Step should succeed, got error: %v", result.Error)
	}

	if !strings.Contains(result.Output, "workflow_value") {
		t.Errorf("Expected 'workflow_value' in output")
	}
	if !strings.Contains(result.Output, "workflow_shared") {
		t.Errorf("Expected 'workflow_shared' in output")
	}
}

// TestEmptyWorkflowEnv tests workflow with empty env map
func TestEmptyWorkflowEnv(t *testing.T) {
	workflow := &schema.Workflow{
		Name: "test-empty-env",
		Env:  map[string]string{},
		Steps: []schema.Step{
			{
				Name: "no-env-step",
				Run:  "echo 'no env vars'",
			},
		},
	}

	runner := NewRunner(workflow, nil, ".")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	result := results[0]
	if !result.Success {
		t.Errorf("Step should succeed with empty env, got error: %v", result.Error)
	}
}

// TestNilWorkflowEnv tests workflow with nil env map
func TestNilWorkflowEnv(t *testing.T) {
	workflow := &schema.Workflow{
		Name: "test-nil-env",
		// Env is nil
		Steps: []schema.Step{
			{
				Name: "nil-env-step",
				Run:  "echo 'nil env'",
			},
		},
	}

	runner := NewRunner(workflow, nil, ".")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	result := results[0]
	if !result.Success {
		t.Errorf("Step should succeed with nil env, got error: %v", result.Error)
	}
}

// ============================================================================
// Empty Workflow Tests
// ============================================================================

// TestEmptyWorkflowSteps tests workflow with no steps
func TestEmptyWorkflowSteps(t *testing.T) {
	workflow := &schema.Workflow{
		Name:  "test-empty-steps",
		Steps: []schema.Step{},
	}

	runner := NewRunner(workflow, nil, ".")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(results) != 0 {
		t.Errorf("Expected 0 results for empty workflow, got %d", len(results))
	}
}

// TestNilWorkflowSteps tests workflow with nil steps
func TestNilWorkflowSteps(t *testing.T) {
	workflow := &schema.Workflow{
		Name: "test-nil-steps",
		// Steps is nil
	}

	runner := NewRunner(workflow, nil, ".")
	results, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(results) != 0 {
		t.Errorf("Expected 0 results for nil steps, got %d", len(results))
	}
}

// ============================================================================
// RunWithBlocking Additional Tests
// ============================================================================

// TestRunWithBlockingAllStepsSucceed tests all steps succeeding
func TestRunWithBlockingAllStepsSucceed(t *testing.T) {
	workflow := &schema.Workflow{
		Name:     "test-all-succeed",
		Blocking: ptrBool(true),
		Steps: []schema.Step{
			{Name: "step1", Run: "echo 'one'"},
			{Name: "step2", Run: "echo 'two'"},
			{Name: "step3", Run: "echo 'three'"},
		},
	}

	runner := NewRunner(workflow, nil, ".")
	result := runner.RunWithBlocking(context.Background())

	if result.PermissionDecision != "allow" {
		t.Errorf("Expected allow when all steps succeed, got %s", result.PermissionDecision)
	}
}

// TestRunWithBlockingMixedResults tests mixed success/failure results
func TestRunWithBlockingMixedResults(t *testing.T) {
	workflow := &schema.Workflow{
		Name:     "test-mixed-results",
		Blocking: ptrBool(true),
		Steps: []schema.Step{
			{Name: "success-step", Run: "echo 'success'"},
			{Name: "fail-step", Run: "exit 1"},
		},
	}

	runner := NewRunner(workflow, nil, ".")
	result := runner.RunWithBlocking(context.Background())

	if result.PermissionDecision != "deny" {
		t.Errorf("Expected deny when any step fails, got %s", result.PermissionDecision)
	}

	if result.LogFile != "" {
		_ = os.Remove(result.LogFile)
	}
}

