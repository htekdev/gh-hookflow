package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/htekdev/gh-hookflow/internal/schema"
	"github.com/htekdev/gh-hookflow/internal/trigger"
	"github.com/spf13/cobra"
)

var testCmd = &cobra.Command{
	Use:   "test",
	Short: "Test a workflow with a mock event",
	Long: `Simulates running a workflow against a mock event without actually executing steps.

This is useful for testing workflow trigger configurations before they run in production.

Examples:
  hookflow test --event commit --workflow lint.yml
  hookflow test --event push --branch main
  hookflow test --event file --action edit --path src/app.ts`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, _ := cmd.Flags().GetString("dir")
		eventType, _ := cmd.Flags().GetString("event")
		workflow, _ := cmd.Flags().GetString("workflow")

		// Event-specific flags
		branch, _ := cmd.Flags().GetString("branch")
		path, _ := cmd.Flags().GetString("path")
		action, _ := cmd.Flags().GetString("action")
		message, _ := cmd.Flags().GetString("message")

		if dir == "" {
			var err error
			dir, err = os.Getwd()
			if err != nil {
				return err
			}
		}

		if eventType == "" {
			return fmt.Errorf("--event is required (commit, push, file)")
		}

		return runTest(dir, eventType, workflow, testEventOptions{
			Branch:  branch,
			Path:    path,
			Action:  action,
			Message: message,
		})
	},
}

type testEventOptions struct {
	Branch  string
	Path    string
	Action  string
	Message string
}

func init() {
	rootCmd.AddCommand(testCmd)

	testCmd.Flags().StringP("dir", "d", "", "Directory to search (default: current directory)")
	testCmd.Flags().StringP("event", "e", "", "Event type to simulate (commit, push, file)")
	testCmd.Flags().StringP("workflow", "w", "", "Specific workflow to test (optional)")

	// Event-specific flags
	testCmd.Flags().String("branch", "main", "Branch name for commit/push events")
	testCmd.Flags().String("path", "", "File path for file events")
	testCmd.Flags().String("action", "edit", "Action for file events (create, edit)")
	testCmd.Flags().String("message", "test commit", "Commit message for commit events")
}

func runTest(dir, eventType, workflow string, opts testEventOptions) error {
	// Build mock event
	evt := buildMockEvent(eventType, opts)

	fmt.Printf("Testing with mock %s event:\n", eventType)
	eventJSON, _ := json.MarshalIndent(evt, "", "  ")
	fmt.Printf("%s\n\n", string(eventJSON))

	// Find workflows to test
	var workflowFiles []string
	if workflow != "" {
		// Test specific workflow
		path, found := findWorkflowFile(dir, workflow)
		if !found {
			return fmt.Errorf("workflow '%s' not found", workflow)
		}
		workflowFiles = append(workflowFiles, path)
	} else {
		// Find all workflows
		workflowDir := filepath.Join(dir, ".github", "hookflows")
		if _, err := os.Stat(workflowDir); os.IsNotExist(err) {
			return fmt.Errorf("no workflows directory found at %s", workflowDir)
		}

		err := filepath.Walk(workflowDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			ext := strings.ToLower(filepath.Ext(path))
			if ext == ".yml" || ext == ".yaml" {
				workflowFiles = append(workflowFiles, path)
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("failed to scan workflows: %w", err)
		}
	}

	if len(workflowFiles) == 0 {
		fmt.Println("No workflows found to test")
		return nil
	}

	// Test each workflow
	fmt.Printf("Testing %d workflow(s):\n\n", len(workflowFiles))

	matchCount := 0
	for _, path := range workflowFiles {
		wf, err := schema.LoadWorkflow(path)
		if err != nil {
			fmt.Printf("✗ %s\n", filepath.Base(path))
			fmt.Printf("  Error loading: %v\n\n", err)
			continue
		}

		matcher := trigger.NewMatcher(wf)
		matches := matcher.Match(evt)

		relPath, _ := filepath.Rel(dir, path)
		if matches {
			matchCount++
			fmt.Printf("✓ %s (%s)\n", wf.Name, relPath)
			fmt.Printf("  Would execute %d step(s):\n", len(wf.Steps))
			for i, step := range wf.Steps {
				stepName := step.Name
				if stepName == "" {
					stepName = fmt.Sprintf("Step %d", i+1)
				}
				fmt.Printf("    %d. %s\n", i+1, stepName)
			}
			if wf.Blocking == nil || *wf.Blocking {
				fmt.Printf("  Blocking: yes (would block if any step fails)\n")
			} else {
				fmt.Printf("  Blocking: no (non-blocking)\n")
			}
		} else {
			fmt.Printf("○ %s (%s)\n", wf.Name, relPath)
			fmt.Printf("  No trigger match for this event\n")
		}
		fmt.Println()
	}

	fmt.Printf("Summary: %d/%d workflow(s) would match\n", matchCount, len(workflowFiles))

	return nil
}

func buildMockEvent(eventType string, opts testEventOptions) *schema.Event {
	evt := &schema.Event{
		Cwd: ".",
	}

	switch eventType {
	case "commit":
		evt.Commit = &schema.CommitEvent{
			SHA:     "abc123",
			Message: opts.Message,
			Author:  "test@example.com",
			Files: []schema.FileStatus{
				{Path: "src/app.ts", Status: "modified"},
			},
		}
		if opts.Path != "" {
			evt.Commit.Files = []schema.FileStatus{
				{Path: opts.Path, Status: "modified"},
			}
		}

	case "push":
		evt.Push = &schema.PushEvent{
			Ref:    "refs/heads/" + opts.Branch,
			Before: "000000",
			After:  "abc123",
		}

	case "file":
		action := opts.Action
		if action == "" {
			action = "edit"
		}
		path := opts.Path
		if path == "" {
			path = "src/app.ts"
		}
		evt.File = &schema.FileEvent{
			Path:   path,
			Action: action,
		}

	case "hook", "tool":
		evt.Hook = &schema.HookEvent{
			Type: "preToolUse",
			Cwd:  ".",
			Tool: &schema.ToolEvent{
				Name: "edit",
				Args: map[string]interface{}{
					"path": opts.Path,
				},
			},
		}
	}

	return evt
}
