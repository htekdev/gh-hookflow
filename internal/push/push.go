// Package push provides the core git-push orchestration logic.
// Used by both the CLI command (cmd/hookflow/git_push.go) and the MCP
// server (internal/mcp/git_push.go).
package push

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/htekdev/gh-hookflow/internal/activity"
	"github.com/htekdev/gh-hookflow/internal/discover"
	"github.com/htekdev/gh-hookflow/internal/logging"
	"github.com/htekdev/gh-hookflow/internal/runner"
	"github.com/htekdev/gh-hookflow/internal/schema"
	"github.com/htekdev/gh-hookflow/internal/trigger"
)

// Response is the JSON response from a git push operation
type Response struct {
	ActivityID string            `json:"activity_id"`
	Status     activity.Status   `json:"status"`
	Push       *PushPhaseResult  `json:"push,omitempty"`
	PrePush    *PhaseResult      `json:"pre_push,omitempty"`
	PostPush   *PostPushResult   `json:"post_push,omitempty"`
	Message    string            `json:"message"`
}

// PushPhaseResult contains the git push result
type PushPhaseResult struct {
	Success bool   `json:"success"`
	Output  string `json:"output,omitempty"`
}

// PhaseResult contains the result of a workflow phase
type PhaseResult struct {
	Passed       bool `json:"passed"`
	WorkflowsRun int  `json:"workflows_run"`
}

// PostPushResult contains the post-push phase result
type PostPushResult struct {
	Passed       bool `json:"passed"`
	WorkflowsRun int  `json:"workflows_run"`
}

// Run executes the full 3-phase git push: pre-push workflows → git push → post-push workflows.
// It updates the activity state on disk as it progresses.
// If verbose is true, progress messages are written to stderr.
func Run(dir string, gitArgs []string, act *activity.Activity, verbose bool) *Response {
	log := logging.Context("git-push")

	// Phase 1: Pre-push workflows
	if verbose {
		fmt.Fprintf(os.Stderr, "⏳ Phase 1/3: Running pre-push workflows...\n")
	}
	log.Info("phase 1: running pre-push workflows")
	act.StartPhase(activity.PhasePrePush)

	prePushResult, err := runPushWorkflows(dir, act, "pre", verbose)
	if err != nil {
		act.FailPhase(activity.PhasePrePush, err.Error())
		act.Complete(activity.StatusFailed, "Pre-push phase failed: "+err.Error())
		return &Response{
			ActivityID: act.ID,
			Status:     activity.StatusFailed,
			PrePush:    &PhaseResult{Passed: false, WorkflowsRun: 0},
			Message:    fmt.Sprintf("Pre-push failed: %v", err),
		}
	}

	if !prePushResult.passed {
		act.CompletePhase(activity.PhasePrePush, false, "workflows denied")
		act.Complete(activity.StatusFailed, "Pre-push workflows denied the push")
		return &Response{
			ActivityID: act.ID,
			Status:     activity.StatusFailed,
			PrePush:    &PhaseResult{Passed: false, WorkflowsRun: prePushResult.workflowsRun},
			Message:    "Pre-push workflows denied the push. Check workflow logs for details.",
		}
	}

	act.CompletePhase(activity.PhasePrePush, true, fmt.Sprintf("%d workflows passed", prePushResult.workflowsRun))
	if verbose {
		fmt.Fprintf(os.Stderr, "✅ Phase 1/3: Pre-push passed (%d workflows)\n", prePushResult.workflowsRun)
	}
	log.Info("pre-push passed (%d workflows)", prePushResult.workflowsRun)

	// Phase 2: Git push
	if verbose {
		fmt.Fprintf(os.Stderr, "⏳ Phase 2/3: Executing git push...\n")
	}
	log.Info("phase 2: executing git push")
	act.StartPhase(activity.PhasePush)

	pushOutput, pushErr := ExecuteGitPush(dir, gitArgs)
	if pushErr != nil {
		act.FailPhase(activity.PhasePush, pushErr.Error())
		act.Complete(activity.StatusFailed, "Git push failed: "+pushErr.Error())
		_ = act.WriteLog(activity.PhasePush, "git-push", pushOutput)
		return &Response{
			ActivityID: act.ID,
			Status:     activity.StatusFailed,
			PrePush:    &PhaseResult{Passed: true, WorkflowsRun: prePushResult.workflowsRun},
			Push:       &PushPhaseResult{Success: false, Output: pushOutput},
			Message:    fmt.Sprintf("Git push failed: %v", pushErr),
		}
	}

	act.CompletePhase(activity.PhasePush, true, pushOutput)
	_ = act.WriteLog(activity.PhasePush, "git-push", pushOutput)
	if verbose {
		fmt.Fprintf(os.Stderr, "✅ Phase 2/3: Git push succeeded\n")
	}
	log.Info("git push succeeded")

	// Phase 3: Post-push workflows
	if verbose {
		fmt.Fprintf(os.Stderr, "⏳ Phase 3/3: Running post-push workflows (this may take several minutes if monitoring CI checks)...\n")
	}
	log.Info("phase 3: running post-push workflows")
	act.StartPhase(activity.PhasePostPush)

	postPushResult, err := runPushWorkflows(dir, act, "post", verbose)
	if err != nil {
		act.FailPhase(activity.PhasePostPush, err.Error())
		act.Complete(activity.StatusFailed, "Post-push phase error: "+err.Error())
		return &Response{
			ActivityID: act.ID,
			Status:     activity.StatusFailed,
			PrePush:    &PhaseResult{Passed: true, WorkflowsRun: prePushResult.workflowsRun},
			Push:       &PushPhaseResult{Success: true, Output: pushOutput},
			PostPush:   &PostPushResult{Passed: false, WorkflowsRun: 0},
			Message:    fmt.Sprintf("Post-push error: %v", err),
		}
	}

	postPushPassed := postPushResult.passed
	act.CompletePhase(activity.PhasePostPush, postPushPassed, fmt.Sprintf("%d workflows completed", postPushResult.workflowsRun))

	finalStatus := activity.StatusCompleted
	message := "Push and all checks completed successfully."
	if !postPushPassed {
		finalStatus = activity.StatusFailed
		message = "Push succeeded but post-push checks failed."
	}

	act.Complete(finalStatus, message)

	return &Response{
		ActivityID: act.ID,
		Status:     finalStatus,
		PrePush:    &PhaseResult{Passed: true, WorkflowsRun: prePushResult.workflowsRun},
		Push:       &PushPhaseResult{Success: true, Output: pushOutput},
		PostPush:   &PostPushResult{Passed: postPushPassed, WorkflowsRun: postPushResult.workflowsRun},
		Message:    message,
	}
}

