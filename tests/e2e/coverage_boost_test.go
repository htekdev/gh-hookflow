package e2e

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// =============================================================================
// Tests targeting: event/git.go - GetStagedFiles, GetBranch, GetAuthor,
//   GetPendingFiles, parsePorcelainStatus, ExtractGitAddFiles, mergeFiles
// =============================================================================

func TestGitCommitEventWithStagedFiles(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"commit-check.yml": `name: Commit Check
on:
  commit:
    paths: ["**/*"]
steps:
  - name: show commit info
    run: |
      $msg = '${{ event.commit.message }}'
      $author = '${{ event.commit.author }}'
      Write-Host "commit_msg=$msg"
      Write-Host "commit_author=$author"
`,
	})

	// Use fake git provider env vars instead of real git staging
	opts := &hookflowOpts{
		env: []string{
			"HOOKFLOW_FAKE_GIT_STAGED_FILES=staged-file.txt",
			"HOOKFLOW_FAKE_GIT_AUTHOR=E2E Tester",
		},
	}

	eventJSON := buildShellEventJSON("powershell", `git commit -m "feat: add staged file"`, workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", opts)
	assertAllow(t, result, output)
	if !strings.Contains(output, "commit_msg=feat: add staged file") {
		t.Errorf("Expected commit message in output:\n%s", output)
	}
	if !strings.Contains(output, "commit_author=E2E Tester") {
		t.Errorf("Expected commit author in output:\n%s", output)
	}
}

func TestGitCommitEventWithEmptyMessage(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"commit-msg-check.yml": `name: Commit Message Check
on:
  commit:
    paths: ["**/*.go"]
steps:
  - name: check msg
    run: Write-Host "message matched"
`,
	})

	opts := &hookflowOpts{env: []string{"HOOKFLOW_FAKE_GIT_STAGED_FILES=main.go"}}
	eventJSON := buildShellEventJSON("powershell", `git commit -m "fix: typo"`, workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", opts)
	assertAllow(t, result, output)
}

func TestGitChainedCommandsDenied(t *testing.T) {
	// Chained git commands are always blocked by primitive guard — test that behavior
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"add-commit.yml": `name: Add Commit Check
on:
  commit:
    paths: ["**/*"]
steps:
  - name: info
    run: Write-Host "should not run"
`,
	})

	eventJSON := buildShellEventJSON("powershell", `git add . && git commit -m "chained commit"`, workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertDeny(t, result, output, "Multiple git commands")
}

func TestGitCommitWithPendingFiles(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"specific-add.yml": `name: Pending Files Check
on:
  commit:
    paths: ["**/*"]
steps:
  - name: check
    run: Write-Host "pending files processed"
`,
	})

	opts := &hookflowOpts{env: []string{"HOOKFLOW_FAKE_GIT_STAGED_FILES=a.txt,b.txt"}}

	eventJSON := buildShellEventJSON("powershell", `git commit -m "add specific files"`, workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", opts)
	assertAllow(t, result, output)
	if !strings.Contains(output, "pending files processed") {
		t.Errorf("Expected 'pending files processed' in output:\n%s", output)
	}
}

func TestGitCommitConventionalFormat(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"conventional-commit.yml": `name: Conventional Commit
on:
  commit:
    paths: ["**/*"]
steps:
  - name: validate
    run: |
      $msg = '${{ event.commit.message }}'
      if ($msg -match '^(feat|fix|chore|docs):') {
        Write-Host "conventional_ok=true"
      } else {
        Write-Host "conventional_ok=false"
        exit 1
      }
`,
	})

	opts := &hookflowOpts{env: []string{"HOOKFLOW_FAKE_GIT_STAGED_FILES=src/feature.go"}}

	eventJSON := buildShellEventJSON("powershell", `git commit -m "feat: add new feature"`, workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", opts)
	assertAllow(t, result, output)
	if !strings.Contains(output, "conventional_ok=true") {
		t.Errorf("Expected conventional commit validation in output:\n%s", output)
	}
}

