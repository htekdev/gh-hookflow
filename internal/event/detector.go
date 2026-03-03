// Package event provides detection and parsing of events from raw Copilot hook input.
// This centralizes all the complex logic for determining what type of event occurred
// (git commit, git push, file edit, etc.) and extracting relevant context.
package event

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/htekdev/gh-hookflow/internal/logging"
	"github.com/htekdev/gh-hookflow/internal/schema"
)

// RawHookInput represents the raw input from a Copilot hook
type RawHookInput struct {
	SessionID string          `json:"sessionId"`
	ToolName  string          `json:"toolName"`
	ToolArgs  json.RawMessage `json:"toolArgs"`
	Cwd       string          `json:"cwd"`
}

// ToolArgs represents parsed tool arguments
type ToolArgs struct {
	Command  string `json:"command"`
	Script   string `json:"script"`
	Code     string `json:"code"`
	Path     string `json:"path"`
	FileText string `json:"file_text"`
	OldStr   string `json:"old_str"`
	NewStr   string `json:"new_str"`
}

// GitContext provides git repository context gathered at runtime
type GitContext struct {
	Branch       string
	Author       string
	StagedFiles  []schema.FileStatus
	PendingFiles []schema.FileStatus // Files from git add that aren't staged yet
	Remote       string
	Ahead        int
	Behind       int
}

// Detector detects and builds events from raw hook input
type Detector struct {
	gitProvider GitProvider
}

// GitProvider interface for gathering git context (allows mocking in tests)
type GitProvider interface {
	GetBranch(cwd string) string
	GetAuthor(cwd string) string
	GetStagedFiles(cwd string) []schema.FileStatus
	GetPendingFiles(cwd string, command string) []schema.FileStatus
	GetRemote(cwd string) string
	GetAheadBehind(cwd string) (ahead, behind int)
}

// NewDetector creates a new event detector
func NewDetector(gitProvider GitProvider) *Detector {
	if gitProvider == nil {
		gitProvider = &RealGitProvider{}
	}
	return &Detector{gitProvider: gitProvider}
}

// DetectFromRawInput parses raw hook input and returns a structured event
func (d *Detector) DetectFromRawInput(input []byte) (*schema.Event, error) {
	var raw RawHookInput
	if err := json.Unmarshal(input, &raw); err != nil {
		return nil, err
	}

	return d.Detect(&raw)
}

// Detect determines the event type and builds the appropriate event structure
func (d *Detector) Detect(raw *RawHookInput) (*schema.Event, error) {
	log := logging.Context("detector")
	log.Debug("detecting event for tool=%s, cwd=%s", raw.ToolName, raw.Cwd)

	event := &schema.Event{
		Cwd: raw.Cwd,
	}

	// Parse tool args
	var args ToolArgs
	if len(raw.ToolArgs) > 0 {
		// Handle both object and string forms
		if err := json.Unmarshal(raw.ToolArgs, &args); err != nil {
			// Try as string
			var strArgs string
			if err := json.Unmarshal(raw.ToolArgs, &strArgs); err == nil {
				// Try parsing the string as JSON
				_ = json.Unmarshal([]byte(strArgs), &args)
			}
		}
	}

	// Get command from various possible fields
	command := args.Command
	if command == "" {
		command = args.Script
	}
	if command == "" {
		command = args.Code
	}

	log.Debug("parsed command=%q, path=%q", command, args.Path)

	// Always set tool event
	toolArgs := make(map[string]interface{})
	if len(raw.ToolArgs) > 0 {
		_ = json.Unmarshal(raw.ToolArgs, &toolArgs)
	}
	event.Tool = &schema.ToolEvent{
		Name:     raw.ToolName,
		Args:     toolArgs,
		HookType: "preToolUse",
	}

	// Detect specific event types based on tool and command
	switch raw.ToolName {
	case "powershell", "bash", "shell", "terminal":
		log.Debug("shell tool, checking for git commands")
		d.detectShellEvent(event, command, raw.Cwd)
	case "create":
		log.Debug("create tool for path=%s", args.Path)
		d.detectCreateEvent(event, &args)
	case "edit":
		log.Debug("edit tool for path=%s", args.Path)
		d.detectEditEvent(event, &args)
	}

	// Log what was detected
	if event.Commit != nil {
		log.Info("detected commit event with %d files", len(event.Commit.Files))
	} else if event.Push != nil {
		log.Info("detected push event to ref=%s", event.Push.Ref)
	} else if event.File != nil {
		log.Info("detected file event: action=%s, path=%s", event.File.Action, event.File.Path)
	} else {
		log.Debug("no specific event type detected, using tool event only")
	}

	return event, nil
}

// detectShellEvent handles shell/terminal commands
func (d *Detector) detectShellEvent(event *schema.Event, command, cwd string) {
	// Check for ALL git command types — do NOT early return so chained
	// commands like "git commit -m 'msg'\ngit push" detect both events.
	if IsGitCommitCommand(command) {
		d.buildCommitEvent(event, command, cwd)
	}

	if IsGitPushCommand(command) {
		d.buildPushEvent(event, command, cwd)
	}
}

