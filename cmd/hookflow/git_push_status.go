package main

import (
	"fmt"
	"strings"

	"github.com/htekdev/gh-hookflow/internal/activity"
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

func runGitPushStatus(activityID string) error {
	act, err := activity.LoadActivity(activityID)
	if err != nil {
		return fmt.Errorf("activity not found: %w", err)
	}

	var message string

	switch act.Status {
	case activity.StatusRunning:
		currentPhase := "pre-push workflows"
		if ps, ok := act.Phases[activity.PhasePush]; ok && ps.Status == activity.StatusRunning {
			currentPhase = "git push"
		} else if ps, ok := act.Phases[activity.PhasePostPush]; ok && ps.Status == activity.StatusRunning {
			currentPhase = "post-push workflows"
		}
		message = fmt.Sprintf(
			"Push is still in progress (%s running). This is NOT hung — it is working normally. "+
				"Wait 30 seconds then call 'gh hookflow git-push-status %s' again. "+
				"You are NOT done. Do NOT tell the user the push is complete. "+
				"You MUST call the status command again until you receive a final result.",
			currentPhase, activityID)

	case activity.StatusCompleted:
		parts := []string{"Push completed successfully."}
		if ps, ok := act.Phases[activity.PhasePrePush]; ok && ps.Status == activity.StatusCompleted && len(ps.Workflows) > 0 {
			parts = append(parts, fmt.Sprintf("Pre-push: %d workflow(s) passed.", len(ps.Workflows)))
		}
		if ps, ok := act.Phases[activity.PhasePostPush]; ok && ps.Status == activity.StatusCompleted && len(ps.Workflows) > 0 {
			parts = append(parts, fmt.Sprintf("Post-push: %d workflow(s) passed.", len(ps.Workflows)))
		}
		message = strings.Join(parts, " ")

	case activity.StatusFailed:
		message = buildFailureMessage(act, activityID)

	default:
		message = fmt.Sprintf(
			"Push status is '%s'. Call 'gh hookflow git-push-status %s' again to get an update.",
			act.Status, activityID)
	}

	fmt.Println(message)
	return nil
}

// buildFailureMessage constructs a detailed failure message including
// per-workflow errors and log output so the agent can diagnose the issue.
func buildFailureMessage(act *activity.Activity, activityID string) string {
	var b strings.Builder

	if ps, ok := act.Phases[activity.PhasePrePush]; ok && ps.Status == activity.StatusFailed {
		b.WriteString("Push FAILED: pre-push workflows denied the push.\n\n")
		writePhaseDetails(&b, ps)
		writePhaseLogs(&b, act, activity.PhasePrePush)
	} else if ps, ok := act.Phases[activity.PhasePush]; ok && ps.Status == activity.StatusFailed {
		b.WriteString("Push FAILED: git push itself failed.\n\n")
		if ps.Output != "" {
			b.WriteString("Git output:\n")
			b.WriteString(ps.Output)
			b.WriteString("\n\n")
		}
		if ps.Error != "" {
			b.WriteString("Error: ")
			b.WriteString(ps.Error)
			b.WriteString("\n\n")
		}
	} else if ps, ok := act.Phases[activity.PhasePostPush]; ok && ps.Status == activity.StatusFailed {
		b.WriteString(fmt.Sprintf("Push succeeded but post-push checks FAILED (%d workflow(s)).\n\n", len(ps.Workflows)))
		writePhaseDetails(&b, ps)
		writePhaseLogs(&b, act, activity.PhasePostPush)
	} else {
		b.WriteString("Push FAILED (unknown phase).\n\n")
	}

	b.WriteString("You MUST investigate the failure details above and take corrective action. ")
	b.WriteString("Do NOT simply report that it failed — review the errors and address them.")

	return b.String()
}

// writePhaseDetails writes per-workflow status and errors to the builder.
func writePhaseDetails(b *strings.Builder, ps *activity.PhaseStatus) {
	if ps.Error != "" {
		b.WriteString("Phase error: ")
		b.WriteString(ps.Error)
		b.WriteString("\n\n")
	}

	for _, wf := range ps.Workflows {
		if wf.Success {
			b.WriteString(fmt.Sprintf("  ✅ %s: passed\n", wf.Name))
		} else {
			b.WriteString(fmt.Sprintf("  ❌ %s: FAILED", wf.Name))
			if wf.Error != "" {
				b.WriteString(fmt.Sprintf(" — %s", wf.Error))
			}
			b.WriteString("\n")
		}
	}

	if len(ps.Workflows) > 0 {
		b.WriteString("\n")
	}
}

// writePhaseLogs reads and includes the log files for the failed phase.
func writePhaseLogs(b *strings.Builder, act *activity.Activity, phase activity.Phase) {
	logs, err := act.ReadLogs()
	if err != nil || len(logs) == 0 {
		return
	}

	prefix := string(phase) + "-"
	for name, content := range logs {
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		content = strings.TrimSpace(content)
		if content == "" {
			continue
		}
		b.WriteString(fmt.Sprintf("--- Logs: %s ---\n", strings.TrimPrefix(name, prefix)))
		b.WriteString(content)
		b.WriteString("\n\n")
	}
}
