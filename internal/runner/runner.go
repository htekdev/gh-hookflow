package runner

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/htekdev/gh-hookflow/internal/expression"
	"github.com/htekdev/gh-hookflow/internal/schema"
	"github.com/htekdev/gh-hookflow/internal/session"
)

// Runner executes workflow steps
type Runner struct {
	workflow   *schema.Workflow
	event      *schema.Event
	exprCtx    *expression.Context
	workingDir string
	env        map[string]string
}

// StepResult contains the result of running a step
type StepResult struct {
	Name            string
	Success         bool
	Output          string
	Error           error
	Duration        time.Duration
	ContinueOnError bool
}

// NewRunner creates a new step runner
func NewRunner(workflow *schema.Workflow, event *schema.Event, workingDir string) *Runner {
	exprCtx := expression.NewContext()

	// Populate event context
	if event != nil {
		exprCtx.Event["cwd"] = event.Cwd
		exprCtx.Event["timestamp"] = event.Timestamp

		if event.Hook != nil {
			exprCtx.Event["hook"] = map[string]interface{}{
				"type": event.Hook.Type,
				"cwd":  event.Hook.Cwd,
			}
			if event.Hook.Tool != nil {
				exprCtx.Event["hook"].(map[string]interface{})["tool"] = map[string]interface{}{
					"name": event.Hook.Tool.Name,
					"args": event.Hook.Tool.Args,
				}
			}
		}

		if event.Tool != nil {
			exprCtx.Event["tool"] = map[string]interface{}{
				"name":      event.Tool.Name,
				"args":      event.Tool.Args,
				"hook_type": event.Tool.HookType,
			}
		}

		if event.File != nil {
			exprCtx.Event["file"] = map[string]interface{}{
				"path":    event.File.Path,
				"action":  event.File.Action,
				"content": event.File.Content,
			}
		}

		if event.Commit != nil {
			files := make([]map[string]string, len(event.Commit.Files))
			for i, f := range event.Commit.Files {
				files[i] = map[string]string{"path": f.Path, "status": f.Status}
			}
			exprCtx.Event["commit"] = map[string]interface{}{
				"sha":     event.Commit.SHA,
				"message": event.Commit.Message,
				"author":  event.Commit.Author,
				"files":   files,
			}
		}

		if event.Push != nil {
			exprCtx.Event["push"] = map[string]interface{}{
				"ref":    event.Push.Ref,
				"before": event.Push.Before,
				"after":  event.Push.After,
			}
		}
	}

	// Merge workflow env with event env
	env := make(map[string]string)
	for k, v := range workflow.Env {
		env[k] = v
	}
	exprCtx.Env = env

	return &Runner{
		workflow:   workflow,
		event:      event,
		exprCtx:    exprCtx,
		workingDir: workingDir,
		env:        env,
	}
}

// Run executes all steps in the workflow
func (r *Runner) Run(ctx context.Context) ([]StepResult, error) {
	var results []StepResult
	var prevStepFailed bool

	for i, step := range r.workflow.Steps {
		stepName := step.Name
		if stepName == "" {
			stepName = fmt.Sprintf("Step %d", i+1)
		}

		// Update step context for expressions
		r.exprCtx.Steps[stepName] = expression.StepContext{
			Outputs: make(map[string]string),
			Outcome: "pending",
		}

		// Check if condition
		if step.If != "" {
			// Evaluate if condition
			shouldRun, err := r.exprCtx.EvaluateBool(step.If)
			if err != nil {
				results = append(results, StepResult{
					Name:            stepName,
					Success:         false,
					Error:           fmt.Errorf("failed to evaluate if condition: %w", err),
					ContinueOnError: step.ContinueOnError,
				})
				if !step.ContinueOnError {
					prevStepFailed = true
				}
				continue
			}
			if !shouldRun {
				results = append(results, StepResult{
					Name:    stepName,
					Success: true,
					Output:  "Skipped (condition not met)",
				})
				continue
			}
		}

		// If previous step failed and this doesn't have always(), skip
		if prevStepFailed && !strings.Contains(step.If, "always()") {
			results = append(results, StepResult{
				Name:    stepName,
				Success: false,
				Output:  "Skipped (previous step failed)",
			})
			continue
		}

		// Execute the step
		result := r.runStep(ctx, step, stepName)
		result.ContinueOnError = step.ContinueOnError
		results = append(results, result)

		// Update step context
		outcome := "success"
		if !result.Success {
			outcome = "failure"
			if !step.ContinueOnError {
				prevStepFailed = true
			}
		}
		r.exprCtx.Steps[stepName] = expression.StepContext{
			Outputs: make(map[string]string),
			Outcome: outcome,
		}
	}

	return results, nil
}

