package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/htekdev/gh-hookflow/internal/logging"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize hookflow globally or for a repository",
	Long: `Initializes hookflow configuration.

Always creates:
- ~/.copilot/skills/hookflow/SKILL.md - AI agent guidance
- .github/hooks/hooks.json - Per-repo hooks (required for Copilot CLI)

With --repo flag, also creates:
- .github/hookflows/example.yml - Example workflow

For global hookflow across all repos, install the hookflow plugin:
  copilot plugin install htekdev/hookflow-gh-copilot-plugin

After running init, you can create workflows using 'hookflow create'
or by manually creating YAML files in .github/hookflows/`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, _ := cmd.Flags().GetString("dir")
		force, _ := cmd.Flags().GetBool("force")
		repo, _ := cmd.Flags().GetBool("repo")

		if dir == "" {
			var err error
			dir, err = os.Getwd()
			if err != nil {
				return err
			}
		}

		return runInit(dir, force, repo)
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().StringP("dir", "d", "", "Directory to initialize (default: current directory)")
	initCmd.Flags().BoolP("force", "f", false, "Overwrite existing configuration")
	initCmd.Flags().BoolP("repo", "r", false, "Also create example workflows and repo scaffolding")
}

func runInit(dir string, force bool, repo bool) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	copilotDir := filepath.Join(homeDir, ".copilot")

	// Always do global setup first
	if err := runGlobalInit(copilotDir, force); err != nil {
		return err
	}

	// Always create per-repo hooks (required for Copilot CLI to discover hookflow)
	fmt.Println()
	if err := runRepoHooksInit(dir, force); err != nil {
		return err
	}

	// If --repo flag, also create example workflow and scaffolding
	if repo {
		fmt.Println()
		if err := runRepoScaffoldInit(dir, force); err != nil {
			return err
		}
	}

	// Print completion message
	fmt.Println("\n✓ hookflow initialized successfully!")
	if !repo {
		fmt.Println("\nRun 'hookflow init --repo' to also create example workflows.")
	} else {
		fmt.Println("\nNext steps:")
		fmt.Println("  1. Create a workflow: hookflow create \"block edits to .env files\"")
		fmt.Println("  2. Or edit the example workflow in .github/hookflows/example.yml")
		fmt.Println("  3. Commit the .github/ directory to enable for your team")
	}
	fmt.Println("\nFor global hookflow across all repos, install the plugin:")
	fmt.Println("  copilot plugin install htekdev/hookflow-gh-copilot-plugin")

	return nil
}

// runGlobalInit creates global configuration in ~/.copilot/
func runGlobalInit(copilotDir string, force bool) error {
	fmt.Println("Setting up global hookflow configuration...")

	// Ensure ~/.copilot/ exists
	if err := os.MkdirAll(copilotDir, 0755); err != nil {
		return fmt.Errorf("failed to create ~/.copilot directory: %w", err)
	}

	// Create ~/.copilot/skills/hookflow/SKILL.md
	skillDir := filepath.Join(copilotDir, "skills", "hookflow")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		return fmt.Errorf("failed to create skill directory: %w", err)
	}
	skillFile := filepath.Join(skillDir, "SKILL.md")
	if _, err := os.Stat(skillFile); err == nil && !force {
		fmt.Printf("⚠ %s already exists (use --force to overwrite)\n", skillFile)
	} else {
		if err := os.WriteFile(skillFile, []byte(generateSkillMD()), 0644); err != nil {
			return fmt.Errorf("failed to create SKILL.md: %w", err)
		}
		fmt.Printf("✓ Created %s\n", skillFile)
	}

	return nil
}

// runRepoHooksInit creates .github/hooks/hooks.json (always needed for Copilot CLI)
func runRepoHooksInit(dir string, force bool) error {
	fmt.Printf("Setting up per-repo hooks in %s...\n", dir)

	hooksDir := filepath.Join(dir, ".github", "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		return fmt.Errorf("failed to create hooks directory: %w", err)
	}

	hooksFile := filepath.Join(hooksDir, "hooks.json")
	if err := mergeRepoHooksJSON(hooksFile, force); err != nil {
		return err
	}

	return nil
}

