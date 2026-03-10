package e2e

import (
	"os"
	"path/filepath"
	"testing"
)

// ── commit trigger path matching ────────────────────────────────────

func TestCommitTriggerPathsMatch(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"commit-paths.yml": `name: Commit Paths
on:
  commit:
    paths:
      - 'src/**/*.ts'
blocking: true
steps:
  - name: Block src commits
    run: |
      Write-Host "Blocked: src TS file committed"
      exit 1
`,
	})

	// Create the TS file for reference (workspace structure)
	srcDir := filepath.Join(workspace, "src")
	_ = os.MkdirAll(srcDir, 0755)
	_ = os.WriteFile(filepath.Join(srcDir, "app.ts"), []byte("const x = 1;"), 0644)

	// Use fake staged files matching the commit paths trigger
	opts := &hookflowOpts{env: []string{"HOOKFLOW_FAKE_GIT_STAGED_FILES=src/app.ts"}}

	eventJSON := buildShellEventJSON("powershell", "git commit -m 'add app'", workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", opts)
	assertDeny(t, result, output, "")
}

func TestCommitTriggerPathsNoMatch(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"commit-src-only.yml": `name: Commit Src Only
on:
  commit:
    paths:
      - 'src/**/*.ts'
blocking: true
steps:
  - name: Block
    run: |
      Write-Host "Blocked"
      exit 1
`,
	})

	// Staged file NOT in src/ — should not match the trigger
	opts := &hookflowOpts{env: []string{"HOOKFLOW_FAKE_GIT_STAGED_FILES=readme.md"}}

	eventJSON := buildShellEventJSON("powershell", "git commit -m 'update readme'", workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", opts)
	assertAllow(t, result, output)
}

func TestCommitTriggerPathsIgnore(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"commit-ignore.yml": `name: Commit Paths Ignore
on:
  commit:
    paths-ignore:
      - '**/*.md'
      - 'docs/**'
blocking: true
steps:
  - name: Block non-docs
    run: |
      Write-Host "Non-docs commit blocked"
      exit 1
`,
	})

	// Stage ONLY a markdown file — should be allowed (all files ignored)
	opts := &hookflowOpts{env: []string{"HOOKFLOW_FAKE_GIT_STAGED_FILES=notes.md"}}

	eventJSON := buildShellEventJSON("powershell", "git commit -m 'add notes'", workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", opts)
	assertAllow(t, result, output)
}

func TestCommitTriggerPathsIgnorePartialMatch(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"commit-ignore-partial.yml": `name: Commit Ignore Partial
on:
  commit:
    paths-ignore:
      - '**/*.md'
blocking: true
steps:
  - name: Block
    run: |
      Write-Host "Blocked"
      exit 1
`,
	})

	// Stage a markdown AND a Go file — should deny (not all files are ignored)
	opts := &hookflowOpts{
		env: []string{
			`HOOKFLOW_FAKE_GIT_STAGED_FILES=[{"path":"readme.md","status":"A"},{"path":"main.go","status":"A"}]`,
		},
	}

	eventJSON := buildShellEventJSON("powershell", "git commit -m 'add files'", workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", opts)
	assertDeny(t, result, output, "")
}

func TestCommitTriggerBareCommit(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"commit-bare.yml": `name: Bare Commit
on:
  commit:
blocking: true
steps:
  - name: Block all commits
    run: |
      Write-Host "All commits blocked"
      exit 1
`,
	})

	// Bare commit trigger (no paths filter) should match any commit
	opts := &hookflowOpts{env: []string{"HOOKFLOW_FAKE_GIT_STAGED_FILES=test.txt"}}

	eventJSON := buildShellEventJSON("powershell", "git commit -m 'test'", workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", opts)
	assertDeny(t, result, output, "")
}

func TestCommitTriggerConventionalMessage(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"conventional.yml": `name: Conventional Commit
on:
  commit:
blocking: true
steps:
  - name: Check conventional format
    run: |
      $msg = "${{ event.commit.message }}"
      if ($msg -match '^(feat|fix|chore|docs|refactor|test|ci|build|perf|style|revert)(\(.+\))?!?:\s') {
        Write-Host "Valid conventional commit: $msg"
      } else {
        Write-Host "Invalid commit message: $msg"
        exit 1
      }
`,
	})

	opts := &hookflowOpts{env: []string{"HOOKFLOW_FAKE_GIT_STAGED_FILES=test.txt"}}

	// Valid conventional commit
	eventJSON := buildShellEventJSON("powershell", "git commit -m 'feat: add feature'", workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", opts)
	assertAllow(t, result, output)
}

func TestCommitTriggerBadMessage(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"conv-check.yml": `name: Conv Check
on:
  commit:
blocking: true
steps:
  - name: Check message
    run: |
      $msg = "${{ event.commit.message }}"
      if ($msg -match '^(feat|fix|chore|docs|refactor|test|ci|build|perf|style|revert)(\(.+\))?!?:\s') {
        Write-Host "Valid"
      } else {
        Write-Host "Invalid message: $msg"
        exit 1
      }
`,
	})

	opts := &hookflowOpts{env: []string{"HOOKFLOW_FAKE_GIT_STAGED_FILES=test.txt"}}

	// Invalid commit message
	eventJSON := buildShellEventJSON("powershell", "git commit -m 'random update'", workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", opts)
	assertDeny(t, result, output, "")
}
