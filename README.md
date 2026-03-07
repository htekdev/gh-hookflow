# gh-hookflow 🔒

A GitHub CLI extension that runs local workflows triggered by GitHub Copilot agent hooks — like GitHub Actions, but for your AI pair programming sessions. Enforce governance, quality gates, and safety checks in real-time.

## What Makes This Different

**GitHub Copilot CLI hooks can block tool calls *before* they happen — but they can't block *after*.** Post-hook output is ignored by the Copilot CLI. This means if you validate a file after creation and it fails, you have no way to tell the agent.

**gh-hookflow solves this.** It implements a **post-error feedback loop** that forces the agent to acknowledge and fix issues caught by post-lifecycle workflows:

1. A post-lifecycle workflow validates content after the agent creates/edits a file
2. If validation fails, hookflow writes an error file and **blocks all subsequent tool calls**
3. The deny message tells the agent to read the error file for details
4. The agent reads the error file (allowed through as a primitive exemption)
5. The error auto-clears, and the agent can retry with the correct approach

This turns post-lifecycle hooks into **blocking validators** — something the Copilot CLI hooks architecture doesn't natively support.

## Overview

`gh-hookflow` lets you run "shift-left" DevOps checks during AI agent editing sessions. Instead of waiting for CI to catch issues on pull requests, you can:

- **Block** dangerous edits in real-time (e.g., .env file modifications)
- **Validate** content after creation and force the agent to fix it
- **Lint** code as the agent writes it  
- **Enforce** commit message conventions
- **Run security scans** before code leaves the local machine
- **Guard git push** — all pushes go through governance workflows

## Prerequisites

