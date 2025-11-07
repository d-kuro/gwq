package cmd

import (
	"github.com/spf13/cobra"
)

var taskAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add new tasks to the queue",
	Long: `Add new tasks to the queue for automated processing.

This command provides different task types that can be added to the queue.
Each task type has its own specific options and configuration.`,
}

func init() {
	taskCmd.AddCommand(taskAddCmd)
}
