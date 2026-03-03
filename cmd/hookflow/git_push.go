package main

import (
	"encoding/json"
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
	done := logging.StartOperation("git-push", fmt.Sprintf("args=%v", gitArgs))

	go func() { _ = activity.CleanupOldActivities(7 * 24 * time.Hour) }()

	act, err := activity.NewActivity(gitArgs)
	if err != nil {
		done(err)
		return fmt.Errorf("failed to create activity: %w", err)
	}
	log.Info("created activity %s for git push %v", act.ID, gitArgs)

	resp := push.Run(dir, gitArgs, act, true)

	if resp.Status == activity.StatusFailed {
		done(fmt.Errorf("push failed: %s", resp.Message))
	} else {
		done(nil)
	}

	return outputGitPushResponse(resp)
}

func outputGitPushResponse(resp *push.Response) error {
	jsonBytes, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}

	fmt.Println(string(jsonBytes))

	log := logging.Context("git-push")
	log.Info("response: status=%s, activity=%s", resp.Status, resp.ActivityID)

	return nil
}
