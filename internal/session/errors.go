package session

import (
	"fmt"
	"os"
	"time"
)

// WriteError writes an error to the session's error.md file.
// workflowName: name of the workflow that failed
// stepName: name of the step that failed
// errorDetails: captured output/error message
func WriteError(workflowName, stepName, errorDetails string) error {
	if err := EnsureSessionDir(); err != nil {
		return fmt.Errorf("failed to ensure session directory: %w", err)
	}

	errorPath, err := GetErrorFilePath()
	if err != nil {
		return fmt.Errorf("failed to get error file path: %w", err)
	}

	content := formatErrorMarkdown(workflowName, stepName, errorDetails)

	if err := os.WriteFile(errorPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write error file: %w", err)
	}

	return nil
}

// HasError returns true if an error file exists for the current session.
func HasError() (bool, error) {
	errorPath, err := GetErrorFilePath()
	if err != nil {
		return false, err
	}

	_, err = os.Stat(errorPath)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, fmt.Errorf("failed to check error file: %w", err)
}

// ReadAndClearError reads the error file content and deletes it.
// Returns empty string if no error exists.
func ReadAndClearError() (string, error) {
	errorPath, err := GetErrorFilePath()
	if err != nil {
		return "", err
	}

	content, err := os.ReadFile(errorPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("failed to read error file: %w", err)
	}

	if err := os.Remove(errorPath); err != nil {
		return string(content), fmt.Errorf("failed to remove error file: %w", err)
	}

	return string(content), nil
}

// formatErrorMarkdown formats the error as markdown content.
func formatErrorMarkdown(workflowName, stepName, errorDetails string) string {
	timestamp := time.Now().UTC().Format(time.RFC3339)

	return fmt.Sprintf(`# Hookflow Error

**Workflow:** %s
**Step:** %s
**Time:** %s

## Error Details

%s
`, workflowName, stepName, timestamp, errorDetails)
}
