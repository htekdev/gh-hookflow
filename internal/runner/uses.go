package runner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/htekdev/gh-hookflow/internal/schema"
	"gopkg.in/yaml.v3"
)

// ActionMetadata represents the structure of action.yml/action.yaml
type ActionMetadata struct {
	Name        string                 `yaml:"name,omitempty"`
	Description string                 `yaml:"description,omitempty"`
	Inputs      map[string]ActionInput `yaml:"inputs,omitempty"`
	Outputs     map[string]ActionOutput `yaml:"outputs,omitempty"`
	Runs        ActionRuns             `yaml:"runs"`
}

// ActionInput represents an action input parameter
type ActionInput struct {
	Description string      `yaml:"description,omitempty"`
	Required    bool        `yaml:"required,omitempty"`
	Default     interface{} `yaml:"default,omitempty"`
}

// ActionOutput represents an action output
type ActionOutput struct {
	Description string `yaml:"description,omitempty"`
	Value       string `yaml:"value,omitempty"`
}

// ActionRuns specifies how the action is executed
type ActionRuns struct {
	Using string         `yaml:"using"`
	Main  string         `yaml:"main,omitempty"`
	Steps []schema.Step  `yaml:"steps,omitempty"`
	Shell string         `yaml:"shell,omitempty"`
	Run   string         `yaml:"run,omitempty"`
}

// ParsedUses contains the parsed uses: reference
type ParsedUses struct {
	IsLocal bool   // true for local paths (./path/to/action)
	Owner   string // GitHub owner
	Repo    string // GitHub repo name
	Path    string // optional path within repo (for sub-actions)
	Version string // version/tag/ref
	Source  string // original source string
}

// parseUsesString parses a uses: string into its components
func parseUsesString(uses string) (*ParsedUses, error) {
	uses = strings.TrimSpace(uses)

	// Check if it's a local action
	if strings.HasPrefix(uses, "./") || strings.HasPrefix(uses, "../") || strings.HasPrefix(uses, "/") {
		return &ParsedUses{
			IsLocal: true,
			Source:  uses,
		}, nil
	}

	// Parse GitHub action format: owner/repo@version or owner/repo/path@version
	parts := strings.Split(uses, "@")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid uses format: %s (expected owner/repo@version or owner/repo/path@version)", uses)
	}

	version := parts[1]
	if version == "" {
		return nil, fmt.Errorf("invalid uses format: missing version after @")
	}

	actionPath := parts[0]
	pathParts := strings.Split(actionPath, "/")

	if len(pathParts) < 2 {
		return nil, fmt.Errorf("invalid uses format: %s (expected at least owner/repo)", uses)
	}

	owner := pathParts[0]
	repo := pathParts[1]
	path := ""

	// If there are additional path components, they are the sub-path within the repo
	if len(pathParts) > 2 {
		path = filepath.Join(pathParts[2:]...)
	}

	return &ParsedUses{
		IsLocal: false,
		Owner:   owner,
		Repo:    repo,
		Path:    path,
		Version: version,
		Source:  uses,
	}, nil
}

// resolveActionPath returns the path to the action directory
func (r *Runner) resolveActionPath(ctx context.Context, parsed *ParsedUses) (string, error) {
	if parsed.IsLocal {
		// Resolve local path relative to working directory
		actionPath := parsed.Source
		if !filepath.IsAbs(actionPath) {
			actionPath = filepath.Join(r.workingDir, actionPath)
		}

		// Verify the path exists
		if _, err := os.Stat(actionPath); err != nil {
			return "", fmt.Errorf("local action path not found: %s", actionPath)
		}

		return actionPath, nil
	}

	// For GitHub actions, clone or use cached version
	// For MVP, we'll use a simple temp directory approach
	return r.cloneGitHubAction(ctx, parsed)
}

// cloneGitHubAction clones a GitHub action to a temp directory
func (r *Runner) cloneGitHubAction(ctx context.Context, parsed *ParsedUses) (string, error) {
	// Create temp directory for action
	tmpDir := filepath.Join(os.TempDir(), "hookflow-actions")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Determine the clone URL
	repoURL := fmt.Sprintf("https://github.com/%s/%s.git", parsed.Owner, parsed.Repo)
	actionDir := filepath.Join(tmpDir, fmt.Sprintf("%s-%s", parsed.Owner, parsed.Repo))

	// Check if already cloned
	if _, err := os.Stat(actionDir); err == nil {
		return r.getActionSubpath(actionDir, parsed.Path), nil
	}

	// Clone the repository
	cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", "--branch", parsed.Version, repoURL, actionDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("failed to clone action repository %s: %w\n%s", repoURL, err, string(output))
	}

	return r.getActionSubpath(actionDir, parsed.Path), nil
}

// getActionSubpath returns the full path to the action considering sub-paths
func (r *Runner) getActionSubpath(baseDir, subpath string) string {
	if subpath == "" {
		return baseDir
	}
	return filepath.Join(baseDir, subpath)
}

