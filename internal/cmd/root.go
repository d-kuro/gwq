// Package cmd provides CLI commands for the gwq application.
package cmd

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/d-kuro/gwq/internal/config"
	"github.com/spf13/cobra"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:   "gwq",
	Short: "Git worktree manager",
	Long: `gwq is a CLI tool for efficiently managing Git worktrees.

Like how 'ghq' manages repository clones, gwq provides intuitive 
operations for creating, switching, and deleting worktrees using 
a fuzzy finder interface.`,
	Version: getVersionString(),
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.CompletionOptions.DisableDefaultCmd = true
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if err := config.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing config: %v\n", err)
		os.Exit(1)
	}
}

// getVersionString returns a formatted version string using build info
func getVersionString() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return fmt.Sprintf("%s (commit: %s, built: %s)", version, commit, date)
	}

	// Extract version information from build info
	buildVersion := version
	buildCommit := commit
	buildDate := date

	// Try to get version from module
	if info.Main.Version != "" && info.Main.Version != "(devel)" {
		buildVersion = info.Main.Version
	}

	// Try to get commit and date from VCS settings
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			if setting.Value != "" {
				buildCommit = setting.Value
				if len(buildCommit) > 7 {
					buildCommit = buildCommit[:7]
				}
			}
		case "vcs.time":
			if setting.Value != "" {
				buildDate = setting.Value
			}
		}
	}

	return fmt.Sprintf("%s (commit: %s, built: %s)", buildVersion, buildCommit, buildDate)
}
