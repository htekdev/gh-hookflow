package e2e

import (
	"testing"
)

// TestBlockEnvFileCreation verifies that creating a .env file is denied by the
// block-sensitive-files workflow. (Ports e2e.yml Test 2)
func TestBlockEnvFileCreation(t *testing.T) {
	workspace := setupWorkspace(t)

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      ".env",
		"file_text": "SECRET=abc123",
	}, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertDeny(t, result, output, "")
}

// TestAllowNormalFileCreation verifies that creating a normal file is allowed.
// (Ports e2e.yml Test 3)
func TestAllowNormalFileCreation(t *testing.T) {
	workspace := setupWorkspace(t)

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      "hello.txt",
		"file_text": "Hello World",
	}, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
}

// TestBlockKeyFileEdit verifies that editing a .key file is denied.
// (Ports e2e.yml Test 4)
func TestBlockKeyFileEdit(t *testing.T) {
	workspace := setupWorkspace(t)

	eventJSON := buildEventJSON("edit", map[string]interface{}{
		"path":    "server.key",
		"old_str": "old",
		"new_str": "new",
	}, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertDeny(t, result, output, "")
}

// TestBlockPemFile verifies that creating a .pem file is denied.
func TestBlockPemFile(t *testing.T) {
	workspace := setupWorkspace(t)

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      "cert.pem",
		"file_text": "-----BEGIN CERTIFICATE-----",
	}, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertDeny(t, result, output, "")
}

// TestBlockSecretFile verifies that creating a .secret file is denied.
func TestBlockSecretFile(t *testing.T) {
	workspace := setupWorkspace(t)

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      "api.secret",
		"file_text": "supersecret",
	}, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertDeny(t, result, output, "")
}

// TestBlockNestedEnvFile verifies that .env files in subdirectories are also blocked.
func TestBlockNestedEnvFile(t *testing.T) {
	workspace := setupWorkspace(t)

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      "config/.env",
		"file_text": "DB_HOST=localhost",
	}, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertDeny(t, result, output, "")
}

// TestAllowNonSensitiveExtensions verifies that files with safe extensions are allowed.
func TestAllowNonSensitiveExtensions(t *testing.T) {
	workspace := setupWorkspace(t)

	files := []struct {
		name string
		path string
	}{
		{"go file", "main.go"},
		{"json file", "config.json"},
		{"yaml file", "workflow.yml"},
		{"markdown", "README.md"},
	}

	for _, f := range files {
		t.Run(f.name, func(t *testing.T) {
			eventJSON := buildEventJSON("create", map[string]interface{}{
				"path":      f.path,
				"file_text": "content",
			}, workspace)
			result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
			assertAllow(t, result, output)
		})
	}
}
