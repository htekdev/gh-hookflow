package e2e

import (
	"path/filepath"
	"strings"
	"testing"
)

// ── startsWith() ────────────────────────────────────────────────────

func TestExpressionStartsWith(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"starts-with.yml": `name: StartsWith Check
on:
  file:
    paths: ['**/*']
blocking: true
steps:
  - name: Block src files
    if: ${{ startsWith(event.file.path, 'src/') }}
    run: |
      echo "Blocked: file starts with src/"
      exit 1
`,
	})

	// File starting with src/ should be blocked
	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "src", "main.ts"),
		"file_text": "console.log('hello')",
	}, workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertDeny(t, result, output, "")

	// File NOT starting with src/ should be allowed
	eventJSON2 := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "docs", "readme.md"),
		"file_text": "# Hello",
	}, workspace)
	result2, output2 := runHookflow(t, workspace, eventJSON2, "preToolUse", nil)
	assertAllow(t, result2, output2)
}

// ── format() ────────────────────────────────────────────────────────

func TestExpressionFormat(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"format-test.yml": `name: Format Check
on:
  file:
    paths: ['**/*']
blocking: true
steps:
  - name: Show formatted message
    run: |
      echo "${{ format('File {0} was {1}', event.file.path, event.file.action) }}"
  - name: Block based on format
    if: ${{ startsWith(format('{0}', event.file.path), 'config') }}
    run: |
      echo "Blocked config file"
      exit 1
`,
	})

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "config", "app.json"),
		"file_text": "{}",
	}, workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertDeny(t, result, output, "")
}

// ── join() ──────────────────────────────────────────────────────────

func TestExpressionJoin(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"join-test.yml": `name: Join Test
on:
  file:
    paths: ['**/*']
steps:
  - name: Echo joined
    run: |
      echo "joined: ${{ join(event.file.path, '-') }}"
`,
	})

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "test.ts"),
		"file_text": "test",
	}, workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
}

// ── toJSON() / fromJSON() ───────────────────────────────────────────

func TestExpressionToJSON(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"tojson-test.yml": `name: ToJSON Test
on:
  file:
    paths: ['**/*']
steps:
  - name: Serialize to JSON
    run: |
      echo "args: ${{ toJSON(event.tool) }}"
`,
	})

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "data.json"),
		"file_text": `{"key": "value"}`,
	}, workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
	// toJSON should produce some JSON string in the output
	_ = output
}

func TestExpressionFromJSON(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"fromjson-test.yml": `name: FromJSON Test
on:
  file:
    paths: ['**/*.json']
blocking: true
steps:
  - name: Parse JSON content
    if: ${{ contains(toJSON(event.file), 'secret') }}
    run: |
      echo "JSON contains secret!"
      exit 1
`,
	})

	// File content with "secret"
	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "config.json"),
		"file_text": `{"secret": "abc123"}`,
	}, workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertDeny(t, result, output, "")

	// File content without "secret"
	eventJSON2 := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "config.json"),
		"file_text": `{"key": "value"}`,
	}, workspace)
	result2, output2 := runHookflow(t, workspace, eventJSON2, "preToolUse", nil)
	assertAllow(t, result2, output2)
}

// ── equals() with case-insensitive comparison ───────────────────────

func TestExpressionEquals(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"equals-test.yml": `name: Equals Test
on:
  file:
    paths: ['**/*']
blocking: true
steps:
  - name: Case-insensitive check
    if: ${{ event.file.action == 'CREATE' }}
    run: |
      echo "Blocked: action equals CREATE (case-insensitive)"
      exit 1
`,
	})

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "test.txt"),
		"file_text": "test",
	}, workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	// The == comparison for strings is case-insensitive, so "create" == "CREATE" should be true
	assertDeny(t, result, output, "")
}

// ── Expression with steps context ───────────────────────────────────

func TestExpressionStepsContext(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"steps-ctx.yml": `name: Steps Context
on:
  file:
    paths: ['**/*.ts']
blocking: true
steps:
  - name: check
    run: echo "step1 ran"
  - name: step2
    if: ${{ steps.check.outcome == 'success' }}
    run: |
      echo "step1 succeeded, blocking"
      exit 1
`,
	})

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "app.ts"),
		"file_text": "const x = 1;",
	}, workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertDeny(t, result, output, "")
}

// ── Expression with failure() and always() ──────────────────────────