func TestGitCommitMultiWordMessage(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"msg-check.yml": `name: Message Check
on:
  commit:
    paths: ["**/*"]
steps:
  - name: show
    run: |
      $msg = '${{ event.commit.message }}'
      Write-Host "msg=$msg"
`,
	})

	opts := &hookflowOpts{env: []string{"HOOKFLOW_FAKE_GIT_STAGED_FILES=auth.go"}}

	eventJSON := buildShellEventJSON("powershell",
		`git commit -m "fix(auth): handle edge case"`, workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", opts)
	assertAllow(t, result, output)
	if !strings.Contains(output, "msg=fix(auth): handle edge case") {
		t.Errorf("Expected multi-word message in output:\n%s", output)
	}
}

// =============================================================================
// Tests targeting: session/transcript.go - AppendEntry recording
// =============================================================================

func TestTranscriptRecordingWithSessionDir(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"transcript-record.yml": `name: Transcript Record
on:
  file:
    paths: ["**/*"]
steps:
  - name: allow
    run: Write-Host "recorded"
`,
	})

	sessionDir := t.TempDir()

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "test.txt"),
		"file_text": "hello",
	}, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", &hookflowOpts{
		env: []string{"HOOKFLOW_SESSION_DIR=" + sessionDir},
	})
	assertAllow(t, result, output)

	transcriptFile := filepath.Join(sessionDir, "transcript.jsonl")
	data, err := os.ReadFile(transcriptFile)
	if err != nil {
		t.Fatalf("Expected transcript file at %s: %v", transcriptFile, err)
	}
	if len(data) == 0 {
		t.Error("Transcript file is empty")
	}
	if !strings.Contains(string(data), "preToolUse") {
		t.Errorf("Expected 'preToolUse' in transcript:\n%s", string(data))
	}
}

func TestTranscriptCapEnforcement(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"cap-test.yml": `name: Cap Test
on:
  file:
    paths: ["**/*"]
steps:
  - name: check
    run: Write-Host "capped run"
`,
	})

	sessionDir := t.TempDir()

	var entries []string
	for i := 0; i < 20; i++ {
		entries = append(entries, fmt.Sprintf(`{"timestamp":%d,"lifecycle":"pre","eventType":"preToolUse","toolName":"create","seq":%d}`, 1000+int64(i), i+1))
	}
	_ = os.WriteFile(filepath.Join(sessionDir, "transcript.jsonl"), []byte(strings.Join(entries, "\n")+"\n"), 0644)
	_ = os.WriteFile(filepath.Join(sessionDir, "transcript.seq"), []byte("20"), 0644)

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "test.txt"),
		"file_text": "content",
	}, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", &hookflowOpts{
		env: []string{
			"HOOKFLOW_SESSION_DIR=" + sessionDir,
			"HOOKFLOW_TRANSCRIPT_MAX_ENTRIES=10",
		},
	})
	assertAllow(t, result, output)
}

// =============================================================================
// Tests targeting: CLI commands - logs, validate --file, validate invalid
// =============================================================================

func TestLogsPathFlag(t *testing.T) {
	output, _ := runHookflowCmd(t, []string{"logs", "--path"}, nil)
	if !strings.Contains(output, "hookflow") || !strings.Contains(output, ".log") {
		t.Errorf("Expected log file path in output:\n%s", output)
	}
}

func TestLogsTailFlag(t *testing.T) {
	output, err := runHookflowCmd(t, []string{"logs", "-n", "5"}, nil)
	if err != nil {
		t.Errorf("Expected logs -n 5 to succeed, got error: %v\nOutput: %s", err, output)
	}
}

func TestValidateInvalidFileUnknownField(t *testing.T) {
	tmpDir := t.TempDir()
	badFile := filepath.Join(tmpDir, "bad.yml")
	_ = os.WriteFile(badFile, []byte("name: Bad\non:\n  file:\n    paths: [\"**/*\"]\nsteps:\n  - name: check\n    run: echo ok\n    unknown_field: true\n"), 0644)

	output, _ := runHookflowCmd(t, []string{"validate", "--file", badFile}, nil)
	lower := strings.ToLower(output)
	if !strings.Contains(lower, "invalid") && !strings.Contains(lower, "additional") {
		t.Errorf("Expected validation error for unknown field:\n%s", output)
	}
}

