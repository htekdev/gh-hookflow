// Package mcp provides an MCP (Model Context Protocol) server implementation
// for hookflow, enabling AI agents to interact with hookflow tools.
package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"

	"github.com/htekdev/gh-hookflow/internal/logging"
)

// JSON-RPC request structure
type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// JSON-RPC response structure
type jsonRPCResponse struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id"`
	Result  any    `json:"result,omitempty"`
	Error   *Error `json:"error,omitempty"`
}

// Error represents a JSON-RPC error
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// Server info
type serverInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Capabilities
type capabilities struct {
	Tools map[string]any `json:"tools"`
}

// Initialize result
type initializeResult struct {
	ProtocolVersion string       `json:"protocolVersion"`
	ServerInfo      serverInfo   `json:"serverInfo"`
	Capabilities    capabilities `json:"capabilities"`
}

// Tools list result
type toolsListResult struct {
	Tools []Tool `json:"tools"`
}

// Tool call params
type toolCallParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

// Tool call result
type toolCallResult struct {
	Content []contentItem `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

type contentItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// Server runs the MCP server, reading JSON-RPC requests from stdin
// and writing responses to stdout.
func Server() error {
	log := logging.Context("mcp")
	log.Info("MCP server starting")

	scanner := bufio.NewScanner(os.Stdin)
	// Increase buffer size for large messages
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		log.Debug("received: %s", line)

		var req jsonRPCRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			log.Error("failed to parse request: %v", err)
			writeError(nil, -32700, "Parse error", err.Error())
			continue
		}

		response := handleRequest(&req)
		writeResponse(response)
	}

	if err := scanner.Err(); err != nil {
		log.Error("scanner error: %v", err)
		return fmt.Errorf("scanner error: %w", err)
	}

	log.Info("MCP server shutting down")
	return nil
}

func handleRequest(req *jsonRPCRequest) *jsonRPCResponse {
	log := logging.Context("mcp")
	log.Debug("handling method: %s", req.Method)

	switch req.Method {
	case "initialize":
		return handleInitialize(req)
	case "initialized":
		// Client notification that initialization is complete - no response needed
		return nil
	case "tools/list":
		return handleToolsList(req)
	case "tools/call":
		return handleToolsCall(req)
	default:
		log.Warn("unknown method: %s", req.Method)
		return &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &Error{
				Code:    -32601,
				Message: "Method not found",
				Data:    fmt.Sprintf("unknown method: %s", req.Method),
			},
		}
	}
}

func handleInitialize(req *jsonRPCRequest) *jsonRPCResponse {
	return &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: initializeResult{
			ProtocolVersion: "2024-11-05",
			ServerInfo: serverInfo{
				Name:    "hookflow",
				Version: "1.0.0",
			},
			Capabilities: capabilities{
				Tools: map[string]any{},
			},
		},
	}
}

func handleToolsList(req *jsonRPCRequest) *jsonRPCResponse {
	return &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: toolsListResult{
			Tools: GetTools(),
		},
	}
}

func handleToolsCall(req *jsonRPCRequest) *jsonRPCResponse {
	log := logging.Context("mcp")

	var params toolCallParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			log.Error("failed to parse tool call params: %v", err)
			return &jsonRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error: &Error{
					Code:    -32602,
					Message: "Invalid params",
					Data:    err.Error(),
				},
			}
		}
	}

	log.Info("executing tool: %s", params.Name)
	result, err := ExecuteTool(params.Name, params.Arguments)
	if err != nil {
		log.Error("tool execution failed: %v", err)
		return &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: toolCallResult{
				Content: []contentItem{{Type: "text", Text: err.Error()}},
				IsError: true,
			},
		}
	}

	return &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: toolCallResult{
			Content: []contentItem{{Type: "text", Text: result}},
		},
	}
}

func writeResponse(resp *jsonRPCResponse) {
	if resp == nil {
		return
	}
	log := logging.Context("mcp")
	data, err := json.Marshal(resp)
	if err != nil {
		log.Error("failed to marshal response: %v", err)
		return
	}
	fmt.Println(string(data))
	log.Debug("sent: %s", string(data))
}

func writeError(id any, code int, message, data string) {
	resp := &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &Error{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
	writeResponse(resp)
}
