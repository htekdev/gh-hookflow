# gh-hookflow

A GitHub CLI extension that runs local workflows triggered by GitHub Copilot agent hooks — like GitHub Actions, but for your AI pair programming sessions.

## Installation

```bash
npm install -g hookflow
```

Or install as a GitHub CLI extension (recommended):

```bash
gh extension install htekdev/gh-hookflow
```

Other options:

```bash
# Go
go install github.com/htekdev/gh-hookflow/cmd/hookflow@latest

# Direct download
curl -sSL https://raw.githubusercontent.com/htekdev/gh-hookflow/main/scripts/install.sh | sh
```

## Quick Start

```bash
# One-time global setup — enables hookflow for all repos
hookflow register

# Add workflows to a specific repo
cd your-project
hookflow init --repo

# Create a workflow using AI
hookflow create "block edits to .env files"

# Test a workflow with a mock event
hookflow test --event file --action edit --path ".env"

# List workflows
hookflow discover
```

## What is gh-hookflow?

gh-hookflow lets you run "shift-left" DevOps checks during AI agent editing sessions. Instead of waiting for CI to catch issues on pull requests, you can:

- **Block** dangerous edits in real-time (e.g., .env file modifications)
- **Lint** code as the agent writes it
- **Validate** configurations before commit
- **Run security scans** before code leaves the local machine

## Commands

| Command | Description |
|---------|-------------|
| `register` | Register personal hooks + skill (global, all repos) |
| `register --unregister` | Remove personal hooks and skill |
| `init` | Initialize per-repo hooks |
| `init --repo` | Also scaffold example workflows |
| `create <prompt>` | Create a workflow using AI |
| `discover` | List workflows in the current repository |
| `validate` | Validate workflow YAML files |
| `test` | Test a workflow with a mock event |
| `run` | Run workflows (used by hooks) |
| `git-push` | Push with governance workflows |
| `logs` | View debug logs |
| `version` | Show version information |

## Workflow Syntax

Workflows use a GitHub Actions-like syntax:

```yaml
name: Block .env edits
description: Prevent modifications to environment files

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
  - name: Block sensitive file edit
    run: |
      echo "❌ Cannot modify sensitive files"
      exit 1
```

## Learn More

- [GitHub Repository](https://github.com/htekdev/gh-hookflow)
- [Workflow Schema Documentation](https://github.com/htekdev/gh-hookflow/blob/main/schema/workflow.schema.json)

## License

MIT
