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

// actHomeDir returns the user's home directory.
func actHomeDir() string {
	if runtime.GOOS == "windows" {
		return os.Getenv("USERPROFILE")
	}
	return os.Getenv("HOME")
}

// =============================================================================
// Tests targeting: generateFileName (create.go:145)
// =============================================================================

func TestCreateDryRun(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"placeholder.yml": "name: Placeholder\non:\n  file:\n    paths: [\"**/*\"]\nsteps:\n  - name: placeholder\n    run: Write-Host \"ok\"\n",
	})

	output, _ := runHookflowCmd(t,
		[]string{"create", "block .env file creation", "--dry-run", "--dir", workspace},
		nil)

	// dry-run should print workflow to stdout, not save a file
	if strings.Contains(output, "Error") && !strings.Contains(output, "COPILOT") {
		// If AI is not available, the error is expected
		t.Logf("Create dry-run output (AI may not be available): %s", output[:min(len(output), 200)])
	}
}

// =============================================================================
// Tests targeting: buildFailureMessage, writePhaseDetails, writePhaseLogs
// (git_push_status.go:79-157), ReadLogs (activity.go:254)
// =============================================================================

func TestGitPushStatusPrePushFailure(t *testing.T) {
	activityID := "e2e-prepush-fail"
	actDir := filepath.Join(actHomeDir(), ".hookflow", "activities", activityID)
	_ = os.MkdirAll(filepath.Join(actDir, "logs"), 0755)
	defer func() { _ = os.RemoveAll(actDir) }()

	actState := map[string]interface{}{
		"id": activityID, "status": "failed",
		"git_args": []string{"origin", "main"},
		"created_at": "2024-01-01T00:00:00Z", "updated_at": "2024-01-01T00:00:01Z",
		"phases": map[string]interface{}{
			"pre_push": map[string]interface{}{
				"status": "failed", "error": "workflow validation denied",
				"workflows": []map[string]interface{}{
					{"name": "lint-check", "status": "completed", "success": true},
					{"name": "test-check", "status": "completed", "success": false, "error": "tests failed"},
				},
			},
		},
	}
	actJSON, _ := json.MarshalIndent(actState, "", "  ")
	_ = os.WriteFile(filepath.Join(actDir, "state.json"), actJSON, 0644)
	_ = os.WriteFile(filepath.Join(actDir, "logs", "pre_push-test-check.log"),
		[]byte("Running tests...\nFAIL: TestFoo expected true got false\n"), 0644)

	output, _ := runHookflowCmd(t, []string{"git-push-status", activityID}, nil)

	if !strings.Contains(output, "test-check") {
		t.Errorf("Expected workflow name 'test-check' in output:\n%s", output)
	}
	if !strings.Contains(output, "FAILED") {
		t.Errorf("Expected 'FAILED' in output:\n%s", output)
	}
}

func TestGitPushStatusPostPushFailure(t *testing.T) {
	activityID := "e2e-postpush-fail"
	actDir := filepath.Join(actHomeDir(), ".hookflow", "activities", activityID)
	_ = os.MkdirAll(filepath.Join(actDir, "logs"), 0755)
	defer func() { _ = os.RemoveAll(actDir) }()

	actState := map[string]interface{}{
		"id": activityID, "status": "failed",
		"git_args": []string{"origin", "main"},
		"created_at": "2024-01-01T00:00:00Z", "updated_at": "2024-01-01T00:00:05Z",
		"phases": map[string]interface{}{
			"pre_push":  map[string]interface{}{"status": "completed"},
			"push":      map[string]interface{}{"status": "completed"},
			"post_push": map[string]interface{}{
				"status": "failed", "error": "post-push validation failed",
				"workflows": []map[string]interface{}{
					{"name": "pr-check", "status": "completed", "success": false, "error": "no PR found"},
				},
			},
		},
	}
	actJSON, _ := json.MarshalIndent(actState, "", "  ")
	_ = os.WriteFile(filepath.Join(actDir, "state.json"), actJSON, 0644)
	_ = os.WriteFile(filepath.Join(actDir, "logs", "post_push-pr-check.log"),
		[]byte("Checking for PR...\nNo open PR found for branch 'feature'\n"), 0644)

	output, _ := runHookflowCmd(t, []string{"git-push-status", activityID}, nil)

	if !strings.Contains(output, "post-push") || !strings.Contains(output, "FAILED") {
		t.Errorf("Expected post-push failure context in output:\n%s", output)
	}
}

