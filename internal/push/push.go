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

// Response is the JSON response from a git push operation.
type Response struct {
	Status   Status           `json:"status"`
	Push     *PushPhaseResult `json:"push,omitempty"`
	PrePush  *PhaseResult     `json:"pre_push,omitempty"`
	PostPush *PostPushResult  `json:"post_push,omitempty"`
	Message  string           `json:"message"`
}

// PushPhaseResult contains the git push result.
type PushPhaseResult struct {
	Success bool   `json:"success"`
	Output  string `json:"output,omitempty"`
}

// PhaseResult contains the result of a workflow phase.
type PhaseResult struct {
	Passed       bool `json:"passed"`
	WorkflowsRun int  `json:"workflows_run"`
}

// PostPushResult contains the post-push phase result.
type PostPushResult struct {
	Passed       bool `json:"passed"`
	WorkflowsRun int  `json:"workflows_run"`
}

// Run executes the full 3-phase git push: pre-push workflows → git push → post-push workflows.
// The command runs synchronously and returns the complete result.
// If verbose is true, progress messages are written to stderr.
func Run(dir string, gitArgs []string, verbose bool) *Response {
	log := logging.Context("git-push")

	// Phase 1: Pre-push workflows
	if verbose {
		fmt.Fprintf(os.Stderr, "⏳ Phase 1/3: Running pre-push workflows...\n")
	}
	log.Info("phase 1: running pre-push workflows")

	prePushResult, err := runPushWorkflows(dir, "pre", verbose)
	if err != nil {
		return &Response{
			Status:  StatusFailed,
			PrePush: &PhaseResult{Passed: false, WorkflowsRun: 0},
			Message: fmt.Sprintf("Pre-push failed: %v", err),
		}
	}

	if !prePushResult.passed {
		return &Response{
			Status:  StatusFailed,
			PrePush: &PhaseResult{Passed: false, WorkflowsRun: prePushResult.workflowsRun},
			Message: "Pre-push workflows denied the push.\n\n" + prePushResult.details,
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
		return &Response{
			Status:  StatusFailed,
			PrePush: &PhaseResult{Passed: true, WorkflowsRun: prePushResult.workflowsRun},
			Push:    &PushPhaseResult{Success: false, Output: pushOutput},
			Message: fmt.Sprintf("Git push failed: %v\n\nOutput:\n%s", pushErr, pushOutput),
		}
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
			Status:   StatusFailed,
			PrePush:  &PhaseResult{Passed: true, WorkflowsRun: prePushResult.workflowsRun},
			Push:     &PushPhaseResult{Success: true, Output: pushOutput},
			PostPush: &PostPushResult{Passed: false, WorkflowsRun: 0},
			Message:  fmt.Sprintf("Post-push error: %v", err),
		}
	}

	postPushPassed := postPushResult.passed

	finalStatus := StatusCompleted
	message := "Push and all checks completed successfully."
	if !postPushPassed {
		finalStatus = StatusFailed
		message = "Push succeeded but post-push checks failed.\n\n" + postPushResult.details
	}

	return &Response{
		Status:   finalStatus,
		PrePush:  &PhaseResult{Passed: true, WorkflowsRun: prePushResult.workflowsRun},
		Push:     &PushPhaseResult{Success: true, Output: pushOutput},
		PostPush: &PostPushResult{Passed: postPushPassed, WorkflowsRun: postPushResult.workflowsRun},
		Message:  message,
	}
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