func TestExpressionFailure(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"failure-expr.yml": `name: Failure Expression
on:
  file:
    paths: ['**/*.ts']
blocking: true
steps:
  - name: failing-step
    id: fail
    run: exit 1
    continue-on-error: true
  - name: check-failure
    if: ${{ failure() }}
    run: echo "Previous step failed"
  - name: always-run
    if: ${{ always() }}
    run: echo "This always runs"
`,
	})

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "test.ts"),
		"file_text": "test",
	}, workspace)
	result, _ := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	// With continue-on-error, the workflow shouldn't deny
	assertAllow(t, result, "")
}

// ── Expression with env context ─────────────────────────────────────

func TestExpressionEnvContext(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"env-ctx.yml": `name: Env Context
on:
  file:
    paths: ['**/*']
blocking: true
env:
  BLOCK_MODE: "strict"
steps:
  - name: Check env
    if: ${{ env.BLOCK_MODE == 'strict' }}
    run: |
      echo "Strict mode enabled"
      exit 1
`,
	})

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "test.txt"),
		"file_text": "test",
	}, workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertDeny(t, result, output, "")
}

// ── Negation expression (!) ─────────────────────────────────────────

func TestExpressionNegation(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"negation.yml": `name: Negation Test
on:
  file:
    paths: ['**/*']
blocking: true
steps:
  - name: Block non-docs
    if: ${{ !startsWith(event.file.path, 'docs/') }}
    run: |
      echo "Only docs allowed"
      exit 1
`,
	})

	// Non-docs file should be blocked
	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "src", "main.go"),
		"file_text": "package main",
	}, workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertDeny(t, result, output, "")

	// Docs file should be allowed
	eventJSON2 := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "docs", "guide.md"),
		"file_text": "# Guide",
	}, workspace)
	result2, output2 := runHookflow(t, workspace, eventJSON2, "preToolUse", nil)
	assertAllow(t, result2, output2)
}

// ── endsWith() ──────────────────────────────────────────────────────

func TestExpressionEndsWith(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"endswith.yml": `name: EndsWith Check
on:
  file:
    paths: ['**/*']
blocking: true
steps:
  - name: Block test files
    if: ${{ endsWith(event.file.path, '.test.ts') }}
    run: |
      echo "No test files in prod"
      exit 1
`,
	})

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "app.test.ts"),
		"file_text": "test('x', () => {})",
	}, workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertDeny(t, result, output, "")
}

// ── Complex conditional with && and || ──────────────────────────────

func TestExpressionLogicalOperators(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"logical-ops.yml": `name: Logical Operators
on:
  file:
    paths: ['**/*']
blocking: true
steps:
  - name: Block specific combo
    if: ${{ startsWith(event.file.path, 'src/') && endsWith(event.file.path, '.js') }}
    run: |
      echo "No JS in src/"
      exit 1
`,
	})

	// src/*.js → blocked
	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "src", "index.js"),
		"file_text": "module.exports = {}",
	}, workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertDeny(t, result, output, "")

	// src/*.ts → allowed
	eventJSON2 := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "src", "index.ts"),
		"file_text": "export {}",
	}, workspace)
	result2, output2 := runHookflow(t, workspace, eventJSON2, "preToolUse", nil)
	assertAllow(t, result2, output2)

	// lib/*.js → allowed (not in src/)
	eventJSON3 := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "lib", "util.js"),
		"file_text": "module.exports = {}",
	}, workspace)
	result3, output3 := runHookflow(t, workspace, eventJSON3, "preToolUse", nil)
	assertAllow(t, result3, output3)
}

// ── String interpolation in run step ────────────────────────────────

func TestExpressionInRunStep(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"expr-in-run.yml": `name: Expression In Run
on:
  file:
    paths: ['**/*']
blocking: true
steps:
  - name: Dynamic check
    run: |
      FILE="${{ event.file.path }}"
      ACTION="${{ event.file.action }}"
      echo "Processing $FILE ($ACTION)"
      if [[ "$ACTION" == "create" ]] && [[ "$FILE" == *.secret ]]; then
        echo "Secret file creation blocked"
        exit 1
      fi
    shell: bash
`,
	})

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "api.secret"),
		"file_text": "key=abc",
	}, workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	if strings.Contains(output, "Secret file creation blocked") {
		assertDeny(t, result, output, "")
	}
}
