package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/htekdev/gh-hookflow/internal/discover"
	"github.com/htekdev/gh-hookflow/internal/event"
	"github.com/htekdev/gh-hookflow/internal/logging"
	"github.com/htekdev/gh-hookflow/internal/runner"
	"github.com/htekdev/gh-hookflow/internal/schema"
	"github.com/htekdev/gh-hookflow/internal/session"
	"github.com/htekdev/gh-hookflow/internal/trigger"
	"github.com/spf13/cobra"
)

var version = "0.1.0"

func main() {
	// Initialize logging (errors are non-fatal)
	_ = logging.Init()
	defer logging.Close()

	logging.Info("hookflow started, version=%s, args=%v", version, os.Args)

	if err := rootCmd.Execute(); err != nil {
		logging.Error("command failed: %v", err)
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "hookflow",
	Short: "Local workflow engine for agentic DevOps",
	Long: `hookflow is a CLI tool that executes local workflows triggered by
Copilot agent hooks, file changes, commits, and pushes.

Workflows are defined in .github/hookflows/*.yml using a GitHub Actions-like syntax.`,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("hookflow version %s\n", version)
	},
}

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Show hookflow logs",
	Long: `Display hookflow logs for debugging.

Logs are stored in ~/.hookflow/logs/ with daily rotation.
Enable debug logging by setting HOOKFLOW_DEBUG=1.

Examples:
  hookflow logs              # Show last 50 lines of today's log
  hookflow logs -n 100       # Show last 100 lines
  hookflow logs --path       # Print log file path (for scripting)
  hookflow logs -f           # Follow log output`,
	RunE: func(cmd *cobra.Command, args []string) error {
		pathOnly, _ := cmd.Flags().GetBool("path")
		tail, _ := cmd.Flags().GetInt("tail")
		follow, _ := cmd.Flags().GetBool("follow")

		logPath := logging.LogPath()
		if logPath == "" {
			// Logger not initialized, construct path
			logPath = filepath.Join(logging.LogDir(), fmt.Sprintf("hookflow-%s.log", time.Now().Format("2006-01-02")))
		}

		// Just print path for scripting
		if pathOnly {
			fmt.Println(logPath)
			return nil
		}

		// Check if log file exists
		if _, err := os.Stat(logPath); os.IsNotExist(err) {
			fmt.Printf("No logs found at: %s\n", logPath)
			fmt.Println("\nTo enable logging, run hookflow commands with HOOKFLOW_DEBUG=1")
			return nil
		}

		// Print log location
		fmt.Printf("Log file: %s\n", logPath)
		fmt.Printf("Log dir:  %s\n", logging.LogDir())
		fmt.Println(strings.Repeat("-", 60))

		// Read and display log file
		if follow {
			return followLog(logPath)
		}

		return tailLog(logPath, tail)
	},
}

// tailLog shows the last n lines of the log file
func tailLog(path string, n int) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read log file: %w", err)
	}

	lines := strings.Split(string(content), "\n")

	// Get last n lines
	start := len(lines) - n
	if start < 0 {
		start = 0
	}

	for _, line := range lines[start:] {
		if line != "" {
			fmt.Println(line)
		}
	}
	return nil
}

// followLog tails the log file continuously (like tail -f)
func followLog(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer func() { _ = file.Close() }()

	// Seek to end
	_, _ = file.Seek(0, io.SeekEnd)

	fmt.Println("Following log output (Ctrl+C to stop)...")
	fmt.Println()

	buf := make([]byte, 1024)
	for {
		n, err := file.Read(buf)
		if err != nil && err != io.EOF {
			return err
		}
		if n > 0 {
			fmt.Print(string(buf[:n]))
		}
		time.Sleep(100 * time.Millisecond)
	}
}

