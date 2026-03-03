package main

import (
	"encoding/json"
	"fmt"

	"github.com/htekdev/gh-hookflow/internal/activity"
	"github.com/htekdev/gh-hookflow/internal/push"
	"github.com/spf13/cobra"
)

var gitPushStatusCmd = &cobra.Command{
	Use:   "git-push-status <activity_id>",
	Short: "Check status of a git push operation",
	Long: `Returns the current status of a git push operation started by 'hookflow git-push'.

The output includes the status of each phase (pre-push, push, post-push)
and guidance on what to do next if the push is still running.

Examples:
  hookflow git-push-status abc12345`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runGitPushStatus(args[0])
	},
}

func init() {
	rootCmd.AddCommand(gitPushStatusCmd)
}

type gitPushStatusOutput struct {
	ActivityID string               `json:"activity_id"`
	Status     string               `json:"status"`
	Message    string               `json:"message"`
	NextStep   string               `json:"next_step,omitempty"`
	PrePush    *push.PhaseResult    `json:"pre_push,omitempty"`
	Push       *push.PushPhaseResult `json:"push,omitempty"`
	PostPush   *push.PostPushResult `json:"post_push,omitempty"`
}

func runGitPushStatus(activityID string) error {
	act, err := activity.LoadActivity(activityID)
	if err != nil {
		return fmt.Errorf("activity not found: %w", err)
	}

	out := gitPushStatusOutput{
		ActivityID: act.ID,
		Status:     string(act.Status),
		Message:    act.Summary,
	}

	// Map phase statuses to response fields
	if ps, ok := act.Phases[activity.PhasePrePush]; ok && ps.Status != activity.StatusPending {
		out.PrePush = &push.PhaseResult{
			Passed:       ps.Status == activity.StatusCompleted,
			WorkflowsRun: len(ps.Workflows),
		}
	}
	if ps, ok := act.Phases[activity.PhasePush]; ok && ps.Status != activity.StatusPending {
		out.Push = &push.PushPhaseResult{
			Success: ps.Status == activity.StatusCompleted,
			Output:  ps.Output,
		}
	}
	if ps, ok := act.Phases[activity.PhasePostPush]; ok && ps.Status != activity.StatusPending {
		out.PostPush = &push.PostPushResult{
			Passed:       ps.Status == activity.StatusCompleted,
			WorkflowsRun: len(ps.Workflows),
		}
	}

	// Provide next step guidance if still running
	if act.Status == activity.StatusRunning {
		currentPhase := "pre-push workflows"
		if ps, ok := act.Phases[activity.PhasePush]; ok && ps.Status == activity.StatusRunning {
			currentPhase = "git push"
		} else if ps, ok := act.Phases[activity.PhasePostPush]; ok && ps.Status == activity.StatusRunning {
			currentPhase = "post-push workflows"
		}
		out.Message = fmt.Sprintf("Push in progress: %s running.", currentPhase)
		out.NextStep = fmt.Sprintf("Use 'hookflow git-push-status %s' to check again.", activityID)
	}

	jsonBytes, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}

	fmt.Println(string(jsonBytes))
	return nil
}