// runRepoScaffoldInit creates example workflows and other repo scaffolding
func runRepoScaffoldInit(dir string, force bool) error {
	fmt.Printf("Setting up repo scaffolding in %s...\n", dir)

	hookflowsDir := filepath.Join(dir, ".github", "hookflows")
	if err := os.MkdirAll(hookflowsDir, 0755); err != nil {
		return fmt.Errorf("failed to create hookflows directory: %w", err)
	}

	exampleWorkflow := filepath.Join(hookflowsDir, "example.yml")
	if _, err := os.Stat(exampleWorkflow); os.IsNotExist(err) || force {
		if err := os.WriteFile(exampleWorkflow, []byte(generateExampleWorkflow()), 0644); err != nil {
			return fmt.Errorf("failed to create example workflow: %w", err)
		}
		fmt.Printf("✓ Created %s\n", exampleWorkflow)
	} else {
		fmt.Printf("⚠ %s already exists (use --force to overwrite)\n", exampleWorkflow)
	}

	return nil
}

// mergeRepoHooksJSON creates or merges hookflow hooks into .github/hooks/hooks.json
// This is the primary hooks file that Copilot CLI reads from (.github/hooks/ in the repo).
func mergeRepoHooksJSON(path string, force bool) error {
	// Define hookflow hooks
	hookflowPreHook := map[string]interface{}{
		"type":       "command",
		"bash":       "gh hookflow run --raw --event-type preToolUse",
		"powershell": "gh hookflow run --raw --event-type preToolUse",
		"timeoutSec": 1800,
	}
	hookflowPostHook := map[string]interface{}{
		"type":       "command",
		"bash":       "gh hookflow run --raw --event-type postToolUse",
		"powershell": "gh hookflow run --raw --event-type postToolUse",
		"timeoutSec": 1800,
	}
	sessionStartHook := map[string]interface{}{
		"type":       "command",
		"bash":       `gh hookflow check-setup || { gh extension install htekdev/gh-hookflow && gh hookflow init; }`,
		"powershell": `gh hookflow check-setup; if ($LASTEXITCODE -ne 0) { gh extension install htekdev/gh-hookflow; gh hookflow init }`,
		"timeoutSec": 1800,
		"comment":    "Run setup check; if it fails, install the extension and init global settings",
	}

	// Load existing config or create new one
	config := map[string]interface{}{
		"version": 1,
		"hooks":   map[string]interface{}{},
	}

	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, &config); err != nil {
			// If file exists but is invalid JSON, backup and start fresh
			_ = os.Rename(path, path+".bak")
			fmt.Printf("⚠ Backed up invalid %s to %s.bak\n", path, path)
		}
	}

	// Ensure hooks map exists
	hooks, ok := config["hooks"].(map[string]interface{})
	if !ok {
		hooks = map[string]interface{}{}
		config["hooks"] = hooks
	}

	// Merge preToolUse hooks
	preToolUse := getHookArray(hooks, "preToolUse")
	if !containsHookflowHook(preToolUse) || force {
		preToolUse = removeHookflowHooks(preToolUse)
		preToolUse = append(preToolUse, hookflowPreHook)
		hooks["preToolUse"] = preToolUse
	}

	// Merge postToolUse hooks
	postToolUse := getHookArray(hooks, "postToolUse")
	if !containsHookflowHook(postToolUse) || force {
		postToolUse = removeHookflowHooks(postToolUse)
		postToolUse = append(postToolUse, hookflowPostHook)
		hooks["postToolUse"] = postToolUse
	}

	// Merge sessionStart hooks
	sessionStart := getHookArray(hooks, "sessionStart")
	if !containsHookflowHook(sessionStart) || force {
		sessionStart = removeHookflowHooks(sessionStart)
		sessionStart = append(sessionStart, sessionStartHook)
		hooks["sessionStart"] = sessionStart
	}

	// Write merged config
	jsonBytes, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal hooks.json: %w", err)
	}

	if err := os.WriteFile(path, jsonBytes, 0644); err != nil {
		return fmt.Errorf("failed to write hooks.json: %w", err)
	}

	fmt.Printf("✓ Created %s (repo hooks: preToolUse, postToolUse, sessionStart)\n", path)
	return nil
}

