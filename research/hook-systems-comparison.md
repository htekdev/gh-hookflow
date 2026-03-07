# Hook Systems Comparison: GitHub Copilot CLI vs VSCode Agent Hooks vs Claude Code

> **Purpose:** Analysis for hookflow multi-platform support design  
> **Last Updated:** March 2026

## Executive Summary

This document compares hook systems across three AI coding assistants to inform hookflow's strategy for universal hook support. All three systems share a common architecture (JSON input/output, shell scripts, lifecycle events) but differ in event types, configuration format, and flow control mechanisms.

**Key Finding:** A unified abstraction layer is feasible. All three use stdin/stdout JSON for hook communication and support PreToolUse/PostToolUse as core events.

---

## Quick Comparison Matrix

| Feature | GitHub Copilot CLI | VSCode Agent Hooks | Claude Code |
|---------|-------------------|-------------------|-------------|
| **Configuration Format** | JSON (`.github/hooks/*.json`) | JSON (`.github/hooks/*.json`, `.claude/`) | JSON (`~/.claude/settings.json`) |
| **Hook Events** | 6 | 8 | 15+ |
| **Can Block Operations** | ✅ PreToolUse only | ✅ PreToolUse, Stop, SubagentStop | ✅ PreToolUse, UserPromptSubmit, TaskCompleted |
| **Input Protocol** | JSON via stdin | JSON via stdin | JSON via stdin |
| **Output Protocol** | JSON via stdout | JSON via stdout | JSON via stdout + exit codes |
| **Permission Decisions** | `allow`, `deny`, `ask` | `allow`, `deny`, `ask` | `allow`, `deny` |
| **Hook Types** | `command` only | `command` only | `command`, `http`, `prompt` |
| **Tool Matchers** | None (manual filter) | None (hooks apply to all) | Regex patterns |
| **Async Hooks** | ❌ | ❌ | ✅ Native `async: true` |
| **Cross-Platform** | `bash`/`powershell` | `command`/`windows`/`linux`/`osx` | `command` only |
| **Timeout** | `timeoutSec` (seconds) | `timeout` (seconds) | `timeout` (milliseconds) |
| **Session Transcript** | ❌ Not in payload | ✅ `transcript_path` (JSONL) | ✅ `transcript_path` (JSONL) |

---

## Hook Event Comparison

### Core Events (Supported by All Three)

| Event | Copilot CLI | VSCode | Claude Code | Notes |
|-------|-------------|--------|-------------|-------|
| **Session Start** | `sessionStart` | `SessionStart` | `SessionStart` | All support |
| **Pre-Tool Use** | `preToolUse` | `PreToolUse` | `PreToolUse` | **Primary blocking point** |
| **Post-Tool Use** | `postToolUse` | `PostToolUse` | `PostToolUse` | All support |
| **Session End** | `sessionEnd` | `Stop` | `SessionEnd`/`Stop` | Naming varies |

### Extended Events

| Event | Copilot CLI | VSCode | Claude Code |
|-------|-------------|--------|-------------|
| User Prompt Submit | `userPromptSubmitted` | `UserPromptSubmit` | `UserPromptSubmit` |
| Error Occurred | `errorOccurred` | ❌ | ❌ |
| Pre-Compact | ❌ | `PreCompact` | `PreCompact` |
| Subagent Start | ❌ | `SubagentStart` | `SubagentStart` |
| Subagent Stop | ❌ | `SubagentStop` | `SubagentStop` |
| Permission Request | ❌ | ❌ | `PermissionRequest` |
| Notification | ❌ | ❌ | `Notification` |
| Post-Tool Failure | ❌ | ❌ | `PostToolUseFailure` |
| Task Completed | ❌ | ❌ | `TaskCompleted` |
| Config Change | ❌ | ❌ | `ConfigChange` |
| Worktree Events | ❌ | ❌ | `WorktreeCreate`, `WorktreeRemove` |
| Teammate Idle | ❌ | ❌ | `TeammateIdle` |

### Event Naming Conventions

| Style | Example | Used By |
|-------|---------|---------|
| lowerCamelCase | `preToolUse` | Copilot CLI |
| PascalCase | `PreToolUse` | VSCode, Claude Code |

**Recommendation:** hookflow should accept both styles and normalize internally.

---

## Input/Output Protocol Comparison

### Input JSON Structure

All three systems pass JSON to stdin with similar structure:

