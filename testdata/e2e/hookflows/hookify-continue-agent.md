---
name: hookify-force-continue
description: Force agent to continue when tasks are incomplete
event: agentStop
action: continue
conditions:
  - field: message
    operator: contains
    pattern: incomplete
---

You are not done yet. Review your task list and continue working on the remaining items. Do not stop until all tasks are marked complete.