type workflowPhaseResult struct {
	passed       bool
	workflowsRun int
}

func runPushWorkflows(dir string, act *activity.Activity, lifecycle string, verbose bool) (*workflowPhaseResult, error) {
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
	phase := LifecycleToPhase(lifecycle)

	for _, wf := range matchingWorkflows {
		log.Info("running workflow: %s", wf.Name)
		if verbose {
			fmt.Fprintf(os.Stderr, "  → Running %s-push workflow: %s\n", lifecycle, wf.Name)
		}
		r := runner.NewRunner(wf, evt, dir)
		result := r.RunWithBlocking(ctx)

		success := result.PermissionDecision == "allow"
		errMsg := ""
		if !success {
			errMsg = result.PermissionDecisionReason
			allPassed = false
		}

		act.AddWorkflowResult(phase, wf.Name, success, errMsg)

		logContent := fmt.Sprintf("Workflow: %s\nDecision: %s\n", wf.Name, result.PermissionDecision)
		if result.PermissionDecisionReason != "" {
			logContent += fmt.Sprintf("Reason: %s\n", result.PermissionDecisionReason)
		}
		if result.StepOutputs != "" {
			logContent += fmt.Sprintf("\n--- Step Output ---\n%s\n", result.StepOutputs)
		}
		if result.LogFile != "" {
			if data, err := os.ReadFile(result.LogFile); err == nil {
				logContent += fmt.Sprintf("\n--- Detailed Logs ---\n%s\n", string(data))
			}
		}
		_ = act.WriteLog(phase, wf.Name, logContent)

		if !success {
			log.Warn("workflow %s denied: %s", wf.Name, errMsg)
			break
		}
	}

	return &workflowPhaseResult{
		passed:       allPassed,
		workflowsRun: len(matchingWorkflows),
	}, nil
}

// BuildPushEvent creates a push event from current git context
func BuildPushEvent(dir, lifecycle string) *schema.Event {
	gitProvider := &gitContextProvider{cwd: dir}

	branch := gitProvider.getBranch()
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

type gitContextProvider struct {
	cwd string
}

func (g *gitContextProvider) getBranch() string {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = g.cwd
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// ExecuteGitPush runs git push with the provided arguments
func ExecuteGitPush(dir string, args []string) (string, error) {
	cmdArgs := append([]string{"push"}, args...)
	cmd := exec.Command("git", cmdArgs...)
	cmd.Dir = dir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	output := stdout.String()
	if stderr.Len() > 0 {
		if output != "" {
			output += "\n"
		}
		output += stderr.String()
	}

	if err != nil {
		return output, fmt.Errorf("git push failed: %w\n%s", err, output)
	}

	return output, nil
}

// LifecycleToPhase converts a lifecycle string to an activity Phase
func LifecycleToPhase(lifecycle string) activity.Phase {
	switch lifecycle {
	case "pre":
		return activity.PhasePrePush
	case "post":
		return activity.PhasePostPush
	default:
		return activity.PhasePrePush
	}
}
