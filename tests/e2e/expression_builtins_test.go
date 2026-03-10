package e2e

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestBuiltinFromJSONObject exercises fromJSON with a JSON object and property access.
// Targets: builtinFromJSON (evaluator.go:615), getProperty (evaluator.go:356)
func TestBuiltinFromJSONObject(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"fromjson-obj.yml": `name: FromJSON Object
on:
  file:
    paths: ["**/*"]
steps:
  - name: parse object
    if: "fromJSON('{\"status\":\"ok\",\"count\":42}').status == 'ok'"
    run: Write-Host "fromJSON object parsed"
`,
	})

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "data.json"),
		"file_text": "{}",
	}, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
}

// TestBuiltinFromJSONArrayIndex exercises fromJSON with array + bracket indexing.
// Targets: builtinFromJSON (evaluator.go:615), getIndex (evaluator.go:390)
func TestBuiltinFromJSONArrayIndex(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"fromjson-arr.yml": `name: FromJSON Array
on:
  file:
    paths: ["**/*"]
steps:
  - name: index zero
    if: "fromJSON('[\"first\",\"second\",\"third\"]')[0] == 'first'"
    run: Write-Host "index 0 works"
  - name: index one
    if: "fromJSON('[\"first\",\"second\",\"third\"]')[1] == 'second'"
    run: Write-Host "index 1 works"
`,
	})

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "data.json"),
		"file_text": "{}",
	}, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
}

// TestBuiltinFromJSONMapIndex exercises getIndex with map[string]interface{} via bracket syntax.
// Targets: getIndex map branch (evaluator.go:390)
func TestBuiltinFromJSONMapIndex(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"fromjson-map-idx.yml": `name: FromJSON Map Index
on:
  file:
    paths: ["**/*"]
steps:
  - name: map bracket access
    if: "fromJSON('{\"key\":\"value\"}')['key'] == 'value'"
    run: Write-Host "map index works"
`,
	})

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "data.json"),
		"file_text": "{}",
	}, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
}

// TestBuiltinFromJSONNested exercises nested property access on fromJSON result.
// Targets: getProperty chain (evaluator.go:356)
func TestBuiltinFromJSONNested(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"fromjson-nested.yml": `name: FromJSON Nested
on:
  file:
    paths: ["**/*"]
steps:
  - name: nested access
    if: "fromJSON('{\"outer\":{\"inner\":\"deep\"}}').outer.inner == 'deep'"
    run: Write-Host "nested works"
`,
	})

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "data.json"),
		"file_text": "{}",
	}, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
}

// TestBuiltinJoinWithSeparator exercises join() with a custom separator.
// Targets: builtinJoin separator branch (evaluator.go:585)
func TestBuiltinJoinWithSeparator(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"join-sep.yml": `name: Join With Sep
on:
  file:
    paths: ["**/*"]
steps:
  - name: join with dash
    if: "join(fromJSON('[\"a\",\"b\",\"c\"]'), '-') == 'a-b-c'"
    run: Write-Host "join with separator works"
`,
	})

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "test.txt"),
		"file_text": "hello",
	}, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
}

// TestBuiltinJoinDefaultSeparator exercises join() with default comma separator.
// Targets: builtinJoin default sep branch (evaluator.go:585)
func TestBuiltinJoinDefaultSeparator(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"join-default.yml": `name: Join Default
on:
  file:
    paths: ["**/*"]
steps:
  - name: join with default comma
    if: "join(fromJSON('[\"x\",\"y\",\"z\"]')) == 'x,y,z'"
    run: Write-Host "join default works"
`,
	})

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "test.txt"),
		"file_text": "hello",
	}, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
}

// TestBuiltinContainsArrayFound exercises contains() with array search (found).
// Targets: builtinContains []interface{} branch (evaluator.go:533)
func TestBuiltinContainsArrayFound(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"contains-arr.yml": `name: Contains Array
on:
  file:
    paths: ["**/*"]
steps:
  - name: find in array
    if: "contains(fromJSON('[\"apple\",\"banana\",\"cherry\"]'), 'banana')"
    run: Write-Host "found in array"
`,
	})

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "test.txt"),
		"file_text": "hello",
	}, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
}