```json
// Common fields across all platforms
{
  "timestamp": "...",      // All three
  "cwd": "/path/to/dir",   // All three
  "toolName": "...",       // Copilot CLI: toolName
  "tool_name": "...",      // VSCode/Claude: tool_name
  "toolArgs": "{}",        // Copilot CLI: JSON string
  "tool_input": {}         // VSCode/Claude: parsed object
}
```

### Key Input Differences

| Field | Copilot CLI | VSCode | Claude Code |
|-------|-------------|--------|-------------|
| Tool name | `toolName` | `tool_name` | `tool_name` |
| Tool args | `toolArgs` (JSON string) | `tool_input` (object) | `tool_input` (object) |
| Session ID | ❌ | `sessionId` | `session_id` |
| Transcript | ❌ | `transcript_path` | ❌ |
| Tool use ID | ❌ | `tool_use_id` | `tool_use_id` |

**hookflow Mapping Strategy:**
```yaml
# Internal hookflow event context should normalize:
event.tool.name    # From toolName OR tool_name
event.tool.args    # Parsed from toolArgs OR tool_input
event.tool.id      # From tool_use_id (if available)
event.session.id   # From sessionId OR session_id
```

### Output JSON Structure

All three support similar permission output:

```json
{
  "permissionDecision": "deny",
  "permissionDecisionReason": "Blocked by policy"
}
```

| Output Field | Copilot CLI | VSCode | Claude Code |
|--------------|-------------|--------|-------------|
| Permission decision | `permissionDecision` | `hookSpecificOutput.permissionDecision` | `hookSpecificOutput.permissionDecision` |
| Decision reason | `permissionDecisionReason` | `hookSpecificOutput.permissionDecisionReason` | `hookSpecificOutput.permissionDecisionReason` |
| Updated input | ❌ | `hookSpecificOutput.updatedInput` | `hookSpecificOutput.updatedInput` |
| Context injection | ❌ | `hookSpecificOutput.additionalContext` | `hookSpecificOutput.additionalContext` |
| System message | ❌ | `systemMessage` | `systemMessage` |
| Continue flag | ❌ | `continue` | ❌ |

**hookflow Output Strategy:**

hookflow should generate output in the correct format based on detected platform:

```go
type HookOutput struct {
    // For Copilot CLI
    PermissionDecision       string `json:"permissionDecision,omitempty"`
    PermissionDecisionReason string `json:"permissionDecisionReason,omitempty"`
    
    // For VSCode/Claude
    HookSpecificOutput *HookSpecificOutput `json:"hookSpecificOutput,omitempty"`
}

type HookSpecificOutput struct {
    HookEventName            string `json:"hookEventName"`
    PermissionDecision       string `json:"permissionDecision,omitempty"`
    PermissionDecisionReason string `json:"permissionDecisionReason,omitempty"`
    UpdatedInput             any    `json:"updatedInput,omitempty"`
    AdditionalContext        string `json:"additionalContext,omitempty"`
}
```

---

## Configuration Format Comparison

### Copilot CLI

```json
{
  "version": 1,
  "hooks": {
    "preToolUse": [
      {
        "type": "command",
        "bash": "./scripts/hook.sh",
        "powershell": "./scripts/hook.ps1",
        "cwd": ".github/hooks",
        "timeoutSec": 30,
        "env": { "LOG_LEVEL": "INFO" }
      }
    ]
  }
}
```

### VSCode Agent Hooks

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "type": "command",
        "command": "./scripts/hook.sh",
        "windows": "powershell -File scripts\\hook.ps1",
        "linux": "./scripts/hook.sh",
        "osx": "./scripts/hook.sh",
        "cwd": ".github/hooks",
        "timeout": 30,
        "env": { "LOG_LEVEL": "INFO" }
      }
    ]
  }
}
```

### Claude Code

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Edit|Write|Bash",
        "hooks": [
          {
            "type": "command",
            "command": "./.claude/hooks/validate.sh",
            "timeout": 30000,
            "async": false
          }
        ]
      }
    ]
  }
}
```

### Configuration Differences Summary

| Aspect | Copilot CLI | VSCode | Claude Code |
|--------|-------------|--------|-------------|
| **Event key case** | lowerCamelCase | PascalCase | PascalCase |
| **Version field** | `"version": 1` | None | None |
| **Command fields** | `bash`/`powershell` | `command`/`windows`/`linux`/`osx` | `command` only |
| **Timeout unit** | Seconds | Seconds | Milliseconds |
| **Timeout field** | `timeoutSec` | `timeout` | `timeout` |
| **Tool filtering** | None | None | `matcher` (regex) |
| **Async support** | No | No | Yes (`async: true`) |
| **Hook types** | `command` | `command` | `command`, `http`, `prompt` |

