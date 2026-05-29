# Testing Patterns & Best Practices

This document describes the testing conventions used in this repository. It serves as the reference for test quality analysis and improvements.

## Testing Philosophy

- **No mocks** — prefer real component interactions over mocked interfaces; mocks are only acceptable for unavoidable external dependencies (e.g., third-party APIs with no local equivalent)
- **No test suites** — use plain `TestXxx` functions with `t.Run()` for subtests
- **Table-driven tests** — preferred for functions with multiple input/output combinations
- **Real filesystem** — use `t.TempDir()` for isolated filesystem tests
- **Descriptive names** — test names should explain what's being tested and the scenario

## Assertion Libraries

The repository uses [testify](https://github.com/stretchr/testify) for assertions. All new tests should use testify.

### `require.*` — stop on failure

Use `require` for critical setup steps. If the assertion fails, the test stops immediately.

```go
import "github.com/stretchr/testify/require"

config, err := LoadConfig()
require.NoError(t, err, "config loading should succeed")
require.NotNil(t, config, "config should not be nil")
```

### `assert.*` — continue after failure

Use `assert` for validation checks where multiple failures can be reported together.

```go
import "github.com/stretchr/testify/assert"

result := ProcessData(input)
assert.Equal(t, expected, result, "should process data correctly")
assert.True(t, result.IsValid(), "result should be valid")
assert.Len(t, items, 3, "should have exactly 3 items")
```

### Common anti-patterns → testify replacements

| Anti-pattern | Testify equivalent |
|---|---|
| `if err != nil { t.Fatal(err) }` | `require.NoError(t, err)` |
| `if err == nil { t.Fatal("expected error") }` | `require.Error(t, err)` |
| `if got != want { t.Errorf("got %v, want %v", got, want) }` | `assert.Equal(t, want, got)` |
| `if got == nil { t.Fatal("expected non-nil") }` | `require.NotNil(t, got)` |
| `if len(got) != 3 { t.Errorf("...") }` | `assert.Len(t, got, 3)` |
| `if !strings.Contains(got, sub) { t.Errorf("...") }` | `assert.Contains(t, got, sub)` |

## Table-Driven Tests

Preferred for any function tested with multiple inputs.

```go
func TestContextEvaluate(t *testing.T) {
    tests := []struct {
        name    string
        expr    string
        want    interface{}
        wantErr bool
    }{
        {name: "literal true", expr: "true", want: true},
        {name: "literal false", expr: "false", want: false},
        {name: "event property", expr: "event.cwd", want: "/test/path"},
        {name: "invalid expr", expr: "!!!bad", wantErr: true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := Evaluate(tt.expr)

            if tt.wantErr {
                require.Error(t, err)
                return
            }

            require.NoError(t, err)
            assert.Equal(t, tt.want, got)
        })
    }
}
```

See the expression package tests (`internal/expression/`) for real examples of this pattern in the codebase.

## Unit Tests

Unit tests live in the same package as the code they test (e.g. `internal/runner/runner_test.go`).

### Before (plain Go)

```go
func TestStepWithoutTimeout(t *testing.T) {
    // ...
    results, err := runner.Run(ctx)
    if err != nil {
        t.Fatalf("Expected no error, got %v", err)
    }
    if len(results) != 1 {
        t.Fatalf("Expected 1 result, got %d", len(results))
    }
    if !results[0].Success {
        t.Errorf("Expected success, got error: %v", results[0].Error)
    }
}
```

### After (testify)

```go
func TestStepWithoutTimeout(t *testing.T) {
    // ...
    results, err := runner.Run(ctx)
    require.NoError(t, err, "runner should execute without error")
    require.Len(t, results, 1, "should produce exactly one result")
    assert.True(t, results[0].Success, "step should succeed")
}
```

## End-to-End Tests

E2E tests live in `tests/e2e/` and test the full `hookflow` binary.

They rely on helpers defined in the package (refer to `tests/e2e/helpers_test.go` or similar for the current signatures):

- `setupWorkspaceWithHookflows(t, map[string]string)` — creates a temp workspace with hookflow YAML files
- `runHookflow(t, workspace, eventJSON, lifecycle, env)` — runs the binary and returns `(exitCode, output)`
- `assertAllow(t, result, output)` — asserts the hookflow allowed the action
- `assertDeny(t, result, output, msg)` — asserts the hookflow denied the action
- `buildEventJSON(action, data, cwd)` — constructs a JSON event payload

Example:

```go
func TestMyWorkflow(t *testing.T) {
    workspace := setupWorkspaceWithHookflows(t, map[string]string{
        "my-workflow.yml": `name: My Workflow
on:
  file:
    paths: ['**/*.go']
blocking: true
steps:
  - name: check
    run: exit 1
`,
    })

    eventJSON := buildEventJSON("create", map[string]interface{}{
        "path":      filepath.Join(workspace, "main.go"),
        "file_text": "package main",
    }, workspace)

    result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
    assertDeny(t, result, output, "")
}
```

## Naming Conventions

- `Test<Function>` — tests a single function
- `Test<Function>_<Scenario>` — tests a specific scenario for a function
- `t.Run("scenario name", ...)` — subtests within table-driven tests use readable names

### Examples

```go
func TestEvaluate(t *testing.T) { ... }
func TestEvaluate_WithEnvContext(t *testing.T) { ... }
func TestEvaluate_ReturnsErrorOnInvalidExpr(t *testing.T) { ... }
```

## Assertion Messages

Always include a helpful message as the final argument so test failures are easier to diagnose:

```go
// ❌ Without message — hard to debug
assert.Equal(t, expected, got)

// ✅ With message — immediately tells you what failed
assert.Equal(t, expected, got, "response body should match expected output")
require.NoError(t, err, "parsing config file should not fail")
```

## Test Setup & Cleanup

- Use `t.TempDir()` for filesystem tests — automatically cleaned up
- Use `t.Setenv(key, val)` to set environment variables — automatically restored
- Avoid global state; each test should be independent

```go
func TestAppendEntry(t *testing.T) {
    dir := t.TempDir()
    t.Setenv("HOOKFLOW_SESSION_DIR", dir)
    // ...
}
```

## Running Tests

```bash
# Run all tests
go test ./...

# Run a specific package
go test ./internal/runner/...

# Run a specific test
go test ./internal/expression/... -run TestContextEvaluate

# Run with verbose output
go test -v ./...

# Run with race detection
go test -race ./...

# Run with coverage
go test -cover ./...
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## Coverage Expectations

- Unit tests should cover all exported functions and error paths
- Table-driven tests should cover success, error, and edge cases
- E2E tests focus on integration behaviour, not exhaustive coverage
