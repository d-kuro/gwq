// Package ui provides user interface utilities for the gwq application.
package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/d-kuro/gwq/internal/table"
	"github.com/d-kuro/gwq/internal/utils"
	"github.com/d-kuro/gwq/pkg/models"
)

// Printer handles output formatting.
type Printer struct {
	useIcons     bool
	useTildeHome bool
}

// UseIcons returns whether icon display is enabled.
func (p *Printer) UseIcons() bool {
	return p.useIcons
}

// UseTildeHome returns whether home directory should be displayed as ~.
func (p *Printer) UseTildeHome() bool {
	return p.useTildeHome
}

// New creates a new Printer instance.
func New(config *models.UIConfig) *Printer {
	return &Printer{
		useIcons:     config.Icons,
		useTildeHome: config.TildeHome,
	}
}

// PrintWorktrees displays worktrees in a formatted table.
func (p *Printer) PrintWorktrees(worktrees []models.Worktree, verbose bool) {
	if len(worktrees) == 0 {
		fmt.Println("No worktrees found")
		return
	}

	var t *table.Builder
	if verbose {
		t = table.New().Headers("BRANCH", "PATH", "COMMIT", "CREATED", "TYPE")
		for _, wt := range worktrees {
			wtType := models.WorktreeTypeWorktree
			if wt.IsMain {
				wtType = models.WorktreeTypeMain
			}

			// Apply marker with consistent spacing
			var branchWithMarker string
			if wt.IsMain && p.useIcons {
				branchWithMarker = "● " + wt.Branch
			} else {
				branchWithMarker = "  " + wt.Branch // Two spaces to match "● " width
			}

			path := wt.Path
			if p.useTildeHome {
				path = utils.TildePath(path)
			}
			t.Row(
				branchWithMarker,
				path,
				p.truncateHash(wt.CommitHash),
				p.formatTime(wt.CreatedAt),
				wtType,
			)
		}
	} else {
		t = table.New().Headers("BRANCH", "PATH")
		for _, wt := range worktrees {
			// Apply marker with consistent spacing
			var branchWithMarker string
			if wt.IsMain && p.useIcons {
				branchWithMarker = "● " + wt.Branch
			} else {
				branchWithMarker = "  " + wt.Branch // Two spaces to match "● " width
			}

			path := wt.Path
			if p.useTildeHome {
				path = utils.TildePath(path)
			}
			t.Row(branchWithMarker, path)
		}
	}

	if err := t.Println(); err != nil {
		fmt.Printf("Error printing table: %v\n", err)
	}
}

// PrintWorktreesJSON displays worktrees in JSON format.
func (p *Printer) PrintWorktreesJSON(worktrees []models.Worktree) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(worktrees)
}

// PrintBranches displays branches in a formatted table.
func (p *Printer) PrintBranches(branches []models.Branch) {
	if len(branches) == 0 {
		fmt.Println("No branches found")
		return
	}

	t := table.New().Headers("BRANCH", "LAST COMMIT", "AUTHOR", "DATE")
	for _, branch := range branches {
		marker := ""
		if p.useIcons {
			if branch.IsCurrent {
				marker = "* "
			} else if branch.IsRemote {
				marker = "→ "
			} else {
				marker = "  "
			}
		}

		t.Row(
			marker+branch.Name,
			p.truncateMessage(branch.LastCommit.Message, 50),
			branch.LastCommit.Author,
			p.formatTime(branch.LastCommit.Date),
		)
	}

	if err := t.Println(); err != nil {
		fmt.Printf("Error printing table: %v\n", err)
	}
}

// PrintConfig displays configuration in a formatted manner.
func (p *Printer) PrintConfig(settings map[string]any) {
	p.printConfigRecursive("", settings)
}

// PrintError displays an error message.
func (p *Printer) PrintError(err error) {
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
}

// PrintSuccess displays a success message.
func (p *Printer) PrintSuccess(message string) {
	fmt.Println(message)
}

// PrintInfo displays an informational message.
func (p *Printer) PrintInfo(message string) {
	fmt.Println(message)
}

// PrintWorktreePath prints only the worktree path (for cd command).
func (p *Printer) PrintWorktreePath(path string) {
	if p.useTildeHome {
		path = utils.TildePath(path)
	}
	fmt.Println(path)
}

// truncateHash truncates a commit hash to 8 characters.
func (p *Printer) truncateHash(hash string) string {
	if len(hash) > 8 {
		return hash[:8]
	}
	return hash
}

// truncateMessage truncates a message to the specified length.
func (p *Printer) truncateMessage(message string, maxLen int) string {
	if len(message) > maxLen {
		return message[:maxLen-3] + "..."
	}
	return message
}

// formatTime formats a time value for display.
func (p *Printer) formatTime(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}

	now := time.Now()
	diff := now.Sub(t)

	switch {
	case diff < time.Hour:
		return fmt.Sprintf("%d minutes ago", int(diff.Minutes()))
	case diff < 24*time.Hour:
		return fmt.Sprintf("%d hours ago", int(diff.Hours()))
	case diff < 7*24*time.Hour:
		return fmt.Sprintf("%d days ago", int(diff.Hours()/24))
	default:
		return t.Format("2006-01-02")
	}
}

// printConfigRecursive recursively prints configuration values.
func (p *Printer) printConfigRecursive(prefix string, data any) {
	switch v := data.(type) {
	case map[string]any:
		for key, value := range v {
			newPrefix := key
			if prefix != "" {
				newPrefix = prefix + "." + key
			}
			p.printConfigRecursive(newPrefix, value)
		}
	default:
		fmt.Printf("%s = %v\n", prefix, v)
	}
}
