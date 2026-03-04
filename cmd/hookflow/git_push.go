package main

import (
	"fmt"
	"os"
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
2. Runs pre-push hookflows (on.push, lifecycle: pre) in the background
3. Executes git push with the provided arguments
4. Runs post-push hookflows (on.push, lifecycle: post)

Use 'hookflow git-push-status <activity_id>' to check progress.

Examples:
  hookflow git-push origin main
  hookflow git-push origin feature/my-branch --force
  hookflow git-push                          # uses default remote and branch`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Manually extract --dir/-d flag since we disable flag parsing
		dir := ""
		var gitArgs []string
		for i := 0; i < len(args); i++ {
			if (args[i] == "--dir" || args[i] == "-d") && i+1 < len(args) {
				dir = args[i+1]
				i++ // skip value
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

	// Print agent-directive message immediately
	fmt.Printf("Git push has started. You MUST now call 'gh hookflow git-push-status %s' to check progress. You are NOT done — do NOT report success or failure until you have checked the status.\n", act.ID)

	// Run synchronously — the CLI process stays alive until done
	resp := push.Run(dir, gitArgs, act, true)

	if resp.Status == activity.StatusFailed {
		log.Warn("push failed: %s", resp.Message)
	} else {
		log.Info("push completed: %s", resp.Message)
	}

	return nil
}
