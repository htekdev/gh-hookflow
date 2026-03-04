package event

import (
	"encoding/json"
	"testing"

	"github.com/htekdev/gh-hookflow/internal/schema"
)

// TestIsGitCommitCommand tests git commit detection patterns
func TestIsGitCommitCommand(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    bool
	}{
		// Should match
		{"simple commit", "git commit -m 'message'", true},
		{"commit with quotes", `git commit -m "message"`, true},
		{"commit without message", "git commit", true},
		{"commit amend", "git commit --amend", true},
		{"commit all", "git commit -am 'message'", true},
		{"commit with path flag", "git -C /path commit -m 'msg'", true},
		{"commit with no-pager", "git --no-pager commit -m 'msg'", true},
		{"commit in chain", "git add . && git commit -m 'msg'", true},
		{"commit after or", "git status || git commit", true},
		{"commit after semicolon", "echo done; git commit -m 'msg'", true},
		{"commit ci alias", "git ci -m 'msg'", false}, // ci is not always an alias
		{"chained with add", "git add -A && git commit -m 'test'", true},
		{"triple chain", "npm test && git add . && git commit -m 'test'", true},

		// Should NOT match
		{"echo git commit", `echo "git commit"`, false}, // False positive avoided - git commit is inside quotes
		{"just git", "git status", false},
		{"git push", "git push origin main", false},
		{"git add only", "git add .", false},
		{"empty", "", false},
		{"unrelated command", "npm install", false},
		{"comment", "# git commit", false},
		{"commit in push message", `git push -m "commit changes"`, false}, // "commit" is not a git subcommand here
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsGitCommitCommand(tt.command)
			if got != tt.want {
				t.Errorf("IsGitCommitCommand(%q) = %v, want %v", tt.command, got, tt.want)
			}
		})
	}
}

// TestIsGitPushCommand tests git push detection patterns
func TestIsGitPushCommand(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    bool
	}{
		// Should match
		{"simple push", "git push", true},
		{"push origin", "git push origin", true},
		{"push origin main", "git push origin main", true},
		{"push with flags", "git push --force", true},
		{"push with path flag", "git -C /path push", true},
		{"push with no-pager", "git --no-pager push", true},
		{"push in chain", "git commit -m 'msg' && git push", true},
		{"push tag", "git push origin v1.0.0", true},
		{"push with upstream", "git push -u origin main", true},
		{"push after newline", "git commit -m 'msg'\ngit push", true},
		{"push after newline with args", "git commit -m 'msg'\ngit push origin main", true},
		{"push after multiple newlines", "git add .\ngit commit -m 'msg'\ngit push", true},

		// Should NOT match
		{"just git", "git status", false},
		{"git commit", "git commit -m 'msg'", false},
		{"empty", "", false},
		{"unrelated command", "npm publish", false},
		{"push in commit message", `git commit -m "feat: add git push support"`, false},
		{"push word in commit message", `git commit -m "fix: push notification handler"`, false},
		{"push in single-quoted message", "git commit -m 'update push logic'", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsGitPushCommand(tt.command)
			if got != tt.want {
				t.Errorf("IsGitPushCommand(%q) = %v, want %v", tt.command, got, tt.want)
			}
		})
	}
}

// TestIsGitAddCommand tests git add detection patterns
func TestIsGitAddCommand(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    bool
	}{
		// Should match
		{"add dot", "git add .", true},
		{"add all flag", "git add -A", true},
		{"add specific file", "git add file.txt", true},
		{"add multiple files", "git add file1.txt file2.txt", true},
		{"add with glob", "git add *.ts", true},
		{"add in chain", "git add . && git commit", true},
		{"add with path flag", "git -C /path add .", true},

		// Should NOT match
		{"just git", "git status", false},
		{"git commit", "git commit -m 'msg'", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsGitAddCommand(tt.command)
			if got != tt.want {
				t.Errorf("IsGitAddCommand(%q) = %v, want %v", tt.command, got, tt.want)
			}
		})
	}
}

