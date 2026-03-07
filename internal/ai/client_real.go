//go:build !e2etest

package ai

import (
	"context"
	"fmt"
	"strings"
	"sync"

	copilot "github.com/github/copilot-sdk/go"
)

// Client wraps the Copilot SDK for workflow generation
type Client struct {
	client  *copilot.Client
	started bool
	mu      sync.Mutex
}

// NewClient creates a new AI client
func NewClient() *Client {
	return &Client{}
}

// Start initializes the Copilot client
func (c *Client) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.started {
		return nil
	}

	c.client = copilot.NewClient(&copilot.ClientOptions{
		LogLevel: "error",
	})

	if err := c.client.Start(ctx); err != nil {
		return fmt.Errorf("failed to start Copilot client: %w", err)
	}

	c.started = true
	return nil
}

// Stop shuts down the Copilot client
func (c *Client) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.started || c.client == nil {
		return nil
	}

	c.started = false
	return c.client.Stop()
}

// GenerateWorkflow creates a workflow from a natural language prompt
func (c *Client) GenerateWorkflow(ctx context.Context, prompt string) (*GenerateWorkflowResult, error) {
	if !c.started {
		return nil, fmt.Errorf("client not started")
	}

	session, err := c.client.CreateSession(ctx, &copilot.SessionConfig{
		Model:               "gpt-4o",
		OnPermissionRequest: copilot.PermissionHandler.ApproveAll,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}
	defer func() { _ = session.Destroy() }()

	fullPrompt := buildWorkflowPrompt(prompt)

	var response strings.Builder
	done := make(chan bool)
	var responseErr error

	session.On(func(event copilot.SessionEvent) {
		switch event.Type {
		case "assistant.message":
			if event.Data.Content != nil {
				response.WriteString(*event.Data.Content)
			}
		case "session.idle":
			close(done)
		case "error":
			if event.Data.Error != nil {
				responseErr = fmt.Errorf("session error: %v", *event.Data.Error)
			}
			close(done)
		}
	})

	_, err = session.Send(ctx, copilot.MessageOptions{
		Prompt: fullPrompt,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to send prompt: %w", err)
	}

	<-done

	if responseErr != nil {
		return nil, responseErr
	}

	yaml := extractYAML(response.String())
	if yaml == "" {
		return nil, fmt.Errorf("no valid YAML found in response")
	}

	name := extractWorkflowName(yaml)

	return &GenerateWorkflowResult{
		Name:        name,
		Description: prompt,
		YAML:        yaml,
	}, nil
}