// getHookArray safely extracts a hook array from the hooks map
func getHookArray(hooks map[string]interface{}, key string) []interface{} {
	if arr, ok := hooks[key].([]interface{}); ok {
		return arr
	}
	return []interface{}{}
}

// containsHookflowHook checks if any hook in the array references hookflow
func containsHookflowHook(hooks []interface{}) bool {
	for _, h := range hooks {
		hookBytes, err := json.Marshal(h)
		if err != nil {
			continue
		}
		hookStr := string(hookBytes)
		if contains(hookStr, "hookflow") {
			return true
		}
	}
	return false
}

// removeHookflowHooks removes any existing hookflow hooks from the array
func removeHookflowHooks(hooks []interface{}) []interface{} {
	var result []interface{}
	for _, h := range hooks {
		hookBytes, err := json.Marshal(h)
		if err != nil {
			result = append(result, h)
			continue
		}
		hookStr := string(hookBytes)
		if !contains(hookStr, "hookflow") {
			result = append(result, h)
		}
	}
	return result
}

// silentAutoInit performs auto-initialization without stdout output.
// Called when hookflow detects hookflows exist but repo hooks are missing.
func silentAutoInit(dir string) error {
	log := logging.Context("auto-init")

	// Global: create skill file if missing
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	skillDir := filepath.Join(homeDir, ".copilot", "skills", "hookflow")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		return fmt.Errorf("failed to create skill directory: %w", err)
	}
	skillFile := filepath.Join(skillDir, "SKILL.md")
	if _, statErr := os.Stat(skillFile); os.IsNotExist(statErr) {
		if writeErr := os.WriteFile(skillFile, []byte(generateSkillMD()), 0644); writeErr != nil {
			log.Warn("failed to create SKILL.md: %v", writeErr)
		} else {
			log.Info("auto-created skill file: %s", skillFile)
		}
	}

	// Repo: create hooks.json
	hooksDir := filepath.Join(dir, ".github", "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		return fmt.Errorf("failed to create hooks directory: %w", err)
	}

	hooksFile := filepath.Join(hooksDir, "hooks.json")
	if err := silentMergeRepoHooksJSON(hooksFile); err != nil {
		return err
	}
	log.Info("auto-created repo hooks: %s", hooksFile)

	return nil
}

// silentMergeRepoHooksJSON creates or merges hookflow hooks into hooks.json without stdout output.
func silentMergeRepoHooksJSON(path string) error {
	hookflowPreHook := map[string]interface{}{
		"type":       "command",
		"bash":       "gh hookflow run --raw --event-type preToolUse",
		"powershell": "gh hookflow run --raw --event-type preToolUse",
		"timeoutSec": 1800,
	}
	hookflowPostHook := map[string]interface{}{
		"type":       "command",
		"bash":       "gh hookflow run --raw --event-type postToolUse",
		"powershell": "gh hookflow run --raw --event-type postToolUse",
		"timeoutSec": 1800,
	}
	sessionStartHook := map[string]interface{}{
		"type":       "command",
		"bash":       `gh hookflow check-setup || { gh extension install htekdev/gh-hookflow && gh hookflow init; }`,
		"powershell": `gh hookflow check-setup; if ($LASTEXITCODE -ne 0) { gh extension install htekdev/gh-hookflow; gh hookflow init }`,
		"timeoutSec": 1800,
		"comment":    "Run setup check; if it fails, install the extension and init global settings",
	}

	config := map[string]interface{}{
		"version": 1,
		"hooks":   map[string]interface{}{},
	}

	if data, err := os.ReadFile(path); err == nil {
		if jsonErr := json.Unmarshal(data, &config); jsonErr != nil {
			_ = os.Rename(path, path+".bak")
		}
	}

	hooks, ok := config["hooks"].(map[string]interface{})
	if !ok {
		hooks = map[string]interface{}{}
		config["hooks"] = hooks
	}

	preToolUse := getHookArray(hooks, "preToolUse")
	if !containsHookflowHook(preToolUse) {
		preToolUse = removeHookflowHooks(preToolUse)
		preToolUse = append(preToolUse, hookflowPreHook)
		hooks["preToolUse"] = preToolUse
	}

	postToolUse := getHookArray(hooks, "postToolUse")
	if !containsHookflowHook(postToolUse) {
		postToolUse = removeHookflowHooks(postToolUse)
		postToolUse = append(postToolUse, hookflowPostHook)
		hooks["postToolUse"] = postToolUse
	}

	sessionStart := getHookArray(hooks, "sessionStart")
	if !containsHookflowHook(sessionStart) {
		sessionStart = removeHookflowHooks(sessionStart)
		sessionStart = append(sessionStart, sessionStartHook)
		hooks["sessionStart"] = sessionStart
	}

	jsonBytes, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal hooks.json: %w", err)
	}

	if err := os.WriteFile(path, jsonBytes, 0644); err != nil {
		return fmt.Errorf("failed to write hooks.json: %w", err)
	}

	return nil
}

