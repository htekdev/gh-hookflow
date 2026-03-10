//go:build e2etest

package ai

import (
	"context"
	"fmt"
	"os"
	"sync"
)

// Client is the fake AI client used during E2E testing.
// It returns canned responses controlled via environment variables.
type Client struct {
	started bool
	mu      sync.Mutex
}

// NewClient creates a new fake AI client
func NewClient() *Client {
	return &Client{}
}

// Start is a no-op for the fake client
func (c *Client) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.started = true
	return nil
}

// Stop is a no-op for the fake client
func (c *Client) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.started = false
	return nil
}

// GenerateWorkflow returns a canned workflow from the HOOKFLOW_FAKE_AI_RESPONSE
// environment variable, or a sensible default if not set.
func (c *Client) GenerateWorkflow(ctx context.Context, prompt string) (*GenerateWorkflowResult, error) {
	if !c.started {
		return nil, fmt.Errorf("client not started")
	}

	if os.Getenv("HOOKFLOW_FAKE_AI_ERROR") == "1" {
		errMsg := os.Getenv("HOOKFLOW_FAKE_AI_ERROR_MSG")
		if errMsg == "" {
			errMsg = "fake: AI service unavailable"
		}
		return nil, fmt.Errorf("%s", errMsg)
	}

	response := os.Getenv("HOOKFLOW_FAKE_AI_RESPONSE")
	if response == "" {
		response = defaultFakeWorkflow(prompt)
	}

	yaml := extractYAML(response)
	if yaml == "" {
		yaml = response
	}

	name := extractWorkflowName(yaml)

	return &GenerateWorkflowResult{
		Name:        name,
		Description: prompt,
		YAML:        yaml,
	}, nil
}

func defaultFakeWorkflow(prompt string) string {
	return fmt.Sprintf(`name: Generated Workflow
description: %s
on:
  file:
    paths:
      - '**/*'
    types:
      - edit
      - create
blocking: true
steps:
  - name: Validate
    run: |
      echo "Validation step"
      exit 0
`, prompt)
}
