// Package mcp provides an MCP (Model Context Protocol) server implementation
// for hookflow, using the official Go SDK.
package mcp

import (
	"context"

	"github.com/htekdev/gh-hookflow/internal/logging"
	"github.com/htekdev/gh-hookflow/internal/session"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// getErrorInput is the input schema for hookflow_get_error (no params needed)
type getErrorInput struct{}

// getErrorOutput is the output for hookflow_get_error
type getErrorOutput struct {
	Message string `json:"message"`
}

// handleGetError reads and clears the current hookflow error
func handleGetError(ctx context.Context, req *mcp.CallToolRequest, input getErrorInput) (*mcp.CallToolResult, getErrorOutput, error) {
	log := logging.Context("mcp")
	log.Debug("executing hookflow_get_error")

	content, err := session.ReadAndClearError()
	if err != nil {
		log.Error("failed to read error: %v", err)
		return &mcp.CallToolResult{IsError: true}, getErrorOutput{Message: err.Error()}, nil
	}

	if content == "" {
		log.Debug("no pending errors")
		return nil, getErrorOutput{Message: "No pending errors"}, nil
	}

	log.Info("returned and cleared error")
	return nil, getErrorOutput{Message: content}, nil
}

// Server runs the MCP server over stdin/stdout.
func Server() error {
	log := logging.Context("mcp")
	log.Info("MCP server starting")

	server := mcp.NewServer(
		&mcp.Implementation{
			Name:    "hookflow",
			Version: "1.0.0",
		},
		nil,
	)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "hookflow_get_error",
		Description: "Get and clear the current hookflow error. Call this when blocked by a previous validation failure.",
	}, handleGetError)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "hookflow_git_push",
		Description: "Start a git push with pre/post workflow validation. Returns an activity_id to track progress. Use hookflow_git_push_status to check the result.",
	}, handleGitPush)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "hookflow_git_push_status",
		Description: "Check the status of a git push operation started by hookflow_git_push.",
	}, handleGitPushStatus)

	err := server.Run(context.Background(), &mcp.StdioTransport{})
	log.Info("MCP server shutting down")
	return err
}