// TestExtractCommitMessage tests commit message extraction
func TestExtractCommitMessage(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    string
	}{
		{"single quotes", "git commit -m 'my message'", "my message"},
		{"double quotes", `git commit -m "my message"`, "my message"},
		{"no quotes", "git commit -m message", "message"},
		{"with special chars", `git commit -m "feat: add feature"`, "feat: add feature"},
		{"multiword no quotes", "git commit -m fix-bug", "fix-bug"},
		{"amend with message", `git commit --amend -m "updated"`, "updated"},
		{"no message flag", "git commit", ""},
		{"empty message", `git commit -m ""`, `""`}, // Edge case: returns the quotes
		{"chained command", `git add . && git commit -m "test"`, "test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractCommitMessage(tt.command)
			if got != tt.want {
				t.Errorf("ExtractCommitMessage(%q) = %q, want %q", tt.command, got, tt.want)
			}
		})
	}
}

// TestExtractPushRef tests push ref extraction
func TestExtractPushRef(t *testing.T) {
	tests := []struct {
		name          string
		command       string
		currentBranch string
		want          string
	}{
		{"simple push", "git push", "main", "refs/heads/main"},
		{"push tag", "git push origin v1.0.0", "main", "refs/tags/v1.0.0"},
		{"push tag with prefix", "git push origin refs/tags/v2.0.0", "main", "refs/tags/v2.0.0"},
		{"push no branch", "git push", "", "refs/heads/main"},
		{"push with branch", "git push origin feature", "feature", "refs/heads/feature"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractPushRef(tt.command, tt.currentBranch)
			if got != tt.want {
				t.Errorf("ExtractPushRef(%q, %q) = %q, want %q", tt.command, tt.currentBranch, got, tt.want)
			}
		})
	}
}

// TestExtractGitAddFiles tests git add file pattern extraction
func TestExtractGitAddFiles(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    []string
	}{
		{"add dot", "git add .", []string{"."}},
		{"add all", "git add -A", nil}, // Flags are filtered out
		{"add file", "git add file.txt", []string{"file.txt"}},
		{"add multiple", "git add file1.txt file2.txt", []string{"file1.txt", "file2.txt"}},
		{"add glob", "git add *.ts", []string{"*.ts"}},
		{"add with chain", "git add src/ && git commit", []string{"src/"}},
		{"no add", "git commit -m 'msg'", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractGitAddFiles(tt.command)
			if len(got) != len(tt.want) {
				t.Errorf("ExtractGitAddFiles(%q) = %v, want %v", tt.command, got, tt.want)
				return
			}
			for i, v := range got {
				if v != tt.want[i] {
					t.Errorf("ExtractGitAddFiles(%q)[%d] = %q, want %q", tt.command, i, v, tt.want[i])
				}
			}
		})
	}
}