---

## Flow Control Comparison

### Exit Code Semantics

| Exit Code | Copilot CLI | VSCode | Claude Code |
|-----------|-------------|--------|-------------|
| `0` | Allow (success) | Allow (parse stdout) | Allow (success) |
| `1` | Allow (error, continue) | Warning (continue) | Error (may block) |
| `2` | Allow (error) | **Block** operation | **Block** operation |
| Non-zero | Allow (error) | Warning | Error |

**Key Insight:** VSCode and Claude Code use exit code `2` for blocking, while Copilot CLI only blocks via JSON output.

### Permission Decision Priority

When multiple hooks run for the same event:

| Platform | Priority Rule |
|----------|---------------|
| Copilot CLI | First deny wins |
| VSCode | Most restrictive wins (`deny` > `ask` > `allow`) |
| Claude Code | Most restrictive wins |

---

## Tool Names / Tool Matching

### Common Tool Names

| Action | Copilot CLI | VSCode | Claude Code |
|--------|-------------|--------|-------------|
| Edit file | `edit` | `editFiles` | `Edit` |
| Create file | `create` | `createFile` | `Write` |
| Shell command | `bash`/`powershell` | `runTerminalCommand` | `Bash` |
| View file | `view` | ❓ | `Read` |
| Delete file | ❌ | `deleteFile` | ❓ |
| Git push | ❌ | `pushToGitHub` | ❌ |

### Tool Filtering Capability

| Platform | Native Filter | hookflow Solution |
|----------|---------------|-------------------|
| Copilot CLI | None (check in script) | `on.tool.name` trigger |
| VSCode | None (hooks apply to all) | `on.tool.name` trigger |
| Claude Code | `matcher` regex | Use native + `on.tool.name` |

**hookflow Advantage:** Provides consistent `on.tool` and `on.file` triggers that work across all platforms.

---

## Feature Gap Analysis

### Features hookflow Should Add

| Feature | Available In | hookflow Priority |
|---------|--------------|-------------------|
| Tool matchers | Claude Code | **High** - already have `on.tool` |
| Async hooks | Claude Code | **Medium** - via `&` background |
| Updated input | VSCode, Claude | **High** - input modification |
| Context injection | VSCode, Claude | **High** - `additionalContext` |
| HTTP hooks | Claude Code | **Low** - can call curl in steps |
| LLM prompt hooks | Claude Code | **Low** - complex, defer |

### hookflow Current Advantages

| Feature | hookflow | Native Hooks |
|---------|----------|--------------|
| GitHub Actions syntax | ✅ YAML workflows | ❌ JSON + scripts |
| Expression engine | ✅ `${{ }}` | ❌ Manual in scripts |
| Multi-step workflows | ✅ Built-in | ❌ Single command |
| Concurrency control | ✅ `concurrency.group` | ❌ Manual |
| File pattern triggers | ✅ `on.file.paths` | ❌ Manual glob in script |
| Git event triggers | ✅ `on.commit`, `on.push` | ❌ Not available |

---

## Recommended hookflow Architecture

### Platform Detection

```go
type Platform string

const (
    PlatformCopilotCLI Platform = "copilot-cli"
    PlatformVSCode     Platform = "vscode"
    PlatformClaudeCode Platform = "claude-code"
    PlatformUnknown    Platform = "unknown"
)

func DetectPlatform(input map[string]any) Platform {
    // Check for platform-specific fields
    if _, ok := input["toolName"]; ok {
        return PlatformCopilotCLI
    }
    if _, ok := input["tool_name"]; ok {
        // Further distinguish VSCode vs Claude
        if _, ok := input["transcript_path"]; ok {
            return PlatformVSCode
        }
        return PlatformClaudeCode
    }
    return PlatformUnknown
}
```

### Unified Event Context

```go
type UnifiedEvent struct {
    Platform  Platform
    Timestamp time.Time
    Cwd       string
    
    // Normalized tool info
    Tool struct {
        Name string
        Args map[string]any
        ID   string
    }
    
    // Session info
    Session struct {
        ID     string
        Source string  // "new", "resume"
    }
    
    // Raw platform-specific data
    Raw map[string]any
}
```

### Output Format Generation