// TestBuiltinContainsArrayNotFound exercises contains() with array search (not found).
// Targets: builtinContains []interface{} loop exhaustion (evaluator.go:533)
func TestBuiltinContainsArrayNotFound(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"contains-notfound.yml": `name: Contains Not Found
on:
  file:
    paths: ["**/*"]
steps:
  - name: not in array skips step
    if: "contains(fromJSON('[\"apple\",\"banana\"]'), 'grape')"
    run: |
      Write-Host "FAIL"
      exit 1
  - name: always runs
    run: Write-Host "passed"
`,
	})

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "test.txt"),
		"file_text": "hello",
	}, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
}

// TestBuiltinBoolCoercion exercises toBool with different types.
// Targets: toBool (evaluator.go:467) — nil, string, int64, float64 branches
func TestBuiltinBoolCoercion(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"bool-coerce.yml": `name: Bool Coercion
on:
  file:
    paths: ["**/*"]
steps:
  - name: null is falsy
    if: "!fromJSON('null')"
    run: Write-Host "null is falsy"
  - name: zero is falsy
    if: "!fromJSON('0')"
    run: Write-Host "zero is falsy"
  - name: nonzero is truthy
    if: fromJSON('1')
    run: Write-Host "one is truthy"
  - name: nonempty string is truthy
    if: "fromJSON('\"hello\"')"
    run: Write-Host "string is truthy"
  - name: integer literal truthy
    if: 42
    run: Write-Host "int is truthy"
`,
	})

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "test.txt"),
		"file_text": "hello",
	}, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
}

// TestBuiltinNumericCoercion exercises toNumber with different types and comparisons.
// Targets: toNumber (evaluator.go:485), parseComparison (evaluator.go:185)
func TestBuiltinNumericCoercion(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"numeric-coerce.yml": `name: Numeric Coercion
on:
  file:
    paths: ["**/*"]
steps:
  - name: float gt int
    if: fromJSON('42.5') > 10
    run: Write-Host "float > int"
  - name: int gte int
    if: fromJSON('100') >= 100
    run: Write-Host "int gte"
  - name: less than
    if: fromJSON('5') < 10
    run: Write-Host "lt works"
  - name: less or equal
    if: fromJSON('5') <= 5
    run: Write-Host "lte works"
  - name: string to number
    if: "fromJSON('\"42\"') > 10"
    run: Write-Host "string number comparison"
  - name: bool to number
    if: true > 0
    run: Write-Host "bool > 0"
`,
	})

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "test.txt"),
		"file_text": "hello",
	}, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
}

// TestBuiltinNullComparison exercises null equality and inequality.
// Targets: equals nil path (evaluator.go:509), parsePrimary null keyword
func TestBuiltinNullComparison(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"null-compare.yml": `name: Null Compare
on:
  file:
    paths: ["**/*"]
steps:
  - name: null equals null
    if: fromJSON('null') == null
    run: Write-Host "null == null"
  - name: null not equals string
    if: "fromJSON('null') != 'something'"
    run: Write-Host "null != string"
`,
	})

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "test.txt"),
		"file_text": "hello",
	}, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
}

// TestBuiltinParenGrouping exercises parenthesized expressions.
// Targets: parsePrimary TokenLeftParen branch (evaluator.go:306)
func TestBuiltinParenGrouping(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"paren-group.yml": `name: Paren Grouping
on:
  file:
    paths: ["**/*"]
steps:
  - name: grouped or and
    if: (true || false) && (false || true)
    run: Write-Host "paren grouping"
  - name: nested parens
    if: (((true)))
    run: Write-Host "nested parens"
`,
	})

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "test.txt"),
		"file_text": "hello",
	}, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
}

// TestBuiltinToStringBranches exercises toString with bool, int64, and float64.
// Targets: toString (evaluator.go:446) — bool, int64, float64 branches
func TestBuiltinToStringBranches(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"tostring-branches.yml": `name: ToString Branches
on:
  file:
    paths: ["**/*"]
steps:
  - name: format bool to string
    if: "format('{0}', true) == 'true'"
    run: Write-Host "bool to string"
  - name: format int to string
    if: "format('{0}', 42) == '42'"
    run: Write-Host "int to string"
  - name: format float to string
    if: "format('{0}', fromJSON('3.14')) == '3.14'"
    run: Write-Host "float to string"
  - name: format false to string
    if: "format('{0}', false) == 'false'"
    run: Write-Host "false to string"
`,
	})

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "test.txt"),
		"file_text": "hello",
	}, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
}