// TestDetector tests the full event detection flow
func TestDetector(t *testing.T) {
	mock := &MockGitProvider{
		Branch: "main",
		Author: "test@example.com",
		StagedFiles: []schema.FileStatus{
			{Path: "src/app.ts", Status: "modified"},
		},
		PendingFiles: []schema.FileStatus{
			{Path: "src/new.ts", Status: "added"},
		},
		Remote: "origin",
		Ahead:  2,
	}

	detector := NewDetector(mock)

	t.Run("git commit detection", func(t *testing.T) {
		input := `{
			"toolName": "powershell",
			"toolArgs": {"command": "git commit -m 'test message'"},
			"cwd": "/test/repo"
		}`

		evt, err := detector.DetectFromRawInput([]byte(input))
		if err != nil {
			t.Fatalf("DetectFromRawInput failed: %v", err)
		}

		if evt.Commit == nil {
			t.Fatal("Expected commit event, got nil")
		}
		if evt.Commit.Message != "test message" {
			t.Errorf("Message = %q, want %q", evt.Commit.Message, "test message")
		}
		if evt.Commit.Author != "test@example.com" {
			t.Errorf("Author = %q, want %q", evt.Commit.Author, "test@example.com")
		}
		if len(evt.Commit.Files) != 1 {
			t.Errorf("Files count = %d, want 1", len(evt.Commit.Files))
		}
	})

	t.Run("git add && commit chain", func(t *testing.T) {
		input := `{
			"toolName": "powershell",
			"toolArgs": {"command": "git add . && git commit -m 'chained'"},
			"cwd": "/test/repo"
		}`

		evt, err := detector.DetectFromRawInput([]byte(input))
		if err != nil {
			t.Fatalf("DetectFromRawInput failed: %v", err)
		}

		if evt.Commit == nil {
			t.Fatal("Expected commit event, got nil")
		}
		// Should have both staged and pending files merged
		if len(evt.Commit.Files) != 2 {
			t.Errorf("Files count = %d, want 2 (merged staged + pending)", len(evt.Commit.Files))
		}
	})

	t.Run("git push detection", func(t *testing.T) {
		input := `{
			"toolName": "powershell",
			"toolArgs": {"command": "git push origin main"},
			"cwd": "/test/repo"
		}`

		evt, err := detector.DetectFromRawInput([]byte(input))
		if err != nil {
			t.Fatalf("DetectFromRawInput failed: %v", err)
		}

		if evt.Push == nil {
			t.Fatal("Expected push event, got nil")
		}
		if evt.Push.Ref != "refs/heads/main" {
			t.Errorf("Ref = %q, want %q", evt.Push.Ref, "refs/heads/main")
		}
	})

	t.Run("chained commit + push newline detects both", func(t *testing.T) {
		input := `{
			"toolName": "powershell",
			"toolArgs": {"command": "git commit -m 'test message'\ngit push origin main"},
			"cwd": "/test/repo"
		}`

		evt, err := detector.DetectFromRawInput([]byte(input))
		if err != nil {
			t.Fatalf("DetectFromRawInput failed: %v", err)
		}

		if evt.Commit == nil {
			t.Fatal("Expected commit event, got nil")
		}
		if evt.Push == nil {
			t.Fatal("Expected push event, got nil — newline-separated push was not detected")
		}
	})

	t.Run("chained commit + push && detects both", func(t *testing.T) {
		input := `{
			"toolName": "powershell",
			"toolArgs": {"command": "git commit -m 'test' && git push"},
			"cwd": "/test/repo"
		}`

		evt, err := detector.DetectFromRawInput([]byte(input))
		if err != nil {
			t.Fatalf("DetectFromRawInput failed: %v", err)
		}

		if evt.Commit == nil {
			t.Fatal("Expected commit event, got nil")
		}
		if evt.Push == nil {
			t.Fatal("Expected push event, got nil — &&-chained push was not detected")
		}
	})

	t.Run("chained add + commit + push detects all", func(t *testing.T) {
		input := `{
			"toolName": "powershell",
			"toolArgs": {"command": "git add .\ngit commit -m 'feat: new feature'\ngit push -u origin main"},
			"cwd": "/test/repo"
		}`

		evt, err := detector.DetectFromRawInput([]byte(input))
		if err != nil {
			t.Fatalf("DetectFromRawInput failed: %v", err)
		}

		if evt.Commit == nil {
			t.Fatal("Expected commit event, got nil")
		}
		if evt.Push == nil {
			t.Fatal("Expected push event, got nil — multi-line chain was not detected")
		}
	})

	t.Run("file create detection", func(t *testing.T) {
		input := `{
			"toolName": "create",
			"toolArgs": {"path": "src/new.ts", "file_text": "content"},
			"cwd": "/test/repo"
		}`

		evt, err := detector.DetectFromRawInput([]byte(input))
		if err != nil {
			t.Fatalf("DetectFromRawInput failed: %v", err)
		}

		if evt.File == nil {
			t.Fatal("Expected file event, got nil")
		}
		if evt.File.Path != "src/new.ts" {
			t.Errorf("Path = %q, want %q", evt.File.Path, "src/new.ts")
		}
		if evt.File.Action != "create" {
			t.Errorf("Action = %q, want %q", evt.File.Action, "create")
		}
	})

	t.Run("file edit detection", func(t *testing.T) {
		input := `{
			"toolName": "edit",
			"toolArgs": {"path": "src/app.ts", "old_str": "old", "new_str": "new"},
			"cwd": "/test/repo"
		}`

		evt, err := detector.DetectFromRawInput([]byte(input))
		if err != nil {
			t.Fatalf("DetectFromRawInput failed: %v", err)
		}

		if evt.File == nil {
			t.Fatal("Expected file event, got nil")
		}
		if evt.File.Path != "src/app.ts" {
			t.Errorf("Path = %q, want %q", evt.File.Path, "src/app.ts")
		}
		if evt.File.Action != "edit" {
			t.Errorf("Action = %q, want %q", evt.File.Action, "edit")
		}
	})

	t.Run("non-git shell command", func(t *testing.T) {
		input := `{
			"toolName": "powershell",
			"toolArgs": {"command": "npm test"},
			"cwd": "/test/repo"
		}`

		evt, err := detector.DetectFromRawInput([]byte(input))
		if err != nil {
			t.Fatalf("DetectFromRawInput failed: %v", err)
		}

		// Should still have tool event
		if evt.Tool == nil {
			t.Fatal("Expected tool event, got nil")
		}
		if evt.Tool.Name != "powershell" {
			t.Errorf("Tool name = %q, want %q", evt.Tool.Name, "powershell")
		}
		// Should NOT have commit or push
		if evt.Commit != nil {
			t.Error("Did not expect commit event for npm test")
		}
		if evt.Push != nil {
			t.Error("Did not expect push event for npm test")
		}
	})

	t.Run("toolArgs as string", func(t *testing.T) {
		input := `{
			"toolName": "powershell",
			"toolArgs": "{\"command\": \"git commit -m 'test'\"}",
			"cwd": "/test/repo"
		}`

		evt, err := detector.DetectFromRawInput([]byte(input))
		if err != nil {
			t.Fatalf("DetectFromRawInput failed: %v", err)
		}

		if evt.Commit == nil {
			t.Fatal("Expected commit event when toolArgs is string JSON")
		}
	})
}

