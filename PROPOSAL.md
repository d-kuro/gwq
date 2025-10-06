# Proposal: Configurable Setup Commands for Worktree Creation

## Overview

This proposal recommends implementing a configurable mechanism to automatically run setup commands (such as `npm install`) and copy files whenever a new worktree is created in the `gwq` project. The configuration for these actions should be defined on a per-repository basis, but stored in the global config file (e.g., `~/.config/gwq/config.toml`). Each repository can have its own override section in the global config, allowing all configuration to remain centralized. The approach is based on Option 1 from the `POSSIBLE_SOLUTIONS.md` document, but with per-repository overrides in the global config.

## Implementation Details

### 1. Repository Settings in Global Config
- Add a `repository_settings` array to the global config file (e.g., `~/.config/gwq/config.toml`).
- Each entry in `repository_settings` specifies a repository path or pattern, and its own `setup_commands` and `copy_files`.
- When creating a new worktree, `gwq` checks if the current repository matches any settings entry and applies those options; otherwise, it falls back to global defaults.

**Example config snippet:**

```toml
[worktree]
basedir = "~/worktrees"
auto_mkdir = true

[[repository_settings]]
repo = "~/src/myproject"
copy_files = ["templates/.env.example"]
setup_commands = ["npm install"]

[[repository_settings]]
repo = "~/src/anotherproject"
copy_files = ["config/*.json"]
setup_commands = ["pip install -r requirements.txt"]
```

### 2. Worktree Manager Logic
- After successfully creating a new worktree (in `Manager.Add` and `Manager.AddFromBase`):
  1. Determine the repository path for the new worktree.
  2. Check the global config for a matching `repository_settings` entry.
  3. If a match is found, use its `copy_files` and `setup_commands` for this worktree. If not, use global defaults (if any).
  4. If `copy_files` is set, iterate over the list and copy each file or glob-matched set of files (relative to the repository root) into the new worktree directory. Preserve directory structure where appropriate. Log any errors encountered, and surface them to the user, but do not abort the worktree creation.
  5. After copying files, iterate over the `setup_commands` list and execute each command in the context of the new worktree directory.
- Log output and errors for each copy or command execution, and surface any failures to the user.

### 3. Error Handling
- If a file copy or setup command fails, report the error but do not remove the worktree.

## Pros
- Declarative and user-configurable on a per-repository basis, but with centralized management.
- Integrates cleanly with the existing config-driven architecture.
- Easy to document, maintain, and extend.
- Follows Go best practices for configuration and separation of concerns.

## Cons
- Requires careful handling of command execution and error reporting.
- Slightly more complex config parsing (list of strings).
- Requires matching repository paths/patterns in the global config, which may be less discoverable than per-repo files.

## Summary

This approach provides a flexible, maintainable, and idiomatic solution for automating setup tasks and file preparation in new worktrees, with configuration that is unique to each repository but managed centrally. It is well-aligned with the current architecture of the `gwq` project.
