package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/d-kuro/gwq/internal/config"
	"github.com/d-kuro/gwq/internal/table"
	"github.com/d-kuro/gwq/internal/tmux"
	"github.com/d-kuro/gwq/internal/ui"
	"github.com/d-kuro/gwq/internal/utils"
	"github.com/d-kuro/gwq/pkg/models"
	"github.com/spf13/cobra"
)

var (
	tmuxListJSON  bool
	tmuxListCSV   bool
	tmuxListWatch bool
	tmuxListSort  string
)

var tmuxListCmd = &cobra.Command{
	Use:   "list",
	Short: "List active tmux sessions",
	Long: `List active tmux sessions with their information.

Shows running tmux sessions with context, identifier, duration and working directory.
Supports various output formats and real-time monitoring.`,
	Example: `  # List all sessions
  gwq tmux list

  # JSON output for scripting  
  gwq tmux list --json

  # Real-time monitoring
  gwq tmux list --watch

  # Sort by duration
  gwq tmux list --sort duration`,
	RunE: runTmuxList,
}

func init() {
	tmuxCmd.AddCommand(tmuxListCmd)

	tmuxListCmd.Flags().BoolVar(&tmuxListJSON, "json", false, "Output as JSON")
	tmuxListCmd.Flags().BoolVar(&tmuxListCSV, "csv", false, "Output as CSV")
	tmuxListCmd.Flags().BoolVarP(&tmuxListWatch, "watch", "w", false, "Real-time monitoring")
	tmuxListCmd.Flags().StringVarP(&tmuxListSort, "sort", "s", "", "Sort by field (duration, context, identifier)")
}

func runTmuxList(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	sessionManager := tmux.NewSessionManager(nil)

	if tmuxListWatch {
		return runTmuxListWatch(sessionManager, cfg)
	}

	return runTmuxListOnce(sessionManager, cfg)
}

func runTmuxListOnce(sessionManager *tmux.SessionManager, cfg *models.Config) error {
	sessions, err := sessionManager.ListSessions()
	if err != nil {
		return fmt.Errorf("failed to list sessions: %w", err)
	}

	sortedSessions := applySessionSort(sessions, tmuxListSort)

	switch {
	case tmuxListJSON:
		return outputSessionsJSON(sortedSessions)
	case tmuxListCSV:
		return outputSessionsCSV(sortedSessions)
	default:
		printer := ui.New(&cfg.UI)
		return outputSessionsTable(sortedSessions, printer)
	}
}

func runTmuxListWatch(sessionManager *tmux.SessionManager, cfg *models.Config) error {
	printer := ui.New(&cfg.UI)

	hideCursor := "\033[?25l"
	showCursor := "\033[?25h"
	clearScreen := "\033[H\033[2J"

	fmt.Print(hideCursor)
	defer fmt.Print(showCursor)

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	refresh := func() error {
		fmt.Print(clearScreen)

		sessions, err := sessionManager.ListSessions()
		if err != nil {
			return fmt.Errorf("failed to list sessions: %w", err)
		}

		sortedSessions := applySessionSort(sessions, tmuxListSort)

		fmt.Printf("tmux Sessions - Updated: %s\n", time.Now().Format("15:04:05"))
		fmt.Printf("Total: %d sessions\n\n", len(sessions))

		if err := outputSessionsTable(sortedSessions, printer); err != nil {
			return err
		}

		fmt.Println("\n[Press Ctrl+C to exit]")
		return nil
	}

	if err := refresh(); err != nil {
		return err
	}

	for range ticker.C {
		if err := refresh(); err != nil {
			fmt.Printf("Error: %v\n", err)
		}
	}

	return nil
}

func applySessionSort(sessions []*tmux.Session, sortBy string) []*tmux.Session {
	if sortBy == "" {
		return sessions
	}

	// Sorting is not implemented - return as-is
	// TODO: Implement sorting if needed in the future
	return sessions
}

func outputSessionsJSON(sessions []*tmux.Session) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(sessions)
}

func outputSessionsCSV(sessions []*tmux.Session) error {
	t := table.New().Headers("context", "identifier", "duration", "command", "working_dir", "session_name")

	// Write data
	for _, session := range sessions {
		duration := time.Since(session.StartTime).Round(time.Second).String()
		t.Row(
			session.Context,
			session.Identifier,
			duration,
			session.Command,
			session.WorkingDir,
			session.SessionName,
		)
	}

	return t.WriteCSV()
}

func outputSessionsTable(sessions []*tmux.Session, printer *ui.Printer) error {
	if len(sessions) == 0 {
		printer.PrintInfo("No tmux sessions found")
		return nil
	}

	t := table.New().Headers("SESSION", "DURATION", "WORKING_DIR")

	for _, session := range sessions {
		sessionIdentifier := session.Context + "/" + session.Identifier
		duration := formatSessionDuration(session.StartTime)
		workdir := formatWorkingDir(session.WorkingDir, printer)

		t.Row(sessionIdentifier, duration, workdir)
	}

	return t.Println()
}

func formatSessionDuration(startTime time.Time) string {
	duration := time.Since(startTime)
	switch {
	case duration < time.Minute:
		return "just now"
	case duration < time.Hour:
		mins := int(duration.Minutes())
		if mins == 1 {
			return "1 min"
		}
		return fmt.Sprintf("%d mins", mins)
	case duration < 24*time.Hour:
		hours := int(duration.Hours())
		if hours == 1 {
			return "1 hour"
		}
		return fmt.Sprintf("%d hours", hours)
	case duration < 7*24*time.Hour:
		days := int(duration.Hours() / 24)
		if days == 1 {
			return "1 day"
		}
		return fmt.Sprintf("%d days", days)
	default:
		weeks := int(duration.Hours() / 24 / 7)
		if weeks == 1 {
			return "1 week"
		}
		return fmt.Sprintf("%d weeks", weeks)
	}
}

func formatWorkingDir(workdir string, printer *ui.Printer) string {
	// Apply tilde home replacement first if enabled
	if printer != nil && printer.UseTildeHome() {
		workdir = utils.TildePath(workdir)
	}

	// Then apply truncation if needed
	if len(workdir) > 30 {
		return "..." + workdir[len(workdir)-27:]
	}
	return workdir
}
