package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var registerCmd = &cobra.Command{
	Use:   "register",
	Short: "Register hookflow personal hooks and skill",
	Long: `Registers hookflow hooks and skill for the current user.

Creates or updates:
- ~/.copilot/hooks/hooks.json — Personal hooks (preToolUse, postToolUse, sessionStart)
- ~/.copilot/skills/hookflow/SKILL.md — Agent skill for workflow creation guidance

Hookflow hooks are registered with --global flag so they defer to repo-level
hooks when both exist. Existing hookflow hooks are detected and replaced;
non-hookflow hooks are preserved.

Use --unregister to remove hookflow hooks and skill.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		unregister, _ := cmd.Flags().GetBool("unregister")
		hooksOnly, _ := cmd.Flags().GetBool("hooks-only")
		skillOnly, _ := cmd.Flags().GetBool("skill-only")

		return runRegister(unregister, hooksOnly, skillOnly)
	},
}

func init() {
	rootCmd.AddCommand(registerCmd)
	registerCmd.Flags().Bool("unregister", false, "Remove hookflow hooks and skill")
	registerCmd.Flags().Bool("hooks-only", false, "Only register hooks (skip skill)")
	registerCmd.Flags().Bool("skill-only", false, "Only register skill (skip hooks)")
}

func runRegister(unregister, hooksOnly, skillOnly bool) error {
	if unregister {
		return runUnregister(hooksOnly, skillOnly)
	}

	doHooks := !skillOnly
	doSkill := !hooksOnly

	if doHooks {
		if err := registerPersonalHooks(); err != nil {
			return err
		}
	}

	if doSkill {
		if err := registerSkill(); err != nil {
			return err
		}
	}

	fmt.Println("\n✓ hookflow registered successfully!")
	if doHooks && doSkill {
		fmt.Println("\nPersonal hooks and skill are now active for all repos.")
	} else if doHooks {
		fmt.Println("\nPersonal hooks are now active for all repos.")
	} else {
		fmt.Println("\nHookflow skill is now available.")
	}
	fmt.Println("\nNext steps:")
	fmt.Println("  1. Initialize a repo: hookflow init")
	fmt.Println("  2. Create a workflow: hookflow create \"block edits to .env files\"")

	return nil
}

func runUnregister(hooksOnly, skillOnly bool) error {
	doHooks := !skillOnly
	doSkill := !hooksOnly

	if doHooks {
		if err := unregisterPersonalHooks(); err != nil {
			return err
		}
	}

	if doSkill {
		if err := unregisterSkill(); err != nil {
			return err
		}
	}

	fmt.Println("\n✓ hookflow unregistered successfully!")
	return nil
}

// registerPersonalHooks creates or merges hookflow hooks into ~/.copilot/hooks/hooks.json
func registerPersonalHooks() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to determine home directory: %w", err)
	}

	hooksDir := filepath.Join(homeDir, ".copilot", "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		return fmt.Errorf("failed to create hooks directory: %w", err)
	}

	hooksFile := filepath.Join(hooksDir, "hooks.json")
	if err := mergePersonalHooksJSON(hooksFile); err != nil {
		return err
	}

	return nil
}

// unregisterPersonalHooks removes hookflow hooks from ~/.copilot/hooks/hooks.json
func unregisterPersonalHooks() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to determine home directory: %w", err)
	}

	hooksFile := filepath.Join(homeDir, ".copilot", "hooks", "hooks.json")

	data, err := os.ReadFile(hooksFile)
	if os.IsNotExist(err) {
		fmt.Println("⚠ No personal hooks.json found — nothing to unregister")
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to read hooks.json: %w", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		fmt.Println("⚠ hooks.json is invalid JSON — nothing to unregister")
		return nil
	}

	hooks, ok := config["hooks"].(map[string]interface{})
	if !ok {
		fmt.Println("⚠ hooks.json has no hooks — nothing to unregister")
		return nil
	}

	// Remove hookflow hooks from each lifecycle
	changed := false
	for _, key := range []string{"preToolUse", "postToolUse", "sessionStart"} {
		arr := getHookArray(hooks, key)
		if containsHookflowHook(arr) {
			hooks[key] = removeHookflowHooks(arr)
			changed = true
		}
	}

	if !changed {
		fmt.Println("⚠ No hookflow hooks found in hooks.json")
		return nil
	}

	// Clean up empty arrays
	for _, key := range []string{"preToolUse", "postToolUse", "sessionStart"} {
		arr := getHookArray(hooks, key)
		if len(arr) == 0 {
			delete(hooks, key)
		}
	}

	// If no hooks remain, remove the file entirely
	if len(hooks) == 0 {
		if err := os.Remove(hooksFile); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove empty hooks.json: %w", err)
		}
		fmt.Printf("✓ Removed %s (no remaining hooks)\n", hooksFile)
		return nil
	}

	jsonBytes, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal hooks.json: %w", err)
	}

	if err := os.WriteFile(hooksFile, jsonBytes, 0644); err != nil {
		return fmt.Errorf("failed to write hooks.json: %w", err)
	}

	fmt.Printf("✓ Removed hookflow hooks from %s (preserved other hooks)\n", hooksFile)
	return nil
}

// registerSkill writes the SKILL.md to ~/.copilot/skills/hookflow/SKILL.md
func registerSkill() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to determine home directory: %w", err)
	}

	skillDir := filepath.Join(homeDir, ".copilot", "skills", "hookflow")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		return fmt.Errorf("failed to create skill directory: %w", err)
	}

	skillFile := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(skillFile, []byte(generateRegisterSkillMD()), 0644); err != nil {
		return fmt.Errorf("failed to write SKILL.md: %w", err)
	}

	fmt.Printf("✓ Created %s\n", skillFile)
	return nil
}

// unregisterSkill removes the hookflow skill directory
func unregisterSkill() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to determine home directory: %w", err)
	}

	skillDir := filepath.Join(homeDir, ".copilot", "skills", "hookflow")
	if _, err := os.Stat(skillDir); os.IsNotExist(err) {
		fmt.Println("⚠ No hookflow skill found — nothing to unregister")
		return nil
	}

	if err := os.RemoveAll(skillDir); err != nil {
		return fmt.Errorf("failed to remove skill directory: %w", err)
	}

	fmt.Printf("✓ Removed %s\n", skillDir)
	return nil
}

// mergePersonalHooksJSON creates or merges hookflow hooks into ~/.copilot/hooks/hooks.json.
// Uses --global flag so personal hooks defer to repo hooks when both exist.
func mergePersonalHooksJSON(path string) error {
	hookflowPreHook := map[string]interface{}{
		"type":       "command",
		"bash":       "gh hookflow run --raw --event-type preToolUse --global",
		"powershell": "gh hookflow run --raw --event-type preToolUse --global",
		"timeoutSec": 1800,
	}
	hookflowPostHook := map[string]interface{}{
		"type":       "command",
		"bash":       "gh hookflow run --raw --event-type postToolUse --global",
		"powershell": "gh hookflow run --raw --event-type postToolUse --global",
		"timeoutSec": 1800,
	}
	sessionStartHook := map[string]interface{}{
		"type":       "command",
		"bash":       `gh hookflow check-setup || echo '{"systemMessage":"⚠️ hookflow not configured. Run: gh extension install htekdev/gh-hookflow && gh hookflow register"}'`,
		"powershell": `gh hookflow check-setup; if ($LASTEXITCODE -ne 0) { Write-Output '{"systemMessage":"hookflow not configured. Run: gh extension install htekdev/gh-hookflow; gh hookflow register"}' }`,
		"timeoutSec": 1800,
		"comment":    "Ensure gh hookflow extension is installed",
	}

	config := map[string]interface{}{
		"version": 1,
		"hooks":   map[string]interface{}{},
	}

	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, &config); err != nil {
			_ = os.Rename(path, path+".bak")
			fmt.Printf("⚠ Backed up invalid %s to %s.bak\n", path, path)
			config = map[string]interface{}{
				"version": 1,
				"hooks":   map[string]interface{}{},
			}
		}
	}

	hooks, ok := config["hooks"].(map[string]interface{})
	if !ok {
		hooks = map[string]interface{}{}
		config["hooks"] = hooks
	}

	// Always replace hookflow hooks (remove old, add new)
	preToolUse := getHookArray(hooks, "preToolUse")
	preToolUse = removeHookflowHooks(preToolUse)
	preToolUse = append(preToolUse, hookflowPreHook)
	hooks["preToolUse"] = preToolUse

	postToolUse := getHookArray(hooks, "postToolUse")
	postToolUse = removeHookflowHooks(postToolUse)
	postToolUse = append(postToolUse, hookflowPostHook)
	hooks["postToolUse"] = postToolUse

	sessionStart := getHookArray(hooks, "sessionStart")
	sessionStart = removeHookflowHooks(sessionStart)
	sessionStart = append(sessionStart, sessionStartHook)
	hooks["sessionStart"] = sessionStart

	jsonBytes, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal hooks.json: %w", err)
	}

	if err := os.WriteFile(path, jsonBytes, 0644); err != nil {
		return fmt.Errorf("failed to write hooks.json: %w", err)
	}

	fmt.Printf("✓ Created %s (personal hooks: preToolUse, postToolUse, sessionStart)\n", path)
	return nil
}

// generateRegisterSkillMD creates the comprehensive SKILL.md for the hookflow skill.
func generateRegisterSkillMD() string {
	return `---
name: hookflow
description: Create and manage hookflow workflows for agent governance. Use this skill when creating, editing, or troubleshooting workflow files in .github/hookflows/. Trigger phrases include "create workflow", "block file edits", "add validation", "hookflow", "agent gate".
---

# Hookflow Governance Rules

This skill helps you create hookflow rules that enforce governance during AI agent sessions.
Rules live in ` + "`" + `.github/hookflows/` + "`" + ` and come in two formats:

1. **Hookify (` + "`" + `.md` + "`" + `) — Recommended.** Simple, declarative markdown rules for pattern matching, blocking commands, and transcript enforcement. Use this for most rules.
2. **YAML Workflows (` + "`" + `.yml` + "`" + `/` + "`" + `.yaml` + "`" + `) — Advanced.** Multi-step workflows with shell scripts, expressions, and complex validation. Use only when you need to run commands or parse output.

**Always default to hookify format** unless the rule requires running shell commands, chaining multiple steps, or evaluating command output. Both formats coexist in the same directory — a deny from either format wins.

` + "```" + `
.github/hookflows/
  block-secrets.md          # hookify rule (recommended)
  require-tests.md          # hookify rule (recommended)
  warn-console-log.md       # hookify rule (recommended)
  coverage-check.yml        # YAML workflow (needs shell commands)
  validate-json.yml         # YAML workflow (needs shell commands)
` + "```" + `

## Quick Reference

| Command | Description |
|---------|-------------|
| ` + "`" + `hookflow register` + "`" + ` | Register personal hooks and skill |
| ` + "`" + `hookflow register --unregister` + "`" + ` | Remove personal hooks and skill |
| ` + "`" + `hookflow init` + "`" + ` | Initialize per-repo hooks |
| ` + "`" + `hookflow init --repo` + "`" + ` | Also create example workflow and scaffolding |
| ` + "`" + `hookflow create "prompt"` + "`" + ` | AI-generate a workflow from description |
| ` + "`" + `hookflow validate` + "`" + ` | Validate all workflows in repo |
| ` + "`" + `hookflow test` + "`" + ` | Test a workflow with simulated events |
| ` + "`" + `hookflow logs` + "`" + ` | View hookflow runtime logs |

---

# Hookify Rules (Recommended)

Hookify rules are markdown files with YAML frontmatter. They are the **recommended format** for most governance rules because they are simpler, more readable, and require no shell scripting.

## When to Use Hookify

- Block a command or file pattern
- Enforce transcript-based requirements (e.g., run tests before commit)
- Warn on code patterns (console.log, TODO comments)
- Quick declarative deny/warn rules
- Post-lifecycle advisory checks

## Hookify Schema

` + "```markdown" + `
---
name: Rule Name              # Required: human-readable name
description: What it does    # Optional: appears in logs and validation output
event: bash                  # Required: bash, file, or all
action: block                # Optional: block or warn (default: warn)
lifecycle: pre               # Optional: pre or post (default: pre)
enabled: true                # Optional: true or false (default: true)
tool_matcher: "powershell"   # Optional: regex to match specific tool names

# Use ONE of pattern OR conditions (not both):

# Option A — Simple pattern (regex matched against the primary field):
pattern: "rm -rf"

# Option B — Conditions (ALL must match — AND logic):
conditions:
  - field: command
    operator: contains
    pattern: "rm -rf"
  - field: transcript
    operator: not_contains
    pattern: "approved"
---

The markdown body below the frontmatter is the **user-facing message** shown
when the rule fires. It supports full markdown formatting — use it to explain
what went wrong and how to fix it.
` + "```" + `

## Frontmatter Fields

| Field | Required | Values | Default | Description |
|-------|----------|--------|---------|-------------|
| ` + "`name`" + ` | Yes | any string | — | Human-readable rule name |
| ` + "`description`" + ` | No | any string | — | Longer description for docs/logs |
| ` + "`event`" + ` | Yes | ` + "`bash`" + `, ` + "`file`" + `, ` + "`all`" + ` | — | Event type to match |
| ` + "`action`" + ` | No | ` + "`block`" + `, ` + "`warn`" + ` | ` + "`warn`" + ` | Block denies the action; warn is advisory |
| ` + "`lifecycle`" + ` | No | ` + "`pre`" + `, ` + "`post`" + ` | ` + "`pre`" + ` | When to evaluate (before or after tool execution) |
| ` + "`enabled`" + ` | No | ` + "`true`" + `, ` + "`false`" + ` | ` + "`true`" + ` | Set to false to disable without deleting |
| ` + "`tool_matcher`" + ` | No | regex string | — | Only match when tool name matches this regex (case-insensitive) |
| ` + "`pattern`" + ` | No* | regex string | — | Simple regex match against the primary field for the event type |
| ` + "`conditions`" + ` | No* | array | — | Complex multi-field matching with AND logic |

*Must have exactly one of ` + "`pattern`" + ` or ` + "`conditions`" + ` — not both, not neither.

## Event Types

| Event | Matches | Primary Field (for ` + "`pattern`" + `) |
|-------|---------|--------------------------------------|
| ` + "`bash`" + ` | Shell/terminal commands (powershell, bash, shell, terminal) | ` + "`command`" + ` |
| ` + "`file`" + ` | File create/edit operations | ` + "`file_path`" + ` |
| ` + "`all`" + ` | Any event type | ` + "`content`" + ` (all args concatenated) |

## Pattern vs Conditions

**` + "`pattern`" + `** is a shorthand for a single regex condition. It automatically targets the primary field for the event type:

- ` + "`event: bash`" + ` + ` + "`pattern: \"rm -rf\"`" + ` → matches ` + "`command`" + ` field with ` + "`regex_match`" + `
- ` + "`event: file`" + ` + ` + "`pattern: \"\\.env$\"`" + ` → matches ` + "`file_path`" + ` field with ` + "`regex_match`" + `
- ` + "`event: all`" + ` + ` + "`pattern: \"secret\"`" + ` → matches ` + "`content`" + ` field with ` + "`regex_match`" + `

**` + "`conditions`" + `** allows multiple checks with different fields and operators. All conditions must match (AND logic):

` + "```yaml" + `
conditions:
  - field: command
    operator: contains
    pattern: "git commit"
  - field: transcript
    operator: not_contains
    pattern: "go test"
` + "```" + `

## Condition Fields

| Field | Description | Available For |
|-------|-------------|---------------|
| ` + "`command`" + ` | The command/script/code being executed | ` + "`bash`" + ` events |
| ` + "`file_path`" + ` | Path of the file being created or edited | ` + "`file`" + ` events |
| ` + "`new_text`" + ` | New content being written (` + "`file_text`" + ` for create, ` + "`new_str`" + ` for edit) | ` + "`file`" + ` events |
| ` + "`old_text`" + ` | Original content being replaced (edit only, from ` + "`old_str`" + `) | ` + "`file`" + ` edit events |
| ` + "`content`" + ` | All tool arguments concatenated (for broad matching) | Any event |
| ` + "`transcript`" + ` | Full session transcript (JSONL of all previous tool calls) | Any event |

## Condition Operators

| Operator | Description | Case Sensitive? |
|----------|-------------|-----------------|
| ` + "`regex_match`" + ` | Matches value against a regex pattern | No (auto ` + "`(?i)`" + `) |
| ` + "`contains`" + ` | Value contains the pattern as a substring | No |
| ` + "`not_contains`" + ` | Value does NOT contain the pattern | No |
| ` + "`equals`" + ` | Value exactly equals the pattern | Yes |
| ` + "`starts_with`" + ` | Value starts with the pattern | No |
| ` + "`ends_with`" + ` | Value ends with the pattern | No |

**Restriction:** The ` + "`content`" + ` field cannot use positional operators (` + "`equals`" + `, ` + "`starts_with`" + `, ` + "`ends_with`" + `) because tool arguments are concatenated in non-deterministic order. Use ` + "`contains`" + `, ` + "`not_contains`" + `, or ` + "`regex_match`" + ` instead.

## Tool Matcher

` + "`tool_matcher`" + ` adds an optional filter on the tool name (case-insensitive regex). This is useful when you want a rule to only fire for specific tools:

` + "```yaml" + `
tool_matcher: "^powershell$"        # Only match powershell tool
tool_matcher: "^(bash|powershell)$" # Match bash or powershell
` + "```" + `

## Disabling Rules

Set ` + "`enabled: false`" + ` to temporarily disable a rule without deleting the file:

` + "```yaml" + `
enabled: false  # Rule will not be evaluated
` + "```" + `

## Lifecycle (pre vs post)

- **pre** (default): Runs BEFORE the action — can block/deny the operation
- **post**: Runs AFTER the action — for advisory validation and notifications

## Message Body

The markdown body after the frontmatter closing ` + "`---`" + ` is the message shown to the agent when the rule fires. Write clear, actionable messages:

- Explain **what** was blocked/warned and **why**
- Tell the agent **how to fix it** or what to do instead
- Use markdown formatting for readability

## Hookify Examples

### Block Dangerous Commands (Simple Pattern)

` + "```markdown" + `
---
name: Block rm -rf
event: bash
action: block
pattern: "rm -rf /"
---

**Blocked:** Running ` + "`rm -rf /`" + ` is dangerous and could destroy the system.

Use targeted ` + "`rm`" + ` commands on specific files instead.
` + "```" + `

### Block Sensitive File Types (File Pattern)

` + "```markdown" + `
---
name: Protect sensitive files
event: file
action: block
pattern: "\\.(env|key|pem|cert)$"
---

**Blocked:** Cannot create or edit sensitive files (` + "`.env`" + `, ` + "`.key`" + `, ` + "`.pem`" + `, ` + "`.cert`" + `).

These files may contain secrets and should be managed manually.
` + "```" + `

### Require Tests Before Commit (Transcript Condition)

` + "```markdown" + `
---
name: Require Tests Before Commit
event: bash
action: block
lifecycle: pre
conditions:
  - field: command
    operator: contains
    pattern: "git commit"
  - field: transcript
    operator: not_contains
    pattern: "go test"
---

You must run ` + "`go test`" + ` before committing.
Run your tests first, then try the commit again.
` + "```" + `

### Warn on console.log in Code (Multi-Condition)

` + "```markdown" + `
---
name: Warn on console.log
event: file
action: warn
conditions:
  - field: file_path
    operator: regex_match
    pattern: "\\.(js|ts|jsx|tsx)$"
  - field: new_text
    operator: contains
    pattern: "console.log"
---

Consider using a proper logging framework instead of ` + "`console.log`" + `.
` + "```" + `

### Block Recursive Delete with Multiple Guards (AND Logic)

` + "```markdown" + `
---
name: Block dangerous rm commands
event: bash
action: block
conditions:
  - field: command
    operator: contains
    pattern: "rm"
  - field: command
    operator: regex_match
    pattern: "-r[f\\s]|--recursive"
  - field: command
    operator: regex_match
    pattern: "/\\s*$|/[*]|\\s/\\s"
---

**Blocked:** Recursive delete targeting root or wildcard paths.

Use specific, targeted paths for file deletion.
` + "```" + `

### Post-Lifecycle Advisory (Warn After Edit)

` + "```markdown" + `
---
name: Remind to update docs
event: file
action: warn
lifecycle: post
conditions:
  - field: file_path
    operator: regex_match
    pattern: "^(cmd|internal)/.*\\.go$"
---

You edited a Go source file. If this changes public behavior, remember to update the relevant documentation.
` + "```" + `

### Block Hardcoded Secrets in Code

` + "```markdown" + `
---
name: Block hardcoded secrets
event: file
action: block
conditions:
  - field: file_path
    operator: regex_match
    pattern: "\\.(js|ts|py|go|rb|java)$"
  - field: new_text
    operator: regex_match
    pattern: "(password|secret|api_key|token)\\s*[:=]\\s*[\"'][^\"']{8,}"
---

**Blocked:** Hardcoded secret detected in source code.

Use environment variables or a secrets manager instead of embedding credentials in code.
` + "```" + `

---

# YAML Workflows (Advanced)

Use YAML workflows when you need to **run shell commands**, chain multiple steps, evaluate command output, or perform complex validation. YAML workflows are more powerful but also more verbose — prefer hookify rules for simple pattern matching.

## When to Use YAML Workflows

- Running shell commands (linters, test runners, validators)
- Multi-step pipelines with conditional logic
- Parsing command output to make decisions
- Coverage threshold enforcement
- Complex file content validation requiring shell tools

## YAML Workflow Schema

### Required Fields

` + "```yaml" + `
name: string          # Human-readable workflow name (required)
on: object            # Trigger configuration (required)
steps: array          # Steps to execute (required)
` + "```" + `

### Optional Fields

` + "```yaml" + `
description: string   # What the workflow does
blocking: boolean     # Block on failure (default: true)
env: object           # Environment variables for all steps
concurrency:          # Concurrency control
  group: string       # Concurrency group identifier
  max-parallel: int   # Max parallel executions (default: 1)
` + "```" + `

## Trigger Types

### Lifecycle (pre vs post)

All triggers support a ` + "`lifecycle`" + ` field:

- **pre** (default): Runs BEFORE the action — can block/deny the operation
- **post**: Runs AFTER the action — for validation, linting, notifications

` + "```yaml" + `
# Block before file is created (pre)
on:
  file:
    lifecycle: pre
    paths: ['**/*.env']
    types: [create]

# Lint after file is edited (post)
on:
  file:
    lifecycle: post
    paths: ['**/*.ts']
    types: [edit]
` + "```" + `

### File Trigger

Matches file create/edit/delete operations.

` + "```yaml" + `
on:
  file:
    lifecycle: pre        # pre (default) or post
    paths:                # File patterns to match (glob supported)
      - '**/*.env'
      - 'secrets/**'
    paths-ignore:         # Patterns to exclude
      - '**/*.md'
    types:                # Event types: create, edit, delete
      - edit
      - create
` + "```" + `

### Tool Trigger

Matches specific tool calls with argument patterns.

` + "```yaml" + `
# Single tool trigger
on:
  tool:
    name: edit          # Tool name: edit, create, powershell, bash, etc.
    args:
      path: '**/secrets/**'  # Glob pattern for argument values
    if: contains(event.tool.args.path, 'secret')  # Optional expression condition
` + "```" + `

### Multiple Tool Triggers

Match multiple tools in a single workflow.

` + "```yaml" + `
on:
  tools:
    - name: powershell
      args:
        command: '*rm -rf*'
    - name: bash
      args:
        command: '*rm -rf*'
` + "```" + `

### Commit Trigger

Matches git commit events.

` + "```yaml" + `
on:
  commit:
    lifecycle: pre        # pre (default) or post
    paths:                # Files that must be in the commit
      - 'src/**'
    paths-ignore:
      - '**/*.md'
    branches:             # Branch filters
      - main
    branches-ignore:      # Branches to exclude
      - 'experimental/*'
` + "```" + `

### Push Trigger

Matches git push events. Pushes must go through ` + "`" + `gh hookflow git-push` + "`" + `.

` + "```yaml" + `
on:
  push:
    lifecycle: pre        # pre (default) or post
    branches:
      - main
      - 'release/*'
    branches-ignore:
      - 'experimental/*'
    tags:
      - 'v*'
    tags-ignore:
      - '*-rc*'
    paths:                # File path filters
      - 'src/**'
    paths-ignore:
      - '**/*.md'
` + "```" + `

## Expression Syntax

Use ` + "`${{ }}`" + ` for dynamic values in YAML workflow steps:

### Event Context

| Expression | Description |
|------------|-------------|
| ` + "`${{ event.file.path }}`" + ` | Path of file being edited |
| ` + "`${{ event.file.action }}`" + ` | Action: edit, create, delete |
| ` + "`${{ event.file.content }}`" + ` | File content (for create) |
| ` + "`${{ event.tool.name }}`" + ` | Tool name being called |
| ` + "`${{ event.tool.args.path }}`" + ` | Tool argument value |
| ` + "`${{ event.tool.args.new_str }}`" + ` | New content (for edit, pre only) |
| ` + "`${{ event.commit.message }}`" + ` | Commit message |
| ` + "`${{ event.commit.sha }}`" + ` | Commit SHA |
| ` + "`${{ event.lifecycle }}`" + ` | Hook lifecycle: pre or post |
| ` + "`${{ event.cwd }}`" + ` | Working directory (absolute) |
| ` + "`${{ env.MY_VAR }}`" + ` | Environment variable |
| ` + "`${{ steps.step_name.outcome }}`" + ` | Previous step outcome |
| ` + "`${{ steps.step_name.outputs }}`" + ` | Previous step outputs |

**Note:** ` + "`event.tool.args.new_str`" + ` is only available during **pre** lifecycle for edit operations.
For **post** lifecycle, use shell commands to read the actual file from disk.

### String Functions

| Function | Example |
|----------|---------|
| ` + "`contains(str, substr)`" + ` | ` + "`${{ contains(event.file.path, '.env') }}`" + ` |
| ` + "`startsWith(str, prefix)`" + ` | ` + "`${{ startsWith(event.file.path, 'src/') }}`" + ` |
| ` + "`endsWith(str, suffix)`" + ` | ` + "`${{ endsWith(event.file.path, '.ts') }}`" + ` |
| ` + "`format(fmt, ...args)`" + ` | ` + "`${{ format('File: {0}', event.file.path) }}`" + ` |
| ` + "`join(array, sep)`" + ` | ` + "`${{ join(event.commit.files, ', ') }}`" + ` |
| ` + "`toJSON(value)`" + ` | ` + "`${{ toJSON(event) }}`" + ` |
| ` + "`fromJSON(str)`" + ` | ` + "`${{ fromJSON(steps.data.outputs) }}`" + ` |

### Step Outcome Functions

Use in ` + "`if:`" + ` conditions to control step execution based on previous step results:

| Function | Description |
|----------|-------------|
| ` + "`success()`" + ` | True if no previous steps failed or were cancelled |
| ` + "`failure()`" + ` | True if any previous step failed |
| ` + "`always()`" + ` | Always true — step runs regardless of previous failures |
| ` + "`cancelled()`" + ` | True if any previous step was cancelled |

` + "```yaml" + `
steps:
  - name: Run tests
    run: |
      go test ./...

  - name: Report failure
    if: failure()
    run: |
      echo "Tests failed — notifying team"

  - name: Always cleanup
    if: always()
    run: |
      rm -f temp-files/*
` + "```" + `

### Transcript Functions

Query the session transcript to inspect what the agent has done previously:

| Function | Description |
|----------|-------------|
| ` + "`transcript()`" + ` | All transcript entries as JSON array |
| ` + "`transcript(regex)`" + ` | Entries matching regex pattern |
| ` + "`transcript_since(regex)`" + ` | Entries after last match of regex |
| ` + "`transcript_count(regex)`" + ` | Count of entries matching regex |
| ` + "`transcript_last(regex)`" + ` | Last entry matching regex as JSON string |

` + "```yaml" + `
# Require tests before allowing source commits
steps:
  - name: Check for test runs
    if: ${{ transcript_count('go test') == 0 }}
    run: |
      echo "No test runs found in session — run tests before committing"
      exit 1
` + "```" + `

### Operators

- Logical: ` + "`||`" + ` (OR), ` + "`&&`" + ` (AND), ` + "`!`" + ` (NOT)
- Comparison: ` + "`==`" + `, ` + "`!=`" + `, ` + "`<`" + `, ` + "`<=`" + `, ` + "`>`" + `, ` + "`>=`" + `
- Property access: ` + "`.`" + ` (dot notation)
- Index access: ` + "`[]`" + ` (array/map indexing)

## Step Configuration

` + "```yaml" + `
steps:
  - name: Step name              # Human-readable name (optional)
    if: ${{ condition }}         # Conditional execution (optional)
    run: |                       # Shell command (required unless uses: is set)
      echo "Running step"
      # exit 1 to deny/block
    shell: pwsh                  # Shell: pwsh (default), bash, sh, cmd
    env:                         # Step-specific env vars (optional)
      MY_VAR: value
    working-directory: ./src     # Working directory (optional)
    timeout: 60                  # Timeout in seconds (optional)
    continue-on-error: true      # Continue workflow on failure (optional)
` + "```" + `

### Reusable Actions

` + "```yaml" + `
steps:
  - name: Use a reusable action
    uses: my-action              # Reference to action
    with:                        # Input parameters
      param1: value1
      param2: value2
` + "```" + `

## YAML Workflow Examples

### Validate JSON Files (Post-Edit)

` + "```yaml" + `
name: Validate JSON
on:
  file:
    lifecycle: post
    paths: ['**/*.json']
    types: [edit, create]
blocking: true
steps:
  - name: Check JSON syntax
    run: |
      $content = Get-Content "${{ event.file.path }}" -Raw
      try {
        $content | ConvertFrom-Json | Out-Null
        Write-Host "Valid JSON"
      } catch {
        Write-Error "Invalid JSON in ${{ event.file.path }}"
        exit 1
      }
` + "```" + `

### Post-Edit Linting (TypeScript)

` + "```yaml" + `
name: Post-Edit TypeScript Lint
on:
  file:
    lifecycle: post
    paths: ['**/*.ts', '**/*.tsx']
    types: [edit]
blocking: false
steps:
  - name: Run ESLint
    run: |
      npx eslint "${{ event.file.path }}" --fix
      echo "Linting complete"
` + "```" + `

### Require Tests for Source Changes

` + "```yaml" + `
name: Require Tests
on:
  commit:
    paths: ['src/**']
    paths-ignore: ['src/**/*.test.*']
blocking: true
steps:
  - name: Check for test files
    run: |
      $files = "${{ event.commit.files }}"
      if ($files -notmatch '\.test\.') {
        Write-Error "Source changes require accompanying tests"
        exit 1
      }
` + "```" + `

### Enforce Conventional Commits

` + "```yaml" + `
name: Conventional Commits
on:
  commit:
    lifecycle: pre
blocking: true
steps:
  - name: Check commit message format
    run: |
      $msg = "${{ event.commit.message }}"
      if ($msg -notmatch '^(feat|fix|docs|style|refactor|perf|test|build|ci|chore|revert)(\(.+\))?!?:') {
        Write-Error "Commit message must follow conventional format: type(scope): description"
        exit 1
      }
` + "```" + `

### Protect Hookflow Governance

` + "```yaml" + `
name: Protect Hookflows
on:
  file:
    lifecycle: pre
    paths: ['.github/hookflows/**']
    types: [edit, create]
blocking: true
steps:
  - name: Block weakening governance
    run: |
      $content = '${{ event.file.content }}'
      if ($content -match 'blocking:\s*false') {
        Write-Error "Cannot set blocking: false on governance workflows"
        exit 1
      }
` + "```" + `

---

# Troubleshooting

## Workflow Not Triggering

1. Check trigger type matches event (file vs tool vs commit vs push)
2. Verify path patterns use correct glob syntax
3. For YAML: ensure ` + "`types`" + ` field matches the action (edit/create/delete)
4. For hookify: ensure ` + "`event`" + ` matches (bash/file/all)
5. Check ` + "`lifecycle`" + ` matches hook type (pre = preToolUse, post = postToolUse)
6. Run ` + "`hookflow logs`" + ` to see what events hookflow received

## Pre vs Post Confusion

- **pre** workflows run in ` + "`preToolUse`" + ` hook — can block actions
- **post** workflows run in ` + "`postToolUse`" + ` hook — run after action completes
- Default is ` + "`pre`" + ` if not specified (both formats)

## Validation Errors

` + "```bash" + `
gh hookflow validate
gh hookflow validate --file .github/hookflows/my-workflow.yml
` + "```" + `

## Testing Workflows

` + "```bash" + `
gh hookflow test --workflow my-workflow --event file --path "test.env"
` + "```" + `

## Git Push

All pushes must go through hookflow governance:

` + "```bash" + `
gh hookflow git-push origin main
gh hookflow git-push-status <activity-id>
` + "```" + `

Never use ` + "`git push`" + ` directly — it is blocked by hookflow primitive guards.
`
}
