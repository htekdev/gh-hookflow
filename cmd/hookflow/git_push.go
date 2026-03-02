package main

import (
	"bytes"
	"context"
	"encoding/json"
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
	"github.com/spf13/cobra"
)

// GitPushResponse is the JSON response from hookflow git-push
type GitPushResponse struct {
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

var gitPushCmd = &cobra.Command{
	Use:   "git-push [git push args...]",
	Short: "Push with pre/post workflow validation",
	Long: `Performs a git push with hookflow workflow orchestration.

This command:
1. Runs pre-push hookflows (on.push, lifecycle: pre)
2. Executes git push with the provided arguments
3. Runs post-push hookflows (on.push, lifecycle: post)
4. Returns a JSON status with an activity ID for tracking

If post-push workflows are long-running, the command will wait for them to complete
before returning.

Examples:
  hookflow git-push origin main
  hookflow git-push origin feature/my-branch --force
  hookflow git-push                          # uses default remote and branch`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, _ := cmd.Flags().GetString("dir")
		if dir == "" {
			var err error
			dir, err = os.Getwd()
			if err != nil {
				return err
			}
		}

		return runGitPush(dir, args)
	},
}

func init() {
	gitPushCmd.Flags().StringP("dir", "d", "", "Working directory (default: current directory)")
}

func runGitPush(dir string, gitArgs []string) error {
	log := logging.Context("git-push")
	done := logging.StartOperation("git-push", fmt.Sprintf("args=%v", gitArgs))

	// Clean up old activities in the background
	go func() { _ = activity.CleanupOldActivities(7 * 24 * time.Hour) }()

	// Create a new activity
	act, err := activity.NewActivity(gitArgs)
	if err != nil {
		done(err)
		return fmt.Errorf("failed to create activity: %w", err)
	}
	log.Info("created activity %s for git push %v", act.ID, gitArgs)

	// Phase 1: Pre-push workflows
	log.Info("phase 1: running pre-push workflows")
	act.StartPhase(activity.PhasePrePush)

	prePushResult, err := runPushWorkflows(dir, act, "pre")
	if err != nil {
		act.FailPhase(activity.PhasePrePush, err.Error())
		act.Complete(activity.StatusFailed, "Pre-push phase failed: "+err.Error())
		return outputGitPushResponse(&GitPushResponse{
			ActivityID: act.ID,
			Status:     activity.StatusFailed,
			PrePush:    &PhaseResult{Passed: false, WorkflowsRun: 0},
			Message:    fmt.Sprintf("Pre-push failed: %v", err),
		})
	}

	if !prePushResult.passed {
		act.CompletePhase(activity.PhasePrePush, false, "workflows denied")
		act.Complete(activity.StatusFailed, "Pre-push workflows denied the push")
		done(fmt.Errorf("pre-push denied"))
		return outputGitPushResponse(&GitPushResponse{
			ActivityID: act.ID,
			Status:     activity.StatusFailed,
			PrePush:    &PhaseResult{Passed: false, WorkflowsRun: prePushResult.workflowsRun},
			Message:    "Pre-push workflows denied the push. Check workflow logs for details.",
		})
	}

	act.CompletePhase(activity.PhasePrePush, true, fmt.Sprintf("%d workflows passed", prePushResult.workflowsRun))
	log.Info("pre-push passed (%d workflows)", prePushResult.workflowsRun)

	// Phase 2: Git push
	log.Info("phase 2: executing git push")
	act.StartPhase(activity.PhasePush)

	pushOutput, pushErr := executeGitPush(dir, gitArgs)
	if pushErr != nil {
		act.FailPhase(activity.PhasePush, pushErr.Error())
		act.Complete(activity.StatusFailed, "Git push failed: "+pushErr.Error())
		_ = act.WriteLog(activity.PhasePush, "git-push", pushOutput)
		done(pushErr)
		return outputGitPushResponse(&GitPushResponse{
			ActivityID: act.ID,
			Status:     activity.StatusFailed,
			PrePush:    &PhaseResult{Passed: true, WorkflowsRun: prePushResult.workflowsRun},
			Push:       &PushPhaseResult{Success: false, Output: pushOutput},
			Message:    fmt.Sprintf("Git push failed: %v", pushErr),
		})
	}

	act.CompletePhase(activity.PhasePush, true, pushOutput)
	_ = act.WriteLog(activity.PhasePush, "git-push", pushOutput)
	log.Info("git push succeeded")

	// Phase 3: Post-push workflows
	log.Info("phase 3: running post-push workflows")
	act.StartPhase(activity.PhasePostPush)

	postPushResult, err := runPushWorkflows(dir, act, "post")
	if err != nil {
		act.FailPhase(activity.PhasePostPush, err.Error())
		act.Complete(activity.StatusFailed, "Post-push phase error: "+err.Error())
		done(err)
		return outputGitPushResponse(&GitPushResponse{
			ActivityID: act.ID,
			Status:     activity.StatusFailed,
			PrePush:    &PhaseResult{Passed: true, WorkflowsRun: prePushResult.workflowsRun},
			Push:       &PushPhaseResult{Success: true, Output: pushOutput},
			PostPush:   &PostPushResult{Passed: false, WorkflowsRun: 0},
			Message:    fmt.Sprintf("Post-push error: %v", err),
		})
	}

	postPushPassed := postPushResult.passed
	act.CompletePhase(activity.PhasePostPush, postPushPassed, fmt.Sprintf("%d workflows completed", postPushResult.workflowsRun))

	// Complete the activity
	finalStatus := activity.StatusCompleted
	message := "Push and all checks completed successfully."
	if !postPushPassed {
		finalStatus = activity.StatusFailed
		message = "Push succeeded but post-push checks failed."
	}

	act.Complete(finalStatus, message)
	done(nil)

	return outputGitPushResponse(&GitPushResponse{
		ActivityID: act.ID,
		Status:     finalStatus,
		PrePush:    &PhaseResult{Passed: true, WorkflowsRun: prePushResult.workflowsRun},
		Push:       &PushPhaseResult{Success: true, Output: pushOutput},
		PostPush:   &PostPushResult{Passed: postPushPassed, WorkflowsRun: postPushResult.workflowsRun},
		Message:    message,
	})
}

