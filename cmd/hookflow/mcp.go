package main

import (
	"github.com/htekdev/gh-hookflow/internal/mcp"
	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "MCP server commands",
	Long:  "Commands for running the MCP (Model Context Protocol) server.",
}

var mcpServeCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the MCP server",
	Long: `Starts an MCP server that provides hookflow tools to AI agents.

The server communicates over stdin/stdout using JSON-RPC 2.0.
It responds to the following MCP methods:
  - initialize: Returns server capabilities
  - tools/list: Returns available tools
  - tools/call: Executes a tool`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return mcp.Server()
	},
}

func init() {
	rootCmd.AddCommand(mcpCmd)
	mcpCmd.AddCommand(mcpServeCmd)
}
