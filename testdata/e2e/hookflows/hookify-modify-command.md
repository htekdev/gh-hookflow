---
name: hookify-modify-add-verbose
description: Append verbose flag to go test commands
event: bash
action: modify
modify_target: command
modify_strategy: append
pattern: go test
---
 -v -count=1