// buildCommitEvent builds a commit event from a git commit command
func (d *Detector) buildCommitEvent(event *schema.Event, command, cwd string) {
	// Get staged files
	stagedFiles := d.gitProvider.GetStagedFiles(cwd)

	// Check if git add is in the command chain
	if IsGitAddCommand(command) {
		// Get pending files that would be added
		pendingFiles := d.gitProvider.GetPendingFiles(cwd, command)
		stagedFiles = mergeFiles(stagedFiles, pendingFiles)
	}

	event.Commit = &schema.CommitEvent{
		SHA:     "pending",
		Message: ExtractCommitMessage(command),
		Author:  d.gitProvider.GetAuthor(cwd),
		Files:   stagedFiles,
	}
}

// buildPushEvent builds a push event from a git push command
func (d *Detector) buildPushEvent(event *schema.Event, command, cwd string) {
	branch := d.gitProvider.GetBranch(cwd)

	event.Push = &schema.PushEvent{
		Ref:    ExtractPushRef(command, branch),
		Before: "",
		After:  "",
	}
}

// detectCreateEvent handles file creation
func (d *Detector) detectCreateEvent(event *schema.Event, args *ToolArgs) {
	event.File = &schema.FileEvent{
		Path:    args.Path,
		Action:  "create",
		Content: args.FileText,
	}
}

// detectEditEvent handles file edits
func (d *Detector) detectEditEvent(event *schema.Event, args *ToolArgs) {
	event.File = &schema.FileEvent{
		Path:   args.Path,
		Action: "edit",
	}
}

// Git command detection patterns
// All patterns treat newlines (\n) as command separators alongside &&, ||, ;
var (
	// Matches git commit at start of line or after command separators (&&, ||, ;, \n)
	gitCommitPattern = regexp.MustCompile(`(?m)(?:^|&&|\|\||;)\s*git\b.*\bcommit\b`)

	// Matches git push at start of line or after command separators
	gitPushPattern = regexp.MustCompile(`(?m)(?:^|&&|\|\||;)\s*git\b.*\bpush\b`)

	// Matches git add at start of line or after command separators
	gitAddPattern = regexp.MustCompile(`(?m)(?:^|&&|\|\||;)\s*git\b.*\badd\b`)

	// Extracts commit message from -m flag
	commitMessagePattern = regexp.MustCompile(`-m\s+["']([^"']+)["']|-m\s+(\S+)`)

	// Extracts tag from git push command
	tagPushPattern = regexp.MustCompile(`git\s+push\s+\S+\s+(v[\d.]+|refs/tags/\S+)`)

	// Extracts files from git add command
	gitAddFilesPattern = regexp.MustCompile(`git\s+add\s+(.+?)(?:&&|\|\||;|$)`)
)

// IsGitCommitCommand checks if a shell command contains a git commit
func IsGitCommitCommand(command string) bool {
	return gitCommitPattern.MatchString(command)
}

// IsGitPushCommand checks if a shell command contains a git push
func IsGitPushCommand(command string) bool {
	return gitPushPattern.MatchString(command)
}

// IsGitAddCommand checks if a shell command contains a git add
func IsGitAddCommand(command string) bool {
	return gitAddPattern.MatchString(command)
}

// ExtractCommitMessage extracts the commit message from a git commit command
func ExtractCommitMessage(command string) string {
	matches := commitMessagePattern.FindStringSubmatch(command)
	if len(matches) >= 2 {
		if matches[1] != "" {
			return matches[1]
		}
		if len(matches) >= 3 && matches[2] != "" {
			return matches[2]
		}
	}
	return ""
}

// ExtractPushRef determines the ref being pushed
func ExtractPushRef(command string, currentBranch string) string {
	// Check if pushing a tag
	matches := tagPushPattern.FindStringSubmatch(command)
	if len(matches) >= 2 {
		tag := matches[1]
		if !strings.HasPrefix(tag, "refs/") {
			return "refs/tags/" + tag
		}
		return tag
	}

	// Default to current branch
	if currentBranch != "" {
		return "refs/heads/" + currentBranch
	}
	return "refs/heads/main"
}

// ExtractGitAddFiles extracts file patterns from a git add command
func ExtractGitAddFiles(command string) []string {
	matches := gitAddFilesPattern.FindStringSubmatch(command)
	if len(matches) >= 2 {
		// Split by spaces, excluding flags
		parts := strings.Fields(matches[1])
		var files []string
		for _, p := range parts {
			if !strings.HasPrefix(p, "-") {
				files = append(files, p)
			}
		}
		return files
	}
	return nil
}

// mergeFiles merges two file lists, deduplicating by path
func mergeFiles(existing, new []schema.FileStatus) []schema.FileStatus {
	seen := make(map[string]bool)
	result := make([]schema.FileStatus, 0, len(existing)+len(new))

	for _, f := range existing {
		if !seen[f.Path] {
			seen[f.Path] = true
			result = append(result, f)
		}
	}
	for _, f := range new {
		if !seen[f.Path] {
			seen[f.Path] = true
			result = append(result, f)
		}
	}
	return result
}
