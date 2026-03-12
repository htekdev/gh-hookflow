package trigger

import (
	"path/filepath"
	"strings"

	"github.com/htekdev/gh-hookflow/internal/logging"
	"github.com/htekdev/gh-hookflow/internal/schema"
)

// Matcher determines if a workflow should be triggered by an event
type Matcher struct {
	workflow *schema.Workflow
}

// NewMatcher creates a new trigger matcher for a workflow
func NewMatcher(workflow *schema.Workflow) *Matcher {
	return &Matcher{workflow: workflow}
}

// Match checks if the event matches any of the workflow's triggers
func (m *Matcher) Match(event *schema.Event) bool {
	log := logging.Context("trigger")
	on := m.workflow.On
	workflowName := m.workflow.Name

	// Check tool trigger (most specific) — tool/hooks triggers match any lifecycle
	if on.Tool != nil && event.Tool != nil {
		log.Debug("[%s] checking tool trigger for tool=%s", workflowName, event.Tool.Name)
		if m.matchToolTrigger(on.Tool, event.Tool) {
			log.Debug("[%s] tool trigger matched", workflowName)
			return true
		}
	}

	// Check tools array
	if len(on.Tools) > 0 && event.Tool != nil {
		log.Debug("[%s] checking %d tools triggers", workflowName, len(on.Tools))
		for i, toolTrigger := range on.Tools {
			if m.matchToolTrigger(&toolTrigger, event.Tool) {
				log.Debug("[%s] tools[%d] trigger matched", workflowName, i)
				return true
			}
		}
	}

	// Check hooks trigger — hooks use types (preToolUse/postToolUse) for lifecycle
	if on.Hooks != nil && event.Hook != nil {
		log.Debug("[%s] checking hooks trigger", workflowName)
		if m.matchHooksTrigger(on.Hooks, event.Hook) {
			log.Debug("[%s] hooks trigger matched", workflowName)
			return true
		}
	}

	// Check file trigger — lifecycle is on the trigger
	if on.File != nil && event.File != nil {
		if on.File.GetLifecycle() != event.GetLifecycle() {
			log.Debug("[%s] file trigger lifecycle mismatch: trigger=%s, event=%s", workflowName, on.File.GetLifecycle(), event.GetLifecycle())
		} else {
			log.Debug("[%s] checking file trigger for path=%s", workflowName, event.File.Path)
			if m.matchFileTrigger(on.File, event.File) {
				log.Debug("[%s] file trigger matched", workflowName)
				return true
			}
		}
	}

	// Check commit trigger — lifecycle is on the trigger
	if on.Commit != nil && event.Commit != nil {
		if on.Commit.GetLifecycle() != event.GetLifecycle() {
			log.Debug("[%s] commit trigger lifecycle mismatch: trigger=%s, event=%s", workflowName, on.Commit.GetLifecycle(), event.GetLifecycle())
		} else {
			log.Debug("[%s] checking commit trigger", workflowName)
			if m.matchCommitTrigger(on.Commit, event.Commit) {
				log.Debug("[%s] commit trigger matched", workflowName)
				return true
			}
		}
	}

	// Check push trigger — lifecycle is on the trigger
	if on.Push != nil && event.Push != nil {
		if on.Push.GetLifecycle() != event.GetLifecycle() {
			log.Debug("[%s] push trigger lifecycle mismatch: trigger=%s, event=%s", workflowName, on.Push.GetLifecycle(), event.GetLifecycle())
		} else {
			log.Debug("[%s] checking push trigger", workflowName)
			if m.matchPushTrigger(on.Push, event.Push) {
				log.Debug("[%s] push trigger matched", workflowName)
				return true
			}
		}
	}

	log.Debug("[%s] no triggers matched", workflowName)
	return false
}

// matchToolTrigger checks if a tool event matches a tool trigger
func (m *Matcher) matchToolTrigger(trigger *schema.ToolTrigger, event *schema.ToolEvent) bool {
	// Check tool name
	if trigger.Name != event.Name {
		return false
	}

	// Check args patterns
	for argName, pattern := range trigger.Args {
		argValue, ok := event.Args[argName]
		if !ok {
			return false
		}
		argStr, _ := argValue.(string)
		if !matchPlainGlob(pattern, argStr) {
			return false
		}
	}

	// Note: trigger.If expression is evaluated separately by expression engine
	return true
}

