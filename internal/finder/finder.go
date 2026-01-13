// Package finder provides fuzzy finder integration for the gwq application.
package finder

import (
	"fmt"
	"strings"
	"time"

	"github.com/d-kuro/gwq/internal/git"
	"github.com/d-kuro/gwq/internal/tmux"
	"github.com/d-kuro/gwq/internal/utils"
	"github.com/d-kuro/gwq/pkg/models"
	"github.com/ktr0731/go-fuzzyfinder"
)

// Finder provides fuzzy finder functionality.
type Finder struct {
	git          *git.Git
	config       *models.FinderConfig
	useTildeHome bool
}

// New creates a new Finder instance.
func New(g *git.Git, config *models.FinderConfig) *Finder {
	return &Finder{
		git:    g,
		config: config,
	}
}

// NewWithUI creates a new Finder instance with UI configuration.
func NewWithUI(g *git.Git, config *models.FinderConfig, uiConfig *models.UIConfig) *Finder {
	return &Finder{
		git:          g,
		config:       config,
		useTildeHome: uiConfig.TildeHome,
	}
}

// SelectWorktree displays a fuzzy finder for worktree selection.
func (f *Finder) SelectWorktree(worktrees []models.Worktree) (*models.Worktree, error) {
	if len(worktrees) == 0 {
		return nil, fmt.Errorf("no worktrees available for selection")
	}

	opts := []fuzzyfinder.Option{
		fuzzyfinder.WithPromptString("Select worktree> "),
	}

	if f.config.Preview {
		opts = append(opts, fuzzyfinder.WithPreviewWindow(func(i, w, h int) string {
			if i == -1 {
				return ""
			}
			return f.generateWorktreePreview(worktrees[i], h)
		}))
	}

	idx, err := fuzzyfinder.Find(
		worktrees,
		func(i int) string {
			wt := worktrees[i]
			marker := ""
			if wt.IsMain {
				marker = "[main] "
			}
			path := wt.Path
			if f.useTildeHome {
				path = utils.TildePath(path)
			}
			return fmt.Sprintf("%s%s (%s)", marker, wt.Branch, path)
		},
		opts...,
	)

	if err != nil {
		return nil, err
	}

	return &worktrees[idx], nil
}

// SelectBranch displays a fuzzy finder for branch selection.
func (f *Finder) SelectBranch(branches []models.Branch) (*models.Branch, error) {
	if len(branches) == 0 {
		return nil, fmt.Errorf("no branches available for selection")
	}

	opts := []fuzzyfinder.Option{
		fuzzyfinder.WithPromptString("Select branch> "),
	}

	if f.config.Preview {
		opts = append(opts, fuzzyfinder.WithPreviewWindow(func(i, w, h int) string {
			if i == -1 {
				return ""
			}
			return f.generateBranchPreview(branches[i], h)
		}))
	}

	idx, err := fuzzyfinder.Find(
		branches,
		func(i int) string {
			branch := branches[i]
			marker := ""
			if branch.IsCurrent {
				marker = "* "
			} else if branch.IsRemote {
				marker = "→ "
			}
			return fmt.Sprintf("%s%s", marker, branch.Name)
		},
		opts...,
	)

	if err != nil {
		return nil, err
	}

	return &branches[idx], nil
}

// SelectMultipleWorktrees displays a fuzzy finder for multiple worktree selection.
func (f *Finder) SelectMultipleWorktrees(worktrees []models.Worktree) ([]models.Worktree, error) {
	if len(worktrees) == 0 {
		return nil, fmt.Errorf("no worktrees available for multiple selection")
	}

	opts := []fuzzyfinder.Option{
		fuzzyfinder.WithPromptString("Select worktrees (Tab to select multiple)> "),
	}

	if f.config.Preview {
		opts = append(opts, fuzzyfinder.WithPreviewWindow(func(i, w, h int) string {
			if i == -1 {
				return ""
			}
			return f.generateWorktreePreview(worktrees[i], h)
		}))
	}

	indices, err := fuzzyfinder.FindMulti(
		worktrees,
		func(i int) string {
			wt := worktrees[i]
			marker := ""
			if wt.IsMain {
				marker = "[main] "
			}
			path := wt.Path
			if f.useTildeHome {
				path = utils.TildePath(path)
			}
			return fmt.Sprintf("%s%s (%s)", marker, wt.Branch, path)
		},
		opts...,
	)

	if err != nil {
		return nil, err
	}

	selected := make([]models.Worktree, len(indices))
	for i, idx := range indices {
		selected[i] = worktrees[idx]
	}

	return selected, nil
}

// SelectSession displays a fuzzy finder for session selection.
func (f *Finder) SelectSession(sessions []*tmux.Session) (*tmux.Session, error) {
	if len(sessions) == 0 {
		return nil, fmt.Errorf("no tmux sessions available for selection")
	}

	opts := f.buildSessionFinderOptions(sessions)

	idx, err := fuzzyfinder.Find(sessions, f.formatSessionForDisplay(sessions), opts...)
	if err != nil {
		return nil, err
	}

	return sessions[idx], nil
}

