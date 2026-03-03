// Package mcp provides an MCP (Model Context Protocol) server implementation
// for hookflow, using the official Go SDK.
package mcp

import (
	"context"

	"github.com/htekdev/gh-hookflow/internal/logging"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

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

	err := server.Run(context.Background(), &mcp.StdioTransport{})
	log.Info("MCP server shutting down")
	return err
}
