package e2e

import (
	"path/filepath"
	"strings"
	"testing"
)

// ── logical operators ───────────────────────────────────────────────

func TestExpressionOrOperator(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"or-expr.yml": `name: OR Expression
on:
  file:
    paths: ['**/*']
blocking: true
steps:
  - name: Check OR condition
    if: ${{ event.file.action == 'create' || event.file.action == 'edit' }}
    run: |
      Write-Host "Matched create or edit"
      exit 1
`,
	})

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "test.txt"),
		"file_text": "hello",
	}, workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertDeny(t, result, output, "")
}

func TestExpressionAndOperator(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"and-expr.yml": `name: AND Expression
on:
  file:
    paths: ['**/*.ts']
blocking: true
steps:
  - name: Check AND condition
    if: ${{ event.file.action == 'create' && contains(event.file.path, 'src') }}
    run: |
      Write-Host "TS create in src"
      exit 1
`,
	})

	// Create in src → both conditions true
	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "src", "app.ts"),
		"file_text": "const x = 1;",
	}, workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertDeny(t, result, output, "")

	// Create NOT in src → second condition false
	eventJSON2 := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "lib", "util.ts"),
		"file_text": "export {}",
	}, workspace)
	result2, output2 := runHookflow(t, workspace, eventJSON2, "preToolUse", nil)
	assertAllow(t, result2, output2)
}

func TestExpressionNotOperator(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"not-expr.yml": `name: NOT Expression
on:
  file:
    paths: ['**/*']
blocking: true
steps:
  - name: Block non-ts
    if: ${{ !endsWith(event.file.path, '.ts') }}
    run: |
      Write-Host "Non-TS file blocked"
      exit 1
`,
	})

	// .txt file → NOT ts → blocked
	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "readme.txt"),
		"file_text": "hello",
	}, workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertDeny(t, result, output, "")

	// .ts file → NOT(!ts) = ts → NOT block
	eventJSON2 := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "app.ts"),
		"file_text": "export {}",
	}, workspace)
	result2, output2 := runHookflow(t, workspace, eventJSON2, "preToolUse", nil)
	assertAllow(t, result2, output2)
}

// ── comparison operators ────────────────────────────────────────────

func TestExpressionNumericComparison(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"numeric-cmp.yml": `name: Numeric Comparison
on:
  file:
    paths: ['**/*']
steps:
  - name: Number equality
    run: |
      Write-Host "5 equals 5"
    if: ${{ 5 == 5 }}
  - name: Greater than
    run: |
      Write-Host "10 > 3"
    if: ${{ 10 > 3 }}
  - name: Less than
    run: |
      Write-Host "2 < 8"
    if: ${{ 2 < 8 }}
  - name: GTE
    run: |
      Write-Host "5 >= 5"
    if: ${{ 5 >= 5 }}
  - name: LTE
    run: |
      Write-Host "3 <= 7"
    if: ${{ 3 <= 7 }}
  - name: Not equal
    run: |
      Write-Host "1 != 2"
    if: ${{ 1 != 2 }}
`,
	})

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "test.txt"),
		"file_text": "hello",
	}, workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
	// All conditions should have been true → steps should run
	if !strings.Contains(output, "5 equals 5") {
		t.Errorf("Expected numeric equality to pass, output: %s", output)
	}
}

// ── string functions ────────────────────────────────────────────────

func TestExpressionStartsWithPrefix(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"starts-with.yml": `name: StartsWith
on:
  file:
    paths: ['**/*']
blocking: true
steps:
  - name: Check prefix
    if: ${{ startsWith(event.file.path, 'src/') }}
    run: |
      Write-Host "Starts with src/"
      exit 1
`,
	})

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "src", "app.ts"),
		"file_text": "code",
	}, workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertDeny(t, result, output, "")
}

func TestExpressionEndsWithSuffix(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"ends-with.yml": `name: EndsWith
on:
  file:
    paths: ['**/*']
blocking: true
steps:
  - name: Check suffix
    if: ${{ endsWith(event.file.path, '.json') }}
    run: |
      Write-Host "JSON file"
      exit 1
`,
	})

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "config.json"),
		"file_text": "{}",
	}, workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertDeny(t, result, output, "")
}