```go
func (h *HookRunner) GenerateOutput(platform Platform, decision string, reason string) []byte {
    switch platform {
    case PlatformCopilotCLI:
        return json.Marshal(map[string]string{
            "permissionDecision":       decision,
            "permissionDecisionReason": reason,
        })
    case PlatformVSCode, PlatformClaudeCode:
        return json.Marshal(map[string]any{
            "hookSpecificOutput": map[string]string{
                "hookEventName":            "PreToolUse",
                "permissionDecision":       decision,
                "permissionDecisionReason": reason,
            },
        })
    }
}
```

---

## Configuration File Locations

### Where Each Platform Looks

| Platform | Primary Location | Additional Locations |
|----------|-----------------|---------------------|
| Copilot CLI | `.github/hooks/*.json` | - |
| VSCode | `.github/hooks/*.json` | `.claude/settings.json`, `~/.claude/settings.json` |
| Claude Code | `.claude/settings.json` | `~/.claude/settings.json` |

### hookflow Strategy

1. **Default:** Use `.github/hooks/` for cross-platform compatibility
2. **Workflows:** Store in `.github/hooks/workflows/` (YAML)
3. **Shim scripts:** Generate platform-specific shims that call hookflow

---

## Exit Code Strategy

### Recommended hookflow Exit Code Behavior

| Condition | Exit Code | Reason |
|-----------|-----------|--------|
| Workflow succeeds, allow | `0` | Universal success |
| Workflow succeeds, deny | `0` + JSON | Copilot CLI requires JSON for deny |
| Workflow fails, blocking=true | `2` | VSCode/Claude respect exit 2 |
| Workflow fails, blocking=false | `0` | Non-blocking, continue |
| hookflow error | `1` | Warning, don't block |

### Platform-Specific Output

```go
func (h *HookRunner) Exit(platform Platform, result WorkflowResult) {
    if result.Decision == "deny" {
        // Always output JSON for deny
        fmt.Println(h.GenerateOutput(platform, "deny", result.Reason))
        
        // Exit 2 for VSCode/Claude for extra safety
        if platform != PlatformCopilotCLI {
            os.Exit(2)
        }
    }
    os.Exit(0)
}
```

---

## Implementation Roadmap

### Phase 1: Core Compatibility (Current)
- ✅ Support Copilot CLI hooks
- ✅ JSON input parsing
- ✅ Permission decision output

### Phase 2: Multi-Platform Input
- [ ] Add VSCode input format parsing (`tool_name`, `tool_input`)
- [ ] Add Claude Code input format parsing
- [ ] Implement platform detection
- [ ] Normalize event context

### Phase 3: Multi-Platform Output
- [ ] Generate platform-specific output format
- [ ] Implement exit code strategy
- [ ] Support `additionalContext` injection
- [ ] Support `updatedInput` for input modification

### Phase 4: Advanced Features
- [ ] Claude Code `matcher` support in config
- [ ] Async workflow execution
- [ ] HTTP hook type (call external APIs)
- [ ] Extended event types (SubagentStart, etc.)

---

## Testing Strategy

### Test Matrix

| Test | Copilot CLI | VSCode | Claude Code |
|------|-------------|--------|-------------|
| Input parsing | ✅ | ⬜ | ⬜ |
| Tool name extraction | ✅ | ⬜ | ⬜ |
| File path extraction | ✅ | ⬜ | ⬜ |
| Deny output format | ✅ | ⬜ | ⬜ |
| Allow output format | ✅ | ⬜ | ⬜ |
| Exit code behavior | ✅ | ⬜ | ⬜ |

### Mock Input Files

Create test fixtures for each platform:
- `testdata/input/copilot-cli-pretooluse.json`
- `testdata/input/vscode-pretooluse.json`
- `testdata/input/claude-pretooluse.json`

---

## Conclusion

All three hook systems share enough commonality that hookflow can provide a unified abstraction:

1. **Input:** Normalize `toolName`/`tool_name` and `toolArgs`/`tool_input`
2. **Output:** Generate platform-specific JSON format
3. **Exit codes:** Use exit 2 for blocking on VSCode/Claude
4. **Config:** Support both camelCase and PascalCase event names

hookflow's YAML workflow model provides significant advantages over raw JSON+scripts, making it a compelling choice for teams wanting consistent behavior across all three platforms.

---

## References

- [GitHub Copilot CLI Hooks](https://docs.github.com/en/copilot/reference/hooks-configuration)
- [VSCode Agent Hooks](https://code.visualstudio.com/docs/copilot/customization/hooks)
- [Claude Code Hooks](https://docs.anthropic.com/en/docs/claude-code/hooks)
- [Model Context Protocol](https://modelcontextprotocol.io)
