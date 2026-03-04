package main

import (
	"encoding/json"
	"fmt"
	"strings"

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

	// Provide agent-directive messaging based on status
	switch act.Status {
	case activity.StatusRunning:
		currentPhase := "pre-push workflows"
		if ps, ok := act.Phases[activity.PhasePush]; ok && ps.Status == activity.StatusRunning {
			currentPhase = "git push"
		} else if ps, ok := act.Phases[activity.PhasePostPush]; ok && ps.Status == activity.StatusRunning {
			currentPhase = "post-push workflows"
		}
		out.Message = fmt.Sprintf(
			"Push is still in progress (%s running). This is NOT hung — it is working normally. "+
				"Wait 30 seconds then call 'gh hookflow git-push-status %s' again. "+
				"You are NOT done. Do NOT tell the user the push is complete. "+
				"You MUST call the status command again until you receive a final result.",
			currentPhase, activityID)

	case activity.StatusCompleted:
		parts := []string{"Push completed successfully."}
		if out.PrePush != nil && out.PrePush.WorkflowsRun > 0 {
			parts = append(parts, fmt.Sprintf("Pre-push: %d workflow(s) passed.", out.PrePush.WorkflowsRun))
		}
		if out.PostPush != nil && out.PostPush.WorkflowsRun > 0 {
			parts = append(parts, fmt.Sprintf("Post-push: %d workflow(s) passed.", out.PostPush.WorkflowsRun))
		}
		out.Message = strings.Join(parts, " ")

	case activity.StatusFailed:
		// Determine which phase failed and build actionable message
		if out.PrePush != nil && !out.PrePush.Passed {
			out.Message = fmt.Sprintf(
				"Push FAILED: pre-push workflows denied the push. "+
					"You MUST investigate the failure and take corrective action. "+
					"Check the activity logs: gh hookflow git-push-status %s. "+
					"Do NOT simply report that the push failed — fix the issue and retry.",
				activityID)
		} else if out.Push != nil && !out.Push.Success {
			out.Message = fmt.Sprintf(
				"Push FAILED: git push itself failed. Output: %s. "+
					"You MUST investigate and resolve the git error before retrying. "+
					"Do NOT simply report that the push failed — fix the issue and retry.",
				out.Push.Output)
		} else if out.PostPush != nil && !out.PostPush.Passed {
			out.Message = fmt.Sprintf(
				"Push succeeded but post-push checks FAILED (%d workflow(s)). "+
					"The code is pushed but post-push validation detected issues. "+
					"You MUST investigate the post-push failures and take corrective action. "+
					"Do NOT simply report that it failed — review the errors and address them.",
				out.PostPush.WorkflowsRun)
		} else {
			out.Message = "Push FAILED. You MUST investigate the failure and take corrective action. " +
				"Do NOT simply report that the push failed."
		}

	default:
		out.Message = fmt.Sprintf(
			"Push status is '%s'. Call 'gh hookflow git-push-status %s' again to get an update.",
			act.Status, activityID)
	}

	jsonBytes, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}

	fmt.Println(string(jsonBytes))
	return nil
}
