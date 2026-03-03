package mcp

import (
	"context"
	"fmt"
	"time"

	"github.com/htekdev/gh-hookflow/internal/activity"
	"github.com/htekdev/gh-hookflow/internal/logging"
	"github.com/htekdev/gh-hookflow/internal/push"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// gitPushInput is the input schema for hookflow_git_push
type gitPushInput struct {
	Cwd  string   `json:"cwd" jsonschema:"Working directory (repository root)"`
	Args []string `json:"args" jsonschema:"Git push arguments (e.g. ['origin', 'main'])"`
}

// gitPushOutput is the output for hookflow_git_push
type gitPushOutput struct {
	ActivityID string `json:"activity_id"`
	Status     string `json:"status"`
	Message    string `json:"message"`
	NextStep   string `json:"next_step"`
}

// gitPushStatusInput is the input schema for hookflow_git_push_status
type gitPushStatusInput struct {
	ActivityID string `json:"activity_id" jsonschema:"Activity ID returned by hookflow_git_push"`
}

// gitPushStatusOutput is the output for hookflow_git_push_status
type gitPushStatusOutput struct {
	ActivityID string            `json:"activity_id"`
	Status     string            `json:"status"`
	Message    string            `json:"message"`
	NextStep   string            `json:"next_step,omitempty"`
	PrePush    *push.PhaseResult `json:"pre_push,omitempty"`
	Push       *push.PushPhaseResult `json:"push,omitempty"`
	PostPush   *push.PostPushResult  `json:"post_push,omitempty"`
}

// handleGitPush starts an async git push and returns the activity ID immediately
func handleGitPush(ctx context.Context, req *mcp.CallToolRequest, input gitPushInput) (*mcp.CallToolResult, gitPushOutput, error) {
	log := logging.Context("mcp")
	log.Info("executing hookflow_git_push cwd=%s args=%v", input.Cwd, input.Args)

	if input.Cwd == "" {
		return &mcp.CallToolResult{IsError: true},
			gitPushOutput{Message: "cwd is required"},
			nil
	}

	go func() { _ = activity.CleanupOldActivities(7 * 24 * time.Hour) }()

	act, err := activity.NewActivity(input.Args)
	if err != nil {
		log.Error("failed to create activity: %v", err)
		return &mcp.CallToolResult{IsError: true},
			gitPushOutput{Message: fmt.Sprintf("Failed to create activity: %v", err)},
			nil
	}

	log.Info("created activity %s, starting async push", act.ID)

	// Run the 3-phase push in a goroutine
	go func() {
		resp := push.Run(input.Cwd, input.Args, act, false)
		log.Info("activity %s completed: status=%s", act.ID, resp.Status)
	}()

	return nil, gitPushOutput{
		ActivityID: act.ID,
		Status:     "running",
		Message:    "Git push started. Pre-push workflows are running.",
		NextStep:   fmt.Sprintf("Use the hookflow_git_push_status tool with activity_id '%s' to check progress.", act.ID),
	}, nil
}

// handleGitPushStatus returns the current status of a git push operation
func handleGitPushStatus(ctx context.Context, req *mcp.CallToolRequest, input gitPushStatusInput) (*mcp.CallToolResult, gitPushStatusOutput, error) {
	log := logging.Context("mcp")
	log.Debug("executing hookflow_git_push_status id=%s", input.ActivityID)

	if input.ActivityID == "" {
		return &mcp.CallToolResult{IsError: true},
			gitPushStatusOutput{Message: "activity_id is required"},
			nil
	}

	act, err := activity.LoadActivity(input.ActivityID)
	if err != nil {
		log.Error("failed to load activity: %v", err)
		return &mcp.CallToolResult{IsError: true},
			gitPushStatusOutput{Message: fmt.Sprintf("Activity not found: %v", err)},
			nil
	}

	out := gitPushStatusOutput{
		ActivityID: act.ID,
		Status:     string(act.Status),
		Message:    act.Summary,
	}

	// Map phase statuses to response fields
	if ps, ok := act.Phases[activity.PhasePrePush]; ok && ps.Status != activity.StatusPending {
		out.PrePush = &push.PhaseResult{
			Passed:       ps.Status == activity.StatusCompleted,
			WorkflowsRun: len(ps.Workflows),
		}
	}
	if ps, ok := act.Phases[activity.PhasePush]; ok && ps.Status != activity.StatusPending {
		out.Push = &push.PushPhaseResult{
			Success: ps.Status == activity.StatusCompleted,
			Output:  ps.Output,
		}
	}
	if ps, ok := act.Phases[activity.PhasePostPush]; ok && ps.Status != activity.StatusPending {
		out.PostPush = &push.PostPushResult{
			Passed:       ps.Status == activity.StatusCompleted,
			WorkflowsRun: len(ps.Workflows),
		}
	}

	// Provide next step guidance if still running
	if act.Status == activity.StatusRunning {
		currentPhase := "pre-push workflows"
		if ps, ok := act.Phases[activity.PhasePush]; ok && ps.Status == activity.StatusRunning {
			currentPhase = "git push"
		} else if ps, ok := act.Phases[activity.PhasePostPush]; ok && ps.Status == activity.StatusRunning {
			currentPhase = "post-push workflows"
		}
		out.Message = fmt.Sprintf("Push in progress: %s running.", currentPhase)
		out.NextStep = fmt.Sprintf("Use hookflow_git_push_status with activity_id '%s' to check again.", act.ID)
	}

	return nil, out, nil
}
