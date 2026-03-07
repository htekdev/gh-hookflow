//go:build e2etest

package push

import (
	"fmt"
	"os"
)

func init() {
	gitExec = &fakeGitExecutor{}
}

type fakeGitExecutor struct{}

func (f *fakeGitExecutor) Push(dir string, args []string) (string, error) {
	if os.Getenv("HOOKFLOW_FAKE_GIT_PUSH_FAIL") == "1" {
		errMsg := os.Getenv("HOOKFLOW_FAKE_GIT_PUSH_ERROR")
		if errMsg == "" {
			errMsg = "fake: remote rejected push"
		}
		return errMsg, fmt.Errorf("git push failed: %s", errMsg)
	}

	output := os.Getenv("HOOKFLOW_FAKE_GIT_PUSH_OUTPUT")
	if output == "" {
		output = "Everything up-to-date"
	}
	return output, nil
}

func (f *fakeGitExecutor) CurrentBranch(dir string) (string, error) {
	branch := os.Getenv("HOOKFLOW_FAKE_GIT_BRANCH")
	if branch == "" {
		branch = "main"
	}
	return branch, nil
}
