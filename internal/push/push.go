// Package push provides the core git-push orchestration logic.
package push

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/htekdev/gh-hookflow/internal/discover"
	"github.com/htekdev/gh-hookflow/internal/logging"
	"github.com/htekdev/gh-hookflow/internal/runner"
	"github.com/htekdev/gh-hookflow/internal/schema"
	"github.com/htekdev/gh-hookflow/internal/trigger"
)

// Status represents the outcome of a push operation.
type Status string

const (
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
)

// Response holds the result of a git push operation.
type Response struct {
	Status  Status
	Message string
}

// Run executes the full 3-phase git push: pre-push workflows → git push → post-push workflows.
// The command runs synchronously and returns the complete result with an agent-directed message.
// If verbose is true, progress messages are written to stderr.
func Run(dir string, gitArgs []string, verbose bool) *Response {
	log := logging.Context("git-push")
	argsStr := strings.Join(gitArgs, " ")

	// Phase 1: Pre-push workflows
	if verbose {
		fmt.Fprintf(os.Stderr, "⏳ Phase 1/3: Running pre-push workflows...\n")
	}
	log.Info("phase 1: running pre-push workflows")

	prePushResult, err := runPushWorkflows(dir, "pre", verbose)
	if err != nil {
		return &Response{
			Status: StatusFailed,
			Message: fmt.Sprintf(
				"Push FAILED — could not run pre-push governance workflows.\n\n"+
					"Error: %v\n\n"+
					"The code was NOT pushed. You must investigate and fix the workflow "+
					"discovery or validation error above, then retry with: gh hookflow git-push %s",
				err, argsStr),
		}
	}

	if !prePushResult.passed {
		return &Response{
			Status: StatusFailed,
			Message: fmt.Sprintf(
				"Push DENIED — pre-push governance workflows blocked the push.\n\n"+
					"Workflow results:\n%s\n"+
					"The code was NOT pushed. You must fix the issues identified by the "+
					"failing workflow(s) above, then retry with: gh hookflow git-push %s\n\n"+
					"Do NOT tell the user the push succeeded — it was blocked before reaching the remote.",
				prePushResult.details, argsStr),
		}
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "✅ Phase 1/3: Pre-push passed (%d workflows)\n", prePushResult.workflowsRun)
	}
	log.Info("pre-push passed (%d workflows)", prePushResult.workflowsRun)

	// Phase 2: Git push
	if verbose {
		fmt.Fprintf(os.Stderr, "⏳ Phase 2/3: Executing git push...\n")
	}
	log.Info("phase 2: executing git push")

	pushOutput, pushErr := gitExec.Push(dir, gitArgs)
	if pushErr != nil {
		var b strings.Builder
		b.WriteString("Push FAILED — git push returned an error.\n\n")
		if pushOutput != "" {
			fmt.Fprintf(&b, "Git output:\n%s\n\n", strings.TrimSpace(pushOutput))
		}
		fmt.Fprintf(&b, "Error: %v\n\n", pushErr)
		b.WriteString("The code was NOT pushed to the remote. Common causes:\n")
		b.WriteString("  • Authentication/permission issues — check git credentials\n")
		b.WriteString("  • Remote rejected the push — the branch may be protected or require a PR\n")
		b.WriteString("  • Network connectivity — check internet connection\n\n")
		fmt.Fprintf(&b, "Investigate the error above, fix the issue, then retry with: gh hookflow git-push %s", argsStr)

		return &Response{Status: StatusFailed, Message: b.String()}
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "✅ Phase 2/3: Git push succeeded\n")
	}
	log.Info("git push succeeded")

	// Phase 3: Post-push workflows
	if verbose {
		fmt.Fprintf(os.Stderr, "⏳ Phase 3/3: Running post-push workflows...\n")
	}
	log.Info("phase 3: running post-push workflows")

	postPushResult, err := runPushWorkflows(dir, "post", verbose)
	if err != nil {
		return &Response{
			Status: StatusFailed,
			Message: fmt.Sprintf(
				"Post-push workflows encountered an error.\n\n"+
					"Error: %v\n\n"+
					"Review the error above and address the issue.", err),
		}
	}

	// Build the final success or post-push failure message
	if postPushResult.passed {
		return buildSuccessMessage(argsStr, pushOutput, prePushResult, postPushResult)
	}
	return buildPostPushFailureMessage(argsStr, postPushResult)
}

