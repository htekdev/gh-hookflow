package mcp

import (
	"fmt"

	"github.com/htekdev/gh-hookflow/internal/logging"
	"github.com/htekdev/gh-hookflow/internal/session"
)

// Tool represents an MCP tool definition
type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema InputSchema `json:"inputSchema"`
}

// InputSchema defines the JSON Schema for tool inputs
type InputSchema struct {
	Type       string         `json:"type"`
	Properties map[string]any `json:"properties"`
	Required   []string       `json:"required"`
}

// GetTools returns the list of available tools
func GetTools() []Tool {
	return []Tool{
		{
			Name:        "hookflow_get_error",
			Description: "Get and clear the current hookflow error. Call this when blocked by a previous validation failure.",
			InputSchema: InputSchema{
				Type:       "object",
				Properties: map[string]any{},
				Required:   []string{},
			},
		},
	}
}

// ExecuteTool runs a tool by name with the given arguments
func ExecuteTool(name string, args map[string]any) (string, error) {
	switch name {
	case "hookflow_get_error":
		return executeGetError()
	default:
		return "", fmt.Errorf("unknown tool: %s", name)
	}
}

// executeGetError reads and clears the current hookflow error
func executeGetError() (string, error) {
	log := logging.Context("mcp")
	log.Debug("executing hookflow_get_error")

	content, err := session.ReadAndClearError()
	if err != nil {
		log.Error("failed to read error: %v", err)
		return "", fmt.Errorf("failed to read error: %w", err)
	}

	if content == "" {
		log.Debug("no pending errors")
		return "No pending errors", nil
	}

	log.Info("returned and cleared error")
	return content, nil
}
