# Hookflow CLI - Copilot Instructions

This repository contains the `hookflow` CLI - a local workflow engine for agentic DevOps.

## Learning & Memory

**Always store memories** when you discover something important about this codebase:
- CI/CD quirks (e.g., golangci-lint requires explicit error handling with `_ =`)
- Cross-platform gotchas (e.g., path separators, `filepath.Match` behavior differs)
- Test patterns that work or don't work
- Build/release process requirements
- Any "lessons learned" from debugging sessions

Use `store_memory` proactively so future sessions don't repeat the same mistakes.

## Testing Standards

**Tests must be extensive.** This is non-negotiable:
- Every code change must have thorough tests covering happy path, edge cases, and error conditions
- Never write a minimal test that only checks the "works" case — test boundaries, invalid inputs, platform differences, and failure modes
- Unit tests must cover all branches and conditions, not just the primary flow
- E2E tests must pass on **all three platforms** (ubuntu, macos, windows) before a PR is approved — no exceptions, no dismissing failures as "pre-existing"
- If a test fails after your change, **you caused it** until you prove otherwise with evidence. Do not blame pre-existing issues without investigation.
- When adding new functions, add tests immediately — not as a follow-up

## Saving Memories

**Always use `store_memory`** when you encounter:
- Bugs and their root causes (so future sessions don't repeat them)
- Cross-platform gotchas (path separators, OS-specific behavior, MSYS paths, etc.)
- CI/CD quirks and requirements
- Patterns that work or don't work in this codebase
- Any "lesson learned" from a debugging session

**Never store incorrect or lazy memories.** If you don't know the root cause, investigate first. A wrong memory is worse than no memory.

## DO NOT DO

These are bad practices that have been caught in this codebase. **Never repeat them:**

1. **DO NOT dismiss CI failures as "pre-existing" without investigation.** If a test was passing before your change and fails after, your change caused it. Investigate the actual root cause.
2. **DO NOT store memories with incorrect root cause analysis.** Verify your diagnosis before committing it to memory. Wrong memories mislead future sessions.
3. **DO NOT write minimal tests.** Tests must be extensive and cover edge cases, not just prove "it compiles."
4. **DO NOT leave platform-specific bugs unresolved.** Windows, macOS, and Linux must all pass. Cross-platform issues (like MSYS path conversion) are real bugs, not flakiness.
5. **DO NOT use `TODO`, `FIXME`, stubs, or placeholder implementations.** Everything must be fully implemented.
6. **DO NOT swallow errors silently.** Every error return must be explicitly handled or ignored with `_ =` (required by golangci-lint).

## Architecture

```
hookflow/
├── cmd/hookflow/         # CLI entry point and commands (main.go has primitive guards + session error gate)
├── internal/
│   ├── ai/               # Copilot AI integration for workflow generation
│   ├── concurrency/      # Semaphore for parallel control
│   ├── discover/         # Workflow file discovery
│   ├── event/            # Event detection from hook input
│   ├── expression/       # ${{ }} expression engine
│   ├── logging/          # Production logging service
│   ├── mcp/              # MCP server infrastructure
│   ├── runner/           # Step execution (records post-lifecycle errors via session.WriteError)
│   ├── schema/           # Workflow types and validation
│   ├── session/          # Session error persistence (error.md read/write/clear)
│   └── trigger/          # Event-to-trigger matching
├── packages/
│   └── npm-wrapper/      # npm package for CLI distribution
└── testdata/
    └── e2e/hookflows/    # E2E test workflow files (copied to e2e-workspace/.github/hookflows/)
```

## Primitive Guards (Critical Safety)

The hookflow runtime enforces these checks **before** any event detection or workflow matching. They are non-negotiable and cannot be bypassed.

1. **`git push` is automatically blocked.** All pushes must go through `gh hookflow git-push`, which runs governance workflows before pushing. Never use `git push` directly.
2. **Multiple git commands in a single tool call are denied immediately.** Do not chain git commands with `&&`, `;`, `|`, or newlines. Each git operation must be a separate tool call so hookflow can evaluate each one individually.

These guards scan raw hook input regardless of tool name, so they work even if the Copilot CLI tool name changes.

### Primitive Exemptions

These tool calls bypass primitive guards and session error checks:
- `view` of the session error file — allowed so the agent can read error details (the file is auto-deleted in postToolUse)

## Session Error / denyNextToolCall Flow

The session error mechanism is the primary way hookflow provides **post-lifecycle feedback** to agents. Copilot CLI postToolUse hooks cannot inject feedback directly (output is ignored), so hookflow uses a file-based error propagation pattern.

### Flow

1. **Post-lifecycle workflow fails** → `internal/runner/runner.go` calls `session.WriteError(workflowName, stepName, details)` → writes `~/.hookflow/sessions/{sessionId}/error.md`
2. **Next preToolUse arrives** → global gate in `cmd/hookflow/main.go` calls `session.HasError()` → if true, immediately denies with the error file path: _"You must read the error file to acknowledge it before continuing."_
3. **Agent calls `view` on the error file** → primitive exemption allows it through (bypasses session error gate)
4. **postToolUse fires for the view** → hookflow detects the agent read the error file → deletes `error.md`
5. **Next preToolUse** → `session.HasError()` returns false → proceeds normally

### Session Identity

Copilot CLI includes `sessionId` (UUID) in every hook event payload. Hookflow uses this to set `HOOKFLOW_SESSION_DIR` to `~/.hookflow/sessions/{sessionId}/`, ensuring all processes agree on the session directory. If `HOOKFLOW_SESSION_DIR` is already set (e.g., in CI tests), the env var takes priority.

### Key Design Details

- **Global gate**: The session error check runs in `cmd/hookflow/main.go` BEFORE event detection or workflow matching. It fires even when no workflow matches.
- **Error file read exemption**: `view` of the error file is always allowed through (checked before session error gate) to prevent deadlock.
- **Error format**: Markdown with workflow name, step name, timestamp, and captured step output/error.
- **Session dir**: Resolved from `sessionId` in hook input → `~/.hookflow/sessions/{sessionId}/`. Override with `HOOKFLOW_SESSION_DIR` env var (used in CI tests). Falls back to PID-based detection if neither is available.
- **Error file**: `error.md` inside the session dir. `session.HasError()` checks existence, `session.ReadAndClearError()` reads + deletes atomically.

### Code Locations

| Concern | File | Key Function/Section |
|---|---|---|
| Write error on post failure | `internal/runner/runner.go` | `recordPostToolUseError()` |
| Error file CRUD | `internal/session/errors.go` | `WriteError()`, `HasError()`, `ReadAndClearError()` |
| Session dir from sessionId | `cmd/hookflow/main.go` | Session identity section (~line 389) |
| Error file read exemption | `cmd/hookflow/main.go` | `isSessionErrorFileRead()` |
| Global pre-lifecycle gate | `cmd/hookflow/main.go` | `runMatchingWorkflowsWithEvent()` |

## Event & File Path Resolution

When Copilot CLI sends hook input like `{"toolName":"create","toolArgs":{"path":"app.config.json"},"cwd":"/workspace"}`:

1. **Event detection** (`internal/event/`): Parses toolName/toolArgs to determine event type (file create, edit, git commit, etc.) and extracts the file path.
2. **Path normalization** (`cmd/hookflow/main.go:normalizeFilePath`): Converts backslashes to forward slashes, strips the cwd prefix to make it relative. E.g., `/workspace/app.config.json` → `app.config.json`.
3. **In workflow steps**: `${{ event.file.path }}` is the **relative** normalized path. `${{ event.cwd }}` is the absolute cwd from the hook input.
4. **Step execution cwd**: The runner sets `cmd.Dir` to the repository root (passed as `dir` to `NewRunner`). Steps can override with `working-directory:`.
5. **File content access**: `${{ event.file.content }}` contains `file_text` (for create) or `new_str` (for edit) from toolArgs.

### Expression Context (`internal/runner/runner.go:NewRunner`)

```
event.cwd          → hook input cwd (absolute)
event.type         → "file", "git_commit", "git_push", etc.
event.file.path    → normalized relative path
event.file.action  → "create" or "edit"
event.file.content → file content from toolArgs
event.git.*        → git-specific fields (message, branch, etc.)
```

## Shell Standard

**All workflow `run:` steps use PowerShell Core (pwsh)** for cross-platform consistency.
- Same syntax works on Windows, macOS, and Linux
- Users must have `pwsh` installed
- Default shell is always `pwsh`, not OS-dependent
- Helpful error message shown if pwsh is not found

## Development Workflow

1. Make changes to Go code
2. Run tests: `go test ./...`
3. Build: `go build -o bin/hookflow ./cmd/hookflow`
4. Install locally: `go install ./cmd/hookflow`

## Key Components

### Event Detection (`internal/event/`)
- Parses raw Copilot hook input (toolName, toolArgs, cwd)
- Detects event type: file edit, git commit, git push, etc.
- Normalizes paths for cross-platform matching

### Trigger Matching (`internal/trigger/`)
- Matches events against workflow triggers
- Glob patterns with `**` for recursive matching
- Negation with `!` prefix
- **Note**: `filepath.Match` behavior differs by OS

### Expression Engine (`internal/expression/`)
- `${{ }}` syntax with GitHub Actions parity
- Context: `event`, `env`, `steps`
- Functions: `contains()`, `startsWith()`, `endsWith()`, etc.

### Production Logging (`internal/logging/`)
- Logs to `~/.hookflow/logs/hookflow-YYYY-MM-DD.log`
- Enable debug: `HOOKFLOW_DEBUG=1`
- 7-day retention with automatic cleanup
- View with: `hookflow logs`

### Git Push (`internal/push/`)
- Core 3-phase orchestration: pre-push workflows → git push → post-push workflows
- Shared by both the CLI command (`hookflow git-push`) and MCP tools
- Updates activity state on disk as each phase progresses

## MCP Server (`internal/mcp/`)

The hookflow MCP server (`gh hookflow mcp serve`) provides infrastructure for future tools. Session errors are now handled via file-based read (the agent reads `error.md` directly using `view`, no MCP tool needed).

## Git Push CLI Commands

Git push uses CLI commands (not MCP) because MCP server processes don't inherit the full terminal PATH:

| Command | Description |
|---|---|
| `gh hookflow git-push [args...]` | Run 3-phase push: pre-push workflows → git push → post-push workflows. Prints activity ID immediately. |
| `gh hookflow git-push-status <id>` | Check progress of a push by activity ID. |

### Git Push Flow

1. Agent runs `gh hookflow git-push origin main` in a shell
2. Command prints `{ activity_id, status: "running", next_step }` immediately
3. Push runs synchronously in the background (pre-push → push → post-push)
4. Agent runs `gh hookflow git-push-status <activity_id>` to check progress
5. When done: response includes full `pre_push`, `push`, `post_push` results

## Testing

```bash
go test ./... -v -timeout 300s    # All tests (use 300s to avoid runner timeouts)
go test ./internal/trigger/... -v  # Specific package
go test ./... -coverprofile=coverage.out  # With coverage
```

## E2E Testing (`.github/workflows/e2e.yml`)

E2E tests run on ubuntu, macos, and windows. They use two modes:

- **Direct tests (deterministic)**: Pipe JSON to `hookflow run --raw --event-type preToolUse|postToolUse` and assert on the JSON output. These don't need Copilot CLI and always produce consistent results.
- **Copilot CLI tests (conditional)**: Require `COPILOT_GITHUB_TOKEN` secret. Run `copilot -p "..." --yolo` and check file system results. Non-deterministic by nature.

### Test Workspace Setup

1. Builds hookflow binary and installs as `gh` extension
2. Creates `e2e-workspace/` with `.github/hookflows/` from `testdata/e2e/hookflows/`
3. Initializes git repo + runs `gh hookflow init` (creates `.github/hooks/hooks.json` + `~/.copilot/mcp-config.json`)

### E2E Workflow Files (`testdata/e2e/hookflows/`)

| File | Purpose | Lifecycle |
|---|---|---|
| `block-sensitive-files.yml` | Blocks .env, .key, .pem, .cert files | pre |
| `content-enforcement.yml` | Blocks JS/TS files with console.log | pre |
| `require-commit-message.yml` | Enforces conventional commit format | pre |
| `post-config-validation.yml` | Validates config file content after create/edit | post (blocking) |
| `post-api-spec-validation.yml` | Validates api-spec.json has endpoint, method, response_schema | post (blocking) |
| `post-edit-notify.yml` | Non-blocking post-edit notification | post |
| `continue-on-error-linter.yml` | Tests continue-on-error step behavior | pre |
| `paths-ignore-tests.yml` | Tests paths-ignore filtering for test files | pre |
| `multi-step-pipeline.yml` | Tests failure()/always() expressions | pre |
| `step-timeout.yml` | Tests step timeout enforcement | pre |

### CI Patterns

- `hookflow run --raw` returns non-zero on deny; use `|| true` in CI scripts with `set -e`
- JSON output is pretty-printed; use `grep -qE` with `\s*` patterns
- Session errors from PID-based dirs can persist between test steps; clean with `find ~/.hookflow -name "error.md" -delete` before Copilot CLI tests
- `HOOKFLOW_SESSION_DIR` env var overrides session dir for test isolation

## CI Requirements

- **golangci-lint with errcheck**: All error returns must be explicitly handled or ignored with `_ =`
- **Cross-platform tests**: Use `runtime.GOOS` checks or skip flags for OS-specific tests
- **Path separators**: Always use forward slashes in test expectations

## Release Process

Releases are automated via `.github/workflows/auto-release.yml`:
1. Push to main triggers version calculation from conventional commits
2. Tests run, binaries built for all platforms
3. GitHub Release created with binaries
4. npm package published (with continue-on-error for OIDC issues)

## Cross-Compilation

```bash
GOOS=windows GOARCH=amd64 go build -o bin/hookflow-windows-amd64.exe ./cmd/hookflow
GOOS=darwin GOARCH=arm64 go build -o bin/hookflow-darwin-arm64 ./cmd/hookflow
GOOS=linux GOARCH=amd64 go build -o bin/hookflow-linux-amd64 ./cmd/hookflow
```
