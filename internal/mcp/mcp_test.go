package mcp

import (
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestServerCreation(t *testing.T) {
	// Verify that building the server doesn't panic
	server := mcpsdk.NewServer(
		&mcpsdk.Implementation{
			Name:    "hookflow",
			Version: "1.0.0",
		},
		nil,
	)

	if server == nil {
		t.Fatal("expected non-nil server")
	}
}
