---
name: hookify-multi-condition
description: Block dangerous rm commands targeting root paths
enabled: true
event: bash
action: block
conditions:
  - field: command
    operator: contains
    pattern: rm
  - field: command
    operator: regex_match
    pattern: -r[f\s]|--recursive
  - field: command
    operator: regex_match
    pattern: /\s*$|/[*]|\s/\s
---

⚠️ **Dangerous recursive delete blocked**

Do not run `rm -rf` targeting root or wildcard paths. This could destroy critical system files.