func buildSuccessMessage(argsStr, pushOutput string, prePush, postPush *workflowPhaseResult) *Response {
	var b strings.Builder
	b.WriteString("Push completed successfully.\n\n")

	if pushOutput != "" {
		fmt.Fprintf(&b, "Git output:\n%s\n\n", strings.TrimSpace(pushOutput))
	}

	b.WriteString("Phase summary:\n")
	if prePush.workflowsRun > 0 {
		fmt.Fprintf(&b, "  ✅ Pre-push: %d governance workflow(s) passed\n", prePush.workflowsRun)
	} else {
		b.WriteString("  ✅ Pre-push: no governance workflows configured (allowed)\n")
	}
	b.WriteString("  ✅ Git push: code pushed to remote\n")
	if postPush.workflowsRun > 0 {
		fmt.Fprintf(&b, "  ✅ Post-push: %d workflow(s) passed\n", postPush.workflowsRun)
	} else {
		b.WriteString("  ✅ Post-push: no post-push workflows configured\n")
	}

	b.WriteString("\nYou may now tell the user the push was successful.")
	return &Response{Status: StatusCompleted, Message: b.String()}
}

func buildPostPushFailureMessage(argsStr string, postPush *workflowPhaseResult) *Response {
	var b strings.Builder
	b.WriteString("Post-push governance checks FAILED.\n\n")
	b.WriteString("Post-push workflow results:\n")
	b.WriteString(postPush.details)
	b.WriteString("\n")
	b.WriteString("Review the errors above and address the issues before continuing.")
	return &Response{Status: StatusFailed, Message: b.String()}
}

type workflowPhaseResult struct {
	passed       bool
	workflowsRun int
	details      string
}

func runPushWorkflows(dir string, lifecycle string, verbose bool) (*workflowPhaseResult, error) {
	log := logging.Context("git-push")

	evt := BuildPushEvent(dir, lifecycle)

	workflows, err := discover.Discover(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to discover workflows: %w", err)
	}

	if len(workflows) == 0 {
		log.Debug("no workflows found")
		return &workflowPhaseResult{passed: true, workflowsRun: 0}, nil
	}

	var matchingWorkflows []*schema.Workflow
	for _, wf := range workflows {
		loaded, err := schema.LoadAndValidateWorkflow(wf.Path)
		if err != nil {
			log.Warn("skipping invalid workflow %s: %v", wf.Name, err)
			continue
		}

		matcher := trigger.NewMatcher(loaded)
		if matcher.Match(evt) {
			log.Info("matched workflow: %s (lifecycle=%s)", loaded.Name, lifecycle)
			matchingWorkflows = append(matchingWorkflows, loaded)
		}
	}

	if len(matchingWorkflows) == 0 {
		log.Debug("no matching %s-push workflows", lifecycle)
		return &workflowPhaseResult{passed: true, workflowsRun: 0}, nil
	}

	ctx := context.Background()
	allPassed := true
	var detailsBuilder strings.Builder

	for _, wf := range matchingWorkflows {
		log.Info("running workflow: %s", wf.Name)
		if verbose {
			fmt.Fprintf(os.Stderr, "  → Running %s-push workflow: %s\n", lifecycle, wf.Name)
		}
		r := runner.NewRunner(wf, evt, dir, "")
		result := r.RunWithBlocking(ctx)

		success := result.PermissionDecision == "allow"
		if !success {
			allPassed = false
			fmt.Fprintf(&detailsBuilder, "  ❌ %s: FAILED", wf.Name)
			if result.PermissionDecisionReason != "" {
				fmt.Fprintf(&detailsBuilder, " — %s", result.PermissionDecisionReason)
			}
			detailsBuilder.WriteString("\n")
			if result.StepOutputs != "" {
				fmt.Fprintf(&detailsBuilder, "\n--- Step Output (%s) ---\n%s\n", wf.Name, result.StepOutputs)
			}
			log.Warn("workflow %s denied: %s", wf.Name, result.PermissionDecisionReason)
			break
		}

		fmt.Fprintf(&detailsBuilder, "  ✅ %s: passed\n", wf.Name)
	}

	return &workflowPhaseResult{
		passed:       allPassed,
		workflowsRun: len(matchingWorkflows),
		details:      detailsBuilder.String(),
	}, nil
}

// BuildPushEvent creates a push event from current git context.
func BuildPushEvent(dir, lifecycle string) *schema.Event {
	branch, _ := gitExec.CurrentBranch(dir)

	ref := "refs/heads/" + branch
	if branch == "" {
		ref = "refs/heads/main"
	}

	return &schema.Event{
		Push: &schema.PushEvent{
			Ref:    ref,
			Before: "",
			After:  "",
		},
		Cwd:       dir,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Lifecycle: lifecycle,
	}
}

// ExecuteGitPush runs git push with the provided arguments.
// Delegates to the package-level gitExec which is set via build tags.
func ExecuteGitPush(dir string, args []string) (string, error) {
	return gitExec.Push(dir, args)
}