// SelectMultipleSessions displays a fuzzy finder for multiple session selection.
func (f *Finder) SelectMultipleSessions(sessions []*tmux.Session) ([]*tmux.Session, error) {
	if len(sessions) == 0 {
		return nil, fmt.Errorf("no tmux sessions available for multiple selection")
	}

	opts := f.buildSessionFinderOptions(sessions)
	opts[0] = fuzzyfinder.WithPromptString("Select sessions (Tab to select multiple)> ")

	indices, err := fuzzyfinder.FindMulti(sessions, f.formatSessionForDisplay(sessions), opts...)
	if err != nil {
		return nil, err
	}

	selected := make([]*tmux.Session, len(indices))
	for i, idx := range indices {
		selected[i] = sessions[idx]
	}

	return selected, nil
}

// generateSessionPreview generates preview content for a session.
func (f *Finder) generateSessionPreview(session *tmux.Session, maxLines int) string {
	preview := []string{
		fmt.Sprintf("Session: %s", session.SessionName),
		fmt.Sprintf("Context: %s", session.Context),
		fmt.Sprintf("Identifier: %s", session.Identifier),
		fmt.Sprintf("Command: %s", session.Command),
		fmt.Sprintf("Duration: %s", formatDuration(time.Since(session.StartTime))),
		fmt.Sprintf("Started: %s", session.StartTime.Format("2006-01-02 15:04:05")),
	}

	if session.WorkingDir != "" {
		preview = append(preview, fmt.Sprintf("Directory: %s", session.WorkingDir))
	}

	if len(session.Metadata) > 0 {
		preview = append(preview, "", "Metadata:")
		for key, value := range session.Metadata {
			preview = append(preview, fmt.Sprintf("  %s: %s", key, value))
		}
	}

	// Limit to maxLines
	if len(preview) > maxLines {
		preview = preview[:maxLines]
	}

	return strings.Join(preview, "\n")
}

func formatDuration(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		mins := int(d.Minutes())
		if mins == 1 {
			return "1 min"
		}
		return fmt.Sprintf("%d mins", mins)
	case d < 24*time.Hour:
		hours := int(d.Hours())
		if hours == 1 {
			return "1 hour"
		}
		return fmt.Sprintf("%d hours", hours)
	default:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1 day"
		}
		return fmt.Sprintf("%d days", days)
	}
}

// generateWorktreePreview generates preview content for a worktree.
func (f *Finder) generateWorktreePreview(wt models.Worktree, maxLines int) string {
	path := wt.Path
	if f.useTildeHome {
		path = utils.TildePath(path)
	}
	preview := []string{
		fmt.Sprintf("Branch: %s", wt.Branch),
		fmt.Sprintf("Path: %s", path),
		fmt.Sprintf("Commit: %s", truncateHash(wt.CommitHash)),
		fmt.Sprintf("Created: %s", wt.CreatedAt.Format("2006-01-02 15:04")),
	}

	if wt.IsMain {
		preview = append(preview, "Type: Main worktree")
	} else {
		preview = append(preview, "Type: Additional worktree")
	}

	remainingLines := maxLines - len(preview) - 2
	if remainingLines > 0 && f.git != nil {
		preview = append(preview, "", "Recent commits:")
		commits, err := f.git.GetRecentCommits(wt.Path, remainingLines)
		if err == nil {
			for _, commit := range commits {
				preview = append(preview, fmt.Sprintf("  %s %s",
					truncateHash(commit.Hash),
					truncateMessage(commit.Message, 50),
				))
			}
		}
	}

	return strings.Join(preview, "\n")
}

// generateBranchPreview generates preview content for a branch.
func (f *Finder) generateBranchPreview(branch models.Branch, maxLines int) string {
	branchType := "Local"
	if branch.IsCurrent {
		branchType = "Current"
	} else if branch.IsRemote {
		branchType = "Remote"
	}

	preview := []string{
		fmt.Sprintf("Branch: %s", branch.Name),
		fmt.Sprintf("Type: %s", branchType),
		fmt.Sprintf("Last commit: %s", truncateMessage(branch.LastCommit.Message, 60)),
		fmt.Sprintf("Author: %s", branch.LastCommit.Author),
		fmt.Sprintf("Date: %s", branch.LastCommit.Date.Format("2006-01-02 15:04")),
		fmt.Sprintf("Hash: %s", truncateHash(branch.LastCommit.Hash)),
	}

	return strings.Join(preview[:min(len(preview), maxLines)], "\n")
}

// truncateHash truncates a commit hash to 8 characters.
func truncateHash(hash string) string {
	if len(hash) > 8 {
		return hash[:8]
	}
	return hash
}

// truncateMessage truncates a message to the specified length.
func truncateMessage(message string, maxLen int) string {
	if len(message) > maxLen {
		return message[:maxLen-3] + "..."
	}
	return message
}

// buildSessionFinderOptions builds common options for session fuzzy finder.
func (f *Finder) buildSessionFinderOptions(sessions []*tmux.Session) []fuzzyfinder.Option {
	opts := []fuzzyfinder.Option{
		fuzzyfinder.WithPromptString("Select session> "),
	}

	if f.config.Preview {
		opts = append(opts, fuzzyfinder.WithPreviewWindow(func(i, w, h int) string {
			if i == -1 {
				return ""
			}
			return f.generateSessionPreview(sessions[i], h)
		}))
	}

	return opts
}

// formatSessionForDisplay formats a session for display in the fuzzy finder.
func (f *Finder) formatSessionForDisplay(sessions []*tmux.Session) func(int) string {
	return func(i int) string {
		session := sessions[i]
		return fmt.Sprintf("%s/%s - %s", session.Context, session.Identifier, session.Command)
	}
}
