# IMPLEMENTATION PLAN: Configurable Setup Commands for Worktree Creation

## Overview
This plan details the steps to implement configurable setup commands and file copying for new worktree creation in the `gwq` project, using repository-specific settings in the global config file, as described in the updated proposal.

---

## 1. Extend Configuration
- **Add `RepositorySetting` struct** in `pkg/models/models.go`:
  - Fields: `Repository string`, `SetupCommands []string`, `CopyFiles []string`.
- **Add `RepositorySettings []RepositorySetting`** to the main config struct.
- **Update config loading** in `internal/config/config.go`:
  - Add TOML parsing for the `repository_settings` array.
  - Update example config and documentation.

## 2. File Copy Logic
- **Implement file copy with glob support**:
  - Use `pkg/filesystem` abstraction for file operations.
  - For each entry in the selected repository setting's `CopyFiles`, resolve globs and copy files into the new worktree directory.
  - Preserve directory structure where appropriate.
  - Log errors for missing files or copy failures, but do not abort worktree creation.

## 3. Setup Command Execution
- **Run setup commands in the new worktree directory**:
  - Use `pkg/command.CommandExecutor` to execute each command from the selected repository setting's `SetupCommands`.
  - Run each command in the context of the new worktree directory.
  - Capture and log stdout/stderr for each command.
  - Log errors, but do not abort worktree creation.

## 4. Integrate with Worktree Manager
- **Update `Manager.Add` and `Manager.AddFromBase`** in `internal/worktree/worktree.go`:
  - After worktree creation, determine the repository path for the new worktree.
  - Check the global config for a matching `repository_settings` entry.
  - If a match is found, use its `copy_files` and `setup_commands` for this worktree. If not, use global defaults (if any).
  - Perform file copying and setup command execution as described above.
  - Ensure all errors are logged and surfaced to the user, but do not remove the worktree on failure.

## 5. Testing
- **Extend and add tests**:
  - Test config parsing for `repository_settings` and matching logic.
  - Test file copy logic (including globbing and error handling).
  - Test setup command execution (using mocks for command execution).
  - Test error handling and logging.

## 6. Documentation
- **Update documentation**:
  - Add new config fields and override examples to `README.md` and example config.
  - Document the new behavior and error handling.

---

## Acceptance Criteria
- Users can specify `copy_files` and `setup_commands` for each repository in the global config.
- When a new worktree is created, the correct override is selected and files are copied and setup commands are run as configured.
- Errors are logged and surfaced, but do not abort worktree creation.
- All new code is covered by tests and documented.