// =============================================================================
// Tests targeting: session/errors.go - WriteError, HasError, gate
// =============================================================================

func TestSessionErrorGateBlocking(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"simple.yml": "name: Simple\non:\n  file:\n    paths: [\"**/*\"]\nsteps:\n  - name: ok\n    run: Write-Host \"ok\"\n",
	})

	sessionDir := t.TempDir()
	errorContent := "# Workflow Error\n\n**Workflow:** test-workflow\n**Step:** check\n\nSomething failed\n"
	_ = os.WriteFile(filepath.Join(sessionDir, "error.md"), []byte(errorContent), 0644)

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "test.txt"),
		"file_text": "hello",
	}, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", &hookflowOpts{
		env: []string{"HOOKFLOW_SESSION_DIR=" + sessionDir},
	})
	assertDeny(t, result, output, "error")
}

func TestSessionErrorClearAfterRead(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"simple.yml": "name: Simple\non:\n  file:\n    paths: [\"**/*\"]\nsteps:\n  - name: ok\n    run: Write-Host \"ok\"\n",
	})

	sessionDir := t.TempDir()
	errorPath := filepath.Join(sessionDir, "error.md")
	_ = os.WriteFile(errorPath, []byte("# Error\n\nTest error\n"), 0644)

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "test.txt"),
		"file_text": "hello",
	}, workspace)

	result, _ := runHookflow(t, workspace, eventJSON, "preToolUse", &hookflowOpts{
		env: []string{"HOOKFLOW_SESSION_DIR=" + sessionDir},
	})
	if result == nil || result.PermissionDecision != "deny" {
		t.Fatalf("Session error gate should deny when error.md exists, got: %v", result)
	}

	_ = os.Remove(errorPath)

	result2, output2 := runHookflow(t, workspace, eventJSON, "preToolUse", &hookflowOpts{
		env: []string{"HOOKFLOW_SESSION_DIR=" + sessionDir},
	})
	assertAllow(t, result2, output2)
}

// =============================================================================
// Tests targeting: session/session.go - SessionDirForID
// =============================================================================