// loadActionMetadata loads and parses the action.yml/action.yaml file
func loadActionMetadata(actionDir string) (*ActionMetadata, error) {
	// Try action.yaml first, then action.yml
	for _, filename := range []string{"action.yaml", "action.yml"} {
		metadataPath := filepath.Join(actionDir, filename)
		data, err := os.ReadFile(metadataPath)
		if err == nil {
			var metadata ActionMetadata
			if err := yaml.Unmarshal(data, &metadata); err != nil {
				return nil, fmt.Errorf("failed to parse %s: %w", filename, err)
			}
			return &metadata, nil
		}
	}

	return nil, fmt.Errorf("action.yaml or action.yml not found in %s", actionDir)
}

// executeAction runs the action based on its metadata
func (r *Runner) executeAction(ctx context.Context, actionDir string, metadata *ActionMetadata, inputs map[string]string) (string, error) {
	runs := metadata.Runs

	// Prepare environment variables from inputs
	// GitHub Actions uses INPUT_<name> convention
	env := os.Environ()
	for k, v := range inputs {
		upperKey := strings.ToUpper(strings.ReplaceAll(k, "-", "_"))
		env = append(env, fmt.Sprintf("INPUT_%s=%s", upperKey, v))
	}

	// Add runner env
	for k, v := range r.env {
		val, _ := r.exprCtx.EvaluateString(v)
		env = append(env, fmt.Sprintf("%s=%s", k, val))
	}

	switch runs.Using {
	case "docker":
		// Docker-based action (MVP: not supported yet)
		return "", fmt.Errorf("docker-based actions not yet supported: %s", metadata.Name)

	case "composite", "node12", "node16", "node20":
		// Composite action or Node.js-based action
		if len(runs.Steps) > 0 {
			return r.executeCompositeAction(ctx, actionDir, runs.Steps, env)
		}

		// Fall through to shell script execution if main is specified
		if runs.Main != "" {
			runCmd := fmt.Sprintf("node %s", filepath.Join(actionDir, runs.Main))
			return r.executeShellCommand(ctx, actionDir, runCmd, env)
		}

		return "", fmt.Errorf("composite/node action has no steps or main")

	case "shell", "bash":
		// Shell-based action
		if runs.Run == "" {
			return "", fmt.Errorf("shell action has no run command")
		}

		shell := runs.Shell
		if shell == "" {
			shell = defaultShell()
		}

		return r.executeShellCommandWithShell(ctx, actionDir, runs.Run, shell, env)

	default:
		return "", fmt.Errorf("unsupported action type: %s", runs.Using)
	}
}

// executeCompositeAction executes composite action steps
func (r *Runner) executeCompositeAction(ctx context.Context, actionDir string, steps []schema.Step, env []string) (string, error) {
	var output string

	for _, step := range steps {
		// Only support run steps in composite actions for MVP
		if step.Run == "" {
			continue
		}

		shell := step.Shell
		if shell == "" {
			shell = defaultShell()
		}

		stepOutput, err := r.executeShellCommandWithShell(ctx, actionDir, step.Run, shell, env)
		if err != nil {
			return output, err
		}

		output += stepOutput
		if output != "" && !strings.HasSuffix(output, "\n") {
			output += "\n"
		}
	}

	return output, nil
}

// executeShellCommand executes a shell command in the action directory
func (r *Runner) executeShellCommand(ctx context.Context, actionDir, command string, env []string) (string, error) {
	return r.executeShellCommandWithShell(ctx, actionDir, command, defaultShell(), env)
}

// executeShellCommandWithShell executes a shell command with explicit shell choice
func (r *Runner) executeShellCommandWithShell(ctx context.Context, actionDir, command, shell string, env []string) (string, error) {
	var cmd *exec.Cmd

	switch shell {
	case "pwsh", "powershell":
		cmd = exec.CommandContext(ctx, "pwsh", "-NoProfile", "-NonInteractive", "-Command", command)
	case "bash":
		cmd = exec.CommandContext(ctx, "bash", "-c", command)
	case "sh":
		cmd = exec.CommandContext(ctx, "sh", "-c", command)
	case "cmd":
		cmd = exec.CommandContext(ctx, "cmd", "/c", command)
	default:
		cmd = exec.CommandContext(ctx, shell, "-c", command)
	}

	cmd.Dir = actionDir
	cmd.Env = env
	cmd.WaitDelay = 3 * time.Second

	// Capture output
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// EvaluateInputs evaluates input expressions using the runner's expression context
func (r *Runner) evaluateInputs(with map[string]string) (map[string]string, error) {
	evaluated := make(map[string]string)

	for k, v := range with {
		val, err := r.exprCtx.EvaluateString(v)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate input %s: %w", k, err)
		}
		evaluated[k] = val
	}

	return evaluated, nil
}
