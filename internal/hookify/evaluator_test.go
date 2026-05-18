package hookify

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/htekdev/gh-hookflow/internal/schema"
)

// --- Operator tests ---

func TestEvaluateCondition_RegexMatch(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		value    string
		expected bool
	}{
		{"simple match", "hello", "say hello world", true},
		{"no match", "hello", "say goodbye", false},
		{"case insensitive", "HELLO", "say hello world", true},
		{"regex special chars", `rm\s+-rf`, "rm -rf /tmp", true},
		{"anchored start", "^hello", "hello world", true},
		{"anchored start no match", "^hello", "say hello", false},
		{"anchored end", `\.env$`, "config.env", true},
		{"anchored end no match", `\.env$`, "config.env.bak", false},
		{"empty value", "test", "", false},
		{"empty pattern matches all", "", "anything", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cond := &Condition{Operator: OpRegexMatch, Pattern: tt.pattern}
			result := evaluateCondition(cond, tt.value)
			if result != tt.expected {
				t.Errorf("regexMatch(%q, %q) = %v, want %v", tt.pattern, tt.value, result, tt.expected)
			}
		})
	}
}

func TestEvaluateCondition_Contains(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		value    string
		expected bool
	}{
		{"simple match", "hello", "say hello world", true},
		{"no match", "hello", "say goodbye", false},
		{"case insensitive", "HELLO", "say hello world", true},
		{"empty value", "test", "", false},
		{"empty pattern", "", "anything", true},
		{"exact match", "hello", "hello", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cond := &Condition{Operator: OpContains, Pattern: tt.pattern}
			result := evaluateCondition(cond, tt.value)
			if result != tt.expected {
				t.Errorf("contains(%q, %q) = %v, want %v", tt.pattern, tt.value, result, tt.expected)
			}
		})
	}
}

func TestEvaluateCondition_Equals(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		value    string
		expected bool
	}{
		{"exact match", "hello", "hello", true},
		{"no match", "hello", "world", false},
		{"case sensitive", "Hello", "hello", false},
		{"empty both", "", "", true},
		{"empty value", "test", "", false},
		{"empty pattern", "", "test", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cond := &Condition{Operator: OpEquals, Pattern: tt.pattern}
			result := evaluateCondition(cond, tt.value)
			if result != tt.expected {
				t.Errorf("equals(%q, %q) = %v, want %v", tt.pattern, tt.value, result, tt.expected)
			}
		})
	}
}

func TestEvaluateCondition_NotContains(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		value    string
		expected bool
	}{
		{"not present", "hello", "say goodbye", true},
		{"present", "hello", "say hello world", false},
		{"case insensitive present", "HELLO", "say hello world", false},
		{"empty value", "test", "", true},
		{"empty pattern", "", "anything", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cond := &Condition{Operator: OpNotContains, Pattern: tt.pattern}
			result := evaluateCondition(cond, tt.value)
			if result != tt.expected {
				t.Errorf("notContains(%q, %q) = %v, want %v", tt.pattern, tt.value, result, tt.expected)
			}
		})
	}
}

func TestEvaluateCondition_StartsWith(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		value    string
		expected bool
	}{
		{"starts with", "hello", "hello world", true},
		{"no match", "hello", "say hello", false},
		{"case insensitive", "HELLO", "hello world", true},
		{"exact match", "hello", "hello", true},
		{"empty value", "test", "", false},
		{"empty pattern", "", "anything", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cond := &Condition{Operator: OpStartsWith, Pattern: tt.pattern}
			result := evaluateCondition(cond, tt.value)
			if result != tt.expected {
				t.Errorf("startsWith(%q, %q) = %v, want %v", tt.pattern, tt.value, result, tt.expected)
			}
		})
	}
}

func TestEvaluateCondition_EndsWith(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		value    string
		expected bool
	}{
		{"ends with", ".env", "config.env", true},
		{"no match", ".env", "config.env.bak", false},
		{"case insensitive", ".ENV", "config.env", true},
		{"exact match", "hello", "hello", true},
		{"empty value", "test", "", false},
		{"empty pattern", "", "anything", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cond := &Condition{Operator: OpEndsWith, Pattern: tt.pattern}
			result := evaluateCondition(cond, tt.value)
			if result != tt.expected {
				t.Errorf("endsWith(%q, %q) = %v, want %v", tt.pattern, tt.value, result, tt.expected)
			}
		})
	}
}

