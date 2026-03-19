package e2e

import (
	"encoding/json"
	"strings"
	"testing"
)

// pushResponse mirrors the JSON structure from push.Response.
type pushResponse struct {
	Status   string          `json:"status"`
	Push     *pushPhaseJSON  `json:"push,omitempty"`
	PrePush  *phaseJSON      `json:"pre_push,omitempty"`
	PostPush *postPushJSON   `json:"post_push,omitempty"`
	Message  string          `json:"message"`
}

type pushPhaseJSON struct {
	Success bool   `json:"success"`
	Output  string `json:"output,omitempty"`
}

type phaseJSON struct {
	Passed       bool `json:"passed"`
	WorkflowsRun int  `json:"workflows_run"`
}

type postPushJSON struct {
	Passed       bool `json:"passed"`
	WorkflowsRun int  `json:"workflows_run"`
}

func parsePushResponse(t *testing.T, output string) *pushResponse {
	t.Helper()

	// The JSON output may be preceded by stderr progress messages; find the JSON object
	start := strings.Index(output, "{")
	if start < 0 {
		t.Fatalf("no JSON object found in output:\n%s", output)
	}
	jsonStr := output[start:]

	var resp pushResponse
	if err := json.Unmarshal([]byte(jsonStr), &resp); err != nil {
		t.Fatalf("failed to parse push response JSON: %v\nOutput:\n%s", err, jsonStr)
	}
	return &resp
}

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

	resp := parsePushResponse(t, output)
	if resp.Status != "completed" {
		t.Errorf("expected status completed, got %s", resp.Status)
	}
	if resp.Push == nil || !resp.Push.Success {
		t.Error("expected push.success = true")
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

	resp := parsePushResponse(t, output)
	if resp.Status != "failed" {
		t.Errorf("expected status failed, got %s", resp.Status)
	}
	if resp.PrePush == nil || resp.PrePush.Passed {
		t.Error("expected pre_push.passed = false")
	}
	if resp.Message == "" {
		t.Error("expected non-empty message on failure")
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

	resp := parsePushResponse(t, output)
	if resp.Status != "failed" {
		t.Errorf("expected status failed, got %s", resp.Status)
	}
	if resp.Push == nil || resp.Push.Success {
		t.Error("expected push.success = false")
	}
	if !strings.Contains(resp.Message, "push failed") && !strings.Contains(resp.Message, "Git push failed") {
		t.Errorf("expected message to mention push failure, got: %s", resp.Message)
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

	resp := parsePushResponse(t, output)
	if resp.Status != "completed" {
		t.Errorf("expected status completed, got %s (message: %s)", resp.Status, resp.Message)
	}
	if resp.PostPush == nil || resp.PostPush.WorkflowsRun < 1 {
		t.Error("expected post_push.workflows_run >= 1")
	}
}

// TestPushPostPushFailure tests that post-push failure is recorded correctly.
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

	resp := parsePushResponse(t, output)
	if resp.Status != "failed" {
		t.Errorf("expected status failed, got %s", resp.Status)
	}
	if resp.Push == nil || !resp.Push.Success {
		t.Error("expected push.success = true (push itself succeeded)")
	}
	if resp.PostPush == nil || resp.PostPush.Passed {
		t.Error("expected post_push.passed = false")
	}
	if !strings.Contains(resp.Message, "post-push") {
		t.Errorf("expected message to mention post-push, got: %s", resp.Message)
	}
}

// TestPushJSONOutputStructure verifies the JSON response has all expected fields.
func TestPushJSONOutputStructure(t *testing.T) {
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

	resp := parsePushResponse(t, output)

	if resp.Status != "completed" && resp.Status != "failed" {
		t.Errorf("expected status 'completed' or 'failed', got %q", resp.Status)
	}
	if resp.Message == "" {
		t.Error("expected non-empty message")
	}
	if resp.Push == nil {
		t.Error("expected push field to be present")
	}
	if resp.PrePush == nil {
		t.Error("expected pre_push field to be present")
	}
	if resp.PostPush == nil {
		t.Error("expected post_push field to be present")
	}
}
