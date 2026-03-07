package expression

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestContextEvaluate(t *testing.T) {
	ctx := NewContext()
	ctx.Event["cwd"] = "/test/path"
	ctx.Event["file"] = map[string]interface{}{
		"path":   "src/main.go",
		"action": "edit",
	}
	ctx.Env["NODE_ENV"] = "development"

	tests := []struct {
		name    string
		expr    string
		want    interface{}
		wantErr bool
	}{
		{
			name: "literal true",
			expr: "true",
			want: true,
		},
		{
			name: "literal false",
			expr: "false",
			want: false,
		},
		{
			name: "literal null",
			expr: "null",
			want: nil,
		},
		{
			name: "literal number",
			expr: "42",
			want: int64(42),
		},
		{
			name: "literal string",
			expr: "'hello'",
			want: "hello",
		},
		{
			name: "event property",
			expr: "event.cwd",
			want: "/test/path",
		},
		{
			name: "nested property",
			expr: "event.file.path",
			want: "src/main.go",
		},
		{
			name: "env access",
			expr: "env.NODE_ENV",
			want: "development",
		},
		{
			name: "equality true",
			expr: "'a' == 'a'",
			want: true,
		},
		{
			name: "equality false",
			expr: "'a' == 'b'",
			want: false,
		},
		{
			name: "inequality",
			expr: "'a' != 'b'",
			want: true,
		},
		{
			name: "case insensitive equality",
			expr: "'Hello' == 'hello'",
			want: true,
		},
		{
			name: "logical and",
			expr: "true && true",
			want: true,
		},
		{
			name: "logical or",
			expr: "false || true",
			want: true,
		},
		{
			name: "logical not",
			expr: "!false",
			want: true,
		},
		{
			name: "comparison less than",
			expr: "1 < 2",
			want: true,
		},
		{
			name: "comparison greater than",
			expr: "2 > 1",
			want: true,
		},
		{
			name: "parentheses",
			expr: "(1 < 2) && (3 > 2)",
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ctx.Evaluate(tt.expr)
			if (err != nil) != tt.wantErr {
				t.Errorf("Evaluate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Evaluate() = %v (%T), want %v (%T)", got, got, tt.want, tt.want)
			}
		})
	}
}