type workflowPhaseResult struct {
	passed       bool
	workflowsRun int
}

// runPushWorkflows discovers and runs push-triggered workflows for the given lifecycle
func runPushWorkflows(dir string, act *activity.Activity, lifecycle string) (*workflowPhaseResult, error) {
	log := logging.Context("git-push")

	// Build a push event from current git context
	evt := buildPushEvent(dir, lifecycle)

	// Discover workflows
	workflows, err := discover.Discover(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to discover workflows: %w", err)
	}

	if len(workflows) == 0 {
		log.Debug("no workflows found")
		return &workflowPhaseResult{passed: true, workflowsRun: 0}, nil
	}

	// Load and match workflows
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

	// Run matching workflows
	ctx := context.Background()
	allPassed := true
	phase := lifecycleToPhase(lifecycle)

	for _, wf := range matchingWorkflows {
		log.Info("running workflow: %s", wf.Name)
		r := runner.NewRunner(wf, evt, dir)
		result := r.RunWithBlocking(ctx)

		success := result.PermissionDecision == "allow"
		errMsg := ""
		if !success {
			errMsg = result.PermissionDecisionReason
			allPassed = false
		}

		act.AddWorkflowResult(phase, wf.Name, success, errMsg)

		// Write workflow logs
		logContent := fmt.Sprintf("Workflow: %s\nDecision: %s\n", wf.Name, result.PermissionDecision)
		if result.PermissionDecisionReason != "" {
			logContent += fmt.Sprintf("Reason: %s\n", result.PermissionDecisionReason)
		}
		if result.LogFile != "" {
			if data, err := os.ReadFile(result.LogFile); err == nil {
				logContent += fmt.Sprintf("\n--- Detailed Logs ---\n%s\n", string(data))
			}
		}
		_ = act.WriteLog(phase, wf.Name, logContent)

		if !success {
			log.Warn("workflow %s denied: %s", wf.Name, errMsg)
			break // Stop on first deny
		}
	}

	return &workflowPhaseResult{
		passed:       allPassed,
		workflowsRun: len(matchingWorkflows),
	}, nil
}

// buildPushEvent creates a push event from current git context
func buildPushEvent(dir, lifecycle string) *schema.Event {
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

// gitContextProvider gathers git context for the push event
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

// executeGitPush runs git push with the provided arguments
func executeGitPush(dir string, args []string) (string, error) {
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

// lifecycleToPhase converts a lifecycle string to an activity Phase
func lifecycleToPhase(lifecycle string) activity.Phase {
	switch lifecycle {
	case "pre":
		return activity.PhasePrePush
	case "post":
		return activity.PhasePostPush
	default:
		return activity.PhasePrePush
	}
}

// outputGitPushResponse outputs the response as JSON
func outputGitPushResponse(resp *GitPushResponse) error {
	jsonBytes, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}

	// Write to stdout for agent consumption
	fmt.Println(string(jsonBytes))

	// Also write to log for debugging
	log := logging.Context("git-push")
	log.Info("response: status=%s, activity=%s", resp.Status, resp.ActivityID)

	return nil
}
