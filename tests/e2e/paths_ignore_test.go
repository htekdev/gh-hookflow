package e2e

import (
	"testing"
)

// TestPathsIgnoreAllowsTestFiles verifies that files matching paths-ignore patterns
// are allowed even though they match the paths pattern. (Ports e2e.yml Test 10a)
func TestPathsIgnoreAllowsTestFiles(t *testing.T) {
	workspace := setupWorkspace(t)

	testFiles := []string{
		"src/app.test.go",
		"src/utils_test.go",
	}

	for _, file := range testFiles {
		t.Run(file, func(t *testing.T) {
			eventJSON := buildEventJSON("create", map[string]interface{}{
				"path":      file,
				"file_text": "package app",
			}, workspace)

			result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
			assertAllow(t, result, output)
		})
	}
}

// TestPathsBlocksSourceFiles verifies that source files matching the paths pattern
// (and not matching paths-ignore) are blocked. (Ports e2e.yml Test 10b)
func TestPathsBlocksSourceFiles(t *testing.T) {
	workspace := setupWorkspace(t)

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      "src/app.go",
		"file_text": "package app",
	}, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertDeny(t, result, output, "")
}