func TestContextEvaluateString(t *testing.T) {
	ctx := NewContext()
	ctx.Event["file"] = map[string]interface{}{
		"path": "test.js",
	}

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "no expressions",
			input: "just text",
			want:  "just text",
		},
		{
			name:  "single expression",
			input: "path: ${{ event.file.path }}",
			want:  "path: test.js",
		},
		{
			name:  "multiple expressions",
			input: "${{ event.file.path }} is a ${{ 'file' }}",
			want:  "test.js is a file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ctx.EvaluateString(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("EvaluateString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("EvaluateString() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestContextEvaluateBool(t *testing.T) {
	ctx := NewContext()

	tests := []struct {
		name    string
		expr    string
		want    bool
		wantErr bool
	}{
		{"true", "true", true, false},
		{"false", "false", false, false},
		{"comparison", "1 == 1", true, false},
		{"and", "true && false", false, false},
		{"or", "true || false", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ctx.EvaluateBool(tt.expr)
			if (err != nil) != tt.wantErr {
				t.Errorf("EvaluateBool() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("EvaluateBool() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuiltinContains(t *testing.T) {
	tests := []struct {
		name    string
		search  interface{}
		item    string
		want    bool
		wantErr bool
	}{
		{
			name:   "string contains",
			search: "Hello World",
			item:   "World",
			want:   true,
		},
		{
			name:   "string not contains",
			search: "Hello World",
			item:   "Foo",
			want:   false,
		},
		{
			name:   "case insensitive",
			search: "Hello World",
			item:   "world",
			want:   true,
		},
		{
			name:   "array contains",
			search: []interface{}{"a", "b", "c"},
			item:   "b",
			want:   true,
		},
		{
			name:   "array not contains",
			search: []interface{}{"a", "b", "c"},
			item:   "d",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := builtinContains(tt.search, tt.item)
			if (err != nil) != tt.wantErr {
				t.Errorf("builtinContains() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("builtinContains() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuiltinStartsWith(t *testing.T) {
	tests := []struct {
		str    string
		prefix string
		want   bool
	}{
		{"Hello World", "Hello", true},
		{"Hello World", "World", false},
		{"Hello World", "hello", true}, // case insensitive
	}

	for _, tt := range tests {
		t.Run(tt.str+"_"+tt.prefix, func(t *testing.T) {
			got, err := builtinStartsWith(tt.str, tt.prefix)
			if err != nil {
				t.Errorf("builtinStartsWith() error = %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("builtinStartsWith() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuiltinEndsWith(t *testing.T) {
	tests := []struct {
		str    string
		suffix string
		want   bool
	}{
		{"Hello World", "World", true},
		{"Hello World", "Hello", false},
		{"test.js", ".js", true},
	}

	for _, tt := range tests {
		t.Run(tt.str+"_"+tt.suffix, func(t *testing.T) {
			got, err := builtinEndsWith(tt.str, tt.suffix)
			if err != nil {
				t.Errorf("builtinEndsWith() error = %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("builtinEndsWith() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuiltinFormat(t *testing.T) {
	tests := []struct {
		name string
		args []interface{}
		want string
	}{
		{
			name: "simple format",
			args: []interface{}{"Hello {0}", "World"},
			want: "Hello World",
		},
		{
			name: "multiple placeholders",
			args: []interface{}{"{0} {1} {2}", "a", "b", "c"},
			want: "a b c",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := builtinFormat(tt.args...)
			if err != nil {
				t.Errorf("builtinFormat() error = %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("builtinFormat() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuiltinJoin(t *testing.T) {
	tests := []struct {
		name string
		arr  []interface{}
		sep  string
		want string
	}{
		{
			name: "default separator",
			arr:  []interface{}{"a", "b", "c"},
			sep:  "",
			want: "a,b,c",
		},
		{
			name: "custom separator",
			arr:  []interface{}{"a", "b", "c"},
			sep:  " - ",
			want: "a - b - c",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got interface{}
			var err error
			if tt.sep == "" {
				got, err = builtinJoin(tt.arr)
			} else {
				got, err = builtinJoin(tt.arr, tt.sep)
			}
			if err != nil {
				t.Errorf("builtinJoin() error = %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("builtinJoin() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuiltinToJSON(t *testing.T) {
	got, err := builtinToJSON(map[string]interface{}{"key": "value"})
	if err != nil {
		t.Errorf("builtinToJSON() error = %v", err)
		return
	}
	// JSON may have different spacing, just check it's valid
	if got != `{"key":"value"}` {
		t.Errorf("builtinToJSON() = %v", got)
	}
}

func TestBuiltinFromJSON(t *testing.T) {
	got, err := builtinFromJSON(`{"key":"value"}`)
	if err != nil {
		t.Errorf("builtinFromJSON() error = %v", err)
		return
	}
	m, ok := got.(map[string]interface{})
	if !ok {
		t.Errorf("builtinFromJSON() returned %T, want map", got)
		return
	}
	if m["key"] != "value" {
		t.Errorf("builtinFromJSON() key = %v, want 'value'", m["key"])
	}
}

func TestFunctionCallInContext(t *testing.T) {
	ctx := NewContext()
	ctx.Event["file"] = map[string]interface{}{
		"path": "src/utils/helper.js",
	}

	tests := []struct {
		name    string
		expr    string
		want    interface{}
		wantErr bool
	}{
		{
			name: "contains function",
			expr: "contains(event.file.path, 'utils')",
			want: true,
		},
		{
			name: "startsWith function",
			expr: "startsWith(event.file.path, 'src')",
			want: true,
		},
		{
			name: "endsWith function",
			expr: "endsWith(event.file.path, '.js')",
			want: true,
		},
		{
			name: "always function",
			expr: "always()",
			want: true,
		},
		{
			name: "nested function in condition",
			expr: "contains(event.file.path, 'utils') && endsWith(event.file.path, '.js')",
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ctx.Evaluate(tt.expr)
			if (err != nil) != tt.wantErr {
				t.Errorf("Evaluate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Evaluate() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestComparisonOperators tests <, <=, >, >= with various types
func TestComparisonOperators(t *testing.T) {
	ctx := NewContext()

	tests := []struct {
		name string
		expr string
		want interface{}
	}{
		// Integer comparisons
		{"less than int", "5 < 10", true},
		{"less than int false", "10 < 5", false},
		{"less than equal int", "5 <= 5", true},
		{"less than equal int true", "5 <= 10", true},
		{"less than equal int false", "10 <= 5", false},
		{"greater than int", "10 > 5", true},
		{"greater than int false", "5 > 10", false},
		{"greater than equal int", "5 >= 5", true},
		{"greater than equal int true", "10 >= 5", true},
		{"greater than equal int false", "5 >= 10", false},
		// Float comparisons
		{"less than float", "3.14 < 3.15", true},
		{"greater than float", "3.15 > 3.14", true},
		{"less than equal float", "3.14 <= 3.14", true},
		{"greater than equal float", "3.14 >= 3.14", true},
		// Mixed int and float
		{"int less than float", "3 < 3.5", true},
		{"float less than int", "2.5 < 3", true},
		// Comparison with zero
		{"zero less than positive", "0 < 5", true},
		{"positive greater than zero", "5 > 0", true},
		// Chained comparisons with parentheses
		{"chained comparison", "(1 < 2) && (3 > 2)", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ctx.Evaluate(tt.expr)
			if err != nil {
				t.Errorf("Evaluate() error = %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("Evaluate() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestIndexAccess tests array and map index access
func TestIndexAccess(t *testing.T) {
	ctx := NewContext()
	ctx.Event["items"] = []interface{}{"a", "b", "c", "d"}
	ctx.Event["nested"] = map[string]interface{}{
		"list": []interface{}{int64(10), int64(20), int64(30)},
	}
	ctx.Event["map"] = map[string]interface{}{
		"key1": "value1",
		"key2": "value2",
	}

	tests := []struct {
		name    string
		expr    string
		want    interface{}
		wantErr bool
	}{
		// Array index access
		{"array index 0", "event.items[0]", "a", false},
		{"array index 1", "event.items[1]", "b", false},
		{"array index 2", "event.items[2]", "c", false},
		{"array index last", "event.items[3]", "d", false},
		// Out of bounds - returns nil
		{"array out of bounds", "event.items[10]", nil, false},
		// Nested array access
		{"nested array index", "event.nested.list[1]", int64(20), false},
		// Map index access with bracket notation
		{"map bracket access", "event.map['key1']", "value1", false},
		{"map bracket access key2", "event.map['key2']", "value2", false},
		// Non-existent key returns nil
		{"map bracket nonexistent", "event.map['nonexistent']", nil, false},
		// Index on nil returns nil
		{"index on nil", "event.nonexistent[0]", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ctx.Evaluate(tt.expr)
			if (err != nil) != tt.wantErr {
				t.Errorf("Evaluate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Evaluate() = %v (%T), want %v (%T)", got, got, tt.want, tt.want)
			}
		})
	}
}

// TestPropertyAccess tests property access on various types
func TestPropertyAccess(t *testing.T) {
	ctx := NewContext()
	ctx.Event["obj"] = map[string]interface{}{
		"name":  "test",
		"value": int64(42),
	}
	ctx.Env["HOME"] = "/home/user"

	tests := []struct {
		name string
		expr string
		want interface{}
	}{
		{"property on map", "event.obj.name", "test"},
		{"property on env map", "env.HOME", "/home/user"},
		{"property on nil", "event.nonexistent.property", nil},
		{"nested property on nil", "event.nonexistent.deep.nested", nil},
		{"property on map returns nil for missing", "event.obj.missing", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ctx.Evaluate(tt.expr)
			if err != nil {
				t.Errorf("Evaluate() error = %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("Evaluate() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestStepContextFunctions tests success(), failure(), cancelled() with step context
func TestStepContextFunctions(t *testing.T) {
	tests := []struct {
		name     string
		steps    map[string]StepContext
		expr     string
		want     bool
	}{
		// success() tests
		{
			name:  "success with no steps",
			steps: map[string]StepContext{},
			expr:  "success()",
			want:  true,
		},
		{
			name: "success with all success steps",
			steps: map[string]StepContext{
				"step1": {Outcome: "success"},
				"step2": {Outcome: "success"},
			},
			expr: "success()",
			want: true,
		},
		{
			name: "success with failure step",
			steps: map[string]StepContext{
				"step1": {Outcome: "success"},
				"step2": {Outcome: "failure"},
			},
			expr: "success()",
			want: false,
		},
		{
			name: "success with cancelled step",
			steps: map[string]StepContext{
				"step1": {Outcome: "cancelled"},
			},
			expr: "success()",
			want: false,
		},
		// failure() tests
		{
			name:  "failure with no steps",
			steps: map[string]StepContext{},
			expr:  "failure()",
			want:  false,
		},
		{
			name: "failure with all success steps",
			steps: map[string]StepContext{
				"step1": {Outcome: "success"},
			},
			expr: "failure()",
			want: false,
		},
		{
			name: "failure with failure step",
			steps: map[string]StepContext{
				"step1": {Outcome: "success"},
				"step2": {Outcome: "failure"},
			},
			expr: "failure()",
			want: true,
		},
		// cancelled() tests
		{
			name:  "cancelled with no steps",
			steps: map[string]StepContext{},
			expr:  "cancelled()",
			want:  false,
		},
		{
			name: "cancelled with all success steps",
			steps: map[string]StepContext{
				"step1": {Outcome: "success"},
			},
			expr: "cancelled()",
			want: false,
		},
		{
			name: "cancelled with cancelled step",
			steps: map[string]StepContext{
				"step1": {Outcome: "cancelled"},
			},
			expr: "cancelled()",
			want: true,
		},
		// Combined conditions
		{
			name: "success or failure",
			steps: map[string]StepContext{
				"step1": {Outcome: "failure"},
			},
			expr: "success() || failure()",
			want: true,
		},
		{
			name: "always regardless of failure",
			steps: map[string]StepContext{
				"step1": {Outcome: "failure"},
			},
			expr: "always()",
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := NewContext()
			ctx.Steps = tt.steps
			got, err := ctx.Evaluate(tt.expr)
			if err != nil {
				t.Errorf("Evaluate() error = %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("Evaluate() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestStepsPropertyAccess tests accessing step outputs and outcomes
func TestStepsPropertyAccess(t *testing.T) {
	ctx := NewContext()
	ctx.Steps["build"] = StepContext{
		Outputs: map[string]string{"artifact": "build.zip"},
		Outcome: "success",
	}
	ctx.Steps["test"] = StepContext{
		Outputs: map[string]string{"coverage": "85%"},
		Outcome: "failure",
	}

	tests := []struct {
		name string
		expr string
		want interface{}
	}{
		{"step outcome", "steps.build.outcome", "success"},
		{"step output", "steps.build.outputs.artifact", "build.zip"},
		{"step failure outcome", "steps.test.outcome", "failure"},
		{"step output coverage", "steps.test.outputs.coverage", "85%"},
		{"nonexistent step", "steps.nonexistent", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ctx.Evaluate(tt.expr)
			if err != nil {
				t.Errorf("Evaluate() error = %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("Evaluate() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestNestedFunctionCalls tests nested function calls
func TestNestedFunctionCalls(t *testing.T) {
	ctx := NewContext()
	ctx.Event["items"] = []interface{}{"a", "b", "c"}

	tests := []struct {
		name string
		expr string
		want interface{}
	}{
		{
			name: "contains with join from context",
			expr: "contains(join(event.items, ','), 'b')",
			want: true,
		},
		{
			name: "contains with join not found",
			expr: "contains(join(event.items, ','), 'x')",
			want: false,
		},
		{
			name: "startsWith with format",
			expr: "startsWith(format('Hello {0}', 'World'), 'Hello')",
			want: true,
		},
		{
			name: "endsWith with format",
			expr: "endsWith(format('{0}.js', 'test'), '.js')",
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ctx.Evaluate(tt.expr)
			if err != nil {
				t.Errorf("Evaluate() error = %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("Evaluate() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestFromJSONToJSONComplex tests fromJSON/toJSON with complex structures
func TestFromJSONToJSONComplex(t *testing.T) {
	tests := []struct {
		name    string
		input   interface{}
		wantErr bool
	}{
		{
			name: "nested object",
			input: map[string]interface{}{
				"level1": map[string]interface{}{
					"level2": map[string]interface{}{
						"value": "deep",
					},
				},
			},
			wantErr: false,
		},
		{
			name:    "array of objects",
			input:   []interface{}{map[string]interface{}{"a": "1"}, map[string]interface{}{"b": "2"}},
			wantErr: false,
		},
		{
			name:    "mixed types",
			input:   map[string]interface{}{"str": "hello", "num": float64(42), "bool": true, "null": nil},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonStr, err := builtinToJSON(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("builtinToJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}
			// Round-trip test
			result, err := builtinFromJSON(jsonStr)
			if err != nil {
				t.Errorf("builtinFromJSON() error = %v", err)
				return
			}
			// Just verify it parses back
			if result == nil && tt.input != nil {
				t.Errorf("round-trip failed: got nil")
			}
		})
	}
}

// TestFromJSONErrors tests fromJSON with invalid JSON
func TestFromJSONErrors(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid json", `{"key":"value"}`, false},
		{"valid array", `[1,2,3]`, false},
		{"invalid json", `{key:value}`, true},
		{"incomplete json", `{"key":`, true},
		{"empty string", ``, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := builtinFromJSON(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("builtinFromJSON() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestExpressionErrors tests various error conditions
func TestExpressionErrors(t *testing.T) {
	ctx := NewContext()

	tests := []struct {
		name    string
		expr    string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "unknown function",
			expr:    "unknownFunc()",
			wantErr: true,
			errMsg:  "unknown function",
		},
		{
			name:    "contains wrong args",
			expr:    "contains('a')",
			wantErr: true,
			errMsg:  "requires 2 arguments",
		},
		{
			name:    "startsWith wrong args",
			expr:    "startsWith('a')",
			wantErr: true,
			errMsg:  "requires 2 arguments",
		},
		{
			name:    "endsWith wrong args",
			expr:    "endsWith('a')",
			wantErr: true,
			errMsg:  "requires 2 arguments",
		},
		{
			name:    "format no args",
			expr:    "format()",
			wantErr: true,
			errMsg:  "requires at least 1 argument",
		},
		{
			name:    "toJSON wrong args",
			expr:    "toJSON('a', 'b')",
			wantErr: true,
			errMsg:  "requires 1 argument",
		},
		{
			name:    "fromJSON wrong args",
			expr:    "fromJSON('a', 'b')",
			wantErr: true,
			errMsg:  "requires 1 argument",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ctx.Evaluate(tt.expr)
			if (err != nil) != tt.wantErr {
				t.Errorf("Evaluate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error message = %q, want to contain %q", err.Error(), tt.errMsg)
				}
			}
		})
	}
}

// TestBooleanCoercion tests toBool with various values
func TestBooleanCoercion(t *testing.T) {
	ctx := NewContext()

	tests := []struct {
		name string
		expr string
		want bool
	}{
		// Truthy values
		{"true literal", "true", true},
		{"non-empty string", "'hello'", true},
		{"number 1", "1", true},
		{"float 0.1", "0.1", true},
		// Falsy values
		{"false literal", "false", false},
		{"null literal", "null", false},
		{"empty string", "''", false},
		{"number 0", "0", false},
		// Logical operations with coercion
		{"not empty string", "!'hello'", false},
		{"not empty string true", "!''", true},
		{"not zero", "!0", true},
		{"not one", "!1", false},
		{"double not string", "!!''", false},
		{"double not non-empty", "!!'hello'", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ctx.Evaluate(tt.expr)
			if err != nil {
				t.Errorf("Evaluate() error = %v", err)
				return
			}
			got := toBool(result)
			if got != tt.want {
				t.Errorf("toBool(%v) = %v, want %v", result, got, tt.want)
			}
		})
	}
}

// TestNumberParsing tests number parsing edge cases
func TestNumberParsing(t *testing.T) {
	ctx := NewContext()

	tests := []struct {
		name string
		expr string
		want interface{}
	}{
		{"positive int", "42", int64(42)},
		{"float", "3.14", float64(3.14)},
		{"scientific notation", "1e10", float64(1e10)},
		{"scientific with decimal", "2.5e3", float64(2500)},
		{"zero", "0", int64(0)},
		{"large int", "999999", int64(999999)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ctx.Evaluate(tt.expr)
			if err != nil {
				t.Errorf("Evaluate() error = %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("Evaluate() = %v (%T), want %v (%T)", got, got, tt.want, tt.want)
			}
		})
	}
}

// TestBracketNotationPropertyAccess tests bracket notation for property access
func TestBracketNotationPropertyAccess(t *testing.T) {
	ctx := NewContext()
	ctx.Event["data"] = map[string]interface{}{
		"key-with-dash": "value1",
		"normalKey":     "value2",
	}
	ctx.Env["MY_VAR"] = "envValue"

	tests := []struct {
		name string
		expr string
		want interface{}
	}{
		{"bracket access on map", "event.data['normalKey']", "value2"},
		{"bracket access with dash", "event.data['key-with-dash']", "value1"},
		{"bracket access on env", "env['MY_VAR']", "envValue"},
		{"bracket access missing key", "event.data['missing']", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ctx.Evaluate(tt.expr)
			if err != nil {
				t.Errorf("Evaluate() error = %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("Evaluate() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestParenthesizedExpressions tests parenthesized expressions
func TestParenthesizedExpressions(t *testing.T) {
	ctx := NewContext()

	tests := []struct {
		name string
		expr string
		want interface{}
	}{
		{"simple parens", "(true)", true},
		{"parens with negation", "!(false)", true},
		{"parens with comparison", "(1 < 2)", true},
		{"nested parens", "((true))", true},
		{"parens in or", "(false) || (true)", true},
		{"parens in and", "(true) && (true)", true},
		{"complex parens", "((1 < 2) && (3 > 2)) || false", true},
		{"parens change precedence", "(true || false) && true", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ctx.Evaluate(tt.expr)
			if err != nil {
				t.Errorf("Evaluate() error = %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("Evaluate() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestEvaluateBoolWithExpressionSyntax tests EvaluateBool with ${{ }} syntax
func TestEvaluateBoolWithExpressionSyntax(t *testing.T) {
	ctx := NewContext()
	ctx.Event["enabled"] = true

	tests := []struct {
		name    string
		expr    string
		want    bool
		wantErr bool
	}{
		{"direct true", "true", true, false},
		{"direct false", "false", false, false},
		{"expression syntax true", "${{ true }}", true, false},
		{"expression syntax false", "${{ false }}", false, false},
		{"expression syntax comparison", "${{ 1 == 1 }}", true, false},
		{"expression syntax event access", "${{ event.enabled }}", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ctx.EvaluateBool(tt.expr)
			if (err != nil) != tt.wantErr {
				t.Errorf("EvaluateBool() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("EvaluateBool() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestToStringConversions tests toString with various types
func TestToStringConversions(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
		want  string
	}{
		{"nil", nil, ""},
		{"string", "hello", "hello"},
		{"bool true", true, "true"},
		{"bool false", false, "false"},
		{"int64", int64(42), "42"},
		{"int64 negative", int64(-7), "-7"},
		{"float64", float64(3.14), "3.14"},
		{"float64 integer", float64(42), "42"},
		{"slice", []interface{}{"a", "b"}, "[a b]"},
		{"map", map[string]interface{}{"key": "value"}, "map[key:value]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toString(tt.input)
			if got != tt.want {
				t.Errorf("toString() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestToNumberConversions tests toNumber with various types
func TestToNumberConversions(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
		want  float64
	}{
		{"nil", nil, 0},
		{"float64", float64(3.14), 3.14},
		{"int64", int64(42), 42},
		{"string number", "123", 123},
		{"string float", "3.14", 3.14},
		{"string invalid", "abc", 0},
		{"bool true", true, 1},
		{"bool false", false, 0},
		{"slice", []interface{}{"a"}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toNumber(tt.input)
			if got != tt.want {
				t.Errorf("toNumber() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestToBoolConversions tests toBool with various types
func TestToBoolConversions(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
		want  bool
	}{
		{"nil", nil, false},
		{"bool true", true, true},
		{"bool false", false, false},
		{"string non-empty", "hello", true},
		{"string empty", "", false},
		{"int64 non-zero", int64(42), true},
		{"int64 zero", int64(0), false},
		{"float64 non-zero", float64(3.14), true},
		{"float64 zero", float64(0), false},
		{"slice non-empty", []interface{}{"a"}, true},
		{"map non-empty", map[string]interface{}{"key": "value"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toBool(tt.input)
			if got != tt.want {
				t.Errorf("toBool() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestContainsEdgeCases tests contains with edge cases
func TestContainsEdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		search  interface{}
		item    string
		want    bool
		wantErr bool
	}{
		{"non-string non-array", int64(42), "42", false, false},
		{"nil search", nil, "a", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := builtinContains(tt.search, tt.item)
			if (err != nil) != tt.wantErr {
				t.Errorf("builtinContains() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("builtinContains() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestJoinEdgeCases tests join with edge cases
func TestJoinEdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		args    []interface{}
		want    interface{}
		wantErr bool
	}{
		{
			name:    "non-array input",
			args:    []interface{}{"not an array"},
			want:    "not an array",
			wantErr: false,
		},
		{
			name:    "empty array",
			args:    []interface{}{[]interface{}{}},
			want:    "",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := builtinJoin(tt.args...)
			if (err != nil) != tt.wantErr {
				t.Errorf("builtinJoin() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("builtinJoin() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestEvaluateStringErrors tests EvaluateString error handling
func TestEvaluateStringErrors(t *testing.T) {
	ctx := NewContext()

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid expression", "value: ${{ 'test' }}", false},
		{"invalid function", "value: ${{ unknownFunc() }}", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ctx.EvaluateString(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("EvaluateString() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestParseCallErrors tests parseCall error paths
func TestParseCallErrors(t *testing.T) {
	ctx := NewContext()

	tests := []struct {
		name    string
		expr    string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "missing closing paren",
			expr:    "contains('a', 'b'",
			wantErr: true,
		},
		{
			name:    "missing property name after dot",
			expr:    "event.",
			wantErr: true,
			errMsg:  "expected property name",
		},
		{
			name:    "missing closing bracket",
			expr:    "event.items[0",
			wantErr: true,
			errMsg:  "expected ']'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ctx.Evaluate(tt.expr)
			if (err != nil) != tt.wantErr {
				t.Errorf("Evaluate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error = %q, want to contain %q", err.Error(), tt.errMsg)
				}
			}
		})
	}
}

// TestGetIndexOnUnsupportedTypes tests getIndex with unsupported types
func TestGetIndexOnUnsupportedTypes(t *testing.T) {
	ctx := NewContext()
	ctx.Event["number"] = int64(42)
	ctx.Event["str"] = "hello"

	tests := []struct {
		name string
		expr string
		want interface{}
	}{
		{"index on number", "event.number[0]", nil},
		{"index on string", "event.str[0]", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ctx.Evaluate(tt.expr)
			if err != nil {
				t.Errorf("Evaluate() error = %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("Evaluate() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestGetPropertyWithReflection tests property access via reflection on structs
func TestGetPropertyWithReflection(t *testing.T) {
	type TestStruct struct {
		Name  string
		Value int
	}

	ctx := NewContext()
	ctx.Event["struct"] = TestStruct{Name: "test", Value: 42}
	ctx.Event["ptr"] = &TestStruct{Name: "ptrtest", Value: 99}

	tests := []struct {
		name string
		expr string
		want interface{}
	}{
		{"struct field access", "event.struct.Name", "test"},
		{"struct field access int", "event.struct.Value", 42},
		{"ptr struct field access", "event.ptr.Name", "ptrtest"},
		{"ptr struct field access int", "event.ptr.Value", 99},
		{"nonexistent struct field", "event.struct.Missing", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ctx.Evaluate(tt.expr)
			if err != nil {
				t.Errorf("Evaluate() error = %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("Evaluate() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestEvaluateErrors tests Evaluate error handling
func TestEvaluateErrors(t *testing.T) {
	ctx := NewContext()

	tests := []struct {
		name    string
		expr    string
		wantErr bool
	}{
		{"invalid syntax unclosed string", "'unclosed", true},
		{"unexpected character", "@invalid", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ctx.Evaluate(tt.expr)
			if (err != nil) != tt.wantErr {
				t.Errorf("Evaluate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestToJSONError tests toJSON with values that might fail
func TestToJSONError(t *testing.T) {
	// Channel cannot be marshaled to JSON
	ch := make(chan int)
	_, err := builtinToJSON(ch)
	if err == nil {
		t.Error("builtinToJSON(channel) expected error, got nil")
	}
}

// TestJoinErrorCases tests join with invalid argument counts
func TestJoinErrorCases(t *testing.T) {
	// No arguments
	_, err := builtinJoin()
	if err == nil {
		t.Error("builtinJoin() expected error with no arguments")
	}
}

// TestParseOrAndShortCircuit tests or/and short-circuit behavior
func TestParseOrAndShortCircuit(t *testing.T) {
	ctx := NewContext()

	tests := []struct {
		name string
		expr string
		want interface{}
	}{
		{"or short circuit", "true || unknownVar", true},
		{"and short circuit", "false && unknownVar", false},
		{"complex and", "true && true && true", true},
		{"complex or", "false || false || true", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ctx.Evaluate(tt.expr)
			if err != nil {
				t.Errorf("Evaluate() error = %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("Evaluate() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestInequalityOperator tests != operator
func TestInequalityOperator(t *testing.T) {
	ctx := NewContext()

	tests := []struct {
		name string
		expr string
		want interface{}
	}{
		{"string inequality true", "'a' != 'b'", true},
		{"string inequality false", "'a' != 'a'", false},
		{"number inequality true", "1 != 2", true},
		{"number inequality false", "1 != 1", false},
		{"mixed type inequality", "1 != '1'", true}, // different types
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ctx.Evaluate(tt.expr)
			if err != nil {
				t.Errorf("Evaluate() error = %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("Evaluate() = %v, want %v", got, tt.want)
			}
		})
	}
}

// --- Transcript function tests ---

func setupTranscriptTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	content := `{"timestamp":1,"lifecycle":"pre","eventType":"preToolUse","toolName":"edit","toolArgs":{"path":"src/main.go","old_str":"old","new_str":"new"},"seq":1}
{"timestamp":2,"lifecycle":"post","eventType":"postToolUse","toolName":"edit","toolResult":{"resultType":"success"},"seq":2}
{"timestamp":3,"lifecycle":"pre","eventType":"preToolUse","toolName":"powershell","toolArgs":{"command":"go test ./...","description":"Run tests"},"seq":3}
{"timestamp":4,"lifecycle":"post","eventType":"postToolUse","toolName":"powershell","toolResult":{"resultType":"success","textResultForLlm":"All tests passed"},"seq":4}
{"timestamp":5,"lifecycle":"pre","eventType":"preToolUse","toolName":"powershell","toolArgs":{"command":"git commit -m \"fix: resolve bug\"","description":"Commit changes"},"seq":5}
{"timestamp":6,"lifecycle":"pre","eventType":"preToolUse","toolName":"edit","toolArgs":{"path":"src/utils.ts","old_str":"a","new_str":"b"},"seq":6}
{"timestamp":7,"lifecycle":"pre","eventType":"preToolUse","toolName":"powershell","toolArgs":{"command":"npm test","description":"Run npm tests"},"seq":7}
`
	if err := os.WriteFile(filepath.Join(dir, "transcript.jsonl"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test transcript: %v", err)
	}
	return dir
}

func TestTranscript_NoArgs_ReturnsAll(t *testing.T) {
	dir := setupTranscriptTestDir(t)
	ctx := NewContext()
	ctx.SessionDir = dir

	result, err := ctx.Evaluate("transcript()")
	if err != nil {
		t.Fatalf("transcript() error: %v", err)
	}

	str, ok := result.(string)
	if !ok {
		t.Fatalf("expected string result, got %T", result)
	}

	var entries []map[string]interface{}
	if err := json.Unmarshal([]byte(str), &entries); err != nil {
		t.Fatalf("transcript() result is not valid JSON array: %v\nResult: %s", err, str)
	}

	if len(entries) != 7 {
		t.Errorf("expected 7 entries, got %d", len(entries))
	}
}

func TestTranscript_WithRegex_FiltersEntries(t *testing.T) {
	dir := setupTranscriptTestDir(t)
	ctx := NewContext()
	ctx.SessionDir = dir

	result, err := ctx.Evaluate("transcript('go test|npm test')")
	if err != nil {
		t.Fatalf("transcript('go test|npm test') error: %v", err)
	}

	str := result.(string)
	var entries []map[string]interface{}
	if err := json.Unmarshal([]byte(str), &entries); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if len(entries) != 2 {
		t.Errorf("expected 2 test entries, got %d", len(entries))
	}
}

func TestTranscript_RegexMatchesGitCommit(t *testing.T) {
	dir := setupTranscriptTestDir(t)
	ctx := NewContext()
	ctx.SessionDir = dir

	result, err := ctx.Evaluate("transcript('git commit')")
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	str := result.(string)
	var entries []map[string]interface{}
	if err := json.Unmarshal([]byte(str), &entries); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if len(entries) != 1 {
		t.Errorf("expected 1 git commit entry, got %d", len(entries))
	}
}

func TestTranscript_RegexMatchesFilePath(t *testing.T) {
	dir := setupTranscriptTestDir(t)
	ctx := NewContext()
	ctx.SessionDir = dir

	result, err := ctx.Evaluate("transcript('\\.ts')")
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	str := result.(string)
	var entries []map[string]interface{}
	if err := json.Unmarshal([]byte(str), &entries); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if len(entries) != 1 {
		t.Errorf("expected 1 .ts file entry, got %d", len(entries))
	}
}

func TestTranscript_NoSessionDir_ReturnsEmptyArray(t *testing.T) {
	ctx := NewContext()

	result, err := ctx.Evaluate("transcript()")
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	if result != "[]" {
		t.Errorf("expected '[]', got %v", result)
	}
}

func TestTranscript_EmptyTranscript_ReturnsEmptyArray(t *testing.T) {
	dir := t.TempDir()
	ctx := NewContext()
	ctx.SessionDir = dir

	result, err := ctx.Evaluate("transcript()")
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	if result != "[]" {
		t.Errorf("expected '[]', got %v", result)
	}
}

func TestTranscriptSince_ReturnsEntriesAfterMatch(t *testing.T) {
	dir := setupTranscriptTestDir(t)
	ctx := NewContext()
	ctx.SessionDir = dir

	result, err := ctx.Evaluate("transcript_since('git commit')")
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	str := result.(string)
	var entries []map[string]interface{}
	if err := json.Unmarshal([]byte(str), &entries); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// After "git commit" (seq 5), there are 2 entries: edit utils.ts (6) and npm test (7)
	if len(entries) != 2 {
		t.Errorf("expected 2 entries after git commit, got %d", len(entries))
	}
}

func TestTranscriptSince_NoMatch_ReturnsAll(t *testing.T) {
	dir := setupTranscriptTestDir(t)
	ctx := NewContext()
	ctx.SessionDir = dir

	result, err := ctx.Evaluate("transcript_since('nonexistent')")
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	str := result.(string)
	var entries []map[string]interface{}
	if err := json.Unmarshal([]byte(str), &entries); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if len(entries) != 7 {
		t.Errorf("expected all 7 entries when no match, got %d", len(entries))
	}
}

func TestTranscriptSince_RequiresOneArg(t *testing.T) {
	ctx := NewContext()
	_, err := ctx.Evaluate("transcript_since()")
	if err == nil {
		t.Error("expected error for transcript_since() with no args")
	}
}

func TestTranscriptCount(t *testing.T) {
	dir := setupTranscriptTestDir(t)
	ctx := NewContext()
	ctx.SessionDir = dir

	result, err := ctx.Evaluate("transcript_count('go test|npm test')")
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	if result != float64(2) {
		t.Errorf("expected count=2, got %v", result)
	}
}

func TestTranscriptCount_NoMatches(t *testing.T) {
	dir := setupTranscriptTestDir(t)
	ctx := NewContext()
	ctx.SessionDir = dir

	result, err := ctx.Evaluate("transcript_count('docker build')")
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	if result != float64(0) {
		t.Errorf("expected count=0, got %v", result)
	}
}

func TestTranscriptCount_RequiresOneArg(t *testing.T) {
	ctx := NewContext()
	_, err := ctx.Evaluate("transcript_count()")
	if err == nil {
		t.Error("expected error for transcript_count() with no args")
	}
}

func TestTranscriptLast(t *testing.T) {
	dir := setupTranscriptTestDir(t)
	ctx := NewContext()
	ctx.SessionDir = dir

	result, err := ctx.Evaluate("transcript_last('\"toolName\":\"edit\"')")
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	str, ok := result.(string)
	if !ok {
		t.Fatalf("expected string, got %T", result)
	}

	if !strings.Contains(str, "utils.ts") {
		t.Errorf("expected last edit to be utils.ts, got: %s", str)
	}
}

func TestTranscriptLast_NoMatch_ReturnsEmpty(t *testing.T) {
	dir := setupTranscriptTestDir(t)
	ctx := NewContext()
	ctx.SessionDir = dir

	result, err := ctx.Evaluate("transcript_last('docker push')")
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	if result != "" {
		t.Errorf("expected empty string, got %v", result)
	}
}

func TestTranscriptLast_RequiresOneArg(t *testing.T) {
	ctx := NewContext()
	_, err := ctx.Evaluate("transcript_last()")
	if err == nil {
		t.Error("expected error for transcript_last() with no args")
	}
}

func TestTranscript_InvalidRegex(t *testing.T) {
	dir := setupTranscriptTestDir(t)
	ctx := NewContext()
	ctx.SessionDir = dir

	_, err := ctx.Evaluate("transcript('[invalid')")
	if err == nil {
		t.Error("expected error for invalid regex in transcript()")
	}
}

func TestTranscriptCount_InCondition(t *testing.T) {
	dir := setupTranscriptTestDir(t)
	ctx := NewContext()
	ctx.SessionDir = dir

	result, err := ctx.Evaluate("transcript_count('go test|npm test') > 0")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if result != true {
		t.Errorf("expected true (tests were run), got %v", result)
	}

	result, err = ctx.Evaluate("transcript_count('docker build') > 0")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if result != false {
		t.Errorf("expected false (no docker builds), got %v", result)
	}
}

func TestTranscript_UsedInEvaluateString(t *testing.T) {
	dir := setupTranscriptTestDir(t)
	ctx := NewContext()
	ctx.SessionDir = dir

	result, err := ctx.EvaluateString("Test count: ${{ transcript_count('go test|npm test') }}")
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	if result != "Test count: 2" {
		t.Errorf("expected 'Test count: 2', got '%s'", result)
	}
}
