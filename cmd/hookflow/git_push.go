package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/htekdev/gh-hookflow/internal/logging"
	"github.com/htekdev/gh-hookflow/internal/push"
	"github.com/spf13/cobra"
)

var gitPushCmd = &cobra.Command{
	Use:   "git-push [git push args...]",
	Short: "Push with pre/post workflow validation",
	Long: `Performs a git push with hookflow workflow orchestration.

This command runs synchronously through 3 phases:
1. Pre-push governance workflows
2. Git push
3. Post-push governance workflows

The command blocks until all phases complete and prints the result as JSON.
Copilot CLI handles long-running commands natively, so no polling is needed.

Examples:
  hookflow git-push origin main
  hookflow git-push origin feature/my-branch --force
  hookflow git-push                          # uses default remote and branch`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := ""
		verbose := false
		var gitArgs []string
		for i := 0; i < len(args); i++ {
			if (args[i] == "--dir" || args[i] == "-d") && i+1 < len(args) {
				dir = args[i+1]
				i++ // skip value
			} else if args[i] == "--verbose" || args[i] == "-v" {
				verbose = true
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

		return runGitPush(dir, gitArgs, verbose)
	},
}

func init() {
	gitPushCmd.DisableFlagParsing = true
}

func runGitPush(dir string, gitArgs []string, verbose bool) error {
	log := logging.Context("git-push")
	log.Info("starting git push %v", gitArgs)

	resp := push.Run(dir, gitArgs, verbose)

	// Output the result as JSON
	data, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}
	fmt.Println(string(data))

	if resp.Status == push.StatusFailed {
		log.Warn("push failed: %s", resp.Message)
		return fmt.Errorf("push failed")
	}

	log.Info("push completed: %s", resp.Message)
	return nil
}
