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

## Architecture

```
hookflow/
├── cmd/hookflow/         # CLI entry point and commands
├── internal/
│   ├── ai/               # Copilot AI integration for workflow generation
│   ├── concurrency/      # Semaphore for parallel control
│   ├── discover/         # Workflow file discovery
│   ├── event/            # Event detection from hook input
│   ├── expression/       # ${{ }} expression engine
│   ├── logging/          # Production logging service
│   ├── runner/           # Step execution
│   ├── schema/           # Workflow types and validation
│   └── trigger/          # Event-to-trigger matching
└── packages/
    └── npm-wrapper/      # npm package for CLI distribution
```

## Primitive Guards (Critical Safety)

The hookflow runtime enforces these checks **before** any event detection or workflow matching. They are non-negotiable and cannot be bypassed.

1. **`git push` is automatically blocked.** All pushes must go through `gh hookflow git-push` (or `hookflow git-push`), which runs governance workflows before pushing. Never use `git push` directly.
2. **Multiple git commands in a single tool call are denied immediately.** Do not chain git commands with `&&`, `;`, `|`, or newlines. Each git operation must be a separate tool call so hookflow can evaluate each one individually.

These guards scan raw hook input regardless of tool name, so they work even if the Copilot CLI tool name changes.

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

## Testing

```bash
go test ./... -v              # All tests
go test ./internal/trigger/... -v  # Specific package
go test ./... -coverprofile=coverage.out  # With coverage
```

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