// RunWithBlocking executes all steps and returns a WorkflowResult based on blocking mode
// If blocking=true and any step fails, returns a deny result with detailed logs
// If blocking=false, returns an allow result even if steps fail (logs warnings instead)
func (r *Runner) RunWithBlocking(ctx context.Context) *schema.WorkflowResult {
	results, err := r.Run(ctx)
	if err != nil {
		if r.workflow.IsBlocking() {
			return schema.NewDenyResult(fmt.Sprintf("workflow execution error: %v", err))
		}
		log.Printf("Warning: workflow execution error (non-blocking): %v", err)
		return schema.NewAllowResult()
	}

	// Check if any step failed (respecting continue-on-error)
	anyStepFailed := false
	for _, result := range results {
		if !result.Success && !result.ContinueOnError {
			anyStepFailed = true
			break
		}
	}

	// If no failures, always allow
	if !anyStepFailed {
		return schema.NewAllowResult()
	}

	// Steps failed - decision depends on blocking mode
	if r.workflow.IsBlocking() {
		// Blocking mode: deny on any failure with detailed logs
		logFile, reason := r.buildDenialWithLogs(results)
		result := schema.NewDenyResult(reason)
		if logFile != "" {
			result.LogFile = logFile
		}

		// Record error for postToolUse workflows so preToolUse can block until acknowledged
		if r.isPostToolUseEvent() {
			r.recordPostToolUseError(results)
		}

		return result
	}

	// Non-blocking mode: log warnings but allow
	for _, result := range results {
		if !result.Success {
			log.Printf("Warning: step '%s' failed (non-blocking): %v", result.Name, result.Error)
		}
	}
	return schema.NewAllowResult()
}

// buildDenialWithLogs creates a detailed log file and returns the path and denial reason
func (r *Runner) buildDenialWithLogs(results []StepResult) (logFile string, reason string) {
	var failedSteps []string
	var logContent strings.Builder

	// Header
	fmt.Fprintf(&logContent, "Workflow: %s\n", r.workflow.Name)
	fmt.Fprintf(&logContent, "Description: %s\n", r.workflow.Description)
	fmt.Fprintf(&logContent, "Time: %s\n", time.Now().Format(time.RFC3339))
	logContent.WriteString(strings.Repeat("=", 60) + "\n\n")

	// Write each step's result
	for _, result := range results {
		fmt.Fprintf(&logContent, "Step: %s\n", result.Name)
		fmt.Fprintf(&logContent, "Status: %s\n", map[bool]string{true: "✓ SUCCESS", false: "✗ FAILED"}[result.Success])
		if result.Duration > 0 {
			fmt.Fprintf(&logContent, "Duration: %s\n", result.Duration.Round(time.Millisecond))
		}
		if result.Error != nil {
			fmt.Fprintf(&logContent, "Error: %v\n", result.Error)
		}
		if result.Output != "" {
			logContent.WriteString("Output:\n")
			// Indent the output
			for _, line := range strings.Split(strings.TrimSpace(result.Output), "\n") {
				logContent.WriteString("  " + line + "\n")
			}
		}
		logContent.WriteString(strings.Repeat("-", 40) + "\n\n")

		if !result.Success {
			failedSteps = append(failedSteps, result.Name)
		}
	}

	// Write to temp file
	tmpFile, err := os.CreateTemp("", "hookflow-*.log")
	if err != nil {
		// Can't create temp file, return reason without log file
		return "", fmt.Sprintf("workflow '%s' blocked due to step failures: %s", r.workflow.Name, strings.Join(failedSteps, ", "))
	}
	defer func() { _ = tmpFile.Close() }()

	_, err = tmpFile.WriteString(logContent.String())
	if err != nil {
		return "", fmt.Sprintf("workflow '%s' blocked due to step failures: %s", r.workflow.Name, strings.Join(failedSteps, ", "))
	}

	logFile = tmpFile.Name()

	// Build detailed reason message
	var reasonBuilder strings.Builder
	fmt.Fprintf(&reasonBuilder, "Workflow '%s' blocked.\n\n", r.workflow.Name)
	reasonBuilder.WriteString("Failed steps:\n")
	for _, result := range results {
		if !result.Success {
			fmt.Fprintf(&reasonBuilder, "  • %s", result.Name)
			if result.Error != nil {
				fmt.Fprintf(&reasonBuilder, ": %v", result.Error)
			}
			reasonBuilder.WriteString("\n")
			// Include brief output snippet (first 200 chars)
			if result.Output != "" {
				output := strings.TrimSpace(result.Output)
				if len(output) > 200 {
					output = output[:200] + "..."
				}
				fmt.Fprintf(&reasonBuilder, "    Output: %s\n", strings.ReplaceAll(output, "\n", " "))
			}
		}
	}
	fmt.Fprintf(&reasonBuilder, "\nFull logs: %s", logFile)

	return logFile, reasonBuilder.String()
}

