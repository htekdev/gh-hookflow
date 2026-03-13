---
name: require-go-test-before-commit
description: Block git commits when go test was not run in the session
enabled: true
event: bash
action: block
conditions:
  - field: command
    operator: contains
    pattern: git commit
  - field: transcript
    operator: not_contains
    pattern: go test
lifecycle: pre
---

⚠️ **Tests Required Before Commit**

You must run `go test` before committing. No test execution was found in this session.

Run your tests first:

```
go test ./... -v
```

Then try your commit again.