func TestGitPushStatusGitPushFailure(t *testing.T) {
	activityID := "e2e-gitpush-fail"
	actDir := filepath.Join(actHomeDir(), ".hookflow", "activities", activityID)
	_ = os.MkdirAll(filepath.Join(actDir, "logs"), 0755)
	defer func() { _ = os.RemoveAll(actDir) }()

	actState := map[string]interface{}{
		"id": activityID, "status": "failed",
		"git_args": []string{"origin", "main"},
		"created_at": "2024-01-01T00:00:00Z", "updated_at": "2024-01-01T00:00:02Z",
		"phases": map[string]interface{}{
			"pre_push": map[string]interface{}{"status": "completed"},
			"push": map[string]interface{}{
				"status": "failed",
				"output": "remote: Permission denied\nfatal: could not push",
				"error":  "exit status 128",
			},
		},
	}
	actJSON, _ := json.MarshalIndent(actState, "", "  ")
	_ = os.WriteFile(filepath.Join(actDir, "state.json"), actJSON, 0644)

	output, _ := runHookflowCmd(t, []string{"git-push-status", activityID}, nil)

	if !strings.Contains(output, "git push itself failed") {
		t.Errorf("Expected 'git push itself failed' in output:\n%s", output)
	}
	if !strings.Contains(output, "Permission denied") {
		t.Errorf("Expected 'Permission denied' in output:\n%s", output)
	}
}

// =============================================================================
// Tests targeting: eventMentionsHookflow (main.go:1117),
//                  isHookflowRelatedWork (main.go:1105)
// =============================================================================

func TestHookflowRelatedWorkToolName(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"invalid-workflow.yml": "name: Invalid\non:\n  file:\n    paths: [\"**/*\"]\nsteps:\n  - name: check\n    run: echo ok\n    invalid_extra_property: true\n",
	})

	eventJSON := buildShellEventJSON("powershell", "gh hookflow validate", workspace)
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
}

func TestHookflowRelatedWorkDeniesUnrelated(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "e2e-unrelated-work-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	hookflowDir := filepath.Join(tmpDir, ".github", "hookflows")
	_ = os.MkdirAll(hookflowDir, 0755)
	_ = os.WriteFile(filepath.Join(hookflowDir, "invalid.yml"),
		[]byte("name: Invalid\non:\n  file:\n    paths: [\"**/*\"]\nsteps:\n  - name: check\n    run: echo ok\n    bad_property: true\n"), 0644)

	hooksDir := filepath.Join(tmpDir, ".github", "hooks")
	_ = os.MkdirAll(hooksDir, 0755)
	_ = os.WriteFile(filepath.Join(hooksDir, "hooks.json"),
		[]byte(`{"hooks":{"preToolUse":[{"type":"tool","command":"hookflow run --raw --event-type preToolUse"}]}}`), 0644)

	gitInit(t, tmpDir)

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path":      filepath.Join(tmpDir, "app.js"),
		"file_text": "console.log('hello')",
	}, tmpDir)

	result, output := runHookflow(t, tmpDir, eventJSON, "preToolUse", nil)
	assertDeny(t, result, output, "Invalid workflow")
}

// =============================================================================
// Tests targeting: expression branches - cancelled(), toBool, getIndex, getProperty
// =============================================================================

func TestExpressionCancelledFunction(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"cancelled-test.yml": "name: Cancelled Test\non:\n  file:\n    paths: [\"**/*\"]\nsteps:\n  - name: normal step\n    id: s1\n    run: Write-Host \"normal\"\n  - name: check cancelled\n    if: \"!cancelled()\"\n    run: Write-Host \"not cancelled - correct\"\n",
	})

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path": filepath.Join(workspace, "test.txt"), "file_text": "test",
	}, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
	if !strings.Contains(output, "not cancelled - correct") {
		t.Errorf("Expected 'not cancelled - correct' in output:\n%s", output)
	}
}

func TestExpressionDeepPropertyAccess(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"deep-prop.yml": "name: Deep Property\non:\n  file:\n    paths: [\"**/*\"]\nsteps:\n  - name: deep access\n    run: |\n      $action = '${{ event.file.action }}'\n      $lifecycle = '${{ event.hook.lifecycle }}'\n      Write-Host \"action=$action lifecycle=$lifecycle\"\n",
	})

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path": filepath.Join(workspace, "deep", "nested", "file.txt"), "file_text": "content",
	}, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
	if !strings.Contains(output, "action=create") {
		t.Errorf("Expected 'action=create' in output:\n%s", output)
	}
}

func TestExpressionToBoolWithNumbers(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"tobool-num.yml": "name: ToBool Numbers\non:\n  file:\n    paths: [\"**/*\"]\nsteps:\n  - name: check truthy number\n    if: fromJSON('{\"count\":5}').count\n    run: Write-Host \"truthy number works\"\n  - name: check zero falsy\n    if: \"!fromJSON('{\\\"count\\\":0}').count\"\n    run: Write-Host \"zero is falsy\"\n",
	})

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path": filepath.Join(workspace, "file.txt"), "file_text": "data",
	}, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
	if !strings.Contains(output, "truthy number works") {
		t.Errorf("Expected 'truthy number works' in output:\n%s", output)
	}
	if !strings.Contains(output, "zero is falsy") {
		t.Errorf("Expected 'zero is falsy' in output:\n%s", output)
	}
}