// runStep executes a single step
func (r *Runner) runStep(ctx context.Context, step schema.Step, name string) StepResult {
	start := time.Now()

	// Handle timeout
	if step.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(step.Timeout)*time.Second)
		defer cancel()
	}

	// Check for uses: action
	if step.Uses != "" {
		return r.runAction(ctx, step, name, start)
	}

	// Execute run: command
	if step.Run != "" {
		return r.runCommand(ctx, step, name, start)
	}

	return StepResult{
		Name:     name,
		Success:  false,
		Error:    fmt.Errorf("step has neither 'run' nor 'uses'"),
		Duration: time.Since(start),
	}
}

// runCommand executes a shell command
func (r *Runner) runCommand(ctx context.Context, step schema.Step, name string, start time.Time) StepResult {
	// Evaluate expressions in command
	command, err := r.exprCtx.EvaluateString(step.Run)
	if err != nil {
		return StepResult{
			Name:     name,
			Success:  false,
			Error:    fmt.Errorf("failed to evaluate command: %w", err),
			Duration: time.Since(start),
		}
	}

	// Determine shell
	shell := step.Shell
	if shell == "" {
		shell = defaultShell()
	}

	// Build command
	var cmd *exec.Cmd
	switch shell {
	case "pwsh", "powershell":
		// Check if pwsh is available
		if _, err := exec.LookPath("pwsh"); err != nil {
			return StepResult{
				Name:    name,
				Success: false,
				Error: fmt.Errorf("pwsh (PowerShell Core) not found. Install it from: https://github.com/PowerShell/PowerShell/releases\n" +
					"  Windows: winget install Microsoft.PowerShell\n" +
					"  macOS: brew install powershell\n" +
					"  Linux: https://learn.microsoft.com/en-us/powershell/scripting/install/installing-powershell-on-linux"),
				Duration: time.Since(start),
			}
		}
		cmd = exec.CommandContext(ctx, "pwsh", "-NoProfile", "-NonInteractive", "-Command", command)
	case "bash":
		cmd = exec.CommandContext(ctx, "bash", "-c", command)
	case "sh":
		cmd = exec.CommandContext(ctx, "sh", "-c", command)
	case "cmd":
		cmd = exec.CommandContext(ctx, "cmd", "/c", command)
	default:
		cmd = exec.CommandContext(ctx, shell, "-c", command)
	}

	// Set working directory
	workDir := r.workingDir
	if step.WorkingDirectory != "" {
		wd, err := r.exprCtx.EvaluateString(step.WorkingDirectory)
		if err == nil {
			workDir = wd
		}
	}
	cmd.Dir = workDir

	// Set environment
	cmd.Env = os.Environ()
	for k, v := range r.env {
		val, _ := r.exprCtx.EvaluateString(v)
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, val))
	}
	for k, v := range step.Env {
		val, _ := r.exprCtx.EvaluateString(v)
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, val))
	}

	// Capture output
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run command
	err = cmd.Run()

	output := stdout.String()
	if stderr.Len() > 0 {
		output += "\n" + stderr.String()
	}

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return StepResult{
				Name:     name,
				Success:  false,
				Output:   output,
				Error:    fmt.Errorf("step timed out after %d seconds", step.Timeout),
				Duration: time.Since(start),
			}
		}
		return StepResult{
			Name:     name,
			Success:  false,
			Output:   output,
			Error:    err,
			Duration: time.Since(start),
		}
	}

	return StepResult{
		Name:     name,
		Success:  true,
		Output:   output,
		Duration: time.Since(start),
	}
}