// TestBuiltinJoinNonArrayFallback exercises join() when argument is not an array.
// Targets: builtinJoin non-array branch (evaluator.go:585)
func TestBuiltinJoinNonArrayFallback(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"join-nonarray.yml": `name: Join Non-Array
on:
  file:
    paths: ["**/*"]
steps:
  - name: join on string returns string
    if: "join('hello') == 'hello'"
    run: Write-Host "join non-array works"
`,
	})

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "test.txt"),
		"file_text": "hello",
	}, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
}

// TestBuiltinFromJSONNull exercises fromJSON with explicit null.
// Targets: builtinFromJSON (evaluator.go:615) with null value
func TestBuiltinFromJSONNull(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"fromjson-null.yml": `name: FromJSON Null
on:
  file:
    paths: ["**/*"]
steps:
  - name: fromjson null
    if: fromJSON('null') == null
    run: Write-Host "fromjson null is null"
  - name: null is falsy
    if: "!fromJSON('null')"
    run: Write-Host "null is falsy"
`,
	})

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "test.txt"),
		"file_text": "hello",
	}, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
}

// TestBuiltinContainsNonStringNonArray exercises contains() with non-string, non-array input.
// Targets: builtinContains default branch (evaluator.go:533)
func TestBuiltinContainsNonStringNonArray(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"contains-default.yml": `name: Contains Default
on:
  file:
    paths: ["**/*"]
steps:
  - name: contains on number returns false
    if: "!contains(42, 'test')"
    run: Write-Host "contains on number is false"
`,
	})

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "test.txt"),
		"file_text": "hello",
	}, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
}

// TestExprToJSONWithNumber exercises toJSON with a numeric argument.
// Targets: builtinToJSON non-string branch (evaluator.go:604)
func TestExprToJSONWithNumber(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"tojson-num.yml": `name: ToJSON Number
on:
  file:
    paths: ["**/*"]
steps:
  - name: tojson number
    if: "contains(toJSON(fromJSON('{\"n\":42}')), '42')"
    run: Write-Host "tojson with number"
`,
	})

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "test.txt"),
		"file_text": "hello",
	}, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
}

// TestExprInequality exercises the != operator path.
// Targets: parseEquality != branch (evaluator.go:163)
func TestExprInequality(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"inequality.yml": `name: Inequality
on:
  file:
    paths: ["**/*"]
steps:
  - name: not equal strings
    if: "'hello' != 'world'"
    run: Write-Host "not equal"
  - name: not equal numbers
    if: 42 != 0
    run: Write-Host "numbers not equal"
`,
	})

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "test.txt"),
		"file_text": "hello",
	}, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
}

// TestExprStepOutputInCondition exercises step output reference in later step conditions.
// Targets: parsePrimary steps context, getProperty on step outcomes
func TestExprStepOutputInCondition(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"step-ref.yml": `name: Step Reference
on:
  file:
    paths: ["**/*"]
steps:
  - name: first
    id: check
    run: Write-Host "checking"
  - name: second
    if: steps.check.outcome == 'success'
    run: Write-Host "first succeeded"
`,
	})

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "test.txt"),
		"file_text": "hello",
	}, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
}

// TestExprStringInterpolation exercises ${{ }} expression interpolation in run steps.
// Targets: EvaluateString, ReplaceExpressions (parser.go:87)
func TestExprStringInterpolation(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"interpolation.yml": `name: String Interpolation
on:
  file:
    paths: ["**/*"]
steps:
  - name: interpolate path
    run: |
      $path = "${{ event.file.path }}"
      if ($path -eq "") { exit 1 }
      Write-Host "path is $path"
  - name: interpolate action
    run: |
      $action = "${{ event.file.action }}"
      if ($action -ne "create") { exit 1 }
      Write-Host "action is $action"
`,
	})

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "src", "app.ts"),
		"file_text": "const x = 1;",
	}, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
	if !strings.Contains(output, "path is") {
		t.Errorf("Expected interpolated path in output:\n%s", output)
	}
}