func TestExpressionContains(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"contains-expr.yml": `name: Contains
on:
  file:
    paths: ['**/*']
steps:
  - name: Check contains
    run: |
      Write-Host "Content contains TODO"
    if: ${{ contains(event.file.content, 'TODO') }}
`,
	})

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "app.ts"),
		"file_text": "// TODO: implement this",
	}, workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
	if !strings.Contains(output, "TODO") {
		t.Logf("Expected contains match output, got: %s", output)
	}
}

// ── step outcomes ───────────────────────────────────────────────────

func TestExpressionStepOutcome(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"step-outcome.yml": `name: Step Outcome
on:
  file:
    paths: ['**/*']
steps:
  - name: first
    run: Write-Host "first step done"
  - name: second
    if: ${{ steps.first.outcome == 'success' }}
    run: Write-Host "first succeeded, running second"
  - name: third
    if: ${{ steps.first.outcome == 'failure' }}
    run: Write-Host "first failed, running third"
`,
	})

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "test.txt"),
		"file_text": "hello",
	}, workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
	if !strings.Contains(output, "running second") {
		t.Errorf("Expected second step to run after first succeeded, output: %s", output)
	}
}

func TestExpressionFailureFunction(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"failure-fn.yml": `name: Failure Function
on:
  file:
    paths: ['**/*']
steps:
  - name: fail-step
    run: exit 1
    continue-on-error: true
  - name: recovery
    if: ${{ failure() }}
    run: Write-Host "Recovery after failure"
`,
	})

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "test.txt"),
		"file_text": "hello",
	}, workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
	if !strings.Contains(output, "Recovery after failure") {
		t.Errorf("Expected failure() to trigger recovery step, output: %s", output)
	}
}

func TestExpressionAlwaysFunction(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"always-fn.yml": `name: Always Function
on:
  file:
    paths: ['**/*']
steps:
  - name: fail
    run: exit 1
    continue-on-error: true
  - name: cleanup
    if: ${{ always() }}
    run: Write-Host "Cleanup always runs"
`,
	})

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "test.txt"),
		"file_text": "hello",
	}, workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
	if !strings.Contains(output, "Cleanup always runs") {
		t.Errorf("Expected always() step to run, output: %s", output)
	}
}

// ── format and fromJSON ─────────────────────────────────────────────

func TestExpressionFormatFunction(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"format-expr.yml": `name: Format Expression
on:
  file:
    paths: ['**/*']
steps:
  - name: Use format
    run: |
      Write-Host "${{ format('File {0} was {1}', event.file.path, event.file.action) }}"
`,
	})

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "demo.txt"),
		"file_text": "demo",
	}, workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
}

// ── environment variables in expressions ────────────────────────────

func TestExpressionEnvAccess(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"env-expr.yml": `name: Env Expression
on:
  file:
    paths: ['**/*']
env:
  MY_VAR: "hello-world"
steps:
  - name: Use env
    run: |
      Write-Host "MY_VAR=$env:MY_VAR"
    if: ${{ env.MY_VAR == 'hello-world' }}
`,
	})

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "test.txt"),
		"file_text": "hello",
	}, workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
	if !strings.Contains(output, "MY_VAR=hello-world") {
		t.Errorf("Expected env var in output, got: %s", output)
	}
}

// ── nested property access ──────────────────────────────────────────

func TestExpressionNestedPropertyAccess(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"nested-prop.yml": `name: Nested Property
on:
  file:
    paths: ['**/*']
steps:
  - name: Access nested
    run: |
      Write-Host "Action=${{ event.file.action }} Path=${{ event.file.path }}"
    if: ${{ event.file.action == 'create' }}
`,
	})

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "test.txt"),
		"file_text": "hello",
	}, workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
	if !strings.Contains(output, "Action=create") {
		t.Errorf("Expected nested property access output, got: %s", output)
	}
}

// ── hook event expressions ──────────────────────────────────────────

func TestExpressionHookType(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"hook-type.yml": `name: Hook Type
on:
  hooks:
    types: [preToolUse]
steps:
  - name: Show hook info
    run: |
      Write-Host "Hook type=${{ event.hook.type }} Tool=${{ event.hook.tool.name }}"
`,
	})

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "test.txt"),
		"file_text": "hello",
	}, workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
	if !strings.Contains(output, "Hook type=preToolUse") {
		t.Logf("Expected hook type in output, got: %s", output)
	}
}
