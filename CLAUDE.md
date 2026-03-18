# Agent Guidelines

## Core Principles

- **Do NOT maintain backward compatibility** unless explicitly requested. Break things boldly.
- **Keep this file under 20-30 lines of instructions.**

---

## Project Overview

**Project type:** CLI tool — Git worktree manager
**Primary language:** Go

---

## Commands

```bash
make build          # Build to ./gwq
make test           # go test ./...
make lint           # golangci-lint run
make install        # Build and install gwq
```

---

## Code Conventions

- Follow the existing patterns in the codebase
- Prefer explicit over clever
- Delete dead code immediately

---

## Maintenance Notes

**Keep this file lean and current:**

1. **Review regularly** - stale instructions poison the agent's context
2. **CRITICAL: Keep total under 20-30 lines** - move detailed docs to separate files and reference them
3. **Update commands immediately** when workflows change
4. **Delete anything the agent can infer** from your code
