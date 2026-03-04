package main

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/htekdev/gh-hookflow/internal/activity"
	"github.com/htekdev/gh-hookflow/internal/logging"
	"github.com/htekdev/gh-hookflow/internal/push"
	"github.com/spf13/cobra"
)

var gitPushCmd = &cobra.Command{
	Use:   "git-push [git push args...]",
	Short: "Push with pre/post workflow validation",
	Long: `Performs a git push with hookflow workflow orchestration.

This command:
1. Creates an activity and returns the activity ID immediately
2. Spawns a background process to run the 3-phase push
3. Exits so the agent can continue (use git-push-status to poll)

Use 'hookflow git-push-status <activity_id>' to check progress.

Examples:
  hookflow git-push origin main
  hookflow git-push origin feature/my-branch --force
  hookflow git-push                          # uses default remote and branch`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Manually extract --dir/-d and --background flags since we disable flag parsing
		dir := ""
		background := false
		var gitArgs []string
		for i := 0; i < len(args); i++ {
			if (args[i] == "--dir" || args[i] == "-d") && i+1 < len(args) {
				dir = args[i+1]
				i++ // skip value
			} else if args[i] == "--background" {
				background = true
			} else {
				gitArgs = append(gitArgs, args[i])
			}
		}
		if dir == "" {
			var err error
			dir, err = os.Getwd()
			if err != nil {
				return err
			}
		}

		if background {
			return runGitPushBackground(dir, gitArgs)
		}
		return runGitPush(dir, gitArgs)
	},
}

func init() {
	gitPushCmd.DisableFlagParsing = true
}

func runGitPush(dir string, gitArgs []string) error {
	log := logging.Context("git-push")

	go func() { _ = activity.CleanupOldActivities(7 * 24 * time.Hour) }()

	act, err := activity.NewActivity(gitArgs)
	if err != nil {
		return fmt.Errorf("failed to create activity: %w", err)
	}
	log.Info("created activity %s for git push %v", act.ID, gitArgs)

	// Spawn a detached background process to run the actual push
	selfPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to resolve executable path: %w", err)
	}

	bgArgs := []string{"git-push", "--background", "--dir", dir}
	bgArgs = append(bgArgs, gitArgs...)
	bgCmd := exec.Command(selfPath, bgArgs...)
	bgCmd.Dir = dir

	// Pass the activity ID and session dir to the background process
	bgCmd.Env = append(os.Environ(),
		"HOOKFLOW_PUSH_ACTIVITY_ID="+act.ID,
	)

	// Detach from parent process
	detachProcess(bgCmd)

	if err := bgCmd.Start(); err != nil {
		return fmt.Errorf("failed to start background push: %w", err)
	}
	log.Info("spawned background push process (pid=%d)", bgCmd.Process.Pid)

	// Print agent-directive message and exit immediately
	fmt.Printf("Git push has started. You MUST now call 'gh hookflow git-push-status %s' to check progress. You are NOT done — do NOT report success or failure until you have checked the status.\n", act.ID)

	return nil
}

// runGitPushBackground is the actual push execution, called in the detached process.
func runGitPushBackground(dir string, gitArgs []string) error {
	log := logging.Context("git-push-bg")

	activityID := os.Getenv("HOOKFLOW_PUSH_ACTIVITY_ID")
	if activityID == "" {
		return fmt.Errorf("HOOKFLOW_PUSH_ACTIVITY_ID not set")
	}

	act, err := activity.LoadActivity(activityID)
	if err != nil {
		return fmt.Errorf("failed to load activity %s: %w", activityID, err)
	}
	log.Info("background push started for activity %s", activityID)

	resp := push.Run(dir, gitArgs, act, false)

	if resp.Status == activity.StatusFailed {
		log.Warn("push failed: %s", resp.Message)
	} else {
		log.Info("push completed: %s", resp.Message)
	}

	return nil
}

// detachProcess configures the command to run as a fully detached background process.
func detachProcess(cmd *exec.Cmd) {
	setDetachAttr(cmd)
}
