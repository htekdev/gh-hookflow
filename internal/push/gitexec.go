package push

// GitExecutor abstracts git operations so they can be swapped at compile time
// via build tags. The real implementation uses exec.Command; the e2etest build
// uses env-var-controlled fakes.
type GitExecutor interface {
	// Push executes a git push with the given arguments in the specified directory.
	Push(dir string, args []string) (output string, err error)

	// CurrentBranch returns the name of the current git branch.
	CurrentBranch(dir string) (string, error)
}

// gitExec is the package-level executor, set by init() in the build-tagged files.
var gitExec GitExecutor
