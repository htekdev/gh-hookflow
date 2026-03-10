package e2e

import (
	"testing"
)

// TestBlockJSWithConsoleLog verifies that a JS file containing console.log is
// blocked by the content enforcement workflow. (Ports e2e.yml Test 13a)
func TestBlockJSWithConsoleLog(t *testing.T) {
	workspace := setupWorkspace(t)

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      "lib/app.js",
		"file_text": "function main() {\n  console.log(\"hello\");\n}",
	}, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertDeny(t, result, output, "console.log")
}

// TestAllowCleanJSFile verifies that a JS file without console.log is allowed.
// (Ports e2e.yml Test 13b)
func TestAllowCleanJSFile(t *testing.T) {
	workspace := setupWorkspace(t)

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      "lib/utils.js",
		"file_text": "function add(a, b) { return a + b; }",
	}, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
}

// TestBlockEditWithConsoleLog verifies that an edit adding console.log to a TS
// file is blocked. (Ports e2e.yml Test 13c)
func TestBlockEditWithConsoleLog(t *testing.T) {
	workspace := setupWorkspace(t)

	eventJSON := buildEventJSON("edit", map[string]interface{}{
		"path":    "lib/utils.ts",
		"old_str": "return x",
		"new_str": "console.log(x); return x",
	}, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertDeny(t, result, output, "console.log")
}

// TestAllowTSFileWithoutConsoleLog verifies that a clean TS file is allowed.
func TestAllowTSFileWithoutConsoleLog(t *testing.T) {
	workspace := setupWorkspace(t)

	// Use a path that doesn't match other blocking workflows (e.g., paths-ignore-tests
	// blocks src/**). Use lib/ which only triggers content-enforcement.
	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      "lib/clean.ts",
		"file_text": "export function add(a: number, b: number): number { return a + b; }",
	}, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
}
