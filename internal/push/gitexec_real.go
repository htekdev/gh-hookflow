//go:build !e2etest

package push

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

func init() {
	gitExec = &realGitExecutor{}
}

type realGitExecutor struct{}

func (r *realGitExecutor) Push(dir string, args []string) (string, error) {
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

func (r *realGitExecutor) CurrentBranch(dir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