// matchHooksTrigger checks if a hook event matches a hooks trigger
func (m *Matcher) matchHooksTrigger(trigger *schema.HooksTrigger, event *schema.HookEvent) bool {
	// Check hook types
	if len(trigger.Types) > 0 {
		found := false
		for _, t := range trigger.Types {
			if t == event.Type {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check tools filter
	if len(trigger.Tools) > 0 && event.Tool != nil {
		found := false
		for _, t := range trigger.Tools {
			if t == event.Tool.Name {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

// matchFileTrigger checks if a file event matches a file trigger
func (m *Matcher) matchFileTrigger(trigger *schema.FileTrigger, event *schema.FileEvent) bool {
	log := logging.Context("trigger")

	// Check file types
	if len(trigger.Types) > 0 {
		found := false
		for _, t := range trigger.Types {
			if t == event.Action {
				found = true
				break
			}
		}
		if !found {
			log.Debug("action %s not in types %v", event.Action, trigger.Types)
			return false
		}
	}

	// Check paths-ignore first
	if len(trigger.PathsIgnore) > 0 {
		for _, pattern := range trigger.PathsIgnore {
			if matchGlob(pattern, event.Path) {
				log.Debug("path %s matches paths-ignore pattern %s", event.Path, pattern)
				return false
			}
		}
	}

	// Check paths
	if len(trigger.Paths) > 0 {
		matched := false
		for _, pattern := range trigger.Paths {
			// Handle negation
			if strings.HasPrefix(pattern, "!") {
				if matchGlob(pattern[1:], event.Path) {
					log.Debug("path %s matches negation pattern %s", event.Path, pattern)
					matched = false
				}
			} else if matchGlob(pattern, event.Path) {
				log.Debug("path %s matches pattern %s", event.Path, pattern)
				matched = true
			}
		}
		if !matched {
			log.Debug("path %s did not match any of %d patterns", event.Path, len(trigger.Paths))
			return false
		}
	}

	log.Debug("file trigger matched for path=%s", event.Path)
	return true
}

// matchCommitTrigger checks if a commit event matches a commit trigger
func (m *Matcher) matchCommitTrigger(trigger *schema.CommitTrigger, event *schema.CommitEvent) bool {
	// Check branches - would need branch info from context
	// For now, focus on path matching

	// Check paths-ignore
	if len(trigger.PathsIgnore) > 0 {
		allIgnored := true
		for _, file := range event.Files {
			ignored := false
			for _, pattern := range trigger.PathsIgnore {
				if matchGlob(pattern, file.Path) {
					ignored = true
					break
				}
			}
			if !ignored {
				allIgnored = false
				break
			}
		}
		if allIgnored {
			return false
		}
	}

	// Check paths
	if len(trigger.Paths) > 0 {
		matched := false
		for _, file := range event.Files {
			for _, pattern := range trigger.Paths {
				if strings.HasPrefix(pattern, "!") {
					continue
				}
				if matchGlob(pattern, file.Path) {
					matched = true
					break
				}
			}
			if matched {
				break
			}
		}
		if !matched {
			return false
		}
	}

	return true
}

// matchPushTrigger checks if a push event matches a push trigger
func (m *Matcher) matchPushTrigger(trigger *schema.PushTrigger, event *schema.PushEvent) bool {
	// Check branches
	if len(trigger.Branches) > 0 {
		branch := extractBranch(event.Ref)
		if branch != "" {
			matched := false
			for _, pattern := range trigger.Branches {
				if strings.HasPrefix(pattern, "!") {
					if matchGlob(pattern[1:], branch) {
						matched = false
					}
				} else if matchGlob(pattern, branch) {
					matched = true
				}
			}
			if !matched {
				return false
			}
		}
	}

	// Check branches-ignore
	if len(trigger.BranchesIgnore) > 0 {
		branch := extractBranch(event.Ref)
		if branch != "" {
			for _, pattern := range trigger.BranchesIgnore {
				if matchGlob(pattern, branch) {
					return false
				}
			}
		}
	}

	// Check tags
	if len(trigger.Tags) > 0 {
		tag := extractTag(event.Ref)
		if tag == "" {
			return false
		}
		matched := false
		for _, pattern := range trigger.Tags {
			if strings.HasPrefix(pattern, "!") {
				if matchGlob(pattern[1:], tag) {
					matched = false
				}
			} else if matchGlob(pattern, tag) {
				matched = true
			}
		}
		if !matched {
			return false
		}
	}

	// Check tags-ignore
	if len(trigger.TagsIgnore) > 0 {
		tag := extractTag(event.Ref)
		if tag != "" {
			for _, pattern := range trigger.TagsIgnore {
				if matchGlob(pattern, tag) {
					return false
				}
			}
		}
	}

	return true
}

// matchGlob performs glob pattern matching for file paths
func matchGlob(pattern, path string) bool {
	// Normalize path separators
	pattern = filepath.ToSlash(pattern)
	path = filepath.ToSlash(path)

	// Handle ** patterns
	if strings.Contains(pattern, "**") {
		return matchDoubleGlob(pattern, path)
	}

	// Use filepath.Match for simple patterns
	matched, _ := filepath.Match(pattern, path)
	return matched
}

// matchPlainGlob performs glob matching without path separator semantics.
// Unlike matchGlob, the * wildcard matches any character including / and \.
// Use this for non-path values like shell commands and tool arguments where
// forward slashes are not directory separators.
func matchPlainGlob(pattern, value string) bool {
	// filepath.Match treats os.PathSeparator as special — * won't match it.
	// On Linux, / is the separator, so *rm* won't match "rm -rf /important".
	// To do path-agnostic matching, replace both / and \ with a character
	// that is never a path separator on any OS.
	const safeChar = "\x01"
	p := strings.ReplaceAll(pattern, "/", safeChar)
	p = strings.ReplaceAll(p, "\\", safeChar)
	v := strings.ReplaceAll(value, "/", safeChar)
	v = strings.ReplaceAll(v, "\\", safeChar)
	matched, _ := filepath.Match(p, v)
	return matched
}

// matchDoubleGlob handles ** patterns that match across directories
func matchDoubleGlob(pattern, path string) bool {
	parts := strings.Split(pattern, "**")
	if len(parts) == 1 {
		matched, _ := filepath.Match(pattern, path)
		return matched
	}

	// For patterns like **/*.js
	if parts[0] == "" {
		suffix := strings.TrimPrefix(parts[1], "/")
		// Match suffix against any path segment
		pathParts := strings.Split(path, "/")
		for i := range pathParts {
			subpath := strings.Join(pathParts[i:], "/")
			if matched, _ := filepath.Match(suffix, subpath); matched {
				return true
			}
		}
		// Also try matching just the filename
		if matched, _ := filepath.Match(suffix, filepath.Base(path)); matched {
			return true
		}
		return false
	}

	// For patterns like src/**/test.js
	prefix := strings.TrimSuffix(parts[0], "/")
	suffix := strings.TrimPrefix(parts[1], "/")

	if !strings.HasPrefix(path, prefix) {
		return false
	}

	remaining := strings.TrimPrefix(path, prefix)
	remaining = strings.TrimPrefix(remaining, "/")

	if suffix == "" {
		return true
	}

	// Match suffix against remaining path
	pathParts := strings.Split(remaining, "/")
	for i := range pathParts {
		subpath := strings.Join(pathParts[i:], "/")
		if matched, _ := filepath.Match(suffix, subpath); matched {
			return true
		}
	}

	return false
}

// extractBranch extracts branch name from a ref
func extractBranch(ref string) string {
	const prefix = "refs/heads/"
	if strings.HasPrefix(ref, prefix) {
		return ref[len(prefix):]
	}
	return ""
}

// extractTag extracts tag name from a ref
func extractTag(ref string) string {
	const prefix = "refs/tags/"
	if strings.HasPrefix(ref, prefix) {
		return ref[len(prefix):]
	}
	return ""
}
