package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/htekdev/gh-hookflow/internal/logging"
	"github.com/htekdev/gh-hookflow/internal/session"
	"github.com/spf13/cobra"
)

var checkSetupCmd = &cobra.Command{
	Use:   "check-setup",
	Short: "Validate hookflow is properly configured",
	Long: `Checks that hookflow is correctly set up with global hooks and MCP.

Validates:
1. hookflow binary is accessible
2. ~/.copilot/hooks.json exists and has hookflow hooks
3. ~/.copilot/mcp-config.json exists and has hookflow MCP server

Exit codes:
  0 - All checks passed
  1 - Configuration missing or incomplete`,
	RunE: runCheckSetup,
}

func init() {
	rootCmd.AddCommand(checkSetupCmd)
}

func runCheckSetup(cmd *cobra.Command, args []string) error {
	log := logging.Context("check-setup")

	// Read stdin to extract sessionId from hook event payload.
	// sessionStart hooks receive event data with sessionId on stdin.
	input, _ := io.ReadAll(os.Stdin)
	if len(input) > 0 {
		var raw struct {
			SessionID string `json:"sessionId"`
		}
		if err := json.Unmarshal(input, &raw); err == nil && raw.SessionID != "" {
			if os.Getenv("HOOKFLOW_SESSION_DIR") == "" {
				if dir, err := session.SessionDirForID(raw.SessionID); err == nil {
					_ = os.Setenv("HOOKFLOW_SESSION_DIR", dir)
					log.Debug("set session dir from sessionId: %s", dir)
				}
			}
		}
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	copilotDir := filepath.Join(homeDir, ".copilot")
	allPassed := true

	// Check 1: gh hookflow extension is available
	if isGHHookflowAvailable() {
		fmt.Println("✓ gh hookflow is available")
	} else {
		fmt.Println("✗ gh hookflow not available - Install with: gh extension install htekdev/gh-hookflow")
		allPassed = false
	}

	// Check 2: ~/.copilot/hooks.json has hookflow
	hooksFile := filepath.Join(copilotDir, "hooks.json")
	if hasHookflowHooks(hooksFile) {
		fmt.Println("✓ Global hooks configured in ~/.copilot/hooks.json")
	} else {
		fmt.Println("✗ Global hooks not configured - run 'hookflow init'")
		allPassed = false
	}

	// Check 3: ~/.copilot/mcp-config.json has hookflow MCP
	mcpFile := filepath.Join(copilotDir, "mcp-config.json")
	if hasHookflowMCP(mcpFile) {
		fmt.Println("✓ MCP server registered in ~/.copilot/mcp-config.json")
	} else {
		fmt.Println("✗ MCP server not registered - run 'hookflow init'")
		allPassed = false
	}

	// Toggle sentinel file for global-only mode detection.
	// First call (global sessionStart) creates it; second call (repo sessionStart) deletes it.
	created, err := session.ToggleSentinel()
	if err != nil {
		log.Warn("failed to toggle sentinel: %v", err)
	} else if created {
		log.Debug("sentinel created (global-only mode)")
		fmt.Println("✓ Global-only sentinel created")
	} else {
		log.Debug("sentinel removed (repo hooks active)")
		fmt.Println("✓ Repo hooks detected, sentinel removed")
	}

	if !allPassed {
		return fmt.Errorf("configuration incomplete")
	}

	return nil
}

// isGHHookflowAvailable checks if the gh hookflow extension is installed and working
func isGHHookflowAvailable() bool {
	cmd := exec.Command("gh", "hookflow", "version")
	err := cmd.Run()
	return err == nil
}

// hasHookflowHooks checks if hooks.json exists and contains hookflow in preToolUse or postToolUse
func hasHookflowHooks(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}

	var config struct {
		Hooks struct {
			PreToolUse  []json.RawMessage `json:"preToolUse"`
			PostToolUse []json.RawMessage `json:"postToolUse"`
		} `json:"hooks"`
	}

	if err := json.Unmarshal(data, &config); err != nil {
		return false
	}

	// Check both preToolUse and postToolUse for hookflow references
	allHooks := append(config.Hooks.PreToolUse, config.Hooks.PostToolUse...)
	for _, hook := range allHooks {
		hookStr := string(hook)
		if strings.Contains(hookStr, "hookflow") {
			return true
		}
	}

	return false
}

// hasHookflowMCP checks if mcp-config.json exists and has hookflow MCP server
func hasHookflowMCP(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}

	var config struct {
		MCPServers map[string]json.RawMessage `json:"mcpServers"`
	}

	if err := json.Unmarshal(data, &config); err != nil {
		return false
	}

	_, exists := config.MCPServers["hookflow"]
	return exists
}