// runAction executes a reusable action
func (r *Runner) runAction(ctx context.Context, step schema.Step, name string, start time.Time) StepResult {
	// Parse the uses: string
	parsed, err := parseUsesString(step.Uses)
	if err != nil {
		return StepResult{
			Name:     name,
			Success:  false,
			Error:    fmt.Errorf("failed to parse uses: %w", err),
			Duration: time.Since(start),
		}
	}

	// Resolve the action path
	actionDir, err := r.resolveActionPath(ctx, parsed)
	if err != nil {
		return StepResult{
			Name:     name,
			Success:  false,
			Error:    fmt.Errorf("failed to resolve action: %w", err),
			Duration: time.Since(start),
		}
	}

	// Load action metadata
	metadata, err := loadActionMetadata(actionDir)
	if err != nil {
		return StepResult{
			Name:     name,
			Success:  false,
			Error:    fmt.Errorf("failed to load action metadata: %w", err),
			Duration: time.Since(start),
		}
	}

	// Evaluate inputs
	inputs, err := r.evaluateInputs(step.With)
	if err != nil {
		return StepResult{
			Name:     name,
			Success:  false,
			Error:    fmt.Errorf("failed to evaluate inputs: %w", err),
			Duration: time.Since(start),
		}
	}

	// Execute the action
	output, err := r.executeAction(ctx, actionDir, metadata, inputs)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return StepResult{
				Name:     name,
				Success:  false,
				Output:   output,
				Error:    fmt.Errorf("action timed out"),
				Duration: time.Since(start),
			}
		}
		return StepResult{
			Name:     name,
			Success:  false,
			Output:   output,
			Error:    err,
			Duration: time.Since(start),
		}
	}

	return StepResult{
		Name:     name,
		Success:  true,
		Output:   output,
		Duration: time.Since(start),
	}
}

// defaultShell returns the default shell for workflows
// We standardize on PowerShell Core (pwsh) for cross-platform consistency
func defaultShell() string {
	return "pwsh"
}

// isPostToolUseEvent returns true if this workflow was triggered by a postToolUse event
func (r *Runner) isPostToolUseEvent() bool {
	if r.event == nil {
		return false
	}

	// Check event lifecycle
	if r.event.GetLifecycle() == "post" {
		return true
	}

	// Check hook type
	if r.event.Hook != nil && r.event.Hook.Type == "postToolUse" {
		return true
	}

	return false
}

// recordPostToolUseError writes a session error for failed postToolUse workflows
func (r *Runner) recordPostToolUseError(results []StepResult) {
	// Find the first failed step
	var failedStep *StepResult
	for i := range results {
		if !results[i].Success {
			failedStep = &results[i]
			break
		}
	}

	if failedStep == nil {
		return
	}

	// Build error details
	var details strings.Builder
	if failedStep.Error != nil {
		fmt.Fprintf(&details, "Error: %v\n", failedStep.Error)
	}
	if failedStep.Output != "" {
		fmt.Fprintf(&details, "\nOutput:\n```\n%s\n```", strings.TrimSpace(failedStep.Output))
	}

	// Write to session error file
	if err := session.WriteError(r.workflow.Name, failedStep.Name, details.String()); err != nil {
		log.Printf("Warning: failed to record postToolUse error: %v", err)
	}
}
