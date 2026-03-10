package e2e

import (
	"path/filepath"
	"testing"
)

// ── tool trigger name matching ──────────────────────────────────────

func TestToolTriggerNameMatch(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"tool-create.yml": `name: Tool Create
on:
  tool:
    name: create
blocking: true
steps:
  - name: Block create tool
    run: |
      Write-Host "Create tool blocked"
      exit 1
`,
	})

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "test.ts"),
		"file_text": "const x = 1;",
	}, workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertDeny(t, result, output, "")
}

func TestToolTriggerNameNoMatch(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"tool-edit-only.yml": `name: Tool Edit Only
on:
  tool:
    name: edit
blocking: true
steps:
  - name: Block
    run: |
      exit 1
`,
	})

	// create tool should NOT match edit trigger
	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "test.txt"),
		"file_text": "hello",
	}, workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
}

func TestToolTriggerWithArgs(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"tool-ts-only.yml": `name: Tool TS Only
on:
  tool:
    name: create
    args:
      path: '**/*.ts'
blocking: true
steps:
  - name: Block TS create
    run: |
      Write-Host "TS creation blocked"
      exit 1
`,
	})

	// .ts file → should match
	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "src", "app.ts"),
		"file_text": "const x = 1;",
	}, workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertDeny(t, result, output, "")

	// .js file → should NOT match
	eventJSON2 := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "src", "app.js"),
		"file_text": "const x = 1;",
	}, workspace)
	result2, output2 := runHookflow(t, workspace, eventJSON2, "preToolUse", nil)
	assertAllow(t, result2, output2)
}

func TestToolTriggerArgMissing(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"tool-with-arg.yml": `name: Tool With Arg
on:
  tool:
    name: create
    args:
      nonexistent_arg: '*'
blocking: true
steps:
  - name: Block
    run: exit 1
`,
	})

	// create event doesn't have nonexistent_arg → should NOT match
	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "test.txt"),
		"file_text": "hello",
	}, workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
}

// ── multiple tool triggers ──────────────────────────────────────────

func TestMultipleToolTriggers(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"multi-tools.yml": `name: Multi Tools
on:
  tools:
    - name: create
      args:
        path: '**/*.env*'
    - name: edit
      args:
        path: '**/*.env*'
blocking: true
steps:
  - name: Block env file access
    run: |
      Write-Host "Env file access blocked"
      exit 1
`,
	})

	// Create .env → should match first tool trigger
	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, ".env"),
		"file_text": "SECRET=xxx",
	}, workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertDeny(t, result, output, "")

	// Edit .env.local → should match second tool trigger
	eventJSON2 := buildEventJSON("edit", map[string]interface{}{
		"path":    filepath.Join(workspace, ".env.local"),
		"old_str": "old",
		"new_str": "new",
	}, workspace)
	result2, output2 := runHookflow(t, workspace, eventJSON2, "preToolUse", nil)
	assertDeny(t, result2, output2, "")

	// Create .txt → should NOT match
	eventJSON3 := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "readme.txt"),
		"file_text": "hello",
	}, workspace)
	result3, output3 := runHookflow(t, workspace, eventJSON3, "preToolUse", nil)
	assertAllow(t, result3, output3)
}

// ── tool trigger with shell commands ────────────────────────────────

func TestToolTriggerShellCommand(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"tool-powershell.yml": `name: Tool PowerShell
on:
  tool:
    name: powershell
    args:
      command: '*rm*'
blocking: true
steps:
  - name: Block destructive commands
    run: |
      Write-Host "Destructive command blocked"
      exit 1
`,
	})

	eventJSON := buildShellEventJSON("powershell", "rm -rf /important", workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertDeny(t, result, output, "")

	eventJSON2 := buildShellEventJSON("powershell", "echo hello", workspace)
	result2, output2 := runHookflow(t, workspace, eventJSON2, "preToolUse", nil)
	assertAllow(t, result2, output2)
}