- [GitHub CLI](https://cli.github.com/) (`gh`) installed and authenticated
- [PowerShell Core](https://github.com/PowerShell/PowerShell) (`pwsh`) installed (workflow steps run in pwsh for cross-platform consistency)

## Installation

```bash
gh extension install htekdev/gh-hookflow
```

This installs gh-hookflow as `gh hookflow` and integrates directly with Copilot CLI hooks.

## Quick Start

### 1. Initialize gh-hookflow in your repository

```bash
cd your-project
gh hookflow init
```

This creates:
- `.github/hookflows/` — Directory for your workflow files
- `.github/hooks/hooks.json` — Copilot CLI hook configuration
- `.github/hookflows/example.yml` — Example workflow to get started
- `.github/skills/hookflow/SKILL.md` — AI agent guidance for workflow creation

### 2. Create a workflow

Use AI to generate a workflow:

```bash
gh hookflow create "block edits to .env files"
```

Or manually create `.github/hookflows/block-env.yml`:

```yaml
name: Block .env Files
description: Prevent edits to environment files

on:
  file:
    paths:
      - '**/.env*'
      - '**/secrets/**'
    types:
      - edit
      - create

blocking: true

steps:
  - name: Deny sensitive file access
    run: |
      echo "❌ Cannot modify sensitive file: ${{ event.file.path }}"
      exit 1
```

### 3. Test your workflow

```bash
# Test with a mock file event
gh hookflow test --event file --action edit --path ".env"

# Test with a mock commit event  
gh hookflow test --event commit --path src/app.ts
```

### 4. Commit and share

```bash
git add .github/
git commit -m "Add gh-hookflow workflows"
git push
```

Team members can install with `gh extension install htekdev/gh-hookflow` to automatically run your workflows during their Copilot sessions.

## Commands

| Command | Description |
|---------|-------------|
| `gh hookflow init` | Initialize gh-hookflow for a repository |
| `gh hookflow create <prompt>` | Create a workflow using AI |
| `gh hookflow discover` | List workflows in the current directory |
| `gh hookflow validate` | Validate workflow YAML files |
| `gh hookflow test` | Test a workflow with a mock event |
| `gh hookflow run` | Run workflows (used by hooks internally) |
| `gh hookflow git-push` | Push with pre/post governance workflows |
| `gh hookflow git-push-status` | Check status of an async git push |
| `gh hookflow logs` | View gh-hookflow debug logs |
| `gh hookflow triggers` | List available trigger types |
| `gh hookflow version` | Show version information |

## How It Works

gh-hookflow integrates with [GitHub Copilot CLI hooks](https://docs.github.com/en/copilot/customizing-copilot/extending-copilot-in-vs-code/copilot-cli-hooks):

```
┌─────────────────────────────────────────────────────────────┐
│  Copilot Agent Session                                      │
│                                                             │
│  User: "Edit the .env file"                                 │
│                    │                                        │
│                    ▼                                        │
│  ┌──────────────────────────────────────────┐               │
│  │ preToolUse Hook                          │               │
│  │  └─> gh hookflow run --event-type pre    │               │
│  │       └─> Matches .github/hookflows/*.yml │               │
│  │       └─> Runs blocking workflow         │               │
│  │       └─> Returns: deny/allow            │               │
│  └──────────────────────────────────────────┘               │
│                    │                                        │
│         ┌─────────┴─────────┐                               │
│         │                   │                               │
│      DENIED              ALLOWED                            │
│         │                   │                               │
│    Agent stops         Tool executes                        │
│                             │                               │
│                             ▼                               │
│  ┌──────────────────────────────────────────┐               │
│  │ postToolUse Hook                         │               │
│  │  └─> gh hookflow run --event-type post   │               │
│  │       └─> Runs validation/linting        │               │
│  │       └─> If blocking step fails:        │               │
│  │            └─> Writes session error      │               │
│  │            └─> BLOCKS next tool call     │               │
│  │            └─> Agent reads error file    │               │
│  │            └─> Error auto-clears        │               │
│  └──────────────────────────────────────────┘               │
└─────────────────────────────────────────────────────────────┘
```

### Post-Error Feedback Loop

GitHub Copilot CLI hooks ignore postToolUse output — there's no native way to give the agent feedback after a tool runs. gh-hookflow works around this with a **session error file**:

1. **Post-lifecycle workflow fails** → hookflow writes `error.md` to the session directory
2. **Next preToolUse** → hookflow detects the error file and **denies** with: *"Read the error file at {path} to acknowledge it"*
3. **Agent reads the error file** → hookflow allows the `view` through (primitive exemption)
4. **postToolUse for the view** → hookflow **deletes** the error file
5. **Next tool call** → no error file exists, agent proceeds normally

The agent learns what went wrong and can fix it — turning a passive post-hook into an active feedback loop.

## Usage

```bash
# Initialize a repository (creates .github/hookflows/ and .github/hooks/hooks.json)
gh hookflow init

# Discover workflows in the current directory
gh hookflow discover

# Validate workflow files
gh hookflow validate

# Test a workflow with a mock commit event
gh hookflow test --event commit --path src/app.ts

# Test a workflow with a mock file event
gh hookflow test --event file --action edit --path src/app.ts

# View logs for debugging
gh hookflow logs
gh hookflow logs -f  # Follow mode (like tail -f)
```

## Workflow Syntax

Workflows are defined in `.github/hookflows/*.yml`:

```yaml
name: Block Sensitive Files
description: Prevent edits to sensitive files

on:
  file:
    lifecycle: pre     # Run BEFORE the action (can block)
    paths:
      - '**/*.env*'
      - '**/secrets/**'
    paths-ignore:
      - '**/*.md'
    types:
      - edit
      - create

blocking: true         # Exit 1 = deny the action

steps:
  - name: Deny edit
    run: |
      echo "❌ Cannot edit sensitive files"
      exit 1
```

### Lifecycle: Pre vs Post

- **`lifecycle: pre`** (default) — Runs BEFORE the tool executes. Can block/deny the operation.
- **`lifecycle: post`** — Runs AFTER the tool executes. For validation, linting, notifications.

Post-lifecycle workflows can be **blocking** — if a step fails, hookflow writes a session error that blocks all subsequent tool calls until the agent reads and acknowledges the error.

```yaml
# Post-edit validation — blocks agent until fixed
name: Validate API Spec
on:
  file:
    lifecycle: post
    paths: ['api-spec.json']
    types: [create, edit]

blocking: true

steps:
  - name: Validate schema
    run: |
      $spec = Get-Content "${{ event.file.path }}" | ConvertFrom-Json
      if (-not $spec.response_schema) {
        Write-Error "api-spec.json must include response_schema"
        exit 1
      }
```

```yaml
# Post-edit linting — non-blocking, just report
on:
  file:
    lifecycle: post
    paths: ['**/*.ts']
    types: [edit]

blocking: false

steps:
  - name: Lint TypeScript
    run: npx eslint "${{ event.file.path }}" --fix
```

## Trigger Types

| Trigger | Description | Example |
|---------|-------------|---------|
| `file` | File create/edit/delete events | Block `.env` edits |
| `tool` | Specific tool calls with arg patterns | Block `rm -rf` commands |
| `commit` | Git commit events | Require tests with source changes |
| `push` | Git push events | Require PR for main branch |
| `hooks` | Match by hook type | Run on all preToolUse |

## Expression Engine

Supports `${{ }}` expressions with GitHub Actions parity:

```yaml
steps:
  - name: Conditional step
    if: ${{ endsWith(event.file.path, '.ts') }}
    run: echo "TypeScript file: ${{ event.file.path }}"
```

### Available Context

| Expression | Description |
|------------|-------------|
| `event.file.path` | Path of file being edited |
| `event.file.action` | Action: edit, create, delete |
| `event.file.content` | File content (for create) |
| `event.tool.name` | Tool name being called |
| `event.tool.args.*` | Tool argument values |
| `event.commit.message` | Commit message |
| `event.commit.sha` | Commit SHA |
| `event.lifecycle` | Hook lifecycle: pre or post |
| `env.MY_VAR` | Environment variable |

### Built-in Functions

| Function | Description |
|----------|-------------|
| `contains(search, item)` | Check if string/array contains item |
| `startsWith(str, value)` | String starts with value |
| `endsWith(str, value)` | String ends with value |
| `format(str, ...args)` | String formatting |
| `join(array, sep)` | Join array to string |
| `toJSON(value)` | Convert to JSON string |
| `fromJSON(str)` | Parse JSON string |
| `always()` | Always true |
| `success()` | Previous steps succeeded |
| `failure()` | Previous step failed |
| `transcript()` | Full session transcript as JSON array |
| `transcript('regex')` | Transcript entries matching regex |
| `transcript_since('regex')` | Entries after last match of regex |
| `transcript_count('regex')` | Count of entries matching regex |
| `transcript_last('regex')` | Last entry matching regex |

## Common Patterns

### Block Sensitive Files

```yaml
name: Block Sensitive Files
on:
  file:
    paths: ['**/.env*', '**/secrets/**', '**/*.pem', '**/*.key']
    types: [edit, create]
blocking: true
steps:
  - name: Deny
    run: |
      echo "❌ Cannot modify: ${{ event.file.path }}"
      exit 1
```

### Require Tests with Source Changes

```yaml
name: Require Tests
on:
  commit:
    paths: ['src/**']
    paths-ignore: ['src/**/*.test.*']
blocking: true
steps:
  - name: Check for test files
    run: |
      if ! echo "${{ event.commit.files }}" | grep -q '\.test\.'; then
        echo "❌ Source changes require tests"
        exit 1
      fi
```

### Post-Edit Linting

```yaml
name: Lint on Save
on:
  file:
    lifecycle: post
    paths: ['**/*.ts', '**/*.tsx']
    types: [edit]
blocking: false
steps:
  - name: ESLint
    run: npx eslint "${{ event.file.path }}" --fix
```

### Session Transcript — Advisory Governance

Hookflow records every tool call it sees into a session transcript. Use `transcript_*()` functions to query what the agent has already done — enabling advisory governance (checking the agent *should have* done something) rather than only blocking governance.

**Check tests were run before commit:**

```yaml
name: Suggest Tests Before Commit
on:
  tool:
    name: powershell
blocking: true
steps:
  - name: check-tests
    if: transcript_count('go test|npm test|pytest|jest') == 0
    run: |
      Write-Output '{"permissionDecision":"deny","permissionDecisionReason":"No test execution found this session. Consider running tests before committing."}'
```

**Check for code review since last edit:**

```yaml
name: Review Before Commit
on:
  tool:
    name: powershell
blocking: true
steps:
  - name: check-review
    if: transcript_count('code-review|code_review') == 0
    run: |
      Write-Output '{"permissionDecision":"deny","permissionDecisionReason":"Consider running a code review before committing."}'
```

The transcript functions use regex matching across the entire serialized hook payload, so patterns like `go test` match even when the command is buried inside a `powershell` tool call's arguments. The transcript is capped at 1000 entries (configurable via `HOOKFLOW_TRANSCRIPT_MAX_ENTRIES`).

## Primitive Guards

Hookflow enforces critical safety checks **before** any workflow matching:

- **`git push` is blocked** — All pushes must go through `gh hookflow git-push`, which runs pre/post governance workflows
- **Multiple git commands in one tool call are denied** — Each git operation must be a separate tool call

These guards scan raw hook input regardless of tool name and cannot be bypassed.

### Git Push Governance

```bash
# Push with governance workflows
gh hookflow git-push origin main

# Check push status
gh hookflow git-push-status <activity_id>
```

The push runs in 3 phases: pre-push workflows → git push → post-push workflows (e.g., monitor CI checks on the PR).

## Debugging

Enable debug logging:

```bash
# Set environment variable
export HOOKFLOW_DEBUG=1

# View logs
gh hookflow logs
gh hookflow logs -n 100    # Last 100 lines
gh hookflow logs -f        # Follow mode
gh hookflow logs --path    # Print log file path
```

Logs are stored in `~/.hookflow/logs/` with 7-day retention.

## Development

```bash
# Build
go build -o bin/gh-hookflow ./cmd/hookflow

# Test
go test ./... -v

# Test with coverage
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

### E2E Testing

The project includes end-to-end tests that validate hookflow against real Copilot CLI integration across platforms (Ubuntu, macOS, Windows).

**15 test scenarios covering:**
- Workflow validation and discovery
- Sensitive file blocking (`.env`, `.key`, `.pem`, `.cert`)
- Normal file operations (allow by default)
- Post-lifecycle hooks (blocking and non-blocking)
- Git commit governance (conventional commit format)
- Content enforcement (e.g., block `console.log` in production code)
- Continue-on-error step behavior
- Paths-ignore filtering
- Multi-step pipelines with `failure()`/`always()` expressions
- Step timeout enforcement
- Primitive guards (git push block, multi-git deny)
- **Post-error feedback loop** — validates the full cycle: post-hook fails → agent is blocked → reads error → fixes the issue
- Copilot CLI integration (`copilot -p` programmatic mode, requires `COPILOT_GITHUB_TOKEN`)

**Running locally with `hookflow run --raw`:**
```bash
# Block test — should return permissionDecision: deny
echo '{"toolName":"create","toolArgs":{"path":".env","file_text":"SECRET=x"},"cwd":"'$(pwd)'"}' \
  | hookflow run --raw --event-type preToolUse

# Allow test — should return permissionDecision: allow
echo '{"toolName":"create","toolArgs":{"path":"hello.txt","file_text":"Hello"},"cwd":"'$(pwd)'"}' \
  | hookflow run --raw --event-type preToolUse
```

**CI setup:** The E2E workflow (`.github/workflows/e2e.yml`) requires a `COPILOT_GITHUB_TOKEN` repository secret (fine-grained PAT with Copilot Requests permission) for the Copilot CLI integration tests. The direct `hookflow run --raw` tests run without any secrets.

## Related Projects

- [GitHub Copilot CLI](https://github.com/github/gh-copilot) — The AI coding assistant this extends
- [Copilot Hooks Documentation](https://docs.github.com/en/copilot/customizing-copilot/extending-copilot-in-vs-code/copilot-cli-hooks) — Official hooks reference

## License

MIT
