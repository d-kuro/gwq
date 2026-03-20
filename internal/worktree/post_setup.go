package worktree

import (
	"context"
	"fmt"
	"os"

	"github.com/d-kuro/gwq/internal/command"
	"github.com/d-kuro/gwq/internal/filesystem"
	"github.com/d-kuro/gwq/internal/utils"
	"github.com/d-kuro/gwq/pkg/models"
)

// runPostWorktreeSetup runs file copy and setup commands for the new worktree.
func (m *Manager) runPostWorktreeSetup(worktreePath string) {
	if len(m.config.RepositorySettings) == 0 {
		return
	}

	repoRoot, err := m.git.GetMainRepositoryPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[gwq] warning: failed to get repository path: %v\n", err)
		return
	}

	repoSetting := findRepoSetting(m.config.RepositorySettings, repoRoot)
	if repoSetting == nil {
		return
	}

	// Copy files
	for _, err := range CopyFilesWithGlob(filesystem.NewStandardFileSystem(), repoRoot, worktreePath, repoSetting.CopyFiles) {
		fmt.Fprintf(os.Stderr, "[gwq] file copy error: %v\n", err)
	}

	// Run setup commands
	outputs, setupErrs := RunSetupCommands(
		context.Background(),
		command.NewStandardExecutor(),
		worktreePath,
		repoSetting.SetupCommands,
	)
	for i, out := range outputs {
		if out != "" {
			fmt.Fprintf(os.Stderr, "[gwq] setup command output: %s\n", out)
		}
		if i < len(setupErrs) && setupErrs[i] != nil {
			fmt.Fprintf(os.Stderr, "[gwq] setup command error: %v\n", setupErrs[i])
		}
	}
}

// findRepoSetting returns the first matching RepositorySetting for the given repo root.
func findRepoSetting(settings []models.RepositorySetting, repoRoot string) *models.RepositorySetting {
	for i, s := range settings {
		if utils.MatchPath(s.Repository, repoRoot) {
			return &settings[i]
		}
	}
	return nil
}
