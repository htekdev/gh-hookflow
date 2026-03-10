package e2e

import (
	"path/filepath"
	"testing"
)

// ── hooks trigger with types filter ─────────────────────────────────

func TestHooksTriggerPreToolUseOnly(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"pre-only.yml": `name: Pre Only
on:
  hooks:
    types: [preToolUse]
blocking: true
steps:
  - name: Block in pre
    run: |
      echo "Blocked in preToolUse"
      exit 1
`,
	})

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "test.txt"),
		"file_text": "hello",
	}, workspace)

	// preToolUse should trigger the workflow → deny
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertDeny(t, result, output, "")

	// postToolUse should NOT trigger → allow
	result2, output2 := runHookflow(t, workspace, eventJSON, "postToolUse", nil)
	assertAllow(t, result2, output2)
}

func TestHooksTriggerPostToolUseOnly(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"post-only.yml": `name: Post Only
on:
  hooks:
    types: [postToolUse]
blocking: true
steps:
  - name: Block in post
    run: |
      echo "Blocked in postToolUse"
      exit 1
`,
	})

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "test.txt"),
		"file_text": "hello",
	}, workspace)

	// preToolUse should NOT trigger → allow
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
}

// ── hooks trigger with tools filter ─────────────────────────────────

func TestHooksTriggerToolsFilter(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"edit-only-hook.yml": `name: Edit Tool Only
on:
  hooks:
    types: [preToolUse]
    tools: [edit]
blocking: true
steps:
  - name: Block edit
    run: |
      echo "Only edit tool matched"
      exit 1
`,
	})

	// edit tool → should trigger
	editEvent := buildEventJSON("edit", map[string]interface{}{
		"path":    filepath.Join(workspace, "test.txt"),
		"old_str": "hello",
		"new_str": "world",
	}, workspace)
	result, output := runHookflow(t, workspace, editEvent, "preToolUse", nil)
	assertDeny(t, result, output, "")

	// create tool → should NOT trigger
	createEvent := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "new.txt"),
		"file_text": "new",
	}, workspace)
	result2, output2 := runHookflow(t, workspace, createEvent, "preToolUse", nil)
	assertAllow(t, result2, output2)
}

func TestHooksTriggerMultipleTools(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"multi-tools.yml": `name: Multiple Tools
on:
  hooks:
    types: [preToolUse]
    tools: [edit, create]
blocking: true
steps:
  - name: Block edit and create
    run: |
      echo "edit or create matched"
      exit 1
`,
	})

	// edit → should trigger
	editEvent := buildEventJSON("edit", map[string]interface{}{
		"path":    filepath.Join(workspace, "test.txt"),
		"old_str": "hello",
		"new_str": "world",
	}, workspace)
	result, output := runHookflow(t, workspace, editEvent, "preToolUse", nil)
	assertDeny(t, result, output, "")

	// create → should also trigger
	createEvent := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "new.txt"),
		"file_text": "new",
	}, workspace)
	result2, output2 := runHookflow(t, workspace, createEvent, "preToolUse", nil)
	assertDeny(t, result2, output2, "")
}

// ── hooks trigger with no types (matches all) ───────────────────────

func TestHooksTriggerNoTypesFilter(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"all-hooks.yml": `name: All Hooks
on:
  hooks:
    types: [preToolUse, postToolUse]
blocking: true
steps:
  - name: Block everything
    run: |
      echo "All hooks blocked"
      exit 1
`,
	})

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "test.txt"),
		"file_text": "hello",
	}, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertDeny(t, result, output, "")
}

// ── file trigger with types filter ──────────────────────────────────

func TestFileTriggerTypeCreate(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"create-only.yml": `name: Create Only
on:
  file:
    paths: ['**/*.ts']
    types: [create]
blocking: true
steps:
  - name: Block create
    run: |
      echo "Create blocked"
      exit 1
`,
	})

	// create → should trigger
	createEvent := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "app.ts"),
		"file_text": "test",
	}, workspace)
	result, output := runHookflow(t, workspace, createEvent, "preToolUse", nil)
	assertDeny(t, result, output, "")

	// edit → should NOT trigger
	editEvent := buildEventJSON("edit", map[string]interface{}{
		"path":    filepath.Join(workspace, "app.ts"),
		"old_str": "old",
		"new_str": "new",
	}, workspace)
	result2, output2 := runHookflow(t, workspace, editEvent, "preToolUse", nil)
	assertAllow(t, result2, output2)
}

func TestFileTriggerTypeEdit(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"edit-type-only.yml": `name: Edit Only
on:
  file:
    paths: ['**/*.ts']
    types: [edit]
blocking: true
steps:
  - name: Block edit
    run: |
      echo "Edit blocked"
      exit 1
`,
	})

	// edit → should trigger
	editEvent := buildEventJSON("edit", map[string]interface{}{
		"path":    filepath.Join(workspace, "app.ts"),
		"old_str": "old",
		"new_str": "new",
	}, workspace)
	result, output := runHookflow(t, workspace, editEvent, "preToolUse", nil)
	assertDeny(t, result, output, "")

	// create → should NOT trigger
	createEvent := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "app.ts"),
		"file_text": "test",
	}, workspace)
	result2, output2 := runHookflow(t, workspace, createEvent, "preToolUse", nil)
	assertAllow(t, result2, output2)
}
