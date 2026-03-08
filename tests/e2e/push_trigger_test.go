package e2e

import (
	"testing"
)

// ── push trigger branch matching ────────────────────────────────────

func TestPushTriggerBranchMatch(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"push-main.yml": `name: Push Main
lifecycle: pre
on:
  push:
    branches:
      - main
blocking: true
steps:
  - name: Block push to main
    run: |
      Write-Host "Push to main blocked"
      exit 1
`,
	})

	eventJSON := buildShellEventJSON("powershell", "git push origin main", workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertDeny(t, result, output, "")
}

func TestPushTriggerBranchNoMatch(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"push-main-only.yml": `name: Push Main Only
lifecycle: pre
on:
  push:
    branches:
      - main
blocking: true
steps:
  - name: Block
    run: |
      Write-Host "blocked"
      exit 1
`,
	})

	// Push to feature branch — primitive guard denies ALL git push commands
	// (push trigger matching is unreachable via binary due to primitive guard)
	eventJSON := buildShellEventJSON("powershell", "git push origin feature/test", workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertDeny(t, result, output, "git push")
}

func TestPushTriggerBranchWildcard(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"push-release.yml": `name: Push Release
lifecycle: pre
on:
  push:
    branches:
      - 'release/**'
blocking: true
steps:
  - name: Block release push
    run: |
      Write-Host "Release push blocked"
      exit 1
`,
	})

	eventJSON := buildShellEventJSON("powershell", "git push origin release/v2.0", workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertDeny(t, result, output, "")
}

func TestPushTriggerBranchNegation(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"push-not-dev.yml": `name: Push Not Dev
lifecycle: pre
on:
  push:
    branches:
      - '**'
      - '!dev'
blocking: true
steps:
  - name: Block non-dev push
    run: |
      Write-Host "Non-dev push blocked"
      exit 1
`,
	})

	// Primitive guard blocks all git push — verify deny reason mentions git push
	eventJSON := buildShellEventJSON("powershell", "git push origin dev", workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertDeny(t, result, output, "git push")
}

func TestPushTriggerBranchesIgnore(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"push-ignore.yml": `name: Push Ignore
lifecycle: pre
on:
  push:
    branches-ignore:
      - 'temp/**'
      - 'wip/**'
blocking: true
steps:
  - name: Block
    run: |
      Write-Host "blocked"
      exit 1
`,
	})

	// Primitive guard blocks all git push commands regardless of branch matching
	eventJSON := buildShellEventJSON("powershell", "git push origin temp/experiment", workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertDeny(t, result, output, "git push")
}

func TestPushTriggerTagMatch(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"push-tags.yml": `name: Push Tags
lifecycle: pre
on:
  push:
    tags:
      - 'v*'
blocking: true
steps:
  - name: Block tag push
    run: |
      Write-Host "Tag push blocked"
      exit 1
`,
	})

	eventJSON := buildShellEventJSON("powershell", "git push origin refs/tags/v1.0.0", workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertDeny(t, result, output, "")
}

func TestPushTriggerTagNoMatch(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"push-vtags.yml": `name: Push V Tags
lifecycle: pre
on:
  push:
    tags:
      - 'v*'
blocking: true
steps:
  - name: Block
    run: |
      Write-Host "blocked"
      exit 1
`,
	})

	// Push a branch via git push — primitive guard blocks all git push commands
	eventJSON := buildShellEventJSON("powershell", "git push origin main", workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertDeny(t, result, output, "git push")
}

func TestPushTriggerTagsIgnore(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"push-tags-ignore.yml": `name: Push Tags Ignore
lifecycle: pre
on:
  push:
    tags-ignore:
      - 'beta-*'
blocking: true
steps:
  - name: Block
    run: |
      Write-Host "blocked"
      exit 1
`,
	})

	// Primitive guard blocks all git push commands
	eventJSON := buildShellEventJSON("powershell", "git push origin refs/tags/beta-1", workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertDeny(t, result, output, "git push")
}

func TestPushTriggerBarePush(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"push-bare.yml": `name: Bare Push
lifecycle: pre
on:
  push:
blocking: true
steps:
  - name: Block all pushes
    run: |
      Write-Host "All pushes blocked"
      exit 1
`,
	})

	eventJSON := buildShellEventJSON("powershell", "git push", workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertDeny(t, result, output, "")
}

func TestPushTriggerRefExpression(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"push-ref-check.yml": `name: Push Ref Check
lifecycle: pre
on:
  push:
steps:
  - name: Show ref
    run: |
      Write-Host "Push ref: ${{ event.push.ref }}"
`,
	})

	// Primitive guard blocks all git push — verify proper deny
	eventJSON := buildShellEventJSON("powershell", "git push origin feature/test", workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertDeny(t, result, output, "git push")
}

// ── push trigger with commit event ──────────────────────────────────

func TestCommitAndPushTriggerBothMatch(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"commit-block.yml": `name: Commit Block
lifecycle: pre
on:
  commit:
blocking: true
steps:
  - name: Block commits
    run: |
      Write-Host "Commit blocked"
      exit 1
`,
		"push-block.yml": `name: Push Block
lifecycle: pre
on:
  push:
blocking: true
steps:
  - name: Block pushes
    run: |
      Write-Host "Push blocked"
      exit 1
`,
	})

	// Use fake staged files so the commit trigger can match
	opts := &hookflowOpts{env: []string{"HOOKFLOW_FAKE_GIT_STAGED_FILES=x.txt"}}

	// A git commit should only trigger the commit workflow
	eventJSON := buildShellEventJSON("powershell", "git commit -m 'test'", workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", opts)
	assertDeny(t, result, output, "")
}
