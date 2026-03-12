---
name: hookify-warn-console-log
description: Warn when adding console.log statements
enabled: true
event: file
action: warn
conditions:
  - field: file_path
    operator: regex_match
    pattern: \.(js|ts|jsx|tsx)$
  - field: new_text
    operator: contains
    pattern: console.log
---

⚠️ **console.log detected**

Avoid adding `console.log` statements in production code. Use a proper logging framework instead.