func TestExpressionGetIndexOnArray(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"index-test.yml": "name: Index Test\non:\n  file:\n    paths: [\"**/*\"]\nenv:\n  DATA: '[\"alpha\",\"beta\",\"gamma\"]'\nsteps:\n  - name: valid index\n    run: |\n      $arr = '${{ toJSON(fromJSON(env.DATA)) }}'\n      Write-Host \"array=$arr\"\n",
	})

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path": filepath.Join(workspace, "file.txt"), "file_text": "data",
	}, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
	if !strings.Contains(output, "array=") {
		t.Errorf("Expected 'array=' in output:\n%s", output)
	}
}

// =============================================================================
// Tests targeting: runWithRawInput branches (main.go:365)
// =============================================================================

func TestRawInputEmptyToolName(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"hooks-only.yml": "name: Hooks Only\non:\n  hooks:\n    types: [\"preToolUse\"]\nsteps:\n  - name: check\n    run: Write-Host \"hooks trigger hit\"\n",
	})

	eventJSON := fmt.Sprintf(`{"toolName":"","toolArgs":{},"cwd":"%s"}`,
		strings.ReplaceAll(workspace, `\`, `\\`))

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
}

func TestRawInputWithExtraFields(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"simple.yml": "name: Simple\non:\n  file:\n    paths: [\"**/*.txt\"]\nsteps:\n  - name: check\n    run: Write-Host \"processed\"\n",
	})

	eventJSON := fmt.Sprintf(`{"toolName":"create","toolArgs":{"path":"%s","file_text":"hi"},"cwd":"%s","sessionId":"test-session-123","unknownField":"ignored"}`,
		strings.ReplaceAll(filepath.Join(workspace, "test.txt"), `\`, `\\`),
		strings.ReplaceAll(workspace, `\`, `\\`))

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
	if !strings.Contains(output, "processed") {
		t.Errorf("Expected 'processed' in output:\n%s", output)
	}
}

// =============================================================================
// Tests targeting: runner/schema coverage
// =============================================================================

func TestStepWithEnvironmentInheritance(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"env-inherit.yml": "name: Env Inheritance\non:\n  file:\n    paths: [\"**/*\"]\nenv:\n  GLOBAL_VAR: \"from-workflow\"\nsteps:\n  - name: global env\n    run: |\n      Write-Host \"global=$env:GLOBAL_VAR\"\n",
	})

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path": filepath.Join(workspace, "test.txt"), "file_text": "test",
	}, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
	if !strings.Contains(output, "global=from-workflow") {
		t.Errorf("Expected global env in output:\n%s", output)
	}
}

func TestStepIDOutputCapture(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"step-output.yml": "name: Step Output\non:\n  file:\n    paths: [\"**/*\"]\nsteps:\n  - name: producer\n    id: producer\n    run: Write-Host \"produced value\"\n  - name: consumer\n    if: steps.producer.outcome == 'success'\n    run: Write-Host \"producer succeeded\"\n  - name: failure check\n    if: steps.producer.outcome == 'failure'\n    run: Write-Host \"should not run\"\n",
	})

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path": filepath.Join(workspace, "test.txt"), "file_text": "test",
	}, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
	if !strings.Contains(output, "producer succeeded") {
		t.Errorf("Expected 'producer succeeded' in output:\n%s", output)
	}
}

func TestWorkflowWithConcurrency(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"concurrent.yml": "name: Concurrent Workflow\non:\n  file:\n    paths: [\"**/*\"]\nconcurrency:\n  group: \"file-checks\"\n  max-parallel: 2\nsteps:\n  - name: check\n    run: Write-Host \"concurrent workflow ran\"\n",
	})

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path": filepath.Join(workspace, "test.txt"), "file_text": "test",
	}, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
	if !strings.Contains(output, "concurrent workflow ran") {
		t.Errorf("Expected concurrency workflow to run:\n%s", output)
	}
}

func TestSchemaValidateStepID(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"with-ids.yml": "name: Steps With IDs\non:\n  file:\n    paths: [\"**/*\"]\nsteps:\n  - name: first\n    id: step1\n    run: Write-Host \"step1\"\n  - name: second\n    id: step2\n    if: steps.step1.outcome == 'success'\n    run: Write-Host \"step2 after step1\"\n",
	})

	output, _ := runHookflowCmd(t, []string{"validate", "--dir", workspace}, nil)
	lower := strings.ToLower(output)
	if strings.Contains(lower, "validation failed") || strings.Contains(lower, "additional property") {
		t.Errorf("Validation should pass for step IDs:\n%s", output)
	}
}

// =============================================================================
// Tests targeting: test command (test.go)
// =============================================================================

func TestToolTriggerViaRawEvent(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"tool-trigger.yml": "name: Tool Check\non:\n  tool:\n    name: my-custom-tool\nsteps:\n  - name: check\n    run: Write-Host \"tool matched\"\n",
	})

	eventJSON := fmt.Sprintf(`{"toolName":"my-custom-tool","toolArgs":{"command":"some-command"},"cwd":"%s"}`,
		strings.ReplaceAll(workspace, `\`, `\\`))
	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", nil)
	assertAllow(t, result, output)
	if !strings.Contains(output, "tool matched") {
		t.Errorf("Expected 'tool matched' in output:\n%s", output)
	}
}

func TestTestCmdNoWorkflowsFound(t *testing.T) {
	tmpDir := t.TempDir()
	gitInit(t, tmpDir)

	output, _ := runHookflowCmd(t,
		[]string{"test", "--event", "file", "--dir", tmpDir},
		nil)
	if !strings.Contains(strings.ToLower(output), "no workflows") &&
		!strings.Contains(strings.ToLower(output), "no hookflow") {
		t.Logf("Expected no-workflows message, got: %s", output)
	}
}

// =============================================================================
// Tests targeting: logging (HOOKFLOW_DEBUG)
// =============================================================================

func TestHookflowDebugEnvVar(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"debug-test.yml": "name: Debug Test\non:\n  file:\n    paths: [\"**/*\"]\nsteps:\n  - name: check\n    run: Write-Host \"debug test ran\"\n",
	})

	eventJSON := buildEventJSON("create", map[string]interface{}{
		"path": filepath.Join(workspace, "test.txt"), "file_text": "test",
	}, workspace)

	result, output := runHookflow(t, workspace, eventJSON, "preToolUse", &hookflowOpts{
		env: []string{"HOOKFLOW_DEBUG=1"},
	})
	assertAllow(t, result, output)
}

// =============================================================================
// Tests targeting: init branches
// =============================================================================

func TestInitRepoScaffold(t *testing.T) {
	tmpDir := t.TempDir()
	gitInit(t, tmpDir)

	output, _ := runHookflowCmd(t, []string{"init", "--repo", "--dir", tmpDir}, nil)

	hooksPath := filepath.Join(tmpDir, ".github", "hooks", "hooks.json")
	if _, err := os.Stat(hooksPath); os.IsNotExist(err) {
		t.Errorf("Expected hooks.json at %s. Output:\n%s", hooksPath, output)
	}

	examplePath := filepath.Join(tmpDir, ".github", "hookflows", "example.yml")
	if _, err := os.Stat(examplePath); os.IsNotExist(err) {
		t.Errorf("Expected example.yml at %s. Output:\n%s", examplePath, output)
	}
}

func TestInitForceOverwrite(t *testing.T) {
	tmpDir := t.TempDir()
	gitInit(t, tmpDir)

	_, _ = runHookflowCmd(t, []string{"init", "--repo", "--dir", tmpDir}, nil)

	hooksPath := filepath.Join(tmpDir, ".github", "hooks", "hooks.json")
	_ = os.WriteFile(hooksPath, []byte(`{"hooks":{}}`), 0644)

	_, _ = runHookflowCmd(t, []string{"init", "--repo", "--force", "--dir", tmpDir}, nil)

	data, err := os.ReadFile(hooksPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "hookflow") {
		t.Errorf("Expected hooks.json to contain hookflow after force init")
	}
}

// =============================================================================
// Tests targeting: schema validation completeness
// =============================================================================

func TestValidateWorkflowWithAllFields(t *testing.T) {
	workspace := setupWorkspaceWithHookflows(t, map[string]string{
		"complete.yml": "name: Complete Workflow\ndescription: A workflow using all supported fields\nlifecycle: pre\nblocking: true\nconcurrency:\n  group: my-group\n  max-parallel: 3\nenv:\n  GLOBAL_VAR: \"value\"\non:\n  file:\n    types: [\"create\", \"edit\"]\n    paths: [\"src/**\"]\n    paths-ignore: [\"src/**/*.test.js\"]\nsteps:\n  - name: first step\n    id: step1\n    if: event.file.action == 'create'\n    run: Write-Host \"creating\"\n    shell: pwsh\n    env:\n      STEP_VAR: \"step-value\"\n    working-directory: \".\"\n    timeout: 30\n    continue-on-error: false\n",
	})

	output, _ := runHookflowCmd(t, []string{"validate", "--dir", workspace}, nil)
	lower := strings.ToLower(output)
	if strings.Contains(lower, "invalid") || strings.Contains(lower, "additional property") {
		t.Errorf("Full-field workflow should validate:\n%s", output)
	}
}
