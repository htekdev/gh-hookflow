---
name: hookify-inject-governance
description: Inject governance context into subagent starts
event: subagentStart
action: inject
conditions:
  - field: agent_name
    operator: contains
    pattern: coding
---

## Governance Rules

You must follow these rules when operating as a subagent:
1. Never commit secrets or API keys
2. Always write tests for new functionality
3. Follow the repo's established conventions
4. Use dev-workflow tools instead of raw git commands