// TestMergeFiles tests file deduplication
func TestMergeFiles(t *testing.T) {
	existing := []schema.FileStatus{
		{Path: "a.ts", Status: "modified"},
		{Path: "b.ts", Status: "modified"},
	}
	new := []schema.FileStatus{
		{Path: "b.ts", Status: "added"}, // duplicate
		{Path: "c.ts", Status: "added"},
	}

	result := mergeFiles(existing, new)

	if len(result) != 3 {
		t.Errorf("mergeFiles returned %d files, want 3", len(result))
	}

	paths := make(map[string]bool)
	for _, f := range result {
		paths[f.Path] = true
	}

	for _, p := range []string{"a.ts", "b.ts", "c.ts"} {
		if !paths[p] {
			t.Errorf("Missing file %s in merged result", p)
		}
	}
}

// TestRawHookInputParsing tests various JSON input formats
func TestRawHookInputParsing(t *testing.T) {
	detector := NewDetector(&MockGitProvider{Branch: "main"})

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid input",
			input:   `{"toolName": "powershell", "toolArgs": {}, "cwd": "/test"}`,
			wantErr: false,
		},
		{
			name:    "empty toolArgs",
			input:   `{"toolName": "powershell", "cwd": "/test"}`,
			wantErr: false,
		},
		{
			name:    "null toolArgs",
			input:   `{"toolName": "powershell", "toolArgs": null, "cwd": "/test"}`,
			wantErr: false,
		},
		{
			name:    "invalid JSON",
			input:   `{invalid}`,
			wantErr: true,
		},
		{
			name:    "empty input",
			input:   ``,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := detector.DetectFromRawInput([]byte(tt.input))
			if (err != nil) != tt.wantErr {
				t.Errorf("DetectFromRawInput() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestDetectorWithRealInput tests with realistic Copilot hook payloads
func TestDetectorWithRealInput(t *testing.T) {
	mock := &MockGitProvider{
		Branch:      "feature/test",
		Author:      "dev@company.com",
		StagedFiles: []schema.FileStatus{{Path: "src/index.ts", Status: "modified"}},
	}
	detector := NewDetector(mock)

	// Real-world Copilot hook payload
	realPayload := map[string]interface{}{
		"toolName": "powershell",
		"toolArgs": map[string]interface{}{
			"command":     "git add -A && git commit -m 'feat: implement login'",
			"description": "Commit changes",
		},
		"cwd": "C:\\Users\\dev\\project",
	}

	input, _ := json.Marshal(realPayload)
	evt, err := detector.DetectFromRawInput(input)
	if err != nil {
		t.Fatalf("Failed to parse real payload: %v", err)
	}

	if evt.Commit == nil {
		t.Fatal("Expected commit event from real payload")
	}
	if evt.Commit.Message != "feat: implement login" {
		t.Errorf("Message = %q, want %q", evt.Commit.Message, "feat: implement login")
	}
	if evt.Cwd != "C:\\Users\\dev\\project" {
		t.Errorf("Cwd = %q, want Windows path", evt.Cwd)
	}
}

// TestDetectorToolArgsStringForm tests that the detector correctly parses
// toolArgs when it arrives as a JSON string (preToolUse format) rather than
// a parsed JSON object (postToolUse format). This is critical because
// evt.Tool.Args must be populated for compliance exemption checks.
func TestDetectorToolArgsStringForm(t *testing.T) {
	detector := NewDetector(&MockGitProvider{Branch: "main"})

	tests := []struct {
		name        string
		input       string
		wantCommand string
		wantArgKeys []string
	}{
		{
			name:        "toolArgs as object (postToolUse format)",
			input:       `{"toolName":"powershell","toolArgs":{"command":"gh hookflow init"},"cwd":"/test"}`,
			wantCommand: "gh hookflow init",
			wantArgKeys: []string{"command"},
		},
		{
			name:        "toolArgs as string (preToolUse format)",
			input:       `{"toolName":"powershell","toolArgs":"{\"command\":\"gh hookflow init\"}","cwd":"/test"}`,
			wantCommand: "gh hookflow init",
			wantArgKeys: []string{"command"},
		},
		{
			name:        "toolArgs string with path for view tool",
			input:       `{"toolName":"view","toolArgs":"{\"path\":\"/home/user/.hookflow/sessions/abc/error.md\"}","cwd":"/test"}`,
			wantCommand: "",
			wantArgKeys: []string{"path"},
		},
		{
			name:        "toolArgs string with multiple args",
			input:       `{"toolName":"edit","toolArgs":"{\"path\":\"src/main.go\",\"old_str\":\"foo\",\"new_str\":\"bar\"}","cwd":"/test"}`,
			wantCommand: "",
			wantArgKeys: []string{"path", "old_str", "new_str"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evt, err := detector.DetectFromRawInput([]byte(tt.input))
			if err != nil {
				t.Fatalf("DetectFromRawInput() error = %v", err)
			}

			if evt.Tool == nil {
				t.Fatal("evt.Tool should not be nil")
			}

			// Check tool args were populated
			for _, key := range tt.wantArgKeys {
				if _, ok := evt.Tool.Args[key]; !ok {
					t.Errorf("evt.Tool.Args[%q] not found; Args = %v", key, evt.Tool.Args)
				}
			}

			// Check command value if expected
			if tt.wantCommand != "" {
				cmd, ok := evt.Tool.Args["command"]
				if !ok {
					t.Fatalf("evt.Tool.Args[\"command\"] not found")
				}
				if cmdStr, ok := cmd.(string); !ok || cmdStr != tt.wantCommand {
					t.Errorf("command = %v, want %q", cmd, tt.wantCommand)
				}
			}
		})
	}
}