// contains is a simple substring check
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// generateExampleWorkflowcreates an example workflow file
func generateExampleWorkflow() string {
	return `# Example hookflow workflow
# Learn more: https://github.com/htekdev/gh-hookflow

name: Example Workflow
description: An example workflow that demonstrates hookflow features

# This workflow is disabled by default - rename or modify to enable
on:
  file:
    paths:
      - '**/.env'
      - '**/.env.*'
    types:
      - edit
      - create

blocking: true

steps:
  - name: Block sensitive file edits
    run: |
      echo "⚠️ Editing environment files requires review"
      echo "File: ${{ event.file.path }}"
      # Uncomment the next line to actually block:
      # exit 1
`
}

// generateSkillMD creates the SKILL.md file for AI agent guidance
func generateSkillMD() string {
	return `---
name: hookflow
description: Create and manage hookflow workflows for agent governance. Use this skill when creating, editing, or troubleshooting workflow files in .github/hookflows/. Trigger phrases include "create workflow", "block file edits", "add validation", "hookflow", "agent gate".
---

# Hookflow Workflow Creation

This skill helps you create hookflow workflow files that enforce governance during AI agent sessions.

## When to Use This Skill

- Creating new workflow files in ` + "`" + `.github/hookflows/` + "`" + `
- Editing existing hookflow workflows
- Troubleshooting workflow triggers or validation
- Understanding the hookflow schema

## Workflow Schema

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
env: object          # Environment variables
concurrency: string   # Concurrency group name
` + "```" + `

## Trigger Types

### Lifecycle (pre vs post)

All triggers support a ` + "`lifecycle`" + ` field to control when workflows run:

- **pre** (default): Runs BEFORE the action - can block/deny the operation
- **post**: Runs AFTER the action - for validation, linting, notifications

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
    paths:              # File patterns to match (glob supported)
      - '**/*.env'
      - 'secrets/**'
    paths-ignore:       # Patterns to exclude
      - '**/*.md'
    types:              # Event types: create, edit, delete
      - edit
      - create
` + "```" + `

### Tool Trigger

Matches specific tool calls with argument patterns.

` + "```yaml" + `
on:
  tool:
    name: edit          # Tool name: edit, create, powershell, bash, etc.
    args:
      path: '**/secrets/**'  # Glob pattern for argument values
` + "```" + `

### Commit Trigger

Matches git commit events.

` + "```yaml" + `
on:
  commit:
    lifecycle: pre        # pre (default) or post
    paths:              # Files that must be in the commit
      - 'src/**'
    paths-ignore:
      - '**/*.md'
` + "```" + `

### Push Trigger

Matches git push events.

` + "```yaml" + `
on:
  push:
    lifecycle: pre        # pre (default) or post
    branches:
      - main
      - 'release/*'
    tags:
      - 'v*'
` + "```" + `

## Expression Syntax

Use ` + "`${{ }}`" + ` for dynamic values:

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
| ` + "`${{ env.MY_VAR }}`" + ` | Environment variable |

**Note:** ` + "`event.tool.args.new_str`" + ` is only available during **pre** lifecycle for edit operations. 
For **post** lifecycle, use shell commands to read the actual file from disk.

### Functions

| Function | Example |
|----------|---------|
| ` + "`contains(str, substr)`" + ` | ` + "`${{ contains(event.file.path, '.env') }}`" + ` |
| ` + "`startsWith(str, prefix)`" + ` | ` + "`${{ startsWith(event.file.path, 'src/') }}`" + ` |
| ` + "`endsWith(str, suffix)`" + ` | ` + "`${{ endsWith(event.file.path, '.ts') }}`" + ` |
| ` + "`format(fmt, ...args)`" + ` | ` + "`${{ format('File: {0}', event.file.path) }}`" + ` |

## Step Configuration

` + "```yaml" + `
steps:
  - name: Step name        # Human-readable name (required)
    if: ${{ condition }}   # Conditional execution (optional)
    run: |                 # Shell command (required)
      echo "Running step"
      # exit 1 to deny/block
    env:                   # Step-specific env vars (optional)
      MY_VAR: value
    shell: pwsh           # Shell: pwsh (default, cross-platform)
    timeout: 60            # Timeout in seconds (optional)
` + "```" + `

## Common Patterns

### Block Sensitive Files

` + "```yaml" + `
name: Block Sensitive Files
on:
  file:
    paths:
      - '**/.env*'
      - '**/secrets/**'
      - '**/*.pem'
      - '**/*.key'
    types: [edit, create]
blocking: true
steps:
  - name: Deny sensitive file access
    run: |
      echo "❌ Cannot modify sensitive file: ${{ event.file.path }}"
      exit 1
` + "```" + `

### Validate JSON Files

` + "```yaml" + `
name: Validate JSON
on:
  file:
    paths: ['**/*.json']
    types: [edit, create]
blocking: true
steps:
  - name: Check JSON syntax
    run: |
      cat "${{ event.file.path }}" | jq . > /dev/null
      echo "✓ Valid JSON"
` + "```" + `

### Post-Edit Linting (TypeScript)

` + "```yaml" + `
name: Post-Edit TypeScript Lint
on:
  file:
    lifecycle: post        # Run AFTER the edit
    paths: ['**/*.ts', '**/*.tsx']
    types: [edit]
blocking: false            # Non-blocking - just report
steps:
  - name: Run ESLint
    run: |
      npx eslint "${{ event.file.path }}" --fix
      echo "✓ Linting complete"
` + "```" + `

### Block Password Strings (Pre-Edit)

` + "```yaml" + `
name: Block Hardcoded Passwords
on:
  file:
    lifecycle: pre
    paths: ['**/*.js', '**/*.ts', '**/*.py']
    types: [edit, create]
blocking: true
steps:
  - name: Check for passwords
    if: contains(event.tool.args.new_str, 'password')
    run: |
      echo "❌ Hardcoded password detected in edit"
      exit 1
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
      if ! echo "${{ event.commit.files }}" | grep -q '\.test\.'; then
        echo "❌ Source changes require accompanying tests"
        exit 1
      fi
` + "```" + `

## Troubleshooting

### Workflow Not Triggering

1. Check trigger type matches event (file vs tool vs commit)
2. Verify path patterns use correct glob syntax
3. Ensure ` + "`types`" + ` field matches the action (edit/create/delete)
4. Check ` + "`lifecycle`" + ` matches hook type (pre = preToolUse, post = postToolUse)

### Pre vs Post Confusion

- **pre** workflows run in ` + "`preToolUse`" + ` hook - can block actions
- **post** workflows run in ` + "`postToolUse`" + ` hook - run after action completes
- Default is ` + "`pre`" + ` if not specified

### Validation Errors

Run ` + "`gh hookflow validate`" + ` to check workflow syntax:

` + "```bash" + `
gh hookflow validate --file .github/hookflows/my-workflow.yml
` + "```" + `

### Testing Workflows

Use ` + "`gh hookflow test`" + ` to simulate events:

` + "```bash" + `
gh hookflow test --workflow my-workflow --event file --path "test.env"
` + "```" + `
`
}