var discoverCmd = &cobra.Command{
	Use:   "discover",
	Short: "Discover workflow files in the current directory",
	Long:  `Searches for .github/hookflows/*.yml files and lists them.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, _ := cmd.Flags().GetString("dir")
		if dir == "" {
			var err error
			dir, err = os.Getwd()
			if err != nil {
				return err
			}
		}
		fmt.Printf("Discovering workflows in: %s\n", dir)

		// Import discover package and call Discover
		workflows, err := discoverWorkflows(dir)
		if err != nil {
			return fmt.Errorf("failed to discover workflows: %w", err)
		}

		if len(workflows) == 0 {
			fmt.Println("No workflows found")
			return nil
		}

		fmt.Printf("Found %d workflow(s):\n", len(workflows))
		for _, wf := range workflows {
			fmt.Printf("  - %s (%s)\n", wf.Name, wf.RelPath)
		}
		return nil
	},
}

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate workflow files",
	Long:  `Validates workflow YAML files against the schema.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, _ := cmd.Flags().GetString("dir")
		file, _ := cmd.Flags().GetString("file")

		if dir == "" {
			var err error
			dir, err = os.Getwd()
			if err != nil {
				return err
			}
		}

		// Validate specific file or directory
		var result *schema.ValidationResult
		if file != "" {
			fmt.Printf("Validating file: %s\n", file)
			result = schema.ValidateWorkflow(file)
		} else {
			fmt.Printf("Validating workflows in: %s\n", dir)
			result = schema.ValidateWorkflowsInDir(dir)
		}

		// Print results
		if result.Valid {
			if file != "" {
				fmt.Printf("✓ File is valid\n")
			} else {
				fmt.Printf("✓ All workflows are valid\n")
			}
			return nil
		}

		// Print errors
		for _, err := range result.Errors {
			fmt.Printf("✗ %s\n", err.File)
			fmt.Printf("  Error: %s\n", err.Message)
			for _, detail := range err.Details {
				fmt.Printf("    - %s\n", detail)
			}
		}

		// Exit with error code
		os.Exit(1)
		return nil
	},
}

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run workflows for an event",
	Long: `Executes matching workflows based on the provided event payload.

Use --raw to pass raw Copilot hook input (toolName, toolArgs, cwd) and let the CLI
detect the event type automatically. This is the preferred mode for hook scripts.

Use --event to pass a pre-built event JSON (legacy mode).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		eventStr, _ := cmd.Flags().GetString("event")
		workflow, _ := cmd.Flags().GetString("workflow")
		dir, _ := cmd.Flags().GetString("dir")
		raw, _ := cmd.Flags().GetBool("raw")
		eventType, _ := cmd.Flags().GetString("event-type")
		global, _ := cmd.Flags().GetBool("global")

		// Convert event type to lifecycle
		lifecycle := eventTypeToLifecycle(eventType)

		if dir == "" {
			var err error
			dir, err = os.Getwd()
			if err != nil {
				return err
			}
		}

		// If workflow is specified, load and run it
		if workflow != "" {
			return runWorkflow(dir, workflow)
		}

		// If --raw flag is set, use the new event detection
		if raw {
			return runWithRawInput(dir, eventStr, lifecycle, global)
		}

		// Legacy mode: pre-built event JSON
		return runMatchingWorkflows(dir, eventStr, lifecycle)
	},
}

var triggersCmd = &cobra.Command{
	Use:   "triggers",
	Short: "List available trigger types",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Available trigger types:")
		fmt.Println("  hooks    - Agent hook events (preToolUse, postToolUse)")
		fmt.Println("  tool     - Tool-specific triggers with argument filtering")
		fmt.Println("  file     - File create/edit events")
		fmt.Println("  commit   - Git commit events")
		fmt.Println("  push     - Git push events")
	},
}

func init() {
	// Add commands
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(discoverCmd)
	rootCmd.AddCommand(validateCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(triggersCmd)
	rootCmd.AddCommand(logsCmd)
	rootCmd.AddCommand(gitPushCmd)

	// discover flags
	discoverCmd.Flags().StringP("dir", "d", "", "Directory to search (default: current directory)")

	// validate flags
	validateCmd.Flags().StringP("dir", "d", "", "Directory to search (default: current directory)")
	validateCmd.Flags().StringP("file", "f", "", "Specific file to validate")

	// run flags
	runCmd.Flags().StringP("event", "e", "", "Event JSON (use '-' for stdin)")
	runCmd.Flags().StringP("workflow", "w", "", "Specific workflow to run")
	runCmd.Flags().StringP("dir", "d", "", "Directory to search (default: current directory)")
	runCmd.Flags().BoolP("raw", "r", false, "Accept raw hook input and auto-detect event type")
	runCmd.Flags().StringP("event-type", "t", "preToolUse", "Hook event type: preToolUse or postToolUse")
	runCmd.Flags().Bool("global", false, "Running from global/plugin hooks (skips if repo hooks already ran)")

	// logs flags
	logsCmd.Flags().IntP("tail", "n", 50, "Number of lines to show")
	logsCmd.Flags().BoolP("follow", "f", false, "Follow log output (like tail -f)")
	logsCmd.Flags().Bool("path", false, "Only print log path (for scripting)")
}

// eventTypeToLifecycle converts Copilot hook event type to workflow lifecycle
func eventTypeToLifecycle(eventType string) string {
	switch eventType {
	case "postToolUse", "post":
		return "post"
	default:
		return "pre" // preToolUse, pre, or any unknown defaults to pre
	}
}

// runWorkflow loads and executes a specific workflow
func runWorkflow(dir, workflowName string) error {
	// Try to find the workflow file
	path, found := findWorkflowFile(dir, workflowName)
	if !found {
		return fmt.Errorf("workflow '%s' not found", workflowName)
	}

	// Load the workflow
	wf, err := schema.LoadWorkflow(path)
	if err != nil {
		return fmt.Errorf("failed to load workflow: %w", err)
	}

	// Execute the workflow
	ctx := context.Background()
	r := runner.NewRunner(wf, nil, dir)
	result := r.RunWithBlocking(ctx)

	// Output the result as JSON
	return outputWorkflowResult(result)
}

// runWithRawInput handles raw Copilot hook input and auto-detects event type
func runWithRawInput(dir, inputStr, lifecycle string, global bool) error {
	log := logging.Context("run")
	done := logging.StartOperation("runWithRawInput", "dir="+dir, "lifecycle="+lifecycle, fmt.Sprintf("global=%v", global))

	// Read from stdin if "-"
	var input []byte
	var err error
	if inputStr == "-" || inputStr == "" {
		input, err = io.ReadAll(os.Stdin)
		if err != nil {
			done(err)
			return fmt.Errorf("failed to read stdin: %w", err)
		}
	} else {
		input = []byte(inputStr)
	}

	// If empty input, allow by default
	if len(input) == 0 || string(input) == "" {
		log.Debug("empty input, allowing by default")
		result := schema.NewAllowResult()
		done(nil)
		return outputWorkflowResult(result)
	}

	log.Debug("input length=%d", len(input))

	// ── Session identity ────────────────────────────────────────────
	// Copilot CLI includes sessionId (UUID) in every hook event.
	// Use it to set the session directory so all downstream code
	// (WriteError, HasError, ReadAndClearError) uses the same path.
	var raw struct {
		ToolName  string `json:"toolName"`
		SessionID string `json:"sessionId"`
	}
	_ = json.Unmarshal(input, &raw)
	if raw.SessionID != "" && os.Getenv("HOOKFLOW_SESSION_DIR") == "" {
		if dir, err := session.SessionDirForID(raw.SessionID); err == nil {
			_ = os.Setenv("HOOKFLOW_SESSION_DIR", dir)
			log.Debug("set session dir from sessionId: %s", dir)
		}
	}

	// ── Global dedup check ─────────────────────────────────────────
	// When invoked with --global (from plugin hooks), check if the repo
	// hooks already ran hookflow for this session. If the repo-hooks-active
	// marker exists, skip to avoid double-execution.
	if global {
		repoActive, err := session.IsRepoHooksActive()
		if err != nil {
			log.Warn("failed to check repo-hooks-active: %v", err)
		}
		if repoActive {
			// Verify repo hooks still exist — hooks.json may have been deleted mid-session
			repoHooksFile := filepath.Join(dir, ".github", "hooks", "hooks.json")
			if hasHookflowHooks(repoHooksFile) {
				log.Debug("global mode: repo hooks already active, skipping")
				result := schema.NewAllowResult()
				done(nil)
				return outputWorkflowResult(result)
			}
			// Stale marker: hooks.json was removed mid-session
			log.Info("global mode: repo-hooks-active marker is stale (hooks.json removed), clearing and processing")
			_ = session.ClearRepoHooksActive()
		}
		log.Debug("global mode: no repo hooks, processing (global-only)")
	}

	// ── Repo hooks marker ──────────────────────────────────────────
	// When invoked without --global (from repo hooks), create a marker
	// so subsequent global hook invocations know to skip.
	if !global {
		if err := session.MarkRepoHooksActive(); err != nil {
			log.Warn("failed to mark repo hooks active: %v", err)
		}
	}

	// ── Error file read exemption ───────────────────────────────────
	// When a session error blocks the agent, the deny message tells it
	// to read the error file. Allow `view` of that file through so the
	// agent can see the error details. The postToolUse handler will
	// delete the file once the agent has read it.
	if lifecycle == "pre" && raw.ToolName == "view" {
		if isSessionErrorFileRead(input) {
			log.Info("allowing view of session error file (primitive exemption)")
			result := schema.NewAllowResult()
			done(nil)
			return outputWorkflowResult(result)
		}
	}

	// ── Post-lifecycle error file cleanup ────────────────────────────
	// After the agent reads the error file via `view`, clear it so the
	// session error gate stops blocking subsequent tool calls.
	if lifecycle == "post" && raw.ToolName == "view" {
		if isSessionErrorFileRead(input) {
			_, _ = session.ReadAndClearError()
			log.Info("cleared session error after agent read the error file")
		}
	}

	// ── Primitive guards ─────────────────────────────────────────────
	// These run on raw input BEFORE event detection. They are critical
	// safety checks that cannot be bypassed by tool name variations.
	if lifecycle == "pre" {
		if result := primitiveGuards(input); result != nil {
			log.Info("primitive guard triggered: %s", result.PermissionDecisionReason)
			done(nil)
			return outputWorkflowResult(result)
		}
	}

	// Use the event detector to parse and build the event
	detector := event.NewDetector(nil) // nil = use real git provider
	evt, err := detector.DetectFromRawInput(input)
	if err != nil {
		done(err)
		return fmt.Errorf("failed to detect event: %w", err)
	}

	// Override cwd if dir is specified
	if dir != "" && evt.Cwd == "" {
		evt.Cwd = dir
	}
	if evt.Cwd == "" {
		evt.Cwd = dir
	}

	// Set lifecycle from CLI flag
	evt.Lifecycle = lifecycle

	log.Debug("detected event: file=%v, tool=%v, commit=%v, push=%v, lifecycle=%s", evt.File != nil, evt.Tool != nil, evt.Commit != nil, evt.Push != nil, lifecycle)

	// Discover and run matching workflows
	err = runMatchingWorkflowsWithEvent(dir, evt, global)
	done(err)
	return err
}

// primitiveGuards performs critical safety checks on raw hook input before
// any event detection or workflow matching. These are non-negotiable.
func primitiveGuards(input []byte) *schema.WorkflowResult {
	// Parse just enough to extract the command
	var raw struct {
		ToolArgs json.RawMessage `json:"toolArgs"`
	}
	if err := json.Unmarshal(input, &raw); err != nil {
		return nil // let event detector handle parse errors
	}

	// Extract command from toolArgs (handles command, script, code fields)
	var args struct {
		Command string `json:"command"`
		Script  string `json:"script"`
		Code    string `json:"code"`
	}
	if len(raw.ToolArgs) > 0 {
		_ = json.Unmarshal(raw.ToolArgs, &args)
	}
	command := args.Command
	if command == "" {
		command = args.Script
	}
	if command == "" {
		command = args.Code
	}
	if command == "" {
		return nil // no command to check
	}

	// Guard 1: Block git push — must use 'gh hookflow git-push' CLI command
	if primitiveGitPushPattern.MatchString(command) {
		return schema.NewDenyResult(
			"Direct git push is blocked by hookflow. Use 'gh hookflow git-push' instead.\n\n" +
				"Run: gh hookflow git-push <remote> <branch> [flags]\n" +
				"Then check status: gh hookflow git-push-status <activity_id>\n\n" +
				"IMPORTANT: Always use 'gh hookflow git-push' instead of 'git push'.")
	}

	// Guard 2: Block multiple git commands in one tool call
	if primitiveMultiGitPattern.MatchString(command) {
		return schema.NewDenyResult(
			"Multiple git commands in a single tool call are not allowed. " +
				"Each git command must be its own separate tool call so hookflow can validate each one individually.")
	}

	return nil
}

var (
	// Matches "git push" anywhere in the command (multiline safe)
	primitiveGitPushPattern = regexp.MustCompile(`(?mi)(?:^|\s|&&|;|\|)git\s+push\b`)
	// Matches two or more git commands chained via &&, ;, newlines, or pipes
	primitiveMultiGitPattern = regexp.MustCompile(`(?mi)(?:^|\s|&&|;|\|)git\s+\w+.*(?:&&|;|\n).*git\s+\w+`)
	// Matches "hookflow init" in a command (for compliance check exemption)
	hookflowInitPattern = regexp.MustCompile(`(?i)hookflow\s+init\b`)
)

// isHookflowInitCommand checks if the event represents a hookflow init command.
func isHookflowInitCommand(evt *schema.Event) bool {
	if evt.Tool == nil {
		return false
	}
	// Check tool args for command/script/code containing "hookflow init"
	for _, key := range []string{"command", "script", "code"} {
		if val, ok := evt.Tool.Args[key]; ok {
			if str, ok := val.(string); ok {
				if hookflowInitPattern.MatchString(str) {
					return true
				}
			}
		}
	}
	return false
}

// runMatchingWorkflowsWithEvent runs workflows with a pre-built event
func runMatchingWorkflowsWithEvent(dir string, evt *schema.Event, global bool) error {
	log := logging.Context("matcher")

	// Block direct git push (belt-and-suspenders — also checked in primitiveGuards)
	if evt.Push != nil && evt.GetLifecycle() == "pre" {
		log.Info("blocking direct git push — use 'gh hookflow git-push' instead")
		result := schema.NewDenyResult(
			"Direct git push is blocked by hookflow. Use 'gh hookflow git-push' instead.\n\n" +
				"Run: gh hookflow git-push <remote> <branch> [flags]\n" +
				"Then check status: gh hookflow git-push-status <activity_id>\n\n" +
				"IMPORTANT: Always use 'gh hookflow git-push' instead of 'git push'.")
		return outputWorkflowResult(result)
	}

	// Normalize file path to be relative to dir (for matching against workflow patterns)
	if evt.File != nil && evt.File.Path != "" {
		originalPath := evt.File.Path
		evt.File.Path = normalizeFilePath(evt.File.Path, dir)
		log.Debug("normalized path: %s -> %s", originalPath, evt.File.Path)
	}

	// Check for unacknowledged errors from previous postToolUse operations.
	// This is a global gate — if there's a pending session error and this is
	// a pre-lifecycle event, deny immediately regardless of workflow matching.
	if evt.GetLifecycle() == "pre" {
		hasErr, err := session.HasError()
		if err != nil {
			log.Warn("failed to check session error state: %v", err)
		} else if hasErr {
			log.Info("blocking pre-lifecycle event due to pending session error")
			errorPath, _ := session.GetErrorFilePath()
			msg := fmt.Sprintf(
				"A previous tool operation failed validation. You must read the error file to acknowledge it before continuing.\n\n"+
					"Error file: %s\n\n"+
					"Use the view tool to read this file, then fix the issue and retry your operation.",
				errorPath,
			)
			result := schema.NewDenyResult(msg)
			return outputWorkflowResult(result)
		}
	}

	// Discover workflows
	workflowDir := filepath.Join(dir, ".github", "hookflows")
	if _, err := os.Stat(workflowDir); os.IsNotExist(err) {
		// No workflows directory, allow by default
		log.Debug("no workflow directory at %s, allowing", workflowDir)
		result := schema.NewAllowResult()
		return outputWorkflowResult(result)
	}

	// Find all workflow files
	var workflowFiles []string
	err := filepath.Walk(workflowDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".yml" || ext == ".yaml" {
			workflowFiles = append(workflowFiles, path)
		}
		return nil
	})
	if err != nil {
		log.Error("workflow scan failed: %v", err)
		return fmt.Errorf("failed to scan workflows: %w", err)
	}

	log.Debug("found %d workflow files in %s", len(workflowFiles), workflowDir)

	// ── Repo compliance check (global-only mode) ────────────────────
	// When running from global/plugin hooks, check if the repo has
	// hookflow entries in .github/hooks/hooks.json. If hookflows exist
	// but hooks.json doesn't have hookflow entries, auto-initialize
	// the repo hooks so workflows are properly triggered going forward.
	if global && len(workflowFiles) > 0 {
		repoHooksFile := filepath.Join(dir, ".github", "hooks", "hooks.json")
		if !hasHookflowHooks(repoHooksFile) {
			// Check if the agent is running hookflow init — allow that through
			if !isHookflowInitCommand(evt) {
				log.Info("repo has hookflows but missing hooks.json — auto-initializing")
				if err := silentAutoInit(dir); err != nil {
					log.Error("auto-init failed: %v", err)
					result := schema.NewDenyResult(
						"This repo has hookflow workflows in .github/hookflows/ but auto-initialization failed.\n\n" +
							"Run manually: gh hookflow init\n\n" +
							"Error: " + err.Error())
					return outputWorkflowResult(result)
				}
				log.Info("auto-init complete — repo hooks created, continuing workflow processing")
			} else {
				log.Debug("allowing hookflow init command through compliance check")
			}
		}
	}

	if len(workflowFiles) == 0 {
		// No workflows found, allow by default
		result := schema.NewAllowResult()
		return outputWorkflowResult(result)
	}

	// Load and validate ALL workflows first - fail fast on invalid workflows
	var matchingWorkflows []*schema.Workflow
	var validationErrors []string
	for _, path := range workflowFiles {
		wf, err := schema.LoadAndValidateWorkflow(path)
		if err != nil {
			// Collect validation errors instead of silently skipping
			relPath, _ := filepath.Rel(dir, path)
			if relPath == "" {
				relPath = path
			}
			log.Warn("workflow validation failed: %s: %v", relPath, err)
			validationErrors = append(validationErrors, fmt.Sprintf("%s: %v", relPath, err))
			continue
		}

		// Check if workflow matches the event
		matcher := trigger.NewMatcher(wf)
		matched := matcher.Match(evt)
		if matched {
			log.Info("workflow matched: %s", wf.Name)
			matchingWorkflows = append(matchingWorkflows, wf)
		} else {
			log.Debug("workflow did not match: %s", wf.Name)
		}
	}

	// If any workflows are invalid, check if agent is trying to fix them
	if len(validationErrors) > 0 {
		// Allow edits/creates to .github/hookflows/ so agent can self-repair
		if isHookflowSelfRepair(evt, dir) {
			log.Info("allowing self-repair for invalid workflows")
			result := schema.NewAllowResult()
			result.PermissionDecisionReason = "Allowing hookflow self-repair (workflows have errors)"
			return outputWorkflowResult(result)
		}

		// Otherwise deny - workflows must be fixed first
		result := &schema.WorkflowResult{
			PermissionDecision:       "deny",
			PermissionDecisionReason: fmt.Sprintf("Invalid workflow(s): %s. Fix workflows in .github/hookflows/ first.", strings.Join(validationErrors, "; ")),
		}
		return outputWorkflowResult(result)
	}

	if len(matchingWorkflows) == 0 {
		// No matching workflows, allow by default
		log.Debug("no matching workflows, allowing")
		result := schema.NewAllowResult()
		return outputWorkflowResult(result)
	}

	log.Info("running %d matching workflows", len(matchingWorkflows))

	// Run matching workflows
	ctx := context.Background()
	var finalResult *schema.WorkflowResult

	for _, wf := range matchingWorkflows {
		log.Debug("executing workflow: %s", wf.Name)
		r := runner.NewRunner(wf, evt, dir)
		result := r.RunWithBlocking(ctx)

		// If any workflow denies, the final result is deny
		if result.PermissionDecision == "deny" {
			log.Warn("workflow %s denied: %s", wf.Name, result.PermissionDecisionReason)
			return outputWorkflowResult(result)
		}

		log.Debug("workflow %s allowed", wf.Name)
		// Keep the last allow result
		finalResult = result
	}

	if finalResult == nil {
		finalResult = schema.NewAllowResult()
	}

	return outputWorkflowResult(finalResult)
}

// runMatchingWorkflows discovers and runs all matching workflows
func runMatchingWorkflows(dir, eventStr, lifecycle string) error {
	// Parse the event
	var eventData map[string]interface{}
	
	// Handle stdin input
	if eventStr == "-" {
		input, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("failed to read stdin: %w", err)
		}
		eventStr = string(input)
	}
	
	if eventStr == "" {
		// No event provided, allow by default
		result := schema.NewAllowResult()
		return outputWorkflowResult(result)
	}
	
	if err := json.Unmarshal([]byte(eventStr), &eventData); err != nil {
		return fmt.Errorf("failed to parse event JSON: %w", err)
	}
	
	// Convert to Event struct
	event := parseEventData(eventData)
	
	// Normalize file path to be relative to dir (for matching against workflow patterns)
	if event.File != nil && event.File.Path != "" {
		event.File.Path = normalizeFilePath(event.File.Path, dir)
	}
	
	// Set lifecycle from CLI flag
	event.Lifecycle = lifecycle
	
	// Discover workflows
	workflowDir := filepath.Join(dir, ".github", "hookflows")
	if _, err := os.Stat(workflowDir); os.IsNotExist(err) {
		// No workflows directory, allow by default
		result := schema.NewAllowResult()
		return outputWorkflowResult(result)
	}
	
	// Find all workflow files
	var workflowFiles []string
	err := filepath.Walk(workflowDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".yml" || ext == ".yaml" {
			workflowFiles = append(workflowFiles, path)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to scan workflows: %w", err)
	}
	
	if len(workflowFiles) == 0 {
		// No workflows found, allow by default
		result := schema.NewAllowResult()
		return outputWorkflowResult(result)
	}
	
	// Load and match workflows
	var matchingWorkflows []*schema.Workflow
	for _, path := range workflowFiles {
		wf, err := schema.LoadWorkflow(path)
		if err != nil {
			// Skip invalid workflows
			continue
		}
		
		// Check if workflow matches the event
		matcher := trigger.NewMatcher(wf)
		if matcher.Match(event) {
			matchingWorkflows = append(matchingWorkflows, wf)
		}
	}
	
	if len(matchingWorkflows) == 0 {
		// No matching workflows, allow by default
		result := schema.NewAllowResult()
		return outputWorkflowResult(result)
	}
	
	// Run matching workflows
	ctx := context.Background()
	var finalResult *schema.WorkflowResult
	
	for _, wf := range matchingWorkflows {
		r := runner.NewRunner(wf, event, dir)
		result := r.RunWithBlocking(ctx)
		
		// If any workflow denies, the final result is deny
		if result.PermissionDecision == "deny" {
			return outputWorkflowResult(result)
		}
		
		// Keep the last allow result
		finalResult = result
	}
	
	if finalResult == nil {
		finalResult = schema.NewAllowResult()
	}
	
	return outputWorkflowResult(finalResult)
}

// parseEventData converts raw event data to a schema.Event
func parseEventData(data map[string]interface{}) *schema.Event {
	event := &schema.Event{}
	
	// Parse hook event
	if hookData, ok := data["hook"].(map[string]interface{}); ok {
		event.Hook = &schema.HookEvent{}
		if t, ok := hookData["type"].(string); ok {
			event.Hook.Type = t
		}
		if cwd, ok := hookData["cwd"].(string); ok {
			event.Hook.Cwd = cwd
		}
		if toolData, ok := hookData["tool"].(map[string]interface{}); ok {
			event.Hook.Tool = &schema.ToolEvent{}
			if name, ok := toolData["name"].(string); ok {
				event.Hook.Tool.Name = name
			}
			if args, ok := toolData["args"].(map[string]interface{}); ok {
				event.Hook.Tool.Args = args
			}
		}
	}
	
	// Parse tool event
	if toolData, ok := data["tool"].(map[string]interface{}); ok {
		event.Tool = &schema.ToolEvent{}
		if name, ok := toolData["name"].(string); ok {
			event.Tool.Name = name
		}
		if args, ok := toolData["args"].(map[string]interface{}); ok {
			event.Tool.Args = args
		}
		if hookType, ok := toolData["hook_type"].(string); ok {
			event.Tool.HookType = hookType
		}
	}
	
	// Parse file event
	if fileData, ok := data["file"].(map[string]interface{}); ok {
		event.File = &schema.FileEvent{}
		if p, ok := fileData["path"].(string); ok {
			event.File.Path = p
		}
		if a, ok := fileData["action"].(string); ok {
			event.File.Action = a
		}
		if c, ok := fileData["content"].(string); ok {
			event.File.Content = c
		}
	}
	
	// Parse commit event
	if commitData, ok := data["commit"].(map[string]interface{}); ok {
		event.Commit = &schema.CommitEvent{}
		if sha, ok := commitData["sha"].(string); ok {
			event.Commit.SHA = sha
		}
		if msg, ok := commitData["message"].(string); ok {
			event.Commit.Message = msg
		}
		if author, ok := commitData["author"].(string); ok {
			event.Commit.Author = author
		}
		if files, ok := commitData["files"].([]interface{}); ok {
			for _, f := range files {
				if fm, ok := f.(map[string]interface{}); ok {
					fs := schema.FileStatus{}
					if p, ok := fm["path"].(string); ok {
						fs.Path = p
					}
					if s, ok := fm["status"].(string); ok {
						fs.Status = s
					}
					event.Commit.Files = append(event.Commit.Files, fs)
				}
			}
		}
	}
	
	// Parse push event
	if pushData, ok := data["push"].(map[string]interface{}); ok {
		event.Push = &schema.PushEvent{}
		if ref, ok := pushData["ref"].(string); ok {
			event.Push.Ref = ref
		}
		if before, ok := pushData["before"].(string); ok {
			event.Push.Before = before
		}
		if after, ok := pushData["after"].(string); ok {
			event.Push.After = after
		}
	}
	
	// Parse top-level cwd and timestamp
	if cwd, ok := data["cwd"].(string); ok {
		event.Cwd = cwd
	}
	if ts, ok := data["timestamp"].(string); ok {
		event.Timestamp = ts
	}
	
	return event
}

// discoverWorkflows finds all workflow files in a directory
func discoverWorkflows(dir string) ([]discover.WorkflowFile, error) {
	return discover.Discover(dir)
}

// findWorkflowFile finds a workflow file by name
func findWorkflowFile(dir, workflowName string) (string, bool) {
	for _, ext := range []string{".yml", ".yaml"} {
		path := fmt.Sprintf("%s/.github/hookflows/%s%s", dir, workflowName, ext)
		if _, err := os.Stat(path); err == nil {
			return path, true
		}
	}
	return "", false
}

// outputWorkflowResult outputs the workflow result as JSON
func outputWorkflowResult(result *schema.WorkflowResult) error {
	jsonBytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal result: %w", err)
	}
	fmt.Println(string(jsonBytes))
	return nil
}

// Git command detection helpers
//
// These patterns are designed to match git commands at the start of a command line
// or after command separators (&&, ||, ;), but NOT inside quoted strings like echo "git commit"

var gitCommitPattern = regexp.MustCompile(`(?m)(?:^|&&|\|\||;|&)\s*git\s+(commit|ci)\b`)
var gitPushPattern = regexp.MustCompile(`(?m)(?:^|&&|\|\||;|&)\s*git\s+push\b`)
var commitMessagePattern = regexp.MustCompile(`-m\s+["']([^"']+)["']`)
var tagPushPattern = regexp.MustCompile(`\bgit\s+push\s+\S+\s+(v[\d.]+)`)

// isGitCommitCommand checks if a shell command contains a git commit
// It avoids false positives from git commands inside echo strings
func isGitCommitCommand(command string) bool {
	return gitCommitPattern.MatchString(command)
}

// isGitPushCommand checks if a shell command contains a git push
// It avoids false positives from git commands inside echo strings
func isGitPushCommand(command string) bool {
	return gitPushPattern.MatchString(command)
}

// extractCommitMessage extracts the commit message from a git commit command
func extractCommitMessage(command string) string {
	matches := commitMessagePattern.FindStringSubmatch(command)
	if len(matches) >= 2 {
		return matches[1]
	}
	return ""
}

// extractPushRef determines the ref being pushed
func extractPushRef(command string, currentBranch string) string {
	// Check if pushing a tag
	matches := tagPushPattern.FindStringSubmatch(command)
	if len(matches) >= 2 {
		return "refs/tags/" + matches[1]
	}
	
	// Default to current branch
	return "refs/heads/" + currentBranch
}

// isHookflowSelfRepair checks if the current event is an edit/create to .github/hookflows/
// This allows the agent to fix invalid workflows without being blocked
func isHookflowSelfRepair(evt *schema.Event, dir string) bool {
	// Must be a file event (edit or create)
	if evt.File == nil {
		return false
	}
	
	// Must be editing/creating a file
	action := evt.File.Action
	if action != "edit" && action != "create" {
		return false
	}
	
	// Check if the path is in .github/hookflows/
	filePath := evt.File.Path
	
	// Normalize path separators (handle both Windows and Unix paths on any platform)
	filePath = strings.ReplaceAll(filePath, "\\", "/")
	
	// Check for .github/hookflows/ in the path
	if strings.Contains(filePath, ".github/hookflows/") {
		// Must be a YAML file
		ext := strings.ToLower(filepath.Ext(filePath))
		if ext == ".yml" || ext == ".yaml" {
			return true
		}
	}
	
	return false
}

// isSessionErrorFileRead checks if the raw hook input is a `view` tool call
// targeting the session error file. Compares the requested path against the
// actual resolved error file path (handles HOOKFLOW_SESSION_DIR overrides).
func isSessionErrorFileRead(input []byte) bool {
	reqPath := extractToolArgsPath(input)
	if reqPath == "" {
		return false
	}

	errorPath, err := session.GetErrorFilePath()
	if err != nil {
		return false
	}

	// Normalize both paths: forward slashes, lowercase (Windows is case-insensitive)
	normalize := func(p string) string {
		return strings.ToLower(strings.ReplaceAll(p, "\\", "/"))
	}
	return normalize(reqPath) == normalize(errorPath)
}

// extractToolArgsPath extracts the "path" field from raw hook input toolArgs.
// Handles both JSON object and JSON string formats (preToolUse sends toolArgs
// as a JSON string, postToolUse sends it as a parsed object).
func extractToolArgsPath(input []byte) string {
	var raw struct {
		ToolArgs json.RawMessage `json:"toolArgs"`
	}
	if err := json.Unmarshal(input, &raw); err != nil || len(raw.ToolArgs) == 0 {
		return ""
	}

	var args struct {
		Path string `json:"path"`
	}
	// Try direct unmarshal (toolArgs is a JSON object)
	if err := json.Unmarshal(raw.ToolArgs, &args); err != nil {
		// Try unwrapping string first (toolArgs is a JSON string in preToolUse)
		var str string
		if err2 := json.Unmarshal(raw.ToolArgs, &str); err2 == nil {
			_ = json.Unmarshal([]byte(str), &args)
		}
	}
	return args.Path
}

// normalizeFilePath converts an absolute file path to a relative path from dir
// This ensures workflow path patterns (like 'plugin.json') match correctly
func normalizeFilePath(filePath, dir string) string {
	// Normalize path separators for cross-platform compatibility
	filePath = strings.ReplaceAll(filePath, "\\", "/")
	dir = strings.ReplaceAll(dir, "\\", "/")
	
	// Ensure dir ends with /
	if !strings.HasSuffix(dir, "/") {
		dir = dir + "/"
	}
	
	// If the file path starts with the dir, make it relative
	if strings.HasPrefix(filePath, dir) {
		return strings.TrimPrefix(filePath, dir)
	}
	
	// Also try case-insensitive match (Windows paths)
	lowerFilePath := strings.ToLower(filePath)
	lowerDir := strings.ToLower(dir)
	if strings.HasPrefix(lowerFilePath, lowerDir) {
		return filePath[len(dir):]
	}
	
	// Return as-is if not under dir
	return filePath
}