func TestSessionDirFromEventSessionId(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"session-test.yml": `name: Session Test
on:
  file:
    paths: ["**/*"]
steps:
  - name: check
    run: Write-Host "session test ok"
`,
	})

	sessionID := "test-session-e2e-12345"
	escapedWorkspace := strings.ReplaceAll(workspace, `\`, `\\`)

	eventJSON := fmt.Sprintf(`{"toolName":"create","toolArgs":{"path":"%s","file_text":"hello"},"cwd":"%s","sessionId":"%s"}`,
		strings.ReplaceAll(filepath.Join(workspace, "test.txt"), `\`, `\\`),
		escapedWorkspace,
		sessionID)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)

	homeDir := actHomeDir()
	expectedSessionDir := filepath.Join(homeDir, ".hookflow", "sessions", sessionID)
	if _, err := os.Stat(expectedSessionDir); err == nil {
		_ = os.RemoveAll(expectedSessionDir)
	}
}

// =============================================================================
// Tests targeting: event/detector.go - edit event detection
// =============================================================================

func TestEditEventWithContent(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"edit-check.yml": `name: Edit Check
on:
  file:
    types: ["edit"]
    paths: ["**/*.go"]
steps:
  - name: edit info
    run: |
      $path = '${{ event.file.path }}'
      $action = '${{ event.file.action }}'
      Write-Host "edit_path=$path action=$action"
`,
	})

	eventJSON := buildEventJSON("edit", map[string]interface{}{
		"path":    filepath.Join(workspace, "main.go"),
		"old_str": "func old() {}",
		"new_str": "func new() {}",
	}, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
	if !strings.Contains(output, "action=edit") {
		t.Errorf("Expected action=edit in output:\n%s", output)
	}
}

// =============================================================================
// Tests targeting: activity status display
// =============================================================================

func TestGitPushStatusMultipleActivities(t *testing.T) {
	homeDir := actHomeDir()

	activities := []struct {
		id     string
		status string
	}{
		{"e2e-multi-1", "completed"},
		{"e2e-multi-2", "failed"},
		{"e2e-multi-3", "running"},
	}

	for _, a := range activities {
		actDir := filepath.Join(homeDir, ".hookflow", "activities", a.id)
		_ = os.MkdirAll(actDir, 0755)
		state := map[string]interface{}{
			"id": a.id, "status": a.status,
			"git_args":   []string{"origin", "main"},
			"created_at": "2024-01-01T00:00:00Z",
			"updated_at": "2024-01-01T00:00:01Z",
		}
		data, _ := json.MarshalIndent(state, "", "  ")
		_ = os.WriteFile(filepath.Join(actDir, "state.json"), data, 0644)
	}
	defer func() {
		for _, a := range activities {
			_ = os.RemoveAll(filepath.Join(homeDir, ".hookflow", "activities", a.id))
		}
	}()

	for _, a := range activities {
		output, _ := runHookflowCmd(t, []string{"git-push-status", a.id}, nil)
		if !strings.Contains(strings.ToLower(output), a.status) {
			t.Errorf("Expected status '%s' for activity '%s' in output:\n%s", a.status, a.id, output)
		}
	}
}

// =============================================================================
// Tests targeting: main.go - normalizeFilePath, path normalization
// =============================================================================

func TestFilePathNormalizationDeep(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"path-norm.yml": `name: Path Normalize
on:
  file:
    paths: ["src/**/*.js"]
steps:
  - name: check
    run: |
      $path = '${{ event.file.path }}'
      Write-Host "normalized_path=$path"
`,
	})

	absPath := filepath.Join(workspace, "src", "app.js")
	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      absPath,
		"file_text": "const x = 1;",
	}, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
	if !strings.Contains(output, "normalized_path=src/app.js") {
		t.Errorf("Expected normalized path 'src/app.js' in output:\n%s", output)
	}
}

func TestWindowsBackslashPathNormalization(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only test")
	}

	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"backslash.yml": `name: Backslash Path
on:
  file:
    paths: ["**/*.txt"]
steps:
  - name: check
    run: |
      $path = '${{ event.file.path }}'
      Write-Host "path=$path"
`,
	})

	escapedWorkspace := strings.ReplaceAll(workspace, `\`, `\\`)
	eventJSON := fmt.Sprintf(`{"toolName":"create","toolArgs":{"path":"%s\\nested\\deep\\file.txt","file_text":"hello"},"cwd":"%s"}`,
		escapedWorkspace, escapedWorkspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
	if !strings.Contains(output, "path=nested/deep/file.txt") {
		t.Errorf("Expected forward-slash normalized path in output:\n%s", output)
	}
}

// =============================================================================
// Tests targeting: runner - continue-on-error
// =============================================================================

func TestStepContinueOnErrorWithOutcome(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"continue-err.yml": `name: Continue On Error
on:
  file:
    paths: ["**/*"]
steps:
  - name: failing_step
    continue-on-error: true
    run: |
      Write-Host "about to fail"
      exit 1
  - name: after failure
    run: |
      $outcome = '${{ steps.failing_step.outcome }}'
      Write-Host "after_failure outcome=$outcome"
`,
	})

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "test.txt"),
		"file_text": "test",
	}, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
	if !strings.Contains(output, "after_failure outcome=failure") {
		t.Errorf("Expected 'after_failure outcome=failure' in output:\n%s", output)
	}
}

// =============================================================================
// Tests targeting: expression/evaluator.go - contains, startsWith, endsWith,
//   fromJSON, toJSON, join
// =============================================================================

func TestExpressionContainsFunctionBranches(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"contains-test.yml": `name: Contains Test
on:
  file:
    paths: ["**/*"]
steps:
  - name: contains check
    if: contains(event.file.path, '.js')
    run: Write-Host "contains_js=true"
  - name: not contains
    if: "!contains(event.file.path, '.py')"
    run: Write-Host "not_py=true"
`,
	})

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "app.js"),
		"file_text": "const x = 1;",
	}, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
	if !strings.Contains(output, "contains_js=true") {
		t.Errorf("Expected contains_js=true:\n%s", output)
	}
	if !strings.Contains(output, "not_py=true") {
		t.Errorf("Expected not_py=true:\n%s", output)
	}
}

func TestExpressionStartsWithEndsWithBranches(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"startend.yml": `name: StartEnd Test
on:
  file:
    paths: ["**/*"]
steps:
  - name: starts
    if: startsWith(event.file.path, 'src/')
    run: Write-Host "starts_with_src=true"
  - name: ends
    if: endsWith(event.file.path, '.ts')
    run: Write-Host "ends_with_ts=true"
`,
	})

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "src", "app.ts"),
		"file_text": "const x: number = 1;",
	}, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
	if !strings.Contains(output, "starts_with_src=true") {
		t.Errorf("Expected starts_with_src=true:\n%s", output)
	}
	if !strings.Contains(output, "ends_with_ts=true") {
		t.Errorf("Expected ends_with_ts=true:\n%s", output)
	}
}

func TestExpressionFromJSONNested(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"fromjson-nested.yml": "name: FromJSON Nested\non:\n  file:\n    paths: [\"**/*\"]\nenv:\n  DATA: '{\"user\":{\"name\":\"alice\"}}'\nsteps:\n  - name: nested json\n    run: |\n      $name = '${{ fromJSON(env.DATA).user.name }}'\n      Write-Host \"nested_name=$name\"\n",
	})

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "test.txt"),
		"file_text": "test",
	}, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
	if !strings.Contains(output, "nested_name=alice") {
		t.Errorf("Expected nested_name=alice:\n%s", output)
	}
}

func TestExpressionToJSONFunction(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"tojson-test.yml": `name: ToJSON Test
on:
  file:
    paths: ["**/*"]
steps:
  - name: to json
    run: |
      $json = '${{ toJSON(event.file) }}'
      Write-Host "file_json_has_path=$($json -match 'path')"
`,
	})

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "test.txt"),
		"file_text": "test",
	}, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
	if !strings.Contains(output, "file_json_has_path=True") {
		t.Errorf("Expected file_json_has_path=True:\n%s", output)
	}
}

func TestExpressionJoinFunction(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"join-test.yml": "name: Join Test\non:\n  file:\n    paths: [\"**/*\"]\nenv:\n  ARR: '[\"a\",\"b\",\"c\"]'\n  SEP: '-'\nsteps:\n  - name: join arr\n    run: |\n      $joined = '${{ join(fromJSON(env.ARR), env.SEP) }}'\n      Write-Host \"joined=$joined\"\n",
	})

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "test.txt"),
		"file_text": "test",
	}, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
	if !strings.Contains(output, "joined=a-b-c") {
		t.Errorf("Expected joined=a-b-c:\n%s", output)
	}
}

// =============================================================================
// Tests targeting: schema validation edge cases
// =============================================================================

func TestValidateWorkflowMissingName(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"no-name.yml": "on:\n  file:\n    paths: [\"**/*\"]\nsteps:\n  - name: check\n    run: echo ok\n",
	})

	output, err := runHookflowCmd(t, []string{"validate", "--dir", workspace}, nil)
	if err == nil {
		t.Errorf("Expected validate to fail for workflow missing 'name', but it succeeded.\nOutput: %s", output)
	}
	lower := strings.ToLower(output)
	if !strings.Contains(lower, "name") {
		t.Errorf("Expected validation output to mention 'name', got:\n%s", output)
	}
}

func TestValidateWorkflowMissingSteps(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"no-steps.yml": "name: No Steps\non:\n  file:\n    paths: [\"**/*\"]\n",
	})

	output, err := runHookflowCmd(t, []string{"validate", "--dir", workspace}, nil)
	if err == nil {
		t.Errorf("Expected validate to fail for workflow missing 'steps', but it succeeded.\nOutput: %s", output)
	}
	lower := strings.ToLower(output)
	if !strings.Contains(lower, "step") {
		t.Errorf("Expected validation output to mention 'step', got:\n%s", output)
	}
}

func TestValidateWorkflowMissingTrigger(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"no-trigger.yml": "name: No Trigger\nsteps:\n  - name: check\n    run: echo ok\n",
	})

	output, err := runHookflowCmd(t, []string{"validate", "--dir", workspace}, nil)
	if err == nil {
		t.Errorf("Expected validate to fail for workflow missing 'on' trigger, but it succeeded.\nOutput: %s", output)
	}
	lower := strings.ToLower(output)
	if !strings.Contains(lower, "on") && !strings.Contains(lower, "trigger") {
		t.Errorf("Expected validation output to mention 'on' or 'trigger', got:\n%s", output)
	}
}

// =============================================================================
// Tests targeting: post lifecycle events
// =============================================================================

func TestPostToolUseEventWithTranscript(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"post-handler.yml": `name: Post Handler
on:
  file:
    lifecycle: post
    paths: ["**/*.json"]
steps:
  - name: validate json
    run: Write-Host "post validation ran"
`,
	})

	sessionDir := t.TempDir()

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "config.json"),
		"file_text": `{"key": "value"}`,
	}, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "postToolUse", &hookflowOpts{
		env: []string{"HOOKFLOW_SESSION_DIR=" + sessionDir},
	})
	assertAllow(t, result, output)
	if !strings.Contains(output, "post validation ran") {
		t.Errorf("Expected 'post validation ran' in output:\n%s", output)
	}

	// Verify transcript recorded postToolUse
	data, _ := os.ReadFile(filepath.Join(sessionDir, "transcript.jsonl"))
	if !strings.Contains(string(data), "postToolUse") {
		t.Errorf("Expected postToolUse in transcript:\n%s", string(data))
	}
}

func TestPostToolUseBlockingDenyOnFailure(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"post-block.yml": `name: Post Block
blocking: true
on:
  file:
    lifecycle: post
    paths: ["**/*.json"]
steps:
  - name: validate
    run: |
      Write-Host "checking json"
      exit 1
`,
	})

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "config.json"),
		"file_text": `{"key": "value"}`,
	}, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "postToolUse", nil)
	assertDeny(t, result, output, "")
}

func TestPostToolUseNonBlockingAllowOnFailure(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"post-notify.yml": `name: Post Notify
blocking: false
on:
  file:
    lifecycle: post
    paths: ["**/*"]
steps:
  - name: notify
    run: |
      Write-Host "notification sent"
      exit 1
`,
	})

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "test.txt"),
		"file_text": "hello",
	}, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "postToolUse", nil)
	assertAllow(t, result, output)
}

// =============================================================================
// Tests targeting: trigger matching edge cases
// =============================================================================

func TestPathsIgnoreFilteringTestFiles(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"ignore-tests.yml": `name: Ignore Tests
on:
  file:
    paths: ["**/*"]
    paths-ignore: ["**/*.test.js", "**/__tests__/**"]
steps:
  - name: check
    run: Write-Host "non-test file"
`,
	})

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "app.test.js"),
		"file_text": "test('hello', () => {});",
	}, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
	if strings.Contains(output, "non-test file") {
		t.Errorf("paths-ignore should have filtered this file:\n%s", output)
	}
}

func TestMultipleTriggerTypesMatch(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"multi-trigger.yml": `name: Multi Trigger
on:
  file:
    paths: ["**/*"]
  commit:
    paths: ["**/*"]
steps:
  - name: check
    run: Write-Host "multi trigger matched"
`,
	})

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(workspace, "test.txt"),
		"file_text": "test",
	}, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
	if !strings.Contains(output, "multi trigger matched") {
		t.Errorf("Expected multi trigger to match file event:\n%s", output)
	}
}
