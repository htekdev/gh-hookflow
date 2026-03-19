package e2e

import (
	"strings"
	"testing"
)

// TestPushSuccessfulNoWorkflows tests the full 3-phase push flow with no workflows.
func TestPushSuccessfulNoWorkflows(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{})

	output, err := runHookflowCmd(t,
		[]string{"git-push", "--dir", workspace, "origin", "main"},
		[]string{
			"HOOKFLOW_FAKE_GIT_BRANCH=main",
			"HOOKFLOW_FAKE_GIT_PUSH_OUTPUT=Everything up-to-date",
		},
	)
	if err != nil {
		t.Fatalf("git-push failed: %v\nOutput: %s", err, output)
	}

	if !strings.Contains(output, "Push completed successfully") {
		t.Errorf("expected success message, got:\n%s", output)
	}
	if !strings.Contains(output, "tell the user the push was successful") {
		t.Errorf("expected agent direction to inform user, got:\n%s", output)
	}
}

// TestPushPrePushDenies tests that a pre-push workflow denial prevents the push.
func TestPushPrePushDenies(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"block-push.yml": `name: Block Push
on:
  push:
    branches:
      - main
blocking: true
steps:
  - name: Deny push
    run: |
      Write-Output "Push denied by policy"
      exit 1
`,
	})

	output, _ := runHookflowCmd(t,
		[]string{"git-push", "--dir", workspace, "origin", "main"},
		[]string{
			"HOOKFLOW_FAKE_GIT_BRANCH=main",
		},
	)

	if !strings.Contains(output, "DENIED") {
		t.Errorf("expected DENIED in output, got:\n%s", output)
	}
	if !strings.Contains(output, "NOT pushed") {
		t.Errorf("expected 'NOT pushed' in output, got:\n%s", output)
	}
	if !strings.Contains(output, "fix the issues") {
		t.Errorf("expected remediation direction, got:\n%s", output)
	}
}

// TestPushGitFailure tests behavior when the git push itself fails.
func TestPushGitFailure(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{})

	output, _ := runHookflowCmd(t,
		[]string{"git-push", "--dir", workspace, "origin", "main"},
		[]string{
			"HOOKFLOW_FAKE_GIT_BRANCH=main",
			"HOOKFLOW_FAKE_GIT_PUSH_FAIL=1",
			"HOOKFLOW_FAKE_GIT_PUSH_ERROR=remote rejected: permission denied",
		},
	)

	if !strings.Contains(output, "FAILED") {
		t.Errorf("expected FAILED in output, got:\n%s", output)
	}
	if !strings.Contains(output, "NOT pushed") {
		t.Errorf("expected 'NOT pushed' in output, got:\n%s", output)
	}
	if !strings.Contains(output, "Investigate") {
		t.Errorf("expected investigation direction, got:\n%s", output)
	}
}

// TestPushWithPostPushWorkflows tests the full 3-phase flow with post-push workflows.
func TestPushWithPostPushWorkflows(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"post-push-notify.yml": `name: Post Push Notify
on:
  push:
    lifecycle: post
blocking: false
steps:
  - name: Notify
    run: |
      Write-Output "Push completed, notifying team"
`,
	})

	output, err := runHookflowCmd(t,
		[]string{"git-push", "--dir", workspace, "origin", "main"},
		[]string{
			"HOOKFLOW_FAKE_GIT_BRANCH=main",
			"HOOKFLOW_FAKE_GIT_PUSH_OUTPUT=To github.com:test/repo.git\n   abc1234..def5678  main -> main",
		},
	)
	if err != nil {
		t.Fatalf("git-push failed: %v\nOutput: %s", err, output)
	}

	if !strings.Contains(output, "Push completed successfully") {
		t.Errorf("expected success message, got:\n%s", output)
	}
	if !strings.Contains(output, "Post-push") {
		t.Errorf("expected post-push summary, got:\n%s", output)
	}
}

// TestPushPostPushFailure tests that post-push failure is reported correctly.
func TestPushPostPushFailure(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"post-push-check.yml": `name: Post Push Check
on:
  push:
    lifecycle: post
blocking: true
steps:
  - name: Check CI
    run: |
      Write-Output "CI check failed"
      exit 1
`,
	})

	output, _ := runHookflowCmd(t,
		[]string{"git-push", "--dir", workspace, "origin", "main"},
		[]string{
			"HOOKFLOW_FAKE_GIT_BRANCH=main",
			"HOOKFLOW_FAKE_GIT_PUSH_OUTPUT=Everything up-to-date",
		},
	)

	if !strings.Contains(output, "post-push") || !strings.Contains(output, "FAILED") {
		t.Errorf("expected post-push failure message, got:\n%s", output)
	}
	if !strings.Contains(output, "IS on the remote") {
		t.Errorf("expected clarification that push succeeded, got:\n%s", output)
	}
}

// TestPushOutputContainsAgentDirection verifies the output includes actionable guidance.
func TestPushOutputContainsAgentDirection(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{})

	output, err := runHookflowCmd(t,
		[]string{"git-push", "--dir", workspace, "origin", "main"},
		[]string{
			"HOOKFLOW_FAKE_GIT_BRANCH=main",
			"HOOKFLOW_FAKE_GIT_PUSH_OUTPUT=Everything up-to-date",
		},
	)
	if err != nil {
		t.Fatalf("git-push failed: %v\nOutput: %s", err, output)
	}

	if !strings.Contains(output, "Phase summary") {
		t.Errorf("expected phase summary in output, got:\n%s", output)
	}
	if !strings.Contains(output, "tell the user") {
		t.Errorf("expected agent direction in output, got:\n%s", output)
	}
}