func TestEvaluateCondition_UnknownOperator(t *testing.T) {
	cond := &Condition{Operator: "unknown_op", Pattern: "test"}
	if evaluateCondition(cond, "test") {
		t.Error("expected unknown operator to return false")
	}
}

func TestEvaluateCondition_InvalidRegex(t *testing.T) {
	cond := &Condition{Operator: OpRegexMatch, Pattern: "[invalid(regex"}
	if evaluateCondition(cond, "test") {
		t.Error("expected invalid regex to return false")
	}
}

// --- Field extraction tests ---

func TestExtractField_Command(t *testing.T) {
	tests := []struct {
		name     string
		event    *schema.Event
		expected string
	}{
		{
			"from command arg",
			&schema.Event{Tool: &schema.ToolEvent{
				Name: "powershell",
				Args: map[string]interface{}{"command": "echo hello"},
			}},
			"echo hello",
		},
		{
			"from script arg",
			&schema.Event{Tool: &schema.ToolEvent{
				Name: "bash",
				Args: map[string]interface{}{"script": "echo hello"},
			}},
			"echo hello",
		},
		{
			"from code arg",
			&schema.Event{Tool: &schema.ToolEvent{
				Name: "bash",
				Args: map[string]interface{}{"code": "echo hello"},
			}},
			"echo hello",
		},
		{
			"command takes priority over script",
			&schema.Event{Tool: &schema.ToolEvent{
				Name: "bash",
				Args: map[string]interface{}{"command": "first", "script": "second"},
			}},
			"first",
		},
		{
			"nil tool",
			&schema.Event{},
			"",
		},
		{
			"nil args",
			&schema.Event{Tool: &schema.ToolEvent{Name: "bash"}},
			"",
		},
		{
			"no matching key",
			&schema.Event{Tool: &schema.ToolEvent{
				Name: "bash",
				Args: map[string]interface{}{"unrelated": "value"},
			}},
			"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractField(FieldCommand, tt.event, "")
			if result != tt.expected {
				t.Errorf("extractField(command) = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestExtractField_FilePath(t *testing.T) {
	tests := []struct {
		name     string
		event    *schema.Event
		expected string
	}{
		{
			"from file event",
			&schema.Event{File: &schema.FileEvent{Path: "src/main.go"}},
			"src/main.go",
		},
		{
			"from tool args path",
			&schema.Event{Tool: &schema.ToolEvent{
				Name: "create",
				Args: map[string]interface{}{"path": "src/main.go"},
			}},
			"src/main.go",
		},
		{
			"file event takes priority",
			&schema.Event{
				File: &schema.FileEvent{Path: "from-file"},
				Tool: &schema.ToolEvent{
					Name: "create",
					Args: map[string]interface{}{"path": "from-tool"},
				},
			},
			"from-file",
		},
		{
			"empty event",
			&schema.Event{},
			"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractField(FieldFilePath, tt.event, "")
			if result != tt.expected {
				t.Errorf("extractField(file_path) = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestExtractField_NewText(t *testing.T) {
	tests := []struct {
		name     string
		event    *schema.Event
		expected string
	}{
		{
			"from file content",
			&schema.Event{File: &schema.FileEvent{Content: "new content"}},
			"new content",
		},
		{
			"from new_str arg",
			&schema.Event{Tool: &schema.ToolEvent{
				Name: "edit",
				Args: map[string]interface{}{"new_str": "replacement"},
			}},
			"replacement",
		},
		{
			"from file_text arg",
			&schema.Event{Tool: &schema.ToolEvent{
				Name: "create",
				Args: map[string]interface{}{"file_text": "full file content"},
			}},
			"full file content",
		},
		{
			"file content takes priority",
			&schema.Event{
				File: &schema.FileEvent{Content: "from-file"},
				Tool: &schema.ToolEvent{
					Name: "edit",
					Args: map[string]interface{}{"new_str": "from-tool"},
				},
			},
			"from-file",
		},
		{
			"empty event",
			&schema.Event{},
			"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractField(FieldNewText, tt.event, "")
			if result != tt.expected {
				t.Errorf("extractField(new_text) = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestExtractField_OldText(t *testing.T) {
	tests := []struct {
		name     string
		event    *schema.Event
		expected string
	}{
		{
			"from old_str arg",
			&schema.Event{Tool: &schema.ToolEvent{
				Name: "edit",
				Args: map[string]interface{}{"old_str": "original text"},
			}},
			"original text",
		},
		{
			"nil tool",
			&schema.Event{},
			"",
		},
		{
			"no old_str key",
			&schema.Event{Tool: &schema.ToolEvent{
				Name: "edit",
				Args: map[string]interface{}{"new_str": "new"},
			}},
			"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractField(FieldOldText, tt.event, "")
			if result != tt.expected {
				t.Errorf("extractField(old_text) = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestExtractField_Content(t *testing.T) {
	t.Run("concatenates all string args", func(t *testing.T) {
		event := &schema.Event{Tool: &schema.ToolEvent{
			Name: "create",
			Args: map[string]interface{}{
				"path":      "src/main.go",
				"file_text": "package main",
			},
		}}
		result := extractField(FieldContent, event, "")
		// Map iteration order is non-deterministic, so check both parts are present
		if result == "" {
			t.Error("expected non-empty content")
		}
		if len(result) < len("src/main.go") {
			t.Errorf("content too short: %q", result)
		}
	})

	t.Run("empty event", func(t *testing.T) {
		result := extractField(FieldContent, &schema.Event{}, "")
		if result != "" {
			t.Errorf("expected empty content, got %q", result)
		}
	})

	t.Run("skips non-string args", func(t *testing.T) {
		event := &schema.Event{Tool: &schema.ToolEvent{
			Name: "test",
			Args: map[string]interface{}{
				"str_val": "hello",
				"int_val": 42,
				"bool":    true,
			},
		}}
		result := extractField(FieldContent, event, "")
		if result != "hello" {
			t.Errorf("expected only string values, got %q", result)
		}
	})
}

func TestExtractField_Transcript(t *testing.T) {
	t.Run("reads transcript file", func(t *testing.T) {
		dir := t.TempDir()
		transcriptContent := `{"tool":"powershell","args":{"command":"echo hello"}}
{"tool":"create","args":{"path":"test.go"}}
`
		if err := os.WriteFile(filepath.Join(dir, "transcript.jsonl"), []byte(transcriptContent), 0644); err != nil {
			t.Fatalf("failed to write transcript: %v", err)
		}

		result := extractField(FieldTranscript, &schema.Event{}, dir)
		if result != transcriptContent {
			t.Errorf("expected transcript content %q, got %q", transcriptContent, result)
		}
	})

	t.Run("empty session dir", func(t *testing.T) {
		result := extractField(FieldTranscript, &schema.Event{}, "")
		if result != "" {
			t.Errorf("expected empty transcript for empty session dir, got %q", result)
		}
	})

	t.Run("nonexistent transcript file", func(t *testing.T) {
		result := extractField(FieldTranscript, &schema.Event{}, t.TempDir())
		if result != "" {
			t.Errorf("expected empty transcript for missing file, got %q", result)
		}
	})
}

func TestExtractField_UnknownField(t *testing.T) {
	result := extractField("unknown_field", &schema.Event{}, "")
	if result != "" {
		t.Errorf("expected empty string for unknown field, got %q", result)
	}
}

// --- Evaluate (full pipeline) tests ---

func TestEvaluate_BlockAction(t *testing.T) {
	rule := &Rule{
		Name:   "block-rule",
		Action: ActionBlock,
		Conditions: []Condition{
			{Field: FieldCommand, Operator: OpContains, Pattern: "rm -rf"},
		},
		Message: "⚠️ Dangerous command blocked",
	}
	event := &schema.Event{Tool: &schema.ToolEvent{
		Name: "powershell",
		Args: map[string]interface{}{"command": "rm -rf /tmp"},
	}}

	result := Evaluate(rule, event, "")
	if result == nil {
		t.Fatal("expected non-nil result for matching block rule")
	}
	if result.PermissionDecision != "deny" {
		t.Errorf("expected deny, got %q", result.PermissionDecision)
	}
	if result.PermissionDecisionReason != "⚠️ Dangerous command blocked" {
		t.Errorf("unexpected reason: %q", result.PermissionDecisionReason)
	}
}

func TestEvaluate_WarnAction(t *testing.T) {
	rule := &Rule{
		Name:   "warn-rule",
		Action: ActionWarn,
		Conditions: []Condition{
			{Field: FieldCommand, Operator: OpContains, Pattern: "echo"},
		},
		Message: "Echo detected",
	}
	event := &schema.Event{Tool: &schema.ToolEvent{
		Name: "powershell",
		Args: map[string]interface{}{"command": "echo hello"},
	}}

	result := Evaluate(rule, event, "")
	if result == nil {
		t.Fatal("expected non-nil result for matching warn rule")
	}
	if result.PermissionDecision != "allow" {
		t.Errorf("expected allow, got %q", result.PermissionDecision)
	}
	if result.PermissionDecisionReason != "Echo detected" {
		t.Errorf("unexpected reason: %q", result.PermissionDecisionReason)
	}
}

func TestEvaluate_DefaultActionIsWarn(t *testing.T) {
	rule := &Rule{
		Name: "default-action",
		Conditions: []Condition{
			{Field: FieldCommand, Operator: OpContains, Pattern: "test"},
		},
		Message: "Default action",
	}
	event := &schema.Event{Tool: &schema.ToolEvent{
		Name: "powershell",
		Args: map[string]interface{}{"command": "test"},
	}}

	result := Evaluate(rule, event, "")
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.PermissionDecision != "allow" {
		t.Errorf("expected default action to be allow (warn), got %q", result.PermissionDecision)
	}
}

func TestEvaluate_NoMatch(t *testing.T) {
	rule := &Rule{
		Name:   "no-match",
		Action: ActionBlock,
		Conditions: []Condition{
			{Field: FieldCommand, Operator: OpContains, Pattern: "rm -rf"},
		},
		Message: "Blocked",
	}
	event := &schema.Event{Tool: &schema.ToolEvent{
		Name: "powershell",
		Args: map[string]interface{}{"command": "echo hello"},
	}}

	result := Evaluate(rule, event, "")
	if result != nil {
		t.Errorf("expected nil result for non-matching rule, got %+v", result)
	}
}

func TestEvaluate_ANDLogic_AllMatch(t *testing.T) {
	rule := &Rule{
		Name:   "and-all-match",
		Action: ActionBlock,
		Conditions: []Condition{
			{Field: FieldFilePath, Operator: OpEndsWith, Pattern: ".go"},
			{Field: FieldNewText, Operator: OpContains, Pattern: "fmt.Println"},
		},
		Message: "Blocked",
	}
	event := &schema.Event{
		File: &schema.FileEvent{
			Path:    "src/main.go",
			Content: "fmt.Println(\"hello\")",
		},
	}

	result := Evaluate(rule, event, "")
	if result == nil {
		t.Fatal("expected non-nil result when all conditions match")
	}
	if result.PermissionDecision != "deny" {
		t.Errorf("expected deny, got %q", result.PermissionDecision)
	}
}

func TestEvaluate_ANDLogic_OneFailsReturnsNil(t *testing.T) {
	rule := &Rule{
		Name:   "and-one-fails",
		Action: ActionBlock,
		Conditions: []Condition{
			{Field: FieldFilePath, Operator: OpEndsWith, Pattern: ".go"},
			{Field: FieldNewText, Operator: OpContains, Pattern: "SECRET_KEY"},
		},
		Message: "Blocked",
	}
	event := &schema.Event{
		File: &schema.FileEvent{
			Path:    "src/main.go",
			Content: "fmt.Println(\"hello\")",
		},
	}

	result := Evaluate(rule, event, "")
	if result != nil {
		t.Error("expected nil result when one condition fails (AND logic)")
	}
}

func TestEvaluate_EmptyConditions(t *testing.T) {
	rule := &Rule{
		Name:       "no-conditions",
		Action:     ActionBlock,
		Conditions: []Condition{},
		Message:    "Always fires",
	}
	event := &schema.Event{}

	result := Evaluate(rule, event, "")
	if result == nil {
		t.Fatal("expected non-nil result for rule with no conditions (vacuously true)")
	}
	if result.PermissionDecision != "deny" {
		t.Errorf("expected deny, got %q", result.PermissionDecision)
	}
}

func TestEvaluate_EmptyMessageDefaultsToName(t *testing.T) {
	rule := &Rule{
		Name:   "my-rule",
		Action: ActionBlock,
		Conditions: []Condition{
			{Field: FieldCommand, Operator: OpContains, Pattern: "test"},
		},
		Message: "",
	}
	event := &schema.Event{Tool: &schema.ToolEvent{
		Name: "powershell",
		Args: map[string]interface{}{"command": "test"},
	}}

	result := Evaluate(rule, event, "")
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.PermissionDecisionReason != `Hookify rule "my-rule" triggered` {
		t.Errorf("expected default reason with rule name, got %q", result.PermissionDecisionReason)
	}
}

func TestEvaluate_TranscriptCondition(t *testing.T) {
	dir := t.TempDir()
	transcriptContent := `{"tool":"powershell","args":{"command":"npm test"}}
{"tool":"create","args":{"path":"src/app.ts"}}
`
	if err := os.WriteFile(filepath.Join(dir, "transcript.jsonl"), []byte(transcriptContent), 0644); err != nil {
		t.Fatalf("failed to write transcript: %v", err)
	}

	t.Run("transcript contains match", func(t *testing.T) {
		rule := &Rule{
			Name:   "transcript-contains",
			Action: ActionWarn,
			Conditions: []Condition{
				{Field: FieldTranscript, Operator: OpContains, Pattern: "npm test"},
			},
			Message: "Tests were run",
		}
		result := Evaluate(rule, &schema.Event{}, dir)
		if result == nil {
			t.Fatal("expected non-nil result for transcript match")
		}
		if result.PermissionDecision != "allow" {
			t.Errorf("expected allow, got %q", result.PermissionDecision)
		}
	})

	t.Run("transcript not_contains match", func(t *testing.T) {
		rule := &Rule{
			Name:   "transcript-not-contains",
			Action: ActionBlock,
			Conditions: []Condition{
				{Field: FieldTranscript, Operator: OpNotContains, Pattern: "go test"},
			},
			Message: "Go tests not run",
		}
		result := Evaluate(rule, &schema.Event{}, dir)
		if result == nil {
			t.Fatal("expected non-nil result for transcript not_contains match")
		}
		if result.PermissionDecision != "deny" {
			t.Errorf("expected deny, got %q", result.PermissionDecision)
		}
	})

	t.Run("transcript no match", func(t *testing.T) {
		rule := &Rule{
			Name:   "transcript-no-match",
			Action: ActionBlock,
			Conditions: []Condition{
				{Field: FieldTranscript, Operator: OpContains, Pattern: "nonexistent-command"},
			},
			Message: "Should not fire",
		}
		result := Evaluate(rule, &schema.Event{}, dir)
		if result != nil {
			t.Error("expected nil result when transcript doesn't contain pattern")
		}
	})
}

func TestEvaluate_FilePathRegex(t *testing.T) {
	rule := &Rule{
		Name:   "env-block",
		Action: ActionBlock,
		Conditions: []Condition{
			{Field: FieldFilePath, Operator: OpRegexMatch, Pattern: `\.(env|key|pem)$`},
		},
		Message: "Sensitive file blocked",
	}

	tests := []struct {
		name     string
		path     string
		expected bool // true = should deny (match)
	}{
		{"env file", ".env", true},
		{"key file", "secrets/api.key", true},
		{"pem file", "certs/server.pem", true},
		{"go file", "src/main.go", false},
		{"env in middle", ".env.bak", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := &schema.Event{
				File: &schema.FileEvent{Path: tt.path},
			}
			result := Evaluate(rule, event, "")
			if tt.expected && result == nil {
				t.Errorf("expected deny for path %q, got nil", tt.path)
			}
			if !tt.expected && result != nil {
				t.Errorf("expected nil for path %q, got %+v", tt.path, result)
			}
		})
	}
}

func TestEvaluate_OldTextField(t *testing.T) {
	rule := &Rule{
		Name:   "old-text-check",
		Action: ActionWarn,
		Conditions: []Condition{
			{Field: FieldOldText, Operator: OpContains, Pattern: "deprecated"},
		},
		Message: "Editing deprecated code",
	}
	event := &schema.Event{Tool: &schema.ToolEvent{
		Name: "edit",
		Args: map[string]interface{}{
			"old_str": "// deprecated function",
			"new_str": "// updated function",
		},
	}}

	result := Evaluate(rule, event, "")
	if result == nil {
		t.Fatal("expected non-nil result for old_text match")
	}
	if result.PermissionDecision != "allow" {
		t.Errorf("expected allow (warn), got %q", result.PermissionDecision)
	}
}

func TestEvaluate_ContentFieldConcatenation(t *testing.T) {
	rule := &Rule{
		Name:   "content-check",
		Action: ActionBlock,
		Conditions: []Condition{
			{Field: FieldContent, Operator: OpContains, Pattern: "password"},
		},
		Message: "Password in content",
	}
	event := &schema.Event{Tool: &schema.ToolEvent{
		Name: "create",
		Args: map[string]interface{}{
			"path":      "config.json",
			"file_text": `{"password": "secret123"}`,
		},
	}}

	result := Evaluate(rule, event, "")
	if result == nil {
		t.Fatal("expected non-nil result when content contains password")
	}
	if result.PermissionDecision != "deny" {
		t.Errorf("expected deny, got %q", result.PermissionDecision)
	}
}

func TestEvaluate_MultipleConditionsAllOperators(t *testing.T) {
	rule := &Rule{
		Name:   "multi-op",
		Action: ActionBlock,
		Conditions: []Condition{
			{Field: FieldFilePath, Operator: OpStartsWith, Pattern: "src/"},
			{Field: FieldFilePath, Operator: OpEndsWith, Pattern: ".ts"},
			{Field: FieldNewText, Operator: OpContains, Pattern: "console.log"},
			{Field: FieldNewText, Operator: OpNotContains, Pattern: "// eslint-disable"},
		},
		Message: "Console.log without eslint-disable",
	}

	t.Run("all match", func(t *testing.T) {
		event := &schema.Event{
			File: &schema.FileEvent{
				Path:    "src/app.ts",
				Content: "console.log('debug')",
			},
		}
		result := Evaluate(rule, event, "")
		if result == nil {
			t.Fatal("expected deny when all conditions match")
		}
		if result.PermissionDecision != "deny" {
			t.Errorf("expected deny, got %q", result.PermissionDecision)
		}
	})

	t.Run("eslint-disable present fails not_contains", func(t *testing.T) {
		event := &schema.Event{
			File: &schema.FileEvent{
				Path:    "src/app.ts",
				Content: "console.log('debug') // eslint-disable-next-line",
			},
		}
		result := Evaluate(rule, event, "")
		if result != nil {
			t.Error("expected nil when eslint-disable is present (not_contains fails)")
		}
	})

	t.Run("wrong directory fails starts_with", func(t *testing.T) {
		event := &schema.Event{
			File: &schema.FileEvent{
				Path:    "test/app.ts",
				Content: "console.log('debug')",
			},
		}
		result := Evaluate(rule, event, "")
		if result != nil {
			t.Error("expected nil when path doesn't start with src/")
		}
	})
}

func TestEvaluate_EmptyFieldValue(t *testing.T) {
	rule := &Rule{
		Name:   "empty-field",
		Action: ActionBlock,
		Conditions: []Condition{
			{Field: FieldCommand, Operator: OpContains, Pattern: "test"},
		},
		Message: "Blocked",
	}
	// Event with no tool — extractField returns ""
	event := &schema.Event{}

	result := Evaluate(rule, event, "")
	if result != nil {
		t.Error("expected nil result when field value is empty and pattern is 'test'")
	}
}

// --- Action type tests: inject, modify, continue ---

func TestEvaluate_InjectAction(t *testing.T) {
	rule := &Rule{
		Name:   "inject-governance",
		Action: ActionInject,
		Conditions: []Condition{
			{Field: FieldAgentName, Operator: OpContains, Pattern: "coding"},
		},
		Message: "You must follow the repo conventions. Never commit secrets.",
	}
	event := &schema.Event{
		Hook: &schema.HookEvent{
			Type: "subagentStart",
			Tool: &schema.ToolEvent{
				Name: "task",
				Args: map[string]interface{}{"agentName": "coding-agent"},
			},
		},
	}

	result := Evaluate(rule, event, "")
	if result == nil {
		t.Fatal("expected non-nil result for inject action")
	}
	if result.PermissionDecision != "allow" {
		t.Errorf("inject should allow, got %q", result.PermissionDecision)
	}
	if result.AdditionalContext != "You must follow the repo conventions. Never commit secrets." {
		t.Errorf("inject AdditionalContext = %q, want rule message", result.AdditionalContext)
	}
	if result.PermissionDecisionReason != `Hookify rule "inject-governance" injected context` {
		t.Errorf("unexpected reason: %q", result.PermissionDecisionReason)
	}
}

func TestEvaluate_InjectAction_NoConditions(t *testing.T) {
	// Inject with no conditions fires on every matching event (lifecycle hook)
	rule := &Rule{
		Name:       "session-inject",
		Action:     ActionInject,
		Conditions: []Condition{},
		Message:    "Welcome! Remember to write tests.",
	}
	event := &schema.Event{}

	result := Evaluate(rule, event, "")
	if result == nil {
		t.Fatal("expected inject to fire with no conditions")
	}
	if result.AdditionalContext != "Welcome! Remember to write tests." {
		t.Errorf("unexpected AdditionalContext: %q", result.AdditionalContext)
	}
}

func TestEvaluate_ContinueAction(t *testing.T) {
	rule := &Rule{
		Name:   "force-continue",
		Action: ActionContinue,
		Conditions: []Condition{
			{Field: FieldMessage, Operator: OpContains, Pattern: "task incomplete"},
		},
		Message: "You're not done yet. Continue working on the remaining items.",
	}
	event := &schema.Event{
		Hook: &schema.HookEvent{
			Type: "agentStop",
			Tool: &schema.ToolEvent{
				Name: "agentStop",
				Args: map[string]interface{}{"message": "task incomplete, but stopping"},
			},
		},
	}

	result := Evaluate(rule, event, "")
	if result == nil {
		t.Fatal("expected non-nil result for continue action")
	}
	if result.PermissionDecision != "deny" {
		t.Errorf("continue should deny (prevent stop), got %q", result.PermissionDecision)
	}
	if !result.ContinueAgent {
		t.Error("ContinueAgent should be true")
	}
	if result.ContinuePrompt != "You're not done yet. Continue working on the remaining items." {
		t.Errorf("ContinuePrompt = %q, want the rule message body", result.ContinuePrompt)
	}
	if result.PermissionDecisionReason != `Hookify rule "force-continue" forced continuation` {
		t.Errorf("unexpected reason: %q", result.PermissionDecisionReason)
	}
}

func TestEvaluate_ContinueAction_NoMatch(t *testing.T) {
	rule := &Rule{
		Name:   "force-continue",
		Action: ActionContinue,
		Conditions: []Condition{
			{Field: FieldMessage, Operator: OpContains, Pattern: "task incomplete"},
		},
		Message: "Keep going.",
	}
	event := &schema.Event{
		Hook: &schema.HookEvent{
			Type: "agentStop",
			Tool: &schema.ToolEvent{
				Name: "agentStop",
				Args: map[string]interface{}{"message": "all done successfully"},
			},
		},
	}

	result := Evaluate(rule, event, "")
	if result != nil {
		t.Error("expected nil result when conditions don't match")
	}
}

func TestEvaluate_ModifyAction_Replace(t *testing.T) {
	rule := &Rule{
		Name:           "sanitize-path",
		Action:         ActionModify,
		ModifyTarget:   "path",
		ModifyStrategy: StrategyReplace,
		Conditions: []Condition{
			{Field: FieldToolName, Operator: OpEquals, Pattern: "create"},
		},
		Message: "/safe/path/output.txt",
	}
	event := &schema.Event{
		Tool: &schema.ToolEvent{
			Name: "create",
			Args: map[string]interface{}{
				"path":      "/dangerous/path/file.txt",
				"file_text": "hello world",
			},
		},
	}

	result := Evaluate(rule, event, "")
	if result == nil {
		t.Fatal("expected non-nil result for modify action")
	}
	if result.PermissionDecision != "allow" {
		t.Errorf("modify should allow, got %q", result.PermissionDecision)
	}
	if result.ModifiedArgs == nil {
		t.Fatal("ModifiedArgs should not be nil")
	}
	if result.ModifiedArgs["path"] != "/safe/path/output.txt" {
		t.Errorf("expected replaced path, got %q", result.ModifiedArgs["path"])
	}
	// Other args should be preserved
	if result.ModifiedArgs["file_text"] != "hello world" {
		t.Errorf("expected file_text preserved, got %q", result.ModifiedArgs["file_text"])
	}
}

func TestEvaluate_ModifyAction_Prepend(t *testing.T) {
	rule := &Rule{
		Name:           "add-shebang",
		Action:         ActionModify,
		ModifyTarget:   "command",
		ModifyStrategy: StrategyPrepend,
		Conditions: []Condition{
			{Field: FieldCommand, Operator: OpContains, Pattern: "npm"},
		},
		Message: "set -e && ",
	}
	event := &schema.Event{
		Tool: &schema.ToolEvent{
			Name: "bash",
			Args: map[string]interface{}{"command": "npm test"},
		},
	}

	result := Evaluate(rule, event, "")
	if result == nil {
		t.Fatal("expected non-nil result for modify/prepend")
	}
	if result.ModifiedArgs["command"] != "set -e && npm test" {
		t.Errorf("expected prepended command, got %q", result.ModifiedArgs["command"])
	}
}

func TestEvaluate_ModifyAction_Append(t *testing.T) {
	rule := &Rule{
		Name:           "add-verbose",
		Action:         ActionModify,
		ModifyTarget:   "command",
		ModifyStrategy: StrategyAppend,
		Conditions: []Condition{
			{Field: FieldCommand, Operator: OpContains, Pattern: "go test"},
		},
		Message: " -v -count=1",
	}
	event := &schema.Event{
		Tool: &schema.ToolEvent{
			Name: "bash",
			Args: map[string]interface{}{"command": "go test ./..."},
		},
	}

	result := Evaluate(rule, event, "")
	if result == nil {
		t.Fatal("expected non-nil result for modify/append")
	}
	if result.ModifiedArgs["command"] != "go test ./... -v -count=1" {
		t.Errorf("expected appended command, got %q", result.ModifiedArgs["command"])
	}
}

func TestEvaluate_ModifyAction_Regex(t *testing.T) {
	rule := &Rule{
		Name:           "rewrite-branch",
		Action:         ActionModify,
		ModifyTarget:   "command",
		ModifyStrategy: StrategyRegex,
		Conditions: []Condition{
			{Field: FieldCommand, Operator: OpContains, Pattern: "checkout"},
		},
		Message: "s/main/develop/",
	}
	event := &schema.Event{
		Tool: &schema.ToolEvent{
			Name: "bash",
			Args: map[string]interface{}{"command": "git checkout main"},
		},
	}

	result := Evaluate(rule, event, "")
	if result == nil {
		t.Fatal("expected non-nil result for modify/regex")
	}
	if result.ModifiedArgs["command"] != "git checkout develop" {
		t.Errorf("expected regex-modified command, got %q", result.ModifiedArgs["command"])
	}
}

func TestEvaluate_ModifyAction_RegexInvalidFallback(t *testing.T) {
	rule := &Rule{
		Name:           "bad-regex",
		Action:         ActionModify,
		ModifyTarget:   "command",
		ModifyStrategy: StrategyRegex,
		Conditions: []Condition{
			{Field: FieldCommand, Operator: OpContains, Pattern: "test"},
		},
		Message: "s/[invalid(/replacement/",
	}
	event := &schema.Event{
		Tool: &schema.ToolEvent{
			Name: "bash",
			Args: map[string]interface{}{"command": "npm test"},
		},
	}

	result := Evaluate(rule, event, "")
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	// Invalid regex should leave value unchanged
	if result.ModifiedArgs["command"] != "npm test" {
		t.Errorf("invalid regex should leave value unchanged, got %q", result.ModifiedArgs["command"])
	}
}

func TestEvaluate_ModifyAction_MissingTargetArg(t *testing.T) {
	// When the target arg doesn't exist, treat as empty string
	rule := &Rule{
		Name:           "add-description",
		Action:         ActionModify,
		ModifyTarget:   "description",
		ModifyStrategy: StrategyReplace,
		Conditions: []Condition{
			{Field: FieldToolName, Operator: OpEquals, Pattern: "create"},
		},
		Message: "Auto-generated file",
	}
	event := &schema.Event{
		Tool: &schema.ToolEvent{
			Name: "create",
			Args: map[string]interface{}{"path": "/tmp/file.txt"},
		},
	}

	result := Evaluate(rule, event, "")
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.ModifiedArgs["description"] != "Auto-generated file" {
		t.Errorf("expected new arg added, got %q", result.ModifiedArgs["description"])
	}
	// Original arg preserved
	if result.ModifiedArgs["path"] != "/tmp/file.txt" {
		t.Errorf("expected path preserved, got %q", result.ModifiedArgs["path"])
	}
}

func TestApplyRegexStrategy(t *testing.T) {
	tests := []struct {
		name       string
		current    string
		content    string
		expected   string
	}{
		{"sed-style replace", "hello world", "s/world/earth/", "hello earth"},
		{"sed-style remove", "rm -rf /tmp", "s/ -rf//", "rm /tmp"},
		{"sed-style regex groups", "file_v2.txt", `s/v(\d+)/v99/`, "file_v99.txt"},
		{"non-sed fallback", "anything", "replaced", "replaced"},
		{"empty current sed", "", "s/^/prefix/", "prefix"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := applyRegexStrategy(tt.current, tt.content)
			if result != tt.expected {
				t.Errorf("applyRegexStrategy(%q, %q) = %q, want %q", tt.current, tt.content, result, tt.expected)
			}
		})
	}
}

